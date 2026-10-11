package cli

import (
	"fmt"
	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelOrdersListCmd(flags *rootFlags) *cobra.Command {
	var from, to string
	var maxPages, limit int
	cmd := &cobra.Command{Use: "list", Short: "List online and in-store purchases by date range with bounded paging, --limit and detail references", Args: cobra.NoArgs, Annotations: map[string]string{"pp:data-source": "live", "mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		start, end, err := homeDepotDateRange(from, to, 3)
		if err != nil {
			return err
		}
		if maxPages < 1 || maxPages > 100 {
			return usageErr(fmt.Errorf("--max-scan-pages must be between 1 and 100"))
		}
		if limit < 1 || limit > 1000 {
			return usageErr(fmt.Errorf("--limit must be between 1 and 1000"))
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "orders list")
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		orders, err := listHomeDepotOrders(ctx, flags, start, end, maxPages)
		if err != nil {
			return err
		}
		type safeOrder struct {
			Reference string   `json:"reference"`
			Numbers   []string `json:"numbers,omitempty"`
			Date      string   `json:"date"`
			Origin    string   `json:"origin"`
			Total     float64  `json:"total"`
		}
		total := len(orders)
		if len(orders) > limit {
			orders = orders[:limit]
		}
		out := make([]safeOrder, 0, len(orders))
		for _, o := range orders {
			out = append(out, safeOrder{o.reference(), o.numbers(), o.SalesDate, o.OrderOrigin, o.TotalAmount})
		}
		return flags.printJSON(cmd, map[string]any{"orders": out, "count": len(out), "scanned_orders": total, "complete": total == len(out), "from": start.Format("2006-01-02"), "to": end.Format("2006-01-02"), "note": "Home Depot controls the available purchase-history window."})
	}}
	cmd.Flags().StringVar(&from, "from", "", "Start date YYYY-MM-DD (default three months ago)")
	cmd.Flags().StringVar(&to, "to", "", "End date YYYY-MM-DD (default today)")
	cmd.Flags().IntVar(&maxPages, "max-scan-pages", 50, "Maximum history pages; incomplete results return an error")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum purchase summaries returned after bounded paging")
	return cmd
}
