// Copyright 2026 Greg Mushen and contributors. Licensed under Apache-2.0. See LICENSE.
// Implemented Neon command behavior tests.
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"encoding/json"
	"time"

	"bytes"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/cloud/neon/internal/cliutil/testenv"
)

// TestNovelStaleHelpWires smoke-tests that the stale command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelStaleHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"stale", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("stale --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "stale"} {
		if !strings.Contains(help, want) {
			t.Fatalf("stale --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNeonStaleSelection(t *testing.T) {
	now := time.Now().UTC()
	old := now.Add(-30 * 24 * time.Hour).Format(time.RFC3339)
	path := seedNeon(t, map[string][]neonObject{"projects": {{"id": "p", "name": "Test"}}, "branches": {
		{"id": "idle", "project_id": "p", "state_changed_at": old},
		{"id": "active", "project_id": "p", "state_changed_at": now.Format(time.RFC3339)},
		{"id": "expired", "project_id": "p", "state_changed_at": now.Format(time.RFC3339), "expires_at": now.Add(-24 * time.Hour).Format(time.RFC3339)},
	}})
	for _, tc := range []struct {
		idle string
		want int
	}{{"14d", 2}, {"60d", 1}} {
		out, err := executeNeon(t, newNovelStaleCmd, "--db", path, "--idle", tc.idle)
		if err != nil {
			t.Fatal(err)
		}
		var rows []staleBranch
		if err := json.Unmarshal([]byte(out), &rows); err != nil {
			t.Fatal(err, out)
		}
		if len(rows) != tc.want {
			t.Fatalf("idle=%s rows=%s", tc.idle, out)
		}
	}
	empty := seedNeon(t, nil)
	out, err := executeNeon(t, newNovelStaleCmd, "--db", empty)
	if err != nil || strings.TrimSpace(out) != "[]" {
		t.Fatalf("empty %q %v", out, err)
	}
}
func TestNeonStaleEndpointPreference(t *testing.T) {
	now := time.Now()
	path := seedNeon(t, map[string][]neonObject{"branches": {{"id": "b", "project_id": "p", "updated_at": now.Add(-40 * 24 * time.Hour).Format(time.RFC3339)}}, "endpoints": {{"id": "e", "project_id": "p", "branch_id": "b", "last_active": now.Format(time.RFC3339)}}})
	out, err := executeNeon(t, newNovelStaleCmd, "--db", path, "--idle", "14d")
	if err != nil || strings.TrimSpace(out) != "[]" {
		t.Fatalf("endpoint override %s %v", out, err)
	}
}
