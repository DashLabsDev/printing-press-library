// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// Regression tests for the PR review follow-ups. All values are fake.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/walmart/internal/cliutil"
)

func TestCookiesFileKeepsPerHostScope(t *testing.T) {
	now := time.Now()
	future := float64(now.Add(24 * time.Hour).Unix())
	past := float64(now.Add(-time.Hour).Unix())
	state := map[string]any{"cookies": []map[string]any{
		{"name": "fake_sid", "value": "www-value", "domain": "www.walmart.com", "path": "/", "expires": future, "secure": true, "httpOnly": true},
		{"name": "fake_sid", "value": "identity-value", "domain": "identity.walmart.com", "path": "/account", "expires": future, "secure": true},
		{"name": "fake_old", "value": "expired-value", "domain": ".walmart.com", "path": "/", "expires": past},
		{"name": "fake_session", "value": "session-value", "domain": ".walmart.com", "path": ""},
	}}
	b, _ := json.Marshal(state)
	p := filepath.Join(t.TempDir(), "cookies.json")
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
	imported, err := loadCookiesFromFile(p, "walmart.com")
	if err != nil {
		t.Fatal(err)
	}
	rows := importedRowsFromFile(imported.Cookies, now)
	if len(rows) != 3 {
		t.Fatalf("want 3 rows (expired dropped), got %d: %+v", len(rows), rows)
	}
	byDomain := map[string]string{}
	for _, r := range rows {
		if r.Name == "fake_old" {
			t.Fatal("expired cookie must be dropped")
		}
		byDomain[r.Domain+"|"+r.Name] = r.Value + "|" + r.Path
	}
	if byDomain["www.walmart.com|fake_sid"] != "www-value|/" {
		t.Fatalf("www row lost scope: %v", byDomain)
	}
	if byDomain["identity.walmart.com|fake_sid"] != "identity-value|/account" {
		t.Fatalf("identity row lost scope: %v", byDomain)
	}
	if byDomain[".walmart.com|fake_session"] != "session-value|/" {
		t.Fatalf("session cookie missing default path: %v", byDomain)
	}
	for _, r := range rows {
		if r.Domain == "www.walmart.com" && (!r.Secure || !r.HTTPOnly || r.Expires.IsZero()) {
			t.Fatalf("flags/expiry not kept: %+v", r)
		}
	}
}

func TestCookiesFileRawHeaderHasNoScopedRows(t *testing.T) {
	p := filepath.Join(t.TempDir(), "cookies.txt")
	if err := os.WriteFile(p, []byte("Cookie: fake_a=1;fake_b=2"), 0o600); err != nil {
		t.Fatal(err)
	}
	imported, err := loadCookiesFromFile(p, "walmart.com")
	if err != nil {
		t.Fatal(err)
	}
	if rows := importedRowsFromFile(imported.Cookies, time.Now()); len(rows) != 0 {
		t.Fatalf("raw header rows have no domain and must use the fallback: %+v", rows)
	}
}

func TestParseCookieStringAcceptsBareSemicolons(t *testing.T) {
	got := parseCookieString("CID=fake1;SPID=fake2; customer=fake3 ;auth=fake=4;;")
	want := map[string]string{"CID": "fake1", "SPID": "fake2", "customer": "fake3", "auth": "fake=4"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("%s: got %q want %q (all: %v)", k, got[k], v, got)
		}
	}
}

func TestLocalDataSourceRejectedBeforeNetwork(t *testing.T) {
	flags := &rootFlags{dataSource: "local"}
	if err := requireLiveSource(flags); err == nil || !strings.Contains(err.Error(), "--data-source") {
		t.Fatalf("local must be rejected, got %v", err)
	}
	for _, ok := range []string{"", "auto", "live"} {
		if err := requireLiveSource(&rootFlags{dataSource: ok}); err != nil {
			t.Fatalf("%q must be allowed: %v", ok, err)
		}
	}
	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	_, err := orchestraGet(cmd, flags, "fake", "fakeOp", "fakehash", "{}", "https://example.invalid/", "", nil)
	if err == nil || !strings.Contains(err.Error(), "no local data") {
		t.Fatalf("orchestraGet must fail before building a client, got %v", err)
	}
}

type fakeRow struct {
	Name  string `json:"name"`
	Price string `json:"price"`
}

func TestEmitReadHonorsOutputFlags(t *testing.T) {
	rows := []fakeRow{{Name: "fake-a", Price: "1.00"}, {Name: "fake-b", Price: "2.00"}}
	full := map[string]any{"products": rows, "page": 1}

	run := func(flags *rootFlags) string {
		var buf bytes.Buffer
		cmd := &cobra.Command{}
		cmd.SetOut(&buf)
		if !wantJSON(cmd, flags) {
			t.Fatalf("wantJSON must be true for %+v", flags)
		}
		if err := emitRead(cmd, flags, full, rows); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}
	if out := run(&rootFlags{asJSON: true, selectFields: "page"}); strings.Contains(out, "fake-a") || !strings.Contains(out, "page") {
		t.Fatalf("--select not applied: %s", out)
	}
	if out := run(&rootFlags{csv: true}); !strings.Contains(out, "fake-a") || !strings.Contains(out, ",") || strings.Contains(out, "{") {
		t.Fatalf("--csv must render rows: %s", out)
	}
}

func TestFetchedCartIDFallsBackToRequestedID(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WALMART_DATA_DIR", dir)
	t.Setenv("XDG_DATA_HOME", dir)
	t.Setenv("WALMART_CART_ID", "")
	const requested = "00000000-0000-4000-8000-0000000000c1"
	const reported = "00000000-0000-4000-8000-0000000000c2"
	rememberFetchedCartID("", requested)
	if id, src, _ := resolveCartID(""); id != requested || src != "remembered" {
		t.Fatalf("requested id must be remembered when response has none: %s %s", id, src)
	}
	rememberFetchedCartID(reported, requested)
	if id, _, _ := resolveCartID(""); id != reported {
		t.Fatalf("response id must win: %s", id)
	}
}

func runCartLink(t *testing.T, flags *rootFlags, args ...string) string {
	t.Helper()
	cmd := newCartLinkCmd(flags)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cart link %v: %v", args, err)
	}
	return buf.String()
}

func TestCartLinkItemsFlagAddsItems(t *testing.T) {
	out := runCartLink(t, &rootFlags{asJSON: true}, "111111", "--items", "222222:2, 333333")
	var plan cartLinkPlan
	if err := json.Unmarshal([]byte(out), &plan); err != nil {
		t.Fatalf("bad json %q: %v", out, err)
	}
	if len(plan.Items) != 3 || plan.Items[1].Quantity != 2 {
		t.Fatalf("items: %+v", plan.Items)
	}
	if f := newCartLinkCmd(&rootFlags{}).Flags().Lookup("items"); f == nil || f.Value.Type() != "string" {
		t.Fatal("--items must be a string flag so MCP exposes it")
	}
}

func TestCartLinkHarnessJSONKeepsURL(t *testing.T) {
	t.Setenv(cliutil.VerifyEnvVar, "1")
	out := runCartLink(t, &rootFlags{asJSON: true}, "111111", "--open")
	var plan cartLinkPlan
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &plan); err != nil {
		t.Fatalf("must be ONE json object, got %q: %v", out, err)
	}
	if !strings.Contains(plan.URL, "111111") {
		t.Fatalf("url missing: %+v", plan)
	}
	if plan.Refusal == nil || !plan.Refusal.Refused || plan.Opened {
		t.Fatalf("refusal missing or opened: %+v", plan)
	}
}

func TestClassifyAccountProbe(t *testing.T) {
	cases := map[string]string{
		`{"data":{"account":{"fakeField":"x"}}}`:                      "valid",
		`{"data":{"account":null},"errors":[{"message":"fake err"}]}`: "invalid",
		`{"data":null}`:               "WARN not verified",
		`{"data":{}}`:                 "WARN not verified",
		`{"data":{"account":null}}`:   "WARN not verified",
		`<html>fake challenge</html>`: "WARN not verified",
	}
	for body, want := range cases {
		if got := classifyAccountProbe([]byte(body)); !strings.HasPrefix(got, want) {
			t.Fatalf("%s: got %q want prefix %q", body, got, want)
		}
	}
}
