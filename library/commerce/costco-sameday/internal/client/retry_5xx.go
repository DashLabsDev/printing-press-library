// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: which GraphQL POSTs may be retried after a 5xx.
// A 5xx from InitializeCheckout (it opens a checkout session; no charge, no
// order) is usually transient and retrying is what the site does on reload.
// Only operations named here qualify, and FinalizeCheckout never does, even
// if its hash were sent under another name. Transport errors (where a write
// may have committed) keep the stricter rule.
// Preserved across regenerate via
// .printing-press-patches/costco-sameday-order-start.json.

package client

import "strings"

// serverErrorRetryableOperations are non-charging GraphQL POSTs that are safe
// to resend after the server answered 5xx.
var serverErrorRetryableOperations = map[string]bool{
	"InitializeCheckout": true,
}

func serverErrorRetryable(path string, params map[string]string, body []byte, finalizeHashes []string) bool {
	if !strings.Contains(path, "graphql") {
		return false
	}
	if requestTargetsFinalizeCheckout(path, params, body, finalizeHashes) {
		return false
	}
	names := collectGraphQLOperationNames(params, body)
	if len(names) == 0 {
		return false
	}
	for _, n := range names {
		if !serverErrorRetryableOperations[n] {
			return false
		}
	}
	return true
}
