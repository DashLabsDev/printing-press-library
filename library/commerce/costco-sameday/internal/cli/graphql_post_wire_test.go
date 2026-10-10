// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.
// Wire-shape tests for GraphQL POST commands against a local stub (fake
// values only, nothing leaves the machine): the request must carry
// operationName / variables / extensions in the JSON body and no query
// string, or Costco answers "Blank Query".

package cli

import "testing"

func TestGraphQLPostCommandsSendBodyOnly(t *testing.T) {
	cases := []struct {
		op   string
		args []string
		want map[string]any
	}{
		{"UpdateCartItemsMutation", []string{"cart", "updatecartitemsmutation", "--cart-id", "cart-fake", "--cart-type", "grocery", "--json"}, map[string]any{"cartId": "cart-fake", "cartType": "grocery"}},
		{"InitializeCheckout", []string{"checkout", "initializecheckout", "--cart-id", "cart-fake", "--shop-id", "shop-fake", "--json"}, map[string]any{"cartId": "cart-fake", "shopId": "shop-fake"}},
		{"FinalizeCheckout", []string{"order", "place", "--checkout-session-id", "sess-fake", "--yes", "--confirm-charge", "--json"}, map[string]any{"checkoutSessionId": "sess-fake"}},
	}
	for _, tc := range cases {
		t.Run(tc.op, func(t *testing.T) {
			rec := routedSameDay(t, map[string]string{tc.op: `{"data":{"ok":true}}`})
			_, _ = runRoot(t, tc.args...)
			rec.mu.Lock()
			defer rec.mu.Unlock()
			if rec.methods[tc.op] != "POST" {
				t.Fatalf("%s not sent as POST (methods %v)", tc.op, rec.methods)
			}
			if q := rec.query[tc.op]; q != "" {
				t.Fatalf("%s POST carried a query string %q", tc.op, q)
			}
			body := rec.bodies[tc.op]
			if body["operationName"] != tc.op {
				t.Fatalf("body operationName = %v", body["operationName"])
			}
			ext, _ := body["extensions"].(map[string]any)
			if pq, _ := ext["persistedQuery"].(map[string]any); pq["sha256Hash"] == nil || pq["sha256Hash"] == "" {
				t.Fatalf("body lacks persisted hash: %v", body["extensions"])
			}
			for k, v := range tc.want {
				if rec.vars[tc.op][k] != v {
					t.Fatalf("variables[%s] = %v (all %v)", k, rec.vars[tc.op][k], rec.vars[tc.op])
				}
			}
		})
	}
}
