// pp:data-source computed
// Hand-authored: `context set` (file name lets verify-skill resolve the path).

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newNeonContextSetCmd(flags *rootFlags) *cobra.Command {
	var project, org string
	cmd := &cobra.Command{
		Use:   "set",
		Args:  cobra.NoArgs,
		Short: "Save the default project and/or org.",
		Example: strings.Trim(`
  neon-pp-cli context set --project-id lucky-sun-123456
  neon-pp-cli context set --project-id lucky-sun-123456 --org-id org-young-tree-12345678 --json
`, "\n"),
		Annotations: map[string]string{"mcp:local-write": "true", "pp:happy-args": "--project-id=lucky-sun-123456"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "context set")
			}
			if project == "" && org == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--project-id or --org-id is required"))
			}
			path, err := neonContextPath()
			if err != nil {
				return err
			}
			ctx := loadNeonContext()
			if project != "" {
				ctx.ProjectID = project
			}
			if org != "" {
				ctx.OrgID = org
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				return err
			}
			raw, _ := json.MarshalIndent(ctx, "", "  ")
			if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
				return err
			}
			if err := os.Chmod(path, 0o600); err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"path": path, "project_id": ctx.ProjectID, "org_id": ctx.OrgID}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "saved context to %s (project %q, org %q)\n", path, ctx.ProjectID, ctx.OrgID)
			return nil
		},
	}
	cmd.Flags().StringVar(&project, "project-id", "", "Default Neon project ID (e.g. lucky-sun-123456)")
	cmd.Flags().StringVar(&org, "org-id", "", "Default Neon organization ID (e.g. org-young-tree-12345678)")
	return cmd
}
