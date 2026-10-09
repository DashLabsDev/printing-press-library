// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/walmart/internal/walmart"
)

const fakeHash = "0000000000000000000000000000000000000000000000000000000000000000"

func mutationHeaders(op string) map[string]string {
	return map[string]string{"x-apollo-operation-name": op, "x-o-gql-query": "mutation " + op}
}

func TestCheckWalmartWriteBlocksWrites(t *testing.T) {
	cases := []struct {
		method, path string
		headers      map[string]string
	}{
		{"POST", "/orchestra/cartxo/graphql/MergeAndGetCart/abc", nil},
		{"GET", "/orchestra/cartxo/graphql/MergeAndGetCart/abc", nil},
		{"GET", "/orchestra/home/graphql/getCart/abc", map[string]string{"x-apollo-operation-name": "MergeAndGetCart"}},
		{"POST", "/orchestra/pdp/graphql/ItemByIdBtf/abc", nil},
		{"POST", "/orchestra/home/graphql", nil},
		{"GET", "/orchestra/home/graphql/updateItems/abc", nil},
		{"GET", "/orchestra/home/graphql/reserveSlotMutation/abc", nil},
		{"GET", "/orchestra/cartxo/graphql/getCart/abc", map[string]string{"x-o-gql-query": "mutation getCart"}},
		{"GET", "/orchestra/x/graphql/foo/abc", map[string]string{"x-apollo-operation-name": "PlaceOrder"}},
		{"POST", "/orchestra/cartxo/graphql/updateItems/" + fakeHash, mutationHeaders("updateItems")},
		{"POST", "/orchestra/cartxo/graphql/reserveSlotMutation/" + fakeHash, mutationHeaders("reserveSlotMutation")},
		{"POST", "/orchestra/x/graphql/cancelReservation/" + fakeHash, nil},
		{"GET", "/checkout", nil},
		{"PUT", "/api/anything", nil},
		{"DELETE", "/api/anything", nil},
	}
	for _, c := range cases {
		if err := checkWalmartWrite(c.method, c.path, c.headers); !errors.Is(err, ErrWriteBlocked) {
			t.Errorf("%s %s: want ErrWriteBlocked, got %v", c.method, c.path, err)
		}
	}
}

func TestCheckWalmartWriteAllowsReads(t *testing.T) {
	ok := []string{
		"/orchestra/cph/graphql/PurchaseHistoryV3/abc",
		"/orchestra/orders/graphql/getOrder/abc",
		"/orchestra/pdp/graphql/ItemById/abc",
		"/orchestra/home/graphql/nearByNodes/abc",
		"/orchestra/home/graphql/getCart/abc",
		"/search",
	}
	for _, p := range ok {
		if err := checkWalmartWrite("GET", p, map[string]string{"x-o-gql-query": "query X"}); err != nil {
			t.Errorf("%s: unexpected %v", p, err)
		}
	}
	gs := walmart.OrchestraPath(walmart.SubgraphCartXO, walmart.OpGetSlots, walmart.HashGetSlots)
	if err := checkWalmartWrite("GET", gs, walmart.OpHeaders(walmart.OpGetSlots, walmart.CartPage)); err != nil {
		t.Errorf("getSlots read must pass: %v", err)
	}
}

func TestIsBotChallenge(t *testing.T) {
	mk := func(code int, path string) *http.Response {
		return &http.Response{StatusCode: code, Request: &http.Request{URL: &url.URL{Path: path}}}
	}
	for _, code := range []int{412, 418, 429, 456} {
		if !isBotChallenge(mk(code, "/orchestra/x"), nil) {
			t.Errorf("%d should be a challenge", code)
		}
	}
	if !isBotChallenge(mk(200, "/blocked"), []byte("<html>")) {
		t.Error("/blocked should be a challenge")
	}
	if !isBotChallenge(mk(200, "/search"), []byte(`<html><div id="px-captcha"></div></html>`)) {
		t.Error("px-captcha body should be a challenge")
	}
	if isBotChallenge(mk(200, "/search"), []byte(`<html>ok</html>`)) || isBotChallenge(mk(200, "/x"), []byte(`{"data":{}}`)) {
		t.Error("normal responses are not challenges")
	}
}

func TestWalmartGuardApplies(t *testing.T) {
	if !walmartGuardApplies("https://www.walmart.com") || !walmartGuardApplies("https://example.com") || !walmartGuardApplies("::bad") {
		t.Error("guard must apply to real hosts")
	}
	if walmartGuardApplies("http://127.0.0.1:1234") || walmartGuardApplies("http://localhost:8080") {
		t.Error("guard is off only for loopback test servers")
	}
}

func TestClientBlocksCartMutations(t *testing.T) {
	c := &Client{BaseURL: "https://www.walmart.com"}
	_, _, err := c.PostWithHeaders(context.Background(), "/orchestra/cartxo/graphql/updateItems/"+fakeHash, map[string]any{"variables": map[string]any{}}, mutationHeaders("updateItems"))
	if !errors.Is(err, ErrWriteBlocked) {
		t.Fatalf("want ErrWriteBlocked, got %v", err)
	}
}

func TestWalmartTransportErrorHidesQueryAndFlagsEOF(t *testing.T) {
	ue := &url.Error{Op: "Get", URL: "https://www.walmart.com/orchestra/home/graphql/getCart/x?variables=%7B%22cartId%22%3A%22placeholder-cart%22%7D", Err: io.EOF}
	err := walmartTransportError("GET", "/orchestra/home/graphql/getCart/x", ue)
	if !errors.Is(err, ErrBotChallenge) {
		t.Fatalf("EOF should be treated as a possible bot block: %v", err)
	}
	if strings.Contains(err.Error(), "placeholder-cart") || strings.Contains(err.Error(), "variables") {
		t.Fatalf("query leaked: %v", err)
	}
	other := walmartTransportError("GET", "/p", &url.Error{Op: "Get", URL: "https://www.walmart.com/p?q=secret", Err: errors.New("dial tcp: i/o timeout")})
	if errors.Is(other, ErrBotChallenge) || strings.Contains(other.Error(), "secret") {
		t.Fatalf("got %v", other)
	}
}
