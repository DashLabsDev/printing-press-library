// Hand-authored Neon fleet command; preserved by generate --force.
// pp:data-source local
package cli

import (
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newNovelStaleCmd(flags *rootFlags) *cobra.Command {
	var idle, dbPath string
	cmd := &cobra.Command{Use: "stale", Short: "Find idle or expired branches across every project.", Long: "Use this command for finding idle or expired branches. Do NOT use this command for ranking cost or consumption; use 'usage top' instead.", Example: strings.Trim(`
  neon-pp-cli stale --idle 14d --db ./dev.db --json
`, "\n"), Annotations: map[string]string{"pp:data-source": "local", "mcp:read-only": "true", "pp:happy-args": "--idle=14d"}, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "stale")
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
		rows, err := selectStale(db, d, time.Now())
		if err != nil {
			return err
		}
		return neonOutput(cmd, flags, rows)
	}}
	cmd.Flags().StringVar(&idle, "idle", "14d", "Minimum idle age (e.g. 14d)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite mirror path")
	return cmd
}
