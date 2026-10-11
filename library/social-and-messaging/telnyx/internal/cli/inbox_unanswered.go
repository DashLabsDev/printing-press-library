// Copyright 2026 Greg Mushen and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type unansweredRow struct {
	Counterpart    string `json:"counterpart"`
	OurNumber      string `json:"our_number"`
	LastInboundAt  string `json:"last_inbound_at"`
	WaitingHours   int    `json:"waiting_hours"`
	InboundInRow   int    `json:"inbound_in_row"`
	LastOutboundAt string `json:"last_outbound_at,omitempty"`
}

func newNovelInboxUnansweredCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "unanswered",
		Short: "Contacts whose latest text was inbound with no reply yet.",
		Long: `List counterpart numbers whose most recent synced message was inbound,
with no later outbound reply from any of your numbers. Longest-waiting first.

Reads the local mirror; run 'telnyx-pp-cli sync --resources detail-records' first.`,
		Example: strings.Trim(`
  telnyx-pp-cli inbox unanswered --since 48h --agent
  telnyx-pp-cli inbox unanswered --since 7d --limit 10`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
			"pp:happy-args":  "--since=7d",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "inbox unanswered")
			}
			sinceISO, err := parseSince(flagSince)
			if err != nil {
				return err
			}
			db, ok, err := openMirror(cmd, flags, &dbPath, "detail-records", make([]unansweredRow, 0))
			if !ok || err != nil {
				return err
			}
			defer db.Close()
			msgs, err := loadMDRs(cmd.Context(), db, sinceISO, "")
			if err != nil {
				return err
			}
			// msgs are oldest-first; walk forward keeping per-counterpart state.
			type state struct {
				row     unansweredRow
				waiting bool
			}
			byNum := map[string]*state{}
			for _, m := range msgs {
				if m.Counterpart == "" {
					continue
				}
				st := byNum[m.Counterpart]
				if st == nil {
					st = &state{row: unansweredRow{Counterpart: m.Counterpart}}
					byNum[m.Counterpart] = st
				}
				if m.Direction == "inbound" {
					if !st.waiting {
						st.row.InboundInRow = 0
					}
					st.waiting = true
					st.row.InboundInRow++
					st.row.LastInboundAt = m.CreatedAt
					st.row.OurNumber = m.Ours
				} else {
					st.waiting = false
					st.row.LastOutboundAt = m.CreatedAt
				}
			}
			rows := make([]unansweredRow, 0)
			now := time.Now().UTC()
			for _, st := range byNum {
				if !st.waiting {
					continue
				}
				if t, err := time.Parse(time.RFC3339, st.row.LastInboundAt); err == nil {
					st.row.WaitingHours = int(now.Sub(t).Hours())
				}
				rows = append(rows, st.row)
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].LastInboundAt < rows[j].LastInboundAt })
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			w := cmd.OutOrStdout()
			if len(rows) == 0 {
				fmt.Fprintln(w, "No unanswered threads in this window.")
				return nil
			}
			for _, r := range rows {
				fmt.Fprintf(w, "%s  waiting %dh  (%d unanswered, to %s, last at %s)\n", r.Counterpart, r.WaitingHours, r.InboundInRow, r.OurNumber, r.LastInboundAt)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "", "Only consider messages newer than this window (e.g. 48h, 7d)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum threads to return (0 = all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path (default: the CLI's synced store)")
	return cmd
}
