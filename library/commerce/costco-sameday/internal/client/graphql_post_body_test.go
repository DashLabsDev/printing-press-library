// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/costco-sameday/internal/config"
)

type wireCapture struct {
	mu     sync.Mutex
	method string
	query  string
	body   map[string]any
}

func newWireStub(t *testing.T) (*httptest.Server, *wireCapture) {
	t.Helper()
	cap := &wireCapture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		cap.mu.Lock()
		cap.method = r.Method
		cap.query = r.URL.RawQuery
		cap.body = nil
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &cap.body)
		}
		cap.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":{"ok":true}}`)
	}))
	t.Cleanup(srv.Close)
	return srv, cap
}

func seedHash(t *testing.T, op string) string {
	t.Helper()
	h := hashesFromPersistedQueryEntries(decodePersistedQueryEntries(persistedQueriesSeedJSON))[op]
	if h == "" {
		t.Fatalf("no seed hash for %s", op)
	}
	return h
}

// GraphQL POSTs must carry operationName / variables / extensions in the JSON
// body only: the server answers "Blank Query" when they are also in the URL.
func TestGraphQLPOSTSendsBodyOnlyNoQueryString(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	cases := []struct {
		op     string
		params map[string]string
		body   map[string]any
		ctx    context.Context
		want   map[string]any
	}{
		{"UpdateCartItemsMutation", map[string]string{"operationName": "UpdateCartItemsMutation", "cartId": "cart-fake", "cartType": "grocery"}, map[string]any{}, context.Background(),
			map[string]any{"cartId": "cart-fake", "cartType": "grocery"}},
		{"InitializeCheckout", map[string]string{"operationName": "InitializeCheckout", "variables": `{"cartId":"cart-fake"}`}, map[string]any{}, context.Background(),
			map[string]any{"cartId": "cart-fake"}},
		{"FinalizeCheckout", map[string]string{"operationName": "FinalizeCheckout"}, map[string]any{"operationName": "FinalizeCheckout", "variables": map[string]any{"checkoutSessionId": "sess-fake"}, "extensions": map[string]any{"persistedQuery": map[string]any{"version": 1}}},
			WithChargeConsent(context.Background(), true, true), map[string]any{"checkoutSessionId": "sess-fake"}},
	}
	for _, tc := range cases {
		srv, cap := newWireStub(t)
		c := New(&config.Config{BaseURL: srv.URL}, time.Second, 0)
		if _, _, err := c.PostWithParams(WithDeclaredGraphQLOperation(tc.ctx, tc.op), "/graphql", tc.params, tc.body); err != nil {
			t.Fatalf("%s: %v", tc.op, err)
		}
		cap.mu.Lock()
		if cap.method != http.MethodPost {
			t.Fatalf("%s: method = %s", tc.op, cap.method)
		}
		if cap.query != "" {
			t.Fatalf("%s: POST leaked a query string: %q", tc.op, cap.query)
		}
		if cap.body["operationName"] != tc.op {
			t.Fatalf("%s: body operationName = %v", tc.op, cap.body["operationName"])
		}
		vars, _ := cap.body["variables"].(map[string]any)
		for k, v := range tc.want {
			if vars[k] != v {
				t.Fatalf("%s: body variables[%s] = %v (all: %v)", tc.op, k, vars[k], vars)
			}
		}
		ext, _ := cap.body["extensions"].(map[string]any)
		pq, _ := ext["persistedQuery"].(map[string]any)
		if pq["sha256Hash"] != seedHash(t, tc.op) {
			t.Fatalf("%s: body persisted hash = %v", tc.op, pq["sha256Hash"])
		}
		cap.mu.Unlock()
	}
}

// Persisted-query GETs keep their URL companions.
func TestGraphQLGETKeepsQueryParams(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	srv, cap := newWireStub(t)
	c := New(&config.Config{BaseURL: srv.URL}, time.Second, 0)
	c.NoCache = true
	if _, err := c.Get(context.Background(), "/graphql", map[string]string{"operationName": "CartTotals", "variables": "{}"}); err != nil {
		t.Fatal(err)
	}
	cap.mu.Lock()
	defer cap.mu.Unlock()
	if cap.method != http.MethodGet || cap.body != nil {
		t.Fatalf("method=%s body=%v", cap.method, cap.body)
	}
	for _, k := range []string{"operationName=CartTotals", "variables=", "extensions="} {
		if !strings.Contains(cap.query, k) {
			t.Fatalf("GET query %q missing %s", cap.query, k)
		}
	}
}
