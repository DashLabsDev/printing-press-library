package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/costco-sameday/internal/config"
)

func TestFoldGraphQLVariablesPreservesTypes(t *testing.T) {
	params := map[string]string{
		"operationName": "Items",
		"ids":           `["a","b"]`,
		"shopId":        "123",
		"zoneId":        "45",
		"postalCode":    "90210",
	}
	out := foldGraphQLVariables(params)
	if out["operationName"] != "Items" {
		t.Fatalf("operationName=%q", out["operationName"])
	}
	if _, ok := out["ids"]; ok {
		t.Fatal("ids should be folded into variables, not left as top-level param")
	}
	var vars map[string]any
	if err := json.Unmarshal([]byte(out["variables"]), &vars); err != nil {
		t.Fatal(err)
	}
	ids, ok := vars["ids"].([]any)
	if !ok || len(ids) != 2 {
		t.Fatalf("ids=%v", vars["ids"])
	}
	if vars["shopId"] != "123" {
		t.Fatalf("shopId=%T %v", vars["shopId"], vars["shopId"])
	}
	if vars["postalCode"] != "90210" {
		t.Fatalf("postalCode=%v", vars["postalCode"])
	}
}

func TestEnforceFinalizeCheckoutRequiresConsent(t *testing.T) {
	params := map[string]string{"operationName": "FinalizeCheckout"}
	cases := []struct {
		name        string
		ctx         context.Context
		wantAllowed bool
	}{
		{"no consent", context.Background(), false},
		{"yes only", WithChargeConsent(context.Background(), true, false), false},
		{"confirm-charge only", WithChargeConsent(context.Background(), false, true), false},
		{"both flags", WithChargeConsent(context.Background(), true, true), true},
	}
	for _, tc := range cases {
		err := enforceChargeGate(tc.ctx, "/graphql", params, nil, nil)
		if tc.wantAllowed && err != nil {
			t.Fatalf("%s: expected allowed, got %v", tc.name, err)
		}
		if !tc.wantAllowed && err == nil {
			t.Fatalf("%s: expected refusal", tc.name)
		}
	}
}

func TestEnforceDeclaredOperationLock(t *testing.T) {
	ctx := WithDeclaredGraphQLOperation(context.Background(), "UpdateCheckout")
	body, _ := json.Marshal(map[string]any{"operationName": "FinalizeCheckout"})
	if err := enforceGraphQLOperationSafety(ctx, nil, body); err == nil {
		t.Fatal("expected refusal for mismatched stdin operation")
	}
}

func TestGraphQLResponseSucceededHonesty(t *testing.T) {
	if GraphQLResponseSucceeded(200, []byte(`{"errors":[{"message":"nope"}]}`)) {
		t.Fatal("errors must not count as success")
	}
	if GraphQLResponseSucceeded(200, []byte(`{"__pp_verify_synthetic__":true,"status":"noop"}`)) {
		t.Fatal("verify synthetic must not count as success")
	}
	if !GraphQLResponseSucceeded(200, []byte(`{"data":{"ok":true}}`)) {
		t.Fatal("clean 200 should succeed")
	}
	if RESTResponseSucceeded(200, []byte(`{"__pp_verify_synthetic__":true}`)) {
		t.Fatal("REST verify synthetic must not count as success")
	}
}

func TestApplyPersistedQueryParamOverridesKeepsRESTQueryParams(t *testing.T) {
	c := &Client{}
	params := map[string]string{"source": "web"}
	out := c.applyPersistedQueryParamOverrides(params)
	if out["source"] != "web" {
		t.Fatalf("REST cancel source=web must remain a query param, got %v", out)
	}
	if _, ok := out["variables"]; ok {
		t.Fatalf("REST params must not be folded into variables, got %v", out)
	}
}

func TestApplyPersistedQueryParamOverridesStillFoldsGraphQL(t *testing.T) {
	c := &Client{}
	params := map[string]string{
		"operationName": "Items",
		"shopId":        "123",
	}
	out := c.applyPersistedQueryParamOverrides(params)
	if out["operationName"] != "Items" {
		t.Fatalf("operationName=%q", out["operationName"])
	}
	if _, ok := out["shopId"]; ok {
		t.Fatal("shopId should be folded into variables for GraphQL")
	}
	var vars map[string]any
	if err := json.Unmarshal([]byte(out["variables"]), &vars); err != nil {
		t.Fatal(err)
	}
	if vars["shopId"] != "123" {
		t.Fatalf("shopId=%v", vars["shopId"])
	}
}

// newCountingGraphQLServer returns a mock server (no network) that counts
// every request it receives and replies with a FinalizeCheckout-shaped body.
func newCountingGraphQLServer(t *testing.T) (*httptest.Server, *int32) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"finalizeCheckout":{"legacyOrderId":"1"}}}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func TestChargeGateRefusesEveryFinalizeCheckoutRouteWithoutBothFlags(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	finalizeHash := (&Client{}).finalizeCheckoutHashes()
	if len(finalizeHash) == 0 {
		t.Fatal("expected embedded FinalizeCheckout persisted-query hash in seed")
	}
	routes := []struct {
		name   string
		method string
		params map[string]string
		body   any
	}{
		// checkout updatecheckout --operation-name FinalizeCheckout
		{"operation-name flag", "POST", map[string]string{"operationName": "FinalizeCheckout"}, map[string]any{}},
		// checkout updatecheckout --stdin with a FinalizeCheckout body
		{"stdin body", "POST", nil, map[string]any{"operationName": "FinalizeCheckout", "variables": map[string]any{"checkoutSessionId": "s"}}},
		// declared op in params, charge op smuggled in body
		{"param/body mismatch", "POST", map[string]string{"operationName": "UpdateCheckout"}, map[string]any{"operationName": "FinalizeCheckout"}},
		{"batched array body", "POST", nil, []any{map[string]any{"operationName": "UpdateCheckout"}, map[string]any{"operationName": "FinalizeCheckout"}}},
		{"query text only", "POST", nil, map[string]any{"query": "mutation X { finalizeCheckout(input:{}) { legacyOrderId } }"}},
		{"persisted hash only", "POST", nil, map[string]any{"operationName": "Innocent", "extensions": map[string]any{"persistedQuery": map[string]any{"version": 1, "sha256Hash": finalizeHash[0]}}}},
		{"GET persisted query", "GET", map[string]string{"operationName": "FinalizeCheckout", "variables": "{}"}, nil},
		{"lowercase op name", "POST", map[string]string{"operationName": "finalizecheckout"}, nil},
	}
	consents := []struct {
		name string
		ctx  context.Context
	}{
		{"no consent", context.Background()},
		{"yes only", WithChargeConsent(context.Background(), true, false)},
		{"confirm-charge only", WithChargeConsent(context.Background(), false, true)},
	}
	for _, rt := range routes {
		for _, cs := range consents {
			srv, hits := newCountingGraphQLServer(t)
			c := New(&config.Config{BaseURL: srv.URL}, time.Second, 0)
			_, _, err := c.doInternal(cs.ctx, rt.method, "/graphql", rt.params, rt.body, nil, false, false)
			if err == nil || !strings.Contains(err.Error(), "refusing FinalizeCheckout without charge consent") {
				t.Fatalf("%s / %s: expected charge-gate refusal, got %v", rt.name, cs.name, err)
			}
			if n := atomic.LoadInt32(hits); n != 0 {
				t.Fatalf("%s / %s: request reached server (%d hits) without both flags", rt.name, cs.name, n)
			}
		}
	}
}

func TestChargeGateAllowsFinalizeCheckoutWithBothFlags(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	srv, hits := newCountingGraphQLServer(t)
	c := New(&config.Config{BaseURL: srv.URL}, time.Second, 0)
	ctx := WithChargeConsent(context.Background(), true, true)
	data, status, err := c.PostWithParams(ctx, "/graphql", map[string]string{"operationName": "FinalizeCheckout"}, map[string]any{"operationName": "FinalizeCheckout"})
	if err != nil {
		t.Fatalf("both flags should allow: %v", err)
	}
	if atomic.LoadInt32(hits) != 1 {
		t.Fatalf("expected exactly one request, got %d", atomic.LoadInt32(hits))
	}
	if ClassifyFinalizeCheckoutResponse(status, data) != ChargeConfirmed {
		t.Fatalf("mock success should classify as charged: %s", data)
	}
}

func TestChargeGateRefusesBeforeVerifyShortCircuit(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "")
	c := New(&config.Config{BaseURL: "http://127.0.0.1:1"}, time.Second, 0)
	if _, _, err := c.PostWithParams(context.Background(), "/graphql", map[string]string{"operationName": "FinalizeCheckout"}, nil); err == nil {
		t.Fatal("charge gate must refuse even in verify mode")
	}
}

func TestChargeGateIgnoresUnrelatedOperations(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	srv, hits := newCountingGraphQLServer(t)
	c := New(&config.Config{BaseURL: srv.URL}, time.Second, 0)
	if _, _, err := c.PostWithParams(context.Background(), "/graphql", map[string]string{"operationName": "UpdateCheckout"}, map[string]any{"operationName": "UpdateCheckout"}); err != nil {
		t.Fatalf("unrelated op should pass: %v", err)
	}
	if atomic.LoadInt32(hits) != 1 {
		t.Fatalf("expected request to be sent, hits=%d", atomic.LoadInt32(hits))
	}
}

func TestClassifyFinalizeCheckoutResponse(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"real success", 200, `{"data":{"finalizeCheckout":{"legacyOrderId":"123","orderDeliveryId":"456"}}}`, ChargeConfirmed},
		{"200 with GraphQL errors", 200, `{"errors":[{"message":"payment declined"}],"data":{"finalizeCheckout":null}}`, ChargeNotCharged},
		{"200 with payload errors", 200, `{"data":{"finalizeCheckout":{"errors":[{"message":"risk"}]}}}`, ChargeNotCharged},
		{"synthetic verify reply", 200, `{"__pp_verify_synthetic__":true,"status":"noop","reason":"verify_short_circuit"}`, ChargeNotCharged},
		{"http 500", 500, `{"data":{"finalizeCheckout":{"legacyOrderId":"1"}}}`, ChargeNotCharged},
		{"200 null data", 200, `{"data":null}`, ChargeUnconfirmed},
		{"200 missing result", 200, `{"data":{"somethingElse":{}}}`, ChargeUnconfirmed},
	}
	for _, tc := range cases {
		if got := ClassifyFinalizeCheckoutResponse(tc.status, []byte(tc.body)); got != tc.want {
			t.Fatalf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
}

func TestRESTResponseSucceededCancelCases(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   bool
	}{
		{"real success empty", 200, ``, true},
		{"real success json", 200, `{"order_id":"1","status":"canceled"}`, true},
		{"synthetic verify reply", 200, `{"__pp_verify_synthetic__":true,"status":"noop"}`, false},
		{"200 error body", 200, `{"error":"order cannot be canceled"}`, false},
		{"200 errors array", 200, `{"errors":[{"message":"nope"}]}`, false},
		{"200 success false", 200, `{"success":false}`, false},
		{"200 status error", 200, `{"status":"error"}`, false},
		{"http 422", 422, `{}`, false},
	}
	for _, tc := range cases {
		if got := RESTResponseSucceeded(tc.status, []byte(tc.body)); got != tc.want {
			t.Fatalf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}
