package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/hostfully/internal/store"
	"github.com/spf13/cobra"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/hostfully/internal/client"
)

type fakeHostfullyLeadAPI struct {
	get  func(path string, params map[string]string) (json.RawMessage, error)
	post func(path string, body any) (json.RawMessage, int, error)
}

func (f *fakeHostfullyLeadAPI) GetNoRetry(path string, params map[string]string) (json.RawMessage, error) {
	return f.get(path, params)
}

func (f *fakeHostfullyLeadAPI) Post(path string, body any) (json.RawMessage, int, error) {
	return f.post(path, body)
}

func fakeLeadPage(uids ...string) json.RawMessage {
	leads := make([]map[string]string, 0, len(uids))
	for _, uid := range uids {
		leads = append(leads, map[string]string{"uid": uid})
	}
	data, _ := json.Marshal(map[string]any{"leads": leads})
	return data
}

func fakeOffset(params map[string]string) int {
	raw, _ := base64.StdEncoding.DecodeString(params["_cursor"])
	var value struct {
		Offset int `json:"offset"`
	}
	_ = json.Unmarshal(raw, &value)
	return value.Offset
}

func TestFetchAllHostfullyUnitLeadsRecoversPoisonedRESTRecord(t *testing.T) {
	rest := &fakeHostfullyLeadAPI{}
	rest.get = func(_ string, params map[string]string) (json.RawMessage, error) {
		offset := fakeOffset(params)
		limit, _ := strconv.Atoi(params["_limit"])
		switch {
		case offset == 0 && limit == hostfullyLeadPageSize:
			return nil, &client.APIError{Method: "GET", Path: "/leads", StatusCode: 500}
		case offset == 0 && limit == 100:
			return fakeLeadPage("a", "b"), nil
		case offset == 100 && limit > 1:
			return nil, &client.APIError{Method: "GET", Path: "/leads", StatusCode: 500}
		case offset == 100 && limit == 1:
			return nil, &client.APIError{Method: "GET", Path: "/leads", StatusCode: 500}
		case offset > 100:
			return fakeLeadPage(), nil
		default:
			return fakeLeadPage(), nil
		}
	}

	graphql := &fakeHostfullyLeadAPI{}
	graphql.post = func(path string, _ any) (json.RawMessage, int, error) {
		if path != "/graphql" {
			t.Fatalf("GraphQL path = %q", path)
		}
		return json.RawMessage(`{"data":{"leads":[{"uid":"a"},{"uid":"b"},{"uid":"c"}]},"extensions":{}}`), 200, nil
	}

	result, err := fetchAllHostfullyUnitLeads(rest, graphql, "agency", "property")
	if err != nil {
		t.Fatal(err)
	}
	if result.Recovered != 1 {
		t.Fatalf("recovered = %d, want 1", result.Recovered)
	}
	got := map[string]bool{}
	for _, lead := range result.Leads {
		got[hostfullyLeadUID(lead)] = true
	}
	for _, uid := range []string{"a", "b", "c"} {
		if !got[uid] {
			t.Errorf("missing lead %q", uid)
		}
	}
}

func TestFetchAllHostfullyUnitLeadsFailsWhenGraphQLDoesNotFillGap(t *testing.T) {
	rest := &fakeHostfullyLeadAPI{}
	rest.get = func(_ string, params map[string]string) (json.RawMessage, error) {
		limit, _ := strconv.Atoi(params["_limit"])
		offset := fakeOffset(params)
		if offset >= hostfullyLeadPageSize && limit == hostfullyLeadPageSize {
			return fakeLeadPage(), nil
		}
		return nil, &client.APIError{Method: "GET", Path: "/leads", StatusCode: 500}
	}
	graphql := &fakeHostfullyLeadAPI{}
	graphql.post = func(_ string, _ any) (json.RawMessage, int, error) {
		return json.RawMessage(`{"data":{"leads":[]},"extensions":{}}`), 200, nil
	}

	_, err := fetchAllHostfullyUnitLeads(rest, graphql, "agency", "property")
	if err == nil {
		t.Fatal("expected incomplete recovery error")
	}
}

func TestHostfullyGraphQLBaseURL(t *testing.T) {
	for input, want := range map[string]string{
		"https://platform.hostfully.com/api/v3": "https://platform.hostfully.com/api/v3.3",
		"https://example.test/api/v3.3/":        "https://example.test/api/v3.3",
	} {
		t.Run(fmt.Sprintf("%s", input), func(t *testing.T) {
			if got := hostfullyGraphQLBaseURL(input); got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
}

func TestDateFilteredLeadFetchUsesPropertyLocalBounds(t *testing.T) {
	t.Parallel()
	day := time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)
	rest := &fakeHostfullyLeadAPI{}
	rest.get = func(_ string, params map[string]string) (json.RawMessage, error) {
		if params["checkInTo"] != "2026-08-30" || params["checkOutFrom"] != "2026-08-30" {
			t.Fatalf("date filters = %#v", params)
		}
		return json.RawMessage(`{"leads":[{"uid":"stay","status":"BOOKED","checkInLocalDateTime":"2026-08-29T15:00:00","checkOutLocalDateTime":"2026-08-31T11:00:00"}]}`), nil
	}
	graphql := &fakeHostfullyLeadAPI{post: func(string, any) (json.RawMessage, int, error) {
		t.Fatal("GraphQL should not be called for a successful REST page")
		return nil, 0, nil
	}}
	result, err := fetchHostfullyUnitLeadsForDate(rest, graphql, "agency", "property", day)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Leads) != 1 || hostfullyLeadUID(result.Leads[0]) != "stay" {
		t.Fatalf("result = %#v", result)
	}
}

func TestRawLeadOverlapUsesInclusiveCheckoutDate(t *testing.T) {
	t.Parallel()
	raw := json.RawMessage(`{"uid":"stay","checkInLocalDateTime":"2026-08-29T15:00:00","checkOutLocalDateTime":"2026-08-30T11:00:00"}`)
	if !rawLeadOverlapsDate(raw, time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("checkout day must be included for departure reporting")
	}
	if rawLeadOverlapsDate(raw, time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("stay must not overlap the day after checkout")
	}
}

func reviewDB(t *testing.T) (*store.Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "archive.db")
	db, err := store.OpenWithContext(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db, path
}

func reviewLead(t *testing.T, db *store.Store, id string, ci, co time.Time) {
	t.Helper()
	data, err := json.Marshal(map[string]any{"uid": id, "propertyUid": testUnitUIDs[1], "status": "BOOKED", "channel": "DIRECT", "checkInLocalDateTime": ci.Format("2006-01-02T15:04:05"), "checkOutLocalDateTime": co.Format("2006-01-02T15:04:05")})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Upsert("leads", id, data); err != nil {
		t.Fatal(err)
	}
}

func TestReviewReportsRequireFetchedArchive(t *testing.T) {
	db, path := reviewDB(t)
	if err := db.Upsert("properties", "placeholder", json.RawMessage(`{"uid":"placeholder"}`)); err != nil {
		t.Fatal(err)
	}
	for _, factory := range []func(*rootFlags) *cobra.Command{newNovelVacancyCmd, newNovelMaintenanceCmd, newNovelChannelsCmd, newNovelTurnoverCmd, newNovelOccupancyCmd, newNovelRevenueCmd, newNovelPatternsCmd, newGuestsCmd} {
		cmd := factory(&rootFlags{asJSON: true})
		cmd.SetArgs([]string{"--db", path})
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "fetch") {
			t.Fatalf("%s: %v", cmd.Name(), err)
		}
	}
	if _, err := loadWhoisReservations(db); err == nil {
		t.Fatal("whois accepted unfetched archive")
	}
	if err := db.SaveSyncState(reservationArchiveState, "", 0); err != nil {
		t.Fatal(err)
	}
	if rows, err := loadReservations(db); err != nil || len(rows) != 0 {
		t.Fatalf("completed empty archive: %v %v", rows, err)
	}
}

func TestReviewFetchCompletionAndWriteFailure(t *testing.T) {
	t.Setenv("HOSTFULLY_API_KEY", "placeholder-key")
	t.Setenv("HOSTFULLY_AGENCY_ID", "placeholder-agency")
	for _, mode := range []string{"empty", "success", "api-failure", "write-failure", "failed-refresh"} {
		t.Run(mode, func(t *testing.T) {
			db, path := reviewDB(t)
			if mode == "failed-refresh" {
				if err := db.SaveSyncState(reservationArchiveState, "", 1); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "write-failure" || mode == "failed-refresh" {
				if _, err := db.DB().Exec(`CREATE TRIGGER reject_review_leads BEFORE INSERT ON resources WHEN NEW.resource_type = 'leads' BEGIN SELECT RAISE(FAIL, 'placeholder write failure'); END`); err != nil {
					t.Fatal(err)
				}
			}
			cmd := newHostfullySyncCmdWithFetcher(&rootFlags{asJSON: true}, func(hostfullyLeadAPI, hostfullyLeadAPI, string, string) (hostfullyLeadFetch, error) {
				if mode == "api-failure" {
					return hostfullyLeadFetch{}, errors.New("placeholder API failure")
				}
				if mode == "empty" {
					return hostfullyLeadFetch{}, nil
				}
				return hostfullyLeadFetch{Leads: []json.RawMessage{json.RawMessage(`{"uid":"placeholder-lead","status":"BOOKED"}`)}}, nil
			})
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs([]string{"--db", path})
			err := cmd.Execute()
			wantSuccess := mode == "empty" || mode == "success"
			if (err == nil) != wantSuccess {
				t.Fatalf("error=%v output=%s", err, out.String())
			}
			if (requireReservationArchive(db) == nil) != (wantSuccess || mode == "failed-refresh") {
				t.Fatal("incorrect archive completion")
			}
			if !wantSuccess && strings.Contains(out.String(), `"status": "ok"`) {
				t.Fatal(out.String())
			}
		})
	}
}
