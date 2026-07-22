package cli

import (
	"testing"

	icaro "github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/icaroclient"
)

func TestToISISDate(t *testing.T) {
	cases := []struct{ in, want string }{
		{"2026-02-25", "260225"},
		{"2026-02-24:2026-02-25", "260224/260225"},
		{"260225", "260225"},                         // already AAMMGG → unchanged
		{"non-data", "non-data"},                     // unparseable → unchanged
		{"abcd-ef-gh", "abcd-ef-gh"},                 // right shape but non-numeric → unchanged
		{"2026-02-25:", "2026-02-25:"},               // trailing colon → not a range; unparseable → unchanged
		{":2026-02-25", ":2026-02-25"},               // leading colon → not a range; unparseable → unchanged
		{"2026-02-25:garbage", "2026-02-25:garbage"}, // one invalid bound → no malformed range
		{"260224:260225", "260224/260225"},           // already-AAMMGG bounds → valid range
	}
	for _, c := range cases {
		if got := toISISDate(c.in); got != c.want {
			t.Errorf("toISISDate(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestYearToISISRange(t *testing.T) {
	cases := []struct{ in, want string }{
		{"2024", "240101/241231"},
		{"1999", "990101/991231"},
		{"24", "24"},           // not 4 digits → unchanged
		{"20245", "20245"},     // not 4 digits → unchanged
		{"abcd", "abcd"},       // non-numeric → unchanged
		{"240101/241231", "240101/241231"}, // already a range → unchanged
	}
	for _, c := range cases {
		if got := yearToISISRange(c.in); got != c.want {
			t.Errorf("yearToISISRange(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestNormalizeParams_AnnoOnDdlBecomesDatpreRange covers the bug where
// `ddl cerca --anno 2024` matched "2024" as free text anywhere in the
// document (no DATPRE field for ddl to qualify a plain year against),
// returning DDLs from other years that merely mention "2024" in the text.
func TestNormalizeParams_AnnoOnDdlBecomesDatpreRange(t *testing.T) {
	arc := *icaro.BySlug("ddl")
	out := normalizeParams(arc, map[string]string{"anno": "2024"})
	if out["anno"] != "240101/241231" {
		t.Errorf("anno = %q, want 240101/241231", out["anno"])
	}
}

// TestNormalizeParams_AnnoOnLeggiUnchanged covers archives that already have
// a real year field (leggi.LEGANN, resoconti.ANNSED): --anno must stay a
// bare year there, not be rewritten into a DATPRE-style range.
func TestNormalizeParams_AnnoOnLeggiUnchanged(t *testing.T) {
	arc := *icaro.BySlug("leggi")
	out := normalizeParams(arc, map[string]string{"anno": "2024"})
	if out["anno"] != "2024" {
		t.Errorf("anno = %q, want unchanged 2024", out["anno"])
	}
}

func TestCommissioneOrdinale(t *testing.T) {
	cases := []struct{ in, want string }{
		{"1", "PRIMA"},
		{"6", "SESTA"},
		{"7", ""},
		{"SESTA", ""},
	}
	for _, c := range cases {
		if got := commissioneOrdinale(c.in); got != c.want {
			t.Errorf("commissioneOrdinale(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNormalizeParams_DataAndCodcom(t *testing.T) {
	arc := *icaro.BySlug("convocazioni")
	out := normalizeParams(arc, map[string]string{
		"legisl": "18",
		"data":   "2026-02-25",
		"codcom": "6",
	})
	if out["data"] != "260225" {
		t.Errorf("data = %q, want 260225", out["data"])
	}
	if _, ok := out["codcom"]; ok {
		t.Errorf("codcom should be rerouted/removed, got %q", out["codcom"])
	}
	if out["commissione"] != "SESTA" {
		t.Errorf("commissione = %q, want SESTA", out["commissione"])
	}
}
