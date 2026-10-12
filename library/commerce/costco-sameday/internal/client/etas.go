// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored helpers for `slots etas` and the guest/member probe: list the
// Costco shops for a ZIP (delivery and pickup) and resolve the Costco
// retailer id. Preserved across regenerate via .printing-press-patches.

package client

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Persisted operations used by `slots etas` (hashes are in the seed file).
const (
	EtaDeliveryMemberOperation  = "GetAccurateRetailerEtas"
	EtaDeliveryVisitorOperation = "GetAccurateVisitorRetailerEtas"
	EtaPickupMemberOperation    = "GetAccurateRetailerPickupEtas"
	EtaPickupVisitorOperation   = "GetAccurateVisitorRetailerPickupEtas"
	retailerIDBySlugOperation   = "GetRetailerIdBySlug"

	// CostcoRetailerIDFallback is Costco's Instacart retailer id. It is used
	// only when neither ShopCollectionScoped nor GetRetailerIdBySlug returns
	// one, and callers report that it was assumed.
	CostcoRetailerIDFallback = "5"
)

// CostcoShop is one Costco shop serving a ZIP.
type CostcoShop struct {
	ID                 string
	RetailerID         string
	RetailerLocationID string
	ServiceType        string
}

// GraphQLQuery runs a persisted GraphQL GET without reading the response
// cache and returns the decoded payload. GraphQL errors with no data come
// back as errors.
func (c *Client) GraphQLQuery(ctx context.Context, op string, vars map[string]any) (map[string]any, error) {
	if c == nil {
		return nil, errors.New("nil client")
	}
	if vars == nil {
		vars = map[string]any{}
	}
	return c.graphQLGet(ctx, op, vars, true)
}

// CostcoShops lists every Costco shop (delivery and pickup) that
// ShopCollectionScoped returns for zip. Coordinates are optional.
func (c *Client) CostcoShops(ctx context.Context, zip string, lat, lng *float64) ([]CostcoShop, error) {
	if err := ValidatePostalCode(zip); err != nil {
		return nil, err
	}
	coords := map[string]any{"latitude": 0, "longitude": 0}
	if lat != nil && lng != nil {
		coords = map[string]any{"latitude": *lat, "longitude": *lng}
	}
	payload, err := c.graphQLGet(withSessionResolving(ctx), shopCollectionScopedOperation, map[string]any{
		"retailerSlug":           costcoRetailerSlug,
		"postalCode":             zip,
		"coordinates":            coords,
		"allowCanonicalFallback": true,
	}, true)
	if err != nil {
		return nil, err
	}
	raw, _ := digJSON(payload, "data", "shopCollection", "shops").([]any)
	var shops []CostcoShop
	for _, s := range raw {
		id := jsonString(digJSON(s, "id"))
		if id == "" {
			continue
		}
		rid := jsonString(digJSON(s, "retailerId"))
		if rid == "" {
			rid = jsonString(digJSON(s, "retailer", "id"))
		}
		shops = append(shops, CostcoShop{
			ID:                 id,
			RetailerID:         rid,
			RetailerLocationID: jsonString(digJSON(s, "retailerLocationId")),
			ServiceType:        strings.ToLower(jsonString(digJSON(s, "serviceType"))),
		})
	}
	if len(shops) == 0 {
		return nil, fmt.Errorf("no Costco shop serves ZIP %s", zip)
	}
	return shops, nil
}

// CostcoRetailerID returns Costco's retailer id and where it came from:
// "shop" (ShopCollectionScoped), "GetRetailerIdBySlug", or "assumed".
func (c *Client) CostcoRetailerID(ctx context.Context, shops []CostcoShop) (string, string) {
	for _, s := range shops {
		if s.RetailerID != "" {
			return s.RetailerID, "shop"
		}
	}
	if payload, err := c.graphQLGet(withSessionResolving(ctx), retailerIDBySlugOperation, map[string]any{"slug": costcoRetailerSlug}, false); err == nil {
		if id := findRetailerID(digJSON(payload, "data")); id != "" {
			return id, retailerIDBySlugOperation
		}
	}
	return CostcoRetailerIDFallback, "assumed"
}

// findRetailerID looks for the first retailer object's id in a response
// whose exact shape is not known offline (retailerBySlug { id } or similar).
func findRetailerID(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		child := m[k]
		if strings.Contains(strings.ToLower(k), "retailer") {
			if id := jsonString(digJSON(child, "id")); id != "" {
				return id
			}
			if s := jsonString(child); s != "" && strings.EqualFold(k, "retailerId") {
				return s
			}
		}
	}
	for _, k := range keys {
		if id := findRetailerID(m[k]); id != "" {
			return id
		}
	}
	return ""
}
