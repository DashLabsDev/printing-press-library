// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.
// Browser-login tests against a local fake sign-in page (fake cookie values
// only). They drive the installed Chrome headless and skip when none exists.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func requireBrowser(t *testing.T) {
	t.Helper()
	if os.Getenv("COSTCO_SAMEDAY_SKIP_BROWSER_TESTS") != "" {
		t.Skip("browser tests disabled")
	}
	if _, err := findBrowserBinary(""); err != nil {
		t.Skip("no Chrome/Chromium installed")
	}
	old := browserLoginExtraFlags
	browserLoginExtraFlags = []string{"--ignore-certificate-errors", "--no-sandbox", "--disable-gpu"}
	oldHosts := rateLimitHosts
	rateLimitHosts = append([]string{"127.0.0.1"}, rateLimitHosts...)
	t.Cleanup(func() { browserLoginExtraFlags, rateLimitHosts = old, oldHosts })
}

// fakeSignin serves a guest landing page that walks to /signin, then a
// callback that sets the member session cookie and returns home.
func fakeSignin(t *testing.T, rateLimit bool) *httptest.Server {
	t.Helper()
	page := func(w http.ResponseWriter, title, next string) {
		w.Header().Set("Content-Type", "text/html")
		refresh := ""
		if next != "" {
			refresh = `<meta http-equiv="refresh" content="1;url=` + next + `">`
		}
		fmt.Fprintf(w, "<html><head><title>%s</title>%s</head><body>%s</body></html>", title, refresh, title)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if _, err := r.Cookie(sessionCookieName); err != nil {
			http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: "FAKE-GUEST-SID", Path: "/", Secure: true, HttpOnly: true})
			page(w, "Fake Same-Day (guest)", "/signin")
			return
		}
		c, _ := r.Cookie(sessionCookieName)
		if c.Value == "FAKE-GUEST-SID" {
			page(w, "Fake Same-Day (guest)", "/signin")
			return
		}
		page(w, "Fake Same-Day (member)", "")
	})
	mux.HandleFunc("/signin", func(w http.ResponseWriter, r *http.Request) {
		if rateLimit {
			w.WriteHeader(http.StatusTooManyRequests)
			page(w, "429 Too Many Requests", "")
			return
		}
		page(w, "Fake Costco sign in", "/callback")
	})
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		exp := time.Now().Add(48 * time.Hour)
		http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: "FAKE-MEMBER-SID", Path: "/", Secure: true, HttpOnly: true, Expires: exp})
		http.SetCookie(w, &http.Cookie{Name: "ahoy_visit", Value: "FAKE-AHOY", Path: "/", Secure: true})
		http.SetCookie(w, &http.Cookie{Name: "unrelated_tracker", Value: "FAKE-TRACKER", Path: "/", Secure: true})
		http.Redirect(w, r, "/", http.StatusFound)
	})
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func fakeProbe(_ context.Context, header string) accountTypeResult {
	if strings.Contains(header, "FAKE-MEMBER-SID") {
		return accountTypeResult{Type: accountTypeMember, Source: "fake"}
	}
	return accountTypeResult{Type: accountTypeGuest, Source: "fake"}
}

func browserOpts(t *testing.T, srv *httptest.Server, probe cookieProbe) browserLoginOptions {
	return browserLoginOptions{
		StartURL:     srv.URL + "/",
		CookieURL:    srv.URL + "/",
		ProfileDir:   filepath.Join(t.TempDir(), browserLoginProfile),
		Headless:     true,
		Timeout:      60 * time.Second,
		PollInterval: 300 * time.Millisecond,
		Probe:        probe,
		Out:          io.Discard,
	}
}

func TestBrowserLoginCapturesOnlySessionCookiesAndCloses(t *testing.T) {
	requireBrowser(t)
	srv := fakeSignin(t, false)
	opts := browserOpts(t, srv, fakeProbe)
	res, err := runBrowserLogin(context.Background(), opts)
	if err != nil {
		t.Fatalf("browser login: %v", err)
	}
	got := map[string]string{}
	for _, c := range res.Cookies {
		got[c.Name] = c.Value
	}
	if got[sessionCookieName] != "FAKE-MEMBER-SID" || got["ahoy_visit"] != "FAKE-AHOY" {
		t.Fatalf("captured = %v", got)
	}
	if _, ok := got["unrelated_tracker"]; ok {
		t.Fatal("captured a cookie the CLI does not need")
	}
	if res.AccountType.Type != accountTypeMember {
		t.Fatalf("account = %+v", res.AccountType)
	}
	if d := time.Until(res.Expires); d < 47*time.Hour || d > 49*time.Hour {
		t.Fatalf("expiry = %v", res.Expires)
	}
	// Pipe transport: Chrome never opened a DevTools TCP port, so it wrote
	// no DevToolsActivePort file into the profile.
	if browserLoginTransport == "pipe" {
		if _, err := os.Stat(filepath.Join(opts.ProfileDir, "DevToolsActivePort")); err == nil {
			t.Fatal("login browser opened a DevTools TCP port; want --remote-debugging-pipe only")
		}
	}
	// The profile is private and marked as CLI-owned.
	if fi, err := os.Stat(opts.ProfileDir); err != nil || (runtime.GOOS != "windows" && fi.Mode().Perm() != 0o700) {
		t.Fatalf("profile dir perms = %v (%v), want 0700", fi.Mode().Perm(), err)
	}
	if !hasLoginProfileMarker(opts.ProfileDir) {
		t.Fatal("profile dir created without the CLI marker")
	}
}

func TestBrowserLoginWithoutProbeUsesSigninAndNewSession(t *testing.T) {
	requireBrowser(t)
	srv := fakeSignin(t, false)
	res, err := runBrowserLogin(context.Background(), browserOpts(t, srv, nil))
	if err != nil {
		t.Fatalf("browser login: %v", err)
	}
	if len(res.Cookies) == 0 || res.Cookies[0].Value != "FAKE-MEMBER-SID" {
		t.Fatalf("captured = %+v", res.Cookies)
	}
}

func TestBrowserLoginStopsOnSigninRateLimit(t *testing.T) {
	requireBrowser(t)
	srv := fakeSignin(t, true)
	_, err := runBrowserLogin(context.Background(), browserOpts(t, srv, fakeProbe))
	if !errors.Is(err, errSigninRateLimited) {
		t.Fatalf("err = %v, want rate-limit", err)
	}
	if msg := browserLoginHint(err).Error(); !strings.Contains(msg, "15-30 minutes") {
		t.Fatalf("hint = %q", msg)
	}
}

func TestBrowserLoginTimesOutAsGuest(t *testing.T) {
	requireBrowser(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: "FAKE-GUEST-SID", Path: "/", Secure: true})
		fmt.Fprint(w, "<html><title>guest</title></html>")
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	opts := browserOpts(t, srv, fakeProbe)
	opts.Timeout = 4 * time.Second
	_, err := runBrowserLogin(context.Background(), opts)
	var te *browserLoginTimeoutError
	if !errors.As(err, &te) || te.AccountType != accountTypeGuest || !strings.Contains(err.Error(), "Sign in via Costco.com") {
		t.Fatalf("err = %v", err)
	}
}

func TestAuthLoginBrowserEndToEnd(t *testing.T) {
	requireBrowser(t)
	// Probe queries go unanswered (account type unknown), so login completes
	// on the signin round trip plus a new session cookie, like a real SSO.
	routedSameDay(t, map[string]string{})
	srv := fakeSignin(t, false)
	profile := filepath.Join(t.TempDir(), browserLoginProfile)
	out, errOut, err := runRootSplit(t, "auth", "login", "--login-url", srv.URL+"/", "--login-headless", "--login-profile-dir", profile, "--timeout", "60s")
	if err != nil {
		t.Fatalf("auth login: %v\nstdout=%s\nstderr=%s", err, out, errOut)
	}
	for _, v := range []string{"FAKE-MEMBER-SID", "FAKE-AHOY", "FAKE-TRACKER"} {
		if strings.Contains(out+errOut, v) {
			t.Fatalf("login output leaks %s", v)
		}
	}
	if c, _ := m0(t); c != 2 {
		t.Fatalf("saved cookie count = %d, want session + ahoy_visit\n%s\n%s", c, out, errOut)
	}
	status, _, err := runRootSplit(t, "auth", "status", "--json")
	if err != nil {
		t.Fatalf("status: %v\n%s", err, status)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(status), &m); err != nil {
		t.Fatalf("status not JSON: %s", status)
	}
	if m["account_type"] != "unknown" || !strings.Contains(fmt.Sprint(m["session_expires"]), "(in ") {
		t.Fatalf("status = %v", m)
	}
}

// m0 returns the saved cookie count from auth status --no-probe.
func m0(t *testing.T) (int, error) {
	t.Helper()
	out, _, err := runRootSplit(t, "auth", "status", "--no-probe", "--json")
	if err != nil {
		return 0, err
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		return 0, err
	}
	c, _ := m["cookies"].(map[string]any)
	n, _ := c["cookie_count"].(float64)
	return int(n), nil
}
