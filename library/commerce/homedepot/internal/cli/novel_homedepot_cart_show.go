// Copyright 2026 Greg Mushen and contributors. Licensed under Apache-2.0.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/homedepot/internal/client"
)

const homeDepotCartFields = `cartId itemCount items { id quantity selectedFulfillment product { itemId identifiers { productLabel brandName modelNumber canonicalUrl } pricing { value } fulfillment { fulfillmentOptions { type services { type selected locations { locationId } } } } } } localization { primaryStoreId } totals { total }`
const homeDepotGetCartQuery = `query getCart($requestContext: RequestContext) { cartInfo(requestContext: $requestContext) { ` + homeDepotCartFields + ` } }`

type homeDepotCartLine struct {
	LineID      string   `json:"line_id"`
	ItemID      string   `json:"item_id"`
	Name        string   `json:"name"`
	Quantity    int      `json:"quantity"`
	Fulfillment string   `json:"fulfillment,omitempty"`
	Location    string   `json:"location,omitempty"`
	Brand       string   `json:"brand,omitempty"`
	Model       string   `json:"model,omitempty"`
	URL         string   `json:"url,omitempty"`
	UnitPrice   *float64 `json:"unit_price,omitempty"`
}
type homeDepotCart struct {
	CartID    string              `json:"cart_id"`
	ItemCount int                 `json:"item_count"`
	Items     []homeDepotCartLine `json:"items"`
	StoreID   json.Number         `json:"store_id,omitempty"`
	Total     *float64            `json:"total,omitempty"`
}
type homeDepotCartWire struct {
	CartID    string `json:"cartId"`
	ItemCount int    `json:"itemCount"`
	Items     []struct {
		ID                  string      `json:"id"`
		Quantity            json.Number `json:"quantity"`
		SelectedFulfillment string      `json:"selectedFulfillment"`
		Product             struct {
			ItemID      string `json:"itemId"`
			Identifiers struct {
				Name  string `json:"productLabel"`
				Brand string `json:"brandName"`
				Model string `json:"modelNumber"`
				URL   string `json:"canonicalUrl"`
			} `json:"identifiers"`
			Pricing struct {
				Value *float64 `json:"value"`
			} `json:"pricing"`
			Fulfillment struct {
				Options []struct {
					Services []struct {
						Type      string `json:"type"`
						Selected  bool   `json:"selected"`
						Locations []struct {
							ID string `json:"locationId"`
						} `json:"locations"`
					} `json:"services"`
				} `json:"fulfillmentOptions"`
			} `json:"fulfillment"`
		} `json:"product"`
	} `json:"items"`
	Localization struct {
		Store json.Number `json:"primaryStoreId"`
	} `json:"localization"`
	Totals struct {
		Total *float64 `json:"total"`
	} `json:"totals"`
}

func homeDepotCartHeaders() map[string]string {
	return map[string]string{"Accept": "*/*", "X-Experience-Name": "my-cart", "Origin": "https://www.homedepot.com", "Referer": "https://www.homedepot.com/", "Sec-Fetch-Site": "same-site", "Sec-Fetch-Mode": "cors", "Sec-Fetch-Dest": "empty"}
}
func homeDepotCartContext() map[string]any {
	return map[string]any{"optimalFulfillment": true, "isBrandPricingPolicyCompliant": true, "isLite": false}
}

func decodeHomeDepotCart(raw []byte, field string) (homeDepotCart, error) {
	var result struct {
		Data   map[string]json.RawMessage `json:"data"`
		Errors []json.RawMessage          `json:"errors"`
		Error  []json.RawMessage          `json:"error"`
	}
	if json.Unmarshal(raw, &result) != nil {
		return homeDepotCart{}, fmt.Errorf("invalid Home Depot cart response")
	}
	body := result.Data[field]
	if len(body) == 0 || string(body) == "null" || len(result.Errors) > 0 || len(result.Error) > 0 {
		return homeDepotCart{}, fmt.Errorf("Home Depot rejected the cart request; re-import the session if it expired")
	}
	var wire homeDepotCartWire
	if json.Unmarshal(body, &wire) != nil {
		return homeDepotCart{}, fmt.Errorf("invalid Home Depot cart fields")
	}
	cart := homeDepotCart{CartID: wire.CartID, ItemCount: wire.ItemCount, StoreID: wire.Localization.Store, Total: wire.Totals.Total, Items: make([]homeDepotCartLine, 0, len(wire.Items))}
	if cart.CartID == "" {
		return homeDepotCart{}, fmt.Errorf("Home Depot response has no cart identity")
	}
	for _, item := range wire.Items {
		quantity, err := strconv.Atoi(item.Quantity.String())
		if err != nil || quantity < 1 {
			return homeDepotCart{}, fmt.Errorf("invalid Home Depot cart quantity")
		}
		if item.ID == "" || item.Product.ItemID == "" {
			return homeDepotCart{}, fmt.Errorf("cart line is missing its identity")
		}
		line := homeDepotCartLine{LineID: item.ID, ItemID: item.Product.ItemID, Name: item.Product.Identifiers.Name, Quantity: quantity, Fulfillment: normalizeHomeDepotFulfillment(item.SelectedFulfillment), Brand: item.Product.Identifiers.Brand, Model: item.Product.Identifiers.Model, UnitPrice: item.Product.Pricing.Value}
		if strings.HasPrefix(item.Product.Identifiers.URL, "/") {
			line.URL = "https://www.homedepot.com" + item.Product.Identifiers.URL
		}
		for _, option := range item.Product.Fulfillment.Options {
			for _, service := range option.Services {
				if service.Selected && len(service.Locations) > 0 {
					line.Location = service.Locations[0].ID
				}
			}
		}
		cart.Items = append(cart.Items, line)
	}
	return cart, nil
}

func readHomeDepotCart(ctx context.Context, c *client.Client) (homeDepotCart, error) {
	c.BaseURL = "https://apionline.homedepot.com"
	c.NoCache = true
	body := map[string]any{"operationName": "getCart", "query": homeDepotGetCartQuery, "variables": map[string]any{"requestContext": homeDepotCartContext()}}
	raw, status, err := c.PostQueryWithParamsAndHeaders(ctx, "/federation-gateway/graphql", map[string]string{"opname": "getCart"}, body, homeDepotCartHeaders())
	if err != nil {
		return homeDepotCart{}, homeDepotRequestError("read Home Depot cart", status, err)
	}
	if status != 200 {
		return homeDepotCart{}, fmt.Errorf("Home Depot cart returned HTTP %d", status)
	}
	return decodeHomeDepotCart(raw, "cartInfo")
}
