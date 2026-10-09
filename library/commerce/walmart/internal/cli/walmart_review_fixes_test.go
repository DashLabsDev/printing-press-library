// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// Regression tests for the PR review follow-ups. All values are fake.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/walmart/internal/client"
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

func TestCookiesFileExpiredAuthReadsLapsed(t *testing.T) {
	now := time.Now()
	future := float64(now.Add(24 * time.Hour).Unix())
	past := float64(now.Add(-time.Minute).Unix())
	b, _ := json.Marshal(map[string]any{"cookies": []map[string]any{
		{"name": "CID", "value": "fake-cid", "domain": ".walmart.com", "expires": future},
		{"name": "SPID", "value": "fake-spid", "domain": ".walmart.com", "expires": future},
		{"name": "customer", "value": "fake-cust", "domain": ".walmart.com", "expires": -1},
		{"name": "auth", "value": "fake-auth", "domain": ".walmart.com", "expires": past},
	}})
	f, err := parseCookiesFileData(b, "walmart.com", now)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(f.Header, "auth=") {
		t.Fatalf("expired auth must not reach the header: %q", f.Header)
	}
	rows := importedRowsFromFile(f.Cookies, now)
	sess := classifyWalmartSession(parseCookieString(f.Header), authCookieExpiry(rows), now)
	if sess.State != sessionLapsed {
		t.Fatalf("expired auth must read as lapsed, got %+v", sess)
	}
	for _, r := range rows {
		if r.Name == "customer" && !r.Expires.IsZero() {
			t.Fatal("expires -1 must be a session cookie (zero expiry)")
		}
	}
}

func TestCookiesFileExtensionArrayFormat(t *testing.T) {
	now := time.Now()
	b, _ := json.Marshal([]map[string]any{
		{"name": "fake_a", "value": "1", "domain": ".walmart.com", "path": "/", "expirationDate": float64(now.Add(time.Hour).Unix()) + 0.5, "secure": true, "httpOnly": true},
		{"name": "fake_s", "value": "2", "domain": "www.walmart.com", "session": true, "expirationDate": float64(now.Add(time.Hour).Unix())},
		{"name": "fake_x", "value": "3", "domain": ".walmart.com", "expirationDate": float64(now.Add(-time.Hour).Unix())},
		{"name": "fake_other", "value": "4", "domain": ".example.com"},
	})
	f, err := parseCookiesFileData(b, "walmart.com", now)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]*http.Cookie{}
	for _, c := range f.Cookies {
		got[c.Name] = c
	}
	if len(got) != 2 || got["fake_a"] == nil || got["fake_s"] == nil {
		t.Fatalf("want fake_a and fake_s only, got %v (header %q)", got, f.Header)
	}
	if got["fake_a"].Expires.IsZero() || !got["fake_a"].HttpOnly || !got["fake_a"].Secure {
		t.Fatalf("fake_a lost expiry/flags: %+v", got["fake_a"])
	}
	if !got["fake_s"].Expires.IsZero() {
		t.Fatal("session:true must ignore expirationDate")
	}
}

func TestCookiesFileNetscapeFormat(t *testing.T) {
	now := time.Now()
	future := now.Add(time.Hour).Unix()
	pastTS := now.Add(-time.Hour).Unix()
	txt := "# Netscape HTTP Cookie File\n" +
		".walmart.com\tTRUE\t/\tTRUE\t" + itoa(future) + "\tfake_n1\tv1\n" +
		"#HttpOnly_www.walmart.com\tFALSE\t/account\tTRUE\t0\tfake_n2\tv2\n" +
		".walmart.com\tTRUE\t/\tFALSE\t" + itoa(pastTS) + "\tfake_n3\tv3\n"
	f, err := parseCookiesFileData([]byte(txt), "walmart.com", now)
	if err != nil {
		t.Fatal(err)
	}
	if f.Header != "fake_n1=v1; fake_n2=v2" {
		t.Fatalf("header: %q", f.Header)
	}
	rows := importedRowsFromFile(f.Cookies, now)
	if len(rows) != 2 || rows[0].Expires.Unix() != future || !rows[0].Secure {
		t.Fatalf("n1 row: %+v", rows)
	}
	if !rows[1].HTTPOnly || !rows[1].Expires.IsZero() || rows[1].Path != "/account" || rows[1].Domain != "www.walmart.com" {
		t.Fatalf("n2 row: %+v", rows[1])
	}
}

func TestCookiesFileAllExpiredAndBadJSON(t *testing.T) {
	now := time.Now()
	b, _ := json.Marshal(map[string]any{"cookies": []map[string]any{
		{"name": "fake_old", "value": "1", "domain": ".walmart.com", "expires": float64(now.Add(-time.Hour).Unix())},
	}})
	if _, err := parseCookiesFileData(b, "walmart.com", now); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("all-expired must error clearly, got %v", err)
	}
	if _, err := parseCookiesFileData([]byte(`{"not":"cookies"`), "walmart.com", now); err == nil {
		t.Fatal("malformed JSON must not fall through to a raw header")
	}
	f, err := parseCookiesFileData([]byte("Cookie: fake_h=1;fake_i=2"), "walmart.com", now)
	if err != nil || f.Header != "fake_h=1;fake_i=2" {
		t.Fatalf("raw header: %q %v", f.Header, err)
	}
}

func itoa(v int64) string { return strconv.FormatInt(v, 10) }

func TestDescribeSavedSession(t *testing.T) {
	now := time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)
	base := []client.ImportedCookie{
		{Name: "CID", Value: "fake-secret-cid", Domain: ".walmart.com", Expires: now.Add(24 * time.Hour)},
		{Name: "SPID", Value: "fake-secret-spid", Domain: ".walmart.com"},
		{Name: "customer", Value: "fake-secret-cust", Domain: ".walmart.com"},
	}
	with := func(extra ...client.ImportedCookie) []client.ImportedCookie {
		return append(append([]client.ImportedCookie{}, base...), extra...)
	}
	cases := []struct {
		name   string
		rows   []client.ImportedCookie
		active bool
		want   string
	}{
		{"expiring", with(client.ImportedCookie{Name: "auth", Value: "fake-secret-auth", Domain: ".walmart.com", Expires: now.Add(25 * time.Minute)}), true, "Auth cookie: expires"},
		{"session", with(client.ImportedCookie{Name: "auth", Value: "fake-secret-auth", Domain: ".walmart.com"}), true, "session cookie (no expiry"},
		{"expired", with(client.ImportedCookie{Name: "auth", Value: "fake-secret-auth", Domain: ".walmart.com", Expires: now.Add(-time.Minute)}), false, "Auth cookie: expired"},
		{"missing", with(), false, "Auth cookie: not present"},
		{"empty", nil, false, "no saved browser cookies"},
	}
	for _, tc := range cases {
		lines, active := describeSavedSession(tc.rows, now)
		out := strings.Join(lines, "\n")
		if active != tc.active || !strings.Contains(out, tc.want) {
			t.Fatalf("%s: active=%v out=%q", tc.name, active, out)
		}
		if strings.Contains(out, "fake-secret") {
			t.Fatalf("%s: cookie values must never be printed: %q", tc.name, out)
		}
	}
	lines, _ := describeSavedSession(with(client.ImportedCookie{Name: "auth", Value: "x", Domain: ".walmart.com", Expires: now.Add(-time.Minute)}), now)
	if !strings.Contains(strings.Join(lines, "\n"), "Cookies: 4 saved (2 session-only, 1 expired)") {
		t.Fatalf("counts: %q", lines)
	}
}

func TestEmitReadAgentReportsLiveSource(t *testing.T) {
	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	flags := &rootFlags{agent: true, asJSON: true}
	if err := emitRead(cmd, flags, map[string]any{"stores": []fakeRow{{Name: "fake-store"}}}, nil); err != nil {
		t.Fatal(err)
	}
	var env struct {
		Meta    map[string]any `json:"meta"`
		Results json.RawMessage
	}
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("%q: %v", buf.String(), err)
	}
	if env.Meta["source"] != "live" {
		t.Fatalf("meta.source = %v, want live (%s)", env.Meta["source"], buf.String())
	}
	if !strings.Contains(string(env.Results), "fake-store") {
		t.Fatalf("results lost: %s", buf.String())
	}
}
