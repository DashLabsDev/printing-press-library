// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.
// normalizeMoneyParam validation (fake amounts only).

package client

import (
	"encoding/json"
	"testing"
)

func TestNormalizeMoneyParamStrict(t *testing.T) {
	good := map[string]string{
		"42":     `{"amount":"42","currencyCode":"USD"}`,
		" 42.5 ": `{"amount":"42.5","currencyCode":"USD"}`,
		"0.99":   `{"amount":"0.99","currencyCode":"USD"}`,
		`{"amount":"12.34","currencyCode":"USD"}`: `{"amount":"12.34","currencyCode":"USD"}`,
		`{"amount":12.3,"currencyCode":"CAD"}`:    `{"amount":"12.3","currencyCode":"CAD"}`,
		`{"amount":"7"}`:                          `{"amount":"7","currencyCode":"USD"}`,
	}
	for in, want := range good {
		out, err := normalizeMoneyParam(map[string]string{"subtotal": in, "other": "x"}, "subtotal")
		if err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		var got, exp any
		if json.Unmarshal([]byte(out["subtotal"]), &got) != nil {
			t.Fatalf("%q produced invalid JSON %q", in, out["subtotal"])
		}
		_ = json.Unmarshal([]byte(want), &exp)
		gb, _ := json.Marshal(got)
		eb, _ := json.Marshal(exp)
		if string(gb) != string(eb) || out["other"] != "x" {
			t.Fatalf("%q -> %s, want %s", in, gb, eb)
		}
	}
	bad := []string{"+50", "50.", ".5", "-5", "1e3", "0x10", "$5", "5.123", "1,000", "007", "NaN", "Inf",
		`{"amount":"+5"}`, `{"amount":true}`, `{"amount":"5","currencyCode":"usd"}`, `{"amount":"5","extra":1}`, `{bad json`}
	for _, in := range bad {
		if _, err := normalizeMoneyParam(map[string]string{"subtotal": in}, "subtotal"); err == nil {
			t.Fatalf("%q accepted", in)
		}
	}
	if out, err := normalizeMoneyParam(map[string]string{}, "subtotal"); err != nil || len(out) != 0 {
		t.Fatalf("empty: %v %v", out, err)
	}
}
