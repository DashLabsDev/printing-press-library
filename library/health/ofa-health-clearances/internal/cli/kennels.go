// Copyright 2026 H179922 and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/health/ofa-health-clearances/internal/ofa"
)

type kennelsView struct {
	Kennels     []ofa.Kennel `json:"kennels"`
	ScannedRows int          `json:"scanned_rows"`
	fetchMeta
	Note string `json:"note,omitempty"`
}

func newNovelKennelsCmd(flags *rootFlags) *cobra.Command {
	var q queryFlags
	var prefixWords, limit int
	cmd := &cobra.Command{
		Use:   "kennels",
		Short: "See which kennels in a breed are actively health-testing young dogs, ranked by tested dogs, CHIC dogs",
		Long: `Use this command to rank kennels across a breed by health-testing activity.
Do NOT use it for a single dog's record; use 'dog' instead.

OFA records carry no breeder field. Registered names begin with the kennel
name, so 'kennels' groups every matching record by that prefix and reports, per
kennel: distinct dogs, CHIC dogs, records, tests, latest test date, decoded hip
grades, and flagged records (no OFA number or carrier). Multi-year filters run
one backend query per year and every page is fetched up to --max-scan-pages.
Prefixes are a heuristic; --prefix-words fixes kennels with multi-word names.`,
		Example: `  ofa-health-clearances-pp-cli kennels --breed AUK --born-from 2021 --report-year 2024,2025,2026 --agent
  ofa-health-clearances-pp-cli kennels --breed AS --chic --report-year 2026 --limit 20`,
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
			if prefixWords < 0 {
				return usageErr(fmt.Errorf("--prefix-words must be 0 (auto) or more"))
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, fmt.Sprintf("%d OFA search quer(ies), up to %d page(s) each, grouped by kennel prefix", len(query.Forms()), q.maxPages))
			}
			c, err := newOFAClient(flags)
			if err != nil {
				return err
			}
			rows, meta, err := fetchRows(cmd.Context(), c, query, q.maxPages, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			view := kennelsView{Kennels: ofa.Rollup(rows, prefixWords), ScannedRows: len(rows), fetchMeta: meta}
			if limit > 0 && len(view.Kennels) > limit {
				view.Kennels = view.Kennels[:limit]
			}
			switch {
			case len(rows) == 0:
				view.Note = "no OFA records matched these filters"
			case meta.Truncated:
				view.Note = fmt.Sprintf("stopped at --max-scan-pages %d before the end of the result set; raise it for complete counts", meta.MaxScanPages)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printKennelsHuman(cmd, view)
			}
			return printJSONFilteredKeep(cmd.OutOrStdout(), view, flags, healthKeepFields...)
		},
	}
	addQueryFlags(cmd, &q)
	cmd.Flags().IntVar(&prefixWords, "prefix-words", 0, "Words in the kennel prefix (0 = auto: first word, two when the first is short)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum kennels to print (0 = all)")
	return cmd
}

func printKennelsHuman(cmd *cobra.Command, v kennelsView) error {
	w := cmd.OutOrStdout()
	if len(v.Kennels) > 0 {
		rows := make([]map[string]any, 0, len(v.Kennels))
		for _, k := range v.Kennels {
			rows = append(rows, map[string]any{"kennel": k.Prefix, "dogs": k.Dogs, "chic_dogs": k.CHICDogs,
				"records": k.Records, "latest_test": k.LatestTest, "flagged": k.Flagged, "tests": len(k.Tests)})
		}
		if err := printAutoTable(w, rows); err != nil {
			return err
		}
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "\n%d kennels from %d records (%d backend quer(ies), %d page(s) scanned)\n",
		len(v.Kennels), v.ScannedRows, v.Queries, v.ScannedPages)
	if v.Note != "" {
		fmt.Fprintln(cmd.ErrOrStderr(), "note: "+v.Note)
	}
	return nil
}
