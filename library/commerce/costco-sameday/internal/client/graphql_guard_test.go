package client

import (
	"context"
	"encoding/json"
	"testing"
)

func TestFoldGraphQLVariablesPreservesTypes(t *testing.T) {
	params := map[string]string{
		"operationName": "Items",
		"ids":           `["a","b"]`,
		"shopId":        "123",
		"zoneId":        "45",
		"postalCode":   "90210",
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
	if err := enforceGraphQLOperationSafety(context.Background(), params, nil); err == nil {
		t.Fatal("expected refusal without consent")
	}
	ctx := WithFinalizeCheckoutConsent(context.Background())
	if err := enforceGraphQLOperationSafety(ctx, params, nil); err != nil {
		t.Fatalf("consent should allow: %v", err)
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
