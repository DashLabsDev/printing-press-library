// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.
// Tests for the review fixes: DevTools pipe transport, marker-gated
// --reset-profile, slots pick failure exit, ambiguous saved address,
// explicit --cart-id subtotal, --show-tokens table output and slots etas
// failure exit. Local stubs only; every value is fake.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// fakePipeBrowser answers CDP commands on the browser end of two pipes.
func fakePipeBrowser(t *testing.T, handle func(method string) (result string, errMsg string)) (*pipeCDP, func()) {
	t.Helper()
	cmdR, cmdW := io.Pipe()   // CLI -> browser
	respR, respW := io.Pipe() // browser -> CLI
	go func() {
		buf := make([]byte, 0, 4096)
		one := make([]byte, 1)
		for {
			if _, err := cmdR.Read(one); err != nil {
				return
			}
			if one[0] != 0 {
				buf = append(buf, one[0])
				continue
			}
			var req struct {
				ID     int64  `json:"id"`
				Method string `json:"method"`
			}
			_ = json.Unmarshal(buf, &req)
			buf = buf[:0]
			// An event first: the client must skip it.
			_, _ = respW.Write([]byte(`{"method":"Target.targetCreated","params":{}}` + "\x00"))
			res, emsg := handle(req.Method)
			var out []byte
			if emsg != "" {
				out, _ = json.Marshal(map[string]any{"id": req.ID, "error": map[string]any{"code": -32000, "message": emsg}})
			} else {
				out = []byte(`{"id":` + jsonInt(req.ID) + `,"result":` + res + `}`)
			}
			if _, err := respW.Write(append(out, 0)); err != nil {
				return
			}
		}
	}()
	p := newPipeCDP(respR, cmdW)
	return p, func() { _ = respW.Close(); _ = cmdR.Close() }
}

func jsonInt(n int64) string { b, _ := json.Marshal(n); return string(b) }

func TestPipeCDPCallsAndDecodes(t *testing.T) {
	p, stop := fakePipeBrowser(t, func(m string) (string, string) {
		switch m {
		case "Target.getTargets":
			return `{"targetInfos":[{"type":"page","url":"https://example.test/","title":"Fake"}]}`, ""
		case "Storage.getCookies":
			return `{"cookies":[{"name":"` + sessionCookieName + `","value":"FAKE-SID","domain":"example.test","expires":1893456000,"session":false}]}`, ""
		}
		return "", "unknown method " + m
	})
	defer stop()
	defer p.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var targets struct {
		TargetInfos []cdpTargetInfo `json:"targetInfos"`
	}
	if err := p.Call(ctx, "Target.getTargets", nil, &targets); err != nil || len(targets.TargetInfos) != 1 || targets.TargetInfos[0].Type != "page" {
		t.Fatalf("getTargets = %+v, %v", targets, err)
	}
	var jar struct {
		Cookies []cdpCookie `json:"cookies"`
	}
	if err := p.Call(ctx, "Storage.getCookies", nil, &jar); err != nil {
		t.Fatal(err)
	}
	picked, sid := pickSessionCookies(jar.Cookies, "example.test")
	if sid == nil || sid.Value != "FAKE-SID" || len(picked) != 1 {
		t.Fatalf("picked = %+v sid=%+v", picked, sid)
	}
	var rerr *cdpRemoteError
	if err := p.Call(ctx, "Runtime.enable", nil, nil); !errors.As(err, &rerr) {
		t.Fatalf("remote error = %v", err)
	}
	stop()
	if err := p.Call(ctx, "Target.getTargets", nil, nil); !errors.Is(err, errCDPClosed) {
		t.Fatalf("after close err = %v", err)
	}
}

func TestLoginTransportIsPipeOffWindows(t *testing.T) {
	want := "pipe"
	if runtime.GOOS == "windows" {
		want = "port"
	}
	if browserLoginTransport != want {
		t.Fatalf("transport = %s, want %s", browserLoginTransport, want)
	}
}

func TestPrepareLoginProfileDirPrivateAndMarked(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", browserLoginProfile)
	if err := prepareLoginProfileDir(dir); err != nil {
		t.Fatal(err)
	}
	fi, _ := os.Stat(dir)
	if runtime.GOOS != "windows" && fi.Mode().Perm() != 0o700 {
		t.Fatalf("perm = %v", fi.Mode().Perm())
	}
	if !hasLoginProfileMarker(dir) {
		t.Fatal("new profile dir not marked")
	}
	// A loosened marked dir is tightened again.
	_ = os.Chmod(dir, 0o755)
	if err := prepareLoginProfileDir(dir); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(dir); runtime.GOOS != "windows" && fi.Mode().Perm() != 0o700 {
		t.Fatalf("perm after re-prepare = %v", fi.Mode().Perm())
	}
	// An existing non-empty unmarked dir is used but never marked.
	other := t.TempDir()
	_ = os.WriteFile(filepath.Join(other, "keep.txt"), []byte("fake"), 0o600)
	if err := prepareLoginProfileDir(other); err != nil {
		t.Fatal(err)
	}
	if hasLoginProfileMarker(other) {
		t.Fatal("marked a pre-existing non-empty directory")
	}
}

func TestResetLoginProfileOnlyDeletesMarkedDir(t *testing.T) {
	base := t.TempDir()
	// Parent of a profile, with unrelated files: refused.
	parent := filepath.Join(base, "parent")
	_ = os.MkdirAll(filepath.Join(parent, "docs"), 0o700)
	_ = os.WriteFile(filepath.Join(parent, "docs", "fake.txt"), []byte("fake"), 0o600)
	if err := resetLoginProfileDir(parent); err == nil || !strings.Contains(err.Error(), "not created by 'auth login'") {
		t.Fatalf("reset unmarked dir err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(parent, "docs", "fake.txt")); err != nil {
		t.Fatal("unmarked dir was deleted")
	}
	// A marker with the wrong content is not enough.
	_ = os.WriteFile(filepath.Join(parent, loginProfileMarker), []byte("forged"), 0o600)
	if err := resetLoginProfileDir(parent); err == nil {
		t.Fatal("reset accepted a forged marker")
	}
	// A symlink to a marked profile is refused.
	prof := filepath.Join(base, "prof")
	if err := prepareLoginProfileDir(prof); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if os.Symlink(prof, link) == nil {
		if err := resetLoginProfileDir(link); err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("symlink reset err = %v", err)
		}
	}
	// HOME is refused even if marked.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	_ = writeLoginProfileMarker(home)
	if err := resetLoginProfileDir(home); err == nil {
		t.Fatal("reset accepted HOME")
	}
	// The CLI-created, marked profile is deleted; a missing one is a no-op.
	if err := resetLoginProfileDir(prof); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(prof); !os.IsNotExist(err) {
		t.Fatal("marked profile not deleted")
	}
	if err := resetLoginProfileDir(prof); err != nil {
		t.Fatalf("missing dir err = %v", err)
	}
}

func TestAuthLoginResetProfileRefusesCustomParent(t *testing.T) {
	routedSameDay(t, map[string]string{})
	parent := t.TempDir()
	_ = os.WriteFile(filepath.Join(parent, "fake-keep.txt"), []byte("fake"), 0o600)
	_, _, err := runRootSplit(t, "auth", "login", "--login-url", "https://127.0.0.1:1/", "--login-headless", "--login-profile-dir", parent, "--reset-profile", "--timeout", "2s")
	if err == nil || !strings.Contains(err.Error(), "refusing to reset") {
		t.Fatalf("err = %v", err)
	}
	if _, serr := os.Stat(filepath.Join(parent, "fake-keep.txt")); serr != nil {
		t.Fatal("custom --login-profile-dir contents were deleted")
	}
}

func TestSlotsPickGraphQLErrorsExitNonZero(t *testing.T) {
	for name, reply := range map[string]string{
		"top-level errors": `{"errors":[{"message":"window no longer available (fake)"}],"data":null}`,
		"error typename":   `{"data":{"updateCheckout":{"__typename":"CheckoutCheckoutErrorResponse","errors":[{"__typename":"CheckoutCheckoutErrorsServiceOptionError","message":"slot full (fake)"}]}}}`,
		"no data":          `{"data":{"updateCheckout":null}}`,
	} {
		t.Run(name, func(t *testing.T) {
			routedSameDay(t, withReplies(map[string]string{windowsOperation: fakeWindows, updateCheckoutOp: reply}))
			out, _, err := runRootSplit(t, "slots", "pick", "--checkout-session-id", "sess-fake", "--index", "2", "--yes", "--json")
			if err == nil {
				t.Fatalf("pick exited zero on a rejected update:\n%s", out)
			}
			if ExitCode(err) == 0 || !strings.Contains(err.Error(), "did not change") {
				t.Fatalf("err = %v (code %d)", err, ExitCode(err))
			}
			var m map[string]any
			if jerr := json.Unmarshal([]byte(out), &m); jerr != nil {
				t.Fatalf("output not JSON: %s", out)
			}
			if m["updated"] != false {
				t.Fatalf("updated = %v, want false\n%s", m["updated"], out)
			}
			if strings.Contains(out, "tok-") {
				t.Fatalf("token printed:\n%s", out)
			}
		})
	}
}

func TestSlotsPickSuccessReportsUpdated(t *testing.T) {
	routedSameDay(t, withReplies(map[string]string{windowsOperation: fakeWindows, updateCheckoutOp: `{"data":{"updateCheckout":{"__typename":"CheckoutCheckoutSuccessResponse"}}}`}))
	out, _, err := runRootSplit(t, "slots", "pick", "--checkout-session-id", "sess-fake", "--index", "2", "--yes", "--json")
	if err != nil || !strings.Contains(out, `"updated": true`) {
		t.Fatalf("err=%v\n%s", err, out)
	}
}

func TestAmbiguousZipAddressRefusesAutoPick(t *testing.T) {
	twoSameZip := `{"data":{"userAddresses":[` +
		`{"id":"addr-1-fake","postalCode":"98027","streetAddress":"1234 Maple Ave"},` +
		`{"id":"addr-3-fake","postalCode":"98027","streetAddress":"77 Oak Rd"}]}}`
	rec := routedSameDay(t, withReplies(map[string]string{
		"UserAddresses":   twoSameZip,
		"CartTotals":      `{"data":{"cartTotals":{"retailerTotal":{"amount":"12.34","currencyCode":"USD"}}}}`,
		"CartSignaledEta": `{"data":{"cartSignaledEta":{"viewSection":{"etaString":"fake"}}}}`,
	}))
	out, errOut, err := runRootSplit(t, "cart", "cartsignaledeta", "--zip", "98027", "--json")
	if err == nil {
		t.Fatalf("ambiguous ZIP auto-picked an address:\n%s", out)
	}
	msg := err.Error()
	for _, want := range []string{"--address-id", "addr-1-fake", "addr-3-fake", "1*** M**** A**", "980xx"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error missing %q: %s", want, msg)
		}
	}
	for _, leak := range []string{"Maple", "Oak", "1234"} {
		if strings.Contains(msg+out+errOut, leak) {
			t.Fatalf("street leaked (%q): %s", leak, msg)
		}
	}
	if _, sent := rec.methods["CartSignaledEta"]; sent {
		t.Fatal("CartSignaledEta was sent without a safe addressId")
	}
	// address list marks the ambiguity and picks nothing.
	out, _, err = runRootSplit(t, "address", "list", "--zip", "98027", "--json")
	if err != nil || strings.Contains(out, "auto_address_id") || !strings.Contains(out, "auto_note") {
		t.Fatalf("address list: %v\n%s", err, out)
	}
	// An explicit --address-id still works.
	if _, _, err := runRootSplit(t, "cart", "cartsignaledeta", "--zip", "98027", "--address-id", "addr-3-fake", "--json"); err != nil {
		t.Fatalf("explicit address: %v", err)
	}
	if rec.vars["CartSignaledEta"]["addressId"] != "addr-3-fake" {
		t.Fatalf("vars = %v", rec.vars["CartSignaledEta"])
	}
}

func TestExplicitCartIDUsesThatCartsSubtotal(t *testing.T) {
	rec := routedSameDay(t, withReplies(map[string]string{
		"UserAddresses":   fakeAddresses,
		"CartTotals":      `{"data":{"cartTotals":{"retailerTotal":{"amount":"77.70","currencyCode":"USD"}}}}`,
		"CartSignaledEta": `{"data":{"cartSignaledEta":{"viewSection":{"etaString":"fake"}}}}`,
	}))
	if _, _, err := runRootSplit(t, "cart", "cartsignaledeta", "--zip", "98027", "--cart-id", "cart-explicit-fake", "--json"); err != nil {
		t.Fatalf("cartsignaledeta: %v", err)
	}
	if got := rec.vars["CartTotals"]["cartId"]; got != "cart-explicit-fake" {
		t.Fatalf("CartTotals read cart %v, want the explicit --cart-id", got)
	}
	sub, _ := rec.vars["CartSignaledEta"]["subtotal"].(map[string]any)
	if sub["amount"] != "77.70" || rec.vars["CartSignaledEta"]["cartId"] != "cart-explicit-fake" {
		t.Fatalf("vars = %v", rec.vars["CartSignaledEta"])
	}
}

func TestExplicitCartIDWithoutReadableTotalRequiresSubtotal(t *testing.T) {
	rec := routedSameDay(t, withReplies(map[string]string{
		"UserAddresses":   fakeAddresses,
		"CartSignaledEta": `{"data":{"cartSignaledEta":{"viewSection":{"etaString":"fake"}}}}`,
		// no CartTotals reply: the total cannot be read
	}))
	_, _, err := runRootSplit(t, "cart", "cartsignaledeta", "--zip", "98027", "--cart-id", "cart-explicit-fake", "--json")
	if err == nil || !strings.Contains(err.Error(), "--subtotal") {
		t.Fatalf("err = %v", err)
	}
	if _, sent := rec.methods["CartSignaledEta"]; sent {
		t.Fatal("sent CartSignaledEta with another cart's (or no) subtotal")
	}
}

func TestBadSubtotalRejectedBeforeSend(t *testing.T) {
	for _, bad := range []string{"+50", "50.", "-5", "1e3", "$5", "5.123", "1,000"} {
		rec := routedSameDay(t, withReplies(map[string]string{"UserAddresses": fakeAddresses}))
		_, _, err := runRootSplit(t, "cart", "cartsignaledeta", "--zip", "98027", "--address-id", "addr-1-fake", "--subtotal", bad, "--json")
		if err == nil || !strings.Contains(err.Error(), "invalid --subtotal") {
			t.Fatalf("--subtotal %q: err = %v", bad, err)
		}
		if _, sent := rec.methods["CartSignaledEta"]; sent {
			t.Fatalf("--subtotal %q was sent", bad)
		}
	}
}

func TestSlotsWindowsShowTokensInTable(t *testing.T) {
	routedSameDay(t, withReplies(map[string]string{windowsOperation: fakeWindows}))
	out, _, err := runRootSplit(t, "slots", "windows", "--checkout-session-id", "sess-fake", "--show-tokens", "--human-friendly")
	if err != nil {
		t.Fatalf("windows: %v\n%s", err, out)
	}
	if !strings.Contains(out, "token: tok-b-fake") || strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Fatalf("table output lacks tokens:\n%s", out)
	}
	out, _, _ = runRootSplit(t, "slots", "windows", "--checkout-session-id", "sess-fake", "--human-friendly")
	if strings.Contains(out, "tok-") {
		t.Fatalf("table printed tokens without --show-tokens:\n%s", out)
	}
}

func TestSlotsEtasFailedQueryExitsNonZeroWithPartialOutput(t *testing.T) {
	routedSameDay(t, withReplies(map[string]string{
		"ShopCollectionScoped":    fakeShops,
		"UserType":                `{"data":{"currentUser":{"guest":false}}}`,
		"GetAccurateRetailerEtas": fakeDeliveryEtas,
		// pickup ETA query unanswered -> GraphQL error
	}))
	out, _, err := runRootSplit(t, "slots", "etas", "--zip", "98027", "--as", "member", "--json")
	if err == nil {
		t.Fatalf("etas exited zero with a failed pickup query:\n%s", out)
	}
	if ExitCode(err) == 0 || !strings.Contains(err.Error(), "pickup") {
		t.Fatalf("err = %v", err)
	}
	m := etaOutput(t, out)
	if m["partial"] != true || len(m["services"].([]any)) == 0 {
		t.Fatalf("partial output missing: %v", m)
	}
	failed, _ := json.Marshal(m["failed"])
	if string(failed) != `["pickup"]` {
		t.Fatalf("failed = %s", failed)
	}
	// Delivery only (no failing query) still exits zero.
	if _, _, err := runRootSplit(t, "slots", "etas", "--zip", "98027", "--as", "member", "--service", "delivery", "--json"); err != nil {
		t.Fatalf("delivery only: %v", err)
	}
}

// fakeBrowserBin writes a stub "browser" script (Unix only).
func fakeBrowserBin(t *testing.T, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script browser stub is Unix-only")
	}
	p := filepath.Join(t.TempDir(), "fake-chrome")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestBrowserLoginHonorsShortTimeoutWhenBrowserIsSilent(t *testing.T) {
	bin := fakeBrowserBin(t, "sleep 30")
	start := time.Now()
	_, err := runBrowserLogin(context.Background(), browserLoginOptions{
		StartURL: "https://example.test/", ProfileDir: filepath.Join(t.TempDir(), browserLoginProfile),
		BrowserPath: bin, Timeout: 1500 * time.Millisecond, Out: io.Discard,
	})
	if el := time.Since(start); el > 6*time.Second {
		t.Fatalf("login waited %s past a 1.5s timeout", el)
	}
	var te *browserLoginTimeoutError
	if !errors.As(err, &te) {
		t.Fatalf("err = %v, want timeout", err)
	}
}

func TestBrowserLoginEarlyExitIsProfileInUse(t *testing.T) {
	bin := fakeBrowserBin(t, "exit 0")
	start := time.Now()
	_, err := runBrowserLogin(context.Background(), browserLoginOptions{
		StartURL: "https://example.test/", ProfileDir: filepath.Join(t.TempDir(), browserLoginProfile),
		BrowserPath: bin, Timeout: 30 * time.Second, Out: io.Discard,
	})
	if !errors.Is(err, errProfileInUse) {
		t.Fatalf("err = %v, want profile-in-use", err)
	}
	if el := time.Since(start); el > 6*time.Second {
		t.Fatalf("early exit took %s to detect", el)
	}
}
