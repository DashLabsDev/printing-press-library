// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/walmart/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/walmart/internal/walmart"
)

// PATCH(walmart-cart-slot-commands): cart view + affiliate add-to-cart link
// builder, and slots list. The CLI never mutates the cart: cart link only
// prints (or optionally opens) a Walmart affiliate URL that ADDS to whatever
// is already in the user's browser cart. Checkout, order placement, cart
// merge, pickup/delivery mode switching, update/remove and slot reserve are
// never sent.

const cartIDHelp = `The cart id is resolved from --cart-id, then WALMART_CART_ID, then the id
remembered from the last cart response this CLI received (stored locally in
the CLI's data directory). Walmart's web app keeps the id in browser storage
and only returns it from cart responses, so pass --cart-id once.`

type cartIDState struct {
	CartID string `json:"cartId"`
}

func cartIDStatePath() (string, error) {
	dir, err := cliutil.DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "cart.json"), nil
}

// resolveCartID returns the cart id and where it came from.
func resolveCartID(flagVal string) (string, string, error) {
	if v := strings.TrimSpace(flagVal); v != "" {
		if !walmart.ValidCartID(v) {
			return "", "", usageErr(errors.New("--cart-id must be a 36-character UUID"))
		}
		return v, "flag", nil
	}
	if v := strings.TrimSpace(os.Getenv("WALMART_CART_ID")); v != "" {
		if !walmart.ValidCartID(v) {
			return "", "", usageErr(errors.New("WALMART_CART_ID must be a 36-character UUID"))
		}
		return v, "env", nil
	}
	if p, err := cartIDStatePath(); err == nil {
		if b, err := os.ReadFile(p); err == nil {
			var st cartIDState
			if json.Unmarshal(b, &st) == nil && walmart.ValidCartID(st.CartID) {
				return st.CartID, "remembered", nil
			}
		}
	}
	return "", "", usageErr(errors.New("no cart id yet: pass --cart-id once (or set WALMART_CART_ID).\n" + cartIDHelp))
}

// rememberCartID stores the cart id a cart response returned.
func rememberCartID(id string) {
	if !walmart.ValidCartID(id) {
		return
	}
	p, err := cartIDStatePath()
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return
	}
	b, _ := json.Marshal(cartIDState{CartID: id})
	_ = os.WriteFile(p, b, 0o600)
}

func fetchCart(cmd *cobra.Command, flags *rootFlags, cartID string) (*walmart.CartView, error) {
	vars, err := walmart.CartVariables(cartID)
	if err != nil {
		return nil, usageErr(err)
	}
	raw, err := orchestraGet(cmd, flags, walmart.SubgraphGetCart, walmart.OpGetCart, walmart.HashGetCart, vars, walmart.CartPage, "", nil)
	if err != nil {
		return nil, err
	}
	v, err := walmart.ParseCart(raw)
	if err != nil {
		return nil, err
	}
	if v.CartID != "" {
		rememberCartID(v.CartID)
	}
	return v, nil
}

func printCart(w io.Writer, v *walmart.CartView) {
	fmt.Fprintf(w, "Mode %s  Store %s  Slot reserved: %v\n", v.Intent, v.StoreID, v.SlotReserved)
	if r := v.Reservation; r != nil && r.Window != "" {
		fmt.Fprintf(w, "Reserved %s %s (hold until %s)\n", dayOf(r.Start), r.Window, r.HoldUntil)
	}
	if len(v.Items) == 0 {
		fmt.Fprintln(w, "Cart is empty.")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "QTY\tPRICE\tITEM ID\tNAME")
	for _, it := range v.Items {
		fmt.Fprintf(tw, "%g\t%s\t%s\t%s\n", it.Quantity, it.PriceDisplay, it.USItemID, it.Name)
	}
	_ = tw.Flush()
	if v.Total != 0 {
		fmt.Fprintf(w, "\nSubtotal %.2f  Estimated total %.2f\n", v.SubTotal, v.Total)
	}
}

func dayOf(rfc string) string {
	if len(rfc) >= 10 {
		return rfc[:10]
	}
	return rfc
}

func newCartCmd(flags *rootFlags) *cobra.Command {
	parent := &cobra.Command{
		Use:         "cart",
		Short:       "View the cart, or build a Walmart add-to-cart link (CLI never mutates the cart)",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	var cartID string
	view := &cobra.Command{
		Use:   "view",
		Short: "Show cart items, mode, store and the reserved time slot (read only)",
		Long:  "Show the cart read-only via the getCart query.\n\n" + cartIDHelp,
		Example: strings.Trim(`
  walmart-pp-cli cart view --cart-id 00000000-0000-0000-0000-000000000000
  walmart-pp-cli cart view --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			id, _, err := resolveCartID(cartID)
			if err != nil {
				return err
			}
			v, err := fetchCart(cmd, flags, id)
			if err != nil {
				return err
			}
			if wantJSON(cmd, flags) {
				return writeJSON(cmd.OutOrStdout(), v)
			}
			printCart(cmd.OutOrStdout(), v)
			return nil
		},
	}
	view.Flags().StringVar(&cartID, "cart-id", "", "Cart id (or WALMART_CART_ID; remembered after first use)")
	parent.AddCommand(view)
	link := newCartLinkCmd(flags)
	parent.AddCommand(link)
	// Alias: cart add → same link builder (print URL; optional --open).
	add := newCartLinkCmd(flags)
	add.Use = "add <item[:qty]> [<item[:qty]>...]"
	add.Short = "Alias for cart link (prints an add-to-cart URL; never mutates the cart)"
	parent.AddCommand(add)
	return parent
}

// cartLinkPlan is the structured result of cart link / cart add.
type cartLinkPlan struct {
	Action      string                  `json:"action"`
	URL         string                  `json:"url"`
	Items       []walmart.AffilCartItem `json:"items"`
	Note        string                  `json:"note"`
	Instruction string                  `json:"next,omitempty"`
	Opened      bool                    `json:"opened,omitempty"`
}

// parseLinkItemArgs accepts item ids / product URLs, optionally with :qty
// (e.g. 51259338:2). A shared --qty applies when an arg has no :qty suffix.
func parseLinkItemArgs(args []string, defaultQty int) ([]walmart.AffilCartItem, error) {
	if defaultQty < 1 || defaultQty > walmart.MaxCartLineQuantity {
		return nil, fmt.Errorf("--qty must be 1-%d", walmart.MaxCartLineQuantity)
	}
	items := make([]walmart.AffilCartItem, 0, len(args))
	for _, a := range args {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		qty := defaultQty
		raw := a
		if i := strings.LastIndex(a, ":"); i > 0 {
			// Only treat as qty when the suffix is all digits (URLs use ://).
			suf := a[i+1:]
			if suf != "" && isAllDigits(suf) {
				q, err := strconv.Atoi(suf)
				if err != nil || q < 1 || q > walmart.MaxCartLineQuantity {
					return nil, fmt.Errorf("quantity in %q must be 1-%d", a, walmart.MaxCartLineQuantity)
				}
				qty = q
				raw = a[:i]
			}
		}
		id, err := walmart.NormalizeItemID(raw)
		if err != nil {
			return nil, err
		}
		items = append(items, walmart.AffilCartItem{USItemID: id, Quantity: qty})
	}
	if len(items) == 0 {
		return nil, errors.New("at least one item id (or product URL) is required")
	}
	return items, nil
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}

func newCartLinkCmd(flags *rootFlags) *cobra.Command {
	var qty int
	var open bool
	cmd := &cobra.Command{
		Use:   "link <item[:qty]> [<item[:qty]>...]",
		Short: "Print one Walmart affiliate add-to-cart URL (ADDS to the existing cart; never mutates via API)",
		Long: `Build ONE Walmart affiliate add-to-cart URL for the given items and print it.

  https://affil.walmart.com/cart/addToCart?items=ID|qty,ID|qty

Open the URL in your own browser. You do not need to be signed in: items
land in a guest cart and carry over into your account cart when you sign in.
When you are already signed in, the link ADDS / INCREMENTS quantity on
whatever is already in that browser cart — it does not replace the cart, set
absolute quantities, remove lines, reserve a slot, or check out. The CLI
never POSTs a cart mutation and never touches your cart cookies.

Pass item ids from search / product output (US item ids). Optional per-item
quantity as item:qty (default 1, or --qty for every arg without :qty).

Default: print the URL only. Pass --open (or --launch) to also open it in
your default browser. Under Printing Press verify/dogfood harnesses the
URL is still printed but the browser is never launched.
`,
		Example: strings.Trim(`
  walmart-pp-cli cart link 51259338
  walmart-pp-cli cart link 51259338:2 44391152:1
  walmart-pp-cli cart link 51259338 44391152 --qty 2
  walmart-pp-cli cart link 51259338:2 --open
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "item=51259338;item=44391152:2",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usageErr(errors.New("at least one item id (or product URL) is required\n\n" + cmd.UsageString()))
			}
			items, err := parseLinkItemArgs(args, qty)
			if err != nil {
				return usageErr(err)
			}
			link, err := walmart.AffilAddToCartURL(items)
			if err != nil {
				return usageErr(err)
			}
			plan := cartLinkPlan{
				Action: "link",
				URL:    link,
				Items:  items,
				Note:   "ADDS to whatever is already in the browser cart (increments qty). Works signed out (guest cart; items carry over on sign-in) or signed in. Open the URL yourself to apply; the CLI never mutates the cart.",
			}
			launch := open && !flags.noInput && !flags.dryRun
			if launch && cliutil.IsAnyHarness() {
				if err := writeHarnessRefusal(cmd.OutOrStdout(), flags, "launch browser"); err != nil {
					return err
				}
				// Still print the URL so harnesses can assert on it.
				if !flags.asJSON {
					fmt.Fprintln(cmd.OutOrStdout(), link)
				}
				return nil
			}
			if launch {
				if err := openInDefaultBrowser(link); err != nil {
					return err
				}
				plan.Opened = true
				plan.Instruction = "opened in your default browser; pick a time and check out yourself on walmart.com"
			} else {
				plan.Instruction = "copy the url into your browser (or re-run with --open)"
			}
			// Bare URL unless the user asked for --json/--agent. Non-TTY
			// alone must not force JSON so the URL stays pipeable.
			if flags.asJSON {
				return writeJSON(cmd.OutOrStdout(), plan)
			}
			fmt.Fprintln(cmd.OutOrStdout(), link)
			if isTerminal(cmd.OutOrStdout()) {
				fmt.Fprintln(cmd.OutOrStdout(), "Note: this link ADDS to whatever is already in your Walmart cart (it increments qty). Works signed out (guest cart; items carry over on sign-in) or signed in. The CLI does not change the cart.")
				if plan.Opened {
					fmt.Fprintln(cmd.OutOrStdout(), "Opened in your default browser. Pick a time and check out yourself on walmart.com.")
				}
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&qty, "qty", 1, "Default quantity for args without :qty (the link increments)")
	cmd.Flags().BoolVar(&open, "open", false, "Also open the URL in your default browser (default: print only)")
	cmd.Flags().BoolVar(&open, "launch", false, "Alias for --open")
	return cmd
}

func newSlotsCmd(flags *rootFlags) *cobra.Command {
	parent := &cobra.Command{
		Use:         "slots",
		Short:       "List pickup/delivery time slots for the cart's current mode (read only; never reserves)",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	var cartID string
	var onlyAvailable bool
	list := &cobra.Command{
		Use:   "list",
		Short: "List time slots for the cart's current pickup/delivery mode (read only; never switches mode)",
		Long: `List bookable time slots via the getSlots query, for whatever mode
(Pickup or Delivery) the cart is already in. The mode is read from the cart
and never changed. This CLI does not reserve a slot — open walmart.com/cart
and pick a time yourself after adding items via cart link.

` + cartIDHelp,
		Example: strings.Trim(`
  walmart-pp-cli slots list
  walmart-pp-cli slots list --available --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			id, _, err := resolveCartID(cartID)
			if err != nil {
				return err
			}
			cart, err := fetchCart(cmd, flags, id)
			if err != nil {
				return err
			}
			mode, err := walmart.SlotMode(cart.Intent)
			if err != nil {
				return err
			}
			raw, err := fetchSlotsRaw(cmd, flags, id, mode)
			if err != nil {
				return err
			}
			v, err := walmart.ParseSlots(raw, mode)
			if err != nil {
				return err
			}
			if onlyAvailable {
				for i := range v.Days {
					kept := v.Days[i].Slots[:0]
					for _, s := range v.Days[i].Slots {
						if s.Available {
							kept = append(kept, s)
						}
					}
					v.Days[i].Slots = kept
				}
			}
			if wantJSON(cmd, flags) {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"slots": v, "reservation": cart.Reservation})
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Mode %s", mode)
			if r := cart.Reservation; r != nil && r.Window != "" && !r.Expired {
				fmt.Fprintf(w, "  Reserved %s %s (hold until %s)", dayOf(r.Start), r.Window, r.HoldUntil)
			}
			fmt.Fprintln(w)
			tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
			fmt.Fprintln(tw, "DAY\tWINDOW\tFEE\tAVAILABLE\tSLOT ID")
			for _, d := range v.Days {
				for _, s := range d.Slots {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%v\t%s\n", d.Day, s.Window, s.Fee, s.Available, s.ID)
				}
			}
			return tw.Flush()
		},
	}
	list.Flags().StringVar(&cartID, "cart-id", "", "Cart id (or WALMART_CART_ID; remembered after first use)")
	list.Flags().BoolVar(&onlyAvailable, "available", false, "Only show available slots")
	parent.AddCommand(list)
	return parent
}

func fetchSlotsRaw(cmd *cobra.Command, flags *rootFlags, cartID, mode string) ([]byte, error) {
	vars, err := walmart.SlotsVariables(cartID, mode)
	if err != nil {
		return nil, usageErr(err)
	}
	return orchestraGet(cmd, flags, walmart.SubgraphCartXO, walmart.OpGetSlots, walmart.HashGetSlots, vars, walmart.CartPage, "", walmart.SlotHeaders())
}
