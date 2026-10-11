// Copyright 2026 Greg Mushen and contributors. Licensed under Apache-2.0. See LICENSE.
// Implemented Neon command behavior tests.
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"fmt"
	"net/http"

	"bytes"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/cloud/neon/internal/cliutil/testenv"
)

// TestNovelWaitHelpWires smoke-tests that the wait command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelWaitHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"wait", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("wait --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "wait"} {
		if !strings.Contains(help, want) {
			t.Fatalf("wait --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNeonWaitPollingTypedFailures(t *testing.T) {
	for _, tc := range []struct {
		name string
		code int
	}{{"finish", 0}, {"failed", 6}, {"timeout", 8}, {"old-failure", 0}} {
		t.Run(tc.name, func(t *testing.T) {
			polls := 0
			neonTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/projects/p/branches" {
					fmt.Fprint(w, `{"branches":[{"id":"b","name":"preview"}]}`)
					return
				}
				polls++
				status := "running"
				if tc.name == "failed" && polls >= 2 {
					status = "failed" // seen running first, then failed during this wait
				}
				if strings.Contains(r.URL.Path, "/operations/") {
					if tc.name == "finish" && polls >= 2 {
						status = "finished"
					}
					fmt.Fprintf(w, `{"operation":{"id":"op","branch_id":"b","status":%q}}`, status)
					return
				}
				if tc.name == "finish" && polls >= 2 {
					status = "finished"
				}
				if tc.name == "old-failure" {
					// A failure from last month must not fail a wait on an idle branch.
					polls = 2
					fmt.Fprint(w, `{"operations":[{"id":"old","branch_id":"b","status":"failed","created_at":"2026-09-01T00:00:00Z"}]}`)
					return
				}
				fmt.Fprintf(w, `{"operations":[{"id":"op","branch_id":"b","status":%q}]}`, status)
			})
			_, err := executeNeon(t, newNovelWaitCmd, "--project-id", "p", "--branch", "preview", "--timeout", "50ms", "--interval", "2ms")
			if tc.code == 0 {
				if err != nil || polls != 2 {
					t.Fatalf("polls=%d err=%v", polls, err)
				}
			} else if err == nil || ExitCode(err) != tc.code {
				t.Fatalf("err=%v code=%d want=%d", err, ExitCode(err), tc.code)
			}
		})
	}
}

func TestNeonWaitAllProjects(t *testing.T) {
	polls := 0
	neonTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects":
			fmt.Fprint(w, `{"projects":[{"id":"p1"},{"id":"p2"}]}`)
		default:
			polls++
			status := "running"
			if polls > 1 {
				status = "finished"
			}
			if strings.Contains(r.URL.Path, "/operations/") {
				fmt.Fprintf(w, `{"operation":{"id":"op","branch_id":"any","status":%q}}`, status)
				return
			}
			fmt.Fprintf(w, `{"operations":[{"id":"op","branch_id":"any","status":%q}]}`, status)
		}
	})
	out, err := executeNeon(t, newNovelWaitCmd, "--scope", "all", "--timeout", "1s", "--interval", "2ms")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "p1") || !strings.Contains(out, "p2") || polls < 3 {
		t.Fatalf("polls=%d out=%s", polls, out)
	}
	if _, err := executeNeon(t, newNovelWaitCmd, "--all", "--branch", "x"); err == nil || ExitCode(err) != 2 {
		t.Fatalf("--all with --branch must be a usage error, got %v", err)
	}
}

func TestNeonWaitWalksFullHistoryThenTracksByID(t *testing.T) {
	listCalls, gets := 0, 0
	old := "2026-01-01T00:00:00Z"
	neonTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/projects/p/branches":
			fmt.Fprint(w, `{"branches":[{"id":"b","name":"preview"}]}`)
		case r.URL.Path == "/projects/p/operations/deep":
			gets++
			status := "running"
			if gets >= 2 {
				status = "finished"
			}
			fmt.Fprintf(w, `{"operation":{"id":"deep","branch_id":"b","status":%q}}`, status)
		case r.URL.Path == "/projects/p/operations":
			listCalls++
			switch r.URL.Query().Get("cursor") {
			case "":
				fmt.Fprintf(w, `{"operations":[{"id":"o1","branch_id":"other","status":"finished","created_at":%q}],"pagination":{"cursor":"c2"}}`, old)
			case "c2":
				// A long-running operation far down the list, older than any lookback.
				fmt.Fprintf(w, `{"operations":[{"id":"deep","branch_id":"b","status":"running","created_at":%q}],"pagination":{"cursor":"c3"}}`, old)
			default:
				fmt.Fprint(w, `{"operations":[],"pagination":{"cursor":"c3"}}`)
			}
		default:
			t.Errorf("unexpected %s", r.URL)
		}
	})
	if _, err := executeNeon(t, newNovelWaitCmd, "--project-id", "p", "--branch", "preview", "--timeout", "1s", "--interval", "2ms"); err != nil {
		t.Fatal(err)
	}
	if gets < 2 {
		t.Fatalf("deep running operation was not tracked to completion (gets=%d)", gets)
	}
}
