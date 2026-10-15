// Copyright 2026 H179922 and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/health/ofa-health-clearances/internal/client"
	"github.com/mvanhorn/printing-press-library/library/health/ofa-health-clearances/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/health/ofa-health-clearances/internal/ofa"
)

// ofaCourtesyRate is the default request ceiling (requests/second) for OFA's
// backend. OFA is a nonprofit registry; --rate-limit overrides it explicitly.
const ofaCourtesyRate = 1.0

// newOFAClient returns the generated client with the OFA courtesy rate limit
// applied unless the user set --rate-limit explicitly.
func newOFAClient(flags *rootFlags) (*client.Client, error) {
	if flags.rateLimit == client.RateLimitAuto {
		flags.rateLimit = ofaCourtesyRate
	}
	return flags.newClient()
}

// healthKeepFields are health signals that --agent/--compact must never drop
// as "sparse": an expired exam or a carrier flag on one row out of twenty is
// exactly the record an agent needs to see.
var healthKeepFields = []string{"expired", "flag", "grade", "hip_grades", "chic", "chic_dogs", "flagged_records", "conclusion"}

// queryFlags are the search filters shared by 'results' and 'kennels'.
type queryFlags struct {
	breeds, tests, reportYears, testYears, birthYears string
	bornFrom, bornThru, sex, registrar, name          string
	regnum, ofanum                                    string
	ageMax                                            int
	chic, nameAny                                     bool
	maxPages                                          int
}

func addQueryFlags(cmd *cobra.Command, q *queryFlags) {
	f := cmd.Flags()
	f.StringVar(&q.breeds, "breed", "", "Breed code(s) or names, comma-separated (e.g. AUK or 'australian koolie'); see 'breeds'")
	f.StringVar(&q.tests, "test", "", "Registry code(s) or names, comma-separated (e.g. HD,EL or hips); see 'tests'")
	f.StringVar(&q.reportYears, "report-year", "", "Report year(s): 2026, 2022-2026, or a comma list (one backend query per year)")
	f.StringVar(&q.testYears, "test-year", "", "Test year(s): 2026, 2022-2026, or a comma list (one backend query per year)")
	f.StringVar(&q.birthYears, "birth-year", "", "Birth year(s): 2024, 2021-2024, or a comma list (one backend query per year)")
	f.StringVar(&q.bornFrom, "born-from", "", "Earliest birth year, e.g. 2021 (dogs born that year or later)")
	f.StringVar(&q.bornThru, "born-thru", "", "Latest birth year, e.g. 2024")
	f.IntVar(&q.ageMax, "age-max", 0, "Only dogs born within this many years of today (sets --born-from)")
	f.BoolVar(&q.chic, "chic", false, "Only CHIC-certified dogs")
	f.StringVar(&q.sex, "sex", "", "M or F")
	f.StringVar(&q.registrar, "registrar", "", "AKC, CKC, or UKC")
	f.StringVar(&q.name, "name", "", "Registered name prefix, usually the kennel name (e.g. AVALON)")
	f.BoolVar(&q.nameAny, "name-any-part", false, "Match --name anywhere in the registered name (slower on OFA's side)")
	f.StringVar(&q.regnum, "regnum", "", "Registration number, e.g. DN12345678")
	f.StringVar(&q.ofanum, "ofanum", "", "OFA number, e.g. AUK-50E24M-C-NOPI")
	f.IntVar(&q.maxPages, "max-scan-pages", 10, "Maximum result pages fetched per backend query (20 rows each)")
}

// toQuery validates the flags and builds an ofa.Query.
func (q *queryFlags) toQuery() (ofa.Query, error) {
	out := ofa.Query{CHIC: q.chic, Sex: q.sex, Registrar: q.registrar, Name: q.name,
		NameAnyPart: q.nameAny, RegNum: q.regnum, OFANum: q.ofanum, BornFrom: q.bornFrom, BornThru: q.bornThru}
	for _, b := range ofa.SplitList(q.breeds) {
		code, err := ofa.ResolveBreed(b)
		if err != nil {
			return out, usageErr(err)
		}
		out.Breeds = append(out.Breeds, code)
	}
	for _, t := range ofa.SplitList(q.tests) {
		code, err := ofa.ResolveTest(t)
		if err != nil {
			return out, usageErr(err)
		}
		out.Tests = append(out.Tests, code)
	}
	var err error
	if out.ReportYears, err = expandYearFlag("report-year", q.reportYears); err != nil {
		return out, err
	}
	if out.TestYears, err = expandYearFlag("test-year", q.testYears); err != nil {
		return out, err
	}
	if out.BirthYears, err = expandYearFlag("birth-year", q.birthYears); err != nil {
		return out, err
	}
	if q.ageMax > 0 {
		out.BornFrom = strconv.Itoa(time.Now().Year() - q.ageMax)
	}
	if s := strings.ToUpper(q.sex); s != "" && s != "M" && s != "F" {
		return out, usageErr(fmt.Errorf("--sex must be M or F"))
	}
	if q.maxPages < 1 {
		return out, usageErr(fmt.Errorf("--max-scan-pages must be at least 1"))
	}
	if out.Empty() {
		return out, usageErr(fmt.Errorf("at least one filter is required (e.g. --breed AUK --report-year 2026)"))
	}
	return out, nil
}

func expandYearFlag(name, v string) ([]string, error) {
	if v == "" {
		return nil, nil
	}
	ys, err := ofa.ExpandYears(v)
	if err != nil {
		return nil, usageErr(fmt.Errorf("--%s: %w", name, err))
	}
	return ys, nil
}

// fetchMeta reports how much of the backend result set a fetch examined.
type fetchMeta struct {
	Queries      int  `json:"backend_queries"`
	ScannedPages int  `json:"scanned_pages"`
	MaxScanPages int  `json:"max_scan_pages"`
	TotalMatches int  `json:"total_matches"`
	Truncated    bool `json:"truncated"`
}

// fetchRows runs every fanned-out form of q, following pages up to maxPages
// per form, and returns the parsed rows in backend order.
func fetchRows(ctx context.Context, c *client.Client, q ofa.Query, maxPages int, progress io.Writer) ([]ofa.Row, fetchMeta, error) {
	if cliutil.IsDogfoodEnv() && maxPages > 1 {
		maxPages = 1
	}
	meta := fetchMeta{MaxScanPages: maxPages}
	rows := []ofa.Row{}
	for _, form := range q.Forms() {
		meta.Queries++
		var first ofa.Page
		for page := 1; page <= maxPages; page++ {
			body, status, err := c.PostQueryFormWithParams(ctx, ofa.SearchPath, nil, ofa.WithPage(form.Fields, page))
			if err != nil {
				return rows, meta, classifyAPIErrorOnly(fmt.Errorf("OFA search (%s page %d): %w", labelOr(form.Label), page, err))
			}
			if status >= 400 {
				return rows, meta, classifyAPIErrorOnly(fmt.Errorf("OFA search (%s page %d): HTTP %d", labelOr(form.Label), page, status))
			}
			p, err := ofa.ParseResults(body)
			if err != nil {
				return rows, meta, apiErr(err)
			}
			if page == 1 {
				first = p
			}
			if err := ofa.ValidatePage(first, page, p); err != nil {
				return rows, meta, apiErr(fmt.Errorf("OFA search (%s): %w", labelOr(form.Label), err))
			}
			meta.ScannedPages++
			if page == 1 {
				meta.TotalMatches += p.Total
			}
			rows = append(rows, p.Rows...)
			if page >= first.PageCount {
				break
			}
			if page == maxPages {
				meta.Truncated = true
				fmt.Fprintf(progress, "warning: %s has %d pages; stopped at --max-scan-pages %d\n", labelOr(form.Label), p.PageCount, maxPages)
			}
		}
	}
	return rows, meta, nil
}

func labelOr(l string) string {
	if l == "" {
		return "query"
	}
	return l
}

// toMaps converts a slice of structs to the generic rows printAutoTable takes.
func toMaps(v any) []map[string]any {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var out []map[string]any
	_ = json.Unmarshal(b, &out)
	return out
}
