// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/walmart/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/walmart/internal/walmart"
)

// Hand-written, read-only Walmart commands. Registered as novel commands so
// regeneration preserves them. See .printing-press-patches/.

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		for _, c := range root.Commands() {
			if c.Name() == "orders" {
				addNovelCommandIfAbsent(c, newOrdersListCmd(flags))
				addNovelCommandIfAbsent(c, newOrdersGetCmd(flags))
			}
		}
		addNovelCommandIfAbsent(root, newSearchCmd(flags))
		addNovelCommandIfAbsent(root, newProductCmd(flags))
		addNovelCommandIfAbsent(root, newStoreCmd(flags))
		addNovelCommandIfAbsent(root, newCartCmd(flags))
		addNovelCommandIfAbsent(root, newSlotsCmd(flags))
	})
}

// wantJSON reports whether a Walmart read should skip its human table and
// go through the shared output path (--json, --agent, --select, --compact,
// --csv, --plain, --quiet, or a non-terminal stdout).
func wantJSON(cmd *cobra.Command, flags *rootFlags) bool {
	return flags.asJSON || flags.agent || flags.csv || flags.plain || flags.quiet ||
		flags.compact || strings.TrimSpace(flags.selectFields) != "" || !isTerminal(cmd.OutOrStdout())
}

// emitRead routes a read result through printJSONFiltered so --select,
// --compact, --csv, --plain, --quiet and --agent behave like every other
// command. rows (may be nil) is the list rendered by the row-oriented
// formats (--csv, --plain, --quiet); JSON keeps the full envelope.
//
// These commands just fetched fresh, uncached data from walmart.com, so the
// --agent envelope reports meta.source "live" (the shared printJSONFiltered
// path assumes "local").
func emitRead(cmd *cobra.Command, flags *rootFlags, full any, rows any) error {
	v := full
	if rows != nil && (flags.csv || flags.plain || flags.quiet) {
		v = rows
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return printOutputWithFlagsMeta(cmd.OutOrStdout(), json.RawMessage(raw), flags, map[string]any{"source": "live"})
}

// requireLiveSource rejects --data-source local for Walmart reads. They only
// read live from walmart.com and have no local store, so "local" must fail
// before any network request instead of silently going online.
func requireLiveSource(flags *rootFlags) error {
	if flags != nil && strings.EqualFold(strings.TrimSpace(flags.dataSource), "local") {
		return usageErr(fmt.Errorf("this command reads live from walmart.com and has no local data; use --data-source auto or live"))
	}
	return nil
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func hashFor(op, def string) string {
	if v := strings.TrimSpace(os.Getenv("WALMART_HASH_" + strings.ToUpper(op))); v != "" {
		return v
	}
	return def
}

func newOrdersListCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var search, cursor, typ, status string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List purchase history (online and in-store), one page",
		Example: `  walmart-pp-cli orders list --limit 5
  walmart-pp-cli orders list --type in-store --json
  walmart-pp-cli orders list --search milk`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			var filters []string
			for _, f := range []string{typ, status} {
				if f == "" {
					continue
				}
				id, ok := walmart.HistoryFilter[f]
				if !ok {
					return usageErr(fmt.Errorf("unknown filter %q (use in-store, online, completed, in-progress)", f))
				}
				filters = append(filters, id)
			}
			vars, err := walmart.HistoryVariables(cursor, search, limit, filters)
			if err != nil {
				return err
			}
			if err := requireLiveSource(flags); err != nil { // pp:data-source live
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := walmart.OrchestraPath("cph", walmart.OpPurchaseHistory, hashFor(walmart.OpPurchaseHistory, walmart.HashPurchaseHistory))
			raw, err := c.GetWithHeadersNoCache(cmd.Context(), path, map[string]string{"variables": vars}, walmart.OpHeaders(walmart.OpPurchaseHistory, walmart.OrdersPage))
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			page, err := walmart.ParseHistory(raw)
			if err != nil {
				return err
			}
			if wantJSON(cmd, flags) {
				return emitRead(cmd, flags, page, page.Orders)
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			fmt.Fprintln(tw, "DATE\tTYPE\tITEMS\tTOTAL\tORDER ID\tGROUP")
			for _, o := range page.Orders {
				fmt.Fprintf(tw, "%s\t%s\t%d\t%.2f\t%s\t%s\n", o.OrderDate, o.Type, o.ItemCount, o.Total, o.ID, o.GroupID)
			}
			_ = tw.Flush()
			if page.NextCursor != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), "\nMore orders: re-run with --cursor <nextCursor from --json>")
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "Orders per page")
	cmd.Flags().StringVar(&search, "search", "", "Search orders by item text")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Page cursor from a previous --json result")
	cmd.Flags().StringVar(&typ, "type", "", "Order type filter: in-store or online")
	cmd.Flags().StringVar(&status, "status", "", "Status filter: completed or in-progress")
	return cmd
}

func newOrdersGetCmd(flags *rootFlags) *cobra.Command {
	var group string
	var inStore bool
	cmd := &cobra.Command{
		Use:   "get <orderId>",
		Short: "Show one order with line items, subtotal, tax, fees and total",
		Example: `  walmart-pp-cli orders get 2000000000000001
  walmart-pp-cli orders get 2000000000000001 --in-store --json`,
		Args: cobra.ExactArgs(1),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "orderId=2000000000000001",
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			vars, err := walmart.OrderVariables(args[0], group, inStore)
			if err != nil {
				return usageErr(err)
			}
			if err := requireLiveSource(flags); err != nil { // pp:data-source live
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			page := walmart.OrdersPage + "/" + url.PathEscape(args[0])
			path := walmart.OrchestraPath("orders", walmart.OpGetOrder, hashFor(walmart.OpGetOrder, walmart.HashGetOrder))
			raw, err := c.GetWithHeadersNoCache(cmd.Context(), path, map[string]string{"variables": vars}, walmart.OpHeaders(walmart.OpGetOrder, page))
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			d, err := walmart.ParseOrder(raw)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			if wantJSON(cmd, flags) {
				return emitRead(cmd, flags, d, nil)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Order %s  %s  %s  %d items\n\n", d.ID, d.OrderDate, d.Type, d.ItemCount)
			tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
			fmt.Fprintln(tw, "QTY\tPRICE\tITEM")
			for _, it := range d.Items {
				fmt.Fprintf(tw, "%d\t%.2f\t%s\n", it.Quantity, it.LinePrice, it.Name)
			}
			_ = tw.Flush()
			fmt.Fprintf(w, "\nSubtotal %.2f  Tax %.2f", d.SubTotal, d.Tax)
			if d.Fees != 0 {
				fmt.Fprintf(w, "  Fees %.2f", d.Fees)
			}
			if d.Tip != 0 {
				fmt.Fprintf(w, "  Tip %.2f", d.Tip)
			}
			fmt.Fprintf(w, "  Total %.2f\n", d.Total)
			return nil
		},
	}
	cmd.Flags().StringVar(&group, "group", "", "Group id from orders list (improves online-order lookups)")
	cmd.Flags().BoolVar(&inStore, "in-store", false, "The order is an in-store receipt")
	return cmd
}

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var page int
	var filter, sort string
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search products with your store's price and availability",
		Example: `  walmart-pp-cli search milk
  walmart-pp-cli search "paper towels" --page 2 --json
  walmart-pp-cli search bananas --filter "brand:Great Value"
  walmart-pp-cli search bananas --list-filters`,
		Args: cobra.MinimumNArgs(1),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "query=bananas",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			q := strings.Join(args, " ")
			if page < 1 {
				page = 1
			}
			if err := requireLiveSource(flags); err != nil { // pp:data-source live
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			params := map[string]string{"q": q}
			if page > 1 {
				params["page"] = strconv.Itoa(page)
			}
			if filter != "" {
				if err := walmart.ValidateFacet(filter); err != nil {
					return usageErr(err)
				}
				params["facet"] = strings.TrimSpace(filter)
			}
			if sort != "" {
				params["sort"] = sort
			}
			headers := map[string]string{
				client.HTMLResponseHeader: "true",
				"Accept":                  "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
				"content-type":            "",
				"sec-fetch-mode":          "navigate",
				"sec-fetch-dest":          "document",
			}
			raw, err := c.GetWithHeadersNoCache(cmd.Context(), "/search", params, headers)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			html := []byte(raw)
			var s string
			if json.Unmarshal(raw, &s) == nil {
				html = []byte(s)
			}
			res, err := walmart.ParseSearchHTML(html, q, page)
			if err != nil {
				return err
			}
			if filter != "" {
				res.Filters = []string{strings.TrimSpace(filter)}
			}
			res.Sort = sort
			listFilters, _ := cmd.Flags().GetBool("list-filters")
			if !listFilters {
				res.Facets = nil
			}
			if listFilters && !wantJSON(cmd, flags) {
				w := cmd.OutOrStdout()
				for _, f := range res.Facets {
					fmt.Fprintf(w, "%s (%s)\n", f.Name, f.Type)
					for _, v := range f.Values {
						mark := " "
						if v.Selected {
							mark = "*"
						}
						fmt.Fprintf(w, "  %s --filter \"%s:%s\"  %s\n", mark, f.Type, v.ID, v.Name)
					}
				}
				return nil
			}
			if wantJSON(cmd, flags) {
				return emitRead(cmd, flags, res, res.Products)
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			fmt.Fprintln(tw, "PRICE\tUNIT\tAVAILABILITY\tFULFILLMENT\tITEM ID\tNAME")
			for _, p := range res.Products {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", p.PriceDisplay, p.UnitPrice, p.Availability, p.Fulfillment, p.USItemID, p.Name)
			}
			_ = tw.Flush()
			fmt.Fprintf(cmd.ErrOrStderr(), "\n%d products on page %d (store %s)\n", len(res.Products), res.Page, res.StoreID)
			return nil
		},
	}
	cmd.Flags().IntVar(&page, "page", 1, "Result page")
	cmd.Flags().StringVar(&filter, "filter", "", "One search filter as type:value, e.g. \"brand:Great Value\" (see --list-filters)")
	cmd.Flags().StringVar(&sort, "sort", "", "Sort id, e.g. best_match or price_low (see --list-filters)")
	cmd.Flags().Bool("list-filters", false, "Show the filter and sort values available for this query")
	return cmd
}
