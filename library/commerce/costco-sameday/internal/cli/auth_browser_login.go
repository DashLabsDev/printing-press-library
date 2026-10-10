// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored one-step `auth login`: opens the installed Chrome/Chromium,
// headed, in a dedicated CLI-owned profile; the user signs in normally; the
// CLI reads only the Same-Day session cookies from THAT browser over CDP,
// saves them, and closes the window. It never reads the user's main Chrome
// cookie store or Chrome Safe Storage. The browser exists only for the
// duration of the login (no resident browser transport); normal commands
// stay plain HTTP.
//
// Detection surface is kept small: the CLI attaches to the browser target
// only (Target.getTargets, Storage.getCookies, Browser.close). It never
// attaches to the page, never enables Runtime, never injects script and
// never types or clicks; Chrome runs without --enable-automation.
//
// Preserved across regenerate via .printing-press-patches.

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/storage"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/mvanhorn/printing-press-library/library/commerce/costco-sameday/internal/cliutil"
)

const (
	sameDayOrigin         = "https://sameday.costco.com/"
	browserLoginProfile   = "login-browser"
	browserLoginEnvBinary = "COSTCO_SAMEDAY_BROWSER"
)

// Typed browser-login failures so callers and tests can map them to
// actionable messages.
var (
	errSigninRateLimited = errors.New("Costco is rate-limiting sign-in (HTTP 429 from signin.costco.com). Wait about 15-30 minutes before trying again; retrying sooner extends the block")
	errBrowserNotFound   = errors.New("no Chrome or Chromium found; install Google Chrome, pass --browser-path, or set " + browserLoginEnvBinary + ". Fallbacks: 'auth login --chrome' or 'pbpaste | auth login --cookies-file -'")
	errBrowserClosed     = errors.New("the login window was closed before sign-in finished; run 'auth login' again")
	errProfileInUse      = errors.New("the CLI login profile is already open in another Chrome window (a previous 'auth login' may still be running); close that window and retry")
)

// browserLoginTimeoutError reports a sign-in that did not finish in time.
type browserLoginTimeoutError struct {
	Timeout     time.Duration
	SawSession  bool
	AccountType string
}

func (e *browserLoginTimeoutError) Error() string {
	msg := fmt.Sprintf("timed out after %s waiting for sign-in", e.Timeout)
	if e.AccountType == accountTypeGuest {
		msg += "; the window still had a guest session. Use 'Sign in via Costco.com' in the Same-Day menu, then run 'auth login' again"
	}
	return msg
}

// cookieProbe classifies a candidate Cookie header (in memory only).
type cookieProbe func(ctx context.Context, cookieHeader string) accountTypeResult

type browserLoginOptions struct {
	StartURL     string
	CookieURL    string
	ProfileDir   string
	BrowserPath  string
	Headless     bool
	Timeout      time.Duration
	PollInterval time.Duration
	ExtraFlags   []string
	Probe        cookieProbe
	Out          io.Writer
}

type browserLoginResult struct {
	Cookies     []sessionCookie
	Expires     time.Time
	AccountType accountTypeResult
}

// browserLoginExtraFlags lets tests add flags (e.g. to trust a local TLS
// test server). Empty in production.
var browserLoginExtraFlags []string

func defaultBrowserProfileDir() (string, error) {
	dir, err := cliutil.DataDir()
	if err != nil || dir == "" {
		return "", fmt.Errorf("resolving data dir for the login profile: %w", err)
	}
	return filepath.Join(dir, browserLoginProfile), nil
}

// findBrowserBinary picks an installed Chrome/Chromium.
func findBrowserBinary(explicit string) (string, error) {
	for _, p := range []string{explicit, os.Getenv(browserLoginEnvBinary)} {
		if p = strings.TrimSpace(p); p != "" {
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
			if lp, err := exec.LookPath(p); err == nil {
				return lp, nil
			}
			return "", fmt.Errorf("browser %q not found", p)
		}
	}
	var candidates []string
	switch runtime.GOOS {
	case "darwin":
		candidates = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Google Chrome Beta.app/Contents/MacOS/Google Chrome Beta",
		}
	case "windows":
		for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)", "LocalAppData"} {
			if base := os.Getenv(env); base != "" {
				candidates = append(candidates, filepath.Join(base, "Google", "Chrome", "Application", "chrome.exe"))
			}
		}
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "chrome"} {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}
	return "", errBrowserNotFound
}

// rateLimitHosts are the host suffixes whose rate-limit pages abort login.
var rateLimitHosts = []string{"costco.com"}

// loginRateLimited spots a rate-limit page on a Costco host from the page
// URL and title (the CLI does not attach to the page to read status codes).
func loginRateLimited(rawURL, title string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	matched := false
	for _, h := range rateLimitHosts {
		if host == h || strings.HasSuffix(host, "."+h) {
			matched = true
		}
	}
	if !matched {
		return false
	}
	s := strings.ToLower(rawURL + " " + title)
	for _, marker := range []string{"429", "too many requests", "rate limit", "ratelimit", "too many attempts"} {
		if strings.Contains(s, marker) {
			return true
		}
	}
	return false
}

func hostMatchesCookie(cookieDomain, host string) bool {
	d := strings.TrimPrefix(strings.ToLower(cookieDomain), ".")
	host = strings.ToLower(host)
	return d == host || strings.HasSuffix(host, "."+d)
}

// pickSessionCookies keeps the session cookie and the optional companions
// for host, nothing else.
func pickSessionCookies(all []*network.Cookie, host string) ([]sessionCookie, *network.Cookie) {
	wanted := map[string]bool{sessionCookieName: true}
	for _, n := range optionalSessionCookies {
		wanted[n] = true
	}
	var out []sessionCookie
	var sid *network.Cookie
	seen := map[string]bool{}
	for _, c := range all {
		if c == nil || !wanted[c.Name] || seen[c.Name] || !hostMatchesCookie(c.Domain, host) {
			continue
		}
		seen[c.Name] = true
		if c.Name == sessionCookieName {
			sid = c
			out = append([]sessionCookie{{Name: c.Name, Value: c.Value}}, out...)
			continue
		}
		out = append(out, sessionCookie{Name: c.Name, Value: c.Value})
	}
	return out, sid
}

func cookieHeaderOf(cs []sessionCookie) string {
	parts := make([]string, 0, len(cs))
	for _, c := range cs {
		parts = append(parts, c.Name+"="+c.Value)
	}
	return strings.Join(parts, "; ")
}

// runBrowserLogin launches the browser, waits for a signed-in session,
// captures the session cookies and closes the browser.
func runBrowserLogin(ctx context.Context, opts browserLoginOptions) (browserLoginResult, error) {
	var res browserLoginResult
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	if opts.StartURL == "" {
		opts.StartURL = sameDayOrigin
	}
	if opts.CookieURL == "" {
		opts.CookieURL = opts.StartURL
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 10 * time.Minute
	}
	if opts.PollInterval <= 0 {
		opts.PollInterval = 2 * time.Second
	}
	cu, err := url.Parse(opts.CookieURL)
	if err != nil || cu.Hostname() == "" {
		return res, fmt.Errorf("invalid cookie URL %q", opts.CookieURL)
	}
	cookieHost := cu.Hostname()
	if opts.ProfileDir == "" {
		if opts.ProfileDir, err = defaultBrowserProfileDir(); err != nil {
			return res, err
		}
	}
	if err := os.MkdirAll(opts.ProfileDir, 0o700); err != nil {
		return res, fmt.Errorf("creating login profile dir: %w", err)
	}
	bin, err := findBrowserBinary(opts.BrowserPath)
	if err != nil {
		return res, err
	}

	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	portFile := filepath.Join(opts.ProfileDir, "DevToolsActivePort")
	_ = os.Remove(portFile)
	args := []string{
		"--user-data-dir=" + opts.ProfileDir,
		"--remote-debugging-port=0",
		"--remote-debugging-address=127.0.0.1",
		"--no-first-run",
		"--no-default-browser-check",
	}
	if opts.Headless {
		args = append(args, "--headless=new")
	}
	args = append(args, browserLoginExtraFlags...)
	args = append(args, opts.ExtraFlags...)
	args = append(args, opts.StartURL)
	proc := exec.Command(bin, args...) // #nosec G204 -- user-selected local browser binary.
	proc.Stdout, proc.Stderr = io.Discard, io.Discard
	if err := proc.Start(); err != nil {
		return res, fmt.Errorf("starting browser: %w", err)
	}
	exited := make(chan error, 1)
	go func() { exited <- proc.Wait() }()
	killed := false
	defer func() {
		if !killed {
			_ = proc.Process.Kill()
			<-exited
		}
	}()

	wsURL, err := waitDevToolsURL(ctx, portFile, exited)
	if err != nil {
		if errors.Is(err, errProfileInUse) || errors.Is(err, errBrowserClosed) {
			killed = true // waitDevToolsURL already reaped the process
		}
		return res, err
	}
	b, err := chromedp.NewBrowser(ctx, wsURL)
	if err != nil {
		return res, fmt.Errorf("connecting to the login browser: %w", err)
	}
	bctx := cdp.WithExecutor(ctx, b)
	closeBrowser := func() {
		cctx, ccancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer ccancel()
		_ = browser.Close().Do(cdp.WithExecutor(cctx, b))
		select {
		case <-exited:
			killed = true
		case <-time.After(5 * time.Second):
		}
	}

	fmt.Fprintln(opts.Out, "A Chrome window opened with a separate profile for this CLI.")
	fmt.Fprintln(opts.Out, "Sign in with 'Sign in via Costco.com' as you normally would. This window closes by itself once you're signed in.")

	var (
		sawSignin   bool
		initialSID  string
		lastProbed  string
		lastProbeAt time.Time
		lastAcct    accountTypeResult
		sawSession  bool
		nextNotice  = time.Now().Add(time.Minute)
	)
	ticker := time.NewTicker(opts.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			closeBrowser()
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return res, &browserLoginTimeoutError{Timeout: opts.Timeout, SawSession: sawSession, AccountType: lastAcct.Type}
			}
			return res, ctx.Err()
		case <-exited:
			killed = true
			return res, errBrowserClosed
		case <-ticker.C:
		}

		infos, err := target.GetTargets().Do(bctx)
		if err != nil {
			continue
		}
		pages, onSameDay := 0, false
		for _, ti := range infos {
			if ti == nil || ti.Type != "page" {
				continue
			}
			pages++
			if loginRateLimited(ti.URL, ti.Title) {
				closeBrowser()
				return res, errSigninRateLimited
			}
			u, perr := url.Parse(ti.URL)
			if perr != nil {
				continue
			}
			lp := strings.ToLower(u.Path)
			if (u.Hostname() != "" && u.Hostname() != cookieHost) || strings.Contains(lp, "signin") || strings.Contains(lp, "login") || strings.Contains(lp, "/sso/") {
				sawSignin = true
			}
			if u.Hostname() == cookieHost && !strings.Contains(lp, "/sso/") {
				onSameDay = true
			}
		}
		if pages == 0 {
			closeBrowser()
			return res, errBrowserClosed
		}
		all, err := storage.GetCookies().Do(bctx)
		if err != nil {
			continue
		}
		picked, sid := pickSessionCookies(all, cookieHost)
		if sid == nil || sid.Value == "" {
			continue
		}
		sawSession = true
		if initialSID == "" {
			// First session seen. Without a member verdict, completion needs
			// a different session cookie after a sign-in page (the SSO
			// callback issues a new one).
			initialSID = sid.Value
		}
		if !onSameDay {
			continue
		}
		acct := accountTypeResult{Type: accountTypeUnknown, Evidence: "not probed"}
		if opts.Probe != nil {
			if sid.Value != lastProbed || time.Since(lastProbeAt) > 15*time.Second {
				lastAcct = opts.Probe(ctx, cookieHeaderOf(picked))
				lastProbed, lastProbeAt = sid.Value, time.Now()
			}
			acct = lastAcct
		}
		done := acct.Type == accountTypeMember ||
			(acct.Type == accountTypeUnknown && sawSignin && sid.Value != initialSID)
		if !done {
			if time.Now().After(nextNotice) {
				fmt.Fprintln(opts.Out, "Still waiting for sign-in (the window has a guest session so far)...")
				nextNotice = time.Now().Add(time.Minute)
			}
			continue
		}
		res.Cookies = picked
		res.AccountType = acct
		if !sid.Session && sid.Expires > 0 {
			res.Expires = time.Unix(int64(sid.Expires), 0)
		}
		closeBrowser()
		return res, nil
	}
}

// waitDevToolsURL reads the browser websocket URL Chrome writes to
// DevToolsActivePort in the profile dir.
func waitDevToolsURL(ctx context.Context, portFile string, exited <-chan error) (string, error) {
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	start := time.Now()
	for {
		if data, err := os.ReadFile(portFile); err == nil { // #nosec G304 -- CLI-owned profile dir.
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			if len(lines) >= 2 && strings.TrimSpace(lines[0]) != "" {
				return "ws://127.0.0.1:" + strings.TrimSpace(lines[0]) + strings.TrimSpace(lines[1]), nil
			}
		}
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("browser did not open its DevTools endpoint: %w", ctx.Err())
		case <-exited:
			if time.Since(start) < 10*time.Second {
				return "", errProfileInUse
			}
			return "", errBrowserClosed
		case <-tick.C:
		}
	}
}
