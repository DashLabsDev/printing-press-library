// pp:data-source local
package cli

import (
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newNovelUsageTopCmd(flags *rootFlags) *cobra.Command {
	var dbPath, since, metric string
	cmd := &cobra.Command{Use: "top", Short: "Rank projects by compute, storage, transfer or written-data use from the local mirror.", Long: "Use this command for ranking projects by consumption. Do NOT use this command for time series; use 'usage trend' instead.", Example: strings.Trim(`
  neon-pp-cli usage top --since 7d --db ./dev.db --json
`, "\n"), Annotations: map[string]string{"pp:data-source": "local", "mcp:read-only": "true", "pp:happy-args": "--since=7d"}, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "usage top")
		}
		if len(args) != 0 {
			return neonMissing(cmd, "unexpected positional arguments")
		}
		d, err := neonDuration(since)
		if err != nil {
			return err
		}
		if err := validateNeonMetric(metric); err != nil {
			return err
		}
		db, err := openNeonMirror(cmd, flags, dbPath, "projects")
		if err != nil {
			return err
		}
		if db != nil {
			defer db.Close()
		}
		pointsFor := consumptionPoints
		if metric == "storage" {
			pointsFor = storageRankPoints
		}
		points, err := pointsFor(db)
		if err != nil {
			return err
		}
		now := time.Now()
		return neonOutput(cmd, flags, rankNeonUsage(points, metric, now.Add(-d), now))
	}}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite mirror path")
	cmd.Flags().StringVar(&since, "since", "7d", "Window duration (e.g. 7d); rows selected by timeframe start")
	cmd.Flags().StringVar(&metric, "metric", "compute", "Metric: compute, storage, transfer, written")
	return cmd
}
