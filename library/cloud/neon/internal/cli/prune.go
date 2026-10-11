// Hand-authored Neon fleet command; preserved by generate --force.
// pp:data-source computed
package cli

import (
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newNovelPruneCmd(flags *rootFlags) *cobra.Command {
	var idle, dbPath string
	var yes bool
	cmd := &cobra.Command{Use: "prune", Short: "Preview or delete idle and expired branches.", Long: "Use this command for deleting idle or expired branches. Do NOT use this command for just listing them; use 'stale' instead.", Example: strings.Trim(`
  neon-pp-cli prune --idle 14d --db ./dev.db --json
  neon-pp-cli prune --idle 14d --yes
`, "\n"), Annotations: map[string]string{"pp:data-source": "computed", "pp:preview-happy-path": "true", "pp:happy-args": "--idle=14d"}, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "prune")
		}
		if len(args) != 0 {
			return neonMissing(cmd, "unexpected positional arguments")
		}
		d, err := neonDuration(idle)
		if err != nil {
			return err
		}
		db, err := openNeonMirror(cmd, flags, dbPath, "branches")
		if err != nil {
			return err
		}
		if db != nil {
			defer db.Close()
		}
		selected, err := selectStale(db, d, time.Now())
		if err != nil {
			return err
		}
		rows := make([]staleBranch, 0)
		for _, r := range selected {
			if !r.Default && !r.Protected {
				rows = append(rows, r)
			}
		}
		if !yes || len(rows) == 0 {
			return neonOutput(cmd, flags, rows)
		}
		// Report what actually happened per branch, even if a delete fails midway.
		results, delErr := deleteNeonBranches(cmd, flags, rows, d, time.Now())
		if err := neonOutput(cmd, flags, results); err != nil {
			return err
		}
		return delErr
	}}
	cmd.Flags().StringVar(&idle, "idle", "14d", "Minimum idle age")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite mirror path")
	cmd.Flags().BoolVar(&yes, "yes", false, "Delete selected branches; default is preview only")
	return cmd
}
