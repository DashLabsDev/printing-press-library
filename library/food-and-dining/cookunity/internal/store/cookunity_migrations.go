// Copyright 2026 Greg Cole and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: CookUnity per-week menu snapshot table for `drift`. Preserved on regen.
//
// The generated `meals` table is keyed by bare meal id, so syncing a new week
// overwrites the previous week's rows. `drift` needs two weeks side by side, so
// this table keys meals by (delivery_date, meal_id) to retain per-week history.
package store

import (
	"encoding/json"
	"fmt"
)

// ReplaceMeals atomically replaces the entire current-week `meals` table with
// items in a single transaction: every prior row is deleted and every new meal
// inserted under one commit. Either the whole new week lands or nothing changes,
// so an interrupted sync can never expose a mix of old and new weeks. Returns
// the number of meals stored.
func (s *Store) ReplaceMeals(items []json.RawMessage) (int, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM "meals"`); err != nil {
		return 0, fmt.Errorf("clearing meals: %w", err)
	}
	stored := 0
	for _, item := range items {
		obj, err := DecodeJSONObject(item)
		if err != nil {
			continue
		}
		id := ExtractResourceID("meals", obj)
		if id == "" {
			continue
		}
		storageID := resourceStorageID("meals", id, obj)
		if err := s.upsertGenericResourceTx(tx, "meals", storageID, item); err != nil {
			return 0, fmt.Errorf("insert meal %s: %w", storageID, err)
		}
		if err := s.upsertMealsTx(tx, storageID, obj, item); err != nil {
			return 0, fmt.Errorf("insert typed meal %s: %w", storageID, err)
		}
		stored++
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return stored, nil
}

// EnsureMealSnapshots lazily creates the per-week snapshot table. Safe to call
// repeatedly.
func (s *Store) EnsureMealSnapshots() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS "meal_snapshots" (
		"delivery_date" TEXT NOT NULL,
		"meal_id"       TEXT NOT NULL,
		"name"          TEXT,
		"final_price"   REAL,
		"calories"      INTEGER,
		"data"          JSON NOT NULL,
		"synced_at"     DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY ("delivery_date", "meal_id")
	)`)
	if err != nil {
		return fmt.Errorf("creating meal_snapshots: %w", err)
	}
	return nil
}

// UpsertMealSnapshot writes one meal into the per-week snapshot table.
func (s *Store) UpsertMealSnapshot(deliveryDate, mealID, name string, finalPrice float64, calories int, data json.RawMessage) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(
		`INSERT INTO "meal_snapshots" ("delivery_date","meal_id","name","final_price","calories","data")
		 VALUES (?,?,?,?,?,?)
		 ON CONFLICT("delivery_date","meal_id") DO UPDATE SET
		   "name"=excluded."name","final_price"=excluded."final_price",
		   "calories"=excluded."calories","data"=excluded."data",
		   "synced_at"=CURRENT_TIMESTAMP`,
		deliveryDate, mealID, name, finalPrice, calories, string(data),
	)
	if err != nil {
		return fmt.Errorf("upserting meal snapshot %s/%s: %w", deliveryDate, mealID, err)
	}
	return nil
}

// SnapshotDates returns the distinct delivery dates that have snapshots, newest first.
func (s *Store) SnapshotDates() ([]string, error) {
	if err := s.EnsureMealSnapshots(); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT DISTINCT "delivery_date" FROM "meal_snapshots" ORDER BY "delivery_date" DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var dates []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		dates = append(dates, d)
	}
	return dates, rows.Err()
}

// SnapshotMeals returns all meal JSON rows for a given delivery date.
func (s *Store) SnapshotMeals(deliveryDate string) ([]json.RawMessage, error) {
	if err := s.EnsureMealSnapshots(); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT "data" FROM "meal_snapshots" WHERE "delivery_date"=? ORDER BY "meal_id"`, deliveryDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []json.RawMessage
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		out = append(out, json.RawMessage(data))
	}
	return out, rows.Err()
}
