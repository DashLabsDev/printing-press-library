// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.

// Package walmart holds the read-only walmart.com operations captured from
// the web app: persisted-query hashes, default variables, and response
// parsers. It performs no I/O; callers send requests through internal/client.
package walmart

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Persisted-query operations captured from the walmart.com web app
// (web builds usweb-1.316.0/1.317.0). Hashes rotate on Walmart deploys; override with
// WALMART_HASH_<OPERATION> when a call returns PersistedQueryNotFound.
const (
	OpPurchaseHistory   = "PurchaseHistoryV3"
	HashPurchaseHistory = "4db6dcf1c46a4f833d68495dec2796180deb99adfdd9182fd53f100869eebcc0"
	OpGetOrder          = "getOrder"
	HashGetOrder        = "ef2bd0d9ecb03b1e3275ecc250017f5529adc9b33fda8f71a359e1a470e543a8"
	OpItemHistory       = "ItemHistory"
	HashItemHistory     = "ba7fc5662250eb1264738ced3e0f16deadd9f69d7c9d5738f432a5d904c62a7a"
)

// OrdersPage is the web page the purchase-history calls are made from.
const OrdersPage = "https://www.walmart.com/orders"

// OrchestraPath builds /orchestra/<subgraph>/graphql/<op>/<hash>.
func OrchestraPath(subgraph, op, hash string) string {
	return fmt.Sprintf("/orchestra/%s/graphql/%s/%s", subgraph, op, hash)
}

// OpHeaders returns the Apollo envelope headers for a read-only query.
func OpHeaders(op, pageURL string) map[string]string {
	return map[string]string{
		"x-apollo-operation-name": op,
		"x-o-gql-query":           "query " + op,
		"wm_page_url":             pageURL,
		"Referer":                 pageURL,
	}
}

// HistoryFilter maps CLI --type/--status values to Walmart filter ids.
var HistoryFilter = map[string]string{
	"in-store":    "in-store",
	"online":      "online",
	"completed":   "completed",
	"in-progress": "in-progress",
}

// HistoryVariables builds PurchaseHistoryV3 variables.
func HistoryVariables(cursor, search string, limit int, filterIDs []string) (string, error) {
	if limit <= 0 {
		limit = 10
	}
	if filterIDs == nil {
		filterIDs = []string{}
	}
	v := map[string]any{
		"input": map[string]any{
			"cursor": cursor, "search": search, "filterIds": filterIDs, "limit": limit,
			"type": nil, "minTimestamp": nil, "maxTimestamp": nil,
			"filters":         map[string]any{"minTimestamp": nil, "maxTimestamp": nil, "filterIds": filterIDs},
			"enabledFeatures": []string{}, "isStrictSearch": false,
			"eligibleFeatures": map[string]any{
				"isEbtEligible": false, "enablePhFiltersEnhancement": true, "isBnbEligible": false,
				"isWcpAccEligible": false, "isLotOnOdpEnable": true, "isSolutranEligible": false,
			},
			"searchItemIdentifierId": "",
		},
		"platform": "WEB", "enableIsWcpOrder": false, "enableWcpPhaseOrder": false, "enableReturnStatusTracker": false,
	}
	b, err := json.Marshal(v)
	return string(b), err
}

// OrderVariables builds getOrder variables.
func OrderVariables(orderID, groupID string, inStore bool) (string, error) {
	if strings.TrimSpace(orderID) == "" {
		return "", errors.New("order id is required")
	}
	v := map[string]any{
		"orderId": orderID, "orderIsInStore": inStore, "clickThroughGroupId": groupID,
		"enableIsWcpOrder": false, "enableWcpPhaseOrder": false, "enableUniquePaymentIdentifier": false,
		"enabledFeatures": []string{"csc", "csat-northstar-v1"}, "enableSignOnDelivery": true,
		"enableVolumePricing": false, "enableCancelFix": false, "enableGroupBannerMessages": true,
		"eligibleFeatures": map[string]any{
			"isEbtEligible": false, "isLotOnOdpEnable": true, "isLmdReturnsBarcodeDisabled": true, "isSolutranEligible": false,
		},
	}
	b, err := json.Marshal(v)
	return string(b), err
}

// Money is a Walmart price value.
type Money struct {
	Value        float64 `json:"value"`
	DisplayValue string  `json:"displayValue"`
}

// OrderSummary is one row of purchase history.
type OrderSummary struct {
	ID         string  `json:"id"`
	OrderDate  string  `json:"orderDate"`
	Type       string  `json:"type"`
	InStore    bool    `json:"isInStore"`
	ItemCount  int     `json:"itemCount"`
	Title      string  `json:"title"`
	Total      float64 `json:"total"`
	GroupID    string  `json:"groupId,omitempty"`
	Fulfilment string  `json:"fulfillmentType,omitempty"`
}

// HistoryPage is a parsed PurchaseHistoryV3 page.
type HistoryPage struct {
	Orders     []OrderSummary `json:"orders"`
	NextCursor string         `json:"nextCursor,omitempty"`
}

type gqlErr struct {
	Message string `json:"message"`
}

func gqlErrors(raw []byte) error {
	var env struct {
		Errors []gqlErr `json:"errors"`
	}
	if json.Unmarshal(raw, &env) == nil && len(env.Errors) > 0 {
		msgs := make([]string, 0, len(env.Errors))
		for _, e := range env.Errors {
			msgs = append(msgs, e.Message)
		}
		joined := strings.Join(msgs, "; ")
		if strings.Contains(joined, "PersistedQueryNotFound") {
			return fmt.Errorf("walmart: persisted query hash is stale (%s); re-capture the operation", joined)
		}
		return fmt.Errorf("walmart: GraphQL error: %s", joined)
	}
	return nil
}

// ParseHistory parses a PurchaseHistoryV3 response.
func ParseHistory(raw []byte) (*HistoryPage, error) {
	var env struct {
		Data struct {
			PurchaseHistory *struct {
				PageInfo struct {
					NextPageCursor string `json:"nextPageCursor"`
				} `json:"pageInfo"`
				Orders []struct {
					ID           string `json:"id"`
					OrderDate    string `json:"orderDate"`
					Type         string `json:"type"`
					IsInStore    bool   `json:"isInStore"`
					ItemCount    int    `json:"itemCount"`
					Title        string `json:"title"`
					PriceDetails struct {
						OrderTotal *Money `json:"orderTotal"`
					} `json:"priceDetails"`
					Groups []struct {
						GroupID         string `json:"groupId"`
						FulfillmentType string `json:"fulfillmentType"`
					} `json:"groups"`
				} `json:"orders"`
			} `json:"purchaseHistory"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("walmart: decoding purchase history: %w", err)
	}
	if env.Data.PurchaseHistory == nil {
		if err := gqlErrors(raw); err != nil {
			return nil, err
		}
		return nil, errors.New("walmart: purchase history missing from response")
	}
	ph := env.Data.PurchaseHistory
	out := &HistoryPage{NextCursor: ph.PageInfo.NextPageCursor, Orders: []OrderSummary{}}
	for _, o := range ph.Orders {
		s := OrderSummary{ID: o.ID, OrderDate: o.OrderDate, Type: o.Type, InStore: o.IsInStore, ItemCount: o.ItemCount, Title: o.Title}
		if o.PriceDetails.OrderTotal != nil {
			s.Total = o.PriceDetails.OrderTotal.Value
		}
		if len(o.Groups) > 0 {
			s.GroupID = o.Groups[0].GroupID
			s.Fulfilment = o.Groups[0].FulfillmentType
		}
		out.Orders = append(out.Orders, s)
	}
	return out, nil
}

// LineItem is one purchased item.
type LineItem struct {
	Name      string  `json:"name"`
	USItemID  string  `json:"usItemId,omitempty"`
	Quantity  int     `json:"quantity"`
	LinePrice float64 `json:"linePrice"`
	Group     string  `json:"fulfillmentType,omitempty"`
}

// OrderDetail is a parsed getOrder response, without customer PII.
type OrderDetail struct {
	ID        string     `json:"id"`
	OrderDate string     `json:"orderDate"`
	Type      string     `json:"type"`
	ItemCount int        `json:"itemCount"`
	Items     []LineItem `json:"items"`
	SubTotal  float64    `json:"subTotal"`
	Tax       float64    `json:"tax"`
	Tip       float64    `json:"driverTip,omitempty"`
	Fees      float64    `json:"fees,omitempty"`
	Discounts float64    `json:"discounts,omitempty"`
	Total     float64    `json:"total"`
}

type priceLine struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
}

// ParseOrder parses a getOrder response. Customer name/email, addresses,
// pickup person and payment details are deliberately not decoded.
func ParseOrder(raw []byte) (*OrderDetail, error) {
	var env struct {
		Data struct {
			Order *struct {
				ID        string `json:"id"`
				OrderDate string `json:"orderDate"`
				Type      string `json:"type"`
				ItemCount int    `json:"itemCount"`
				Groups    []struct {
					FulfillmentType string `json:"fulfillmentType"`
					Items           []struct {
						Quantity    int `json:"quantity"`
						ProductInfo struct {
							Name     string `json:"name"`
							USItemID string `json:"usItemId"`
						} `json:"productInfo"`
						PriceInfo struct {
							LinePrice *Money `json:"linePrice"`
						} `json:"priceInfo"`
					} `json:"items"`
				} `json:"groups_2101"`
				PriceDetails *struct {
					SubTotal           *priceLine  `json:"subTotal"`
					TaxTotal           *priceLine  `json:"taxTotal"`
					GrandTotal         *priceLine  `json:"grandTotal"`
					GrandTotalWithTips *priceLine  `json:"grandTotalWithTips"`
					DriverTip          *priceLine  `json:"driverTip"`
					Fees               []priceLine `json:"fees"`
					Discounts          []priceLine `json:"discounts"`
				} `json:"priceDetails"`
			} `json:"order"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("walmart: decoding order: %w", err)
	}
	o := env.Data.Order
	if o == nil {
		if err := gqlErrors(raw); err != nil {
			return nil, err
		}
		return nil, errors.New("walmart: order missing from response")
	}
	d := &OrderDetail{ID: o.ID, OrderDate: o.OrderDate, Type: o.Type, ItemCount: o.ItemCount, Items: []LineItem{}}
	for _, g := range o.Groups {
		for _, it := range g.Items {
			li := LineItem{Name: it.ProductInfo.Name, USItemID: it.ProductInfo.USItemID, Quantity: it.Quantity, Group: g.FulfillmentType}
			if it.PriceInfo.LinePrice != nil {
				li.LinePrice = it.PriceInfo.LinePrice.Value
			}
			d.Items = append(d.Items, li)
		}
	}
	if p := o.PriceDetails; p != nil {
		if p.SubTotal != nil {
			d.SubTotal = p.SubTotal.Value
		}
		if p.TaxTotal != nil {
			d.Tax = p.TaxTotal.Value
		}
		if p.DriverTip != nil {
			d.Tip = p.DriverTip.Value
		}
		for _, f := range p.Fees {
			d.Fees += f.Value
		}
		for _, x := range p.Discounts {
			d.Discounts += x.Value
		}
		switch {
		case p.GrandTotalWithTips != nil:
			d.Total = p.GrandTotalWithTips.Value
		case p.GrandTotal != nil:
			d.Total = p.GrandTotal.Value
		}
	}
	return d, nil
}

// Product is one search result.
type Product struct {
	USItemID     string  `json:"usItemId"`
	Name         string  `json:"name"`
	Price        float64 `json:"price"`
	PriceDisplay string  `json:"priceDisplay,omitempty"`
	UnitPrice    string  `json:"unitPrice,omitempty"`
	Availability string  `json:"availability,omitempty"`
	Fulfillment  string  `json:"fulfillment,omitempty"`
	Seller       string  `json:"seller,omitempty"`
	Aisle        string  `json:"aisle,omitempty"`
	Rating       float64 `json:"rating,omitempty"`
	URL          string  `json:"url,omitempty"`
}

// SearchPage is a parsed /search result page.
type SearchPage struct {
	Query    string    `json:"query"`
	Page     int       `json:"page"`
	Total    int       `json:"total"`
	HasMore  bool      `json:"hasMore"`
	StoreID  string    `json:"storeId,omitempty"`
	Filters  []string  `json:"filters,omitempty"`
	Sort     string    `json:"sort,omitempty"`
	Products []Product `json:"products"`
	Facets   []Facet   `json:"facets,omitempty"`
}

// Facet is one search filter group; pass "<type>:<value id>" to --filter.
type Facet struct {
	Type   string       `json:"type"`
	Name   string       `json:"name"`
	Values []FacetValue `json:"values"`
}

// FacetValue is one selectable filter value.
type FacetValue struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Selected bool   `json:"selected,omitempty"`
}

var nextDataRe = regexp.MustCompile(`(?s)<script[^>]*id="__NEXT_DATA__"[^>]*>(.*?)</script>`)

// ParseSearchHTML extracts products from the server-rendered search page.
func ParseSearchHTML(html []byte, query string, page int) (*SearchPage, error) {
	m := nextDataRe.FindSubmatch(html)
	if m == nil {
		return nil, errors.New("walmart: search page had no __NEXT_DATA__ (blocked or layout changed)")
	}
	var nd struct {
		Props struct {
			PageProps struct {
				InitialData struct {
					SearchResult struct {
						AggregatedCount int  `json:"aggregatedCount"`
						HasMorePages    bool `json:"hasMorePages"`
						PaginationV2    struct {
							MaxPage int `json:"maxPage"`
						} `json:"paginationV2"`
						ItemStacks []struct {
							Items []json.RawMessage `json:"items"`
						} `json:"itemStacks"`
					} `json:"searchResult"`
					ContentLayout struct {
						Modules []struct {
							Configs struct {
								Facets []struct {
									Type   string `json:"type"`
									Name   string `json:"name"`
									Values []struct {
										ID         string `json:"id"`
										Name       string `json:"name"`
										IsSelected bool   `json:"isSelected"`
									} `json:"values"`
								} `json:"allSortAndFilterFacets"`
							} `json:"configs"`
						} `json:"modules"`
					} `json:"contentLayout"`
				} `json:"initialData"`
			} `json:"pageProps"`
		} `json:"props"`
	}
	if err := json.Unmarshal(m[1], &nd); err != nil {
		return nil, fmt.Errorf("walmart: decoding search data: %w", err)
	}
	sr := nd.Props.PageProps.InitialData.SearchResult
	out := &SearchPage{Query: query, Page: page, Total: sr.AggregatedCount, HasMore: sr.HasMorePages || sr.PaginationV2.MaxPage > page, Products: []Product{}}
	for _, m := range nd.Props.PageProps.InitialData.ContentLayout.Modules {
		if len(m.Configs.Facets) == 0 || len(out.Facets) > 0 {
			continue
		}
		for _, f := range m.Configs.Facets {
			if len(f.Values) == 0 {
				continue
			}
			fc := Facet{Type: f.Type, Name: f.Name, Values: []FacetValue{}}
			for _, v := range f.Values {
				fc.Values = append(fc.Values, FacetValue{ID: v.ID, Name: v.Name, Selected: v.IsSelected})
			}
			out.Facets = append(out.Facets, fc)
		}
	}
	for _, st := range sr.ItemStacks {
		for _, rawItem := range st.Items {
			var it struct {
				Typename      string  `json:"__typename"`
				USItemID      string  `json:"usItemId"`
				Name          string  `json:"name"`
				SellerName    string  `json:"sellerName"`
				AverageRating float64 `json:"averageRating"`
				CanonicalURL  string  `json:"canonicalUrl"`
				Availability  struct {
					Display string `json:"display"`
				} `json:"availabilityStatusV2"`
				PriceInfo struct {
					LinePrice        string `json:"linePrice"`
					LinePriceDisplay string `json:"linePriceDisplay"`
					UnitPrice        string `json:"unitPrice"`
					ItemPrice        string `json:"itemPrice"`
				} `json:"priceInfo"`
				Price          float64 `json:"price"`
				FulfillmentSum []struct {
					Fulfillment string `json:"fulfillment"`
					StoreID     string `json:"storeId"`
				} `json:"fulfillmentSummary"`
				Aisle            string `json:"productLocationDisplayValue"`
				FulfillmentBadge string `json:"fulfillmentBadge"`
			}
			if json.Unmarshal(rawItem, &it) != nil || it.Typename != "Product" || it.USItemID == "" {
				continue
			}
			p := Product{USItemID: it.USItemID, Name: it.Name, Price: it.Price, PriceDisplay: it.PriceInfo.LinePrice,
				UnitPrice: it.PriceInfo.UnitPrice, Aisle: it.Aisle, Availability: it.Availability.Display, Seller: it.SellerName, Rating: it.AverageRating}
			if p.PriceDisplay == "" {
				p.PriceDisplay = it.PriceInfo.LinePriceDisplay
			}
			if p.PriceDisplay == "" {
				p.PriceDisplay = it.PriceInfo.ItemPrice
			}
			if p.PriceDisplay == "" && p.Price > 0 {
				p.PriceDisplay = fmt.Sprintf("$%.2f", p.Price)
			}
			if it.CanonicalURL != "" {
				p.URL = "https://www.walmart.com" + it.CanonicalURL
			}
			var fs []string
			for _, f := range it.FulfillmentSum {
				if f.Fulfillment != "" {
					fs = append(fs, strings.ToLower(f.Fulfillment))
				}
				if out.StoreID == "" && f.StoreID != "" && f.StoreID != "0" {
					out.StoreID = f.StoreID
				}
			}
			if len(fs) == 0 && it.FulfillmentBadge != "" {
				fs = append(fs, it.FulfillmentBadge)
			}
			p.Fulfillment = strings.Join(fs, ",")
			out.Products = append(out.Products, p)
		}
	}
	return out, nil
}
