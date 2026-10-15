// Copyright 2026 H179922 and contributors. Licensed under Apache-2.0. See LICENSE.

package ofa

import (
	"regexp"
	"strings"
)

// hipNumberRe matches OFA hip numbers such as "AUK-50E24M-C-NOPI": breed code,
// sequence digits, a grade letter (E/G/F), age in months, and sex.
var hipNumberRe = regexp.MustCompile(`^[A-Z0-9]+-\d+([EGF])\d+[MF]`)

var gradeNames = map[string]string{"E": "EXCELLENT", "G": "GOOD", "F": "FAIR"}

// Grade decodes the hip grade embedded in an OFA hip number. It returns ""
// for non-hip tests and for numbers it cannot decode; the summary result
// rows carry no conclusion column, so this is the only grade signal there.
func Grade(ofaNumber, test string) string {
	if !strings.EqualFold(strings.TrimSpace(test), "HIPS") {
		return ""
	}
	m := hipNumberRe.FindStringSubmatch(strings.ToUpper(strings.TrimSpace(ofaNumber)))
	if m == nil {
		return ""
	}
	return gradeNames[m[1]]
}

// Flag returns a heuristic attention flag for a summary row:
//
//	"no-number" — OFA issues numbers only for passing results, so a record
//	              with a blank OFA number usually means an abnormal finding.
//	"carrier"   — DNA numbers carrying the -CAR suffix.
//
// Clear DNA results (including MDR1 normal/normal) and every numbered
// phenotypic result return "". Confirm with 'dog <appnum>', which shows the
// published conclusion.
func Flag(ofaNumber string) string {
	n := strings.ToUpper(strings.TrimSpace(ofaNumber))
	switch {
	case n == "":
		return "no-number"
	case strings.Contains(n, "-CAR"):
		return "carrier"
	default:
		return ""
	}
}
