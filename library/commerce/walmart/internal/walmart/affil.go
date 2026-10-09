// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.

package walmart

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// AffilAddToCartHost is Walmart's affiliate add-to-cart host. Opening a link
// there in a signed-in browser 307s onto www.walmart.com and adds the items
// using the browser's own session (no CLI GraphQL write).
const AffilAddToCartHost = "https://affil.walmart.com"

// AffilAddToCartPath is the path of the affiliate add-to-cart endpoint.
const AffilAddToCartPath = "/cart/addToCart"

// MaxAffilItems is a soft upper bound taken from the affiliate page's
// cart meta.maxItemsInOrder default (12). The link builder refuses more.
const MaxAffilItems = 12

// AffilCartItem is one line of an affiliate add-to-cart link. Quantity is the
// amount to ADD (the link increments; it does not set absolute quantity).
type AffilCartItem struct {
	USItemID string `json:"usItemId"`
	Quantity int    `json:"quantity"`
}

// AffilAddToCartURL builds the affiliate add-to-cart URL for one or more
// items. Multiple items are joined with commas as id|qty pairs, matching the
// affiliate page's own parser (split on "," then "|" or "_").
//
// Example: https://affil.walmart.com/cart/addToCart?items=51259338|1,44391152|2
func AffilAddToCartURL(items []AffilCartItem) (string, error) {
	if len(items) == 0 {
		return "", fmt.Errorf("walmart: at least one item is required")
	}
	if len(items) > MaxAffilItems {
		return "", fmt.Errorf("walmart: at most %d items per add-to-cart link (got %d)", MaxAffilItems, len(items))
	}
	parts := make([]string, 0, len(items))
	seen := map[string]int{}
	for _, it := range items {
		id, err := NormalizeItemID(it.USItemID)
		if err != nil {
			return "", err
		}
		if it.Quantity < 1 || it.Quantity > MaxCartLineQuantity {
			return "", fmt.Errorf("quantity for item %s must be between 1 and %d, got %d", id, MaxCartLineQuantity, it.Quantity)
		}
		if prev, ok := seen[id]; ok {
			return "", fmt.Errorf("item %s appears more than once (qty %d and %d); merge them first", id, prev, it.Quantity)
		}
		seen[id] = it.Quantity
		parts = append(parts, id+"|"+strconv.Itoa(it.Quantity))
	}
	u, err := url.Parse(AffilAddToCartHost + AffilAddToCartPath)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("items", strings.Join(parts, ","))
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// AffilParseItems parses an items= query value (for tests and dry-run checks).
func AffilParseItems(raw string) ([]AffilCartItem, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("walmart: empty items")
	}
	var out []AffilCartItem
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		sep := "|"
		if strings.Contains(part, "_") && !strings.Contains(part, "|") {
			sep = "_"
		}
		bits := strings.SplitN(part, sep, 2)
		id, err := NormalizeItemID(bits[0])
		if err != nil {
			return nil, err
		}
		qty := 1
		if len(bits) == 2 && bits[1] != "" {
			q, err := strconv.Atoi(bits[1])
			if err != nil || q < 1 {
				return nil, fmt.Errorf("walmart: bad quantity in %q", part)
			}
			qty = q
		}
		out = append(out, AffilCartItem{USItemID: id, Quantity: qty})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("walmart: empty items")
	}
	return out, nil
}
