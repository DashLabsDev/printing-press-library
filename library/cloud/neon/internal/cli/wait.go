// pp:data-source live
package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

func newNovelWaitCmd(flags *rootFlags) *cobra.Command {
	var branch, project, operation, timeout, interval string
	var all bool
	var scope string
	cmd := &cobra.Command{Use: "wait", Short: "Wait for branch operations to finish.", Long: "Poll branch operations until complete. --scope all (or --all) waits until no operation is running in any project the key can see. Exit codes: 0 success, 2 invalid input, 3 branch not found, 6 failed operation, 8 timeout. Running, scheduling and cancelling operations remain pending.", Example: strings.Trim(`
  neon-pp-cli wait --scope all --timeout 2m
  neon-pp-cli wait --branch pr-123 --project-id lucky-sun-123456 --timeout 5m --interval 2s --json
`, "\n"), Annotations: map[string]string{"pp:data-source": "live", "mcp:read-only": "true", "pp:typed-exit-codes": "0,2,3,6,8", "pp:happy-args": "--scope=all;--timeout=20s"}, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "wait")
		}
		if len(args) != 0 {
			return neonMissing(cmd, "unexpected positional arguments")
		}
		switch scope {
		case "", "branch":
		case "all":
			all = true
		default:
			return neonMissing(cmd, "--scope must be branch or all")
		}
		if !all {
			project = defaultNeonProject(project)
		}
		if !all && (branch == "" || project == "") {
			return neonMissing(cmd, "--branch and --project-id are required (or use --all)")
		}
		if all && (branch != "" || operation != "") {
			return neonMissing(cmd, "--all cannot be combined with --branch or --operation")
		}
		d, err := neonDuration(timeout)
		if err != nil {
			return err
		}
		every, err := neonDuration(interval)
		if err != nil {
			return err
		}
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		pollFlags := *flags
		pollFlags.timeout = d
		ctx, cancel := boundCtx(cmd.Context(), &pollFlags)
		defer cancel()
		if all {
			projects, err := neonLiveList(ctx, c, "/projects", "projects", nil)
			if err != nil {
				return neonWaitError(err)
			}
			rows := make([]map[string]any, 0, len(projects))
			for _, p := range projects {
				if err := waitNeonOperations(ctx, c, ns(p, "id"), "", nil, d, every); err != nil {
					return err
				}
				rows = append(rows, map[string]any{"project_id": ns(p, "id"), "status": "idle"})
			}
			return neonOutput(cmd, flags, rows)
		}
		b, err := resolveNeonBranch(ctx, c, project, branch)
		if err != nil {
			return neonWaitError(err)
		}
		ids := make([]string, 0)
		if operation != "" {
			ids = append(ids, operation)
		}
		if err := waitNeonOperations(ctx, c, project, ns(b, "id"), ids, d, every); err != nil {
			return err
		}
		return neonOutput(cmd, flags, []map[string]any{{"project_id": project, "branch_id": ns(b, "id"), "status": "finished"}})
	}}
	cmd.Flags().StringVar(&branch, "branch", "", "Branch name or ID")
	cmd.Flags().StringVar(&project, "project-id", "", "Project ID")
	cmd.Flags().StringVar(&operation, "operation", "", "Specific operation ID to wait for")
	cmd.Flags().StringVar(&timeout, "timeout", "5m", "Maximum polling duration")
	cmd.Flags().StringVar(&interval, "interval", "2s", "Delay between polls")
	cmd.Flags().BoolVar(&all, "all", false, "Shorthand for --scope all")
	cmd.Flags().StringVar(&scope, "scope", "branch", "What to wait for: branch (needs --branch) or all (every project)")
	return cmd
}
