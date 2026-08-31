// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel source layer — NOT generator output.
//
// auth implements Snipd's UUID device-pairing flow, the only way to mint an
// account token. The reference implementation is Snipd's own open-source
// Obsidian plugin (github.com/snipd-app/snipd-obsidian): connectToSnipd in
// src/settings_modal.ts, with AUTH_URL in src/main.ts. Same three HTTP facts in Go.
//
// The flow, in full:
//
//  1. generate a random v4 UUID, client-side and disposable
//  2. the user opens app.snipd.com/obsidian/auth?uuid=<uuid> and signs in
//  3. GET <api>/obsidian/auth?uuid=<uuid> returns {"token": "..."}
//
// Step 3 is a LONG-POLL, not a client retry loop: the Snipd server holds the
// request open until the browser sign-in in step 2 completes. The UUID is the
// only thing correlating the two — it is random, client-generated, and carries
// no secret, which is why it is safe to put in a URL and to log.
package snipd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/snipd/internal/cliutil"
)

const (
	// signInBase is Snipd-hosted web, not an API endpoint. It is titled
	// "Obsidian integration" purely because that is Snipd's internal name for
	// this export API; no Obsidian app, plugin, or mobile app is involved.
	signInBase = "https://app.snipd.com/obsidian/auth"

	// authPollPath is joined to the configured API base URL.
	authPollPath = "/obsidian/auth"

	// maxAuthBody caps the token response read. The real body is a few dozen
	// bytes; anything larger is a captive portal or an error page.
	maxAuthBody = 64 << 10

	// pollAttempts guards against the server closing the long-poll early
	// (proxy idle timeout, load-balancer reap) before the user has finished
	// signing in. The plugin's equivalent is telling the user to click
	// "Connect" again; here it is transparent.
	pollAttempts = 3
)

// ErrSignInIncomplete reports that the pairing endpoint answered but had no
// token to give — the browser sign-in did not complete inside the window.
// Distinct from a transport failure so the CLI can advise "try again" rather
// than "check your network".
var ErrSignInIncomplete = fmt.Errorf("sign-in did not complete")

// SignInURL builds the browser URL the user signs in at. The uuid is a
// throwaway correlator, not a credential.
func SignInURL(pairingUUID string) string {
	return signInBase + "?uuid=" + url.QueryEscape(pairingUUID)
}

// PollForToken waits for the browser sign-in keyed to pairingUUID and returns
// the minted account token. The caller owns the deadline via ctx; this does
// not impose its own, because the server deliberately holds the connection.
//
// The token is never written to an error, a log, or any returned value other
// than the success return.
func PollForToken(ctx context.Context, baseURL, pairingUUID string) (string, error) {
	endpoint := strings.TrimRight(baseURL, "/") + authPollPath + "?uuid=" + url.QueryEscape(pairingUUID)

	// No client-level deadline: the long-poll is meant to hang. Cancellation
	// flows through ctx, matching NewClient's rationale for the export POST.
	hc := &http.Client{Timeout: 0}

	// Same adaptive limiter the export client uses. Pairing is low-volume (at
	// most pollAttempts requests per login), but an operator re-running `auth
	// login` after a failed sign-in can hit the endpoint in a tight loop, and
	// an unlimited retry against an auth endpoint is exactly what earns a 429.
	limiter := cliutil.NewAdaptiveLimiter(defaultRateSec)

	var lastErr error = ErrSignInIncomplete
	for attempt := 0; attempt < pollAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		limiter.Wait()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("Accept", "application/json")

		resp, err := hc.Do(req)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return "", ctxErr
			}
			lastErr = err
			continue
		}

		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxAuthBody))
		_ = resp.Body.Close()

		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			limiter.OnRateLimit()
			// Typed, so the CLI's shared rate-limit handling can advise a wait
			// instead of reporting a generic pairing failure. Body omitted for
			// the same reason as below.
			lastErr = &cliutil.RateLimitError{
				URL:        endpoint,
				RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After")),
			}
			continue
		}
		if resp.StatusCode != http.StatusOK {
			// Body deliberately omitted: on a success-shaped response it holds
			// the token, and this error string may be printed or logged.
			lastErr = fmt.Errorf("snipd pairing endpoint returned HTTP %d", resp.StatusCode)
			continue
		}

		var payload struct {
			Token string `json:"token"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			lastErr = fmt.Errorf("unexpected response from snipd pairing endpoint (not JSON)")
			continue
		}
		if tok := strings.TrimSpace(payload.Token); tok != "" {
			return tok, nil
		}

		// 200 with no token: the poll window elapsed before the user finished.
		lastErr = ErrSignInIncomplete
	}

	return "", lastErr
}
