// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// walmartGuardApplies reports whether the read-only guard and bot-challenge
// stop apply. They apply to every non-loopback host; loopback is left alone
// and reserved test names (.test, .example, .invalid) are left alone
// only so the generator's own httptest-based client tests keep exercising
// generic retry/POST behaviour.
func walmartGuardApplies(baseURL string) bool {
	u, err := url.Parse(baseURL)
	if err != nil || u.Hostname() == "" {
		return true
	}
	host := u.Hostname()
	if host == "localhost" {
		return false
	}
	// RFC 2606 / 6761 reserved names never resolve on the internet.
	for _, tld := range []string{".test", ".example", ".invalid", ".localhost"} {
		if strings.HasSuffix(host, tld) {
			return false
		}
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return false
	}
	return true
}

// ErrWriteBlocked is returned before any network I/O when a request would
// change Walmart account state (cart, slot reservation, checkout, orders).
var ErrWriteBlocked = errors.New("walmart: write operation blocked (this CLI is read-only)")

// ErrBotChallenge is returned on the first PerimeterX/Akamai challenge. The
// client never retries a challenge; the user must refresh cookies from a
// browser session instead.
var ErrBotChallenge = errors.New("walmart: bot challenge")

// deniedOperations are Walmart orchestra GraphQL operations this CLI must
// never send. Matching is case-insensitive on the operation path segment
// and on the x-apollo-operation-name / x-o-gql-query headers.
var deniedOperations = []string{
	"updateitems", "mergeandgetcart", "reserveslot", "reserveslotmutation",
	"setpickup", "setshipping", "setfulfillment", "createcontract", "placeorder",
	"createdeliveryaddress", "createaccountcreditcard", "savetenderplantopc",
	"updatetenderplan", "cancelorder", "initiatereturn", "amendorder",
	"cancelreservation", "createreservation", "checkout",
}

// checkWalmartWrite rejects any request that could mutate account state.
// This CLI never sends cart/slot/checkout mutations; cart changes are
// link-only (affiliate add-to-cart URL printed for the user to open).
func checkWalmartWrite(method, path string, headers map[string]string) error {
	if method != http.MethodGet {
		return fmt.Errorf("%w: %s %s", ErrWriteBlocked, method, path)
	}
	lowerPath := strings.ToLower(path)
	for _, op := range deniedOperations {
		if strings.Contains(lowerPath, "/"+op+"/") || strings.HasSuffix(lowerPath, "/"+op) {
			return fmt.Errorf("%w: operation %s", ErrWriteBlocked, op)
		}
	}
	for k, v := range headers {
		lk := strings.ToLower(k)
		lv := strings.ToLower(strings.TrimSpace(v))
		if lk == "x-o-gql-query" && strings.HasPrefix(lv, "mutation") {
			return fmt.Errorf("%w: GraphQL mutation", ErrWriteBlocked)
		}
		if lk == "x-apollo-operation-name" || lk == "x-o-gql-query" {
			for _, op := range deniedOperations {
				if strings.HasSuffix(lv, op) || lv == op {
					return fmt.Errorf("%w: operation %s", ErrWriteBlocked, op)
				}
			}
		}
	}
	return nil
}

// isBotChallenge classifies a response as a PerimeterX / Akamai challenge.
// 412 (PerimeterX), 418 (orchestra scripted-traffic), 429 (Akamai), 456
// (Walmart bot block) and any landing on /blocked count.
func isBotChallenge(resp *http.Response, body []byte) bool {
	if resp == nil {
		return false
	}
	switch resp.StatusCode {
	case 412, 418, 429, 456:
		return true
	}
	if resp.Request != nil && resp.Request.URL != nil && strings.HasPrefix(resp.Request.URL.Path, "/blocked") {
		return true
	}
	if resp.StatusCode == http.StatusOK && len(body) > 0 && body[0] == '<' {
		s := string(body)
		if strings.Contains(s, "px-captcha") || strings.Contains(s, "Robot or human") {
			return true
		}
	}
	return false
}

// walmartTransportError describes a failed round trip without echoing the
// request URL or query (url.Error embeds the full URL). A connection the
// server closed or reset is treated as a possible bot block.
func walmartTransportError(method, path string, err error) error {
	inner := err
	var ue *url.Error
	if errors.As(err, &ue) && ue.Err != nil {
		inner = ue.Err
	}
	msg := inner.Error()
	if errors.Is(inner, io.EOF) || errors.Is(inner, io.ErrUnexpectedEOF) || strings.Contains(msg, "connection reset") || strings.Contains(msg, "EOF") {
		return fmt.Errorf("%w (%s %s: Walmart closed the connection: %s). Stopped without retrying; this often means bot protection flagged the session. Check walmart.com in your browser, then re-run `walmart-pp-cli auth login --chrome`", ErrBotChallenge, method, path, msg)
	}
	return fmt.Errorf("%s %s: %s (not retried)", method, path, msg)
}

func botChallengeError(resp *http.Response, body []byte) error {
	status := 0
	if resp != nil {
		status = resp.StatusCode
	}
	return fmt.Errorf("%w (HTTP %d, %s). Stopped without retrying. Open walmart.com in your browser, clear any challenge, then re-run `walmart-pp-cli auth login --chrome`", ErrBotChallenge, status, challengeKind(body))
}

// challengeKind names the shape of a challenge response without echoing it.
func challengeKind(body []byte) string {
	s := strings.ToLower(string(body))
	switch {
	case strings.Contains(s, "px-captcha") || strings.Contains(s, "blockscript") || strings.Contains(s, "/blocked") || strings.Contains(s, "\"appid\""):
		return "PerimeterX block page"
	case len(strings.TrimSpace(s)) == 0:
		return "empty body"
	case strings.HasPrefix(strings.TrimSpace(s), "{"):
		return "JSON body without block marker"
	default:
		return "unrecognized body"
	}
}

// setWalmartTraceHeaders adds the per-call correlation headers the Walmart
// web app sends on every orchestra request. Values are random per call.
func setWalmartTraceHeaders(req *http.Request) {
	if req == nil || req.URL == nil || !strings.HasPrefix(req.URL.Path, "/orchestra/") {
		return
	}
	corr := randomUUIDv4()
	req.Header.Set("x-o-correlation-id", corr)
	req.Header.Set("wm_qos.correlation_id", corr)
	req.Header.Set("wm-client-traceid", randomHexBytes(16))
	req.Header.Set("traceparent", "00-"+randomHexBytes(16)+"-"+randomHexBytes(8)+"-00")
	req.Header.Set("sec-fetch-site", "same-origin")
	req.Header.Set("sec-fetch-mode", "cors")
	req.Header.Set("sec-fetch-dest", "empty")
}

func randomHexBytes(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func randomUUIDv4() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b)
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

// PATCH(walmart-browser-shape): Walmart's bot layer binds a session's cookies
// to the browser that minted them, so requests carry the same user agent,
// client hints and cache headers the captured Chrome sent (compared by name
// against the web capture). Session- and device-identifying headers the web
// app adds (baggage, device_profile_ref_id) are deliberately not forged.
const (
	defaultWalmartPlatformVersion = "usweb-1.317.0"
	walmartBrowserUA              = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/154.0.0.0 Safari/537.36"
	walmartSecCHUA                = `"Chromium";v="154", "Google Chrome";v="154", "Not A(Brand";v="99"`
)

// WalmartPlatformVersion is the web build id sent as x-o-platform-version.
// Override with WALMART_PLATFORM_VERSION after Walmart ships a new web build.
func WalmartPlatformVersion() string {
	if v := strings.TrimSpace(os.Getenv("WALMART_PLATFORM_VERSION")); v != "" {
		return v
	}
	return defaultWalmartPlatformVersion
}

func setWalmartBrowserShape(req *http.Request) {
	if req == nil || req.URL == nil {
		return
	}
	if strings.TrimSpace(os.Getenv("WALMART_USER_AGENT")) == "" {
		req.Header.Set("User-Agent", walmartBrowserUA)
		req.Header.Set("sec-ch-ua", walmartSecCHUA)
		req.Header.Set("sec-ch-ua-mobile", "?0")
		req.Header.Set("sec-ch-ua-platform", `"Linux"`)
	}
	if req.Header.Get("Accept-Language") == "" {
		req.Header.Set("Accept-Language", "en-US")
	}
	if !strings.HasPrefix(req.URL.Path, "/orchestra/") {
		return
	}
	req.Header.Set("x-o-platform-version", WalmartPlatformVersion())
	req.Header.Set("cache-control", "no-cache")
	req.Header.Set("pragma", "no-cache")
	req.Header.Set("priority", "u=1, i")
	req.Header.Set("downlink", "10")
	req.Header.Set("dpr", "1")
	req.Header.Set("device-memory", "16")
	req.Header.Set("sec-ch-dpr", "1")
	req.Header.Set("sec-ch-device-memory", "16")
	if strings.Contains(req.URL.Path, "/graphql/ItemById/") {
		// The product page request also carries these two; the empty
		// value is sent as-is, as the browser does.
		if req.Header.Get("ip-referer") == "" {
			req.Header.Set("ip-referer", "https://www.walmart.com/")
		}
		req.Header["Ip-Session-Traffic-Type"] = []string{""}
	}
}
