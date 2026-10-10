// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.

package walmart

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Read-only shopping operations captured from the walmart.com web app
// (product page, store chooser list, cart page). All are GET persisted
// queries. Hashes rotate on Walmart deploys; override with
// WALMART_HASH_<OPERATION>.
const (
	OpItemByID         = "ItemById"
	HashItemByID       = "63a5914ae94e02b4076e3b54e0eed12e7b168c95b6ff0c355e5eef7a090a0d63"
	SubgraphItemByID   = "pdp"
	OpNearByNodes      = "nearByNodes"
	HashNearByNodes    = "d26e41479a06dc27775a042b88b74ea8b5b75d3a670bcafd080eb7a4e2bdf66f"
	SubgraphNearByNode = "home"
	OpGetCart          = "getCart"
	HashGetCart        = "f9e23365bcbea20fa5894ef17dd63c0cc3da2e3e8f96798320cc7d6752f95941"
	SubgraphGetCart    = "home"
	CartPage           = "https://www.walmart.com/cart"
)

// Variable templates are the captured request variables with every
// session-specific value replaced by a {{PLACEHOLDER}}.
//
//go:embed templates/*.json
var templates embed.FS

var (
	itemIDRe   = regexp.MustCompile(`^[0-9]{4,20}$`)
	postalRe   = regexp.MustCompile(`^[0-9]{5}$`)
	cartIDRe   = regexp.MustCompile(`^[0-9a-fA-F-]{36}$`)
	ipURLIDRe  = regexp.MustCompile(`/ip/(?:[^/]+/)?([0-9]{4,20})`)
	facetPartR = regexp.MustCompile(`^[A-Za-z0-9_]+:.+$`)
)

func fillTemplate(name string, repl map[string]string) (string, error) {
	b, err := templates.ReadFile("templates/" + name + ".json")
	if err != nil {
		return "", err
	}
	s := string(b)
	for k, v := range repl {
		enc, _ := json.Marshal(v)
		s = strings.ReplaceAll(s, `"{{`+k+`}}"`, string(enc))
		inner := strings.Trim(string(enc), `"`)
		s = strings.ReplaceAll(s, `{{`+k+`}}`, inner)
	}
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return "", fmt.Errorf("walmart: template %s: %w", name, err)
	}
	out, err := json.Marshal(v)
	return string(out), err
}

// NormalizeItemID accepts a numeric item id or a walmart.com /ip/ URL.
func NormalizeItemID(s string) (string, error) {
	s = strings.TrimSpace(s)
	if m := ipURLIDRe.FindStringSubmatch(s); m != nil {
		return m[1], nil
	}
	if !itemIDRe.MatchString(s) {
		return "", fmt.Errorf("item id must be numeric (the number at the end of a walmart.com/ip/ URL), got %q", s)
	}
	return s, nil
}

// ItemVariables builds ItemById variables for one item.
func ItemVariables(itemID string) (string, error) {
	id, err := NormalizeItemID(itemID)
	if err != nil {
		return "", err
	}
	return fillTemplate(OpItemByID, map[string]string{"ITEM_ID": id})
}

// ItemPathSuffix is appended after the ItemById hash: the web app requests
// /orchestra/pdp/graphql/ItemById/<hash>/ip/<itemId>.
func ItemPathSuffix(itemID string) string { return "/ip/" + itemID }

// ItemPage is the product page URL used as wm_page_url/Referer.
func ItemPage(itemID string) string { return "https://www.walmart.com/ip/" + itemID }

// StoreVariables builds nearByNodes variables for a 5-digit ZIP.
func StoreVariables(zip string) (string, error) {
	zip = strings.TrimSpace(zip)
	if !postalRe.MatchString(zip) {
		return "", errors.New("a 5-digit ZIP code is required (--zip or WALMART_ZIP)")
	}
	return fillTemplate(OpNearByNodes, map[string]string{"POSTAL_CODE": zip})
}

// CartVariables builds getCart variables for a cart id.
func CartVariables(cartID string) (string, error) {
	cartID = strings.TrimSpace(cartID)
	if !cartIDRe.MatchString(cartID) {
		return "", errors.New("a cart id (36-character UUID) is required (--cart-id or WALMART_CART_ID)")
	}
	return fillTemplate(OpGetCart, map[string]string{"CART_ID": cartID})
}

type moneyV2 struct {
	Price       float64 `json:"price"`
	PriceString string  `json:"priceString"`
}

// FulfillmentOption is one way to get a product.
type FulfillmentOption struct {
	Type         string `json:"type"`
	Availability string `json:"availability,omitempty"`
	MaxQuantity  int    `json:"maxQuantity,omitempty"`
}

// ProductDetail is a parsed ItemById response. Shopper location fields
// (address, ZIP, city, address id, "deliver to" text) are never decoded.
type ProductDetail struct {
	USItemID       string              `json:"usItemId"`
	Name           string              `json:"name"`
	Brand          string              `json:"brand,omitempty"`
	Price          float64             `json:"price"`
	PriceDisplay   string              `json:"priceDisplay,omitempty"`
	UnitPrice      string              `json:"unitPrice,omitempty"`
	WasPrice       string              `json:"wasPrice,omitempty"`
	Availability   string              `json:"availability,omitempty"`
	Seller         string              `json:"seller,omitempty"`
	SoldByWalmart  bool                `json:"soldByWalmart"`
	OfferType      string              `json:"offerType,omitempty"`
	OfferID        string              `json:"offerId,omitempty"`
	Fulfillment    []FulfillmentOption `json:"fulfillment"`
	PickupStatus   string              `json:"pickup,omitempty"`
	DeliveryStatus string              `json:"delivery,omitempty"`
	ShippingStatus string              `json:"shipping,omitempty"`
	Aisle          string              `json:"aisle,omitempty"`
	StoreID        string              `json:"storeId,omitempty"`
	OrderLimit     int                 `json:"orderLimit,omitempty"`
	SNAPEligible   bool                `json:"snapEligible"`
	Rating         float64             `json:"rating,omitempty"`
	Reviews        int                 `json:"reviews,omitempty"`
	Category       string              `json:"category,omitempty"`
	Description    string              `json:"description,omitempty"`
	URL            string              `json:"url,omitempty"`
}

var tagRe = regexp.MustCompile(`<[^>]+>`)

// ParseProduct parses an ItemById response.
func ParseProduct(raw []byte) (*ProductDetail, error) {
	var env struct {
		Data struct {
			Product *struct {
				USItemID           string   `json:"usItemId"`
				Name               string   `json:"name"`
				Brand              string   `json:"brand"`
				AvailabilityStatus string   `json:"availabilityStatus"`
				SellerName         string   `json:"sellerName"`
				SellerDisplayName  string   `json:"sellerDisplayName"`
				SellerType         string   `json:"sellerType"`
				OfferType          string   `json:"offerType"`
				OfferID            string   `json:"offerId"`
				CanonicalURL       string   `json:"canonicalUrl"`
				ShortDescription   string   `json:"shortDescription"`
				AverageRating      *float64 `json:"averageRating"`
				NumberOfReviews    *int     `json:"numberOfReviews"`
				OrderLimit         int      `json:"orderLimit"`
				SnapEligible       bool     `json:"snapEligible"`
				PriceInfo          struct {
					CurrentPrice *moneyV2        `json:"currentPrice"`
					UnitPrice    json.RawMessage `json:"unitPrice"`
					WasPrice     json.RawMessage `json:"wasPrice"`
				} `json:"priceInfo"`
				FulfillmentOptions []struct {
					Type               string `json:"type"`
					AvailabilityStatus string `json:"availabilityStatus"`
					MaxOrderQuantity   int    `json:"maxOrderQuantity"`
				} `json:"fulfillmentOptions"`
				PickupOption *struct {
					AvailabilityStatus string `json:"availabilityStatus"`
					StoreID            any    `json:"storeId"`
				} `json:"pickupOption"`
				ShippingOption *struct {
					AvailabilityStatus string `json:"availabilityStatus"`
				} `json:"shippingOption"`
				FulfillmentSummary []struct {
					Fulfillment string `json:"fulfillment"`
					StoreID     string `json:"storeId"`
				} `json:"fulfillmentSummary"`
				ProductLocation json.RawMessage `json:"productLocation"`
				Location        *struct {
					StoreIDs []string `json:"storeIds"`
				} `json:"location"`
				Category *struct {
					Path []struct {
						Name string `json:"name"`
					} `json:"path"`
				} `json:"category"`
			} `json:"product"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("walmart: decoding product: %w", err)
	}
	p := env.Data.Product
	if p == nil {
		if err := gqlErrors(raw); err != nil {
			return nil, err
		}
		return nil, errors.New("walmart: product missing from response (unknown item id?)")
	}
	d := &ProductDetail{USItemID: p.USItemID, Name: p.Name, Brand: p.Brand, Availability: p.AvailabilityStatus,
		Seller: p.SellerDisplayName, OfferType: p.OfferType, OfferID: p.OfferID, OrderLimit: p.OrderLimit, SNAPEligible: p.SnapEligible,
		Fulfillment: []FulfillmentOption{}}
	if d.Seller == "" {
		d.Seller = p.SellerName
	}
	d.SoldByWalmart = strings.EqualFold(p.SellerName, "Walmart.com") || strings.EqualFold(p.SellerType, "INTERNAL")
	if cp := p.PriceInfo.CurrentPrice; cp != nil {
		d.Price, d.PriceDisplay = cp.Price, cp.PriceString
	}
	d.UnitPrice = rawPriceString(p.PriceInfo.UnitPrice)
	d.WasPrice = rawPriceString(p.PriceInfo.WasPrice)
	if p.AverageRating != nil {
		d.Rating = *p.AverageRating
	}
	if p.NumberOfReviews != nil {
		d.Reviews = *p.NumberOfReviews
	}
	for _, f := range p.FulfillmentOptions {
		d.Fulfillment = append(d.Fulfillment, FulfillmentOption{Type: f.Type, Availability: f.AvailabilityStatus, MaxQuantity: f.MaxOrderQuantity})
		switch strings.ToUpper(f.Type) {
		case "PICKUP":
			d.PickupStatus = f.AvailabilityStatus
		case "DELIVERY":
			d.DeliveryStatus = f.AvailabilityStatus
		case "SHIPPING":
			d.ShippingStatus = f.AvailabilityStatus
		}
	}
	if d.PickupStatus == "" {
		d.PickupStatus = "NOT_AVAILABLE"
		if p.PickupOption != nil && p.PickupOption.AvailabilityStatus != "" {
			d.PickupStatus = p.PickupOption.AvailabilityStatus
		}
	}
	if d.DeliveryStatus == "" {
		d.DeliveryStatus = "NOT_AVAILABLE"
	}
	if d.ShippingStatus == "" && p.ShippingOption != nil {
		d.ShippingStatus = p.ShippingOption.AvailabilityStatus
	}
	d.Aisle = rawAisle(p.ProductLocation)
	if p.Location != nil && len(p.Location.StoreIDs) > 0 {
		d.StoreID = p.Location.StoreIDs[0]
	}
	for _, f := range p.FulfillmentSummary {
		if d.StoreID == "" && f.StoreID != "" && f.StoreID != "0" {
			d.StoreID = f.StoreID
		}
	}
	if p.Category != nil {
		names := make([]string, 0, len(p.Category.Path))
		for _, c := range p.Category.Path {
			names = append(names, c.Name)
		}
		d.Category = strings.Join(names, " > ")
	}
	d.Description = strings.TrimSpace(tagRe.ReplaceAllString(p.ShortDescription, " "))
	if p.CanonicalURL != "" {
		d.URL = "https://www.walmart.com" + p.CanonicalURL
	}
	return d, nil
}

// Store is one nearby pickup location. Store addresses are public business
// locations.
type Store struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Type        string   `json:"type,omitempty"`
	Distance    string   `json:"distance,omitempty"`
	Address     string   `json:"address,omitempty"`
	City        string   `json:"city,omitempty"`
	State       string   `json:"state,omitempty"`
	PostalCode  string   `json:"postalCode,omitempty"`
	AccessTypes []string `json:"accessTypes,omitempty"`
	Open24Hours bool     `json:"open24Hours"`
	Selectable  bool     `json:"selectableOnline"`
	TodayHours  string   `json:"hours,omitempty"`
}

// ParseStores parses a nearByNodes response.
func ParseStores(raw []byte) ([]Store, error) {
	var env struct {
		Data struct {
			NearByNodes *struct {
				Nodes []struct {
					ID          string `json:"id"`
					Distance    any    `json:"distance"`
					Type        string `json:"type"`
					DisplayName string `json:"displayName"`
					Name        string `json:"name"`
					Address     struct {
						AddressLineOne string `json:"addressLineOne"`
						City           string `json:"city"`
						State          string `json:"state"`
						PostalCode     string `json:"postalCode"`
					} `json:"address"`
					Open24Hours        bool     `json:"open24Hours"`
					DisplayAccessTypes []string `json:"displayAccessTypes"`
					Selectable         bool     `json:"isNodeSelectableOnline"`
					OperationalHours   []struct {
						Day    string `json:"day"`
						Start  string `json:"start"`
						End    string `json:"end"`
						Closed bool   `json:"closed"`
					} `json:"operationalHours"`
				} `json:"nodes"`
			} `json:"nearByNodes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("walmart: decoding stores: %w", err)
	}
	if env.Data.NearByNodes == nil {
		if err := gqlErrors(raw); err != nil {
			return nil, err
		}
		return nil, errors.New("walmart: store list missing from response")
	}
	out := []Store{}
	for _, n := range env.Data.NearByNodes.Nodes {
		s := Store{ID: n.ID, Name: n.DisplayName, Type: n.Type, Address: n.Address.AddressLineOne, City: n.Address.City,
			State: n.Address.State, PostalCode: n.Address.PostalCode, AccessTypes: n.DisplayAccessTypes,
			Open24Hours: n.Open24Hours, Selectable: n.Selectable}
		if s.Name == "" {
			s.Name = n.Name
		}
		if n.Distance != nil {
			s.Distance = strings.TrimSpace(fmt.Sprint(n.Distance))
		}
		if len(n.OperationalHours) > 0 {
			h := n.OperationalHours[0]
			if h.Closed {
				s.TodayHours = h.Day + " closed"
			} else {
				s.TodayHours = fmt.Sprintf("%s %s-%s", h.Day, h.Start, h.End)
			}
		}
		out = append(out, s)
	}
	return out, nil
}

// CartLine is one cart line item.
type CartLine struct {
	USItemID     string  `json:"usItemId,omitempty"`
	OfferID      string  `json:"offerId,omitempty"`
	Name         string  `json:"name"`
	Quantity     float64 `json:"quantity"`
	LinePrice    float64 `json:"linePrice,omitempty"`
	PriceDisplay string  `json:"priceDisplay,omitempty"`
	UnitPrice    string  `json:"unitPrice,omitempty"`
}

// CartReservation is the reserved pickup/delivery slot, if any.
type CartReservation struct {
	Start     string `json:"start,omitempty"`
	End       string `json:"end,omitempty"`
	Window    string `json:"window,omitempty"`
	HoldUntil string `json:"holdUntil,omitempty"`
	Expired   bool   `json:"expired"`
}

// CartView is a parsed getCart cart (also accepts updateItems/reserveSlot envelopes if seen). Customer
// identity, phone, delivery address, pickup store address and payment data
// are never decoded.
type CartView struct {
	CartID       string           `json:"cartId,omitempty"`
	ItemCount    int              `json:"itemCount"`
	Items        []CartLine       `json:"items"`
	Intent       string           `json:"mode,omitempty"`
	StoreID      string           `json:"storeId,omitempty"`
	PickupPoint  string           `json:"pickupPoint,omitempty"`
	SlotReserved bool             `json:"slotReserved"`
	Reservation  *CartReservation `json:"reservation,omitempty"`
	SlotStatus   string           `json:"slotStatus,omitempty"`
	SubTotal     float64          `json:"subTotal,omitempty"`
	Total        float64          `json:"total,omitempty"`
	Checkoutable bool             `json:"checkoutable"`
}

type cartMoney struct {
	Value        float64 `json:"value"`
	DisplayValue string  `json:"displayValue"`
}

type cartPayload struct {
	ID           string `json:"id"`
	Checkoutable *bool  `json:"checkoutable"`
	LineItems    []struct {
		Quantity float64 `json:"quantity"`
		Product  struct {
			USItemID string `json:"usItemId"`
			OfferID  string `json:"offerId"`
			Name     string `json:"name"`
		} `json:"product"`
		PriceInfo struct {
			LinePrice *cartMoney `json:"linePrice"`
			UnitPrice *cartMoney `json:"unitPrice"`
		} `json:"priceInfo"`
	} `json:"lineItems"`
	Fulfillment *struct {
		Intent      string `json:"intent"`
		StoreID     any    `json:"storeId"`
		Reservation *struct {
			ExpiryTime   string `json:"expiryTime"`
			Expired      bool   `json:"expired"`
			ReservedSlot *struct {
				StartTime string `json:"startTime"`
				EndTime   string `json:"endTime"`
			} `json:"reservedSlot"`
		} `json:"reservation"`
		AccessPoint *struct {
			DisplayName string `json:"displayName"`
		} `json:"accessPoint"`
		Bookslot *struct {
			Title string `json:"title"`
		} `json:"homepageBookslotDetails"`
	} `json:"fulfillment"`
	PriceDetails *struct {
		SubTotal   *cartMoney `json:"subTotal"`
		GrandTotal *cartMoney `json:"grandTotal"`
	} `json:"priceDetails"`
}

// ParseCart parses the cart object from getCart (data.cart) or from the
// cart payload under data.cart (also data.updateItems / data.reserveSlot if present).
func ParseCart(raw []byte) (*CartView, error) {
	var env struct {
		Data map[string]*cartPayload `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("walmart: decoding cart: %w", err)
	}
	var c *cartPayload
	for _, k := range []string{"cart", "updateItems", "reserveSlot"} {
		if env.Data[k] != nil {
			c = env.Data[k]
			break
		}
	}
	if c == nil {
		if err := gqlErrors(raw); err != nil {
			return nil, err
		}
		return nil, errors.New("walmart: cart missing from response")
	}
	v := &CartView{CartID: c.ID, Items: []CartLine{}}
	if c.Checkoutable != nil {
		v.Checkoutable = *c.Checkoutable
	}
	for _, li := range c.LineItems {
		l := CartLine{USItemID: li.Product.USItemID, OfferID: li.Product.OfferID, Name: li.Product.Name, Quantity: li.Quantity}
		if lp := li.PriceInfo.LinePrice; lp != nil {
			l.LinePrice, l.PriceDisplay = lp.Value, lp.DisplayValue
		}
		if up := li.PriceInfo.UnitPrice; up != nil {
			l.UnitPrice = up.DisplayValue
		}
		v.Items = append(v.Items, l)
		v.ItemCount += int(li.Quantity)
	}
	if f := c.Fulfillment; f != nil {
		v.Intent = f.Intent
		if f.StoreID != nil {
			v.StoreID = strings.TrimSuffix(fmt.Sprint(f.StoreID), ".0")
		}
		if f.AccessPoint != nil {
			v.PickupPoint = f.AccessPoint.DisplayName
		}
		if r := f.Reservation; r != nil {
			v.SlotReserved = !r.Expired
			v.Reservation = &CartReservation{HoldUntil: r.ExpiryTime, Expired: r.Expired}
			if rs := r.ReservedSlot; rs != nil {
				v.Reservation.Start, v.Reservation.End = rs.StartTime, rs.EndTime
				v.Reservation.Window = TimeWindow(rs.StartTime, rs.EndTime)
			}
		}
		if f.Bookslot != nil {
			v.SlotStatus = f.Bookslot.Title
		}
	}
	if pd := c.PriceDetails; pd != nil {
		if pd.SubTotal != nil {
			v.SubTotal = pd.SubTotal.Value
		}
		if pd.GrandTotal != nil {
			v.Total = pd.GrandTotal.Value
		}
	}
	return v, nil
}

// FindLine returns the cart line for an item id, or nil.
func (v *CartView) FindLine(usItemID string) *CartLine {
	for i := range v.Items {
		if v.Items[i].USItemID == usItemID {
			return &v.Items[i]
		}
	}
	return nil
}

// ValidateFacet checks a search filter of the form "type:value"
// (for example "brand:Great Value").
func ValidateFacet(f string) error {
	if !facetPartR.MatchString(strings.TrimSpace(f)) {
		return fmt.Errorf("filter must look like type:value (for example brand:Great Value), got %q", f)
	}
	return nil
}

// rawPriceString reads a price that may be a string or a {priceString} object.
func rawPriceString(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var m moneyV2
	if json.Unmarshal(raw, &m) == nil {
		return m.PriceString
	}
	return ""
}

// rawAisle reads productLocation as a list or a single {displayValue}.
func rawAisle(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	type loc struct {
		DisplayValue string `json:"displayValue"`
	}
	var list []loc
	if json.Unmarshal(raw, &list) == nil {
		for _, l := range list {
			if l.DisplayValue != "" {
				return l.DisplayValue
			}
		}
		return ""
	}
	var one loc
	if json.Unmarshal(raw, &one) == nil {
		return one.DisplayValue
	}
	return ""
}
