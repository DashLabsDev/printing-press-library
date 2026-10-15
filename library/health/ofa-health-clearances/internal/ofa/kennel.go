// Copyright 2026 H179922 and contributors. Licensed under Apache-2.0. See LICENSE.

package ofa

import (
	"sort"
	"strings"
)

// Kennel is one kennel-prefix rollup over search rows.
type Kennel struct {
	Prefix     string         `json:"prefix"`
	Dogs       int            `json:"dogs"`
	CHICDogs   int            `json:"chic_dogs"`
	Records    int            `json:"records"`
	Tests      []string       `json:"tests"`
	LatestTest string         `json:"latest_test"`
	HipGrades  map[string]int `json:"hip_grades,omitempty"`
	Flagged    int            `json:"flagged_records"`
	SampleDogs []string       `json:"sample_dogs"`
}

// titleTokens are show/performance title prefixes that can lead a registered
// name and are not part of the kennel prefix.
var titleTokens = map[string]bool{
	"CH": true, "GCH": true, "GCHB": true, "GCHS": true, "GCHG": true, "GCHP": true,
	"DC": true, "FC": true, "AFC": true, "MACH": true, "UCH": true, "OTCH": true,
	"HC": true, "BIS": true, "BISS": true, "RBIS": true, "INT": true, "AM": true,
	"CAN": true, "MBIS": true, "MBISS": true, "UGRCH": true, "GRCH": true,
}

// KennelPrefix derives the kennel prefix from a registered name. Registered
// names conventionally begin with the kennel name; OFA exposes no breeder
// field, so this prefix is the only breeder signal.
//
// words > 0 takes exactly that many leading words. words == 0 uses the
// default: skip leading title tokens, take the first word, and extend to two
// words when the first is a number or one or two characters (e.g. "2 BAR").
func KennelPrefix(name string, words int) string {
	fields := strings.Fields(strings.ToUpper(name))
	for len(fields) > 1 && titleTokens[strings.TrimRight(fields[0], ".")] {
		fields = fields[1:]
	}
	if len(fields) == 0 {
		return ""
	}
	n := words
	if n <= 0 {
		n = 1
		if len(fields[0]) <= 2 || isDigits(fields[0]) {
			n = 2
		}
	}
	if n > len(fields) {
		n = len(fields)
	}
	prefix := strings.Join(fields[:n], " ")
	return strings.TrimSuffix(prefix, "'S")
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// Rollup groups rows by kennel prefix and ranks kennels by distinct dogs,
// then CHIC dogs, then most recent test date.
func Rollup(rows []Row, prefixWords int) []Kennel {
	type acc struct {
		k     Kennel
		dogs  map[string]bool
		chic  map[string]bool
		tests map[string]bool
	}
	byPrefix := map[string]*acc{}
	var order []string
	for _, r := range rows {
		p := KennelPrefix(r.Name, prefixWords)
		if p == "" {
			continue
		}
		a, ok := byPrefix[p]
		if !ok {
			a = &acc{k: Kennel{Prefix: p, HipGrades: map[string]int{}}, dogs: map[string]bool{}, chic: map[string]bool{}, tests: map[string]bool{}}
			byPrefix[p] = a
			order = append(order, p)
		}
		dogKey := r.AppNum
		if dogKey == "" {
			dogKey = r.Registration + "|" + r.Name
		}
		if !a.dogs[dogKey] && len(a.k.SampleDogs) < 3 {
			a.k.SampleDogs = append(a.k.SampleDogs, r.Name)
		}
		a.dogs[dogKey] = true
		if r.CHIC {
			a.chic[dogKey] = true
		}
		if r.Test != "" {
			a.tests[r.Test] = true
		}
		a.k.Records++
		if r.TestDate > a.k.LatestTest {
			a.k.LatestTest = r.TestDate
		}
		if r.Grade != "" {
			a.k.HipGrades[r.Grade]++
		}
		if r.Flag != "" {
			a.k.Flagged++
		}
	}
	out := make([]Kennel, 0, len(order))
	for _, p := range order {
		a := byPrefix[p]
		a.k.Dogs, a.k.CHICDogs = len(a.dogs), len(a.chic)
		for t := range a.tests {
			a.k.Tests = append(a.k.Tests, t)
		}
		sort.Strings(a.k.Tests)
		if len(a.k.HipGrades) == 0 {
			a.k.HipGrades = nil
		}
		out = append(out, a.k)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Dogs != out[j].Dogs {
			return out[i].Dogs > out[j].Dogs
		}
		if out[i].CHICDogs != out[j].CHICDogs {
			return out[i].CHICDogs > out[j].CHICDogs
		}
		return out[i].LatestTest > out[j].LatestTest
	})
	return out
}
