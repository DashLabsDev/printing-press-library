// Copyright 2026 Greg Mushen and contributors. Licensed under Apache-2.0. See LICENSE.
// Implemented Neon command behavior tests.
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"bytes"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/cloud/neon/internal/cliutil/testenv"
)

// TestNovelPruneHelpWires smoke-tests that the prune command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelPruneHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"prune", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("prune --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "prune"} {
		if !strings.Contains(help, want) {
			t.Fatalf("prune --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNeonPrunePreviewAndDelete(t *testing.T) {
	calls, deletes := 0, make([]string, 0)
	neonTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method == "DELETE" {
			deletes = append(deletes, r.URL.Path)
			fmt.Fprint(w, `{}`)
			return
		}
		fmt.Fprintf(w, `{"branch":{"id":"idle","default":false,"protected":false,"updated_at":%q}}`, time.Now().Add(-30*24*time.Hour).Format(time.RFC3339))
	})
	old := time.Now().Add(-30 * 24 * time.Hour).Format(time.RFC3339)
	path := seedNeon(t, map[string][]neonObject{"branches": {{"id": "idle", "project_id": "p", "updated_at": old}, {"id": "default", "project_id": "p", "updated_at": old, "default": true}, {"id": "protected", "project_id": "p", "updated_at": old, "protected": true}}})
	if _, err := executeNeon(t, newNovelPruneCmd, "--db", path, "--idle", "14d"); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("preview made %d calls", calls)
	}
	if _, err := executeNeon(t, newNovelPruneCmd, "--db", path, "--idle", "14d", "--yes"); err != nil {
		t.Fatal(err)
	}
	if len(deletes) != 1 || deletes[0] != "/projects/p/branches/idle" {
		t.Fatalf("deletes=%v", deletes)
	}
}

func TestNeonPruneLiveRecheck(t *testing.T) {
	deletes := make([]string, 0)
	now := time.Now().UTC().Format(time.RFC3339)
	neonTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "DELETE":
			deletes = append(deletes, r.URL.Path)
			fmt.Fprint(w, `{}`)
		case r.URL.Path == "/projects/p/branches/gone":
			w.WriteHeader(404)
			fmt.Fprint(w, `{"message":"not found"}`)
		case r.URL.Path == "/projects/p/branches/revived/endpoints":
			fmt.Fprintf(w, `{"endpoints":[{"id":"ep","last_active":%q}]}`, now)
		case strings.HasSuffix(r.URL.Path, "/endpoints"):
			fmt.Fprint(w, `{"endpoints":[]}`)
		default:
			id := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
			fmt.Fprintf(w, `{"branch":{"id":%q,"updated_at":%q}}`, id, time.Now().Add(-30*24*time.Hour).Format(time.RFC3339))
		}
	})
	old := time.Now().Add(-30 * 24 * time.Hour).Format(time.RFC3339)
	path := seedNeon(t, map[string][]neonObject{"branches": {
		{"id": "gone", "project_id": "p", "updated_at": old},
		{"id": "idle", "project_id": "p", "updated_at": old},
		{"id": "revived", "project_id": "p", "updated_at": old},
	}})
	out, err := executeNeon(t, newNovelPruneCmd, "--db", path, "--idle", "14d", "--yes")
	if err != nil {
		t.Fatal(err)
	}
	if len(deletes) != 1 || deletes[0] != "/projects/p/branches/idle" {
		t.Fatalf("only the still-idle branch may be deleted; deletes=%v", deletes)
	}
	var results []map[string]any
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Fatalf("output %q: %v", out, err)
	}
	status := map[string]string{}
	for _, r := range results {
		status[r["branch_id"].(string)] = r["status"].(string)
	}
	if status["gone"] != "gone" || status["idle"] != "deleted" || status["revived"] != "skipped_active" {
		t.Fatalf("statuses=%v", status)
	}
}

func TestNeonPruneSkipsWithoutActivityEvidence(t *testing.T) {
	deletes := 0
	neonTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "DELETE":
			deletes++
		case strings.HasSuffix(r.URL.Path, "/endpoints"):
			fmt.Fprint(w, `{"endpoints":[]}`)
		default:
			fmt.Fprint(w, `{"branch":{"id":"x"}}`) // expiry removed, no timestamps
		}
	})
	path := seedNeon(t, map[string][]neonObject{"branches": {{"id": "x", "project_id": "p", "expires_at": time.Now().Add(-time.Hour).Format(time.RFC3339)}}})
	if _, err := executeNeon(t, newNovelPruneCmd, "--db", path, "--idle", "14d", "--yes"); err != nil {
		t.Fatal(err)
	}
	if deletes != 0 {
		t.Fatalf("deleted a branch with no live idle evidence")
	}
}
