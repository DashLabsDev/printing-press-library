package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/cloud/neon/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/cloud/neon/internal/store"
)

type neonObject map[string]any

func ns(o neonObject, key string) string  { s, _ := o[key].(string); return s }
func nf(o neonObject, key string) float64 { f, _ := o[key].(float64); return f }
func nb(o neonObject, key string) bool    { b, _ := o[key].(bool); return b }
func projectOf(o neonObject) string {
	for _, k := range []string{"project_id", "projects_id"} {
		if v := ns(o, k); v != "" {
			return v
		}
	}
	return ns(o, "parent_id")
}
func neonDuration(s string) (time.Duration, error) {
	d, e := cliutil.ParseDurationLoose(s)
	if e != nil || d <= 0 {
		return 0, usageErr(fmt.Errorf("duration must be positive: %q", s))
	}
	return d, nil
}
func neonMissing(cmd *cobra.Command, text string) error {
	_ = cmd.Usage()
	return usageErr(fmt.Errorf("%s", text))
}

// openNeonMirror never creates a missing mirror, including for prune previews.
func openNeonMirror(cmd *cobra.Command, flags *rootFlags, path, resource string) (*store.Store, error) {
	if path == "" {
		path = defaultDBPath("neon-pp-cli")
	}
	if _, err := os.Stat(path); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		fmt.Fprintln(cmd.ErrOrStderr(), "hint: missing local mirror; run: neon-pp-cli sync --db", path)
		return nil, nil
	}
	db, err := store.OpenReadOnlyContext(cmd.Context(), path)
	if err != nil {
		return nil, err
	}
	if !hintIfUnsynced(cmd, db, resource) {
		hintIfStale(cmd, db, resource, flags.maxAge)
	}
	return db, nil
}

// Read full JSON instead of nullable json_extract columns. Drain and close before
// callers query another resource (the SQLite store has a single connection).
func neonRows(db *store.Store, names ...string) ([]neonObject, error) {
	out := make([]neonObject, 0)
	if db == nil {
		return out, nil
	}
	placeholders := make([]string, len(names))
	args := make([]any, len(names))
	for i, n := range names {
		placeholders[i] = "?"
		args[i] = n
	}
	rows, err := db.DB().Query("SELECT data FROM resources WHERE resource_type IN ("+strings.Join(placeholders, ",")+") ORDER BY id", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var o neonObject
		if err := json.Unmarshal(raw, &o); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
func neonOutput(cmd *cobra.Command, flags *rootFlags, v any) error {
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		raw, err := json.Marshal(v)
		if err != nil {
			return err
		}
		var rows []map[string]any
		if json.Unmarshal(raw, &rows) == nil {
			if len(rows) == 0 {
				_, err = fmt.Fprintln(cmd.OutOrStdout(), "No results.")
				return err
			}
			return printAutoTable(cmd.OutOrStdout(), rows)
		}
	}
	return printJSONFiltered(cmd.OutOrStdout(), v, flags)
}

type staleBranch struct {
	ProjectID     string   `json:"project_id"`
	ProjectName   string   `json:"project_name"`
	BranchID      string   `json:"branch_id"`
	BranchName    string   `json:"branch_name"`
	LastActiveAt  string   `json:"last_active_at"`
	IdleDays      *float64 `json:"idle_days"`
	ExpiresAt     string   `json:"expires_at"`
	Default       bool     `json:"default"`
	Protected     bool     `json:"protected"`
	DeleteCommand string   `json:"delete_command,omitempty"`
}

func selectStale(db *store.Store, idle time.Duration, now time.Time) ([]staleBranch, error) {
	result := make([]staleBranch, 0)
	projects, err := neonRows(db, "projects")
	if err != nil {
		return nil, err
	}
	names := map[string]string{}
	for _, p := range projects {
		names[ns(p, "id")] = ns(p, "name")
	}
	branches, err := neonRows(db, "branches", "projects/branches", "projects_branches")
	if err != nil {
		return nil, err
	}
	endpoints, err := neonRows(db, "endpoints", "projects/endpoints", "projects_endpoints")
	if err != nil {
		return nil, err
	}
	active := map[string]time.Time{}
	for _, e := range endpoints {
		t, ok := parseRestSyncTimestamp(ns(e, "last_active"))
		key := projectOf(e) + "/" + ns(e, "branch_id")
		if ok && t.After(active[key]) {
			active[key] = t
		}
	}
	for _, b := range branches {
		p, id := projectOf(b), ns(b, "id")
		if p == "" || id == "" {
			continue
		}
		last := active[p+"/"+id]
		if last.IsZero() {
			for _, k := range []string{"state_changed_at", "updated_at"} {
				if t, ok := parseRestSyncTimestamp(ns(b, k)); ok {
					last = t
					break
				}
			}
		}
		expires, ok := parseRestSyncTimestamp(ns(b, "expires_at"))
		expired := ok && !expires.After(now)
		if !expired && (last.IsZero() || now.Sub(last) < idle) {
			continue
		}
		row := staleBranch{ProjectID: p, ProjectName: names[p], BranchID: id, BranchName: ns(b, "name"), ExpiresAt: ns(b, "expires_at"), Default: nb(b, "default"), Protected: nb(b, "protected")}
		if !row.Default && !row.Protected {
			row.DeleteCommand = fmt.Sprintf("neon-pp-cli projects branches delete-project-branch %s %s", p, id)
		}
		if !last.IsZero() {
			row.LastActiveAt = last.UTC().Format(time.RFC3339)
			days := now.Sub(last).Hours() / 24
			row.IdleDays = &days
		}
		result = append(result, row)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].ProjectID != result[j].ProjectID {
			return result[i].ProjectID < result[j].ProjectID
		}
		return result[i].BranchID < result[j].BranchID
	})
	return result, nil
}
