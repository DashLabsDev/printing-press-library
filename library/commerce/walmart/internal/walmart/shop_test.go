// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.

package walmart

import (
	"encoding/json"
	"strings"
	"testing"
)

// All fixtures below are synthetic placeholders.

func TestItemVariablesFillsPlaceholders(t *testing.T) {
	v, err := ItemVariables("https://www.walmart.com/ip/Example-Item/10000000001")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(v, "{{") {
		t.Fatalf("unfilled placeholder: %s", v)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(v), &m); err != nil {
		t.Fatal(err)
	}
	if m["iId"] != "10000000001" {
		t.Fatalf("iId = %v", m["iId"])
	}
	if !strings.Contains(v, "https://www.walmart.com/ip/10000000001") {
		t.Fatal("pageUrl not filled")
	}
	if _, err := ItemVariables("abc"); err == nil {
		t.Fatal("expected error for non-numeric id")
	}
}

func TestStoreAndCartVariables(t *testing.T) {
	v, err := StoreVariables("00000")
	if err != nil || !strings.Contains(v, `"postalCode":"00000"`) {
		t.Fatalf("store vars %v %s", err, v)
	}
	if _, err := StoreVariables("1234"); err == nil {
		t.Fatal("expected ZIP validation error")
	}
	id := "00000000-0000-0000-0000-000000000000"
	v, err = CartVariables(id)
	if err != nil || !strings.Contains(v, `"cartId":"`+id+`"`) {
		t.Fatalf("cart vars %v", err)
	}
	if _, err := CartVariables(""); err == nil {
		t.Fatal("expected cart id error")
	}
}

const productFixture = `{"data":{"product":{
 "usItemId":"10000000001","name":"Example Dried Fruit 200g","brand":"Example Brand",
 "availabilityStatus":"IN_STOCK","sellerName":"Example Seller Ltd","sellerDisplayName":"Example Shop",
 "offerType":"ONLINE_ONLY","canonicalUrl":"/ip/Example-Dried-Fruit/10000000001",
 "shortDescription":"<p>Tasty <b>example</b></p>","averageRating":4.5,"numberOfReviews":12,"orderLimit":12,"snapEligible":false,
 "priceInfo":{"currentPrice":{"price":3.5,"priceString":"$3.50"},"unitPrice":{"priceString":"1.8 ¢/g"},"wasPrice":null},
 "fulfillmentOptions":[{"type":"SHIPPING","availabilityStatus":"IN_STOCK","maxOrderQuantity":12},{"type":"PICKUP","availabilityStatus":"NOT_AVAILABLE"}],
 "fulfillmentSummary":[{"fulfillment":"DELIVERY","storeId":"0"}],
 "productLocation":[{"displayValue":"A1"}],
 "location":{"postalCode":"99999","city":"Testville","addressId":"addr-placeholder","storeIds":["0000"]},
 "fulfillmentLabel":[{"locationText":"1 Example St","message":"arrives to 1 Example St"}],
 "category":{"path":[{"name":"Food"},{"name":"Snacks"}]}}}}`

func TestParseProductDropsShopperLocation(t *testing.T) {
	d, err := ParseProduct([]byte(productFixture))
	if err != nil {
		t.Fatal(err)
	}
	if d.Price != 3.5 || d.PriceDisplay != "$3.50" || d.UnitPrice != "1.8 ¢/g" {
		t.Fatalf("price %+v", d)
	}
	if d.PickupStatus != "NOT_AVAILABLE" || d.DeliveryStatus != "NOT_AVAILABLE" || d.ShippingStatus != "IN_STOCK" {
		t.Fatalf("fulfillment %+v", d)
	}
	if d.StoreID != "0000" || d.Aisle != "A1" || d.Seller != "Example Shop" || d.Category != "Food > Snacks" {
		t.Fatalf("fields %+v", d)
	}
	if d.Description != "Tasty  example" && !strings.Contains(d.Description, "Tasty") {
		t.Fatalf("description %q", d.Description)
	}
	b, _ := json.Marshal(d)
	for _, bad := range []string{"Testville", "addr-placeholder", "1 Example St", "99999"} {
		if strings.Contains(string(b), bad) {
			t.Fatalf("shopper location leaked: %s", bad)
		}
	}
}

func TestParseStores(t *testing.T) {
	raw := `{"data":{"nearByNodes":{"nodes":[{"id":"0000","distance":"1.2","type":"STORE","displayName":"Example Supercenter",
	 "address":{"addressLineOne":"100 Example Rd","city":"Testville","state":"ZZ","postalCode":"00000"},
	 "open24Hours":false,"displayAccessTypes":["PICKUP_CURBSIDE"],"isNodeSelectableOnline":true,
	 "operationalHours":[{"day":"Monday","start":"06:00","end":"23:00","closed":false}]}]}}}`
	s, err := ParseStores([]byte(raw))
	if err != nil || len(s) != 1 {
		t.Fatalf("%v %d", err, len(s))
	}
	if s[0].ID != "0000" || s[0].Distance != "1.2" || s[0].TodayHours != "Monday 06:00-23:00" || !s[0].Selectable {
		t.Fatalf("%+v", s[0])
	}
}

func TestParseCartDropsCustomer(t *testing.T) {
	raw := `{"data":{"cart":{"id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","checkoutable":false,
	 "customer":{"id":"cust-placeholder","firstName":"Test","phone":"0000000000"},
	 "lineItems":[{"quantity":2,"product":{"usItemId":"10000000001","name":"Example Milk"},"priceInfo":{"linePrice":{"value":7,"displayValue":"$7.00"}}}],
	 "fulfillment":{"intent":"PICKUP","storeId":0,"reservation":null,
	  "accessPoint":{"displayName":"Example Supercenter","fulfillmentType":"INSTORE_PICKUP"},
	  "homepageBookslotDetails":{"title":"Reserve a time"},
	  "deliveryAddress":{"addressLineOne":"1 Example St","firstName":"Test","phone":"0000000000"}},
	 "priceDetails":{"subTotal":{"value":7},"grandTotal":{"value":7}}}}}`
	v, err := ParseCart([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if v.ItemCount != 2 || len(v.Items) != 1 || v.Items[0].PriceDisplay != "$7.00" || v.Intent != "PICKUP" || v.SlotReserved || v.Total != 7 {
		t.Fatalf("%+v", v)
	}
	b, _ := json.Marshal(v)
	// The cart id itself is reported (it is needed to address the cart);
	// customer identity and addresses never are.
	for _, bad := range []string{"cust-placeholder", "0000000000", "1 Example St", `"Test"`} {
		if strings.Contains(string(b), bad) {
			t.Fatalf("customer data leaked: %s", bad)
		}
	}
}

func TestSearchFacetsAndPaging(t *testing.T) {
	nd := `{"props":{"pageProps":{"initialData":{"searchResult":{"aggregatedCount":50,"hasMorePages":false,"paginationV2":{"maxPage":2},
	 "itemStacks":[{"items":[{"__typename":"Product","usItemId":"10000000001","name":"Example","price":1.5,"fulfillmentSummary":[{"fulfillment":"DELIVERY","storeId":"0"},{"fulfillment":"PICKUP","storeId":"0000"}]}]}]},
	 "contentLayout":{"modules":[{"configs":{}},{"configs":{"allSortAndFilterFacets":[
	  {"type":"sort","name":"Sort by","values":[{"id":"best_match","name":"Best Match","isSelected":true}]},
	  {"type":"price","name":"Price","values":[]},
	  {"type":"brand","name":"Brand","values":[{"id":"Example Brand","name":"Example Brand"}]}]}}]}}}}}`
	html := `<html><script id="__NEXT_DATA__" type="application/json">` + nd + `</script></html>`
	r, err := ParseSearchHTML([]byte(html), "example", 1)
	if err != nil {
		t.Fatal(err)
	}
	if !r.HasMore || r.StoreID != "0000" || len(r.Facets) != 2 || r.Facets[1].Values[0].ID != "Example Brand" || !r.Facets[0].Values[0].Selected {
		t.Fatalf("%+v", r)
	}
	if ValidateFacet("brand:Example Brand") != nil || ValidateFacet("nocolon") == nil {
		t.Fatal("facet validation")
	}
}
