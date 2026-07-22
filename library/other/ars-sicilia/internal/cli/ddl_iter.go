// pp:data-source live
// pp:client-call
// Novel feature — ricostruisce la cronologia completa di un DDL.
// Combina ricerche su più archivi (DDL 221, sommari commissione 230,
// resoconti d'aula 217) usando direttamente l'icaroclient.

package cli

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	icaro "github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/icaroclient"
	"github.com/spf13/cobra"
)

func newNovelDdlIterCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "iter <legisl> <numero>",
		Short:   "Ricostruisce la cronologia completa di un disegno di legge: presentazione, commissione, aula, eventuale legge.",
		Example: "  ars-sicilia-pp-cli ddl iter 18 1500 --json",
		Args:    cobra.MaximumNArgs(2),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				if dryRunOK(flags) || cliIsVerify() {
					return cmd.Help()
				}
				return usageErr(fmt.Errorf("richiesti 2 argomenti: <legisl> e <numero>"))
			}
			legisl, err := atoiArg(args[0], "legisl")
			if err != nil {
				return err
			}
			numero, err := atoiArg(args[1], "numero")
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				return emitDdlIterDryRun(cmd, legisl, numero)
			}
			return runDdlIter(cmd, flags, legisl, numero)
		},
	}
	return cmd
}

type iterEvent struct {
	Fase      string `json:"fase"`
	Data      string `json:"data,omitempty"`
	Sede      string `json:"sede,omitempty"`
	Titolo    string `json:"titolo,omitempty"`
	Oratori   string `json:"oratori,omitempty"`
	URL       string `json:"url,omitempty"`
	ArchiveID string `json:"archive_id,omitempty"`
	DocID     int    `json:"doc_id,omitempty"`
}

type iterReport struct {
	Legisl int         `json:"legisl"`
	Numero int         `json:"numero"`
	Titolo string      `json:"titolo,omitempty"`
	Eventi []iterEvent `json:"eventi"`
	Note   string      `json:"note,omitempty"`
}

func runDdlIter(cmd *cobra.Command, flags *rootFlags, legisl, numero int) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	c, err := icaro.New(nil)
	if err != nil {
		return err
	}
	report := iterReport{Legisl: legisl, Numero: numero, Eventi: []iterEvent{}}

	// 1. DDL stesso (archivio 221): presentazione + apertura del documento per
	// leggere l'iter reale (sezione "Attuale … Storico" nel corpo del doc).
	arc := icaro.BySlug("ddl")
	if arc == nil {
		report.Note = "archivio ddl non disponibile"
		return emitIter(cmd, flags, report)
	}
	recs, err := c.Search(ctx, *arc, icaro.SearchOptions{
		Params: map[string]string{"legisl": itoa(legisl), "numero": itoa(numero)},
		Limit:  1,
	})
	if err != nil {
		return fmt.Errorf("ricerca ddl: %w", err)
	}
	if len(recs) == 0 {
		report.Note = fmt.Sprintf("DDL %d non trovato nell'archivio della legislatura %d. Verifica legisl e numero con `ars-sicilia-pp-cli ddl cerca`.", numero, legisl)
		return emitIter(cmd, flags, report)
	}
	report.Titolo = recs[0].Title
	report.Eventi = append(report.Eventi, iterEvent{
		Fase:      "presentazione",
		Data:      recs[0].Fields["Data"],
		Sede:      "Assemblea (presentazione DDL)",
		Titolo:    recs[0].Title,
		URL:       recs[0].URL,
		ArchiveID: arc.ID,
		DocID:     recs[0].DocID,
	})

	// 2. Iter reale: preferisce il blocco HTML etichettato "Iter" che il
	// portale rende separato dal testo del disegno di legge (doc.Fields),
	// con fallback sul corpo del documento solo se quel campo manca. Vedi
	// docIterEvents.
	if doc, derr := c.GetDoc(ctx, *arc, recs[0].DocID); derr == nil {
		for _, ev := range docIterEvents(doc) {
			ev.URL = recs[0].URL
			ev.ArchiveID = arc.ID
			ev.DocID = recs[0].DocID
			report.Eventi = append(report.Eventi, ev)
		}
	}

	// Sort events by ISO date when parseable.
	sort.SliceStable(report.Eventi, func(i, j int) bool {
		return iterDateKey(report.Eventi[i].Data) < iterDateKey(report.Eventi[j].Data)
	})
	return emitIter(cmd, flags, report)
}

// emitDdlIterDryRun previews the ddl iter request without sending it. Unlike
// the silent no-op this used to be, it mirrors the ISIS-query preview that
// `*/cerca` commands already show — ddl iter's --dry-run should be as useful
// a diagnostic as the rest of the CLI.
func emitDdlIterDryRun(cmd *cobra.Command, legisl, numero int) error {
	arc := icaro.BySlug("ddl")
	if arc == nil {
		return fmt.Errorf("archivio ddl non disponibile")
	}
	expr := icaro.BuildQuery(*arc, map[string]string{"legisl": itoa(legisl), "numero": itoa(numero)}, "")
	out := map[string]any{
		"archive":     arc.Slug,
		"archive_id":  arc.ID,
		"isis_query":  expr,
		"would_fetch": fmt.Sprintf("%s/icaro/default.jsp?icaDB=%s&icaQuery=%s", icaro.DefaultBaseURL, arc.ID, expr),
		"note":        "pins the DDL via this query, then fetches its document body to parse the iter timeline",
		"dry_run":     true,
	}
	return writeJSON(cmd.OutOrStdout(), out)
}

func emitIter(cmd *cobra.Command, flags *rootFlags, report iterReport) error {
	out := cmd.OutOrStdout()
	if flags.asJSON || !isTerminal(out) {
		return printJSONFiltered(out, report, flags)
	}
	fmt.Fprintf(out, "DDL %d/%d — %s\n", report.Legisl, report.Numero, report.Titolo)
	if report.Note != "" {
		fmt.Fprintf(out, "  %s\n", report.Note)
	}
	for _, e := range report.Eventi {
		fmt.Fprintf(out, "  [%s] %s — %s\n", e.Fase, e.Data, strings.TrimSpace(e.Sede+" "+e.Titolo))
	}
	return nil
}

// reIterDate matches an Italian short date "<DD> <mese> <YYYY>" used to anchor
// each iter step in the document status block.
var reIterDate = regexp.MustCompile(`(\d{1,2})\s+([a-zàèéìòù]{3,})\s+(\d{4})`)

// reLrAnnotation matches the portal's raw law-registration annotation
// ("Lr <giorno> <mese> alr <anno> nlr <numero> Titolo : ...") that appears as
// a DDL's own iter event once it is promulgated. Everything after "Titolo :"
// just repeats the bill's title — sometimes mangled with runs of stray quote
// characters, a portal rendering quirk — and duplicates information already
// carried by year+numero, so it is dropped in favor of a short, correctly
// classified event (see its use in parseIterFromBody).
var reLrAnnotation = regexp.MustCompile(`(?i)^Lr\s+\d{1,2}\s+\S+\s+alr\s+(\d{4})\s+nlr\s+(\d+)\b`)

var itaMonths = map[string]string{
	"gen": "01", "feb": "02", "mar": "03", "apr": "04", "mag": "05", "giu": "06",
	"lug": "07", "ago": "08", "set": "09", "ott": "10", "nov": "11", "dic": "12",
}

// parseIterFromBody extracts iter events (committee assignment, aula passage,
// approval/rejection, promulgation) from the status block the portal renders at
// the top of a DDL document body. The block runs from "Attuale" up to the bill
// header "(n. …)" and contains both the current status and, after the "Storico"
// label, the full chronological history. Each step is "<date> <action> [Seduta
// n. N …]"; we cut the action at "Seduta" to drop the sitting metadata (and its
// stray digits). The raw "Lr ... alr ... nlr ..." law-registration annotation
// (see reLrAnnotation) is reduced to a short, correctly classified event.
// Returns nil when no status block is present.
func parseIterFromBody(body string) []iterEvent {
	if body == "" {
		return nil
	}
	start := strings.Index(body, "Attuale")
	if start < 0 {
		return nil
	}
	region := body[start+len("Attuale"):]
	// The bill text proper begins either with the "(n. <numero>)" header or,
	// for records lacking that header (e.g. the finanziaria and other
	// governativi that open straight into the masthead), with the fixed
	// "ASSEMBLEA REGIONALE SICILIANA" line. Cut the region at whichever comes
	// first; everything after it is document content, not iter. Without this,
	// long articolati leak into the region and dates cited inside the bill
	// text (e.g. "3 luglio 1950, n. 51") get parsed as iter events.
	cutAt := -1
	for _, marker := range []string{"(n.", "ASSEMBLEA REGIONALE SICILIANA"} {
		if i := strings.Index(region, marker); i >= 0 && (cutAt < 0 || i < cutAt) {
			cutAt = i
		}
	}
	if cutAt >= 0 {
		region = region[:cutAt]
	}
	// "Storico" is a section label between current status and history, not an event.
	region = strings.ReplaceAll(region, "Storico", " ")

	locs := reIterDate.FindAllStringIndex(region, -1)
	subs := reIterDate.FindAllStringSubmatch(region, -1)
	var events []iterEvent
	for i, loc := range locs {
		dd, mon, yyyy := subs[i][1], strings.ToLower(subs[i][2]), subs[i][3]
		actEnd := len(region)
		if i+1 < len(locs) {
			actEnd = locs[i+1][0]
		}
		action := region[loc[1]:actEnd]
		if s := strings.Index(action, "Seduta"); s >= 0 {
			action = action[:s]
		}
		action = strings.Join(strings.Fields(action), " ")
		if action == "" {
			continue
		}
		if m := reLrAnnotation.FindStringSubmatch(action); m != nil {
			action = fmt.Sprintf("Promulgata legge regionale n. %s/%s", m[2], m[1])
		}
		events = append(events, iterEvent{
			Fase:   classifyIterFase(action),
			Data:   fmt.Sprintf("%s %s %s", dd, mon, yyyy),
			Sede:   iterSede(action),
			Titolo: action,
		})
	}
	return events
}

// docIterEvents reads a DDL's iter timeline. It prefers the page's labeled
// "Iter" block, which the portal renders in a div separate from the bill
// text and contains nothing but the "Attuale ... Storico ..." status steps,
// and only falls back to scanning the flattened Body when that block is
// absent. The distinction matters: DDLs whose relazione/articolato quotes
// dates from the law they amend (e.g. "l.r. 8 aprile 2010, n. 9" repeated
// throughout) have no reliable end-of-status marker in Body, so those quoted
// dates used to leak in as spurious iter events. The labeled field has no
// such text to leak from, and is not truncated even for long iters (verified
// on a 40-event, 2.7kB history).
func docIterEvents(doc icaro.Doc) []iterEvent {
	if s := doc.Fields["Iter"]; s != "" {
		if ev := parseIterFromBody(s); len(ev) > 0 {
			return ev
		}
	}
	return parseIterFromBody(doc.Body)
}

// currentIterState returns the DDL's current iter status as a single stable
// string for `sync --deep` to persist as `iter` — the field `ddl drift` compares
// across snapshots. It prefers the page's labeled "Iter" block (the same source
// docIterEvents trusts first) and falls back to the flattened Body.
func currentIterState(doc icaro.Doc) string {
	if s := doc.Fields["Iter"]; s != "" {
		if c := currentIterFromBody(s); c != "" {
			return c
		}
	}
	return currentIterFromBody(doc.Body)
}

// currentIterFromBody extracts the collapsed current-status segment the portal
// renders between the "Attuale" marker and the "Storico" history label (or the
// bill header when there is no history yet). Returns "" when no status block is
// present. Mirrors parseIterFromBody's region boundaries so the two stay in sync.
func currentIterFromBody(body string) string {
	start := strings.Index(body, "Attuale")
	if start < 0 {
		return ""
	}
	region := body[start+len("Attuale"):]
	if end := strings.Index(region, "Storico"); end >= 0 {
		region = region[:end]
	} else {
		// No history yet: cut at whichever bill-header marker comes first.
		cutAt := -1
		for _, marker := range []string{"(n.", "ASSEMBLEA REGIONALE SICILIANA"} {
			if i := strings.Index(region, marker); i >= 0 && (cutAt < 0 || i < cutAt) {
				cutAt = i
			}
		}
		if cutAt >= 0 {
			region = region[:cutAt]
		}
	}
	return strings.Join(strings.Fields(region), " ")
}

type firmatario struct {
	Nome   string `json:"nome"`
	Gruppo string `json:"gruppo,omitempty"`
}

// reFirmEntry matches a "Nome Cognome (Gruppo)" firmatario entry. The name is a
// run of capitalised words; the group is the parenthesised text.
var reFirmEntry = regexp.MustCompile(`([A-ZÀ-Ù][\p{L}'.\-]*(?:\s+[A-ZÀ-Ù][\p{L}'.\-]*){0,3})\s*\(([^)]{2,90})\)`)

// firmLabelWords are non-name capitalised words that can sit right before a
// firmatario in the flattened body (section labels / iniziativa values).
var firmLabelWords = map[string]bool{
	"Parlamentare": true, "Governativa": true, "Popolare": true, "Iniziativa": true,
	"Gruppo": true, "Firmatari": true, "Argomenti": true, "Premier": true, "ARS": true,
}

// docFirmatari reads the signatories of a document. It prefers the page's
// labeled "Firmatari" block, which contains nothing but signatories, and only
// falls back to scanning the whole flattened Body when that block is absent.
// The distinction matters: in Body the neighbouring "Gruppo Parlamentare"
// block runs straight into the first name ("Partito Democratico Chinnici
// Valentina"), and the bullet-segment heuristics drop signatories that share
// a segment.
func docFirmatari(doc icaro.Doc) []firmatario {
	if s := doc.Fields["Firmatari"]; s != "" {
		if f := parseFirmatariBlock(s); len(f) > 0 {
			return f
		}
	}
	return parseDdlFirmatari(doc.Body)
}

// firmatariNames joins signatory names into the comma-separated string form that
// `sync --deep` persists as `$.firmatari` — the shape splitFirmatari re-parses in
// analytics cofirmatari. Only names are kept (party groups dropped) so the split
// yields clean deputy names. Returns "" for no signatories.
func firmatariNames(fs []firmatario) string {
	names := make([]string, 0, len(fs))
	for _, f := range fs {
		if n := strings.TrimSpace(f.Nome); n != "" {
			names = append(names, n)
		}
	}
	return strings.Join(names, ", ")
}

// reFirmTrunc matches a trailing signatory whose group parenthesis the portal
// left unclosed — it truncates the Firmatari field at a fixed width, so the
// last entry can arrive as "Galluzzo Giuseppe (Fratelli d'Italia".
var reFirmTrunc = regexp.MustCompile(`([A-ZÀ-Ù][\p{L}'.\-]*(?:\s+[A-ZÀ-Ù][\p{L}'.\-]*){0,3})\s*\(([^)]*)$`)

// parseFirmatariBlock parses the page's "Firmatari" block: a run of
// "Nome Cognome (Gruppo)" entries separated by bullets, <br> or nothing at
// all. Because the block holds only signatories, every match is taken
// verbatim — no label stripping, no per-segment guessing.
func parseFirmatariBlock(s string) []firmatario {
	var out []firmatario
	seen := map[string]bool{}
	add := func(nome, grp string) {
		nome = strings.Join(strings.Fields(nome), " ")
		grp = strings.Join(strings.Fields(grp), " ")
		if nome == "" || seen[nome+"|"+grp] {
			return
		}
		seen[nome+"|"+grp] = true
		out = append(out, firmatario{Nome: nome, Gruppo: grp})
	}
	locs := reFirmEntry.FindAllStringSubmatchIndex(s, -1)
	for _, l := range locs {
		add(s[l[2]:l[3]], s[l[4]:l[5]])
	}
	// Recover a truncated trailing entry rather than dropping the signatory:
	// the name is intact even when the portal cut its group short.
	tail := s
	if len(locs) > 0 {
		tail = s[locs[len(locs)-1][1]:]
	}
	if m := reFirmTrunc.FindStringSubmatch(tail); m != nil {
		add(m[1], m[2])
	}
	return out
}

// parseDdlFirmatari extracts the bill signatories from the document body. It
// handles the structured sidebar form ("Nome (Gruppo). • Nome (Gruppo).", with
// party groups) and the relazione form ("presentato dai deputati: A, B, C",
// names only). Returns nil when none is found (e.g. some governativi).
// Prefer docFirmatari, which reads the labeled block when the page has one.
func parseDdlFirmatari(body string) []firmatario {
	if body == "" {
		return nil
	}
	// Form A: bullet-separated "Nome (Gruppo)" entries.
	if strings.Contains(body, "•") {
		var out []firmatario
		seen := map[string]bool{}
		for _, seg := range strings.Split(body, "•") {
			ms := reFirmEntry.FindAllStringSubmatch(seg, -1)
			if len(ms) == 0 {
				continue
			}
			m := ms[len(ms)-1] // firmatario sits at the end of the segment
			nome := cleanFirmatarioName(m[1])
			grp := strings.Join(strings.Fields(m[2]), " ")
			if nome == "" || seen[nome+"|"+grp] {
				continue
			}
			seen[nome+"|"+grp] = true
			out = append(out, firmatario{Nome: nome, Gruppo: grp})
		}
		if len(out) > 0 {
			return out
		}
	}
	// Form B: "presentato dai deputati: A, B, C" (no groups).
	for _, marker := range []string{"presentato dai deputati", "presentato dal deputato", "presentato dalla deputata"} {
		if i := strings.Index(strings.ToLower(body), marker); i >= 0 {
			rest := body[i+len(marker):]
			rest = strings.TrimLeft(rest, ": ")
			if e := strings.IndexAny(rest, ".\n"); e >= 0 {
				rest = rest[:e]
			}
			var out []firmatario
			for _, n := range strings.Split(rest, ",") {
				if n = strings.Join(strings.Fields(n), " "); n != "" {
					out = append(out, firmatario{Nome: n})
				}
			}
			if len(out) > 0 {
				return out
			}
		}
	}
	return nil
}

// cleanFirmatarioName strips leading section-label words and caps the name to
// its trailing capitalised tokens.
func cleanFirmatarioName(s string) string {
	toks := strings.Fields(s)
	for len(toks) > 0 && firmLabelWords[toks[0]] {
		toks = toks[1:]
	}
	if len(toks) > 4 {
		toks = toks[len(toks)-4:]
	}
	return strings.Join(toks, " ")
}

func classifyIterFase(action string) string {
	a := strings.ToLower(action)
	switch {
	case strings.Contains(a, "promulgat") || strings.Contains(a, "legge regionale") || strings.Contains(a, "l.r."):
		return "legge"
	case strings.Contains(a, "aula") || strings.Contains(a, "assemblea") || strings.Contains(a, "approvat"):
		return "aula"
	case strings.Contains(a, "commissione") || strings.Contains(a, "esame") || strings.Contains(a, "parere"):
		return "commissione"
	default:
		return "iter"
	}
}

// iterSede returns the committee name when the action references one.
func iterSede(action string) string {
	if i := strings.Index(strings.ToLower(action), "commissione"); i >= 0 {
		return strings.TrimSpace(action[i:])
	}
	return ""
}

// iterDateKey returns a sortable "YYYY-MM-DD" key for both the short-list date
// form (DD.MM.YY) and the document status form ("DD mese YYYY").
func iterDateKey(s string) string {
	s = strings.TrimSpace(s)
	if strings.Contains(s, ".") {
		return parseICaroDate(s)
	}
	parts := strings.Fields(s)
	if len(parts) == 3 {
		if mm, ok := itaMonths[strings.ToLower(parts[1])[:min3(len(parts[1]))]]; ok {
			dd := parts[0]
			if len(dd) == 1 {
				dd = "0" + dd
			}
			return parts[2] + "-" + mm + "-" + dd
		}
	}
	return s
}

func min3(n int) int {
	if n < 3 {
		return n
	}
	return 3
}

// parseICaroDate converts DD.MM.YYYY (or DD.MM.YY) into a sortable string
// "YYYY-MM-DD"; returns the input as-is when the format isn't recognized.
func parseICaroDate(s string) string {
	parts := strings.Split(strings.TrimSpace(s), ".")
	if len(parts) != 3 {
		return s
	}
	dd, mm, yy := parts[0], parts[1], parts[2]
	if len(yy) == 2 {
		// Crude century pivot — atti precedenti al 2000 sono rari nel sito.
		if yy[0] >= '0' && yy[0] <= '4' {
			yy = "20" + yy
		} else {
			yy = "19" + yy
		}
	}
	if len(yy) != 4 || len(mm) > 2 || len(dd) > 2 {
		return s
	}
	if len(mm) == 1 {
		mm = "0" + mm
	}
	if len(dd) == 1 {
		dd = "0" + dd
	}
	return yy + "-" + mm + "-" + dd
}
