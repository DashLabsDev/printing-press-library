// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.
// Actionable auth error and session-health tests (fake values only).

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestKeychainDenialSuggestsBrowserLoginOrCurl(t *testing.T) {
	for _, msg := range []string{
		"pycookiecheat: SecKeychainSearchCopyNext: The user name or passphrase you entered is not correct. (-25293)",
		"Unable to get key for cookie decryption: Chrome Safe Storage not accessible",
		"secretstorage.exceptions.LockedException: keyring is locked",
	} {
		err := keychainHint(errors.New(msg))
		if !strings.Contains(err.Error(), "auth login' (no keychain needed)") || !strings.Contains(err.Error(), "--cookies-file -") {
			t.Fatalf("hint for %q = %q", msg, err)
		}
	}
	if err := keychainHint(errors.New("network down")); strings.Contains(err.Error(), "keychain") {
		t.Fatalf("unrelated error got keychain hint: %v", err)
	}
}

func TestExpiredSessionErrorsSayRunAuthLogin(t *testing.T) {
	err := classifyAPIErrorOnly(errors.New("GET /graphql returned HTTP 401: unauthorized"))
	if !strings.Contains(err.Error(), "missing or expired") || !strings.Contains(err.Error(), "auth login") || strings.Contains(err.Error(), "--chrome") {
		t.Fatalf("401 hint = %q", err)
	}
	if ExitCode(err) != 4 {
		t.Fatalf("401 exit = %d", ExitCode(err))
	}
}

func TestSigninRateLimitMessage(t *testing.T) {
	err := classifyAPIErrorOnly(errors.New("GET https://signin.costco.com/x returned HTTP 429"))
	if !strings.Contains(err.Error(), "rate-limiting sign-in") || !strings.Contains(err.Error(), "15-30 minutes") {
		t.Fatalf("429 hint = %q", err)
	}
	if !loginRateLimited("https://signin.costco.com/oauth2/authorize", "Too Many Requests") {
		t.Fatal("signin 429 page not detected")
	}
	if loginRateLimited("https://example.com/429", "429") {
		t.Fatal("non-Costco host treated as Costco rate limit")
	}
	if loginRateLimited("https://signin.costco.com/oauth2/authorize", "Sign In") {
		t.Fatal("normal sign-in page treated as rate limited")
	}
}

func TestSessionHealthVerdicts(t *testing.T) {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	h := checkSessionHealth(context.Background(), nil, now.Add(-time.Hour), now)
	if !h.Expired || !strings.HasPrefix(h.Expires, "ERROR expired") || !strings.Contains(h.Expires, "auth login") {
		t.Fatalf("expired = %+v", h)
	}
	h = checkSessionHealth(context.Background(), nil, now.Add(72*time.Hour), now)
	if h.Expired || !strings.Contains(h.Expires, "in 3 days") {
		t.Fatalf("future = %+v", h)
	}
	if !sessionErrorsLookExpired([]error{errors.New("HTTP 401"), errors.New("GraphQL UserType failed: unauthorized")}) {
		t.Fatal("401s should read as expired")
	}
	if sessionErrorsLookExpired([]error{errors.New("HTTP 401"), errors.New("dial tcp: timeout")}) {
		t.Fatal("network error should not read as expired")
	}
}

func TestAuthStatusExpiredSessionExitsWithHint(t *testing.T) {
	routedSameDay(t, nil)
	writeFakeLogin(t)
	expired := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer expired.Close()
	t.Setenv("COSTCO_SAMEDAY_BASE_URL", expired.URL)
	out, _, err := runRootSplit(t, "auth", "status", "--json")
	var m map[string]any
	if jerr := json.Unmarshal([]byte(out), &m); jerr != nil {
		t.Fatalf("status not JSON: %s", out)
	}
	if m["session_valid"] != false || !strings.Contains(fmt.Sprint(m["session"]), "expired or missing") {
		t.Fatalf("status = %v", m)
	}
	if err == nil || !strings.Contains(err.Error(), "auth login") || ExitCode(err) != 4 {
		t.Fatalf("expired status err = %v", err)
	}
	if strings.Contains(out, "FAKESIDVALUE") {
		t.Fatal("status leaks the cookie value")
	}
}

func TestAuthStatusUnansweredProbeIsNotVerified(t *testing.T) {
	routedSameDay(t, map[string]string{})
	writeFakeLogin(t)
	out, _, err := runRootSplit(t, "auth", "status", "--json")
	if err != nil {
		t.Fatalf("unanswered probe should not fail status: %v", err)
	}
	if !strings.Contains(out, "not verified") {
		t.Fatalf("status = %s", out)
	}
}

func TestDoctorReportsSession(t *testing.T) {
	routedSameDay(t, map[string]string{
		"UserType":             `{"data":{"currentUser":{"guest":true}}}`,
		"GlobalMainMenuLayout": `{"data":{}}`,
	})
	writeFakeLogin(t)
	out, _, _ := runRootSplit(t, "doctor", "--json")
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("doctor not JSON: %s", out)
	}
	if s, _ := m["session"].(string); !strings.Contains(s, "guest") || !strings.Contains(s, "auth login") {
		t.Fatalf("doctor session = %v", m["session"])
	}
	if _, ok := m["session_expires"]; !ok {
		t.Fatalf("doctor lacks session_expires: %v", m)
	}
	if strings.Contains(out, "FAKESIDVALUE") {
		t.Fatal("doctor leaks the cookie value")
	}
}

func TestLoginProfileIsCLIOwned(t *testing.T) {
	dir := isolatedAuthEnv(t)
	p, err := defaultBrowserProfileDir()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(p, dir) || !strings.HasSuffix(p, browserLoginProfile) || strings.Contains(p, "google-chrome") || strings.Contains(p, "Google/Chrome") {
		t.Fatalf("login profile %q is not under the CLI data dir", p)
	}
	if got := sameDayCookieURL("https://sameday.costco.com/store"); got != sameDayOrigin {
		t.Fatalf("cookie URL = %q", got)
	}
}

func TestBrowserNotFoundNamesFallbacks(t *testing.T) {
	t.Setenv(browserLoginEnvBinary, "")
	if _, err := findBrowserBinary("/nonexistent/chrome-fake"); err == nil {
		t.Fatal("missing explicit browser accepted")
	}
	if !strings.Contains(errBrowserNotFound.Error(), "--cookies-file -") || !strings.Contains(errBrowserNotFound.Error(), "--chrome") {
		t.Fatal("browser-not-found error must name the fallbacks")
	}
}
