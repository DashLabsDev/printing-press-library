// Copyright 2026 dashlabsdev and contributors. Licensed under Apache-2.0.
// Hand-authored GraphQL safety guards (charge consent + variables fold).
// Preserved across regenerate via .printing-press-patches records.

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type ctxKeyDeclaredOp struct{}
type ctxKeyFinalizeConsent struct{}

// WithDeclaredGraphQLOperation locks outbound GraphQL requests in ctx to op.
// Generated commands should pass their declared default; overrides and stdin
// bodies that name a different operation are rejected in doInternal.
func WithDeclaredGraphQLOperation(ctx context.Context, op string) context.Context {
	op = strings.TrimSpace(op)
	if op == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKeyDeclaredOp{}, op)
}

// WithFinalizeCheckoutConsent permits FinalizeCheckout for this request only.
// Set only after order place dual-gate (--yes + --confirm-charge) passes.
func WithFinalizeCheckoutConsent(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKeyFinalizeConsent{}, true)
}

func declaredGraphQLOperation(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	op, _ := ctx.Value(ctxKeyDeclaredOp{}).(string)
	return op
}

func hasFinalizeCheckoutConsent(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	ok, _ := ctx.Value(ctxKeyFinalizeConsent{}).(bool)
	return ok
}

var graphqlParamMetaKeys = map[string]struct{}{
	"operationName": {},
	"extensions":    {},
	"variables":     {},
	"query":         {},
	"rawQuery":      {},
}

// foldGraphQLVariables moves non-meta query params into the GraphQL variables
// JSON object so flag values reach the server. Preserves object/array/number/
// boolean types when the flag value is valid JSON; otherwise keeps a string.
func foldGraphQLVariables(params map[string]string) map[string]string {
	if len(params) == 0 {
		return params
	}
	updated := make(map[string]string, len(params))
	vars := map[string]any{}
	if raw := params["variables"]; raw != "" && raw != "{}" {
		_ = json.Unmarshal([]byte(raw), &vars)
		if vars == nil {
			vars = map[string]any{}
		}
	}
	for k, v := range params {
		if _, meta := graphqlParamMetaKeys[k]; meta {
			updated[k] = v
			continue
		}
		if v == "" {
			continue
		}
		vars[k] = decodeGraphQLVariableValue(v)
	}
	if len(vars) > 0 {
		b, err := json.Marshal(vars)
		if err == nil {
			updated["variables"] = string(b)
		}
	} else if updated["variables"] == "" {
		updated["variables"] = "{}"
	}
	return updated
}

func decodeGraphQLVariableValue(raw string) any {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	// Only JSON-decode when the flag value is already JSON-shaped. Bare
	// digit strings (shop ids, postal codes) stay strings so GraphQL String
	// fields are not widened to Int unexpectedly.
	if s[0] == '{' || s[0] == '[' || s[0] == '"' || s == "true" || s == "false" || s == "null" {
		var v any
		if err := json.Unmarshal([]byte(s), &v); err == nil {
			return v
		}
	}
	return raw
}

func extractGraphQLOperationName(params map[string]string, body []byte) string {
	if params != nil {
		if op := strings.TrimSpace(params["operationName"]); op != "" {
			return op
		}
	}
	if len(body) == 0 {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	op, _ := payload["operationName"].(string)
	return strings.TrimSpace(op)
}

// enforceGraphQLOperationSafety rejects FinalizeCheckout without consent and
// rejects operation names that disagree with a declared command lock.
func enforceGraphQLOperationSafety(ctx context.Context, params map[string]string, body []byte) error {
	op := extractGraphQLOperationName(params, body)
	if op == "" {
		return nil
	}
	if declared := declaredGraphQLOperation(ctx); declared != "" && op != declared {
		return fmt.Errorf("refusing GraphQL operation %q: this command is locked to %q (use the narrative order commands for charge/cancel flows)", op, declared)
	}
	if op == "FinalizeCheckout" && !hasFinalizeCheckoutConsent(ctx) {
		return fmt.Errorf("refusing FinalizeCheckout without charge consent: use `order place --yes --confirm-charge` (or --dry-run); raw checkout commands cannot charge")
	}
	return nil
}

// coalesceGraphQLPOSTBody ensures POST /graphql sends operationName + variables
// in the JSON body (Costco Same-Day expects body variables, not loose query params).
func coalesceGraphQLPOSTBody(params map[string]string, body []byte) ([]byte, map[string]string, error) {
	params = foldGraphQLVariables(params)
	var payload map[string]any
	if len(body) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			return body, params, nil // leave non-JSON bodies alone
		}
	}
	if payload == nil {
		payload = map[string]any{}
	}
	if op := strings.TrimSpace(params["operationName"]); op != "" {
		if existing, _ := payload["operationName"].(string); existing == "" {
			payload["operationName"] = op
		}
	}
	if _, ok := payload["variables"]; !ok {
		vars := map[string]any{}
		if raw := params["variables"]; raw != "" {
			_ = json.Unmarshal([]byte(raw), &vars)
		}
		payload["variables"] = vars
	}
	if _, ok := payload["extensions"]; !ok {
		if raw := params["extensions"]; raw != "" {
			var ext any
			if err := json.Unmarshal([]byte(raw), &ext); err == nil {
				payload["extensions"] = ext
			}
		}
	}
	out, err := json.Marshal(payload)
	if err != nil {
		return body, params, err
	}
	// Keep operationName + extensions + variables as query companions for persisted-query GETs;
	// for POST, strip folded variable keys already moved into body (meta keys may remain).
	return out, params, nil
}

// GraphQLResponseSucceeded reports whether a GraphQL HTTP response actually
// succeeded: 2xx, no top-level errors, and not a verify-mode synthetic noop.
func GraphQLResponseSucceeded(status int, data []byte) bool {
	if status < 200 || status >= 300 {
		return false
	}
	if len(data) == 0 {
		return false
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return false
	}
	if syn, _ := payload["__pp_verify_synthetic__"].(bool); syn {
		return false
	}
	if errs, ok := payload["errors"]; ok && errs != nil {
		switch e := errs.(type) {
		case []any:
			if len(e) > 0 {
				return false
			}
		case string:
			if strings.TrimSpace(e) != "" {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// RESTResponseSucceeded reports whether a REST mutating response succeeded
// (2xx and not a verify-mode synthetic noop).
func RESTResponseSucceeded(status int, data []byte) bool {
	if status < 200 || status >= 300 {
		return false
	}
	if len(data) == 0 {
		return true // empty 2xx is fine for some REST cancels
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return true // non-JSON 2xx body still counts as HTTP success
	}
	if syn, _ := payload["__pp_verify_synthetic__"].(bool); syn {
		return false
	}
	return true
}
