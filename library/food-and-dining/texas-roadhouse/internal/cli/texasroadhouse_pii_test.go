// Copyright 2026 Thomas McCormick and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

const submitGuestStdinJSON = `{"EmailAddress":"guest@example.test","FirstName":"Test","LastName":"User","PrimaryPhoneAreaCode":"555","PrimaryPhoneNumber":"555-0100","PartySize":2,"WaitMinutes":10}`

func runRootArgsWithStdin(t *testing.T, stdin string, args ...string) (string, string, error) {
	t.Helper()
	rootCmd := RootCmd()
	var stdout, stderr strings.Builder
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetIn(strings.NewReader(stdin))
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	return stdout.String(), stderr.String(), err
}

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

func TestRejectWaitlistPIIFlagsWithoutYes(t *testing.T) {
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
		t.Fatal("expected argv PII flags without --yes to be refused")
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
	for _, leak := range []string{"guest@example.test", "555-0100"} {
		if strings.Contains(stderr, leak) {
			t.Fatalf("dry-run stderr leaked %q: %s", leak, stderr)
		}
	}
	if !strings.Contains(stderr, "redacted") {
		t.Fatalf("dry-run stderr should redact PII, got %q", stderr)
	}
}

func TestSubmitHelpHasNoUUIDOrStreetEmail(t *testing.T) {
	stdout, stderr, err := runRootArgs(t, "texasroadhouse", "submit", "--help")
	if err != nil {
		t.Fatalf("submit --help: %v (stderr=%q)", err, stderr)
	}
	if strings.Contains(stdout, "550e8400-e29b-41d4-a716-446655440000") {
		t.Fatalf("submit help still uses UUID mock: %s", stdout)
	}
	if strings.Contains(stdout, "123 Test St") {
		t.Fatalf("submit help still puts a street on --email-address: %s", stdout)
	}
	if !strings.Contains(stdout, "218") {
		t.Fatalf("submit help should use store extref 218: %s", stdout)
	}
	if !strings.Contains(stdout, "guest@example.test") {
		t.Fatalf("submit help should use placeholder email: %s", stdout)
	}
}

func TestHelpExamplesDropFakeUUID(t *testing.T) {
	for _, args := range [][]string{
		{"mapbox", "--help"},
		{"texasroadhouse", "checkin", "--help"},
		{"texasroadhouse", "get-status", "--help"},
	} {
		stdout, stderr, err := runRootArgs(t, args...)
		if err != nil {
			t.Fatalf("%v: %v (stderr=%q)", args, err, stderr)
		}
		if strings.Contains(stdout, "550e8400-e29b-41d4-a716-446655440000") {
			t.Fatalf("%v help still uses UUID mock: %s", args, stdout)
		}
	}
}

func TestCheckinHelpDescribesArrivalAndREMOVE(t *testing.T) {
	stdout, stderr, err := runRootArgs(t, "texasroadhouse", "checkin", "--help")
	if err != nil {
		t.Fatalf("checkin --help: %v (stderr=%q)", err, stderr)
	}
	if strings.Contains(strings.ToLower(stdout), "host stand") {
		t.Fatalf("checkin help must not direct guests to the host stand: %q", stdout)
	}
	if !strings.Contains(stdout, "HERE") {
		t.Fatalf("checkin help must describe the guest HERE text: %q", stdout)
	}
	if !strings.Contains(stdout, "once everyone has arrived") {
		t.Fatalf("checkin help must say once everyone has arrived: %q", stdout)
	}
	if !strings.Contains(stdout, "REMOVE") {
		t.Fatalf("checkin help must document SMS REMOVE: %q", stdout)
	}
}

func TestCancelHelpDocumentsSMSREMOVE(t *testing.T) {
	stdout, stderr, err := runRootArgs(t, "texasroadhouse", "cancel", "--help")
	if err != nil {
		t.Fatalf("cancel --help: %v (stderr=%q)", err, stderr)
	}
	if !strings.Contains(stdout, "REMOVE") {
		t.Fatalf("cancel help must document SMS REMOVE: %q", stdout)
	}
}

func TestMapboxHelpUsesZip(t *testing.T) {
	stdout, stderr, err := runRootArgs(t, "mapbox", "--help")
	if err != nil {
		t.Fatalf("mapbox --help: %v (stderr=%q)", err, stderr)
	}
	if !strings.Contains(stdout, "65804") {
		t.Fatalf("mapbox help should use zip 65804: %q", stdout)
	}
}

func writeGuestFile(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "guest.json")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func clearWaitlistGuestEnv(t *testing.T) {
	t.Helper()
	t.Setenv(waitlistGuestEmailEnv, "")
	t.Setenv(waitlistGuestFirstEnv, "")
	t.Setenv(waitlistGuestLastEnv, "")
	t.Setenv(waitlistGuestPhoneAreaEnv, "")
	t.Setenv(waitlistGuestPhoneNumEnv, "")
}

type failOnRead struct {
	t *testing.T
}

func (r failOnRead) Read([]byte) (int, error) {
	r.t.Fatal("stdin was read; --guest-file must not drain stdin")
	return 0, io.EOF
}

func TestGuestFileWinsOverEnv(t *testing.T) {
	clearWaitlistGuestEnv(t)
	t.Setenv(waitlistGuestEmailEnv, "env@example.test")
	t.Setenv(waitlistGuestFirstEnv, "EnvFirst")
	t.Setenv(waitlistGuestLastEnv, "EnvLast")
	t.Setenv(waitlistGuestPhoneAreaEnv, "999")
	t.Setenv(waitlistGuestPhoneNumEnv, "555-0199")

	path := writeGuestFile(t, `{"EmailAddress":"guest@example.test","FirstName":"Test","LastName":"User","PrimaryPhoneAreaCode":"555","PrimaryPhoneNumber":"555-0100"}`)
	cmd := &cobra.Command{Use: "submit"}
	cmd.SetIn(failOnRead{t: t})

	body, err := collectWaitlistGuestPII(cmd, &rootFlags{agent: true}, false, path, waitlistPIIFlagValues{})
	if err != nil {
		t.Fatalf("collectWaitlistGuestPII: %v", err)
	}
	if got, want := guestString(body["EmailAddress"]), "guest@example.test"; got != want {
		t.Fatalf("EmailAddress = %q, want file value %q (env must not override explicit --guest-file)", got, want)
	}
	if got, want := guestString(body["FirstName"]), "Test"; got != want {
		t.Fatalf("FirstName = %q, want file value %q", got, want)
	}
	if got, want := guestString(body["PrimaryPhoneNumber"]), "555-0100"; got != want {
		t.Fatalf("PrimaryPhoneNumber = %q, want file value %q", got, want)
	}
}

func TestEnvFillsEmptyGuestFileFields(t *testing.T) {
	clearWaitlistGuestEnv(t)
	t.Setenv(waitlistGuestEmailEnv, "guest@example.test")
	t.Setenv(waitlistGuestPhoneAreaEnv, "555")
	t.Setenv(waitlistGuestPhoneNumEnv, "555-0100")

	path := writeGuestFile(t, `{"FirstName":"Test","LastName":"User"}`)
	cmd := &cobra.Command{Use: "submit"}
	cmd.SetIn(failOnRead{t: t})

	body, err := collectWaitlistGuestPII(cmd, &rootFlags{agent: true}, false, path, waitlistPIIFlagValues{})
	if err != nil {
		t.Fatalf("collectWaitlistGuestPII: %v", err)
	}
	if got, want := guestString(body["FirstName"]), "Test"; got != want {
		t.Fatalf("FirstName = %q, want file value %q", got, want)
	}
	if got, want := guestString(body["EmailAddress"]), "guest@example.test"; got != want {
		t.Fatalf("EmailAddress = %q, want env fill %q", got, want)
	}
	if got, want := guestString(body["PrimaryPhoneNumber"]), "555-0100"; got != want {
		t.Fatalf("PrimaryPhoneNumber = %q, want env fill %q", got, want)
	}
}

func TestGuestFileDoesNotReadStdin(t *testing.T) {
	clearWaitlistGuestEnv(t)
	path := writeGuestFile(t, `{"EmailAddress":"guest@example.test","FirstName":"Test","LastName":"User","PrimaryPhoneAreaCode":"555","PrimaryPhoneNumber":"555-0100"}`)
	cmd := &cobra.Command{Use: "submit"}
	cmd.SetIn(failOnRead{t: t})

	body, err := collectWaitlistGuestPII(cmd, &rootFlags{agent: true}, false, path, waitlistPIIFlagValues{})
	if err != nil {
		t.Fatalf("collectWaitlistGuestPII: %v", err)
	}
	if got, want := guestString(body["EmailAddress"]), "guest@example.test"; got != want {
		t.Fatalf("EmailAddress = %q, want %q", got, want)
	}
}

func guestString(v any) string {
	s, _ := v.(string)
	return s
}
