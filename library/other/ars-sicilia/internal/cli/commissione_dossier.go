// pp:data-source live
// pp:client-call
// Novel feature — dossier 360° di una commissione: convocazioni, sommari,
// pareri al governo e DDL assegnati raggruppati per codice commissione.

package cli

import (
	"context"
	"fmt"
	"strings"

	icaro "github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/icaroclient"
	"github.com/spf13/cobra"
)

func newNovelCommissioneDossierCmd(flags *rootFlags) *cobra.Command {
	var (
		flagLegisl int
		flagLimit  int
	)
	cmd := &cobra.Command{
		Use:     "dossier <codcom-o-nome>",
		Short:   "Vista completa di una commissione: convocazioni, sommari, pareri al Governo e DDL assegnati.",
		Example: "  ars-sicilia-pp-cli commissione dossier 5 --legisl 18 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			arg := strings.TrimSpace(strings.Join(args, " "))
			return runCommissioneDossier(cmd, flags, arg, flagLegisl, flagLimit)
		},
	}
	cmd.Flags().IntVar(&flagLegisl, "legisl", 0, "Legislatura (es. 18).")
	cmd.Flags().IntVar(&flagLimit, "limit", 30, "Max risultati per sezione.")
	return cmd
}

type dossierSection struct {
	Tipo      string           `json:"tipo"`
	Archivio  string           `json:"archivio"`
	Risultati []map[string]any `json:"risultati"`
}

type dossierReport struct {
	Commissione string         `json:"commissione"`
	Legisl      int            `json:"legisl,omitempty"`
	Conteggio   map[string]int `json:"conteggio"`
	// Troncato lists the section labels where Conteggio is a --limit cap,
	// not the true total: the portal had more matching records than were
	// fetched. Re-run with a higher --limit to see the rest.
	Troncato []string         `json:"troncato,omitempty"`
	Sezioni  []dossierSection `json:"sezioni"`
}

func runCommissioneDossier(cmd *cobra.Command, flags *rootFlags, arg string, legisl, perSection int) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	if perSection <= 0 {
		perSection = 30
	}
	report := dossierReport{
		Commissione: arg,
		Legisl:      legisl,
		Conteggio:   map[string]int{},
	}

	// Detect whether arg is a numeric codice commissione or a name string.
	codeKey, nameKey := "codcom", "commissione"
	isNumeric := true
	for _, r := range arg {
		if r < '0' || r > '9' {
			isNumeric = false
			break
		}
	}

	section := func(slug, label, paramKey string) {
		arc := icaro.BySlug(slug)
		if arc == nil {
			return
		}
		c, err := icaro.New(nil)
		if err != nil {
			return
		}
		params := map[string]string{paramKey: arg}
		if legisl > 0 {
			params["legisl"] = itoa(legisl)
		}
		var truncated bool
		recs, err := c.Search(ctx, *arc, icaro.SearchOptions{
			Params:    params,
			Limit:     perSection,
			MaxPages:  maxInt(1, (perSection+9)/10),
			Truncated: &truncated,
		})
		if err != nil {
			return
		}
		s := dossierSection{Tipo: label, Archivio: arc.ID}
		for _, r := range recs {
			s.Risultati = append(s.Risultati, map[string]any{
				"doc_id":  r.DocID,
				"data":    r.Fields["Data"],
				"numero":  r.Fields["Numero"],
				"titolo":  r.Title,
				"excerpt": r.Excerpt,
				"url":     r.URL,
			})
		}
		report.Sezioni = append(report.Sezioni, s)
		report.Conteggio[label] = len(s.Risultati)
		if truncated {
			report.Troncato = append(report.Troncato, label)
		}
	}

	pickKey := nameKey
	if isNumeric {
		pickKey = codeKey
	}
	section("convocazioni", "convocazioni", pickKey)
	section("sommari", "sommari", pickKey)
	section("pareri", "pareri", "commissione")
	if !isNumeric {
		section("ddl", "ddl_assegnati", "testo")
	}

	total := 0
	for _, v := range report.Conteggio {
		total += v
	}
	if total == 0 {
		return fmt.Errorf("nessuna commissione trovata per: %q", arg)
	}

	out := cmd.OutOrStdout()
	if flags.asJSON || !isTerminal(out) {
		return printJSONFiltered(out, report, flags)
	}
	fmt.Fprintf(out, "Commissione: %s\n", report.Commissione)
	if report.Legisl > 0 {
		fmt.Fprintf(out, "Legislatura: %d\n\n", report.Legisl)
	}
	troncato := map[string]bool{}
	for _, label := range report.Troncato {
		troncato[label] = true
	}
	for _, s := range report.Sezioni {
		suffix := ""
		if troncato[s.Tipo] {
			suffix = " (troncato, aumenta --limit)"
		}
		fmt.Fprintf(out, "[%s] %d risultati%s\n", s.Tipo, len(s.Risultati), suffix)
		for _, r := range s.Risultati {
			fmt.Fprintf(out, "  #%v  %v  %v\n", r["doc_id"], r["data"], r["titolo"])
		}
		fmt.Fprintln(out)
	}
	return nil
}
