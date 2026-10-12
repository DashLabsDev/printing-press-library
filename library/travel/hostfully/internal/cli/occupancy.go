// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/hostfully/internal/store"
	"github.com/spf13/cobra"
)

type unitOccupancy struct {
	Unit         int     `json:"unit"`
	OccupiedDays int     `json:"occupied_days"`
	TotalDays    int     `json:"total_days"`
	Rate         float64 `json:"rate"`
}

type occupancyResult struct {
	Period    string          `json:"period"`
	Units     []unitOccupancy `json:"units"`
	Aggregate unitOccupancy   `json:"aggregate"`
}

func newNovelOccupancyCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var month string
	var days int

	cmd := &cobra.Command{
		Use:   "occupancy",
		Short: "Calculate per-unit and aggregate occupancy rates over any time period",
		Example: strings.Trim(`
  hostfully-pp-cli occupancy --month 2026-05
  hostfully-pp-cli occupancy --days 90
  hostfully-pp-cli occupancy --month 2026-05 --json`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "occupancy")
			}

			if dbPath == "" {
				dbPath = defaultDBPath("hostfully-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w (run 'hostfully-pp-cli sync' first)", err)
			}
			defer db.Close()

			reservations, err := loadReservations(db)
			if err != nil {
				return fmt.Errorf("loading reservations: %w", err)
			}

			location, err := propertyLocation()
			if err != nil {
				return err
			}

			var start, end time.Time
			if month != "" {
				t, err := time.ParseInLocation("2006-01", month, location)
				if err != nil {
					return fmt.Errorf("invalid month format (use YYYY-MM): %w", err)
				}
				start = t
				end = t.AddDate(0, 1, 0)
			} else {
				end = calendarDate(time.Now(), location)
				start = end.AddDate(0, 0, -days)
			}

			totalDays := calendarDays(start, end)

			// Count occupied days per unit
			occDays := make(map[int]int)
			for _, u := range unitNumbers() {
				occDays[u] = 0
			}

			for _, r := range reservations {
				ciDate := calendarDate(r.CheckIn, r.CheckIn.Location())
				coDate := calendarDate(r.CheckOut, r.CheckIn.Location())

				for d := ciDate; d.Before(coDate); d = d.AddDate(0, 0, 1) {
					if d.Format("2006-01-02") >= start.Format("2006-01-02") && d.Format("2006-01-02") < end.Format("2006-01-02") {
						occDays[r.Unit]++
					}
				}
			}

			result := occupancyResult{
				Period: fmt.Sprintf("%s – %s", start.Format("Jan 2, 2006"), end.AddDate(0, 0, -1).Format("Jan 2, 2006")),
			}

			totalOccupied := 0
			for _, u := range unitNumbers() {
				rate := 0.0
				if totalDays > 0 {
					rate = float64(occDays[u]) / float64(totalDays) * 100
				}
				result.Units = append(result.Units, unitOccupancy{
					Unit:         u,
					OccupiedDays: occDays[u],
					TotalDays:    totalDays,
					Rate:         rate,
				})
				totalOccupied += occDays[u]
			}

			totalPossible := totalDays * len(unitNumbers())
			aggRate := 0.0
			if totalPossible > 0 {
				aggRate = float64(totalOccupied) / float64(totalPossible) * 100
			}
			result.Aggregate = unitOccupancy{
				Unit:         0,
				OccupiedDays: totalOccupied,
				TotalDays:    totalPossible,
				Rate:         aggRate,
			}

			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "## Occupancy — %s\n\n", result.Period)
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "Unit\tOccupied\tTotal\tRate")

			// Sort by rate descending
			sorted := make([]unitOccupancy, len(result.Units))
			copy(sorted, result.Units)
			sort.Slice(sorted, func(i, j int) bool {
				return sorted[i].Rate > sorted[j].Rate
			})

			for _, u := range sorted {
				fmt.Fprintf(w, "#%d\t%d days\t%d days\t%.1f%%\n",
					u.Unit, u.OccupiedDays, u.TotalDays, u.Rate)
			}
			fmt.Fprintf(w, "\nAll\t%d days\t%d days\t%.1f%%\n",
				result.Aggregate.OccupiedDays, result.Aggregate.TotalDays, result.Aggregate.Rate)
			return w.Flush()
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&month, "month", "", "Month to analyze (YYYY-MM)")
	cmd.Flags().IntVar(&days, "days", 30, "Number of days to look back (used when --month not set)")

	return cmd
}
