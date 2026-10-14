// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored `order start`: opens a checkout session for the active cart
// with InitializeCheckout, filling cartId, shopId, serviceType and
// userLocation from the session context (the same values 'session context'
// shows), and prints the checkoutSessionId the other checkout commands need.
//
// Response shape (sameday.costco.com public rspack chunk 93526,
// CheckoutSession store): data.initializeCheckout.__typename is
// CheckoutInitializeCheckoutSuccessResponse with
// checkoutData.{checkoutSessionId, cartId, serviceType}; failures are
// CheckoutCheckoutErrors{AddressServiceability,EmptyCart,CheckoutRedirect,
// PickupRetailerLocation,InStoreRetailerLocation}Error.
//
// InitializeCheckout opens a checkout session only: it never charges or
// places an order (that is 'order place', separately gated).
// Registered under the existing `order` parent; preserved across regenerate
// via .printing-press-patches/costco-sameday-order-start.json.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/costco-sameday/internal/client"
	"github.com/spf13/cobra"
)

const (
	initializeCheckoutOp       = "InitializeCheckout"
	initializeCheckoutSuccess  = "CheckoutInitializeCheckoutSuccessResponse"
	orderStartUserLocationHelp = `--user-location is a CheckoutInputsUserLocation JSON object:
  {"postalCode":"<ZIP>","coordinates":{"latitude":<lat>,"longitude":<lng>},"addressId":"<id>"}
All keys are optional; by default the ZIP and coordinates come from the
session context and --address-id adds a saved address ('address list').`
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		for _, child := range root.Commands() {
			if child.Name() == "order" {
				addNovelCommandIfAbsent(child, newOrderStartCmd(flags))
				return
			}
		}
	})
}

func newOrderStartCmd(flags *rootFlags) *cobra.Command {
	var serviceType, shopID, cartID, addressID, userLocation string
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Open a checkout session for the active cart (InitializeCheckout; no charge)",
		Long: strings.TrimSpace(`
Opens a checkout session for your active cart with InitializeCheckout and
prints its checkoutSessionId for 'slots windows', 'slots pick',
'order preview' and 'order place'. cartId, shopId, serviceType and the
location are filled from the session context ('session context' shows
them); flags override any of them. This never charges or places an order.

Shop id vs store location id: --shop-id is the Same-Day shop id
('session context' shop_id). It is NOT the store's retailer location id
(retailer_location_id, the number inside item ids). Passing the location id
as the shop makes Costco answer "No items are available in your cart" even
when the cart is full, so 'order start' refuses a location id.

--service-type is delivery (default) or pickup, in any case.

` + orderStartUserLocationHelp),
		Example: `  costco-sameday-pp-cli order start
  costco-sameday-pp-cli order start --zip 98027 --json
  costco-sameday-pp-cli order start --service-type pickup
  costco-sameday-pp-cli order start --dry-run`,
		Annotations: map[string]string{"pp:narrative": "order.start", "pp:charge": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := client.NormalizeServiceType(serviceType)
			if err != nil {
				return usageErr(err)
			}
			if st != "DELIVERY" && st != "PICKUP" {
				return usageErr(fmt.Errorf("--service-type must be delivery or pickup (got %q)", serviceType))
			}
			var loc map[string]any
			if strings.TrimSpace(userLocation) != "" {
				if err := json.Unmarshal([]byte(userLocation), &loc); err != nil || loc == nil {
					return usageErr(fmt.Errorf("--user-location must be a JSON object\n%s", orderStartUserLocationHelp))
				}
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				return printNarrativeJSON(cmd, flags, map[string]any{
					"dry_run":   true,
					"operation": initializeCheckoutOp,
					"would":     "resolve session context (shop, cart, ZIP), then POST /graphql InitializeCheckout (opens a checkout session; no charge)",
				})
			}
			ctx := cmd.Context()
			plan, err := resolveOrderStart(cmd, c, st, strings.TrimSpace(shopID), strings.TrimSpace(cartID), strings.TrimSpace(addressID), loc)
			if err != nil {
				return err
			}
			body := map[string]any{
				"operationName": initializeCheckoutOp,
				"variables":     plan.variables(),
				"extensions":    map[string]any{"persistedQuery": map[string]any{"version": 1}},
			}
			data, status, err := c.PostWithParams(client.WithDeclaredGraphQLOperation(ctx, initializeCheckoutOp), "/graphql", map[string]string{"operationName": initializeCheckoutOp}, body)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			out, failure := orderStartResult(data, status, plan)
			if perr := printNarrativeJSON(cmd, flags, out); perr != nil {
				return perr
			}
			if failure != "" {
				return apiErr(fmt.Errorf("InitializeCheckout did not open a checkout: %s", failure))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&serviceType, "service-type", "delivery", "delivery or pickup (any case)")
	cmd.Flags().StringVar(&shopID, "shop-id", "", "Same-Day shop id ('session context' shop_id; NOT the store retailer location id)")
	cmd.Flags().StringVar(&cartID, "cart-id", "", "Cart id (default: the active cart for the shop)")
	cmd.Flags().StringVar(&addressID, "address-id", "", "Saved delivery address id from 'address list' (optional)")
	cmd.Flags().StringVar(&userLocation, "user-location", "", "CheckoutInputsUserLocation JSON, e.g. {\"postalCode\":\"<ZIP>\",\"coordinates\":{\"latitude\":0,\"longitude\":0}} (default: from the session context)")
	return cmd
}

type orderStartPlan struct {
	ServiceType string
	ShopID      string
	CartID      string
	Location    map[string]any
	Sources     map[string]string
}

func (p orderStartPlan) variables() map[string]any {
	return map[string]any{
		"cartId":                         p.CartID,
		"shopId":                         p.ShopID,
		"serviceType":                    p.ServiceType,
		"userLocation":                   p.Location,
		"checkoutCreationTrackingParams": map[string]any{"pageViewId": client.NewPageViewID()},
	}
}

// resolveOrderStart fills the InitializeCheckout inputs from flags, then the
// session context. An explicit --shop-id that is really a store location id
// is refused with the right shop id.
func resolveOrderStart(cmd *cobra.Command, c *client.Client, serviceType, shopID, cartID, addressID string, loc map[string]any) (orderStartPlan, error) {
	ctx := cmd.Context()
	plan := orderStartPlan{ServiceType: serviceType, Sources: map[string]string{}}
	// An explicit --user-location ZIP drives session resolution, so the
	// shop and cart are picked for that ZIP. With location, shop and cart
	// all supplied, no lookup is needed.
	explicitZip, _ := loc["postalCode"].(string)
	explicitZip = strings.TrimSpace(explicitZip)
	var sc client.SessionContext
	if explicitZip != "" {
		if err := client.ValidatePostalCode(explicitZip); err != nil {
			return plan, usageErr(fmt.Errorf("--user-location postalCode: %w", err))
		}
		c.SetSessionPostalCode(explicitZip)
	}
	if explicitZip != "" && shopID != "" && cartID != "" {
		sc.PostalCode = explicitZip
		sc.Latitude, sc.Longitude = locationCoordinates(loc)
	} else {
		var err error
		sc, err = c.ResolveSessionContext(ctx, client.SessionNeeds{Location: true})
		if err != nil {
			return plan, classifyAPIErrorOnly(err)
		}
		if explicitZip != "" && sc.PostalCode != explicitZip {
			// Geolocation coordinates belong to another ZIP; use only the
			// caller's.
			sc.PostalCode = explicitZip
			sc.Latitude, sc.Longitude = locationCoordinates(loc)
		}
	}
	shops, shopsErr := c.CostcoShops(ctx, sc.PostalCode, sc.Latitude, sc.Longitude)
	want := strings.ToLower(serviceType)
	if shopID != "" {
		if shopsErr == nil {
			if err := checkShopIDNotLocation(shopID, shops, want); err != nil {
				return plan, err
			}
		}
		plan.ShopID, plan.Sources["shop_id"] = shopID, "flag"
	} else {
		if shopsErr != nil {
			return plan, classifyAPIErrorOnly(fmt.Errorf("could not look up the Costco shop for ZIP %s: %w", sc.PostalCode, shopsErr))
		}
		for _, s := range shops {
			if s.ServiceType == want && s.ID != "" {
				plan.ShopID = s.ID
				break
			}
		}
		if plan.ShopID == "" {
			return plan, fmt.Errorf("no Costco %s shop is listed for ZIP %s (pass --zip or --shop-id)", want, sc.PostalCode)
		}
		plan.Sources["shop_id"] = "session context"
	}
	if cartID != "" {
		plan.CartID, plan.Sources["cart_id"] = cartID, "flag"
	} else {
		cs, err := c.ResolveSessionContext(ctx, client.SessionNeeds{Cart: true, ShopID: plan.ShopID})
		if err != nil {
			return plan, classifyAPIErrorOnly(err)
		}
		if cs.CartID == "" {
			return plan, fmt.Errorf("no active cart for shop %s; add items first or pass --cart-id", plan.ShopID)
		}
		plan.CartID, plan.Sources["cart_id"] = cs.CartID, "session context"
	}
	if loc == nil {
		loc = map[string]any{}
		plan.Sources["user_location"] = "session context"
	} else {
		plan.Sources["user_location"] = "flag"
	}
	if _, ok := loc["postalCode"]; !ok && sc.PostalCode != "" {
		loc["postalCode"] = sc.PostalCode
	}
	if _, ok := loc["coordinates"]; !ok && sc.Latitude != nil && sc.Longitude != nil {
		loc["coordinates"] = map[string]any{"latitude": *sc.Latitude, "longitude": *sc.Longitude}
	}
	if addressID != "" {
		loc["addressId"] = addressID
	}
	plan.Location = loc
	return plan, nil
}

// checkShopIDNotLocation refuses a --shop-id that matches a store's retailer
// location id (and no shop id) for this ZIP.
func checkShopIDNotLocation(shopID string, shops []client.CostcoShop, serviceType string) error {
	isShop := false
	var asLocation *client.CostcoShop
	for i, s := range shops {
		if s.ID == shopID {
			isShop = true
		}
		if s.RetailerLocationID == shopID && asLocation == nil {
			asLocation = &shops[i]
		}
	}
	if isShop || asLocation == nil {
		return nil
	}
	right := ""
	for _, s := range shops {
		if s.ServiceType == serviceType && s.RetailerLocationID == asLocation.RetailerLocationID {
			right = s.ID
		}
	}
	if right == "" {
		right = asLocation.ID
	}
	return usageErr(fmt.Errorf("--shop-id %s is a store location id (retailer_location_id), not a Same-Day shop id; use --shop-id %s (see 'session context'), or omit --shop-id", shopID, right))
}

// orderStartResult turns the InitializeCheckout reply into the printed
// result and a failure reason ("" on success).
func orderStartResult(data []byte, status int, plan orderStartPlan) (map[string]any, string) {
	out := map[string]any{
		"operation":    initializeCheckoutOp,
		"http_status":  status,
		"service_type": strings.ToLower(plan.ServiceType),
		"shop_id":      plan.ShopID,
		"cart_id":      plan.CartID,
		"sources":      plan.Sources,
		"started":      false,
	}
	var resp map[string]any
	if err := json.Unmarshal(data, &resp); err != nil || resp == nil {
		return out, "response was not a JSON object"
	}
	if report := detectGraphQLFailure(resp); report != nil {
		out["error"] = report.Message
		if report.Field != "errors" {
			out["error_type"] = strings.SplitN(report.Message, ":", 2)[0]
		}
		msg := report.Message
		if strings.Contains(strings.ToLower(msg), "empty_cart") || strings.Contains(msg, "EmptyCartError") || strings.Contains(strings.ToLower(msg), "no items are available") {
			hint := "Costco saw no items for this shop and cart. Check that --shop-id is the Same-Day shop id from 'session context' (not the store retailer location id) and that --cart-id is the active cart for that shop"
			out["hint"] = hint
			msg += "\nhint: " + hint
		}
		return out, msg
	}
	ic, _ := digAny(resp, "data", "initializeCheckout").(map[string]any)
	if ic == nil {
		return out, "response had no data.initializeCheckout"
	}
	tn, _ := ic["__typename"].(string)
	sid := strAt(ic, "checkoutData", "checkoutSessionId")
	if tn != initializeCheckoutSuccess || sid == "" {
		return out, fmt.Sprintf("unexpected result %q without a checkoutSessionId", tn)
	}
	out["started"] = true
	out["checkout_session_id"] = sid
	if cid := strAt(ic, "checkoutData", "cartId"); cid != "" {
		out["cart_id"] = cid
	}
	out["next"] = []string{
		"costco-sameday-pp-cli slots windows --checkout-session-id " + sid,
		"costco-sameday-pp-cli order preview --checkout-session-id " + sid,
	}
	return out, ""
}

// locationCoordinates reads coordinates from a --user-location object.
func locationCoordinates(loc map[string]any) (*float64, *float64) {
	coords, _ := loc["coordinates"].(map[string]any)
	lat, okLat := coords["latitude"].(float64)
	lng, okLng := coords["longitude"].(float64)
	if !okLat || !okLng {
		return nil, nil
	}
	return &lat, &lng
}
