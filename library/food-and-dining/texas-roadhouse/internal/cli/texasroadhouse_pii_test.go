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
	got := RedactWaitlistPII(map[string]any{
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

func TestRedactWaitlistPIIRecursivelyAndCaseInsensitively(t *testing.T) {
	body := map[string]any{
		"emailaddress": "guest@example.test",
		"Nested": map[string]any{
			"FiRsTnAmE": "Guest",
			"items": []any{
				map[string]any{
					"primaryphonenumber": "555-0100",
					"keep":               "safe",
				},
			},
		},
		"PartySize": 2,
	}

	got := RedactWaitlistPII(body)
	if got["emailaddress"] != waitlistPIIRedacted {
		t.Fatalf("lowercase email = %v, want %s", got["emailaddress"], waitlistPIIRedacted)
	}
	nested, ok := got["Nested"].(map[string]any)
	if !ok {
		t.Fatalf("nested value = %T, want map[string]any", got["Nested"])
	}
	if nested["FiRsTnAmE"] != waitlistPIIRedacted {
		t.Fatalf("mixed-case nested name = %v, want %s", nested["FiRsTnAmE"], waitlistPIIRedacted)
	}
	items, ok := nested["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("nested items = %#v, want one item", nested["items"])
	}
	item, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("nested item = %T, want map[string]any", items[0])
	}
	if item["primaryphonenumber"] != waitlistPIIRedacted {
		t.Fatalf("lowercase nested phone = %v, want %s", item["primaryphonenumber"], waitlistPIIRedacted)
	}
	if item["keep"] != "safe" || got["PartySize"] != 2 {
		t.Fatalf("non-PII fields changed: %#v", got)
	}
	if body["emailaddress"] != "guest@example.test" {
		t.Fatalf("redaction mutated input: %#v", body)
	}
}

func TestSubmitDryRunRejectsUnknownStdinFields(t *testing.T) {
	withTempLearnHome(t)
	_, _, err := runRootArgsWithStdin(t,
		`{"EmailAddress":"guest@example.test","FirstName":"Test","LastName":"User","PrimaryPhoneAreaCode":"555","PrimaryPhoneNumber":"555-0100","PartySize":2,"WaitMinutes":10,"unexpected":"must-not-reach-upstream"}`,
		"texasroadhouse", "submit", "218",
		"--stdin", "--dry-run", "--agent", "--no-learn",
	)
	if err == nil {
		t.Fatal("expected dry-run stdin with an unknown field to be refused")
	}
	if !strings.Contains(err.Error(), "unknown stdin JSON field") || !strings.Contains(err.Error(), "unexpected") {
		t.Fatalf("error = %v, want unknown stdin field refusal", err)
	}
}

func TestValidateWaitlistSubmitBodyFieldsRejectsWrongTypes(t *testing.T) {
	tests := []struct {
		name  string
		field string
		value any
		want  string
	}{
		{name: "email object", field: "EmailAddress", value: map[string]any{"unexpected": "must-not-reach-upstream"}, want: "string"},
		{name: "first name array", field: "FirstName", value: []any{"Test"}, want: "string"},
		{name: "last name null", field: "LastName", value: nil, want: "string"},
		{name: "smoking string", field: "IsSmoking", value: "false", want: "boolean"},
		{name: "area code boolean", field: "PrimaryPhoneAreaCode", value: false, want: "string"},
		{name: "extension object", field: "PrimaryPhoneExtension", value: map[string]any{}, want: "string"},
		{name: "phone array", field: "PrimaryPhoneNumber", value: []any{"555-0100"}, want: "string"},
		{name: "phone type boolean", field: "PrimaryPhoneType", value: true, want: "integer"},
		{name: "party size object", field: "PartySize", value: map[string]any{"unexpected": "must-not-reach-upstream"}, want: "number"},
		{name: "wait minutes fraction", field: "WaitMinutes", value: 10.5, want: "integer"},
		{name: "platform null", field: "Platform", value: nil, want: "string"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := validWaitlistSubmitBody()
			body[tc.field] = tc.value
			err := validateWaitlistSubmitBodyFields(body)
			if err == nil {
				t.Fatalf("%s accepted %T", tc.field, tc.value)
			}
			if !strings.Contains(err.Error(), tc.field) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want field %q and type %q", err, tc.field, tc.want)
			}
		})
	}
}

func TestValidateWaitlistSubmitBodyFieldsAcceptsDocumentedScalarTypes(t *testing.T) {
	if err := validateWaitlistSubmitBodyFields(validWaitlistSubmitBody()); err != nil {
		t.Fatalf("documented scalar types rejected: %v", err)
	}

	// JSON decoding produces float64 values, while the non-PII CLI flags add
	// native ints after stdin is decoded. Both forms are documented numeric
	// values and must remain valid at the final submit guard.
	body := validWaitlistSubmitBody()
	body["PrimaryPhoneType"] = 1
	body["PartySize"] = 2
	body["WaitMinutes"] = 10
	if err := requireWaitlistSubmitFields(body, false); err != nil {
		t.Fatalf("CLI numeric scalar types rejected after merge: %v", err)
	}
}

func TestRequireWaitlistSubmitFieldsRejectsStructuredValueAfterMerge(t *testing.T) {
	body := validWaitlistSubmitBody()
	body["PartySize"] = map[string]any{"unexpected": "must-not-reach-upstream"}

	err := requireWaitlistSubmitFields(body, false)
	if err == nil {
		t.Fatal("expected final submit guard to reject structured PartySize")
	}
	if !strings.Contains(err.Error(), "PartySize") || !strings.Contains(err.Error(), "number") {
		t.Fatalf("error = %v, want PartySize number refusal", err)
	}
}

func TestSubmitDryRunRejectsStructuredScalarStdin(t *testing.T) {
	withTempLearnHome(t)
	_, _, err := runRootArgsWithStdin(t,
		`{"EmailAddress":"guest@example.test","FirstName":"Test","LastName":"User","PrimaryPhoneAreaCode":"555","PrimaryPhoneNumber":"555-0100","PartySize":{"unexpected":"must-not-reach-upstream"},"WaitMinutes":10}`,
		"texasroadhouse", "submit", "218",
		"--stdin", "--dry-run", "--agent", "--no-learn",
	)
	if err == nil {
		t.Fatal("expected dry-run stdin with an object-valued PartySize to be refused")
	}
	if !strings.Contains(err.Error(), "PartySize") || !strings.Contains(err.Error(), "number") {
		t.Fatalf("error = %v, want PartySize number refusal", err)
	}
}

func validWaitlistSubmitBody() map[string]any {
	return map[string]any{
		"EmailAddress":          "guest@example.test",
		"FirstName":             "Test",
		"LastName":              "User",
		"IsSmoking":             false,
		"PrimaryPhoneAreaCode":  "555",
		"PrimaryPhoneExtension": "",
		"PrimaryPhoneNumber":    "555-0100",
		"PrimaryPhoneType":      1.0,
		"PartySize":             2.5,
		"WaitMinutes":           10.0,
		"Platform":              "web",
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
	for _, leak := range []string{"guest@example.test", "555-0100"} {
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
