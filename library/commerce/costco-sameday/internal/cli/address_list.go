// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: `address list` shows saved delivery address ids with a masked
// label (read-only UserAddresses query). The full street is never printed.
// Preserved across regenerate via registerNovelCommand and
// .printing-press-patches/costco-sameday-address-list.json.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/costco-sameday/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/costco-sameday/internal/cliutil"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		var parent *cobra.Command
		for _, child := range root.Commands() {
			if child.Name() == "address" {
				parent = child
			}
		}
		if parent == nil {
			parent = &cobra.Command{Use: "address", Short: "Saved delivery addresses (read-only)"}
			root.AddCommand(parent)
		}
		addNovelCommandIfAbsent(parent, newAddressListCmd(flags))
	})
}

type addressRow struct {
	Index      int    `json:"index"`
	AddressID  string `json:"address_id"`
	Label      string `json:"label"`
	MatchesZIP bool   `json:"matches_zip,omitempty"`
}

func newAddressListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List saved delivery address ids with a masked label (read-only)",
		Long: strings.TrimSpace(`
Lists the signed-in account's saved delivery addresses using the read-only
UserAddresses query: the address id (for --address-id flags such as
'cart cartsignaledeta') and a masked label (first letter of each word, ZIP
prefix). The full street is never printed.

'cart cartsignaledeta' fills --address-id itself from the saved address whose
ZIP matches the session ZIP, so this is only needed to pick another one.`),
		Example: `  costco-sameday-pp-cli address list
  costco-sameday-pp-cli address list --zip 98027 --json`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:narrative": "address.list"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if c.IsDryRun() || cliutil.IsVerifyEnv() {
				return flags.printJSON(cmd, map[string]any{"dry_run": true, "would": "GET /graphql UserAddresses"})
			}
			addrs, err := c.UserAddresses(cmd.Context())
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			zip := strings.TrimSpace(sessionZipFlag)
			if zip == "" {
				if sc, err := c.ResolveSessionContext(cmd.Context(), client.SessionNeeds{Location: true}); err == nil {
					zip = sc.PostalCode
				}
			}
			chosen, how, pickErr := client.PickAddressID(addrs, zip)
			rows := make([]addressRow, 0, len(addrs))
			for i, a := range addrs {
				rows = append(rows, addressRow{Index: i + 1, AddressID: a.ID, Label: client.MaskedAddressLabel(a), MatchesZIP: zip != "" && a.PostalCode == zip})
			}
			out := map[string]any{"addresses": rows}
			if chosen != "" {
				out["auto_address_id"] = chosen
				out["auto_reason"] = how
			}
			if pickErr != nil {
				out["auto_note"] = "several saved addresses match the ZIP; pass --address-id explicitly"
			}
			if len(rows) == 0 {
				out["note"] = "no saved delivery addresses (guest session, or none saved)"
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				w := cmd.OutOrStdout()
				for _, r := range rows {
					mark := ""
					if r.AddressID == chosen {
						mark = "  (auto)"
					}
					fmt.Fprintf(w, "%2d  %s  %s%s\n", r.Index, r.AddressID, r.Label, mark)
				}
				if len(rows) == 0 {
					fmt.Fprintln(w, out["note"])
				}
				if pickErr != nil {
					fmt.Fprintln(w, out["auto_note"])
				}
				return nil
			}
			return flags.printJSON(cmd, out)
		},
	}
	return cmd
}
