// Copyright 2026 H179922 and contributors. Licensed under Apache-2.0. See LICENSE.

package ofa

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Dog is one dog's published OFA record.
type Dog struct {
	AppNum       string       `json:"appnum"`
	Name         string       `json:"name"`
	Registration string       `json:"registration"`
	Sex          string       `json:"sex"`
	Color        string       `json:"color,omitempty"`
	Breed        string       `json:"breed"`
	Born         string       `json:"born,omitempty"`
	CHIC         bool         `json:"chic"`
	Sire         string       `json:"sire_registration,omitempty"`
	Dam          string       `json:"dam_registration,omitempty"`
	Tests        []TestResult `json:"tests"`
	Relatives    []Relative   `json:"relatives,omitempty"`
}

// TestResult is one registry conclusion on a dog's detail record.
type TestResult struct {
	Registry   string `json:"registry"`
	TestDate   string `json:"test_date"`
	ReportDate string `json:"report_date"`
	AgeMonths  int    `json:"age_months"`
	Conclusion string `json:"conclusion"`
	OFANumber  string `json:"ofa_number"`
	Expired    bool   `json:"expired,omitempty"` // OFA marks lapsed time-limited results (e.g. eye exams)
}

// Relative is a related dog listed on a detail record (littermates and other
// relatives OFA links). Sire and dam appear by registration number on Dog.
type Relative struct {
	AppNum       string `json:"appnum"`
	Name         string `json:"name"`
	Sex          string `json:"sex"`
	Relation     string `json:"relation"`
	Registration string `json:"registration"`
	Birthdate    string `json:"birthdate"`
	CHIC         bool   `json:"chic"`
}

// ErrDogNotFound means the backend returned no detail record for the appnum.
var ErrDogNotFound = errors.New("no OFA record for that application number")

var (
	detailNameRe  = regexp.MustCompile(`<span id='as_detail_name'>(.*?)</span>`)
	detailCHICRe  = regexp.MustCompile(`<div class='as_chic' data-chic='([^']*)'`)
	sireRe        = regexp.MustCompile(`Sire:\s*([^<]+?)\s*<br`)
	damRe         = regexp.MustCompile(`Dam:\s*([^<]+?)\s*<br`)
	tdRe          = regexp.MustCompile(`(?s)<td[^>]*>(.*?)</td>`)
	relativeRowRe = regexp.MustCompile(`(?s)<tr class='as_relatives_row'(.*?)</tr>`)
	htmlCommentRe = regexp.MustCompile(`(?s)<!--.*?-->`)
	apiKeyRe      = regexp.MustCompile(`name='api_key' value='(\d*)'`)
)

// ParseDog parses a dog detail page. appnum is echoed into the result.
func ParseDog(body []byte, appnum string) (Dog, error) {
	s := string(body)
	name := detailNameRe.FindStringSubmatch(s)
	if name == nil {
		// Only the rendered detail shell without a name is a missing record.
		// Empty bodies, backend SQL errors (OFA's per-IP lookup throttle
		// answers some appnums with a raw SQL Server error), and unknown
		// pages are unexpected responses, not "not found".
		if strings.Contains(s, "id='as_detail_full'") {
			return Dog{}, ErrDogNotFound
		}
		return Dog{}, &UnexpectedResponseError{Snippet: snippet(s)}
	}
	// The response names its own dog in the hidden api_key input. Trust that,
	// not the request, so a misrouted or replayed page can't be attributed to
	// the wrong appnum.
	if k := apiKeyRe.FindStringSubmatch(s); k == nil || k[1] != appnum {
		got := "missing"
		if k != nil {
			got = k[1]
		}
		return Dog{}, &UnexpectedResponseError{Snippet: fmt.Sprintf("response is for appnum %s, requested %s", got, appnum)}
	}
	d := Dog{AppNum: appnum, Name: CleanText(name[1])}
	if c := detailCHICRe.FindStringSubmatch(s); c != nil {
		d.CHIC = c[1] == "cur"
	}
	parseIndividual(s, &d)
	if m := sireRe.FindStringSubmatch(s); m != nil {
		d.Sire = CleanText(m[1])
	}
	if m := damRe.FindStringSubmatch(s); m != nil {
		d.Dam = CleanText(m[1])
	}
	tests, err := parseDetailTests(s)
	if err != nil {
		return Dog{}, err
	}
	d.Tests = tests
	rel, err := parseRelatives(s)
	if err != nil {
		return Dog{}, err
	}
	d.Relatives = rel
	return d, nil
}

// parseIndividual reads the identity block: registration, "SEX [COLOR] BREED",
// and "Born <date>" lines, each a <td colspan='3'> cell.
func parseIndividual(s string, d *Dog) {
	start := strings.Index(s, "id='as_detail_individual'")
	if start < 0 {
		return
	}
	block := cutAt(s[start:], "</table>")
	// The live markup carries a commented-out "<!--<tr><td colspan='3'>-->".
	block = htmlCommentRe.ReplaceAllString(block, "")
	var lines []string
	for _, part := range strings.Split(block, "<td colspan='3'>")[1:] {
		if t := CleanText(cutAt(part, "<tr", "<td", "</table")); t != "" {
			lines = append(lines, t)
		}
	}
	for i, line := range lines {
		switch {
		case i == 0 && line == d.Name:
			continue
		case strings.HasPrefix(line, "Born "):
			d.Born = NormalizeDate(strings.TrimPrefix(line, "Born "))
		case strings.HasPrefix(line, "Sire:"), strings.HasPrefix(line, "Dam:"):
		case d.Registration == "":
			d.Registration = line
		case d.Breed == "":
			d.Sex, d.Color, d.Breed = splitSexColorBreed(line)
		}
	}
}

// splitSexColorBreed splits "M BLACK AUSTRALIAN SHEPHERD" using the embedded
// breed table to find where the color ends and the breed begins.
func splitSexColorBreed(line string) (sex, color, breed string) {
	fields := strings.Fields(line)
	if len(fields) > 0 && (fields[0] == "M" || fields[0] == "F") {
		sex, fields = fields[0], fields[1:]
	}
	rest := strings.Join(fields, " ")
	for i := range fields {
		candidate := strings.Join(fields[i:], " ")
		if BreedByName(candidate) != nil {
			return sex, strings.Join(fields[:i], " "), candidate
		}
	}
	return sex, "", rest
}

// parseDetailTests reads the registry table. Every data row must be closed
// and carry exactly six cells; a malformed row or an unterminated table is an
// error rather than a silently shorter list of conclusions.
// A complete detail response always carries the tests table followed by the
// as_detail_tools2 toolbar, so a body missing either (or a table that does not
// close before the toolbar) was cut off and must not read as "no tests".
func parseDetailTests(s string) ([]TestResult, error) {
	start := strings.Index(s, "id='as_detail_tests'")
	toolbar := strings.Index(s, "id='as_detail_tools2'")
	if start < 0 || toolbar < start {
		return nil, &UnexpectedResponseError{Snippet: "incomplete detail response (tests table or trailing toolbar missing)"}
	}
	section := s[start:toolbar]
	if !strings.Contains(section, "</table>") {
		return nil, &UnexpectedResponseError{Snippet: "unterminated test table"}
	}
	out := []TestResult{}
	// Rows end at <tfoot>; the footer carries unclosed notes such as
	// "* Eye Certification is valid for one year from the date of the exam."
	table := cutAt(section, "</table>", "<tfoot")
	for _, seg := range strings.Split(table, "<tr")[1:] {
		if !strings.Contains(seg, "<td") {
			continue // header row (<th> cells)
		}
		end := strings.Index(seg, "</tr>")
		if end < 0 {
			return nil, &UnexpectedResponseError{Snippet: "unterminated test row: " + snippet(seg)}
		}
		cells := tdRe.FindAllStringSubmatch(seg[:end], -1)
		if len(cells) != 6 {
			return nil, &UnexpectedResponseError{Snippet: fmt.Sprintf("test row with %d cells (want 6): %s", len(cells), snippet(seg[:end]))}
		}
		age, _ := strconv.Atoi(CleanText(cells[3][1]))
		out = append(out, TestResult{
			Registry:   CleanText(cells[0][1]),
			TestDate:   NormalizeDate(CleanText(cells[1][1])),
			ReportDate: NormalizeDate(CleanText(cells[2][1])),
			AgeMonths:  age,
			Conclusion: CleanText(cells[4][1]),
			OFANumber:  CleanText(cells[5][1]),
			Expired:    strings.Contains(seg[:strings.Index(seg, ">")], "data-expired='Y'"),
		})
	}
	return out, nil
}

// parseRelatives reads the relatives table. Every relatives row that opens
// must close; a cut-off table is an error rather than a shorter list.
func parseRelatives(s string) ([]Relative, error) {
	var out []Relative
	start := strings.Index(s, "id='as_detail_relatives'")
	if start < 0 {
		return nil, &UnexpectedResponseError{Snippet: "incomplete detail response (relatives table missing)"}
	}
	end := strings.Index(s[start:], "</table>")
	if end < 0 {
		return nil, &UnexpectedResponseError{Snippet: "relatives table does not close"}
	}
	// Parse only inside the relatives table. A row opened after the table's
	// first </table> means the table closed early (inside a row).
	table := s[start : start+end]
	if strings.Count(s[start:], "<tr class='as_relatives_row'") != strings.Count(table, "<tr class='as_relatives_row'") {
		return nil, &UnexpectedResponseError{Snippet: "relatives table closes before its last row"}
	}
	matches := relativeRowRe.FindAllStringSubmatch(table, -1)
	if opened := strings.Count(table, "<tr class='as_relatives_row'"); opened != len(matches) {
		return nil, &UnexpectedResponseError{Snippet: fmt.Sprintf("relatives table cut off: %d rows opened, %d closed", opened, len(matches))}
	}
	for _, m := range matches {
		cells := cellTexts(m[1])
		if len(cells) < 6 {
			return nil, &UnexpectedResponseError{Snippet: fmt.Sprintf("relatives row with %d cells (want at least 6)", len(cells))}
		}
		r := Relative{
			Name:         cells[1],
			Sex:          cells[2],
			Relation:     cells[3],
			Registration: cells[4],
			Birthdate:    NormalizeDate(cells[5]),
		}
		if a := appnumRe.FindStringSubmatch(m[1]); a != nil {
			r.AppNum = a[1]
		}
		if c := chicAttrRe.FindStringSubmatch(m[1]); c != nil {
			r.CHIC = c[1] == "cur"
		}
		out = append(out, r)
	}
	return out, nil
}
