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

// TestNovelUsageTrendHelpWires smoke-tests that the usage trend command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelUsageTrendHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"usage", "trend", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("usage trend --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "trend"} {
		if !strings.Contains(help, want) {
			t.Fatalf("usage trend --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNeonUsageTrendNoFabricatedDelta(t *testing.T) {
	now := time.Now().UTC()
	path := seedNeon(t, map[string][]neonObject{"consumption-history-projects": {{"id": "a", "project_id": "a", "timeframe_start": now.Add(-24 * time.Hour).Format(time.RFC3339), "compute_time_seconds": 100}}})
	out, err := executeNeon(t, newNovelUsageTrendCmd, "--db", path, "--since", "30d")
	if err != nil {
		t.Fatal(err)
	}
	var rows []neonUsageDelta
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatal(err, out)
	}
	if len(rows) != 1 || rows[0].Delta != nil {
		t.Fatalf("rows=%s", out)
	}
}
func TestNeonUsageTrendConsecutivePeriods(t *testing.T) {
	now := time.Now().UTC()
	start := now.Add(-48 * time.Hour).Format(time.RFC3339)
	middle := now.Add(-24 * time.Hour).Format(time.RFC3339)
	points := []neonUsagePoint{{ProjectID: "p", Start: start, End: middle, Values: map[string]float64{"compute": 10}}, {ProjectID: "p", Start: middle, End: now.Format(time.RFC3339), Values: map[string]float64{"compute": 15}}}
	rows := trendNeonUsage(points, now.Add(-72*time.Hour), now)
	if len(rows) != 2 || rows[1].Delta == nil || *rows[1].Delta != 5 {
		t.Fatalf("%+v", rows)
	}
}
