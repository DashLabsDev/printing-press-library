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

// TestNovelTreeHelpWires smoke-tests that the tree command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelTreeHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"tree", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("tree --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "tree"} {
		if !strings.Contains(help, want) {
			t.Fatalf("tree --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNeonTreeNestedBranch(t *testing.T) {
	path := seedNeon(t, map[string][]neonObject{"projects": {{"id": "p", "org_id": "o"}}, "branches": {{"id": "parent", "project_id": "p"}, {"id": "child", "project_id": "p", "parent_id": "parent"}}, "endpoints": {{"id": "e", "project_id": "p", "branch_id": "child"}}})
	out, err := executeNeon(t, newNovelTreeCmd, "--db", path)
	if err != nil {
		t.Fatal(err)
	}
	var nodes []*neonTreeNode
	if err := json.Unmarshal([]byte(out), &nodes); err != nil {
		t.Fatal(err, out)
	}
	branches := nodes[0].Children[0].Children
	if len(branches) != 1 || branches[0].ID != "parent" || branches[0].Children[0].ID != "child" || branches[0].Children[0].Children[0].ID != "e" {
		t.Fatal(out)
	}
}
