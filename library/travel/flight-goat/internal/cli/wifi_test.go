// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// Tests for the `wifi` (SeatWifi) command group.

package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func runWifiCmd(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	flags := &rootFlags{}
	cmd := newWifiCmd(flags)
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return out.String(), errb.String(), err
}

func TestWifiCmd_HelpListsSubcommands(t *testing.T) {
	stdout, _, err := runWifiCmd(t, "--help")
	if err != nil {
		t.Fatalf("--help: %v", err)
	}
	for _, want := range []string{"flight", "airline", "airlines", "rollouts", "speed", "airline-speed", "search"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("help missing %q", want)
		}
	}
}

func TestWifiFlight_DryRun(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newWifiCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"flight", "UA1234"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "/api/v1/flights/UA1234") {
		t.Errorf("dry-run should contain flight path, got: %s", s)
	}
	if !strings.Contains(s, "dry_run") && !strings.Contains(s, "dry run") {
		t.Errorf("dry-run marker missing: %s", s)
	}
}

func TestWifiAirline_DryRun(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newWifiCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"airline", "UA"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "/api/airlines/UA") {
		t.Errorf("got %q", out.String())
	}
}

func TestWifiAirlines_DryRun(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newWifiCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"airlines"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "/api/airlines") {
		t.Errorf("got %q", out.String())
	}
}

func TestWifiRollouts_DryRun(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newWifiCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"rollouts", "UA"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "/api/rollouts/UA") {
		t.Errorf("got %q", out.String())
	}
}

func TestWifiRollouts_All_DryRun(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newWifiCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"rollouts"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "/api/rollouts") {
		t.Errorf("got %q", out.String())
	}
}

func TestWifiSpeed_DryRun(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newWifiCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"speed", "UA1234"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "/api/speed-reports/stats/UA1234") {
		t.Errorf("got %q", out.String())
	}
}

func TestWifiSearch_DryRun(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newWifiCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"search", "united"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "/api/search") {
		t.Errorf("got %q", out.String())
	}
}

func TestWifiAirlineSpeed_DryRun(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newWifiCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"airline-speed", "UA"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "/api/speed-reports/airline/UA") {
		t.Errorf("got %q", out.String())
	}
}

func TestWifiFlight_DryRunQuiet(t *testing.T) {
	flags := &rootFlags{dryRun: true, quiet: true}
	cmd := newWifiCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"flight", "UA1234"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("quiet dry-run should emit nothing, got %q", out.String())
	}
}

func TestWifiFlight_DryRunJSON(t *testing.T) {
	flags := &rootFlags{dryRun: true, asJSON: true}
	cmd := newWifiCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"flight", "UA1234"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := out.String()
	if strings.Contains(got, "dry run - no request sent") {
		t.Fatalf("json dry-run leaked human text: %s", got)
	}
	if !strings.Contains(got, `"dry_run"`) || !strings.Contains(got, `/api/v1/flights/UA1234`) {
		t.Fatalf("json dry-run missing payload, got: %s", got)
	}
}

func TestWifiFlight_RequiresArg(t *testing.T) {
	_, _, err := runWifiCmd(t, "flight")
	if err == nil {
		t.Fatal("expected error for missing flight arg")
	}
}

func TestWifiSearch_RequiresArg(t *testing.T) {
	_, _, err := runWifiCmd(t, "search")
	if err == nil {
		t.Fatal("expected error for missing search arg")
	}
}

func TestWifiFlight_WhitespaceArgRejectedBeforeDryRun(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newWifiCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"flight", "  "})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for whitespace-only flight")
	}
	if !strings.Contains(err.Error(), "flight number required") {
		t.Fatalf("error = %v, want flight number required", err)
	}
	got := out.String()
	if strings.Contains(got, "dry run") || strings.Contains(got, "dry_run") || strings.Contains(got, "/api/v1/flights/") {
		t.Fatalf("should not print a dry-run URL, got %q", got)
	}
}

func TestWifiSearch_WhitespaceArgRejectedBeforeDryRun(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newWifiCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"search", "  "})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for whitespace-only search query")
	}
	if !strings.Contains(err.Error(), "query required") {
		t.Fatalf("error = %v, want query required", err)
	}
	got := out.String()
	if strings.Contains(got, "dry run") || strings.Contains(got, "dry_run") || strings.Contains(got, "/api/search") {
		t.Fatalf("should not print a dry-run URL, got %q", got)
	}
}

func TestWifiSearch_DryRunEncodesQuery(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newWifiCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"search", "foo&bar"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "q=foo%26bar") {
		t.Errorf("dry-run should query-escape reserved chars, got: %s", got)
	}
	if strings.Contains(got, "q=foo&bar") {
		t.Errorf("dry-run leaked unescaped query: %s", got)
	}
}

func TestWifiFlight_DryRunPathEscapes(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newWifiCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"flight", "ua/1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "/api/v1/flights/UA%2F1") {
		t.Errorf("dry-run should path-escape flight, got: %s", got)
	}
}

func TestWifiMachineFlagsSkipHumanTable(t *testing.T) {
	// Greptile: --csv/--quiet/--select/--compact must not pick the TTY table.
	cases := map[string]*rootFlags{
		"csv":     {csv: true},
		"quiet":   {quiet: true},
		"compact": {compact: true},
		"plain":   {plain: true},
		"select":  {selectFields: "code"},
		"json":    {asJSON: true},
	}
	for name, flags := range cases {
		if wantsHumanTable(os.Stdout, flags) {
			t.Errorf("%s: still chose human table on TTY", name)
		}
	}
}
