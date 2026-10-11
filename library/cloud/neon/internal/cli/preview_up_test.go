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

// TestNovelPreviewUpHelpWires smoke-tests that the preview up command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelPreviewUpHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"preview", "up", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("preview up --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "up"} {
		if !strings.Contains(help, want) {
			t.Fatalf("preview up --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNeonPreviewUpIdempotentAndCreate(t *testing.T) {
	for _, exists := range []bool{true, false} {
		t.Run(fmt.Sprint(exists), func(t *testing.T) {
			creates := 0
			neonTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == "POST":
					creates++
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					branch := body["branch"].(map[string]any)
					expires, err := time.Parse(time.RFC3339, branch["expires_at"].(string))
					if err != nil || expires.Before(time.Now().Add(6*24*time.Hour)) {
						t.Error("missing future expires_at", branch)
					}
					endpoints := body["endpoints"].([]any)
					if endpoints[0].(map[string]any)["type"] != "read_write" {
						t.Error(body)
					}
					fmt.Fprint(w, `{"branch":{"id":"b"},"operations":[{"id":"op"}]}`)
				case strings.HasSuffix(r.URL.Path, "/branches"):
					if exists {
						fmt.Fprint(w, `{"branches":[{"id":"b","name":"preview"}]}`)
					} else {
						fmt.Fprint(w, `{"branches":[]}`)
					}
				case strings.Contains(r.URL.Path, "/operations/op"):
					fmt.Fprint(w, `{"operation":{"id":"op","branch_id":"b","status":"finished"}}`)
				case strings.HasSuffix(r.URL.Path, "/operations"):
					fmt.Fprint(w, `{"operations":[]}`)
				case strings.HasSuffix(r.URL.Path, "/databases"):
					fmt.Fprint(w, `{"databases":[{"name":"neondb","owner_name":"neondb_owner"}]}`)
				case strings.HasSuffix(r.URL.Path, "/connection_uri"):
					fmt.Fprint(w, `{"uri":"postgres://test:test@localhost/neondb"}`)
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
					w.WriteHeader(404)
				}
			})
			out, err := executeNeon(t, newNovelPreviewUpCmd, "preview", "--project-id", "p", "--expires", "7d")
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out, "postgres://test:test@localhost/neondb") {
				t.Fatal(out)
			}
			want := 1
			if exists {
				want = 0
			}
			if creates != want {
				t.Fatalf("creates=%d want=%d", creates, want)
			}
		})
	}
}

func TestNeonPreviewHumanEnvLine(t *testing.T) {
	neonTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/branches"):
			fmt.Fprint(w, `{"branches":[{"id":"b","name":"preview"}]}`)
		case strings.HasSuffix(r.URL.Path, "/operations"):
			fmt.Fprint(w, `{"operations":[]}`)
		case strings.HasSuffix(r.URL.Path, "/databases"):
			fmt.Fprint(w, `{"databases":[{"name":"neondb","owner_name":"neondb_owner"}]}`)
		case strings.HasSuffix(r.URL.Path, "/connection_uri"):
			fmt.Fprint(w, `{"uri":"postgres://test:test@localhost/neondb"}`)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	previous := humanFriendly
	humanFriendly = true
	defer func() { humanFriendly = previous }()
	cmd := newNovelPreviewUpCmd(&rootFlags{noCache: true})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"preview", "--project-id", "p"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if out.String() != "DATABASE_URL=postgres://test:test@localhost/neondb\n" {
		t.Fatal(out.String())
	}
}

func TestNeonPreviewUpConflictWaitsOnlyOnThisBranch(t *testing.T) {
	for _, otherFailed := range []bool{false, true} {
		t.Run(fmt.Sprint(otherFailed), func(t *testing.T) {
			listed := 0
			neonTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == "POST":
					w.WriteHeader(409)
					fmt.Fprint(w, `{"message":"branch already exists"}`)
				case strings.HasSuffix(r.URL.Path, "/branches"):
					listed++
					if listed == 1 {
						fmt.Fprint(w, `{"branches":[]}`)
						return
					}
					fmt.Fprint(w, `{"branches":[{"id":"b","name":"preview","created_at":"2026-10-10T12:00:00Z"}]}`)
				case strings.Contains(r.URL.Path, "/operations/"):
					id := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
					status := "finished"
					if id == "mine" && otherFailed {
						status = "failed"
					}
					fmt.Fprintf(w, `{"operation":{"id":%q,"branch_id":"b","status":%q}}`, id, status)
				case strings.HasSuffix(r.URL.Path, "/operations"):
					mine := "running"
					if otherFailed {
						mine = "failed"
					}
					fmt.Fprintf(w, `{"operations":[
						{"id":"elsewhere","branch_id":"other","status":"running","created_at":"2026-10-10T12:00:01Z"},
						{"id":"mine","branch_id":"b","status":%q,"created_at":"2026-10-10T12:00:00Z"},
						{"id":"old","branch_id":"b","status":"failed","created_at":"2026-10-01T00:00:00Z"}]}`, mine)
				case strings.HasSuffix(r.URL.Path, "/databases"):
					fmt.Fprint(w, `{"databases":[{"name":"neondb","owner_name":"neondb_owner"}]}`)
				case strings.HasSuffix(r.URL.Path, "/connection_uri"):
					fmt.Fprint(w, `{"uri":"postgres://test:test@localhost/neondb"}`)
				default:
					t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
				}
			})
			_, err := executeNeon(t, newNovelPreviewUpCmd, "preview", "--project-id", "p")
			if otherFailed {
				if err == nil || ExitCode(err) != 6 {
					t.Fatalf("a failed concurrent create must exit 6, got %v", err)
				}
			} else if err != nil {
				t.Fatalf("other branches' and old operations must be ignored: %v", err)
			}
		})
	}
}
