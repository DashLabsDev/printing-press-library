// pp:data-source live
// Hand-authored: neonctl-style connection-string with branch/db/role by name.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNeonConnectionStringCmd(flags *rootFlags) *cobra.Command {
	var project, database, role, format string
	var pooled bool
	cmd := &cobra.Command{
		Use:   "connection-string [branch]",
		Short: "Print a Postgres connection string for a branch (by name or ID).",
		Long:  "Print a Postgres connection string for a branch, resolved by name or ID. Without a branch it uses the project's default branch. Without --project-id it uses the project saved by 'context set'.",
		Example: strings.Trim(`
  neon-pp-cli connection-string pr-123 --project-id lucky-sun-123456
  neon-pp-cli connection-string --project-id lucky-sun-123456 --pooled --format env
  neon-pp-cli connection-string main --project-id lucky-sun-123456 --database neondb --role neondb_owner --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--project-id=example-value"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 && loadNeonContext().ProjectID == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "connection-string")
			}
			if len(args) > 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("at most one branch argument is allowed"))
			}
			project = defaultNeonProject(project)
			if project == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--project-id is required (or save one with 'neon-pp-cli context set --project-id <id>')"))
			}
			switch format {
			case "uri", "env", "psql":
			default:
				return usageErr(fmt.Errorf("--format must be uri, env or psql"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			branchID := ""
			if len(args) == 1 {
				b, err := resolveNeonBranch(ctx, c, project, args[0])
				if err != nil {
					return err
				}
				branchID = ns(b, "id")
			}
			if branchID == "" {
				b, err := neonDefaultBranch(ctx, c, project)
				if err != nil {
					return err
				}
				branchID = ns(b, "id")
			}
			uri, database, role, err := neonConnectionURI(ctx, c, project, branchID, database, role, pooled)
			if err != nil {
				return err
			}
			env := struct{ URI string }{uri}
			// An explicit --format wins over pipe detection so
			// `connection-string --format env >> .env` works.
			explicitText := cmd.Flags().Changed("format") && !flags.asJSON && !flags.agent
			if !explicitText && !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"project_id": project, "branch_id": branchID, "database": database, "role": role, "pooled": pooled, "connection_uri": env.URI}, flags)
			}
			switch format {
			case "env":
				fmt.Fprintf(cmd.OutOrStdout(), "DATABASE_URL=%s\n", env.URI)
			case "psql":
				fmt.Fprintf(cmd.OutOrStdout(), "psql '%s'\n", env.URI)
			default:
				fmt.Fprintln(cmd.OutOrStdout(), env.URI)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&project, "project-id", "", "Neon project ID (defaults to 'context set' project)")
	cmd.Flags().StringVar(&database, "database", "", "Database name (default: the branch's only database)")
	cmd.Flags().StringVar(&role, "role", "", "Role name (default: the database owner)")
	cmd.Flags().BoolVar(&pooled, "pooled", false, "Use the PgBouncer pooled host")
	cmd.Flags().StringVar(&format, "format", "uri", "Human output format: uri, env (DATABASE_URL=...) or psql")
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newNeonConnectionStringCmd(flags))
	})
}
