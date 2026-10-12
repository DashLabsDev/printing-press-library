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

// TestNovelChannelsHelpWires smoke-tests that the channels command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelChannelsHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"channels", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("channels --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "channels"} {
		if !strings.Contains(help, want) {
			t.Fatalf("channels --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestReviewChannelsExcludeFutureAndOldStays(t *testing.T) {
	t.Setenv(timeZoneEnvVar, "UTC")
	db, path := reviewDB(t)
	now := time.Now().UTC()
	for id, delta := range map[string]int{"recent": -5, "old": -40, "future": 40} {
		ci := now.AddDate(0, 0, delta)
		reviewLead(t, db, id, ci, ci.AddDate(0, 0, 2))
	}
	if err := db.SaveSyncState(reservationArchiveState, "", 3); err != nil {
		t.Fatal(err)
	}
	cmd := newNovelChannelsCmd(&rootFlags{asJSON: true})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--db", path, "--period", "30d"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var got channelsResult
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Total != 1 || len(got.Channels) != 1 || got.Channels[0].Percent != 100 {
		t.Fatal(out.String())
	}
}
