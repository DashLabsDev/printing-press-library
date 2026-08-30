// Copyright 2026 Thomas McCormick and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
)

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

func TestSubmitDryRunRedactsPII(t *testing.T) {
	withTempLearnHome(t)
	_, stderr, err := runRootArgsWithStdin(t, submitGuestStdinJSON,
		"texasroadhouse", "submit", "218",
		"--stdin", "--dry-run", "--agent", "--no-learn",
	)
	if err != nil {
		t.Fatalf("dry-run submit: %v (stderr=%q)", err, stderr)
	}
	leaks := []string{"guest@example.test", "555-0100", `"Test"`, `"User"`}
	for _, leak := range leaks {
		if strings.Contains(stderr, leak) {
			t.Fatalf("dry-run stderr leaked %q: %s", leak, stderr)
		}
	}
	if !strings.Contains(stderr, waitlistPIIRedacted) {
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
