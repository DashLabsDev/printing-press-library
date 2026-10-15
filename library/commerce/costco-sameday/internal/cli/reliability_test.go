// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.
// Tests for the reliability fixes: debounced login-window close with a
// redacted last URL, GraphQL failures exiting non-zero, `order start`
// auto-fill / service type / shop-vs-location check, and 5xx retry for
// non-charging calls only. Local stubs only; every value is fake.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// scriptedLogin runs runBrowserLogin against a fake pipe browser whose
// Target.getTargets answers come from pages(poll) and cookies from sid(poll).
func scriptedLogin(t *testing.T, pages func(poll int) string, sid func(poll int) string, grace time.Duration) (browserLoginResult, error, string) {
	return scriptedLoginKill(t, pages, sid, grace, 0)
}

// scriptedLoginKill ends the browser process at poll killAt (0: never).
func scriptedLoginKill(t *testing.T, pages func(poll int) string, sid func(poll int) string, grace time.Duration, killAt int) (browserLoginResult, error, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("uses a Unix stand-in process")
	}
	oldGrace, oldStart := browserClosedGrace, startLoginBrowserFn
	browserClosedGrace = grace
	t.Cleanup(func() { browserClosedGrace, startLoginBrowserFn = oldGrace, oldStart })
	var mu sync.Mutex
	poll := 0
	startLoginBrowserFn = func(_ context.Context, _ string, _ []string) (*exec.Cmd, cdpCaller, <-chan error, error) {
		proc := exec.Command("sleep", "60")
		if err := proc.Start(); err != nil {
			return nil, nil, nil, err
		}
		exited := make(chan error, 1)
		go func() { exited <- proc.Wait() }()
		p, _ := fakePipeBrowser(t, func(m string) (string, string) {
			mu.Lock()
			defer mu.Unlock()
			switch m {
			case "Browser.getVersion":
				return `{"product":"FakeChrome/1"}`, ""
			case "Target.getTargets":
				poll++
				if killAt > 0 && poll == killAt {
					_ = proc.Process.Kill()
				}
				return `{"targetInfos":[` + pages(poll) + `]}`, ""
			case "Storage.getCookies":
				return `{"cookies":[{"name":"` + sessionCookieName + `","value":"` + sid(poll) + `","domain":"sameday.fake.test","session":true}]}`, ""
			case "Browser.close":
				_ = proc.Process.Kill()
				return `{}`, ""
			}
			return "", "unexpected " + m
		})
		return proc, p, exited, nil
	}
	var out strings.Builder
	res, err := runBrowserLogin(context.Background(), browserLoginOptions{
		StartURL: "https://sameday.fake.test/", CookieURL: "https://sameday.fake.test/",
		ProfileDir: filepath.Join(t.TempDir(), browserLoginProfile), BrowserPath: "/bin/sh",
		Timeout: 20 * time.Second, PollInterval: 50 * time.Millisecond, Out: &out,
	})
	return res, err, out.String()
}

const (
	fakeSSOPage  = `{"type":"page","url":"https://signin.fake.test/sso/callback?code=FAKE-SECRET-CODE&state=FAKE-STATE#frag","title":"Sign in"}`
	fakeHomePage = `{"type":"page","url":"https://sameday.fake.test/store/home?utm=FAKE","title":"Home"}`
)

func TestBrowserLoginSurvivesBriefNoPageGap(t *testing.T) {
	res, err, _ := scriptedLogin(t,
		func(p int) string {
			switch {
			case p <= 3:
				return fakeSSOPage
			case p <= 8: // ~250ms with no page target mid-redirect
				return ""
			default:
				return fakeHomePage
			}
		},
		func(p int) string {
			if p < 9 {
				return "FAKE-GUEST-SID"
			}
			return "FAKE-MEMBER-SID"
		}, 2*time.Second)
	if err != nil {
		t.Fatalf("a brief no-page gap ended the login: %v", err)
	}
	if len(res.Cookies) == 0 || res.Cookies[0].Value != "FAKE-MEMBER-SID" {
		t.Fatalf("cookies = %+v", res.Cookies)
	}
}

func TestBrowserLoginClosedAfterGraceNamesRedactedLastURL(t *testing.T) {
	start := time.Now()
	_, err, out := scriptedLogin(t,
		func(p int) string {
			if p <= 2 {
				return fakeSSOPage
			}
			return ""
		},
		func(int) string { return "FAKE-GUEST-SID" }, 400*time.Millisecond)
	if !errors.Is(err, errBrowserClosed) {
		t.Fatalf("err = %v, want closed", err)
	}
	if el := time.Since(start); el < 400*time.Millisecond {
		t.Fatalf("closed after %s, before the grace period", el)
	}
	for _, s := range []string{err.Error(), out} {
		if !strings.Contains(s, "https://signin.fake.test/sso/callback") {
			t.Fatalf("last URL missing: %q", s)
		}
		for _, leak := range []string{"FAKE-SECRET-CODE", "FAKE-STATE", "code=", "#frag"} {
			if strings.Contains(s, leak) {
				t.Fatalf("query/fragment leaked (%s): %q", leak, s)
			}
		}
	}
}

func TestRedactedPageURL(t *testing.T) {
	cases := map[string]string{
		"https://fakeuser:fakepw" + "@signin.fake.test/a/b?x=1#y": "https://signin.fake.test/a/b",
		"about:blank":       "",
		"chrome://newtab/":  "chrome://newtab/",
		"not a url\x7f":     "",
		"https://fake.test": "https://fake.test",
	}
	for in, want := range cases {
		if got := redactedPageURL(in); got != want {
			t.Fatalf("redactedPageURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDetectGraphQLFailureShapes(t *testing.T) {
	fail := map[string]string{
		"errors only":     `{"errors":[{"message":"Variable $userLocation invalid (fake)"}],"data":null}`,
		"errors and data": `{"errors":[{"message":"partial (fake)"}],"data":{"x":{"id":"1"}}}`,
		"redirect error":  `{"data":{"initializeCheckout":{"__typename":"CheckoutCheckoutErrorsCheckoutRedirectError","error":"empty_cart","message":"No items are available in your cart"}}}`,
		"error response":  `{"data":{"updateCheckout":{"__typename":"CheckoutCheckoutErrorResponse","errors":[{"__typename":"CheckoutCheckoutErrorsServiceOptionError","message":"fake"}]}}}`,
	}
	for name, body := range fail {
		r := detectPartialFailure([]byte(body))
		if r == nil {
			t.Fatalf("%s: not flagged", name)
		}
		if name == "redirect error" && (!strings.Contains(r.Message, "empty_cart") || !strings.Contains(r.Message, "CheckoutRedirectError")) {
			t.Fatalf("redirect message = %q", r.Message)
		}
	}
	ok := []string{
		`{"data":{"initializeCheckout":{"__typename":"CheckoutInitializeCheckoutSuccessResponse"}}}`,
		`{"data":{"x":null}}`,
		`{"errors":[],"data":{"x":{}}}`,
		`{"__pp_verify_synthetic__":true,"errors":[{"message":"x"}]}`,
		`[1,2]`,
	}
	for _, body := range ok {
		if r := detectPartialFailure([]byte(body)); r != nil {
			t.Fatalf("%s flagged: %+v", body, r)
		}
	}
}

func TestGeneratedCommandGraphQLErrorsExitNonZero(t *testing.T) {
	routedSameDay(t, withReplies(map[string]string{
		initializeCheckoutOp: `{"errors":[{"message":"Variable $userLocation of type CheckoutInputsUserLocation! was provided invalid value (fake)"}],"data":null}`,
	}))
	out, _, err := runRootSplit(t, "checkout", "initializecheckout", "--cart-id", "cart-fake", "--shop-id", "shop-fake", "--service-type", "delivery", "--json")
	if err == nil || ExitCode(err) == 0 {
		t.Fatalf("exit zero on a GraphQL errors body:\n%s", out)
	}
	var env map[string]any
	if json.Unmarshal([]byte(out), &env) != nil || env["success"] != false {
		t.Fatalf("envelope success not false:\n%s", out)
	}
}

func TestRawInitializeCheckoutUppercasesServiceType(t *testing.T) {
	rec := routedSameDay(t, withReplies(map[string]string{
		initializeCheckoutOp: `{"data":{"initializeCheckout":{"__typename":"CheckoutInitializeCheckoutSuccessResponse","checkoutData":{"checkoutSessionId":"sess-fake"}}}}`,
	}))
	if _, _, err := runRootSplit(t, "checkout", "initializecheckout", "--cart-id", "cart-fake", "--shop-id", "shop-fake", "--service-type", "delivery", "--json"); err != nil {
		t.Fatal(err)
	}
	if got := rec.vars[initializeCheckoutOp]["serviceType"]; got != "DELIVERY" {
		t.Fatalf("serviceType = %v", got)
	}
}

const fakeStartOK = `{"data":{"initializeCheckout":{"__typename":"CheckoutInitializeCheckoutSuccessResponse","checkoutData":{"checkoutSessionId":"sess-start-fake","cartId":"cart-1","serviceType":"delivery"}}}}`

func TestOrderStartFillsFromSessionContext(t *testing.T) {
	rec := routedSameDay(t, withReplies(map[string]string{
		"ShopCollectionScoped": `{"data":{"shopCollection":{"shops":[` +
			`{"id":"shop-1","serviceType":"delivery","retailerLocationId":"loc-1","retailerId":"ret-fake"},` +
			`{"id":"shop-p","serviceType":"pickup","retailerLocationId":"loc-1","retailerId":"ret-fake"}]}}}`,
		initializeCheckoutOp: fakeStartOK,
	}))
	out, _, err := runRootSplit(t, "order", "start", "--zip", "98027", "--service-type", "Delivery", "--json")
	if err != nil {
		t.Fatalf("order start: %v\n%s", err, out)
	}
	if rec.methods[initializeCheckoutOp] != "POST" || rec.query[initializeCheckoutOp] != "" {
		t.Fatalf("method=%s query=%q", rec.methods[initializeCheckoutOp], rec.query[initializeCheckoutOp])
	}
	v := rec.vars[initializeCheckoutOp]
	loc, _ := v["userLocation"].(map[string]any)
	if v["cartId"] != "cart-1" || v["shopId"] != "shop-1" || v["serviceType"] != "DELIVERY" || loc["postalCode"] != "98027" {
		t.Fatalf("vars = %v", v)
	}
	if !strings.Contains(out, `"checkout_session_id": "sess-start-fake"`) || !strings.Contains(out, `"started": true`) {
		t.Fatalf("output:\n%s", out)
	}
	if _, called := rec.methods["FinalizeCheckout"]; called {
		t.Fatal("order start must never finalize")
	}
	// pickup picks the pickup shop
	if _, _, err := runRootSplit(t, "order", "start", "--zip", "98027", "--service-type", "pickup", "--json"); err != nil {
		t.Fatal(err)
	}
	if v := rec.vars[initializeCheckoutOp]; v["shopId"] != "shop-p" || v["serviceType"] != "PICKUP" {
		t.Fatalf("pickup vars = %v", v)
	}
}

func TestOrderStartRefusesStoreLocationAsShop(t *testing.T) {
	rec := routedSameDay(t, withReplies(map[string]string{initializeCheckoutOp: fakeStartOK}))
	_, _, err := runRootSplit(t, "order", "start", "--zip", "98027", "--shop-id", "loc-1", "--json")
	if err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), "store location id") || !strings.Contains(err.Error(), "--shop-id shop-1") {
		t.Fatalf("err = %v", err)
	}
	if _, sent := rec.methods[initializeCheckoutOp]; sent {
		t.Fatal("sent InitializeCheckout with a location id as the shop")
	}
}

func TestOrderStartBadServiceTypeAndLocation(t *testing.T) {
	routedSameDay(t, sessionReplies)
	if _, _, err := runRootSplit(t, "order", "start", "--service-type", "drone"); err == nil || ExitCode(err) != 2 {
		t.Fatalf("bad service type err = %v", err)
	}
	_, _, err := runRootSplit(t, "order", "start", "--user-location", "98027")
	if err == nil || !strings.Contains(err.Error(), "postalCode") {
		t.Fatalf("bad location err = %v", err)
	}
	out, _, _ := runRootSplit(t, "order", "start", "--help")
	for _, want := range []string{"retailer location id", "\"coordinates\"", "delivery (default) or pickup"} {
		if !strings.Contains(out, want) {
			t.Fatalf("help missing %q", want)
		}
	}
}

func TestOrderStartEmptyCartRedirectExplainsShopID(t *testing.T) {
	routedSameDay(t, withReplies(map[string]string{
		initializeCheckoutOp: `{"data":{"initializeCheckout":{"__typename":"CheckoutCheckoutErrorsCheckoutRedirectError","error":"empty_cart","message":"No items are available in your cart"}}}`,
	}))
	out, _, err := runRootSplit(t, "order", "start", "--zip", "98027", "--json")
	if err == nil || ExitCode(err) == 0 || !strings.Contains(err.Error(), "retailer location id") {
		t.Fatalf("err = %v\n%s", err, out)
	}
	if !strings.Contains(out, `"started": false`) {
		t.Fatalf("output:\n%s", out)
	}
}

func TestOrderStartDryRunSendsNothing(t *testing.T) {
	rec := routedSameDay(t, withReplies(map[string]string{initializeCheckoutOp: fakeStartOK}))
	out, _, err := runRootSplit(t, "order", "start", "--dry-run", "--json")
	if err != nil || !strings.Contains(out, "dry_run") {
		t.Fatalf("dry-run: %v\n%s", err, out)
	}
	if _, sent := rec.methods[initializeCheckoutOp]; sent {
		t.Fatal("dry-run sent InitializeCheckout")
	}
}

// flakyServer answers 503 to the first n requests for op, then delegates.
func flakyServer(t *testing.T, op string, n int32) *int32 {
	t.Helper()
	upstream, _ := url.Parse(os.Getenv("COSTCO_SAMEDAY_BASE_URL"))
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		name := r.URL.Query().Get("operationName")
		var b map[string]any
		if json.Unmarshal(raw, &b) == nil {
			if s, _ := b["operationName"].(string); s != "" {
				name = s
			}
		}
		if name == op && atomic.AddInt32(&hits, 1) <= n {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, `{"error":"unavailable (fake)"}`)
			return
		}
		req, _ := http.NewRequest(r.Method, upstream.String()+r.URL.RequestURI(), strings.NewReader(string(raw)))
		req.Header = r.Header.Clone()
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("COSTCO_SAMEDAY_BASE_URL", srv.URL)
	return &hits
}

func TestOrderStartRetries503(t *testing.T) {
	routedSameDay(t, withReplies(map[string]string{initializeCheckoutOp: fakeStartOK}))
	hits := flakyServer(t, initializeCheckoutOp, 1)
	out, _, err := runRootSplit(t, "order", "start", "--zip", "98027", "--json")
	if err != nil || !strings.Contains(out, "sess-start-fake") {
		t.Fatalf("order start after a 503: %v\n%s", err, out)
	}
	if got := atomic.LoadInt32(hits); got != 2 {
		t.Fatalf("InitializeCheckout attempts = %d, want 2", got)
	}
}

func TestWritesAreNotRetriedOn503(t *testing.T) {
	routedSameDay(t, withReplies(map[string]string{"UpdateCheckout": `{"data":{"updateCheckout":{"__typename":"X"}}}`}))
	hits := flakyServer(t, "UpdateCheckout", 1)
	if _, _, err := runRootSplit(t, "order", "tip", "--checkout-session-id", "sess-fake", "--tip-input-token", "tip-fake", "--yes", "--json"); err == nil {
		t.Fatal("tip succeeded through a 503")
	}
	if got := atomic.LoadInt32(hits); got != 1 {
		t.Fatalf("UpdateCheckout attempts = %d, want 1 (no retry for writes)", got)
	}
}

func TestOrderTipGraphQLErrorExitsNonZero(t *testing.T) {
	routedSameDay(t, withReplies(map[string]string{
		"UpdateCheckout": `{"data":{"updateCheckout":{"__typename":"CheckoutCheckoutErrorResponse","errors":[{"message":"tip rejected (fake)"}]}}}`,
	}))
	out, _, err := runRootSplit(t, "order", "tip", "--checkout-session-id", "sess-fake", "--tip-input-token", "tip-fake", "--yes", "--json")
	if err == nil || !strings.Contains(err.Error(), "tip rejected (fake)") || !strings.Contains(out, `"updated": false`) {
		t.Fatalf("err=%v\n%s", err, out)
	}
}

func TestBrowserLoginProcessExitNamesLastPage(t *testing.T) {
	_, err, out := scriptedLoginKill(t,
		func(int) string { return fakeSSOPage },
		func(int) string { return "FAKE-GUEST-SID" }, 10*time.Second, 3)
	if !errors.Is(err, errBrowserClosed) {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(err.Error(), "https://signin.fake.test/sso/callback") || !strings.Contains(out, "last page seen") || strings.Contains(err.Error()+out, "FAKE-SECRET-CODE") {
		t.Fatalf("err=%v out=%q", err, out)
	}
}

func TestOrderStartUserLocationZipDrivesShopAndCart(t *testing.T) {
	rec := routedSameDay(t, withReplies(map[string]string{initializeCheckoutOp: fakeStartOK}))
	out, _, err := runRootSplit(t, "order", "start", "--user-location", `{"postalCode":"10001"}`, "--json")
	if err != nil {
		t.Fatalf("order start: %v\n%s", err, out)
	}
	if got := rec.vars["ShopCollectionScoped"]["postalCode"]; got != "10001" {
		t.Fatalf("shop lookup ZIP = %v", got)
	}
	loc, _ := rec.vars[initializeCheckoutOp]["userLocation"].(map[string]any)
	if loc["postalCode"] != "10001" {
		t.Fatalf("userLocation = %v", loc)
	}
	if _, has := loc["coordinates"]; has {
		t.Fatalf("coordinates from another ZIP were sent: %v", loc)
	}
}

func TestOrderStartFullySpecifiedNeedsNoGeolocation(t *testing.T) {
	replies := withReplies(map[string]string{initializeCheckoutOp: fakeStartOK})
	delete(replies, "Geolocation")
	rec := routedSameDay(t, replies)
	out, _, err := runRootSplit(t, "order", "start", "--shop-id", "shop-1", "--cart-id", "cart-fake",
		"--user-location", `{"postalCode":"10001","coordinates":{"latitude":1.5,"longitude":2.5}}`, "--json")
	if err != nil {
		t.Fatalf("order start: %v\n%s", err, out)
	}
	if _, called := rec.methods["Geolocation"]; called {
		t.Fatal("Geolocation called although location, shop and cart were given")
	}
	v := rec.vars[initializeCheckoutOp]
	loc, _ := v["userLocation"].(map[string]any)
	if v["cartId"] != "cart-fake" || v["shopId"] != "shop-1" || loc["postalCode"] != "10001" || loc["coordinates"] == nil {
		t.Fatalf("vars = %v", v)
	}
}

func TestOrderStartUserLocationZipIsNotSavedAsDefault(t *testing.T) {
	rec := routedSameDay(t, withReplies(map[string]string{initializeCheckoutOp: fakeStartOK}))
	if _, _, err := runRootSplit(t, "order", "start", "--user-location", `{"postalCode":"10001"}`, "--json"); err != nil {
		t.Fatal(err)
	}
	// A later command without a ZIP must resolve from the session default, not 10001.
	out, _, err := runRootSplit(t, "order", "start", "--json")
	if err != nil {
		t.Fatalf("second start: %v\n%s", err, out)
	}
	if got := rec.vars["ShopCollectionScoped"]["postalCode"]; got == "10001" {
		t.Fatalf("--user-location ZIP was remembered as the default")
	}
	loc, _ := rec.vars[initializeCheckoutOp]["userLocation"].(map[string]any)
	if loc["postalCode"] == "10001" {
		t.Fatalf("second request used the earlier --user-location ZIP: %v", loc)
	}
}
