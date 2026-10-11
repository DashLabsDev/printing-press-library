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

// TestNovelUsageTopHelpWires smoke-tests that the usage top command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelUsageTopHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"usage", "top", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("usage top --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "top"} {
		if !strings.Contains(help, want) {
			t.Fatalf("usage top --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNeonUsageRankingAndWindow(t *testing.T) {
	now := time.Now().UTC()
	path := seedNeon(t, map[string][]neonObject{"consumption-history-projects": {
		{"id": "a", "project_id": "a", "timeframe_start": now.Add(-24 * time.Hour).Format(time.RFC3339), "compute_time_seconds": 100, "written_data_bytes": 1},
		{"id": "b", "project_id": "b", "timeframe_start": now.Add(-24 * time.Hour).Format(time.RFC3339), "compute_time_seconds": 10, "written_data_bytes": 200},
		{"id": "old", "project_id": "old", "timeframe_start": now.Add(-30 * 24 * time.Hour).Format(time.RFC3339), "compute_time_seconds": 1000, "written_data_bytes": 1000},
	}})
	for _, tc := range []struct{ metric, first string }{{"compute", "a"}, {"written", "b"}} {
		out, err := executeNeon(t, newNovelUsageTopCmd, "--db", path, "--metric", tc.metric, "--since", "7d")
		if err != nil {
			t.Fatal(err)
		}
		var rows []neonUsageRank
		if err := json.Unmarshal([]byte(out), &rows); err != nil {
			t.Fatal(err, out)
		}
		if len(rows) != 2 || rows[0].ProjectID != tc.first {
			t.Fatalf("%s: %s", tc.metric, out)
		}
	}
}
