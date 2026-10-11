package cli

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/cloud/neon/internal/store"
)

func seedSnapshotStore(t *testing.T, compute map[string]float64) *store.Store {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "neon.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	setNeonCounters(t, db, compute)
	return db
}

func setNeonCounters(t *testing.T, db *store.Store, compute map[string]float64) {
	setNeonCountersPeriod(t, db, compute, "2026-11-01T00:00:00Z")
}

func setNeonCountersPeriod(t *testing.T, db *store.Store, compute map[string]float64, period string) {
	t.Helper()
	for id, c := range compute {
		p, _ := json.Marshal(map[string]any{"id": id, "name": "name-" + id, "synthetic_storage_size": 100, "quota_reset_at": period})
		if err := db.Upsert("projects", id, p); err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(map[string]any{"id": "br-" + id, "project_id": id, "compute_time_seconds": c, "data_transfer_bytes": 10, "written_data_bytes": 5})
		if err := db.Upsert("branches", "br-"+id, b); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNeonSnapshotFallbackSingleSnapshotIsPeriodToDate(t *testing.T) {
	db := seedSnapshotStore(t, map[string]float64{"small": 50, "big": 900})
	now := time.Now().UTC()
	if err := recordNeonUsageSnapshot(context.Background(), db, now); err != nil {
		t.Fatal(err)
	}
	points, err := consumptionPoints(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 2 {
		t.Fatalf("want 2 period-to-date points, got %d: %+v", len(points), points)
	}
	ranked := rankNeonUsage(points, "compute", now.Add(-time.Hour), now.Add(time.Hour))
	if len(ranked) != 2 || ranked[0].ProjectID != "big" || ranked[0].Value != 900 {
		t.Fatalf("unexpected ranking: %+v", ranked)
	}
	if ranked[0].Basis == "" {
		t.Fatal("snapshot-derived rows must say their basis")
	}
	if trend := trendNeonUsage(points, now.Add(-time.Hour), now.Add(time.Hour)); len(trend) == 0 || trend[0].Delta != nil {
		t.Fatalf("single snapshot must not fabricate a delta: %+v", trend)
	}
}

func TestNeonSnapshotFallbackDiffsAndCounterReset(t *testing.T) {
	db := seedSnapshotStore(t, map[string]float64{"p": 100})
	t0 := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	if err := recordNeonUsageSnapshot(context.Background(), db, t0); err != nil {
		t.Fatal(err)
	}
	setNeonCounters(t, db, map[string]float64{"p": 160})
	if err := recordNeonUsageSnapshot(context.Background(), db, t0.Add(24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	setNeonCountersPeriod(t, db, map[string]float64{"p": 30}, "2026-12-01T00:00:00Z") // billing period reset
	if err := recordNeonUsageSnapshot(context.Background(), db, t0.Add(48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	points, err := consumptionPoints(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 3 || points[0].Values["compute"] != 100 || points[1].Values["compute"] != 60 || points[2].Values["compute"] != 30 {
		t.Fatalf("want baseline 100 then increments 60, 30 (reset), got %+v", points)
	}
	trend := trendNeonUsage(points, t0.Add(-time.Hour), t0.Add(72*time.Hour))
	var computeDeltas int
	for _, r := range trend {
		if r.Metric == "compute" && r.Delta != nil {
			computeDeltas++
			if *r.Delta != -30 {
				t.Fatalf("want delta -30, got %v", *r.Delta)
			}
		}
	}
	if computeDeltas != 1 {
		t.Fatalf("want exactly one compute delta, got %d", computeDeltas)
	}
}

func TestNeonSnapshotDeletedBranchIsNotUsage(t *testing.T) {
	db := seedSnapshotStore(t, map[string]float64{"p": 900})
	t0 := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	if err := recordNeonUsageSnapshot(context.Background(), db, t0); err != nil {
		t.Fatal(err)
	}
	setNeonCounters(t, db, map[string]float64{"p": 300}) // same period, busy branch deleted
	if err := recordNeonUsageSnapshot(context.Background(), db, t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	points, _ := consumptionPoints(db)
	if len(points) != 2 || points[1].Values["compute"] != 0 {
		t.Fatalf("a drop within one period must count as 0, got %+v", points)
	}
	ranked := rankNeonUsage(points, "storage", t0.Add(-time.Hour), t0.Add(2*time.Hour))
	if len(ranked) != 1 || ranked[0].Value != 100 {
		t.Fatalf("snapshot storage must be the peak size, not a sum: %+v", ranked)
	}
}

func TestNeonSnapshotEligible(t *testing.T) {
	ok := map[string]bool{"projects": true, "branches": true}
	if !neonSnapshotEligible(ok, nil) || neonSnapshotEligible(map[string]bool{"projects": true}, nil) || neonSnapshotEligible(ok, map[string]bool{"branches": true}) {
		t.Fatal("snapshot gating wrong")
	}
}

func TestNeonPlanGated(t *testing.T) {
	if !neonPlanGated(errors.New(`GET /consumption_history/projects returned HTTP 403: {"message":"This endpoint is not available. It is included with Scale plans and above."}`)) {
		t.Fatal("plan refusal not detected")
	}
	if neonPlanGated(errors.New("HTTP 500")) || neonPlanGated(nil) {
		t.Fatal("false positive")
	}
}

func TestNeonUsageChoosesSourcePerProject(t *testing.T) {
	db := seedSnapshotStore(t, map[string]float64{"paid": 5, "free": 70})
	if err := recordNeonUsageSnapshot(context.Background(), db, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	hist, _ := json.Marshal(map[string]any{"id": "h1", "project_id": "paid", "timeframe_start": "2026-10-08T00:00:00Z", "timeframe_end": "2026-10-09T00:00:00Z", "compute_time_seconds": 900})
	if err := db.Upsert("consumption-history-projects", "h1", hist); err != nil {
		t.Fatal(err)
	}
	points, err := consumptionPoints(db)
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string][]string{}
	for _, p := range points {
		sources[p.ProjectID] = append(sources[p.ProjectID], p.Basis)
	}
	if len(sources["paid"]) != 1 || sources["paid"][0] != "" {
		t.Fatalf("paid project must use history only: %v", sources["paid"])
	}
	if len(sources["free"]) != 1 || sources["free"][0] == "" {
		t.Fatalf("free project must fall back to snapshots: %v", sources["free"])
	}
}

func TestNeonStorageRanksOnOneMeasurement(t *testing.T) {
	db := seedSnapshotStore(t, map[string]float64{"paid": 5, "free": 70})
	now := time.Now().UTC()
	if err := recordNeonUsageSnapshot(context.Background(), db, now); err != nil {
		t.Fatal(err)
	}
	// Byte-month history for "paid" must not be compared with snapshot bytes.
	hist, _ := json.Marshal(map[string]any{"id": "h1", "project_id": "paid", "timeframe_start": now.Format(time.RFC3339), "timeframe_end": now.Format(time.RFC3339), "storage": 9e12})
	if err := db.Upsert("consumption-history-projects", "h1", hist); err != nil {
		t.Fatal(err)
	}
	points, err := storageRankPoints(db)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range points {
		if p.Basis == "" {
			t.Fatalf("storage ranking used a history point: %+v", p)
		}
	}
	ranked := rankNeonUsage(points, "storage", now.Add(-time.Hour), now.Add(time.Hour))
	if len(ranked) != 2 || ranked[0].Value != 100 || ranked[1].Value != 100 {
		t.Fatalf("both projects ranked on snapshot bytes: %+v", ranked)
	}
}

func TestNeonStorageKeepsFreshReadingAfterGap(t *testing.T) {
	db := seedSnapshotStore(t, map[string]float64{"p": 1})
	now := time.Now().UTC()
	if err := recordNeonUsageSnapshot(context.Background(), db, now.Add(-10*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := recordNeonUsageSnapshot(context.Background(), db, now); err != nil {
		t.Fatal(err)
	}
	points, err := storageRankPoints(db)
	if err != nil {
		t.Fatal(err)
	}
	ranked := rankNeonUsage(points, "storage", now.Add(-7*24*time.Hour), now.Add(time.Minute))
	if len(ranked) != 1 || ranked[0].Value != 100 {
		t.Fatalf("today's storage reading must count in a 7d window: %+v", ranked)
	}
}
