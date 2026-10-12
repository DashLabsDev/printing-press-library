// pp:data-source local

package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/hostfully/internal/store"
	"github.com/spf13/cobra"
)

type dayPattern struct {
	Day    string  `json:"day"`
	Rate   float64 `json:"occupancy_rate"`
	Nights int     `json:"nights"`
}

type patternsResult struct {
	Weeks      int          `json:"weeks_analyzed"`
	ByDay      []dayPattern `json:"by_day_of_week"`
	PeakDay    string       `json:"peak_day"`
	OffPeakDay string       `json:"off_peak_day"`
}

func newNovelPatternsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var weeks int

	cmd := &cobra.Command{
		Use:   "patterns",
		Short: "Identify peak vs off-peak trends: weekday vs weekend, seasonal patterns",
		Example: strings.Trim(`
  hostfully-pp-cli patterns
  hostfully-pp-cli patterns --weeks 12
  hostfully-pp-cli patterns --json`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "patterns")
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

			end := calendarDate(time.Now(), location)
			start := end.AddDate(0, 0, -weeks*7)
			totalDays := calendarDays(start, end)

			// Count occupied nights per day of week (across all units)
			dayNights := make(map[time.Weekday]int)
			dayCounts := make(map[time.Weekday]int) // total possible days per weekday

			for i := 0; i < totalDays; i++ {
				d := start.AddDate(0, 0, i)
				dayCounts[d.Weekday()]++
			}

			for _, r := range reservations {
				ciDate := calendarDate(r.CheckIn, r.CheckIn.Location())
				coDate := calendarDate(r.CheckOut, r.CheckIn.Location())
				for d := ciDate; d.Before(coDate); d = d.AddDate(0, 0, 1) {
					if d.Format("2006-01-02") >= start.Format("2006-01-02") && d.Format("2006-01-02") < end.Format("2006-01-02") {
						dayNights[d.Weekday()]++
					}
				}
			}

			// Build results
			dayOrder := []time.Weekday{
				time.Monday, time.Tuesday, time.Wednesday,
				time.Thursday, time.Friday, time.Saturday, time.Sunday,
			}

			var byDay []dayPattern
			peakRate := 0.0
			offPeakRate := 100.0
			peakDay := ""
			offPeakDay := ""

			for _, wd := range dayOrder {
				possible := dayCounts[wd] * len(unitNumbers())
				rate := 0.0
				if possible > 0 {
					rate = float64(dayNights[wd]) / float64(possible) * 100
				}
				byDay = append(byDay, dayPattern{
					Day:    wd.String(),
					Rate:   rate,
					Nights: dayNights[wd],
				})
				if rate > peakRate {
					peakRate = rate
					peakDay = wd.String()
				}
				if rate < offPeakRate {
					offPeakRate = rate
					offPeakDay = wd.String()
				}
			}

			result := patternsResult{
				Weeks:      weeks,
				ByDay:      byDay,
				PeakDay:    peakDay,
				OffPeakDay: offPeakDay,
			}

			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "## Booking Patterns — Last %d Weeks\n\n", weeks)
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "Day\tOccupancy\tNights")
			for _, d := range byDay {
				marker := ""
				if d.Day == peakDay {
					marker = " ← PEAK"
				} else if d.Day == offPeakDay {
					marker = " ← OFF-PEAK"
				}
				fmt.Fprintf(w, "%s\t%.1f%%\t%d%s\n", d.Day, d.Rate, d.Nights, marker)
			}
			return w.Flush()
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&weeks, "weeks", 8, "Number of weeks to analyze")

	return cmd
}
