// Copyright 2026 H179922 and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newResultsCmd(flags *rootFlags) *cobra.Command {
	var q queryFlags
	var limit int
	cmd := &cobra.Command{
		Use:   "results",
		Short: "Search OFA health-test records by breed, test, year, CHIC status, and name",
		Long: `Search OFA Advanced Search records. Each row is one test result for one dog.

Multi-year filters (--report-year, --test-year, --birth-year) run one backend
query per year: OFA's backend keeps only the last value of a repeated year
field, so a single combined query would silently return one year only.
Hip rows carry the grade decoded from the OFA number (summary rows have no
conclusion column); 'flag' marks rows with no OFA number or a carrier suffix.
Use 'dog <appnum>' for a dog's published conclusions.`,
		Example: `  ofa-health-clearances-pp-cli results --breed AUK --report-year 2026
  ofa-health-clearances-pp-cli results --breed AS --chic --report-year 2026 --agent --select name,ofa_number,test
  ofa-health-clearances-pp-cli results --breed AUK --name AVALON --test HD`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live",
			"pp:happy-args": "--breed=AUK;--report-year=2026"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			query, err := q.toQuery()
			if err != nil {
				_ = cmd.Usage()
				return err
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, fmt.Sprintf("%d OFA search quer(ies), up to %d page(s) each", len(query.Forms()), q.maxPages))
			}
			c, err := newOFAClient(flags)
			if err != nil {
				return err
			}
			rows, meta, err := fetchRows(cmd.Context(), c, query, q.maxPages, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				if len(rows) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "No matching OFA records.")
					return nil
				}
				if err := printAutoTable(cmd.OutOrStdout(), toMaps(rows)); err != nil {
					return err
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "\n%d rows (%d total matches across %d backend quer(ies), %d page(s) scanned)\n",
					len(rows), meta.TotalMatches, meta.Queries, meta.ScannedPages)
				return nil
			}
			return printJSONFilteredKeep(cmd.OutOrStdout(), rows, flags, healthKeepFields...)
		},
	}
	addQueryFlags(cmd, &q)
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows to print (0 = all fetched rows)")
	return cmd
}
