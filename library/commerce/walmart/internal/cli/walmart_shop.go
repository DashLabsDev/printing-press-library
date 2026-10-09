// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/walmart/internal/walmart"
)

// Read-only product and store commands. None of these change the selected
// store, pickup/delivery mode, cart or slot; the client's read-only guard
// rejects every non-GET and denylisted mutation regardless. Cart and slot
// commands live in walmart_cart.go.

func orchestraGet(cmd *cobra.Command, flags *rootFlags, subgraph, op, hash, vars, pageURL, pathSuffix string, extra map[string]string) ([]byte, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	headers := walmart.OpHeaders(op, pageURL)
	for k, v := range extra {
		headers[k] = v
	}
	path := walmart.OrchestraPath(subgraph, op, hashFor(op, hash)) + pathSuffix
	raw, err := c.GetWithHeadersNoCache(cmd.Context(), path, map[string]string{"variables": vars}, headers)
	if err != nil {
		return nil, classifyAPIError(cmd.OutOrStdout(), err, flags)
	}
	return raw, nil
}

func newProductCmd(flags *rootFlags) *cobra.Command {
	parent := &cobra.Command{
		Use:         "product",
		Short:       "Product detail with your store's price and pickup/delivery availability (read only)",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	get := &cobra.Command{
		Use:   "get <itemId|url>",
		Short: "Show one product: price, unit price, availability, pickup/delivery/shipping status, seller",
		Example: `  walmart-pp-cli product get 10000000001
  walmart-pp-cli product get https://www.walmart.com/ip/Example-Item/10000000001 --json`,
		Args: cobra.ExactArgs(1),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "itemId=51259338",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := walmart.NormalizeItemID(args[0])
			if err != nil {
				return usageErr(err)
			}
			vars, err := walmart.ItemVariables(id)
			if err != nil {
				return usageErr(err)
			}
			raw, err := orchestraGet(cmd, flags, walmart.SubgraphItemByID, walmart.OpItemByID, walmart.HashItemByID, vars,
				walmart.ItemPage(id), walmart.ItemPathSuffix(id), map[string]string{"x-o-item-id": id, "calltype": "CLIENT", "is-variant-fetch": "false", "traffic-type": "Internal", "cyomv2enabled": "true"})
			if err != nil {
				return err
			}
			d, err := walmart.ParseProduct(raw)
			if err != nil {
				return err
			}
			if wantJSON(cmd, flags) {
				return writeJSON(cmd.OutOrStdout(), d)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s\n", d.Name)
			fmt.Fprintf(w, "  Item      %s\n", d.USItemID)
			fmt.Fprintf(w, "  Price     %s", d.PriceDisplay)
			if d.UnitPrice != "" {
				fmt.Fprintf(w, "  (%s)", d.UnitPrice)
			}
			if d.WasPrice != "" {
				fmt.Fprintf(w, "  was %s", d.WasPrice)
			}
			fmt.Fprintln(w)
			fmt.Fprintf(w, "  Status    %s\n", d.Availability)
			fmt.Fprintf(w, "  Pickup    %s\n  Delivery  %s\n", d.PickupStatus, d.DeliveryStatus)
			if d.ShippingStatus != "" {
				fmt.Fprintf(w, "  Shipping  %s\n", d.ShippingStatus)
			}
			if d.Aisle != "" {
				fmt.Fprintf(w, "  Aisle     %s\n", d.Aisle)
			}
			if d.Seller != "" {
				fmt.Fprintf(w, "  Seller    %s\n", d.Seller)
			}
			if d.StoreID != "" {
				fmt.Fprintf(w, "  Store     %s\n", d.StoreID)
			}
			if d.URL != "" {
				fmt.Fprintf(w, "  URL       %s\n", d.URL)
			}
			return nil
		},
	}
	parent.AddCommand(get)
	return parent
}

func newStoreCmd(flags *rootFlags) *cobra.Command {
	parent := &cobra.Command{
		Use:         "store",
		Short:       "Nearby Walmart pickup stores (read only; never changes your store)",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	var zip string
	list := &cobra.Command{
		Use:   "list",
		Short: "List pickup stores near a ZIP code (view only)",
		Example: `  walmart-pp-cli store list --zip 00000
  WALMART_ZIP=00000 walmart-pp-cli store list --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if zip == "" {
				zip = os.Getenv("WALMART_ZIP")
			}
			vars, err := walmart.StoreVariables(zip)
			if err != nil {
				return usageErr(err)
			}
			raw, err := orchestraGet(cmd, flags, walmart.SubgraphNearByNode, walmart.OpNearByNodes, walmart.HashNearByNodes, vars,
				"https://www.walmart.com/", "", nil)
			if err != nil {
				return err
			}
			stores, err := walmart.ParseStores(raw)
			if err != nil {
				return err
			}
			if wantJSON(cmd, flags) {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"stores": stores})
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tDISTANCE\tNAME\tADDRESS\tPICKUP")
			for _, s := range stores {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s, %s %s\t%s\n", s.ID, s.Distance, s.Name, s.Address, s.City, s.State, strings.Join(s.AccessTypes, ","))
			}
			return tw.Flush()
		},
	}
	list.Flags().StringVar(&zip, "zip", "", "5-digit ZIP code to search near (or WALMART_ZIP)")
	parent.AddCommand(list)
	return parent
}
