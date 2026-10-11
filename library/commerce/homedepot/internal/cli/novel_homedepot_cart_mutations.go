// Copyright 2026 Greg Mushen and contributors. Licensed under Apache-2.0.
package cli

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/homedepot/internal/cliutil"
	"github.com/spf13/cobra"
)

var homeDepotItemID = regexp.MustCompile(`^[0-9]{9}$`)
var homeDepotStoreID = regexp.MustCompile(`^[0-9]{1,5}$`)

func cartWriteAllowed(cmd *cobra.Command, flags *rootFlags, action string, requested map[string]any) (bool, error) {
	if dryRunOK(flags) || (cliutil.IsAnyHarness() && !cliutil.IsDogfoodEnv()) {
		return false, writeDryRun(cmd.OutOrStdout(), flags, action)
	}
	confirm, _ := cmd.Flags().GetBool("confirm")
	if !flags.yes && !confirm {
		c, err := newHomeDepotSessionClient(flags)
		if err != nil {
			return false, err
		}
		cart, err := readHomeDepotCart(cmd.Context(), c)
		if err != nil {
			return false, err
		}
		if lineID, ok := requested["line_id"].(string); ok {
			if _, err := findHomeDepotCartLine(cart, lineID); err != nil {
				return false, err
			}
		}
		return false, flags.printJSON(cmd, map[string]any{"preview": true, "action": action, "requested": requested, "current_item_count": cart.ItemCount, "note": "Pass --confirm or --yes to apply this change."})
	}
	if cliutil.IsAnyHarness() {
		return false, writeDryRun(cmd.OutOrStdout(), flags, action)
	}
	return true, nil
}

func findHomeDepotCartLine(cart homeDepotCart, id string) (homeDepotCartLine, error) {
	for _, line := range cart.Items {
		if line.LineID == id {
			return line, nil
		}
	}
	return homeDepotCartLine{}, notFoundErr(fmt.Errorf("cart line not found; run 'cart show' and use its line_id"))
}

func mutateHomeDepotCart(cmd *cobra.Command, flags *rootFlags, operation string, request map[string]any, before homeDepotCart, confirm func(homeDepotCart) bool) error {
	c, err := newHomeDepotSessionClient(flags)
	if err != nil {
		return err
	}
	c.BaseURL = "https://apionline.homedepot.com"
	c.NoCache = true
	query := fmt.Sprintf("mutation %s($cartRequest: CartInfoRequest!, $requestContext: RequestContext) { %s(cartRequest: $cartRequest, requestContext: $requestContext) { %s } }", operation, operation, "cartId itemCount")

	if operation == "addToCart" {
		query = homeDepotObservedAddQuery
	}
	body := struct {
		Operation string `json:"operationName"`
		Variables any    `json:"variables"`
		Query     string `json:"query"`
	}{operation, map[string]any{"cartRequest": request, "requestContext": map[string]any{"isBrandPricingPolicyCompliant": true}}, query}
	headers := homeDepotCartHeaders()
	headers["X-Experience-Name"] = "my-cart"
	if operation == "addToCart" {
		headers["X-Experience-Name"] = "cart"
	}
	headers["X-HD-DC"] = "origin"
	var trace [16]byte
	if _, err := rand.Read(trace[:]); err != nil {
		return fmt.Errorf("create cart request trace: %w", err)
	}
	traceID := hex.EncodeToString(trace[:])
	headers["X-Cloud-Trace-Context"] = traceID + "/1;o=1"
	headers["X-Parent-Trace-Id"] = traceID + "/1"
	// Cart writes require the browser-observed application header contract.
	// The generated transport and its TLS profile remain unchanged.
	headers["User-Agent"] = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/154.0.0.0 Safari/537.36"
	headers["Sec-CH-UA"] = `"Chromium";v="154", "Google Chrome";v="154", "Not A(Brand";v="99"`
	headers["Sec-CH-UA-Platform"] = `"macOS"`
	headers["Sec-CH-UA-Mobile"] = "?0"
	identity, err := homeDepotCartIdentityHeaders()
	if err != nil {
		return err
	}
	for k, v := range identity {
		headers[k] = v
	}
	if operation != "addToCart" {
		headers["X-Current-URL"] = "/cart"
	}
	c.HTTPClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	_, status, writeErr := c.PostWithParamsAndHeaders(cmd.Context(), "/federation-gateway/graphql", map[string]string{"opname": operation}, body, headers)

	// POST is deliberately not retried. Even a lost response may represent a
	// completed write; observe the cart before deciding whether it worked.
	after, readErr := readHomeDepotCart(cmd.Context(), c)
	if readErr != nil {
		return apiErr(fmt.Errorf("cart change outcome is unknown; inspect 'cart show' before retrying"))
	}
	if after.CartID == before.CartID && confirm(after) {
		return flags.printJSON(cmd, map[string]any{"confirmed": true, "operation": operation, "cart": after})
	}
	if writeErr != nil {
		return apiErr(fmt.Errorf("cart change was not confirmed (HTTP %d); inspect 'cart show' before retrying", status))
	}
	return apiErr(fmt.Errorf("Home Depot did not apply the requested cart change; inspect 'cart show' before retrying"))
}

func cartLineRequest(line homeDepotCartLine, quantity int) (map[string]any, error) {
	typ, group := "", ""
	switch line.Fulfillment {
	case "BOPIS":
		typ, group = "bopis", "pickup"
	case "ShipToStore":
		typ, group = "boss", "pickup"
	case "ShipToHome":
		typ, group = "sth", "delivery"
	default:
		return nil, usageErr(fmt.Errorf("this line's fulfillment is unsupported; change it on homedepot.com"))
	}
	if line.Location == "" {
		return nil, fmt.Errorf("cart line has no fulfillment location; refresh it on homedepot.com")
	}
	item := map[string]any{"id": line.LineID, "itemId": line.ItemID, "quantity": strconv.Itoa(quantity), "type": typ, "location": line.Location, "store": line.Location}
	return map[string]any{"items": map[string]any{group: []any{item}}}, nil
}

// pp:data-source live
func newCartRemoveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "remove <line-id>", Short: "Preview removing one current line ID; --confirm removes it and verifies the resulting cart", Annotations: map[string]string{"pp:data-source": "live", "mcp:read-only": "false", "pp:happy-args": "line=sample-line;--dry-run"}, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
			return usageErr(fmt.Errorf("provide a line_id from 'cart show'"))
		}
		allowed, err := cartWriteAllowed(cmd, flags, "cart remove", map[string]any{"line_id": args[0]})
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
		request := map[string]any{"items": map[string]any{"itemReq": []any{map[string]any{"id": line.LineID}}}}
		return mutateHomeDepotCart(cmd, flags, "deleteItem", request, before, func(after homeDepotCart) bool {
			for _, l := range after.Items {
				if l.LineID == line.LineID {
					return false
				}
			}
			return homeDepotCartOtherLinesUnchanged(before, after, line.LineID)
		})
	}}
	cmd.Flags().Bool("confirm", false, "Confirm removal of this exact cart line")
	cmd.Annotations["pp:verified-by-proof"] = "cart-lifecycle-proof.md"
	return cmd
}

// The browser sends the complete signed THD_CUSTOMER cookie as its cart token.
// Derive account metadata from that same cookie; never reuse captured HAR values.
func homeDepotCartIdentityHeaders() (map[string]string, error) {
	cookies, creds, err := loadHomeDepotSession()
	if err != nil {
		return nil, err
	}
	for _, ck := range cookies {
		if ck.Name != "THD_CUSTOMER" {
			continue
		}
		part := strings.SplitN(ck.Value, ".", 2)[0]
		var decoded []byte
		for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
			decoded, err = enc.DecodeString(part)
			if err == nil {
				break
			}
		}
		var p struct {
			User    string `json:"u"`
			Role    string `json:"r"`
			Type    string `json:"c"`
			Segment string `json:"g"`
		}
		if err != nil || json.Unmarshal(decoded, &p) != nil || strings.TrimSpace(p.User) != creds.UserID {
			continue
		}
		apiCookies := map[string]string{"x-user-id": creds.UserID}
		for _, other := range cookies {
			if other.Name == "tt_search" {
				apiCookies["tt_search"] = other.Value
			}
		}
		encoded, _ := json.Marshal(apiCookies)
		return map[string]string{"X-THD-Customer-Token": ck.Value, "X-User-ID": creds.UserID, "X-SVOC-ID": creds.CustomerAccountID, "X-Customer-Role": p.Role, "X-Customer-Type": p.Type, "X-Segment-ID": p.Segment, "X-API-Cookies": string(encoded), "X-Current-URL": "/cart/atc", "X-Debug": "false"}, nil
	}
	return nil, authErr(fmt.Errorf("no usable signed-in cart identity; import the browser session again"))
}

func homeDepotCartOtherLinesUnchanged(before, after homeDepotCart, exceptID string) bool {
	return homeDepotCartLinesUnchangedExcept(before, after, func(line homeDepotCartLine) bool { return line.LineID == exceptID })
}

func homeDepotCartLinesUnchangedExcept(before, after homeDepotCart, except func(homeDepotCartLine) bool) bool {
	unchanged := func(first, second []homeDepotCartLine) bool {
		for _, a := range first {
			if except(a) {
				continue
			}
			found := false
			for _, b := range second {
				if !except(b) && a.LineID == b.LineID && a.ItemID == b.ItemID && a.Quantity == b.Quantity && a.Fulfillment == b.Fulfillment && a.Location == b.Location {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
		return true
	}
	return unchanged(before.Items, after.Items) && unchanged(after.Items, before.Items)
}
