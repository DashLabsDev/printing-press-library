// Copyright 2026 Greg Mushen and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type failureGroup struct {
	ErrorCode    string   `json:"error_code"`
	Status       string   `json:"status"`
	Count        int      `json:"count"`
	Counterparts []string `json:"counterparts"`
	LastAt       string   `json:"last_at"`
	LastFrom     string   `json:"last_from"`
}

type failuresView struct {
	Groups         []failureGroup `json:"groups"`
	TotalFailed    int            `json:"total_failed"`
	ScannedRecords int            `json:"scanned_records"`
	Note           string         `json:"note,omitempty"`
}

func newNovelFailuresCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "failures",
		Short: "Failed and undelivered texts grouped by error code and recipient.",
		Long: `Group synced outbound messages that did not arrive (failed, delivery_failed,
sending_failed, delivery_unconfirmed, ...) by carrier error code, listing the
recipients affected and the most recent attempt.

Reads the local mirror; run 'telnyx-pp-cli sync --resources detail-records' first.`,
		Example: strings.Trim(`
  telnyx-pp-cli failures --since 7d --agent
  telnyx-pp-cli failures --since 30d`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
			"pp:happy-args":  "--since=30d",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "failures")
			}
			sinceISO, err := parseSince(flagSince)
			if err != nil {
				return err
			}
			db, ok, err := openMirror(cmd, flags, &dbPath, "detail-records", failuresView{Groups: make([]failureGroup, 0)})
			if !ok || err != nil {
				return err
			}
			defer db.Close()
			msgs, err := loadMDRs(cmd.Context(), db, sinceISO, "")
			if err != nil {
				return err
			}
			groups := map[string]*failureGroup{}
			seen := map[string]map[string]bool{}
			view := failuresView{Groups: make([]failureGroup, 0), ScannedRecords: len(msgs)}
			for _, m := range msgs {
				if m.Direction == "inbound" || !isFailedStatus(m.Status) {
					continue
				}
				view.TotalFailed++
				code := "none"
				if len(m.ErrorCodes) > 0 {
					code = m.ErrorCodes[0]
				}
				key := code + "|" + m.Status
				g := groups[key]
				if g == nil {
					g = &failureGroup{ErrorCode: code, Status: m.Status, Counterparts: []string{}}
					groups[key] = g
					seen[key] = map[string]bool{}
				}
				g.Count++
				if !seen[key][m.Counterpart] {
					seen[key][m.Counterpart] = true
					g.Counterparts = append(g.Counterparts, m.Counterpart)
				}
				if m.CreatedAt >= g.LastAt {
					g.LastAt, g.LastFrom = m.CreatedAt, m.Ours
				}
			}
			for _, g := range groups {
				view.Groups = append(view.Groups, *g)
			}
			if view.ScannedRecords == 0 {
				view.Note = "no synced messaging records in this window; run 'telnyx-pp-cli sync --resources detail-records' if this is unexpected"
			}
			sort.Slice(view.Groups, func(i, j int) bool { return view.Groups[i].Count > view.Groups[j].Count })
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			if view.TotalFailed == 0 {
				fmt.Fprintf(w, "No failed messages among %d synced records in this window.\n", view.ScannedRecords)
				return nil
			}
			for _, g := range view.Groups {
				fmt.Fprintf(w, "error %s (%s): %d message(s) to %s; last %s from %s\n", g.ErrorCode, g.Status, g.Count, strings.Join(g.Counterparts, ", "), g.LastAt, g.LastFrom)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "", "Only messages newer than this window (e.g. 7d, 30d)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path (default: the CLI's synced store)")
	return cmd
}
