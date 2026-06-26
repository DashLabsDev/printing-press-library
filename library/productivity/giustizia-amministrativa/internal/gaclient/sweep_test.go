package gaclient

import "testing"

func TestYearRange(t *testing.T) {
	cases := []struct {
		from, to     int
		wantF, wantT int
	}{
		{2016, 2026, 2016, 2026}, // normal span
		{2020, 0, 2020, 2020},    // only from → single year
		{0, 2020, 2020, 2020},    // only to → single year
		{2026, 2016, 2016, 2026}, // reversed → swapped
		{2021, 2021, 2021, 2021}, // single year
	}
	for _, c := range cases {
		f, to := yearRange(c.from, c.to)
		if f != c.wantF || to != c.wantT {
			t.Errorf("yearRange(%d,%d) = (%d,%d), want (%d,%d)", c.from, c.to, f, to, c.wantF, c.wantT)
		}
	}
}

func TestDedupKey(t *testing.T) {
	// ECLI wins; then idprovv; then the document coordinates. An item lacking
	// every identifier must still yield a stable, non-empty key so the sweep
	// de-duplicates it instead of repeating it once per year.
	if got := dedupKey(Provvedimento{Ecli: "E", Idprovv: "I"}); got != "E" {
		t.Errorf("ecli should win, got %q", got)
	}
	if got := dedupKey(Provvedimento{Idprovv: "I"}); got != "I" {
		t.Errorf("idprovv fallback, got %q", got)
	}
	noID := Provvedimento{Schema: "tar_na", Nrg: "202102978", NomeFile: "202106765_01.html"}
	if got := dedupKey(noID); got != "tar_na|202102978|202106765_01.html" {
		t.Errorf("coordinate fallback, got %q", got)
	}
	if dedupKey(noID) != dedupKey(noID) {
		t.Errorf("dedupKey not stable for identical items")
	}
}
