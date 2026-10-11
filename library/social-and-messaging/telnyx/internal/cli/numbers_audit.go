// Copyright 2026 Greg Mushen and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/telnyx/internal/store"
)

type auditRow struct {
	PhoneNumber        string   `json:"phone_number"`
	Status             string   `json:"status"`
	MessagingProfileID string   `json:"messaging_profile_id"`
	MessagingProfile   string   `json:"messaging_profile"`
	ConnectionName     string   `json:"connection_name"`
	CampaignID         string   `json:"campaign_id"`
	CampaignAssignment string   `json:"campaign_assignment"`
	TCRCampaignID      string   `json:"tcr_campaign_id"`
	CanText            bool     `json:"can_text"`
	Problems           []string `json:"problems"`
}

// loadResourceRows returns decoded JSON objects for one synced resource type.
func loadResourceRows(ctx context.Context, db *store.Store, types ...string) ([]map[string]any, error) {
	ph := strings.TrimSuffix(strings.Repeat("?,", len(types)), ",")
	args := make([]any, len(types))
	for i, t := range types {
		args[i] = t
	}
	rows, err := db.DB().QueryContext(ctx, `SELECT data FROM resources WHERE resource_type IN (`+ph+`)`, args...)
	if err != nil {
		return nil, fmt.Errorf("query %v: %w", types, err)
	}
	datas := make([]string, 0)
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan %v: %w", types, err)
		}
		datas = append(datas, d)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(datas))
	for _, d := range datas {
		var m map[string]any
		if json.Unmarshal([]byte(d), &m) == nil {
			out = append(out, m)
		}
	}
	return out, nil
}

func sval(m map[string]any, k string) string {
	if v, ok := m[k]; ok && v != nil {
		return fmt.Sprint(v)
	}
	return ""
}

// buildNumberAudit joins owned numbers, their messaging settings, messaging
// profile names, and 10DLC campaign assignments from the local store.
func buildNumberAudit(ctx context.Context, db *store.Store) ([]auditRow, error) {
	numbers, err := loadResourceRows(ctx, db, "phone-numbers")
	if err != nil {
		return nil, err
	}
	msgSettings, err := loadResourceRows(ctx, db, "phone-numbers-messaging")
	if err != nil {
		return nil, err
	}
	profiles, err := loadResourceRows(ctx, db, "messaging-profiles")
	if err != nil {
		return nil, err
	}
	assignments, err := loadResourceRows(ctx, db, "10dlc")
	if err != nil {
		return nil, err
	}
	profileName := map[string]string{}
	for _, p := range profiles {
		profileName[sval(p, "id")] = sval(p, "name")
	}
	profileFor := map[string]string{}
	for _, s := range msgSettings {
		profileFor[sval(s, "phone_number")] = sval(s, "messaging_profile_id")
	}
	assignFor := map[string]map[string]any{}
	for _, a := range assignments {
		if pn := sval(a, "phoneNumber"); pn != "" {
			assignFor[pn] = a
		}
	}
	out := make([]auditRow, 0, len(numbers))
	for _, n := range numbers {
		pn := sval(n, "phone_number")
		r := auditRow{
			PhoneNumber:    pn,
			Status:         sval(n, "status"),
			ConnectionName: sval(n, "connection_name"),
			Problems:       []string{},
		}
		r.MessagingProfileID = profileFor[pn]
		if r.MessagingProfileID == "" {
			r.MessagingProfileID = sval(n, "messaging_profile_id")
		}
		r.MessagingProfile = profileName[r.MessagingProfileID]
		if a := assignFor[pn]; a != nil {
			r.CampaignID = sval(a, "campaignId")
			r.CampaignAssignment = sval(a, "assignmentStatus")
			r.TCRCampaignID = sval(a, "tcrCampaignId")
		}
		if r.Status != "" && r.Status != "active" {
			r.Problems = append(r.Problems, "number status is "+r.Status)
		}
		if r.MessagingProfileID == "" {
			r.Problems = append(r.Problems, "no messaging profile (cannot send or receive SMS)")
		}
		if strings.HasPrefix(pn, "+1") && sval(n, "phone_number_type") != "toll_free" {
			switch {
			case r.CampaignID == "":
				r.Problems = append(r.Problems, "no 10DLC campaign assignment (US carriers will block A2P texts)")
			case r.CampaignAssignment != "" && r.CampaignAssignment != "ASSIGNED":
				r.Problems = append(r.Problems, "10DLC campaign assignment is "+r.CampaignAssignment)
			}
		}
		r.CanText = len(r.Problems) == 0
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CanText != out[j].CanText {
			return !out[i].CanText
		}
		return out[i].PhoneNumber < out[j].PhoneNumber
	})
	return out, nil
}

func newNovelNumbersAuditCmd(flags *rootFlags) *cobra.Command {
	var all bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Find owned numbers that cannot text: missing messaging profile or missing 10DLC campaign assignment.",
		Long: `Use this command to find numbers that can't send or are non-compliant. Do NOT use it for cost; use 'spend' instead.

Joins synced phone numbers, their messaging settings, messaging profiles and
10DLC phone-number campaign assignments (assignment status only; brand and
campaign approval are not checked). Shows only numbers with problems
unless --all is set. Run 'telnyx-pp-cli sync --resources phone-numbers,phone-numbers-messaging,messaging-profiles,10dlc' first.`,
		Example: strings.Trim(`
  telnyx-pp-cli numbers audit --agent
  telnyx-pp-cli numbers audit --all`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
			"pp:happy-args":  "--all",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "numbers audit")
			}
			db, ok, err := openMirror(cmd, flags, &dbPath, "phone-numbers", make([]auditRow, 0))
			if !ok || err != nil {
				return err
			}
			defer db.Close()
			rows, err := buildNumberAudit(cmd.Context(), db)
			if err != nil {
				return err
			}
			if !all {
				filtered := make([]auditRow, 0)
				for _, r := range rows {
					if !r.CanText {
						filtered = append(filtered, r)
					}
				}
				rows = filtered
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			w := cmd.OutOrStdout()
			if len(rows) == 0 {
				fmt.Fprintln(w, "Every synced number has a messaging profile and a 10DLC campaign assignment.")
				return nil
			}
			for _, r := range rows {
				state := "OK"
				if !r.CanText {
					state = "PROBLEM"
				}
				fmt.Fprintf(w, "%-14s %-8s profile=%q campaign=%s %s\n", r.PhoneNumber, state, r.MessagingProfile, r.TCRCampaignID, strings.Join(r.Problems, "; "))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Include numbers with no problems")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path (default: the CLI's synced store)")
	return cmd
}
