// pp:data-source local
package cli

import (
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newNovelUsageTrendCmd(flags *rootFlags) *cobra.Command {
	var dbPath, since string
	cmd := &cobra.Command{Use: "trend", Short: "Show consecutive period usage deltas.", Long: "Use this command for usage change over time. Do NOT use this command for ranking which projects consume most; use 'usage top' instead.", Example: strings.Trim(`
  neon-pp-cli usage trend --since 30d --db ./dev.db --json
`, "\n"), Annotations: map[string]string{"pp:data-source": "local", "mcp:read-only": "true", "pp:happy-args": "--since=30d"}, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "usage trend")
		}
		if len(args) != 0 {
			return neonMissing(cmd, "unexpected positional arguments")
		}
		d, err := neonDuration(since)
		if err != nil {
			return err
		}

		db, err := openNeonMirror(cmd, flags, dbPath, "projects")
		if err != nil {
			return err
		}
		if db != nil {
			defer db.Close()
		}
		points, err := consumptionPoints(db)
		if err != nil {
			return err
		}
		now := time.Now()
		return neonOutput(cmd, flags, trendNeonUsage(points, now.Add(-d), now))
	}}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite mirror path")
	cmd.Flags().StringVar(&since, "since", "30d", "Window duration (e.g. 7d); rows selected by timeframe start")

	return cmd
}
