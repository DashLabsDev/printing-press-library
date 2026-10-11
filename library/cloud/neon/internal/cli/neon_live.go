package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/cloud/neon/internal/client"
	"github.com/mvanhorn/printing-press-library/library/cloud/neon/internal/cliutil"
)

func neonProjectPath(project string) string {
	return replacePathParam("/projects/{project_id}", "project_id", project)
}
func neonBranchPath(project, branch string) string {
	return replacePathParam(neonProjectPath(project)+"/branches/{branch_id}", "branch_id", branch)
}

// All list reads drain pagination, reject malformed envelopes and repeated cursors.
func neonLiveList(ctx context.Context, c *client.Client, path, key string, params map[string]string) ([]neonObject, error) {
	out := make([]neonObject, 0)
	query := map[string]string{}
	for k, v := range params {
		query[k] = v
	}
	query["limit"] = "100"
	seen := map[string]bool{}
	for {
		raw, err := c.GetNoCache(ctx, path, query)
		if err != nil {
			return nil, err
		}
		var env map[string]json.RawMessage
		if err := json.Unmarshal(raw, &env); err != nil {
			return nil, err
		}
		list, ok := env[key]
		if !ok {
			return nil, fmt.Errorf("missing %s in response", key)
		}
		var rows []neonObject
		if err := json.Unmarshal(list, &rows); err != nil {
			return nil, err
		}
		out = append(out, rows...)
		var page struct {
			Cursor string `json:"cursor"`
		}
		if raw, ok := env["pagination"]; ok {
			if err := json.Unmarshal(raw, &page); err != nil {
				return nil, err
			}
		}
		// Neon echoes the last cursor on an empty final page.
		if page.Cursor == "" || len(rows) == 0 {
			break
		}
		if seen[page.Cursor] {
			return nil, fmt.Errorf("repeated pagination cursor for %s", key)
		}
		seen[page.Cursor] = true
		query["cursor"] = page.Cursor
	}
	return out, nil
}
func resolveNeonBranch(ctx context.Context, c *client.Client, project, branch string) (neonObject, error) {
	rows, err := neonLiveList(ctx, c, neonProjectPath(project)+"/branches", "branches", nil)
	if err != nil {
		return nil, err
	}
	var match neonObject
	for _, b := range rows {
		if ns(b, "id") == branch {
			return b, nil
		}
		if ns(b, "name") == branch {
			if match != nil {
				return nil, fmt.Errorf("ambiguous branch name %q", branch)
			}
			match = b
		}
	}
	if match == nil {
		return nil, notFoundErr(fmt.Errorf("branch %q not found in project %s", branch, project))
	}
	return match, nil
}

// Wait errors use the CLI's typed error mechanism: 6=failed, 8=timeout.
func neonWaitError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return &cliError{code: 8, err: fmt.Errorf("operation wait timed out: %w", err)}
	}
	return err
}
func waitNeonOperations(ctx context.Context, c *client.Client, project, branch string, ids []string, timeout, interval time.Duration) error {
	if cliutil.IsDogfoodEnv() && timeout > 20*time.Second {
		timeout = 20 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	started := time.Now().Add(-5 * time.Second)
	seenPending := map[string]bool{}
	for {
		var operations []neonObject
		if len(ids) > 0 {
			operations = make([]neonObject, 0, len(ids))
			for _, id := range ids {
				raw, err := c.GetNoCache(ctx, replacePathParam(neonProjectPath(project)+"/operations/{operation_id}", "operation_id", id), nil)
				if err != nil {
					return neonWaitError(err)
				}
				var env struct {
					Operation neonObject `json:"operation"`
				}
				if err := json.Unmarshal(raw, &env); err != nil {
					return err
				}
				if env.Operation == nil || ns(env.Operation, "id") != id {
					return fmt.Errorf("missing operation in response")
				}
				operations = append(operations, env.Operation)
			}
		} else {
			return waitNeonUntracked(ctx, c, project, branch, started, interval)
		}
		pending := false
		for _, op := range operations {
			if branch != "" && ns(op, "branch_id") != "" && ns(op, "branch_id") != branch {
				if len(ids) > 0 {
					return usageErr(fmt.Errorf("operation %s belongs to a different branch", ns(op, "id")))
				}
				continue
			}
			opID := ns(op, "id")
			switch ns(op, "status") {
			case "finished", "skipped":
			case "failed", "error", "cancelled":
				// Without explicit IDs, only failures this wait observed (or that
				// started after it began) count; old history must not fail it.
				if len(ids) > 0 || seenPending[opID] || neonOpStartedSince(op, started) {
					return &cliError{code: 6, err: fmt.Errorf("operation %s %s: %s", opID, ns(op, "status"), ns(op, "error"))}
				}
			case "running", "scheduling", "cancelling":
				pending = true
				seenPending[opID] = true
			default:
				if len(ids) > 0 {
					return fmt.Errorf("unknown operation status %q", ns(op, "status"))
				}
			}
		}
		if !pending {
			return nil
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return neonWaitError(ctx.Err())
		case <-timer.C:
		}
	}
}

type pruneResult struct {
	staleBranch
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// deleteNeonBranches re-checks each target live (identity, default/protected,
// expiry and endpoint activity) before deleting, so a stale local mirror cannot
// delete a branch that became active again. It returns one result per row, even
// when it stops early on an error.
func deleteNeonBranches(cmd *cobra.Command, flags *rootFlags, rows []staleBranch, idle time.Duration, now time.Time) ([]pruneResult, error) {
	results := make([]pruneResult, 0, len(rows))
	c, err := flags.newClient()
	if err != nil {
		return results, err
	}
	ctx := cmd.Context()
	for _, row := range rows {
		res := pruneResult{staleBranch: row}
		raw, err := c.GetNoCache(ctx, neonBranchPath(row.ProjectID, row.BranchID), nil)
		if neonNotFound(err) {
			res.Status = "gone"
			results = append(results, res)
			continue
		}
		if err != nil {
			res.Status, res.Detail = "error", err.Error()
			return append(results, res), err
		}
		var env struct {
			Branch neonObject `json:"branch"`
		}
		if err := json.Unmarshal(raw, &env); err != nil {
			return results, err
		}
		if ns(env.Branch, "id") != row.BranchID {
			err := fmt.Errorf("branch identity changed before delete")
			res.Status, res.Detail = "error", err.Error()
			return append(results, res), err
		}
		if nb(env.Branch, "default") || nb(env.Branch, "protected") {
			res.Status = "skipped_protected"
			results = append(results, res)
			continue
		}
		expires, ok := parseRestSyncTimestamp(ns(env.Branch, "expires_at"))
		expired := ok && !expires.After(now)
		if !expired {
			last, err := neonBranchLastActive(ctx, c, row.ProjectID, row.BranchID)
			if err != nil {
				res.Status, res.Detail = "error", err.Error()
				return append(results, res), err
			}
			// A recently updated branch (e.g. its expiry was extended) counts
			// as active; with no activity evidence at all, do not delete.
			if t, ok := parseRestSyncTimestamp(ns(env.Branch, "updated_at")); ok && t.After(last) {
				last = t
			}
			if last.IsZero() {
				res.Status, res.Detail = "skipped_active", "no live activity timestamp; not deleting"
				results = append(results, res)
				continue
			}
			if now.Sub(last) < idle {
				res.Status, res.Detail = "skipped_active", "active at "+last.UTC().Format(time.RFC3339)+" (local mirror was stale)"
				results = append(results, res)
				continue
			}
		}
		if _, _, err := c.Delete(ctx, neonBranchPath(row.ProjectID, row.BranchID)); err != nil {
			if neonNotFound(err) {
				res.Status = "gone"
				results = append(results, res)
				continue
			}
			res.Status, res.Detail = "error", err.Error()
			return append(results, res), err
		}
		res.Status = "deleted"
		results = append(results, res)
	}
	return results, nil
}

// neonBranchLastActive returns the most recent endpoint activity for a branch.
func neonBranchLastActive(ctx context.Context, c *client.Client, project, branch string) (time.Time, error) {
	var last time.Time
	raw, err := c.GetNoCache(ctx, neonBranchPath(project, branch)+"/endpoints", nil)
	if err != nil {
		return last, err
	}
	var env struct {
		Endpoints []neonObject `json:"endpoints"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return last, err
	}
	for _, e := range env.Endpoints {
		for _, k := range []string{"last_active", "updated_at"} {
			if t, ok := parseRestSyncTimestamp(ns(e, k)); ok && t.After(last) {
				last = t
			}
		}
	}
	return last, nil
}

// neonOperationsUntil walks project operations newest first, following every
// cursor, and stops early only when stop returns true for an operation (that
// operation is not included). Neon ignores branch filters on this endpoint.
func neonOperationsUntil(ctx context.Context, c *client.Client, project string, stop func(neonObject) bool) ([]neonObject, error) {
	out := make([]neonObject, 0)
	query := map[string]string{"limit": "100"}
	seen := map[string]bool{}
	for {
		raw, err := c.GetNoCache(ctx, neonProjectPath(project)+"/operations", query)
		if err != nil {
			return nil, err
		}
		var env struct {
			Operations []neonObject `json:"operations"`
			Pagination struct {
				Cursor string `json:"cursor"`
			} `json:"pagination"`
		}
		if err := json.Unmarshal(raw, &env); err != nil {
			return nil, err
		}
		for _, op := range env.Operations {
			if stop != nil && stop(op) {
				return out, nil
			}
			out = append(out, op)
		}
		next := env.Pagination.Cursor
		if len(env.Operations) == 0 || next == "" || seen[next] {
			return out, nil
		}
		seen[next] = true
		query["cursor"] = next
	}
}

// neonRecentOperations returns every operation created at or after since.
func neonRecentOperations(ctx context.Context, c *client.Client, project string, since time.Time) ([]neonObject, error) {
	return neonOperationsUntil(ctx, c, project, func(op neonObject) bool {
		t, ok := parseRestSyncTimestamp(ns(op, "created_at"))
		return ok && t.Before(since)
	})
}

// waitNeonUntracked waits for a branch (or, with branch "", a whole project)
// to have no in-flight operations. The first pass walks the full operation
// history; later passes re-check tracked operations by ID and read only the
// new operations at the head of the list, so no page is ever skipped.
func waitNeonUntracked(ctx context.Context, c *client.Client, project, branch string, started time.Time, interval time.Duration) error {
	known := map[string]bool{}
	tracked := map[string]bool{}
	first := true
	for {
		var ops []neonObject
		var err error
		if first {
			ops, err = neonOperationsUntil(ctx, c, project, nil)
			first = false
		} else {
			ops, err = neonOperationsUntil(ctx, c, project, func(op neonObject) bool { return known[ns(op, "id")] })
		}
		if err != nil {
			return neonWaitError(err)
		}
		for _, op := range ops {
			id := ns(op, "id")
			known[id] = true
			if branch != "" && ns(op, "branch_id") != "" && ns(op, "branch_id") != branch {
				continue
			}
			switch ns(op, "status") {
			case "running", "scheduling", "cancelling":
				tracked[id] = true
			case "failed", "error", "cancelled":
				// Old failures in history must not fail the wait.
				if neonOpStartedSince(op, started) {
					return &cliError{code: 6, err: fmt.Errorf("operation %s %s: %s", id, ns(op, "status"), ns(op, "error"))}
				}
			}
		}
		for id := range tracked {
			raw, err := c.GetNoCache(ctx, replacePathParam(neonProjectPath(project)+"/operations/{operation_id}", "operation_id", id), nil)
			if err != nil {
				return neonWaitError(err)
			}
			var env struct {
				Operation neonObject `json:"operation"`
			}
			if err := json.Unmarshal(raw, &env); err != nil {
				return err
			}
			switch ns(env.Operation, "status") {
			case "finished", "skipped":
				delete(tracked, id)
			case "failed", "error", "cancelled":
				return &cliError{code: 6, err: fmt.Errorf("operation %s %s: %s", id, ns(env.Operation, "status"), ns(env.Operation, "error"))}
			}
		}
		if len(tracked) == 0 {
			return nil
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return neonWaitError(ctx.Err())
		case <-timer.C:
		}
	}
}

func neonNotFound(err error) bool {
	var apiErr *client.APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == 404
}

func neonOpStartedSince(op neonObject, since time.Time) bool {
	t, ok := parseRestSyncTimestamp(ns(op, "created_at"))
	return ok && !t.Before(since)
}

// neonConnectionURI fetches a connection URI, inferring the database and its
// owner role when the branch has exactly one database.
func neonConnectionURI(ctx context.Context, c *client.Client, project, branchID, database, role string, pooled bool) (string, string, string, error) {
	if database == "" || role == "" {
		dbs, err := neonLiveList(ctx, c, neonBranchPath(project, branchID)+"/databases", "databases", nil)
		if err != nil {
			return "", "", "", err
		}
		if database == "" {
			if len(dbs) != 1 {
				return "", "", "", usageErr(fmt.Errorf("branch has %d databases; pass --database", len(dbs)))
			}
			database = ns(dbs[0], "name")
		}
		if role == "" {
			for _, d := range dbs {
				if ns(d, "name") == database {
					role = ns(d, "owner_name")
				}
			}
			if role == "" {
				return "", "", "", usageErr(fmt.Errorf("could not infer the database owner role; pass --role"))
			}
		}
	}
	params := map[string]string{"branch_id": branchID, "pooled": fmt.Sprint(pooled), "database_name": database, "role_name": role}
	raw, err := c.GetNoCache(ctx, neonProjectPath(project)+"/connection_uri", params)
	if err != nil {
		return "", "", "", err
	}
	var env struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return "", "", "", err
	}
	if env.URI == "" {
		return "", "", "", fmt.Errorf("missing connection URI in response")
	}
	return env.URI, database, role, nil
}

// neonDefaultBranch returns the project's default branch.
func neonDefaultBranch(ctx context.Context, c *client.Client, project string) (neonObject, error) {
	rows, err := neonLiveList(ctx, c, neonProjectPath(project)+"/branches", "branches", nil)
	if err != nil {
		return nil, err
	}
	for _, b := range rows {
		if nb(b, "default") {
			return b, nil
		}
	}
	return nil, notFoundErr(fmt.Errorf("project %s has no default branch", project))
}
