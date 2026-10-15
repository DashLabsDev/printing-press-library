// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: GraphQL failure detection for detectPartialFailure.
// GraphQL answers HTTP 200 even when an operation failed. Two shapes mean
// failure and must exit non-zero with "success": false:
//   - a non-empty top-level "errors" array (with or without data), and
//   - a root field whose __typename is an error type, such as
//     InitializeCheckout's CheckoutCheckoutErrorsCheckoutRedirectError /
//     ...EmptyCartError or UpdateCheckout's CheckoutCheckoutErrorResponse.
// Preserved across regenerate via
// .printing-press-patches/costco-sameday-graphql-failures-exit-nonzero.json.

package cli

import (
	"fmt"
	"sort"
	"strings"
)

// isGraphQLErrorTypename reports whether a __typename names an error result.
func isGraphQLErrorTypename(tn string) bool {
	return strings.HasSuffix(tn, "Error") || strings.HasSuffix(tn, "ErrorResponse")
}

// detectGraphQLFailure returns a report for a GraphQL failure body, or nil.
func detectGraphQLFailure(top map[string]any) *partialFailureReport {
	if v, _ := top["__pp_verify_synthetic__"].(bool); v {
		return nil
	}
	if errs, ok := top["errors"].([]any); ok && len(errs) > 0 {
		var msgs []string
		for _, e := range errs {
			m, _ := e.(map[string]any)
			if s, _ := m["message"].(string); strings.TrimSpace(s) != "" {
				msgs = append(msgs, strings.TrimSpace(s))
			}
		}
		if len(msgs) == 0 {
			msgs = []string{fmt.Sprintf("%d GraphQL error(s)", len(errs))}
		}
		return &partialFailureReport{Field: "errors", Message: strings.Join(msgs, "; "), Details: errs}
	}
	data, _ := top["data"].(map[string]any)
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		root, _ := data[k].(map[string]any)
		tn, _ := root["__typename"].(string)
		if !isGraphQLErrorTypename(tn) {
			continue
		}
		return &partialFailureReport{Field: "data." + k + ".__typename", Message: graphQLErrorText(tn, root), Details: root}
	}
	return nil
}

// graphQLErrorText builds a readable message from an error-typed result:
// its typename plus any error code and message it (or its first nested
// error) carries.
func graphQLErrorText(tn string, root map[string]any) string {
	parts := []string{tn}
	add := func(m map[string]any) {
		for _, k := range []string{"error", "code", "message"} {
			if s, _ := m[k].(string); strings.TrimSpace(s) != "" {
				parts = append(parts, strings.TrimSpace(s))
			}
		}
		if vs, ok := m["viewSection"].(map[string]any); ok {
			for _, k := range []string{"titleString", "toastDescriptionString", "descriptionString"} {
				if s, _ := vs[k].(string); strings.TrimSpace(s) != "" {
					parts = append(parts, strings.TrimSpace(s))
				}
			}
		}
	}
	add(root)
	if inner, ok := root["errors"].([]any); ok && len(inner) > 0 {
		if m, ok := inner[0].(map[string]any); ok {
			if itn, _ := m["__typename"].(string); itn != "" {
				parts = append(parts, itn)
			}
			add(m)
		}
	}
	return strings.Join(parts, ": ")
}
