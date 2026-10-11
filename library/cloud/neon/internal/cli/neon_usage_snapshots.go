// Hand-authored usage snapshots for Neon plans without consumption history.
//
// The consumption_history endpoints are gated to Launch/Scale plans. Every
// project and branch row still carries billing-period-to-date counters, so each
// sync records one snapshot per project. Usage commands fall back to the
// differences between consecutive snapshots when no consumption rows exist.

package cli

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/cloud/neon/internal/store"
)

const neonSnapshotBasis = "sync-snapshots"

func ensureNeonUsageSnapshots(ctx context.Context, db *store.Store) error {
	_, err := db.DB().ExecContext(ctx, `CREATE TABLE IF NOT EXISTS neon_usage_snapshots (
		project_id TEXT NOT NULL,
		taken_at TEXT NOT NULL,
		compute REAL NOT NULL,
		storage REAL NOT NULL,
		transfer REAL NOT NULL,
		written REAL NOT NULL,
		PRIMARY KEY (project_id, taken_at)
	)`)
	if err != nil {
		return err
	}
	// period = the project's quota_reset_at; a change marks a billing reset.
	if _, err := db.DB().ExecContext(ctx, `ALTER TABLE neon_usage_snapshots ADD COLUMN period TEXT NOT NULL DEFAULT ''`); err != nil && !strings.Contains(err.Error(), "duplicate column") {
		return err
	}
	return nil
}

// neonSnapshotEligible is true only when this sync run refreshed projects and
// branches with no error or warning, so stale counters are never recorded.
func neonSnapshotEligible(clean, dirty map[string]bool) bool {
	for _, r := range []string{"projects", "branches"} {
		if !clean[r] || dirty[r] {
			return false
		}
	}
	return true
}

// recordNeonUsageSnapshot stores the current period-to-date totals per project:
// compute, transfer and written are summed across branches; storage is the
// project's synthetic storage size (a gauge, not a counter).
func recordNeonUsageSnapshot(ctx context.Context, db *store.Store, now time.Time) error {
	projects, err := neonRows(db, "projects")
	if err != nil {
		return err
	}
	branches, err := neonRows(db, "branches", "projects/branches", "projects_branches")
	if err != nil {
		return err
	}
	if len(projects) == 0 {
		return nil
	}
	totals := map[string]*[4]float64{}
	periods := map[string]string{}
	for _, p := range projects {
		totals[ns(p, "id")] = &[4]float64{0, nf(p, "synthetic_storage_size"), 0, 0}
		periods[ns(p, "id")] = ns(p, "quota_reset_at")
	}
	for _, b := range branches {
		t := totals[ns(b, "project_id")]
		if t == nil {
			continue
		}
		t[0] += nf(b, "compute_time_seconds")
		t[2] += nf(b, "data_transfer_bytes")
		t[3] += nf(b, "written_data_bytes")
	}
	if err := ensureNeonUsageSnapshots(ctx, db); err != nil {
		return err
	}
	takenAt := now.UTC().Format(time.RFC3339)
	for id, t := range totals {
		if _, err := db.DB().ExecContext(ctx,
			`INSERT OR REPLACE INTO neon_usage_snapshots (project_id, taken_at, compute, storage, transfer, written, period) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			id, takenAt, t[0], t[1], t[2], t[3], periods[id]); err != nil {
			return err
		}
	}
	return nil
}

type neonSnapshot struct {
	projectID string
	takenAt   string
	period    string
	values    [4]float64
}

// snapshotPoints turns snapshots into usage periods: a period-to-date baseline
// from the first snapshot, then one increment per consecutive pair.
// Counters that drop (billing period reset) use the new total as the increment.
func snapshotPoints(db *store.Store, names map[string]string) ([]neonUsagePoint, error) {
	out := make([]neonUsagePoint, 0)
	if db == nil {
		return out, nil
	}
	var exists int
	if err := db.DB().QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='neon_usage_snapshots'`).Scan(&exists); err != nil || exists == 0 {
		return out, err
	}
	rows, err := db.DB().Query(`SELECT project_id, taken_at, COALESCE(period, ''), compute, storage, transfer, written FROM neon_usage_snapshots ORDER BY project_id, taken_at`)
	if err != nil {
		return nil, err
	}
	snaps := make([]neonSnapshot, 0)
	for rows.Next() {
		var s neonSnapshot
		if err := rows.Scan(&s.projectID, &s.takenAt, &s.period, &s.values[0], &s.values[1], &s.values[2], &s.values[3]); err != nil {
			_ = rows.Close()
			return nil, err
		}
		snaps = append(snaps, s)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	byProject := map[string][]neonSnapshot{}
	for _, s := range snaps {
		byProject[s.projectID] = append(byProject[s.projectID], s)
	}
	for id, list := range byProject {
		// The first snapshot is the billing-period-to-date baseline; later
		// snapshots contribute increments. Summing over a window therefore gives
		// period-to-date usage plus everything observed since.
		first := list[0]
		out = append(out, neonUsagePoint{ProjectID: id, ProjectName: names[id], Start: first.takenAt, End: first.takenAt, Basis: neonSnapshotBasis + " (period to date)",
			Values: map[string]float64{"compute": first.values[0], "storage": first.values[1], "transfer": first.values[2], "written": first.values[3]}})
		for i := 1; i < len(list); i++ {
			prev, cur := list[i-1], list[i]
			v := map[string]float64{"storage": cur.values[1]}
			for idx, metric := range map[int]string{0: "compute", 2: "transfer", 3: "written"} {
				d := cur.values[idx] - prev.values[idx]
				if cur.period != prev.period && cur.period != "" && prev.period != "" {
					d = cur.values[idx] // new billing period: counters restarted
				} else if d < 0 && (cur.period == "" || prev.period == "") {
					d = cur.values[idx] // no period info: assume a reset
				} else if d < 0 {
					d = 0 // a deleted branch lowered the project total
				}
				v[metric] = d
			}
			out = append(out, neonUsagePoint{ProjectID: id, ProjectName: names[id], Start: prev.takenAt, End: cur.takenAt, Basis: neonSnapshotBasis, Values: v})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ProjectID != out[j].ProjectID {
			return out[i].ProjectID < out[j].ProjectID
		}
		if out[i].Start != out[j].Start {
			return out[i].Start < out[j].Start
		}
		return out[i].End < out[j].End
	})
	return out, nil
}

// neonPlanGated reports Neon's "included with <plan> plans and above" refusal.
func neonPlanGated(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "not available") && strings.Contains(msg, "plans and above")
}
