package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/cloud/neon/internal/store"
)

func TestNeonSyncConsumptionDefaultsPaginationAndIdentity(t *testing.T) {
	calls := 0
	neonTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("mutation: %s", r.Method)
		}
		if r.URL.Path == "/projects" {
			fmt.Fprint(w, `{"projects":[{"id":"p","org_id":"o"}]}`)
			return
		}
		calls++
		q := r.URL.Query()
		if q.Get("org_id") != "o" || q.Get("granularity") != "daily" || q.Get("from") == "" || q.Get("to") == "" {
			t.Errorf("missing defaults: %s", r.URL)
		}
		if r.URL.Path == "/consumption_history/v2/projects" && q.Get("metrics") == "" {
			t.Error("missing v2 metrics")
		}
		next := `,"pagination":{"cursor":"next"}`
		start := "2026-10-08T00:00:00Z"
		if q.Get("cursor") == "next" {
			next = ""
			start = "2026-10-09T00:00:00Z"
		}
		fmt.Fprintf(w, `{"projects":[{"project_id":"p","periods":[{"period_id":"billing","period_start":"2026-10-01T00:00:00Z","consumption":[{"timeframe_start":%q,"timeframe_end":"2026-10-10T00:00:00Z","compute_time_seconds":10}]}]}]%s}`, start, next)
	})
	path := seedNeon(t, nil)
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	flags := &rootFlags{noCache: true}
	c, err := flags.newClient()
	if err != nil {
		t.Fatal(err)
	}
	for _, resource := range []string{"consumption-history-projects", "consumption-history-v2-projects"} {
		var events bytes.Buffer
		result := syncNeonConsumption(context.Background(), c, db, resource, "", 0, nil, &events)
		if result.Err != nil || result.Warn != nil || result.Count != 2 {
			t.Fatalf("%+v events=%s", result, events.String())
		}
		rows, err := neonRows(db, resource)
		if err != nil || len(rows) != 2 {
			t.Fatalf("rows=%v err=%v", rows, err)
		}
	}
	if calls != 4 {
		t.Fatal(calls)
	}
}
func TestNeonSyncProjectsAndEndpointsAndAncestry(t *testing.T) {
	for _, resource := range defaultSyncResources() {
		if resource == "users" || resource == "regions" || resource == "api-keys" {
			t.Fatalf("org-key-incompatible default: %s", resource)
		}
	}
	hasProjects, hasEndpoints := false, false
	for _, r := range defaultSyncResources() {
		if r == "projects" {
			hasProjects = true
		}
	}
	for _, d := range dependentResourceDefs() {
		if d.Name == "endpoints" && d.ParentTable == "projects" {
			hasEndpoints = true
		}
	}
	if !hasProjects || !hasEndpoints {
		t.Fatal("missing defaults or endpoint dependent")
	}
	neonTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("mutation %s", r.Method)
		}
		switch r.URL.Path {
		case "/projects":
			fmt.Fprint(w, `{"projects":[{"id":"p","name":"Test","org_id":"o"}]}`)
		case "/projects/p/branches":
			fmt.Fprint(w, `{"branches":[{"id":"root","project_id":"p"},{"id":"child","project_id":"p","parent_id":"root"}]}`)
		case "/projects/p/endpoints":
			fmt.Fprint(w, `{"endpoints":[{"id":"ep","project_id":"p","branch_id":"child","last_active":"2026-10-10T00:00:00Z","suspend_timeout_seconds":300,"autoscaling_limit_min_cu":0.25,"autoscaling_limit_max_cu":1}]}`)
		default:
			fmt.Fprint(w, `{"operations":[]}`)
		}
	})
	path := seedNeon(t, nil)
	flags := &rootFlags{asJSON: true, noCache: true}
	cmd := newSyncCmd(flags)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"--resources", "projects", "--db", path})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err, output.String())
	}
	db, err := store.OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	endpoints, err := neonRows(db, "endpoints")
	if err != nil || len(endpoints) != 1 || projectOf(endpoints[0]) != "p" {
		t.Fatalf("endpoints %v err %v", endpoints, err)
	}
	branches, err := neonRows(db, "branches")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, b := range branches {
		if ns(b, "id") == "child" {
			found = ns(b, "branch_parent_id") == "root"
		}
	}
	if !found {
		raw, _ := json.Marshal(branches)
		t.Fatal(string(raw))
	}
}

func TestNeonSyncConsumptionContinuesPastPlanGatedOrg(t *testing.T) {
	neonTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/projects" {
			fmt.Fprint(w, `{"projects":[{"id":"free-p","org_id":"a-free"},{"id":"paid-p","org_id":"b-paid"}]}`)
			return
		}
		if r.URL.Query().Get("org_id") == "a-free" {
			w.WriteHeader(403)
			fmt.Fprint(w, `{"message":"This endpoint is not available. It is included with Scale plans and above."}`)
			return
		}
		fmt.Fprint(w, `{"projects":[{"project_id":"paid-p","periods":[{"consumption":[{"timeframe_start":"2026-10-08T00:00:00Z","timeframe_end":"2026-10-09T00:00:00Z","compute_time_seconds":10}]}]}]}`)
	})
	path := seedNeon(t, nil)
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c, err := (&rootFlags{noCache: true}).newClient()
	if err != nil {
		t.Fatal(err)
	}
	var events bytes.Buffer
	result := syncNeonConsumption(context.Background(), c, db, "consumption-history-projects", "", 0, nil, &events)
	if result.Err != nil || result.Warn == nil || result.Count != 1 {
		t.Fatalf("paid org must still sync after a gated org: %+v events=%s", result, events.String())
	}
}
