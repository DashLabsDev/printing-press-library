// Copyright 2026 Greg Mushen and contributors. Licensed under Apache-2.0. See LICENSE.
// Implemented Neon command behavior tests.
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"encoding/json"

	"bytes"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/cloud/neon/internal/cliutil/testenv"
)

// TestNovelComputeAuditHelpWires smoke-tests that the compute audit command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelComputeAuditHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"compute", "audit", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("compute audit --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "audit"} {
		if !strings.Contains(help, want) {
			t.Fatalf("compute audit --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNeonComputeAuditFindingsAndHealthy(t *testing.T) {
	for _, bad := range []bool{true, false} {
		min, max, suspend := 0.25, 1.0, 300
		if bad {
			min, max, suspend = 4, 4, -1
		}
		path := seedNeon(t, map[string][]neonObject{"branches": {{"id": "b", "project_id": "p", "default": false}}, "endpoints": {{"id": "e", "project_id": "p", "branch_id": "b", "suspend_timeout_seconds": suspend, "autoscaling_limit_min_cu": min, "autoscaling_limit_max_cu": max}}})
		out, err := executeNeon(t, newNovelComputeAuditCmd, "--db", path, "--max-cu", "2")
		if err != nil {
			t.Fatal(err)
		}
		var rows []computeFinding
		if err := json.Unmarshal([]byte(out), &rows); err != nil {
			t.Fatal(err, out)
		}
		want := 0
		if bad {
			want = 3
		}
		if len(rows) != want {
			t.Fatal(out)
		}
	}
}
