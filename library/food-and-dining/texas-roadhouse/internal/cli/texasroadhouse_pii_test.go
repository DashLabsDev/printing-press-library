// Copyright 2026 Thomas McCormick and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"io"
	"os"
	"strings"
	"testing"
)

func captureOSStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = orig }()
	fn()
	_ = w.Close()
	out, err := io.ReadAll(r)
	_ = r.Close()
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestRejectWaitlistPIIFlags(t *testing.T) {
	withTempLearnHome(t)
	_, _, err := runRootArgs(t,
		"texasroadhouse", "submit", "218",
		"--email-address", "guest@example.test",
		"--first-name", "Test",
		"--last-name", "User",
		"--primary-phone-area-code", "555",
		"--primary-phone-number", "555-0100",
		"--party-size", "2",
		"--wait-minutes", "10",
		"--dry-run", "--agent", "--no-learn",
	)
	if err == nil {
		t.Fatal("expected argv PII flags to be refused")
	}
	if !strings.Contains(err.Error(), "must not be passed as argv flags") {
		t.Fatalf("error = %v, want argv PII refusal", err)
	}
}

func TestRedactWaitlistPII(t *testing.T) {
	got := redactWaitlistPII(map[string]any{
		"EmailAddress":         "guest@example.test",
		"FirstName":            "Test",
		"LastName":             "User",
		"PrimaryPhoneAreaCode": "555",
		"PrimaryPhoneNumber":   "555-0100",
		"PartySize":            2,
		"WaitMinutes":          10,
	})
	for _, key := range []string{"EmailAddress", "FirstName", "LastName", "PrimaryPhoneAreaCode", "PrimaryPhoneNumber"} {
		if got[key] != waitlistPIIRedacted {
			t.Fatalf("%s = %v, want %s", key, got[key], waitlistPIIRedacted)
		}
	}
	if got["PartySize"] != 2 || got["WaitMinutes"] != 10 {
		t.Fatalf("non-PII fields were changed: %#v", got)
	}
}

func TestSubmitDryRunRedactsPII(t *testing.T) {
	withTempLearnHome(t)
	stderr := captureOSStderr(t, func() {
		_, _, err := runRootArgsWithStdin(t, submitGuestStdinJSON,
			"texasroadhouse", "submit", "218",
			"--stdin", "--dry-run", "--agent", "--no-learn",
		)
		if err != nil {
			t.Fatalf("dry-run submit: %v", err)
		}
	})
	leaks := []string{"guest@example.test", "555-0100"}
	for _, leak := range leaks {
		if strings.Contains(stderr, leak) {
			t.Fatalf("dry-run stderr leaked %q: %s", leak, stderr)
		}
	}
	if !strings.Contains(stderr, "redacted") {
		t.Fatalf("dry-run stderr should redact PII, got %q", stderr)
	}
}

func TestLooksLikeCloudflareChallenge(t *testing.T) {
	if !LooksLikeCloudflareChallenge("GET /api/stores/near returned HTTP 403: HTML error page (1200 bytes): Just a moment") {
		t.Fatal("expected Just a moment 403 to classify as Cloudflare")
	}
	if !LooksLikeCloudflareChallenge("GET /x returned HTTP 403: cf-mitigated: challenge") {
		t.Fatal("expected cf-mitigated 403 to classify as Cloudflare")
	}
	if LooksLikeCloudflareChallenge("GET /x returned HTTP 403: permission denied") {
		t.Fatal("generic 403 should not be a Cloudflare challenge")
	}
}

func TestClassifyWaitlistHTTP404OmitsListCommand(t *testing.T) {
	err := classifyAPIErrorOnly(fmtWaitlistAPIError("GET /api/stores/near returned HTTP 404: missing"))
	if err == nil {
		t.Fatal("expected 404 error")
	}
	if strings.Contains(err.Error(), "list' command") || strings.Contains(err.Error(), "list command to see") {
		t.Fatalf("404 hint still mentions list command: %v", err)
	}
	if !strings.Contains(err.Error(), "stores") || !strings.Contains(err.Error(), "get-quote") {
		t.Fatalf("404 hint should point at stores/get-quote: %v", err)
	}
}

func TestClassifyWaitlistHTTP403Cloudflare(t *testing.T) {
	err := classifyAPIErrorOnly(fmtWaitlistAPIError("GET /api/stores/near returned HTTP 403: HTML error page (800 bytes): Just a moment"))
	if err == nil {
		t.Fatal("expected 403 error")
	}
	if !strings.Contains(err.Error(), "Cloudflare challenge") {
		t.Fatalf("403 Cloudflare hint missing: %v", err)
	}
	if !strings.Contains(err.Error(), "Chrome-compatible") {
		t.Fatalf("403 should mention Chrome-compatible transport: %v", err)
	}
}

func fmtWaitlistAPIError(msg string) error {
	return &simpleError{msg: msg}
}

type simpleError struct{ msg string }

func (e *simpleError) Error() string { return e.msg }
