package cli

import (
	"fmt"
	"github.com/spf13/cobra"
	"strings"
)

// pp:data-source live
func newNovelCartUpdateCmd(flags *rootFlags) *cobra.Command {
	var quantity int
	cmd := &cobra.Command{Use: "update <line-id>", Short: "Preview setting a current line ID to --quantity; --confirm applies and verifies the change", Annotations: map[string]string{"pp:data-source": "live", "mcp:read-only": "false", "pp:verified-by-proof": "cart-lifecycle-proof.md", "pp:happy-args": "line=sample-line;--quantity=2;--dry-run"}, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
			return usageErr(fmt.Errorf("provide a line_id from 'cart show'"))
		}
		if quantity < 1 || quantity > 999 {
			return usageErr(fmt.Errorf("quantity must be between 1 and 999; use 'cart remove' to delete a line"))
		}
		allowed, err := cartWriteAllowed(cmd, flags, "cart update", map[string]any{"line_id": args[0], "quantity": quantity})
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
		line, err := findHomeDepotCartLine(before, args[0])
		if err != nil {
			return err
		}
		if line.Quantity == quantity {
			return flags.printJSON(cmd, map[string]any{"confirmed": true, "changed": false, "cart": before})
		}
		request, err := cartLineRequest(line, quantity)
		if err != nil {
			return err
		}
		request["localization"] = map[string]any{"primaryStoreId": before.StoreID}
		return mutateHomeDepotCart(cmd, flags, "updateCart", request, before, func(after homeDepotCart) bool {
			for _, l := range after.Items {
				if l.LineID == line.LineID && l.ItemID == line.ItemID && l.Fulfillment == line.Fulfillment && l.Location == line.Location && l.Quantity == quantity && homeDepotCartOtherLinesUnchanged(before, after, line.LineID) {
					return true
				}
			}
			return false
		})
	}}
	cmd.Flags().IntVar(&quantity, "quantity", 0, "Total desired quantity for this cart line (required)")
	cmd.Flags().Bool("confirm", false, "Confirm this requested cart change without prompting")
	return cmd
}
