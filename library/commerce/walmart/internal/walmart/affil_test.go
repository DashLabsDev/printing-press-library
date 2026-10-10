// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.

package walmart

import (
	"net/url"
	"strings"
	"testing"
)

func TestAffilAddToCartURLSingle(t *testing.T) {
	u, err := AffilAddToCartURL([]AffilCartItem{{USItemID: "51259338", Quantity: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(u, AffilAddToCartHost+AffilAddToCartPath+"?") {
		t.Fatalf("host/path: %s", u)
	}
	q, err := url.Parse(u)
	if err != nil {
		t.Fatal(err)
	}
	if got := q.Query().Get("items"); got != "51259338|1" {
		t.Fatalf("items=%q", got)
	}
}

func TestAffilAddToCartURLMulti(t *testing.T) {
	u, err := AffilAddToCartURL([]AffilCartItem{
		{USItemID: "51259338", Quantity: 1},
		{USItemID: "44391152", Quantity: 2},
		{USItemID: "https://www.walmart.com/ip/Example/10000000001", Quantity: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := mustQuery(t, u, "items")
	if got != "51259338|1,44391152|2,10000000001|3" {
		t.Fatalf("items=%q", got)
	}
	parsed, err := AffilParseItems(got)
	if err != nil || len(parsed) != 3 || parsed[1].Quantity != 2 {
		t.Fatalf("round-trip: %v %v", parsed, err)
	}
}

func TestAffilAddToCartURLRejects(t *testing.T) {
	for _, items := range [][]AffilCartItem{
		nil,
		{{USItemID: "not-an-id", Quantity: 1}},
		{{USItemID: "51259338", Quantity: 0}},
		{{USItemID: "51259338", Quantity: MaxCartLineQuantity + 1}},
		{{USItemID: "51259338", Quantity: 1}, {USItemID: "51259338", Quantity: 2}},
	} {
		if _, err := AffilAddToCartURL(items); err == nil {
			t.Errorf("want error for %+v", items)
		}
	}
}



func mustQuery(t *testing.T, raw, key string) string {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u.Query().Get(key)
}
