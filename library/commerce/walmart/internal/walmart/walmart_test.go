// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.

package walmart

import (
	"encoding/json"
	"strings"
	"testing"
)

// All fixtures are synthetic. Never paste real order data here.

const fakeHistory = `{"data":{"purchaseHistory":{"pageInfo":{"nextPageCursor":"cursor-fake-2"},
"orders":[{"id":"2000000000000001","orderDate":"2026-01-02","type":"GLASS","isInStore":false,"itemCount":3,"title":"Pickup order",
"priceDetails":{"orderTotal":{"value":42.5,"displayValue":"$42.50"}},"groups":[{"groupId":"9000000000000001","fulfillmentType":"PICKUP"}]},
{"id":"3000000000000002","orderDate":"2026-01-01","type":"IN_STORE","isInStore":true,"itemCount":1,"title":"Store purchase",
"priceDetails":{"orderTotal":{"value":5,"displayValue":"$5.00"}},"groups":[]}]}}}`

func TestParseHistory(t *testing.T) {
	p, err := ParseHistory([]byte(fakeHistory))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Orders) != 2 || p.NextCursor != "cursor-fake-2" {
		t.Fatalf("unexpected page: %+v", p)
	}
	if p.Orders[0].Total != 42.5 || p.Orders[0].GroupID != "9000000000000001" || p.Orders[0].Fulfilment != "PICKUP" {
		t.Fatalf("bad first order: %+v", p.Orders[0])
	}
	if !p.Orders[1].InStore || p.Orders[1].GroupID != "" {
		t.Fatalf("bad second order: %+v", p.Orders[1])
	}
}

func TestParseHistoryStaleHash(t *testing.T) {
	_, err := ParseHistory([]byte(`{"errors":[{"message":"PersistedQueryNotFound"}]}`))
	if err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("want stale-hash error, got %v", err)
	}
}

const fakeOrder = `{"data":{"order":{"id":"2000000000000001","orderDate":"2026-01-02","type":"GLASS","itemCount":2,
"customer":{"firstName":"Test","lastName":"User","email":"test@example.com"},
"groups_2101":[{"fulfillmentType":"PICKUP","items":[
{"quantity":2,"productInfo":{"name":"Test Milk 1 gal","usItemId":"100000001"},"priceInfo":{"linePrice":{"value":7.5,"displayValue":"$7.50"}}},
{"quantity":1,"productInfo":{"name":"Test Bread","usItemId":"100000002"},"priceInfo":{"linePrice":{"value":2.25,"displayValue":"$2.25"}}}]}],
"priceDetails":{"subTotal":{"value":9.75},"taxTotal":{"value":0.5},"grandTotal":{"value":10.25},"fees":[{"label":"Fee","value":1}],"discounts":[]}}}}`

func TestParseOrderDropsCustomerPII(t *testing.T) {
	d, err := ParseOrder([]byte(fakeOrder))
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Items) != 2 || d.Items[0].Quantity != 2 || d.Items[0].LinePrice != 7.5 {
		t.Fatalf("bad items: %+v", d.Items)
	}
	if d.SubTotal != 9.75 || d.Tax != 0.5 || d.Total != 10.25 || d.Fees != 1 {
		t.Fatalf("bad totals: %+v", d)
	}
	b, _ := json.Marshal(d)
	for _, pii := range []string{"test@example.com", "Test User", "\"User\""} {
		if strings.Contains(string(b), pii) {
			t.Fatalf("customer PII leaked into output: %s", pii)
		}
	}
}

func TestOrderVariablesRequiresID(t *testing.T) {
	if _, err := OrderVariables(" ", "", false); err == nil {
		t.Fatal("want error for empty order id")
	}
	v, err := OrderVariables("2000000000000001", "9000000000000001", true)
	if err != nil || !strings.Contains(v, `"orderIsInStore":true`) {
		t.Fatalf("bad vars %s %v", v, err)
	}
}

func TestHistoryVariablesFilters(t *testing.T) {
	v, err := HistoryVariables("", "milk", 0, []string{"in-store"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"limit":10`, `"search":"milk"`, `"filterIds":["in-store"]`, `"platform":"WEB"`} {
		if !strings.Contains(v, want) {
			t.Fatalf("vars missing %s: %s", want, v)
		}
	}
}

const fakeSearch = `<html><head></head><body><script id="__NEXT_DATA__" type="application/json">{"props":{"pageProps":{"initialData":{"searchResult":{
"aggregatedCount":2,"hasMorePages":false,"itemStacks":[{"items":[
{"__typename":"Product","usItemId":"100000001","name":"Test Milk","price":3.5,"priceInfo":{"linePrice":"","linePriceDisplay":"","itemPrice":"","unitPrice":"5.5 ¢/fl oz"},
 "availabilityStatusV2":{"display":"In stock","value":"IN_STOCK"},"fulfillmentSummary":[{"fulfillment":"PICKUP","storeId":"0000"},{"fulfillment":"DELIVERY","storeId":"0000"}],"canonicalUrl":"/ip/test-milk/100000001"},
{"__typename":"AdPlaceholder"}]}]}}}}}</script></body></html>`

func TestParseSearchHTML(t *testing.T) {
	s, err := ParseSearchHTML([]byte(fakeSearch), "milk", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Products) != 1 || s.StoreID != "0000" || s.Total != 2 {
		t.Fatalf("bad search: %+v", s)
	}
	p := s.Products[0]
	if p.PriceDisplay != "$3.50" || p.Fulfillment != "pickup,delivery" || p.Availability != "In stock" || p.URL == "" {
		t.Fatalf("bad product: %+v", p)
	}
	if _, err := ParseSearchHTML([]byte("<html>Robot or human?</html>"), "milk", 1); err == nil {
		t.Fatal("want error when __NEXT_DATA__ is missing")
	}
}
