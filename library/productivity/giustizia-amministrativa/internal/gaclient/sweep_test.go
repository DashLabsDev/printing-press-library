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
