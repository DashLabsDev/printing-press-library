package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/cloud/neon/internal/client"
	"github.com/mvanhorn/printing-press-library/library/cloud/neon/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/cloud/neon/internal/store"
)

func seedNeon(t *testing.T, resources map[string][]neonObject) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "dev.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for kind, rows := range resources {
		for _, row := range rows {
			raw, err := json.Marshal(row)
			if err != nil {
				t.Fatal(err)
			}
			if err := db.Upsert(kind, ns(row, "id"), raw); err != nil {
				t.Fatal(err)
			}
		}
	}
	return path
}
func executeNeon(t *testing.T, makeCmd func(*rootFlags) *cobra.Command, args ...string) (string, error) {
	t.Helper()
	flags := &rootFlags{asJSON: true, noCache: true}
	cmd := makeCmd(flags)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), err
}

type neonHandlerTransport struct{ handler http.Handler }

func (h neonHandlerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if err := r.Context().Err(); err != nil {
		return nil, err
	}
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, r)
	return rec.Result(), nil
}
func neonTestServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	testenv.Isolate(t)
	t.Setenv("NEON_API_KEY", "test-only-neon-key")
	if os.Getenv("NEON_TEST_IN_MEMORY") == "1" {
		// Explicit sandbox fallback. Normal test runs use actual httptest servers.
		t.Setenv("NEON_BASE_URL", "http://neon-test.invalid")
		previous := clientHooks
		clientHooks = append(append([]func(*client.Client) error(nil), previous...), func(c *client.Client) error { c.HTTPClient.Transport = neonHandlerTransport{handler}; return nil })
		t.Cleanup(func() { clientHooks = previous })
		return
	}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	t.Setenv("NEON_BASE_URL", server.URL)
}

func TestNeonMissingMirrorSelectAndDryRun(t *testing.T) {
	testenv.Isolate(t)
	for _, makeCmd := range []func(*rootFlags) *cobra.Command{newNovelStaleCmd, newNovelTreeCmd, newNovelUsageTopCmd, newNovelUsageTrendCmd, newNovelComputeAuditCmd, newNovelPruneCmd} {
		path := filepath.Join(t.TempDir(), "absent.db")
		out, err := executeNeon(t, makeCmd, "--db", path)
		if err != nil || strings.TrimSpace(out) != "[]" {
			t.Fatalf("missing mirror %q %v", out, err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("created missing mirror %s", path)
		}
	}
	path := seedNeon(t, map[string][]neonObject{"branches": {{"id": "expired", "project_id": "p", "expires_at": time.Now().Add(-time.Hour).Format(time.RFC3339)}}})
	flags := &rootFlags{asJSON: true, selectFields: "branch_id"}
	cmd := newNovelStaleCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--db", path, "--idle", "60d"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || len(rows[0]) != 1 || rows[0]["branch_id"] != "expired" {
		t.Fatal(out.String())
	}
	for _, makeCmd := range []func(*rootFlags) *cobra.Command{newNovelWaitCmd, newNovelPreviewUpCmd, newNovelPruneCmd} {
		f := &rootFlags{asJSON: true, dryRun: true}
		c := makeCmd(f)
		c.Flags().Bool("dry-run", true, "test")
		c.SetArgs([]string{"--dry-run"})
		c.SetOut(io.Discard)
		c.SetErr(io.Discard)
		if err := c.Execute(); err != nil {
			t.Fatalf("dry-run attempted IO: %v", err)
		}
	}
}
