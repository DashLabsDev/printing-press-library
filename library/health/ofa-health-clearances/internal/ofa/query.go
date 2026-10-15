// Copyright 2026 H179922 and contributors. Licensed under Apache-2.0. See LICENSE.

// Package ofa holds the pure logic behind the OFA (Orthopedic Foundation for
// Animals) Advanced Search commands: request-form construction, HTML parsing,
// OFA-number decoding, kennel rollups, and the embedded breed/test tables.
package ofa

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// SearchPath is the backend endpoint every search and detail request uses.
const SearchPath = "/api/as.php"

// DetailRoute is the backend page route for a dog's detail record.
const DetailRoute = "/advanced-search/"

// Query is one logical Advanced Search.
//
// The backend is PHP and the year filters are scalar form fields (no "[]"),
// so a request that repeats as_filter[reportyear] keeps only the LAST value.
// Forms therefore fans the multi-valued year fields out into one request per
// combination instead of sending repeated keys.
type Query struct {
	Breeds      []string // as_filter[brdvar][] (array field)
	Tests       []string // as_filter[test][] (array field)
	ReportYears []string // as_filter[reportyear] (scalar: fanned out)
	TestYears   []string // as_filter[testyear] (scalar: fanned out)
	BirthYears  []string // as_filter[birthyear] (scalar: fanned out)
	BornFrom    string   // as_filter[birthyear_from]
	BornThru    string   // as_filter[birthyear_thru]
	CHIC        bool     // as_filter[special][chic]=Y
	Sex         string   // as_filter[sex] M|F
	Registrar   string   // as_filter[registrar] AKC|CKC|UKC
	Name        string   // as_filter[regname] (prefix unless NameAnyPart)
	NameAnyPart bool     // as_filter[fullpart]=A
	RegNum      string   // as_filter[regnum]
	OFANum      string   // as_filter[ofanum]
}

// Form is one backend request: the fields to POST plus a label describing the
// fanned-out scalar values it carries (for progress and partial-failure notes).
type Form struct {
	Label  string
	Fields url.Values
}

// Empty reports whether the query has no filter at all. The backend answers an
// unfiltered search with its form page, so callers must reject it.
func (q Query) Empty() bool {
	return len(q.Breeds) == 0 && len(q.Tests) == 0 && len(q.ReportYears) == 0 &&
		len(q.TestYears) == 0 && len(q.BirthYears) == 0 && q.BornFrom == "" &&
		q.BornThru == "" && !q.CHIC && q.Sex == "" && q.Registrar == "" &&
		q.Name == "" && q.RegNum == "" && q.OFANum == ""
}

// Forms expands the query into one backend form per combination of report
// year, test year, and birth year.
func (q Query) Forms() []Form {
	base := url.Values{}
	base.Set("api_action", "as_action")
	for _, b := range q.Breeds {
		base.Add("as_filter[brdvar][]", b)
	}
	for _, t := range q.Tests {
		base.Add("as_filter[test][]", t)
	}
	setIf(base, "as_filter[birthyear_from]", q.BornFrom)
	setIf(base, "as_filter[birthyear_thru]", q.BornThru)
	if q.CHIC {
		base.Set("as_filter[special][chic]", "Y")
	}
	setIf(base, "as_filter[sex]", strings.ToUpper(q.Sex))
	setIf(base, "as_filter[registrar]", strings.ToUpper(q.Registrar))
	setIf(base, "as_filter[regname]", strings.ToUpper(q.Name))
	if q.Name != "" && q.NameAnyPart {
		base.Set("as_filter[fullpart]", "A")
	}
	setIf(base, "as_filter[regnum]", q.RegNum)
	setIf(base, "as_filter[ofanum]", strings.ToUpper(q.OFANum))

	forms := []Form{{Fields: base}}
	forms = fanOut(forms, "as_filter[reportyear]", "reported", q.ReportYears)
	forms = fanOut(forms, "as_filter[testyear]", "tested", q.TestYears)
	forms = fanOut(forms, "as_filter[birthyear]", "born", q.BirthYears)
	return forms
}

func fanOut(forms []Form, key, label string, values []string) []Form {
	if len(values) == 0 {
		return forms
	}
	out := make([]Form, 0, len(forms)*len(values))
	for _, f := range forms {
		for _, v := range values {
			fields := cloneValues(f.Fields)
			fields.Set(key, v)
			l := label + " " + v
			if f.Label != "" {
				l = f.Label + ", " + l
			}
			out = append(out, Form{Label: l, Fields: fields})
		}
	}
	return out
}

// WithPage returns a copy of fields requesting the given result page.
func WithPage(fields url.Values, page int) url.Values {
	out := cloneValues(fields)
	out.Set("api_page", strconv.Itoa(page))
	return out
}

func cloneValues(v url.Values) url.Values {
	out := make(url.Values, len(v))
	for k, vs := range v {
		out[k] = append([]string(nil), vs...)
	}
	return out
}

func setIf(v url.Values, key, value string) {
	if value != "" {
		v.Set(key, value)
	}
}

// ExpandYears parses a year spec such as "2024", "2022-2026", or
// "2019,2022-2024" into a sorted, de-duplicated list of four-digit years.
func ExpandYears(spec string) ([]string, error) {
	seen := map[int]bool{}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		lo, hi := part, part
		if i := strings.Index(part, "-"); i > 0 {
			lo, hi = strings.TrimSpace(part[:i]), strings.TrimSpace(part[i+1:])
		}
		a, errA := parseYear(lo)
		b, errB := parseYear(hi)
		if errA != nil || errB != nil {
			return nil, fmt.Errorf("invalid year %q (use 2024, 2022-2026, or a comma list)", part)
		}
		if a > b {
			a, b = b, a
		}
		if b-a > 50 {
			return nil, fmt.Errorf("year range %q spans more than 50 years", part)
		}
		for y := a; y <= b; y++ {
			seen[y] = true
		}
	}
	years := make([]int, 0, len(seen))
	for y := range seen {
		years = append(years, y)
	}
	sort.Ints(years)
	out := make([]string, len(years))
	for i, y := range years {
		out[i] = strconv.Itoa(y)
	}
	return out, nil
}

func parseYear(s string) (int, error) {
	y, err := strconv.Atoi(s)
	if err != nil || y < 1900 || y > 2100 {
		return 0, fmt.Errorf("bad year")
	}
	return y, nil
}

// SplitList splits a comma-separated flag value into trimmed, non-empty items.
func SplitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
