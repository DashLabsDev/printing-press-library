package cli

import (
	"fmt"
	"github.com/spf13/cobra"
	"strconv"
	"strings"
)

// pp:data-source live
func newNovelCartAddCmd(flags *rootFlags) *cobra.Command {
	var quantity int
	var store, fulfillment, zip string
	cmd := &cobra.Command{Use: "add <item-id>", Short: "Preview adding an item ID by quantity and pickup/store or delivery/ZIP; --confirm applies it", Example: "  homedepot-pp-cli cart add 100001123 --store 4706 --dry-run", Annotations: map[string]string{"pp:data-source": "live", "mcp:read-only": "false", "pp:preview-happy-path": "true", "pp:happy-args": "item=300951230;--store=8964;--quantity=1"}, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 || !homeDepotItemID.MatchString(args[0]) {
			return usageErr(fmt.Errorf("provide one nine-digit product item ID"))
		}
		if quantity < 1 || quantity > 999 {
			return usageErr(fmt.Errorf("quantity must be between 1 and 999"))
		}
		group, typ, location := "pickup", "bopis", store
		switch fulfillment {
		case "pickup":
			if !homeDepotStoreID.MatchString(store) {
				return usageErr(fmt.Errorf("pickup requires --store with a Home Depot store ID"))
			}
		case "ship-to-store":
			typ = "boss"
			if !homeDepotStoreID.MatchString(store) {
				return usageErr(fmt.Errorf("ship-to-store requires --store"))
			}
		case "delivery":
			group, typ, location = "delivery", "sth", zip
			if len(zip) != 5 || strings.Trim(zip, "0123456789") != "" {
				return usageErr(fmt.Errorf("delivery requires --zip with five digits"))
			}
		default:
			return usageErr(fmt.Errorf("fulfillment must be pickup, ship-to-store, or delivery"))
		}
		allowed, err := cartWriteAllowed(cmd, flags, "cart add", map[string]any{"item_id": args[0], "quantity": quantity, "fulfillment": fulfillment, "location": location})
		if err != nil || !allowed {
			return err
		}
		if _, _, err = loadHomeDepotSession(); err != nil {
			return err
		}
		c, err := newHomeDepotSessionClient(flags)
		if err != nil {
			return err
		}
		before, err := readHomeDepotCart(cmd.Context(), c)
		if err != nil {
			return err
		}
		baseline := 0
		for _, line := range before.Items {
			if line.ItemID == args[0] && ((fulfillment == "pickup" && line.Fulfillment == "BOPIS") || (fulfillment == "ship-to-store" && line.Fulfillment == "ShipToStore") || (fulfillment == "delivery" && line.Fulfillment == "ShipToHome")) && line.Location == location {
				baseline += line.Quantity
			}
		}
		item := struct {
			ItemID   string `json:"itemId"`
			Quantity string `json:"quantity"`
			Type     string `json:"type"`
			Location string `json:"location"`
		}{args[0], strconv.Itoa(quantity), typ, location}
		request := map[string]any{"filterItem": true, "items": map[string]any{group: []any{item}}}
		if group == "pickup" {
			n, _ := strconv.Atoi(store)
			request["localization"] = map[string]any{"primaryStoreId": n}
		}
		return mutateHomeDepotCart(cmd, flags, "addToCart", request, before, func(after homeDepotCart) bool {
			actual := 0
			for _, line := range after.Items {
				if line.ItemID == args[0] && ((fulfillment == "pickup" && line.Fulfillment == "BOPIS") || (fulfillment == "ship-to-store" && line.Fulfillment == "ShipToStore") || (fulfillment == "delivery" && line.Fulfillment == "ShipToHome")) && line.Location == location {
					actual += line.Quantity
				}
			}
			if actual != baseline+quantity {
				return false
			}
			return homeDepotCartLinesUnchangedExcept(before, after, func(line homeDepotCartLine) bool {
				return line.ItemID == args[0] && line.Location == location && ((fulfillment == "pickup" && line.Fulfillment == "BOPIS") || (fulfillment == "ship-to-store" && line.Fulfillment == "ShipToStore") || (fulfillment == "delivery" && line.Fulfillment == "ShipToHome"))
			})
		})
	}}
	cmd.Flags().IntVar(&quantity, "quantity", 1, "Number of units to add to this fulfillment")
	cmd.Flags().StringVar(&store, "store", "", "Store ID for pickup or ship-to-store")
	cmd.Flags().StringVar(&fulfillment, "fulfillment", "pickup", "pickup, ship-to-store, or delivery")
	cmd.Flags().StringVar(&zip, "zip", "", "Five-digit ZIP code for delivery fulfillment")
	cmd.Flags().Bool("confirm", false, "Confirm this requested cart change without prompting")
	return cmd
}
