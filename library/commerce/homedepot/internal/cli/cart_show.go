package cli

import (
	"github.com/spf13/cobra"
	"strings"
)

// pp:data-source live
func newCartShowCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "show", Short: "Show cart line IDs, quantities, fulfillment and total", Args: cobra.NoArgs, Annotations: map[string]string{"pp:data-source": "live", "mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "cart show")
		}
		if _, _, err := loadHomeDepotSession(); err != nil {
			return err
		}
		c, err := newHomeDepotSessionClient(flags)
		if err != nil {
			return err
		}
		cart, err := readHomeDepotCart(cmd.Context(), c)
		if err != nil {
			return err
		}
		return flags.printJSON(cmd, cart)
	}}
}

func normalizeHomeDepotFulfillment(value string) string {
	switch strings.ToLower(value) {
	case "bopis", "pickup":
		return "BOPIS"
	case "boss", "shiptostore":
		return "ShipToStore"
	case "sth", "shiptohome":
		return "ShipToHome"
	default:
		return value
	}
}
