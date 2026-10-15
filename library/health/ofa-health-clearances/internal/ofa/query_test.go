// Copyright 2026 H179922 and contributors. Licensed under Apache-2.0. See LICENSE.

package ofa

import (
	"reflect"
	"testing"
)

func TestExpandYears(t *testing.T) {
	cases := []struct {
		in      string
		want    []string
		wantErr bool
	}{
		{"2024", []string{"2024"}, false},
		{"2022-2024", []string{"2022", "2023", "2024"}, false},
		{"2024-2022", []string{"2022", "2023", "2024"}, false},
		{"2019, 2022-2023,2019", []string{"2019", "2022", "2023"}, false},
		{"", []string{}, false},
		{"20x4", nil, true},
		{"1900-2000", nil, true},
	}
	for _, c := range cases {
		got, err := ExpandYears(c.in)
		if (err != nil) != c.wantErr {
			t.Fatalf("ExpandYears(%q) err=%v wantErr=%v", c.in, err, c.wantErr)
		}
		if !c.wantErr && !reflect.DeepEqual(got, c.want) {
			t.Errorf("ExpandYears(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// The backend keeps only the last value of a repeated scalar field, so every
// multi-year query must become one form per year — never repeated keys.
func TestFormsFanOutScalarYears(t *testing.T) {
	q := Query{Breeds: []string{"AUK", "AKP"}, ReportYears: []string{"2025", "2026"}, BirthYears: []string{"2023", "2024"}, CHIC: true}
	forms := q.Forms()
	if len(forms) != 4 {
		t.Fatalf("got %d forms, want 4 (2 report years x 2 birth years)", len(forms))
	}
	seen := map[string]bool{}
	for _, f := range forms {
		if n := len(f.Fields["as_filter[reportyear]"]); n != 1 {
			t.Errorf("form %q carries %d reportyear values, want exactly 1", f.Label, n)
		}
		if n := len(f.Fields["as_filter[birthyear]"]); n != 1 {
			t.Errorf("form %q carries %d birthyear values, want exactly 1", f.Label, n)
		}
		if got := f.Fields["as_filter[brdvar][]"]; !reflect.DeepEqual(got, []string{"AUK", "AKP"}) {
			t.Errorf("breeds array not preserved: %v", got)
		}
		if f.Fields.Get("api_action") != "as_action" || f.Fields.Get("as_filter[special][chic]") != "Y" {
			t.Errorf("missing fixed fields in %v", f.Fields)
		}
		seen[f.Fields.Get("as_filter[reportyear]")+"/"+f.Fields.Get("as_filter[birthyear]")] = true
	}
	for _, k := range []string{"2025/2023", "2025/2024", "2026/2023", "2026/2024"} {
		if !seen[k] {
			t.Errorf("missing combination %s", k)
		}
	}
}

func TestFormsSingleWithoutYears(t *testing.T) {
	forms := Query{Name: "avalon", NameAnyPart: true, Sex: "m"}.Forms()
	if len(forms) != 1 {
		t.Fatalf("got %d forms, want 1", len(forms))
	}
	f := forms[0].Fields
	if f.Get("as_filter[regname]") != "AVALON" || f.Get("as_filter[fullpart]") != "A" || f.Get("as_filter[sex]") != "M" {
		t.Errorf("unexpected fields: %v", f)
	}
	if forms[0].Label != "" {
		t.Errorf("label should be empty without fan-out, got %q", forms[0].Label)
	}
}

func TestWithPageDoesNotMutate(t *testing.T) {
	base := Query{Breeds: []string{"AUK"}}.Forms()[0].Fields
	p2 := WithPage(base, 2)
	if p2.Get("api_page") != "2" || base.Get("api_page") != "" {
		t.Errorf("WithPage mutated base or failed: base=%v p2=%v", base, p2)
	}
}

func TestQueryEmpty(t *testing.T) {
	if !(Query{}).Empty() {
		t.Error("zero Query should be empty")
	}
	if (Query{CHIC: true}).Empty() || (Query{OFANum: "X"}).Empty() {
		t.Error("query with a filter should not be empty")
	}
}

func TestSplitList(t *testing.T) {
	if got := SplitList(" AUK, ,AKP,"); !reflect.DeepEqual(got, []string{"AUK", "AKP"}) {
		t.Errorf("SplitList = %v", got)
	}
}
