package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/cloud/neon/internal/cliutil/testenv"
)

func runNeonRoot(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := RootCmd()
	cmd.SetArgs(args)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	return out.String(), err
}

func TestNeonContextSetShowClear(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("NEON_CONTEXT_FILE", filepath.Join(t.TempDir(), "ctx.json"))
	if _, err := runNeonRoot(t, "context", "set", "--project-id", "lucky-sun-123456", "--json"); err != nil {
		t.Fatal(err)
	}
	if got := defaultNeonProject(""); got != "lucky-sun-123456" {
		t.Fatalf("saved project not used as default: %q", got)
	}
	if got := defaultNeonProject("explicit-1"); got != "explicit-1" {
		t.Fatalf("explicit project must win: %q", got)
	}
	out, err := runNeonRoot(t, "context", "show", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var shown map[string]any
	if err := json.Unmarshal([]byte(out), &shown); err != nil || shown["project_id"] != "lucky-sun-123456" {
		t.Fatalf("context show = %q (%v)", out, err)
	}
	if _, err := runNeonRoot(t, "context", "clear", "--json"); err != nil {
		t.Fatal(err)
	}
	if got := defaultNeonProject(""); got != "" {
		t.Fatalf("context not cleared: %q", got)
	}
}

func TestNeonContextSetRequiresInput(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("NEON_CONTEXT_FILE", filepath.Join(t.TempDir(), "ctx.json"))
	_, err := runNeonRoot(t, "context", "set", "--json")
	var ce *cliError
	if !errors.As(err, &ce) || ce.code != 2 {
		t.Fatalf("want usage error exit 2, got %v", err)
	}
}

func TestNeonConnectionStringNeedsProject(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("NEON_CONTEXT_FILE", filepath.Join(t.TempDir(), "ctx.json"))
	_, err := runNeonRoot(t, "connection-string", "pr-123", "--json")
	var ce *cliError
	if !errors.As(err, &ce) || ce.code != 2 || !strings.Contains(err.Error(), "project-id") {
		t.Fatalf("want usage error naming --project-id, got %v", err)
	}
	if out, err := runNeonRoot(t, "connection-string", "pr-123", "--project-id", "lucky-sun-123456", "--dry-run", "--json"); err != nil || !strings.Contains(out, "dry_run") {
		t.Fatalf("dry-run must short-circuit without network: %q %v", out, err)
	}
}
