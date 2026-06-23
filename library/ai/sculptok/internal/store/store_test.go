// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	st, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestJobsUpsertListSearch(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)

	if err := st.UpsertJob(ctx, Job{PromptID: "p1", Kind: "depthmap", Status: "completed", ImageURL: "u1", Params: `{"style":"pro"}`, ResultURLs: `["a","b"]`, CreditCost: 15, CreatedAt: "2026-01-01 00:00:00"}); err != nil {
		t.Fatalf("UpsertJob: %v", err)
	}
	if err := st.UpsertJob(ctx, Job{PromptID: "p2", Kind: "stl", Status: "submitted", CreditCost: 3, CreatedAt: "2026-01-02 00:00:00"}); err != nil {
		t.Fatalf("UpsertJob: %v", err)
	}
	// Upsert same prompt updates, not duplicates.
	if err := st.UpsertJob(ctx, Job{PromptID: "p1", Kind: "depthmap", Status: "completed", CreditCost: 15, CreatedAt: "2026-01-01 00:00:00"}); err != nil {
		t.Fatalf("UpsertJob update: %v", err)
	}

	n, err := st.CountJobs(ctx)
	if err != nil || n != 2 {
		t.Fatalf("CountJobs = %d, %v; want 2", n, err)
	}

	jobs, err := st.ListJobs(ctx, 10)
	if err != nil || len(jobs) != 2 {
		t.Fatalf("ListJobs = %d, %v; want 2", len(jobs), err)
	}
	// Newest first by created_at.
	if jobs[0].PromptID != "p2" {
		t.Fatalf("ListJobs order: got %s first, want p2", jobs[0].PromptID)
	}

	found, err := st.SearchJobs(ctx, "stl", 10)
	if err != nil || len(found) != 1 || found[0].PromptID != "p2" {
		t.Fatalf("SearchJobs(stl) = %v, %v; want [p2]", found, err)
	}
	// Negative: a term matching nothing returns empty, not all rows.
	none, err := st.SearchJobs(ctx, "nonexistent-term-xyz", 10)
	if err != nil || len(none) != 0 {
		t.Fatalf("SearchJobs(miss) = %d rows; want 0", len(none))
	}
}

func TestCreditEventsAnalytics(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)

	events := []CreditEvent{
		{ID: "e1", ActionType: 5, ChangeNum: -10, Remarks: "API Draw: p1", CreateDate: "2026-01-01 10:00:00"},
		{ID: "e2", ActionType: 5, ChangeNum: -3, Remarks: "API Draw: p2", CreateDate: "2026-01-01 11:00:00"},
		{ID: "e3", ActionType: 1, ChangeNum: 100, Remarks: "check-in", CreateDate: "2026-01-02 09:00:00"},
	}
	for _, e := range events {
		if err := st.UpsertCreditEvent(ctx, e); err != nil {
			t.Fatalf("UpsertCreditEvent: %v", err)
		}
	}

	groups, err := st.AnalyticsCreditEvents(ctx, "actionType", 10)
	if err != nil {
		t.Fatalf("Analytics: %v", err)
	}
	byGroup := map[string]GroupCount{}
	for _, g := range groups {
		byGroup[g.Group] = g
	}
	if byGroup["5"].Count != 2 || byGroup["5"].TotalChange != -13 {
		t.Fatalf("actionType 5 = count %d total %d; want 2 / -13", byGroup["5"].Count, byGroup["5"].TotalChange)
	}
	if byGroup["1"].TotalChange != 100 {
		t.Fatalf("actionType 1 total = %d; want 100", byGroup["1"].TotalChange)
	}

	// day grouping
	dayGroups, err := st.AnalyticsCreditEvents(ctx, "day", 10)
	if err != nil || len(dayGroups) != 2 {
		t.Fatalf("Analytics day = %d groups, %v; want 2", len(dayGroups), err)
	}

	// unsupported group-by is an error, not a silent empty result.
	if _, err := st.AnalyticsCreditEvents(ctx, "bogus", 10); err == nil {
		t.Fatal("Analytics(bogus) should error")
	}
}

func TestReconcile(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)

	// A recorded job for p1, but not p2.
	if err := st.UpsertJob(ctx, Job{PromptID: "p1", Kind: "depthmap", CreatedAt: "2026-01-01 00:00:00"}); err != nil {
		t.Fatalf("UpsertJob: %v", err)
	}
	// Two spend events: one matches p1, one references p2 (no job).
	_ = st.UpsertCreditEvent(ctx, CreditEvent{ID: "e1", ChangeNum: -10, Remarks: "API Draw: p1", CreateDate: "2026-01-01 10:00:00"})
	_ = st.UpsertCreditEvent(ctx, CreditEvent{ID: "e2", ChangeNum: -3, Remarks: "API Draw: p2", CreateDate: "2026-01-01 11:00:00"})
	// A positive (earn) event must be ignored.
	_ = st.UpsertCreditEvent(ctx, CreditEvent{ID: "e3", ChangeNum: 100, Remarks: "check-in", CreateDate: "2026-01-02 09:00:00"})

	rows, err := st.Reconcile(ctx, 100)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if len(rows) != 1 || rows[0].EventID != "e2" {
		t.Fatalf("Reconcile = %v; want only e2 unmatched", rows)
	}
}

func TestDrawings(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	if err := st.UpsertDrawing(ctx, Drawing{ID: "d1", ImgURL: "http://x/1.png", CreateDate: "2026-01-01 00:00:00"}); err != nil {
		t.Fatalf("UpsertDrawing: %v", err)
	}
	n, err := st.CountDrawings(ctx)
	if err != nil || n != 1 {
		t.Fatalf("CountDrawings = %d, %v; want 1", n, err)
	}
	rows, err := st.ListDrawings(ctx, 10)
	if err != nil || len(rows) != 1 || rows[0].ImgURL != "http://x/1.png" {
		t.Fatalf("ListDrawings = %v, %v", rows, err)
	}
}

func TestOpenReadOnlyMissing(t *testing.T) {
	ctx := context.Background()
	missing := filepath.Join(t.TempDir(), "nope.db")
	st, ok, err := OpenReadOnly(ctx, missing)
	if err != nil {
		t.Fatalf("OpenReadOnly: %v", err)
	}
	if ok || st != nil {
		t.Fatal("OpenReadOnly on missing file should return ok=false, nil store")
	}
}
