// Copyright 2026 Greg Mushen and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type conversationView struct {
	Counterpart string   `json:"counterpart"`
	Messages    []mdrRow `json:"messages"`
	Inbound     int      `json:"inbound"`
	Outbound    int      `json:"outbound"`
	Failed      int      `json:"failed"`
	TotalCost   float64  `json:"total_cost"`
	Note        string   `json:"note,omitempty"`
}

func newNovelConversationCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "conversation [number]",
		Short: "See every text to and from one guest or vendor number in order, with delivery status.",
		Long: `Show the synced message history with one counterpart number, oldest first:
direction, which of your numbers was used, delivery status, parts and cost.

Telnyx detail records carry delivery metadata only, not message bodies.
Reads the local mirror; run 'telnyx-pp-cli sync --resources detail-records' first.`,
		Example: strings.Trim(`
  telnyx-pp-cli conversation +12075551234 --since 30d --agent
  telnyx-pp-cli conversation 2075551234 --limit 20`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
			"pp:happy-args":  "number=+12075551234;--since=30d",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "conversation")
			}
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a counterpart phone number is required, e.g. conversation +12075551234"))
			}
			number := normalizePhone(args[0])
			if len(number) < 8 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("%q is not a phone number; pass E.164 like +12075551234 or a 10-digit US number", args[0]))
			}
			sinceISO, err := parseSince(flagSince)
			if err != nil {
				return err
			}
			db, ok, err := openMirror(cmd, flags, &dbPath, "detail-records", conversationView{Counterpart: number, Messages: make([]mdrRow, 0), Note: "no local mirror yet; run sync"})
			if !ok || err != nil {
				return err
			}
			defer db.Close()
			msgs, err := loadMDRs(cmd.Context(), db, sinceISO, number)
			if err != nil {
				return err
			}
			if limit > 0 && len(msgs) > limit {
				msgs = msgs[len(msgs)-limit:]
			}
			view := conversationView{Counterpart: number, Messages: msgs}
			for _, m := range msgs {
				if m.Direction == "inbound" {
					view.Inbound++
				} else {
					view.Outbound++
				}
				if isFailedStatus(m.Status) {
					view.Failed++
				}
				view.TotalCost += m.Cost
			}
			view.TotalCost = round4(view.TotalCost)
			if len(msgs) == 0 {
				view.Note = fmt.Sprintf("no synced messages with %s in this window; widen --since or run sync", number)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			if len(msgs) == 0 {
				fmt.Fprintln(w, view.Note)
				return nil
			}
			for _, m := range msgs {
				arrow := "→"
				if m.Direction == "inbound" {
					arrow = "←"
				}
				fmt.Fprintf(w, "%s  %s %s via %s  %-10s %d part(s)  $%.4f\n", m.CreatedAt, arrow, number, m.Ours, m.Status, m.Parts, m.Cost)
			}
			fmt.Fprintf(w, "\n%d in, %d out, %d failed, $%.4f total\n", view.Inbound, view.Outbound, view.Failed, view.TotalCost)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "", "Only messages newer than this window (e.g. 48h, 7d, 30d)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Show only the most recent N messages (0 = all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path (default: the CLI's synced store)")
	return cmd
}
