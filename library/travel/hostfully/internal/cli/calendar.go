package cli

import "time"

func calendarDate(t time.Time, location *time.Location) time.Time {
	t = t.In(location)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, location)
}

func calendarDays(start, end time.Time) int {
	n := 0
	for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
		n++
	}
	return n
}
