// Copyright 2026 Greg Mushen and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/hostfully/internal/cliutil/testenv"
)

// TestNovelTurnoverHelpWires smoke-tests that the turnover command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelTurnoverHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"turnover", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("turnover --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "turnover"} {
		if !strings.Contains(help, want) {
			t.Fatalf("turnover --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestReviewTurnoverKeepsNextStayBeyondWindow(t *testing.T) {
	t.Setenv(timeZoneEnvVar, "UTC")
	db, path := reviewDB(t)
	today := time.Now().UTC().Truncate(24 * time.Hour)
	reviewLead(t, db, "past", today.AddDate(0, 0, -4), today.AddDate(0, 0, -2))
	reviewLead(t, db, "current", today.AddDate(0, 0, -1), today.AddDate(0, 0, 1))
	reviewLead(t, db, "next", today.AddDate(0, 0, 14), today.AddDate(0, 0, 16))
	if err := db.SaveSyncState(reservationArchiveState, "", 3); err != nil {
		t.Fatal(err)
	}
	cmd := newNovelTurnoverCmd(&rootFlags{asJSON: true})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--db", path, "--days", "7"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var got turnoverResult
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Gaps) != 1 || got.Gaps[0].Hours != 13*24 {
		t.Fatal(out.String())
	}
}
