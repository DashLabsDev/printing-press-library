// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: GraphQL ServiceType values are upper-case enums (DELIVERY,
// PICKUP, INSTORE, SCAN_AND_PAY, VIRTUAL). Accept any case on --service-type
// so "delivery" works instead of failing with a GraphQL enum error.
// Preserved across regenerate via
// .printing-press-patches/costco-sameday-order-start.json.

package client

import (
	"fmt"
	"strings"
)

// ServiceTypes are the GraphQL ServiceType enum values.
var ServiceTypes = []string{"DELIVERY", "PICKUP", "INSTORE", "SCAN_AND_PAY", "VIRTUAL"}

// NormalizeServiceType upper-cases a known ServiceType ("delivery" ->
// "DELIVERY", "scan-and-pay" -> "SCAN_AND_PAY"); unknown values error.
func NormalizeServiceType(v string) (string, error) {
	up := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(v), "-", "_"))
	for _, s := range ServiceTypes {
		if up == s {
			return s, nil
		}
	}
	return "", fmt.Errorf("invalid service type %q: use one of %s (any case)", v, strings.ToLower(strings.Join(ServiceTypes, ", ")))
}

// normalizeServiceTypeParam rewrites a known serviceType flag value to its
// enum form; anything else is left for the server to judge.
func normalizeServiceTypeParam(params map[string]string) map[string]string {
	raw, ok := params["serviceType"]
	if !ok {
		return params
	}
	norm, err := NormalizeServiceType(strings.Trim(strings.TrimSpace(raw), `"`))
	if err != nil || norm == raw {
		return params
	}
	out := make(map[string]string, len(params))
	for k, v := range params {
		out[k] = v
	}
	out["serviceType"] = norm
	return out
}
