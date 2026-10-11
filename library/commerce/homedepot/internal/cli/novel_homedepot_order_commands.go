// Copyright 2026 Greg Mushen and contributors. Licensed under Apache-2.0.
package cli

import (
	"fmt"
	"github.com/spf13/cobra"
	"strings"
	"time"
)

func homeDepotDateRange(from, to string, months int) (time.Time, time.Time, error) {
	end := time.Now()
	start := end.AddDate(0, -months, 0)
	var err error
	if from != "" {
		start, err = time.Parse("2006-01-02", from)
		if err != nil {
			return start, end, usageErr(fmt.Errorf("--from must be YYYY-MM-DD"))
		}
	}
	if to != "" {
		end, err = time.Parse("2006-01-02", to)
		if err != nil {
			return start, end, usageErr(fmt.Errorf("--to must be YYYY-MM-DD"))
		}
	}
	if end.Before(start) {
		return start, end, usageErr(fmt.Errorf("--to is before --from"))
	}
	return start, end, nil
}

// pp:data-source live
func newOrdersGetCmd(flags *rootFlags) *cobra.Command {
	var from, to string
	var maxPages int
	cmd := &cobra.Command{Use: "get <reference>", Short: "Read items and totals by a reference from orders list; --from/--to bound the history lookup", Annotations: map[string]string{"pp:data-source": "live", "mcp:read-only": "true", "pp:happy-args": "ref=sample-order;--dry-run"}, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
			return usageErr(fmt.Errorf("provide a reference from 'orders list'"))
		}
		start, end, err := homeDepotDateRange(from, to, 24)
		if err != nil {
			return err
		}
		if maxPages < 1 || maxPages > 100 {
			return usageErr(fmt.Errorf("--max-scan-pages must be between 1 and 100"))
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "orders get")
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		orders, err := listHomeDepotOrders(ctx, flags, start, end, maxPages)
		if err != nil {
			return err
		}
		for _, o := range orders {
			match := o.reference() == args[0]
			for _, n := range o.numbers() {
				if n == args[0] {
					match = true
					if o.OrderOrigin == "online" {
						o.OrderNumber = n
						o.OrderNumbers = []string{n}
					}
				}
			}
			if !match {
				continue
			}
			detail, err := getHomeDepotOrder(ctx, flags, o)
			if err != nil {
				return err
			}
			return printHomeDepotOrderDetail(cmd, flags, o, detail)
		}
		return notFoundErr(fmt.Errorf("order was not found in the requested history range; use --from and --to to change the range"))
	}}
	cmd.Flags().StringVar(&from, "from", "", "Start date YYYY-MM-DD (default two years ago)")
	cmd.Flags().StringVar(&to, "to", "", "End date YYYY-MM-DD (default today)")
	cmd.Flags().IntVar(&maxPages, "max-scan-pages", 50, "Maximum history pages to scan")
	return cmd
}

func printHomeDepotOrderDetail(cmd *cobra.Command, flags *rootFlags, o observedOrderSummary, detail observedOrderDetail) error {
	type safeItem struct {
		Description string  `json:"description"`
		Quantity    float64 `json:"quantity"`
		Total       float64 `json:"total"`
	}
	items := make([]safeItem, 0)
	for _, group := range detail.FulfillmentGroups {
		for _, item := range group.LineItems {
			items = append(items, safeItem{item.Description, item.CurrentQuantity - item.CancelledQuantity, item.TotalPrice})
		}
	}
	return printJSONFilteredKeep(cmd.OutOrStdout(), map[string]any{"reference": o.reference(), "date": detail.SalesDate, "status": detail.StatusDescription, "subtotal": detail.SubTotalAmount, "tax": detail.TaxTotalAmount, "total": detail.GrandTotalAmount, "items": items}, flags, "description")
}
