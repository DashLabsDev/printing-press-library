package store

import (
	"encoding/json"
	"fmt"
	"time"
)

// ReplaceHostfullyReservations publishes a complete per-property fetch atomically.
// Other properties/agencies and unrelated resource types are left untouched.
func (s *Store) ReplaceHostfullyReservations(agency string, byProperty map[string][]json.RawMessage, checkpoint string) error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	total := 0
	for property, leads := range byProperty {
		seen := map[string]bool{}
		for _, lead := range leads {
			var item map[string]any
			if err := json.Unmarshal(lead, &item); err != nil {
				return fmt.Errorf("invalid reservation: %w", err)
			}
			id, _ := item["uid"].(string)
			if id == "" {
				return fmt.Errorf("reservation has no uid")
			}
			if value, _ := item["propertyUid"].(string); value != "" && value != property {
				return fmt.Errorf("reservation property differs from fetched property")
			}
			if value, _ := item["agencyUid"].(string); value != "" && value != agency {
				return fmt.Errorf("reservation agency differs from fetched agency")
			}
			item["propertyUid"] = property
			item["agencyUid"] = agency
			normalized, err := json.Marshal(item)
			if err != nil {
				return err
			}
			if err := s.upsertGenericResourceTx(tx, "leads", id, s.mergeIncomingResourceData(tx, "leads", id, normalized)); err != nil {
				return err
			}
			seen[id] = true
			total++
		}
		rows, err := tx.Query(`SELECT resource_type, id FROM resources
   WHERE resource_type IN ('leads', 'leads_leads') AND json_extract(data, '$.propertyUid') = ?
   AND (json_extract(data, '$.agencyUid') IS NULL OR json_extract(data, '$.agencyUid') = ?)`, property, agency)
		if err != nil {
			return err
		}
		type key struct{ kind, id string }
		var stale []key
		for rows.Next() {
			var k key
			if err := rows.Scan(&k.kind, &k.id); err != nil {
				rows.Close()
				return err
			}
			// The canonical leads row supersedes any old generic-sync duplicate.
			if !seen[k.id] || k.kind == "leads_leads" {
				stale = append(stale, k)
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, k := range stale {
			if _, err := tx.Exec(`DELETE FROM resources WHERE resource_type = ? AND id = ?`, k.kind, k.id); err != nil {
				return err
			}
			if _, err := tx.Exec(`DELETE FROM resources_fts WHERE rowid = ?`, ftsRowID(k.kind, k.id)); err != nil {
				return err
			}
		}
	}
	_, err = tx.Exec(`INSERT INTO sync_state (resource_type, last_cursor, last_synced_at, total_count, last_attempt_complete)
 VALUES (?, '', ?, ?, 1) ON CONFLICT(resource_type) DO UPDATE SET last_cursor = '',
 last_synced_at = excluded.last_synced_at, total_count = excluded.total_count, last_attempt_complete = 1`,
		checkpoint, time.Now().UTC().Format(time.RFC3339), total)
	if err != nil {
		return err
	}
	return tx.Commit()
}
