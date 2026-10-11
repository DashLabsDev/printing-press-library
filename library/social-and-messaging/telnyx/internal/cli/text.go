// Copyright 2026 Greg Mushen and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/telnyx/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/telnyx/internal/store"
)

type sendPreflight struct {
	From      string    `json:"from"`
	To        string    `json:"to"`
	Text      string    `json:"text"`
	Segments  int       `json:"estimated_segments"`
	Sender    *auditRow `json:"sender,omitempty"`
	Warnings  []string  `json:"warnings"`
	Recent    []mdrRow  `json:"recent_history"`
	Sent      bool      `json:"sent"`
	MessageID string    `json:"message_id,omitempty"`
	Status    string    `json:"status,omitempty"`
	Note      string    `json:"note,omitempty"`
}

// smsSegments estimates billable parts: GSM-7 160/153, UCS-2 70/67.
func smsSegments(text string) int {
	n := len([]rune(text))
	single, multi := 160, 153
	for _, r := range text {
		if r > 127 {
			single, multi = 70, 67
			break
		}
	}
	if n <= single {
		return 1
	}
	return (n + multi - 1) / multi
}

func newNovelTextCmd(flags *rootFlags) *cobra.Command {
	var flagFrom, flagTo, flagText, dbPath string
	var yes, sendAnyway bool

	cmd := &cobra.Command{
		Use:   "text",
		Short: "Send a text after checking the sender is wired correctly and showing recent history; requires --yes.",
		Long: `Use this command for an agent-safe send with preflight checks and recent history. Do NOT use it for raw API sends; use 'messages send' instead.

Before sending, checks the --from number against the local number audit
(messaging profile, 10DLC campaign) and shows the last 3 synced messages with
the recipient. Without --yes it only prints the preflight and sends nothing.
With --yes it still refuses (exit 2) when the sender fails preflight, unless
--send-anyway is also set.
Refuses to send under printing-press verify/dogfood harnesses.`,
		Example: strings.Trim(`
  telnyx-pp-cli text --from +12075550100 --to +12075551234 --message 'Your door code is ready' --dry-run
  telnyx-pp-cli text --from +12075550100 --to +12075551234 --message 'Checking in' --yes`, "\n"),
		Annotations: map[string]string{
			"pp:data-source":        "live",
			"pp:preview-happy-path": "true",
			"pp:happy-args":         "--from=+12075550100;--to=+12075551234;--message=preview only",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "text")
			}
			if flagFrom == "" || flagTo == "" || strings.TrimSpace(flagText) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--from, --to and --message are all required"))
			}
			blocking := false
			pf := sendPreflight{
				From:     normalizePhone(flagFrom),
				To:       normalizePhone(flagTo),
				Text:     flagText,
				Segments: smsSegments(flagText),
				Warnings: []string{},
				Recent:   make([]mdrRow, 0),
			}
			if dbPath == "" {
				dbPath = defaultDBPath("telnyx-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); statErr == nil {
				db, err := store.OpenWithContext(cmd.Context(), dbPath)
				if err != nil {
					pf.Warnings = append(pf.Warnings, "preflight check failed: "+err.Error())
				} else {
					if rows, err := buildNumberAudit(cmd.Context(), db); err != nil {
						pf.Warnings = append(pf.Warnings, "preflight check failed: "+err.Error())
					} else {
						for i := range rows {
							if rows[i].PhoneNumber == pf.From {
								r := rows[i]
								pf.Sender = &r
								pf.Warnings = append(pf.Warnings, r.Problems...)
							}
						}
						if pf.Sender == nil && len(rows) > 0 {
							pf.Warnings = append(pf.Warnings, "--from is not one of your synced numbers")
							blocking = true
						}
						if pf.Sender != nil && len(pf.Sender.Problems) > 0 {
							blocking = true
						}
					}
					if msgs, err := loadMDRs(cmd.Context(), db, "", pf.To); err != nil {
						pf.Warnings = append(pf.Warnings, "history lookup failed: "+err.Error())
					} else {
						if len(msgs) > 3 {
							msgs = msgs[len(msgs)-3:]
						}
						pf.Recent = msgs
					}
					_ = db.Close()
				}
			} else {
				pf.Warnings = append(pf.Warnings, "no local mirror; sender checks and history skipped (run sync)")
			}
			if !yes {
				pf.Note = "preview only; re-run with --yes to send"
				return printSendPreflight(cmd, flags, pf)
			}
			if blocking && !sendAnyway {
				pf.Note = "not sent: the sender failed preflight; fix the warnings or pass --send-anyway"
				if err := printSendPreflight(cmd, flags, pf); err != nil {
					return err
				}
				return usageErr(fmt.Errorf("sender %s failed preflight: %s", pf.From, strings.Join(pf.Warnings, "; ")))
			}
			if cliutil.IsAnyHarness() {
				return writeHarnessRefusal(cmd.OutOrStdout(), flags, "send SMS")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, _, err := c.Post(cmd.Context(), "/messages", map[string]any{"from": pf.From, "to": pf.To, "text": pf.Text})
			if err != nil {
				return classifyAPIErrorOnly(err)
			}
			var env struct {
				Data struct {
					ID string `json:"id"`
					To []struct {
						Status string `json:"status"`
					} `json:"to"`
				} `json:"data"`
			}
			_ = json.Unmarshal(data, &env)
			pf.Sent = true
			pf.MessageID = env.Data.ID
			if len(env.Data.To) > 0 {
				pf.Status = env.Data.To[0].Status
			}
			return printSendPreflight(cmd, flags, pf)
		},
	}
	cmd.Flags().StringVar(&flagFrom, "from", "", "Your Telnyx number to send from (E.164, e.g. +12075550100)")
	cmd.Flags().StringVar(&flagTo, "to", "", "Recipient phone number (E.164 or 10-digit US)")
	cmd.Flags().StringVar(&flagText, "message", "", "Message body to send")
	cmd.Flags().BoolVar(&yes, "yes", false, "Actually send; without it only the preflight is shown")
	cmd.Flags().BoolVar(&sendAnyway, "send-anyway", false, "With --yes, send even when the sender fails preflight (no messaging profile or 10DLC campaign)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path used for preflight checks")
	return cmd
}

func printSendPreflight(cmd *cobra.Command, flags *rootFlags, pf sendPreflight) error {
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printJSONFiltered(cmd.OutOrStdout(), pf, flags)
	}
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "From %s to %s (%d segment(s)):\n  %s\n", pf.From, pf.To, pf.Segments, pf.Text)
	for _, warn := range pf.Warnings {
		fmt.Fprintf(w, "  ! %s\n", warn)
	}
	if len(pf.Recent) > 0 {
		fmt.Fprintln(w, "Recent history:")
		for _, m := range pf.Recent {
			fmt.Fprintf(w, "  %s %s %s\n", m.CreatedAt, m.Direction, m.Status)
		}
	}
	if pf.Sent {
		fmt.Fprintf(w, "Sent: id=%s status=%s\n", pf.MessageID, pf.Status)
	} else {
		fmt.Fprintln(w, pf.Note)
	}
	return nil
}
