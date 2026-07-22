// Helpers that bridge generated Cobra commands to the hand-rolled icaroclient.
// Each <archivio>_cerca.go / <archivio>_get.go file delegates here so the
// search-engine logic lives in one place.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/cliutil"
	icaro "github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/icaroclient"
	"github.com/spf13/cobra"
)

// cercaParams collects the cleaned param map the icaroclient expects, plus
// the few search-time tunables the CLI exposes.
type cercaParams struct {
	Params   map[string]string
	ISISRaw  string
	Limit    int
	MaxPages int
}

// runCerca executes a search against an archive and emits JSON or table-shaped
// output according to flags. archiveSlug names one of the entries in
// internal/icaroclient/archives.go (e.g. "leggi", "ddl").
func runCerca(cmd *cobra.Command, flags *rootFlags, archiveSlug string, p cercaParams) error {
	arc := icaro.BySlug(archiveSlug)
	if arc == nil {
		return fmt.Errorf("unknown archive slug: %q", archiveSlug)
	}
	// --escludi is registered on every search command; read it centrally so the
	// exclusion (ISIS NOT) is applied uniformly without per-command plumbing.
	if cmd.Flags().Lookup("escludi") != nil {
		if esc, _ := cmd.Flags().GetString("escludi"); strings.TrimSpace(esc) != "" {
			if p.Params == nil {
				p.Params = map[string]string{}
			}
			p.Params["escludi"] = esc
		}
	}
	if flags.dryRun {
		return emitDryRun(cmd, *arc, p)
	}
	if cliIsVerify() {
		return emitDryRun(cmd, *arc, p)
	}
	// Default MaxPages: if Limit is set and small, one page is enough; if
	// caller asked for >50, fan out multiple pages (Icaro paginates ~10/pg).
	maxPages := p.MaxPages
	if maxPages == 0 {
		if p.Limit > 10 {
			maxPages = (p.Limit + 9) / 10
		} else {
			maxPages = 1
		}
	}
	c, err := icaro.New(nil)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	recs, err := c.Search(ctx, *arc, icaro.SearchOptions{
		Params:   normalizeParams(*arc, p.Params),
		ISISRaw:  p.ISISRaw,
		Limit:    p.Limit,
		MaxPages: maxPages,
	})
	if err != nil {
		if rlErr := new(icaro.HTTPRateLimitError); errors.As(err, &rlErr) {
			return rateLimitErr(fmt.Errorf("ricerca %s: %w", arc.Slug, err))
		}
		return fmt.Errorf("ricerca %s: %w", arc.Slug, err)
	}
	var firm map[int][]firmatario
	if conFirmatariRequested(cmd) {
		if _, ok := arc.FieldMap["firmatario"]; ok {
			firm = firmatariByDoc(ctx, c, *arc, recs)
		}
	}
	return emitRecords(cmd, flags, *arc, recs, firm)
}

// conFirmatariRequested reports whether --con-firmatari was set. Like
// --escludi, the flag is registered per-command and read centrally here so
// the behavior stays uniform without per-command plumbing.
func conFirmatariRequested(cmd *cobra.Command) bool {
	if cmd.Flags().Lookup("con-firmatari") == nil {
		return false
	}
	v, _ := cmd.Flags().GetBool("con-firmatari")
	return v
}

// firmatariByDoc opens each record's document and parses its full signatory
// list. The portal's short list carries only the FIRST signatory (the column
// is literally "Titolo e Primo Firmatario") or none at all for ddl, so a
// --firmatario hit cannot be verified from the list alone. Resolving that
// costs one extra request per row — paced by the client's rate limiter —
// which is why this is opt-in behind --con-firmatari rather than always on.
// A document that fails to load is skipped: one bad row must not sink the
// whole result set.
func firmatariByDoc(ctx context.Context, c *icaro.Client, arc icaro.Archive, recs []icaro.Record) map[int][]firmatario {
	out := make(map[int][]firmatario, len(recs))
	for _, r := range recs {
		doc, err := c.GetDoc(ctx, arc, r.DocID)
		if err != nil {
			continue
		}
		if f := docFirmatari(doc); len(f) > 0 {
			out[r.DocID] = f
		}
	}
	return out
}

// runGet fetches and emits a single document. Get needs a fresh session, so
// we Search first with a narrow query that pins the record, then GetDoc on
// the returned docID. For the typical case where the caller passes legisl
// and numero, the query is `<legisl>.LEGISL E <numero>.<KEY>` where KEY is
// the archive-specific id field.
func runGet(cmd *cobra.Command, flags *rootFlags, archiveSlug string, legisl, numero int) error {
	return runGetExtra(cmd, flags, archiveSlug, legisl, numero, nil)
}

// runGetExtra is runGet with additional pinning params (e.g. --anno) so callers
// can disambiguate records that share legisl+numero (leggi reuse a number per
// year). The extra keys are translated through the archive FieldMap like any
// other criterion.
func runGetExtra(cmd *cobra.Command, flags *rootFlags, archiveSlug string, legisl, numero int, extra map[string]string) error {
	arc := icaro.BySlug(archiveSlug)
	if arc == nil {
		return fmt.Errorf("unknown archive slug: %q", archiveSlug)
	}
	if flags.dryRun || cliIsVerify() {
		out := map[string]any{
			"archive": arc.Slug,
			"legisl":  legisl,
			"numero":  numero,
			"dry_run": true,
			"would_fetch": fmt.Sprintf("%s/icaro/doc%s-1.jsp?icaDocId=N&legisl=%d&numero=%d",
				icaro.DefaultBaseURL, arc.ID, legisl, numero),
		}
		return writeJSON(cmd.OutOrStdout(), out)
	}
	params := map[string]string{}
	if legisl > 0 {
		params["legisl"] = fmt.Sprintf("%d", legisl)
	}
	if numero > 0 {
		params["numero"] = fmt.Sprintf("%d", numero)
	}
	for k, v := range extra {
		if v = strings.TrimSpace(v); v != "" {
			params[k] = v
		}
	}
	c, err := icaro.New(nil)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	recs, err := c.Search(ctx, *arc, icaro.SearchOptions{
		Params: normalizeParams(*arc, params),
		Limit:  1,
	})
	if err != nil {
		if rlErr := new(icaro.HTTPRateLimitError); errors.As(err, &rlErr) {
			return rateLimitErr(fmt.Errorf("locating document: %w", err))
		}
		return fmt.Errorf("locating document: %w", err)
	}
	if len(recs) == 0 {
		return fmt.Errorf("nessun documento trovato per legisl=%d numero=%d in %s", legisl, numero, arc.Slug)
	}
	doc, err := c.GetDoc(ctx, *arc, recs[0].DocID)
	if err != nil {
		return err
	}
	// Merge the short-list fields into the doc so callers see legisl, atto, etc.
	for k, v := range recs[0].Fields {
		if _, exists := doc.Fields[k]; !exists {
			doc.Fields[k] = v
		}
	}
	if recs[0].Excerpt != "" && doc.Body == "" {
		doc.Body = recs[0].Excerpt
	}
	// For every archive with signatories (same gate as --con-firmatari in
	// runCerca: ddl, interrogazioni, interpellanze, mozioni, odg, risoluzioni
	// all share the FIRMAT field and the portal's "Nome (Gruppo)." doc
	// format), surface them as a structured field instead of leaving the
	// caller to parse fields.Firmatari by hand. Was ddl-only: interrogazioni
	// get et al. left the raw string in place, forcing a second parsing path
	// for any consumer iterating across atto types.
	if _, ok := arc.FieldMap["firmatario"]; ok {
		if firm := docFirmatari(doc); len(firm) > 0 {
			return printJSONFiltered(cmd.OutOrStdout(), struct {
				icaro.Doc
				Firmatari []firmatario `json:"firmatari"`
			}{doc, firm}, flags)
		}
	}
	// printJSONFiltered (not the bare writeJSON) so --select/--compact/--csv
	// behave the same as on generator-emitted commands — writeJSON always
	// dumped the full payload regardless of --select.
	return printJSONFiltered(cmd.OutOrStdout(), doc, flags)
}

// normalizeParams rewrites a few flag inputs to the shape the portal expects:
//   - dates given as YYYY-MM-DD become AAMMGG (the 6-digit numeric form the
//     ISIS date fields store, e.g. DATPRE/DATSED); a range YYYY-MM-DD:YYYY-MM-DD
//     becomes AAMMGG/AAMMGG (ISIS interval syntax)
//   - on ddl, a bare --anno year becomes a DATPRE Jan-1..Dec-31 AAMMGG range
//     (ddl has no year field to qualify a plain year against)
//   - a numeric commission code (--codcom 1..6) is rerouted to the COMMIS field
//     as its Roman ordinal name, since the upstream CODCOM field is not indexed
//   - whitespace is trimmed
func normalizeParams(arc icaro.Archive, in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		switch {
		case k == "data":
			v = toISISDate(v)
		case k == "anno" && arc.Slug == "ddl":
			// ddl has no year field (unlike leggi.LEGANN/resoconti.ANNSED):
			// --anno is qualified on DATPRE (presentation date) as a
			// Jan-1..Dec-31 range. See archives.go's ddl.FieldMap["anno"].
			v = yearToISISRange(v)
		}
		out[k] = v
	}
	// --codcom 1..6 has no working upstream field; reroute to COMMIS by name.
	if code, ok := out["codcom"]; ok {
		if name := commissioneOrdinale(code); name != "" {
			delete(out, "codcom")
			if out["commissione"] == "" {
				out["commissione"] = name
			}
		}
	}
	return out
}

// toISISDate converts a date (or a date range) to the 6-digit AAMMGG form the
// ISIS engine stores. Accepts YYYY-MM-DD and ranges as YYYY-MM-DD:YYYY-MM-DD.
// Values it cannot parse are returned unchanged (so already-AAMMGG input or
// raw expressions still pass through).
func toISISDate(v string) string {
	// Emit a range only when BOTH bounds convert to a valid AAMMGG date.
	// Otherwise (empty, non-numeric, or one-sided bound) fall through to the
	// single-value path so we never produce a malformed "260225/" or
	// "260225/garbage" range expression.
	if lo, hi, isRange := strings.Cut(v, ":"); isRange {
		loC, hiC := aammgg(lo), aammgg(hi)
		if isAAMMGG(loC) && isAAMMGG(hiC) {
			return loC + "/" + hiC
		}
		return aammgg(v)
	}
	return aammgg(v)
}

// yearToISISRange converts a bare 4-digit year to a DATPRE-style AAMMGG range
// spanning the whole year (Jan 1 to Dec 31), e.g. "2024" -> "240101/241231".
// Anything else (already-AAMMGG input, a range from --isis-query, garbage)
// passes through unchanged rather than producing a malformed expression.
func yearToISISRange(v string) string {
	if len(v) != 4 || !isDigits(v) {
		return v
	}
	yy := v[2:]
	return yy + "0101/" + yy + "1231"
}

func aammgg(v string) string {
	v = strings.TrimSpace(v)
	iso := strings.SplitN(v, "-", 3)
	if len(iso) == 3 && len(iso[0]) == 4 && len(iso[1]) == 2 && len(iso[2]) == 2 {
		// Verify all components are numeric before accepting as a date, so the
		// documented pass-through guarantee holds for non-date input.
		if isDigits(iso[0]) && isDigits(iso[1]) && isDigits(iso[2]) {
			return iso[0][2:] + iso[1] + iso[2]
		}
	}
	return v
}

// isAAMMGG reports whether s is the 6-digit numeric date form the ISIS engine
// stores (e.g. "260225").
func isAAMMGG(s string) bool { return len(s) == 6 && isDigits(s) }

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// commissioneOrdinale maps a numeric commission code to its Roman ordinal name
// as stored in the COMMIS field. Returns "" for unknown codes.
func commissioneOrdinale(code string) string {
	switch strings.TrimSpace(code) {
	case "1":
		return "PRIMA"
	case "2":
		return "SECONDA"
	case "3":
		return "TERZA"
	case "4":
		return "QUARTA"
	case "5":
		return "QUINTA"
	case "6":
		return "SESTA"
	}
	return ""
}

// emitDryRun prints the would-be query without hitting the network, useful
// for --dry-run flows and Printing Press verify checks.
func emitDryRun(cmd *cobra.Command, arc icaro.Archive, p cercaParams) error {
	expr := icaro.BuildQuery(arc, normalizeParams(arc, p.Params), p.ISISRaw)
	out := map[string]any{
		"archive":     arc.Slug,
		"archive_id":  arc.ID,
		"isis_query":  expr,
		"would_fetch": fmt.Sprintf("%s/icaro/default.jsp?icaDB=%s&icaQuery=%s", icaro.DefaultBaseURL, arc.ID, expr),
		"dry_run":     true,
	}
	return writeJSON(cmd.OutOrStdout(), out)
}

// emitRecords prints search records honoring --json/--csv/table formats.
// When the user did not pass --json explicitly and stdout is a TTY, we
// produce a small table; otherwise we default to JSON for pipe friendliness.
// firmatari, when non-nil, carries the full signatory list per doc ID (see
// firmatariByDoc); a nil map simply leaves the field out.
func emitRecords(cmd *cobra.Command, flags *rootFlags, arc icaro.Archive, recs []icaro.Record, firmatari map[int][]firmatario) error {
	out := cmd.OutOrStdout()
	asJSON := flags.asJSON || (!isTerminal(out) && !flags.csv && !flags.quiet && !flags.plain)
	if asJSON {
		// Convert to a flat shape: {doc_id, title, excerpt, url, <fields...>}.
		flat := make([]map[string]any, 0, len(recs))
		for _, r := range recs {
			row := map[string]any{
				"doc_id":  r.DocID,
				"title":   r.Title,
				"excerpt": r.Excerpt,
				"url":     r.URL,
			}
			for k, v := range r.Fields {
				row[strings.ToLower(strings.TrimSuffix(k, "."))] = v
			}
			if f, ok := firmatari[r.DocID]; ok {
				row["firmatari"] = f
			}
			flat = append(flat, row)
		}
		// printJSONFiltered (not the bare writeJSON) so --select/--compact
		// behave the same as on generator-emitted commands — writeJSON
		// always dumped the full array regardless of --select.
		return printJSONFiltered(out, flat, flags)
	}
	if flags.csv {
		return writeRecordsCSV(out, arc, recs, firmatari)
	}
	// Table view (default for TTY).
	if len(recs) == 0 {
		fmt.Fprintln(out, "Nessun risultato.")
		return nil
	}
	for _, r := range recs {
		fmt.Fprintf(out, "#%d  %s\n", r.DocID, r.Title)
		for i, col := range arc.Columns {
			if i == len(arc.Columns)-1 {
				continue // last col is the title block, already printed
			}
			if v, ok := r.Fields[col]; ok {
				fmt.Fprintf(out, "  %-10s %s\n", col, v)
			}
		}
		if r.Excerpt != "" {
			fmt.Fprintf(out, "  %s\n", r.Excerpt)
		}
		if f, ok := firmatari[r.DocID]; ok {
			fmt.Fprintf(out, "  %-10s %s\n", "Firmatari", firmatariLine(f))
		}
		fmt.Fprintln(out)
	}
	return nil
}

// firmatariLine renders a signatory list as "Nome (Gruppo); Nome (Gruppo)"
// for the flat table and CSV views.
func firmatariLine(f []firmatario) string {
	parts := make([]string, 0, len(f))
	for _, x := range f {
		if x.Gruppo != "" {
			parts = append(parts, x.Nome+" ("+x.Gruppo+")")
			continue
		}
		parts = append(parts, x.Nome)
	}
	return strings.Join(parts, "; ")
}

func writeRecordsCSV(out io.Writer, arc icaro.Archive, recs []icaro.Record, firmatari map[int][]firmatario) error {
	// Header. Unnamed columns are portal placeholders (see Archive.Columns)
	// and get no CSV column of their own.
	cols := make([]string, 0, len(arc.Columns))
	for _, c := range arc.Columns {
		if c != "" {
			cols = append(cols, c)
		}
	}
	hdr := []string{"doc_id", "title", "excerpt", "url"}
	for _, c := range cols {
		hdr = append(hdr, strings.ToLower(strings.TrimSuffix(c, ".")))
	}
	if firmatari != nil {
		hdr = append(hdr, "firmatari")
	}
	for i, h := range hdr {
		if i > 0 {
			fmt.Fprint(out, ",")
		}
		fmt.Fprint(out, csvEscape(h))
	}
	fmt.Fprintln(out)
	for _, r := range recs {
		row := []string{fmt.Sprintf("%d", r.DocID), r.Title, r.Excerpt, r.URL}
		for _, c := range cols {
			row = append(row, r.Fields[c])
		}
		if firmatari != nil {
			row = append(row, firmatariLine(firmatari[r.DocID]))
		}
		for i, v := range row {
			if i > 0 {
				fmt.Fprint(out, ",")
			}
			fmt.Fprint(out, csvEscape(v))
		}
		fmt.Fprintln(out)
	}
	return nil
}

func csvEscape(s string) string {
	if strings.ContainsAny(s, ",\"\n\r") {
		return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
	}
	return s
}

func writeJSON(out io.Writer, v any) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// cliIsVerify mirrors cliutil.IsVerifyEnv so callers can short-circuit
// outbound network calls during Printing Press verify runs.
func cliIsVerify() bool {
	return cliutil.IsVerifyEnv()
}

// itoa is a tiny shorthand so cerca-wrapper commands don't need strconv.
func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}

// atoiArg parses a positional CLI argument as an int, returning a
// human-friendly Italian error when the input is malformed.
func atoiArg(s, name string) (int, error) {
	var n int
	if _, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &n); err != nil {
		return 0, fmt.Errorf("argomento %q non valido (atteso numero intero): %s", name, s)
	}
	return n, nil
}
