// Copyright 2026 Greg Mushen and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type spendGroup struct {
	Key      string  `json:"key"`
	Messages int     `json:"messages"`
	Parts    int     `json:"parts"`
	Cost     float64 `json:"cost"`
}

type spendView struct {
	GroupBy     string       `json:"group_by"`
	Since       string       `json:"since,omitempty"`
	Currency    string       `json:"currency"`
	Groups      []spendGroup `json:"groups"`
	TotalCost   float64      `json:"total_cost"`
	Messages    int          `json:"messages"`
	CoveredDays float64      `json:"covered_days"`
	DailyAvg    float64      `json:"daily_avg"`
	Note        string       `json:"note,omitempty"`
	Balance     *float64     `json:"balance,omitempty"`
	RunwayDays  *float64     `json:"runway_days,omitempty"`
	RunwayError string       `json:"runway_error,omitempty"`
}

func newNovelSpendCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var flagGroupBy string
	var runway bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "spend",
		Short: "Total messaging cost by number, profile, direction, or day, with balance runway.",
		Long: `Use this command for message/call cost totals. Do NOT use it for the account balance; use 'balance' instead.

Sums the cost of synced messaging detail records, grouped by your number,
messaging profile, direction, or day. --runway additionally makes one live
GET /balance call and divides the available balance by the average daily
spend over the window.`,
		Example: strings.Trim(`
  telnyx-pp-cli spend --since 30d --group-by number --agent
  telnyx-pp-cli spend --since 7d --group-by day
  telnyx-pp-cli spend --since 30d --runway`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
			"pp:happy-args":  "--since=30d;--group-by=number",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "spend")
			}
			if flagGroupBy == "" {
				flagGroupBy = "number"
			}
			switch flagGroupBy {
			case "number", "profile", "direction", "day":
			default:
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--group-by must be one of number, profile, direction, day (got %q)", flagGroupBy))
			}
			if flagSince == "" {
				flagSince = "30d"
			}
			sinceISO, err := parseSince(flagSince)
			if err != nil {
				return err
			}
			db, ok, err := openMirror(cmd, flags, &dbPath, "detail-records", spendView{GroupBy: flagGroupBy, Since: flagSince, Currency: "USD", Groups: make([]spendGroup, 0)})
			if !ok || err != nil {
				return err
			}
			defer db.Close()
			msgs, err := loadMDRs(cmd.Context(), db, sinceISO, "")
			if err != nil {
				return err
			}
			view := spendView{GroupBy: flagGroupBy, Since: flagSince, Currency: "USD", Groups: make([]spendGroup, 0)}
			groups := map[string]*spendGroup{}
			for _, m := range msgs {
				var key string
				switch flagGroupBy {
				case "number":
					key = m.Ours
				case "profile":
					key = m.Profile
				case "direction":
					key = m.Direction
				case "day":
					if len(m.CreatedAt) >= 10 {
						key = m.CreatedAt[:10]
					}
				}
				if key == "" {
					key = "(unknown)"
				}
				g := groups[key]
				if g == nil {
					g = &spendGroup{Key: key}
					groups[key] = g
				}
				g.Messages++
				g.Parts += m.Parts
				g.Cost += m.Cost
				view.TotalCost += m.Cost
				view.Messages++
				if m.Currency != "" {
					view.Currency = m.Currency
				}
			}
			for _, g := range groups {
				g.Cost = round4(g.Cost)
				view.Groups = append(view.Groups, *g)
			}
			if flagGroupBy == "day" {
				sort.Slice(view.Groups, func(i, j int) bool { return view.Groups[i].Key < view.Groups[j].Key })
			} else {
				sort.Slice(view.Groups, func(i, j int) bool { return view.Groups[i].Cost > view.Groups[j].Cost })
			}
			view.TotalCost = round4(view.TotalCost)
			// Average over the requested --since window. The oldest synced
			// message is not a coverage boundary (quiet days have no rows), so
			// shrinking the window to it would inflate spend and cut runway.
			if d, err := parseWindowDays(flagSince); err == nil && d > 0 {
				if d < 1 {
					d = 1
				}
				view.CoveredDays = round4(d)
				view.DailyAvg = round4(view.TotalCost / d)
			}
			if view.Messages == 0 {
				view.Note = "no synced messaging records in this window; run 'telnyx-pp-cli sync --resources detail-records' if this is unexpected"
			}
			if runway {
				bal, err := fetchAvailableBalance(cmd, flags)
				if err != nil {
					view.RunwayError = err.Error()
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: runway unavailable: %v\n", err)
				} else {
					view.Balance = &bal
					if view.DailyAvg > 0 {
						r := round4(bal / view.DailyAvg)
						view.RunwayDays = &r
					}
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			if view.Messages == 0 {
				fmt.Fprintf(w, "No synced messages in the last %s.\n", flagSince)
				return nil
			}
			for _, g := range view.Groups {
				fmt.Fprintf(w, "%-24s %5d msgs %6d parts  $%.4f\n", g.Key, g.Messages, g.Parts, g.Cost)
			}
			fmt.Fprintf(w, "\nTotal $%.4f %s over %s (avg $%.4f/day)\n", view.TotalCost, view.Currency, flagSince, view.DailyAvg)
			if view.Balance != nil {
				fmt.Fprintf(w, "Available balance $%.2f", *view.Balance)
				if view.RunwayDays != nil {
					fmt.Fprintf(w, " ≈ %.0f days of runway", *view.RunwayDays)
				}
				fmt.Fprintln(w)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "30d", "Window to total (e.g. 7d, 30d)")
	cmd.Flags().StringVar(&flagGroupBy, "group-by", "number", "Group by: number, profile, direction, or day")
	cmd.Flags().BoolVar(&runway, "runway", false, "Also fetch the live balance and estimate days of runway at the window's daily average")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path (default: the CLI's synced store)")
	return cmd
}

func parseWindowDays(since string) (float64, error) {
	sinceISO, err := parseSince(since)
	if err != nil || sinceISO == "" {
		return 0, fmt.Errorf("no window")
	}
	t, err := time.Parse(time.RFC3339, sinceISO)
	if err != nil {
		return 0, err
	}
	return time.Since(t).Hours() / 24, nil
}

// fetchAvailableBalance calls GET /balance and returns available_credit
// (falling back to balance).
func fetchAvailableBalance(cmd *cobra.Command, flags *rootFlags) (float64, error) {
	c, err := flags.newClient()
	if err != nil {
		return 0, err
	}
	data, err := c.Get(cmd.Context(), "/balance", nil)
	if err != nil {
		return 0, classifyAPIErrorOnly(err)
	}
	var env struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return 0, fmt.Errorf("parsing balance: %w", err)
	}
	for _, k := range []string{"available_credit", "balance"} {
		if raw, ok := env.Data[k]; ok {
			var s string
			if json.Unmarshal(raw, &s) == nil {
				if f, err := strconv.ParseFloat(s, 64); err == nil {
					return f, nil
				}
			}
			var f float64
			if json.Unmarshal(raw, &f) == nil {
				return f, nil
			}
		}
	}
	return 0, fmt.Errorf("balance response had no available_credit")
}
