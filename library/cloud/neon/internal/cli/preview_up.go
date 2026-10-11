// pp:data-source live
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/cloud/neon/internal/client"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newNovelPreviewUpCmd(flags *rootFlags) *cobra.Command {
	var project, expires, parent, database, role string
	var pooled bool
	var waitFor string
	cmd := &cobra.Command{Use: "up [name]", Short: "Create or reuse a named preview and get its connection string.", Long: "Use this command for idempotently creating or reusing a named preview branch and getting its connection string. Do NOT use this command for removing branches; use 'prune' instead. Existing branches keep their current expiration and endpoint settings. Exit codes: 6 failed operation, 8 timeout.", Example: strings.Trim(`
  neon-pp-cli preview up pr-123 --project-id lucky-sun-123456 --expires 7d --database neondb --role neondb_owner --pooled --json
`, "\n"), Annotations: map[string]string{"pp:data-source": "live", "pp:typed-exit-codes": "0,2,3,6,8", "pp:happy-args": "name=example-value;--project-id=example-value;--expires=7d", "pp:verified-by-proof": "preview-up-lifecycle.md"}, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "preview up")
		}
		project = defaultNeonProject(project)
		if len(args) != 1 || strings.TrimSpace(args[0]) == "" || project == "" {
			return neonMissing(cmd, "one preview name and --project-id are required")
		}
		duration, err := neonDuration(expires)
		if err != nil {
			return err
		}
		waitBudget, err := neonDuration(waitFor)
		if err != nil {
			return err
		}
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		// The generated client bounds each request with --timeout; the operation
		// wait has its own --wait-timeout budget.
		ctx := cmd.Context()
		branch, err := resolveNeonBranch(ctx, c, project, args[0])
		ids := make([]string, 0)
		if err != nil {
			var typed *cliError
			if !errors.As(err, &typed) || typed.code != 3 {
				return err
			}
			branchBody := map[string]any{"name": args[0], "expires_at": time.Now().UTC().Add(duration).Format(time.RFC3339)}
			if parent != "" {
				b, err := resolveNeonBranch(ctx, c, project, parent)
				if err != nil {
					return err
				}
				branchBody["parent_id"] = ns(b, "id")
			}
			raw, _, err := c.Post(ctx, neonProjectPath(project)+"/branches", map[string]any{"branch": branchBody, "endpoints": []map[string]any{{"type": "read_write"}}})
			var apiErr *client.APIError
			if errors.As(err, &apiErr) && apiErr.StatusCode == 409 {
				// A concurrent run created it first; reuse that branch.
				branch, err = resolveNeonBranch(ctx, c, project, args[0])
				if err != nil {
					return err
				}
				raw = nil
				// Wait on the other run's in-flight or failed operations by ID so a
				// failed create is reported instead of returning a broken branch.
				createdAt, _ := parseRestSyncTimestamp(ns(branch, "created_at"))
				recent, err := neonRecentOperations(ctx, c, project, createdAt)
				if err != nil {
					return err
				}
				ops := struct{ Operations []neonObject }{recent}
				for _, op := range ops.Operations {
					if ns(op, "branch_id") != ns(branch, "id") {
						continue
					}
					if t, ok := parseRestSyncTimestamp(ns(op, "created_at")); ok && t.Before(createdAt) {
						continue
					}
					if st := ns(op, "status"); st != "finished" && st != "skipped" {
						ids = append(ids, ns(op, "id"))
					}
				}
			} else if err != nil {
				return err
			}
			var env struct {
				Branch     neonObject   `json:"branch"`
				Operations []neonObject `json:"operations"`
			}
			if raw != nil {
				if err := json.Unmarshal(raw, &env); err != nil {
					return err
				}
				branch = env.Branch
			}
			for _, op := range env.Operations {
				if id := ns(op, "id"); id != "" {
					ids = append(ids, id)
				}
			}
		}
		id := ns(branch, "id")
		if id == "" {
			return fmt.Errorf("missing branch ID in response")
		}
		if err := waitNeonOperations(ctx, c, project, id, ids, waitBudget, 2*time.Second); err != nil {
			return err
		}
		uri, database, role, err := neonConnectionURI(ctx, c, project, id, database, role, pooled)
		if err != nil {
			return err
		}
		if wantsHumanTable(cmd.OutOrStdout(), flags) {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "DATABASE_URL=%s\n", uri)
			return err
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"project_id": project, "branch_id": id, "database": database, "role": role, "connection_uri": uri}, flags)
	}}
	cmd.Flags().StringVar(&project, "project-id", "", "Project ID")
	cmd.Flags().StringVar(&expires, "expires", "7d", "Expiration duration for a newly created branch (existing branches keep theirs)")
	cmd.Flags().StringVar(&waitFor, "wait-timeout", "5m", "How long to wait for branch operations to finish (e.g. 5m)")
	cmd.Flags().StringVar(&parent, "parent", "", "Parent branch name or ID")
	cmd.Flags().StringVar(&database, "database", "", "Database name")
	cmd.Flags().StringVar(&role, "role", "", "Role name")
	cmd.Flags().BoolVar(&pooled, "pooled", false, "Use a pooled connection")
	return cmd
}
