// Copyright 2026 H179922 and contributors. Licensed under Apache-2.0. See LICENSE.

package ofa

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Row is one Advanced Search result: a single test record for a single dog.
type Row struct {
	AppNum       string `json:"appnum"`
	CHIC         bool   `json:"chic"`
	Name         string `json:"name"`
	Registration string `json:"registration"`
	Breed        string `json:"breed"`
	Sex          string `json:"sex"`
	Color        string `json:"color,omitempty"`
	Birthdate    string `json:"birthdate"`
	TestDate     string `json:"test_date"`
	AgeMonths    int    `json:"age_months"`
	OFANumber    string `json:"ofa_number"`
	Test         string `json:"test"`
	Grade        string `json:"grade,omitempty"`
	Flag         string `json:"flag,omitempty"`
}

// Page is one parsed backend result page.
type Page struct {
	Rows      []Row
	Total     int // "N matches." header; 0 when nothing matched
	Number    int // page number the response says it is
	PageCount int // pages available for this query
}

var (
	headerRe   = regexp.MustCompile(`([\d,]+) matches\.\s+Page (\d+) of <span id='api_page_count'>(\d+)</span>`)
	tagRe      = regexp.MustCompile(`<[^>]*>`)
	spaceRe    = regexp.MustCompile(`\s+`)
	appnumRe   = regexp.MustCompile(`data-appnum='(\d+)'`)
	chicAttrRe = regexp.MustCompile(`data-chic='([^']*)'`)
)

// ResultsPageSize is the number of rows the backend returns on every page
// except the last.
const ResultsPageSize = 20

// ExpectedRows returns how many rows page (1-based) of a result set with the
// given total and page count must carry: a full page everywhere except the
// last, which holds the remainder.
func ExpectedRows(total, page, pageCount int) int {
	if page < pageCount {
		return ResultsPageSize
	}
	return total - ResultsPageSize*(pageCount-1)
}

// ValidatePage checks page (1-based) of a fanned-out query against the totals
// page 1 advertised: later pages must repeat the same match count and page
// count, and every page must carry exactly ExpectedRows rows. A later page
// that answers "No matches found" fails here instead of silently ending the
// scan short.
func ValidatePage(first Page, page int, p Page) error {
	if page > 1 && (p.Total != first.Total || p.PageCount != first.PageCount) {
		return &UnexpectedResponseError{Snippet: fmt.Sprintf("page %d reports %d matches over %d pages; page 1 reported %d over %d", page, p.Total, p.PageCount, first.Total, first.PageCount)}
	}
	if p.Total > 0 && p.Number != page {
		return &UnexpectedResponseError{Snippet: fmt.Sprintf("requested page %d but the response is page %d", page, p.Number)}
	}
	if first.Total > 0 {
		if want := ExpectedRows(first.Total, page, first.PageCount); len(p.Rows) != want {
			return &UnexpectedResponseError{Snippet: fmt.Sprintf("page %d of %d (%d matches) returned %d rows, want %d", page, first.PageCount, first.Total, len(p.Rows), want)}
		}
	}
	return nil
}

// noMatchesMarker is the text the backend returns when a search matches nothing.
const noMatchesMarker = "No matches found"

// ParseResults parses one search response fragment.
func ParseResults(body []byte) (Page, error) {
	s := string(body)
	if strings.Contains(s, noMatchesMarker) {
		return Page{}, nil
	}
	m := headerRe.FindStringSubmatch(s)
	if m == nil {
		return Page{}, &UnexpectedResponseError{Snippet: snippet(s)}
	}
	total, _ := strconv.Atoi(strings.ReplaceAll(m[1], ",", ""))
	number, _ := strconv.Atoi(m[2])
	pages, _ := strconv.Atoi(m[3])
	p := Page{Total: total, Number: number, PageCount: pages}
	// A complete result fragment closes its table after the last row; a body
	// cut inside the final cell (e.g. "<td>HI") would otherwise still count as
	// a full row with a truncated value.
	if last := strings.LastIndex(s, "<tr class='as_results_row'"); last >= 0 && !strings.Contains(s[last:], "</table>") {
		return Page{}, &UnexpectedResponseError{Snippet: "result table does not close after its last row (truncated response)"}
	}
	for _, seg := range strings.Split(s, "<tr class='as_results_row'")[1:] {
		seg = cutAt(seg, "</table>")
		cells := cellTexts(seg)
		if len(cells) < 11 {
			return Page{}, &UnexpectedResponseError{Snippet: fmt.Sprintf("result row with %d cells (want 11): %s", len(cells), snippet(seg))}
		}
		r := Row{
			Name:         cells[1],
			Registration: cells[2],
			Breed:        cells[3],
			Sex:          cells[4],
			Color:        cells[5],
			Birthdate:    NormalizeDate(cells[6]),
			TestDate:     NormalizeDate(cells[7]),
			OFANumber:    cells[9],
			Test:         cells[10],
		}
		r.AgeMonths, _ = strconv.Atoi(cells[8])
		if a := appnumRe.FindStringSubmatch(seg); a != nil {
			r.AppNum = a[1]
		}
		if c := chicAttrRe.FindStringSubmatch(seg); c != nil {
			r.CHIC = c[1] == "cur"
		}
		r.Grade = Grade(r.OFANumber, r.Test)
		r.Flag = Flag(r.OFANumber)
		p.Rows = append(p.Rows, r)
	}
	if p.Total > 0 && len(p.Rows) == 0 {
		return Page{}, &UnexpectedResponseError{Snippet: fmt.Sprintf("header reports %d matches but no result rows parsed", p.Total)}
	}
	return p, nil
}

// cellTexts returns the text of every <td> in a row. Result cells are not
// closed in the backend markup, so a cell runs until the next <td, <tbody,
// or </tr>.
func cellTexts(row string) []string {
	var out []string
	for {
		i := strings.Index(row, "<td")
		if i < 0 {
			return out
		}
		row = row[i:]
		gt := strings.Index(row, ">")
		if gt < 0 {
			return out
		}
		row = row[gt+1:]
		cell := cutAt(row, "<td", "<tbody", "</tr>")
		out = append(out, CleanText(cell))
		row = row[len(cell):]
	}
}

// CleanText strips tags, decodes entities, and collapses whitespace.
func CleanText(s string) string {
	s = tagRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, "​", "")
	s = strings.ReplaceAll(s, " ", " ")
	return strings.TrimSpace(spaceRe.ReplaceAllString(s, " "))
}

// NormalizeDate converts backend dates like "Feb 10 2026" or "Dec  1 2023"
// to ISO 8601. Unparseable values are returned trimmed and unchanged.
func NormalizeDate(s string) string {
	s = strings.TrimSpace(spaceRe.ReplaceAllString(s, " "))
	if t, err := time.Parse("Jan 2 2006", s); err == nil {
		return t.Format("2006-01-02")
	}
	return s
}

// cutAt returns s up to the first occurrence of any stop string.
func cutAt(s string, stops ...string) string {
	end := len(s)
	for _, stop := range stops {
		if i := strings.Index(s, stop); i >= 0 && i < end {
			end = i
		}
	}
	return s[:end]
}

// UnexpectedResponseError means the backend answered with a page that is
// neither a result list nor the no-matches notice (usually a markup change).
type UnexpectedResponseError struct{ Snippet string }

func (e *UnexpectedResponseError) Error() string {
	return "unexpected OFA response (markup may have changed): " + e.Snippet
}

func snippet(s string) string {
	s = CleanText(s)
	if len(s) > 120 {
		s = s[:120] + "..."
	}
	return s
}
