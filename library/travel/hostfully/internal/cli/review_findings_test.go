package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/travel/hostfully/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/travel/hostfully/internal/mcp/cobratree"
	"github.com/spf13/cobra"
)

type rejectReviewTransport struct{ t *testing.T }

func (r rejectReviewTransport) RoundTrip(*http.Request) (*http.Response, error) {
	r.t.Error("unexpected network request")
	return nil, errors.New("network forbidden")
}

func TestFindingsMockIsolation(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("HOSTFULLY_API_KEY", "")
	t.Setenv("HOSTFULLY_AGENCY_ID", "")
	old := http.DefaultTransport
	http.DefaultTransport = rejectReviewTransport{t}
	t.Cleanup(func() { http.DefaultTransport = old; mockModeEnabled = false; setUnitRegistry(testUnitUIDs, "test") })
	dir := t.TempDir()
	config := filepath.Join(dir, "invalid.json")
	if err := os.WriteFile(config, []byte("invalid config"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"jobs", "list", "--from", "2030-01-01", "--to", "2030-01-03"},
		{"inbox"}, {"inbox", "--since", "1h"}, {"inbox", "threads"}, {"inbox", "show", "fake-thread"},
		{"inbox", "messages"}, {"inbox", "message", "fake-message"},
		{"inbox", "reply", "fake-thread", "hello"}, {"inbox", "reply", "fake-thread", "hello", "--yes"},
		{"fetch", "--db", filepath.Join(dir, "archive.db")},
		{"quote", "--guests", "2", "--unit", "4", "--check-in", "2030-01-01", "--check-out", "2030-01-03"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			out := runMockHostfully(t, append(args, "--mock", "--json", "--config", config, "--deliver", "file:"+filepath.Join(dir, "delivery"))...)
			if !strings.Contains(string(out), "mock") {
				t.Fatalf("missing mock marker: %s", out)
			}
			if len(args) > 1 && args[1] == "reply" && !strings.Contains(string(out), `"would_send": true`) {
				t.Fatal(string(out))
			}
		})
	}
	for _, args := range [][]string{{"sync"}, {"vacancy"}, {"teach", "example"}} {
		var flags rootFlags
		root := newRootCmd(&flags)
		root.SetArgs(append(args, "--mock"))
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "--mock is not supported") {
			t.Fatalf("%v: %v", args, err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("mock wrote files: %v %v", entries, err)
	}
}

func TestFindingsInboxReplyHiddenAndConfirmed(t *testing.T) {
	mockModeEnabled = false
	root := &cobra.Command{Use: "hostfully"}
	root.AddCommand(newNovelInboxCmd(&rootFlags{}))
	mcpServer := server.NewMCPServer("test", "0.0.0")
	cobratree.RegisterAll(mcpServer, root, func() (string, error) { return "unused", nil })
	if mcpServer.GetTool("inbox_reply") != nil {
		t.Fatal("reply registered as MCP tool")
	}
	if mcpServer.GetTool("inbox_threads") == nil {
		t.Fatal("inbox reads missing from MCP")
	}
	cmd := newInboxReplyCmd(&rootFlags{})
	if cmd.Annotations["mcp:hidden"] != "true" {
		t.Fatal("reply is exposed to MCP")
	}
	cmd.SetArgs([]string{"fake-thread", "hello"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("confirmation changed: %v", err)
	}
}

func TestFindingsAvailabilityDiscoveryFailure(t *testing.T) {
	mockModeEnabled = false
	unitState.mu.Lock()
	unitState.ready = true
	unitState.err = errors.New("discovery failed")
	unitState.reg = unitRegistry{}
	unitState.mu.Unlock()
	t.Cleanup(func() { setUnitRegistry(testUnitUIDs, "test") })
	for _, args := range [][]string{nil, {"--unit", "1"}} {
		cmd := newAvailabilityCmdWithLoader(&rootFlags{}, func(*rootFlags, []int) ([]reservation, error) {
			t.Fatal("loader called after discovery failure")
			return nil, nil
		})
		cmd.SetArgs(args)
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "discovery failed") {
			t.Fatalf("%v: %v", args, err)
		}
	}
}

func TestFindingsGuestsLiveNoArchive(t *testing.T) {
	t.Setenv(timeZoneEnvVar, "UTC")
	for _, fail := range []bool{false, true} {
		called := false
		cmd := newGuestsCmdWithLoader(&rootFlags{dataSource: "live", asJSON: true}, func(_ *rootFlags) ([]reservation, error) {
			day := time.Now().UTC()
			called = true
			if fail {
				return nil, errors.New("live unavailable")
			}
			return []reservation{{Unit: 1, Status: "BOOKED", GuestFirst: "Fresh", CheckIn: day.AddDate(0, 0, -1), CheckOut: day.AddDate(0, 0, 1)}, {Unit: 2, Status: "CANCELLED", CheckIn: day.AddDate(0, 0, -1), CheckOut: day.AddDate(0, 0, 1)}}, nil
		})
		path := filepath.Join(t.TempDir(), "never-open.db")
		var out bytes.Buffer
		cmd.SetArgs([]string{"--db", path})
		cmd.SetOut(&out)
		cmd.SetErr(&bytes.Buffer{})
		err := cmd.Execute()
		if !called || (err != nil) != fail {
			t.Fatalf("called=%v err=%v", called, err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("live opened archive")
		}
		if !fail {
			var guests []guestInfo
			if err := json.Unmarshal(out.Bytes(), &guests); err != nil || len(guests) != 1 || guests[0].Name != "Fresh " {
				t.Fatal(out.String(), err)
			}
		}
	}
}

func TestFindingsCalendarNights(t *testing.T) {
	t.Setenv(timeZoneEnvVar, "America/New_York")
	loc, _ := propertyLocation()
	for _, start := range []time.Time{time.Date(2026, 3, 7, 15, 0, 0, 0, loc), time.Date(2026, 10, 31, 15, 0, 0, 0, loc)} {
		end := start.AddDate(0, 0, 2).Add(-4 * time.Hour)
		if n := calendarDays(calendarDate(start, loc), calendarDate(end, loc)); n != 2 {
			t.Fatalf("DST nights=%d", n)
		}
	}
	db, path := reviewDB(t)
	reviewLead(t, db, "spring", time.Date(2026, 3, 7, 15, 0, 0, 0, loc), time.Date(2026, 3, 9, 11, 0, 0, 0, loc))
	if err := db.SaveSyncState(reservationArchiveState, "", 1); err != nil {
		t.Fatal(err)
	}
	for _, factory := range []func(*rootFlags) *cobra.Command{newNovelOccupancyCmd, newNovelRevenueCmd} {
		cmd := factory(&rootFlags{asJSON: true})
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--db", path, "--month", "2026-03"})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		if cmd.Name() == "occupancy" {
			var got occupancyResult
			json.Unmarshal(out.Bytes(), &got)
			if got.Aggregate.OccupiedDays != 2 || got.Units[0].TotalDays != 31 {
				t.Fatal(out.String())
			}
		} else {
			var got revenueResult
			json.Unmarshal(out.Bytes(), &got)
			if got.TotalNights != 2 {
				t.Fatal(out.String())
			}
		}
	}
}

func TestFindingsVacancyMaintenanceLocalDates(t *testing.T) {
	t.Setenv(timeZoneEnvVar, "Etc/GMT+12")
	loc, _ := propertyLocation()
	today := calendarDate(time.Now(), loc)
	db, path := reviewDB(t)
	reviewLead(t, db, "stay", today.Add(15*time.Hour), today.AddDate(0, 0, 2).Add(11*time.Hour))
	if err := db.SaveSyncState(reservationArchiveState, "", 1); err != nil {
		t.Fatal(err)
	}
	for _, factory := range []func(*rootFlags) *cobra.Command{newNovelVacancyCmd, newNovelMaintenanceCmd} {
		cmd := factory(&rootFlags{asJSON: true})
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--db", path, "--units", "1", "--days", "4"})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		var result struct {
			Windows []vacancyWindow `json:"windows"`
		}
		if err := json.Unmarshal(out.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if len(result.Windows) != 1 || result.Windows[0].Start != today.AddDate(0, 0, 2).Format("Mon Jan 2") || result.Windows[0].Days != 2 {
			t.Fatal(out.String())
		}
	}
}

func TestFindingsTurnoverChronological(t *testing.T) {
	t.Setenv(timeZoneEnvVar, "UTC")
	db, path := reviewDB(t)
	year := time.Now().Year() + 1
	for i, d := range []time.Time{time.Date(year, 10, 2, 11, 0, 0, 0, time.UTC), time.Date(year, 10, 10, 11, 0, 0, 0, time.UTC), time.Date(year, 11, 2, 11, 0, 0, 0, time.UTC)} {
		reviewLead(t, db, fmt.Sprint(i), d.AddDate(0, 0, -1), d)
	}
	reviewLead(t, db, "last", time.Date(year, 12, 1, 15, 0, 0, 0, time.UTC), time.Date(year, 12, 2, 11, 0, 0, 0, time.UTC))
	if err := db.SaveSyncState(reservationArchiveState, "", 4); err != nil {
		t.Fatal(err)
	}
	cmd := newNovelTurnoverCmd(&rootFlags{asJSON: true})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--db", path, "--days", "800"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var got turnoverResult
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Gaps) != 3 {
		t.Fatal(out.String())
	}
	for i, want := range []string{"Oct 2 11:00 AM", "Oct 10 11:00 AM", "Nov 2 11:00 AM"} {
		if got.Gaps[i].GapStart != want {
			t.Fatal(out.String())
		}
	}
}

func TestFindingsFetchReconcilesAtomically(t *testing.T) {
	t.Setenv("HOSTFULLY_API_KEY", "placeholder-key")
	t.Setenv("HOSTFULLY_AGENCY_ID", "placeholder-agency")
	setUnitRegistry(map[int]string{1: "p1", 2: "p2"}, "test")
	t.Cleanup(func() { setUnitRegistry(testUnitUIDs, "test") })
	for _, mode := range []string{"success", "empty", "fetch-failure", "write-failure", "delete-failure", "checkpoint-failure", "invalid"} {
		t.Run(mode, func(t *testing.T) {
			db, path := reviewDB(t)
			seeds := map[string]string{
				"old":          `{"uid":"old","propertyUid":"p1","status":"BOOKED"}`,
				"keep":         `{"uid":"keep","propertyUid":"p1","status":"BOOKED"}`,
				"outside":      `{"uid":"outside","propertyUid":"p3","status":"BOOKED"}`,
				"other-agency": `{"uid":"other-agency","propertyUid":"p1","agencyUid":"other","status":"BOOKED"}`,
			}
			for id, raw := range seeds {
				if err := db.Upsert("leads", id, json.RawMessage(raw)); err != nil {
					t.Fatal(err)
				}
			}
			if err := db.Upsert("leads_leads", "legacy", json.RawMessage(`{"uid":"legacy","propertyUid":"p1"}`)); err != nil {
				t.Fatal(err)
			}
			if err := db.SaveSyncState(reservationArchiveState, "", 4); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "write-failure":
				_, err := db.DB().Exec(`CREATE TRIGGER reject_new BEFORE INSERT ON resources WHEN NEW.id = 'new' BEGIN SELECT RAISE(FAIL, 'write failure'); END`)
				if err != nil {
					t.Fatal(err)
				}
			case "delete-failure":
				_, err := db.DB().Exec(`CREATE TRIGGER reject_delete BEFORE DELETE ON resources BEGIN SELECT RAISE(FAIL, 'delete failure'); END`)
				if err != nil {
					t.Fatal(err)
				}
			case "checkpoint-failure":
				_, err := db.DB().Exec(`CREATE TRIGGER reject_checkpoint BEFORE UPDATE ON sync_state BEGIN SELECT RAISE(FAIL, 'checkpoint failure'); END`)
				if err != nil {
					t.Fatal(err)
				}
			}
			cmd := newHostfullySyncCmdWithFetcher(&rootFlags{asJSON: true}, func(_ hostfullyLeadAPI, _ hostfullyLeadAPI, _ string, property string) (hostfullyLeadFetch, error) {
				if property == "p2" {
					if mode == "fetch-failure" {
						return hostfullyLeadFetch{}, errors.New("fetch failure")
					}
					return hostfullyLeadFetch{}, nil
				}
				if mode == "empty" {
					return hostfullyLeadFetch{}, nil
				}
				if mode == "invalid" {
					return hostfullyLeadFetch{Leads: []json.RawMessage{json.RawMessage(`{"status":"BOOKED"}`)}}, nil
				}
				return hostfullyLeadFetch{Leads: []json.RawMessage{json.RawMessage(`{"uid":"keep","propertyUid":"p1","status":"CANCELLED"}`), json.RawMessage(`{"uid":"new","propertyUid":"p1","status":"BOOKED"}`)}}, nil
			})
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs([]string{"--db", path})
			err := cmd.Execute()
			success := mode == "success" || mode == "empty"
			if (err == nil) != success {
				t.Fatalf("%s: %v", mode, err)
			}
			if err := requireReservationArchive(db); err != nil {
				t.Fatal("prior completion lost", err)
			}
			for id, raw := range seeds {
				got, err := db.Get("leads", id)
				if !success || id == "outside" || id == "other-agency" {
					if err != nil || string(got) != raw {
						t.Fatalf("changed preserved row %s: %s %v", id, got, err)
					}
				} else if id == "old" || mode == "empty" {
					if err == nil {
						t.Fatalf("stale row %s survives", id)
					}
				} else if !strings.Contains(string(got), "CANCELLED") {
					t.Fatalf("row not updated: %s", got)
				}
			}
			if _, err := db.Get("leads", "new"); (err == nil) != (mode == "success") {
				t.Fatalf("new row commit/rollback: %v", err)
			}
			if _, err := db.Get("leads_leads", "legacy"); (err != nil) != success {
				t.Fatalf("legacy row commit/rollback: %v", err)
			}
		})
	}
}

func TestLiveLocalTodayUsesEachPropertyZone(t *testing.T) {
	t.Setenv(timeZoneEnvVar, "")
	reg := newUnitRegistry(map[int]string{1: "east", 2: "west"}, nil, "test")
	reg.setPropertyTimeZones([]propertySummary{{UID: "east", TimeZone: "Pacific/Kiritimati"}, {UID: "west", TimeZone: "Pacific/Pago_Pago"}})
	unitState.mu.Lock()
	unitState.reg = reg
	unitState.ready = true
	unitState.err = nil
	unitState.mu.Unlock()
	t.Cleanup(func() { setUnitRegistry(testUnitUIDs, "test") })

	// 12:00 UTC is already the next calendar day in Kiritimati (UTC+14) but
	// still the previous day in Pago Pago (UTC-11).
	fixed := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	dayFor := localTodayDayFor(func() time.Time { return fixed })
	for uid, want := range map[string]string{"east": "2026-03-11", "west": "2026-03-10"} {
		got, err := dayFor(uid)
		if err != nil {
			t.Fatal(err)
		}
		if got.Format("2006-01-02") != want {
			t.Fatalf("%s: requested date %s, want %s", uid, got.Format("2006-01-02"), want)
		}
	}
}
