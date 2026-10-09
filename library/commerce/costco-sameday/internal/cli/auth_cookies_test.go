// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0.
// Hand-authored auth cookie import tests. All cookie values are obviously fake.

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

const (
	fakeSID     = "FAKE-sid-value-0000"
	fakeBCX     = "FAKE-bcx-1111"
	fakeVisitor = "FAKE-visitor-2222"
	fakeVisit   = "FAKE-visit-3333"
	fakeBuild   = "FAKE-build-4444"
)

func authCmds(flags *rootFlags) []*cobra.Command { return []*cobra.Command{newAuthCmd(flags)} }

func isolatedAuthEnv(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))
	t.Setenv("COSTCO_SAMEDAY_CONFIG", filepath.Join(dir, "config.toml"))
	t.Setenv("COSTCO_SAMEDAY_BASE_URL", "")
	return dir
}

func writeCookieHeader(t *testing.T, dir, header string) string {
	t.Helper()
	p := filepath.Join(dir, "cookie-header.txt")
	if err := os.WriteFile(p, []byte(header), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSelectSessionCookiesWithoutInstacartSessionID(t *testing.T) {
	header := "__Host-instacart_sid=" + fakeSID + "; X-IC-bcx=" + fakeBCX +
		"; ahoy_visitor=" + fakeVisitor + "; ahoy_visit=" + fakeVisit + "; build_sha=" + fakeBuild
	kept, dropped, err := selectSessionCookies(header)
	if err != nil {
		t.Fatalf("expected success without _instacart_session_id, got %v", err)
	}
	if len(dropped) != 0 {
		t.Fatalf("unexpected dropped cookies: %v", dropped)
	}
	if len(kept) != 5 || kept[0].Name != "__Host-instacart_sid" || kept[0].Value != fakeSID {
		t.Fatalf("unexpected kept cookies: %+v", kept)
	}
}

func TestSelectSessionCookiesOnlySessionCookie(t *testing.T) {
	kept, _, err := selectSessionCookies("Cookie: __Host-instacart_sid=" + fakeSID)
	if err != nil || len(kept) != 1 {
		t.Fatalf("expected the session cookie alone to be enough, got kept=%+v err=%v", kept, err)
	}
}

func TestSelectSessionCookiesKeepsUnknownAndDropsMalformedOptional(t *testing.T) {
	header := "ahoy_visit=" + fakeVisit + ";__Host-instacart_sid=" + fakeSID + "; bad=x\\y; extra_cookie=FAKE-extra"
	kept, dropped, err := selectSessionCookies(header)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dropped) != 1 || dropped[0] != "bad" {
		t.Fatalf("expected malformed optional cookie to be dropped by name, got %v", dropped)
	}
	names := []string{}
	for _, c := range kept {
		names = append(names, c.Name)
	}
	if strings.Join(names, ",") != "ahoy_visit,__Host-instacart_sid,extra_cookie" {
		t.Fatalf("unexpected kept order: %v", names)
	}
}

func TestSelectSessionCookiesMissingSessionCookieFails(t *testing.T) {
	header := "_instacart_session_id=FAKE-legacy; X-IC-bcx=" + fakeBCX + "; ahoy_visitor=" + fakeVisitor
	_, _, err := selectSessionCookies(header)
	if err == nil {
		t.Fatal("expected failure without __Host-instacart_sid")
	}
	if !strings.Contains(err.Error(), "__Host-instacart_sid") {
		t.Fatalf("error should name the required cookie: %v", err)
	}
	if strings.Contains(err.Error(), "FAKE") {
		t.Fatalf("error must not echo cookie values: %v", err)
	}
}

func TestAuthLoginCookiesFileWithoutInstacartSessionIDLogsIn(t *testing.T) {
	dir := isolatedAuthEnv(t)
	path := writeCookieHeader(t, dir, "__Host-instacart_sid="+fakeSID+"; X-IC-bcx="+fakeBCX+
		"; ahoy_visitor="+fakeVisitor+"; ahoy_visit="+fakeVisit+"; build_sha="+fakeBuild+"\n")
	out, err := runCLI(t, []string{"auth", "login", "--cookies-file", path}, authCmds)
	if err != nil {
		t.Fatalf("login failed: %v\n%s", err, out)
	}
	if strings.Contains(out, "FAKE") {
		t.Fatalf("login output must not echo cookie values:\n%s", out)
	}
	if !strings.Contains(out, "_instacart_session_id") || !strings.Contains(out, "not required") {
		t.Fatalf("expected an optional-cookie note, got:\n%s", out)
	}
	status, err := runCLI(t, []string{"auth", "status"}, authCmds)
	if err != nil || !strings.Contains(status, "Authenticated") {
		t.Fatalf("expected Authenticated after login, err=%v\n%s", err, status)
	}
}

func TestAuthLoginCookiesFromStdin(t *testing.T) {
	isolatedAuthEnv(t)
	flags := &rootFlags{noCache: true}
	root := &cobra.Command{Use: "root", SilenceUsage: true, SilenceErrors: true}
	root.AddCommand(newAuthCmd(flags))
	root.SetArgs([]string{"auth", "login", "--cookies-file", "-"})
	root.SetIn(strings.NewReader("cookie: __Host-instacart_sid=" + fakeSID + "; ahoy_visit=" + fakeVisit + "\n"))
	var buf strings.Builder
	root.SetOut(&buf)
	root.SetErr(&buf)
	if err := root.Execute(); err != nil {
		t.Fatalf("stdin login failed: %v\n%s", err, buf.String())
	}
	if !strings.Contains(buf.String(), "Loaded cookies from stdin") || strings.Contains(buf.String(), "FAKE") {
		t.Fatalf("unexpected output:\n%s", buf.String())
	}
}

func TestAuthLoginMissingSessionCookieFailsClearly(t *testing.T) {
	dir := isolatedAuthEnv(t)
	path := writeCookieHeader(t, dir, "_instacart_session_id=FAKE-legacy; X-IC-bcx="+fakeBCX+"; ahoy_visitor="+fakeVisitor)
	out, err := runCLI(t, []string{"auth", "login", "--cookies-file", path}, authCmds)
	if err == nil {
		t.Fatalf("expected login to fail without __Host-instacart_sid\n%s", out)
	}
	if !strings.Contains(err.Error(), "__Host-instacart_sid") || !strings.Contains(out, "__Host-instacart_sid") {
		t.Fatalf("failure should name __Host-instacart_sid, err=%v\n%s", err, out)
	}
	if strings.Contains(out+err.Error(), "FAKE") {
		t.Fatalf("failure must not echo cookie values")
	}
	status, serr := runCLI(t, []string{"auth", "status"}, authCmds)
	if serr == nil || strings.Contains(status, "Authenticated\n") && !strings.Contains(status, "Not authenticated") {
		t.Fatalf("no credentials should be saved after a failed login:\n%s", status)
	}
}

func TestRequiredAuthCookiesIsSessionOnly(t *testing.T) {
	got := requiredAuthCookies()
	if len(got) != 1 || got[0] != "__Host-instacart_sid" {
		t.Fatalf("requiredAuthCookies() = %v", got)
	}
}
