// Copyright 2026 Greg Mushen and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/telnyx/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/telnyx/internal/store"
)

// mdrRow is one synced messaging detail record, normalized so "ours" is the
// owned Telnyx number and "counterpart" is the guest/vendor on the other end.
// Telnyx MDRs carry delivery metadata only; message bodies are not included.
type mdrRow struct {
	ID          string   `json:"id"`
	Direction   string   `json:"direction"`
	Ours        string   `json:"our_number"`
	Counterpart string   `json:"counterpart"`
	Status      string   `json:"status"`
	MessageType string   `json:"message_type"`
	Parts       int      `json:"parts"`
	Cost        float64  `json:"cost"`
	Currency    string   `json:"currency"`
	Profile     string   `json:"profile"`
	CreatedAt   string   `json:"created_at"`
	ErrorCodes  []string `json:"error_codes,omitempty"`
}

// normalizePhone turns common US inputs (2075551234, 1-207-555-1234) into
// E.164 so they match Telnyx's +1XXXXXXXXXX storage.
func normalizePhone(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	plus := strings.HasPrefix(s, "+")
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	d := b.String()
	switch {
	case plus:
		return "+" + d
	case len(d) == 10:
		return "+1" + d
	case len(d) == 11 && strings.HasPrefix(d, "1"):
		return "+" + d
	default:
		return "+" + d
	}
}

// parseSince converts a --since window (7d, 48h, 2w) into an RFC3339 lower
// bound. Empty means no bound.
func parseSince(since string) (string, error) {
	if strings.TrimSpace(since) == "" {
		return "", nil
	}
	d, err := cliutil.ParseDurationLoose(since)
	if err != nil {
		return "", usageErr(fmt.Errorf("invalid --since %q: use a window like 48h, 7d or 2w", since))
	}
	return time.Now().UTC().Add(-d).Format(time.RFC3339), nil
}

// openMirror resolves the DB path and opens the local store. ok=false means
// there is no mirror yet; an empty JSON list (or a stderr hint) was written.
// empty is what machine-readable callers receive when no mirror exists yet.
func openMirror(cmd *cobra.Command, flags *rootFlags, dbPath *string, resource string, empty any) (*store.Store, bool, error) {
	if *dbPath == "" {
		*dbPath = defaultDBPath("telnyx-pp-cli")
	}
	if _, statErr := os.Stat(*dbPath); os.IsNotExist(statErr) {
		fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: telnyx-pp-cli sync --resources %s --db %s\n", *dbPath, resource, *dbPath)
		if !wantsHumanTable(cmd.OutOrStdout(), flags) {
			return nil, false, printJSONFiltered(cmd.OutOrStdout(), empty, flags)
		}
		return nil, false, nil
	}
	db, err := store.OpenWithContext(cmd.Context(), *dbPath)
	if err != nil {
		return nil, false, fmt.Errorf("opening database: %w", err)
	}
	if !hintIfUnsynced(cmd, db, resource) {
		hintIfStale(cmd, db, resource, flags.maxAge)
	}
	return db, true, nil
}

// loadMDRs reads synced messaging detail records, oldest first. counterpart
// filters to one remote number when non-empty. Rows are drained before
// returning so callers may issue follow-up queries.
func loadMDRs(ctx context.Context, db *store.Store, sinceISO, counterpart string) ([]mdrRow, error) {
	q := `SELECT id, data FROM detail_records WHERE COALESCE(record_type,'') IN ('message','message_detail_record','messaging')`
	args := []any{}
	if sinceISO != "" {
		q += ` AND julianday(created_at) >= julianday(?)`
		args = append(args, sinceISO)
	}
	if counterpart != "" {
		q += ` AND ((direction = 'inbound' AND cli = ?) OR (direction <> 'inbound' AND cld = ?))`
		args = append(args, counterpart, counterpart)
	}
	q += ` ORDER BY created_at ASC`
	rows, err := db.DB().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query detail records: %w", err)
	}
	type raw struct{ id, data string }
	raws := make([]raw, 0)
	for rows.Next() {
		var r raw
		if err := rows.Scan(&r.id, &r.data); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan detail record: %w", err)
		}
		raws = append(raws, r)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterate detail records: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close rows: %w", err)
	}
	out := make([]mdrRow, 0, len(raws))
	for _, r := range raws {
		m, ok := decodeMDR(r.id, r.data)
		if ok {
			out = append(out, m)
		}
	}
	return out, nil
}

func decodeMDR(id, data string) (mdrRow, bool) {
	var d map[string]json.RawMessage
	if err := json.Unmarshal([]byte(data), &d); err != nil {
		return mdrRow{}, false
	}
	str := func(k string) string {
		var s string
		if v, ok := d[k]; ok {
			_ = json.Unmarshal(v, &s)
		}
		return s
	}
	num := func(k string) float64 {
		f, ok := cliutil.ExtractNumber(d, k)
		if !ok {
			return 0
		}
		return f
	}
	m := mdrRow{
		ID:          id,
		Direction:   str("direction"),
		Status:      str("status"),
		MessageType: str("message_type"),
		Parts:       int(num("parts")),
		Cost:        num("cost"),
		Currency:    str("currency"),
		Profile:     str("profile_name"),
		CreatedAt:   str("created_at"),
	}
	cli, cld := str("cli"), str("cld")
	if m.Direction == "inbound" {
		m.Ours, m.Counterpart = cld, cli
	} else {
		m.Ours, m.Counterpart = cli, cld
	}
	if raw, ok := d["errors"]; ok {
		var errs []map[string]any
		if json.Unmarshal(raw, &errs) == nil {
			for _, e := range errs {
				if c, ok := e["code"]; ok {
					m.ErrorCodes = append(m.ErrorCodes, fmt.Sprint(c))
				}
			}
		}
	}
	return m, true
}

// isFailedStatus reports whether an MDR status means the text did not arrive.
func isFailedStatus(s string) bool {
	switch strings.ToLower(s) {
	case "failed", "delivery_failed", "sending_failed", "delivery_unconfirmed", "expired", "rejected", "undelivered", "gw_timeout", "dlr_timeout":
		return true
	}
	return false
}

func round4(f float64) float64 {
	v, _ := strconv.ParseFloat(strconv.FormatFloat(f, 'f', 4, 64), 64)
	return v
}

// nullStr reads a nullable TEXT column.
func nullStr(n sql.NullString) string {
	if n.Valid {
		return n.String
	}
	return ""
}
