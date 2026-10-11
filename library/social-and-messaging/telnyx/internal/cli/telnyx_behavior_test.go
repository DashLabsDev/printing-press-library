package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/telnyx/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/telnyx/internal/store"
)

// seedMirror writes a local store with two owned numbers: +15550001 is
// fully wired (profile + ASSIGNED 10DLC campaign); +15550009 has neither.
func seedMirror(t *testing.T) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "mirror.db")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()
	up := func(rt, id, js string) {
		if err := db.Upsert(rt, id, json.RawMessage(js)); err != nil {
			t.Fatalf("upsert %s: %v", rt, err)
		}
	}
	up("phone-numbers", "n1", `{"id":"n1","phone_number":"+15550001","status":"active","phone_number_type":"local"}`)
	up("phone-numbers", "n9", `{"id":"n9","phone_number":"+15550009","status":"active","phone_number_type":"local"}`)
	up("phone-numbers-messaging", "n1", `{"id":"n1","phone_number":"+15550001","messaging_profile_id":"p1"}`)
	up("messaging-profiles", "p1", `{"id":"p1","name":"Front desk"}`)
	up("10dlc", "+15550001", `{"phoneNumber":"+15550001","campaignId":"c1","assignmentStatus":"ASSIGNED","tcrCampaignId":"CABC"}`)
	now := time.Now().UTC()
	mdr := func(id, dir, cli, cld, status, cost string, ago time.Duration) {
		js := `{"id":"` + id + `","record_type":"message","direction":"` + dir + `","cli":"` + cli + `","cld":"` + cld + `","status":"` + status + `","cost":"` + cost + `","parts":1,"currency":"USD","created_at":"` + now.Add(-ago).Format(time.RFC3339) + `"}`
		if err := db.UpsertDetailRecords(json.RawMessage(js)); err != nil {
			t.Fatalf("upsert mdr: %v", err)
		}
	}
	mdr("m1", "outbound", "+15550001", "+15557777", "delivered", "0.0040", 72*time.Hour)
	mdr("m2", "inbound", "+15557777", "+15550001", "received", "0.0040", 48*time.Hour)
	mdr("m3", "inbound", "+15558888", "+15550001", "received", "0.0020", 2*time.Hour)
	mdr("m4", "outbound", "+15550001", "+15558888", "delivery_failed", "0.0010", time.Hour)
	return dbPath
}

func fakeTelnyx(t *testing.T, hits *int32) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/messages") {
			atomic.AddInt32(hits, 1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"id":"msg-1","to":[{"status":"queued"}]}}`))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("TELNYX_BASE_URL", srv.URL)
	t.Setenv("TELNYX_API_KEY", "KEYtest")
}

func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := RootCmd()
	cmd.SetArgs(args)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	return out.String(), err
}

func TestTextRefusesSenderThatFailsPreflight(t *testing.T) {
	testenv.Isolate(t)
	var hits int32
	fakeTelnyx(t, &hits)
	db := seedMirror(t)
	out, err := runCLI(t, "text", "--from", "+15550009", "--to", "+15557777", "--message", "hi", "--yes", "--json", "--db", db)
	if err == nil {
		t.Fatalf("expected refusal error, got success: %s", out)
	}
	if atomic.LoadInt32(&hits) != 0 {
		t.Fatalf("POST /messages was called %d times; preflight failure must not send", hits)
	}
	if !strings.Contains(out, "no messaging profile") {
		t.Fatalf("expected sender problem in output, got: %s", out)
	}
}

func TestTextPreviewWithoutYesDoesNotSend(t *testing.T) {
	testenv.Isolate(t)
	var hits int32
	fakeTelnyx(t, &hits)
	db := seedMirror(t)
	if out, err := runCLI(t, "text", "--from", "+15550001", "--to", "+15557777", "--message", "hi", "--json", "--db", db); err != nil {
		t.Fatalf("preview failed: %v %s", err, out)
	}
	if atomic.LoadInt32(&hits) != 0 {
		t.Fatalf("preview sent %d messages", hits)
	}
}

func TestTextSendsExactlyOnceForWiredSender(t *testing.T) {
	testenv.Isolate(t)
	var hits int32
	fakeTelnyx(t, &hits)
	db := seedMirror(t)
	out, err := runCLI(t, "text", "--from", "+15550001", "--to", "+15557777", "--message", "hi", "--yes", "--json", "--db", db)
	if err != nil {
		t.Fatalf("send failed: %v %s", err, out)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("POST /messages hits = %d, want 1", got)
	}
	var pf sendPreflight
	if err := json.Unmarshal([]byte(out[strings.Index(out, "{"):]), &pf); err != nil {
		t.Fatalf("parse output: %v\n%s", err, out)
	}
	if !pf.Sent || pf.MessageID != "msg-1" || len(pf.Recent) != 2 {
		t.Fatalf("unexpected result: %+v", pf)
	}
}

func TestNumbersAuditFlagsUnwiredNumber(t *testing.T) {
	testenv.Isolate(t)
	db := seedMirror(t)
	out, err := runCLI(t, "numbers", "audit", "--json", "--db", db)
	if err != nil {
		t.Fatalf("audit: %v %s", err, out)
	}
	var rows []auditRow
	if err := json.Unmarshal([]byte(out[strings.Index(out, "["):]), &rows); err != nil {
		t.Fatalf("parse: %v\n%s", err, out)
	}
	if len(rows) != 1 || rows[0].PhoneNumber != "+15550009" || len(rows[0].Problems) != 2 {
		t.Fatalf("want only +15550009 with 2 problems, got %+v", rows)
	}
}

func TestSpendTotalsUseRequestedWindow(t *testing.T) {
	testenv.Isolate(t)
	db := seedMirror(t)
	out, err := runCLI(t, "spend", "--since", "10d", "--group-by", "direction", "--json", "--db", db)
	if err != nil {
		t.Fatalf("spend: %v %s", err, out)
	}
	var v spendView
	if err := json.Unmarshal([]byte(out[strings.Index(out, "{"):]), &v); err != nil {
		t.Fatalf("parse: %v\n%s", err, out)
	}
	if v.Messages != 4 || v.TotalCost != 0.011 {
		t.Fatalf("messages=%d total=%v, want 4 and 0.011", v.Messages, v.TotalCost)
	}
	if v.CoveredDays < 9.9 || v.DailyAvg != round4(0.011/v.CoveredDays) {
		t.Fatalf("daily avg must use the 10d window: covered=%v avg=%v", v.CoveredDays, v.DailyAvg)
	}
}

func TestInboxUnansweredAndFailures(t *testing.T) {
	testenv.Isolate(t)
	db := seedMirror(t)
	out, err := runCLI(t, "inbox", "unanswered", "--json", "--db", db)
	if err != nil {
		t.Fatalf("inbox: %v %s", err, out)
	}
	// +15557777 got a reply before texting back (still waiting); +15558888 was
	// answered (outbound after inbound, even though delivery failed).
	if !strings.Contains(out, "+15557777") || strings.Contains(out, "+15558888") {
		t.Fatalf("unexpected unanswered set: %s", out)
	}
	out, err = runCLI(t, "failures", "--json", "--db", db)
	if err != nil {
		t.Fatalf("failures: %v %s", err, out)
	}
	if !strings.Contains(out, `"total_failed": 1`) || !strings.Contains(out, "+15558888") {
		t.Fatalf("unexpected failures: %s", out)
	}
}

func TestWithDefaultRecordTypeNullFilter(t *testing.T) {
	cases := map[string]string{
		"":                       `{"record_type":"messaging"}`,
		"null":                   `{"record_type":"messaging"}`,
		`{"status":"delivered"}`: `{"record_type":"messaging","status":"delivered"}`,
		`{"record_type":"call"}`: `{"record_type":"call"}`,
		"not json":               "not json",
	}
	for in, want := range cases {
		if got := withDefaultRecordType(in); got != want {
			t.Errorf("withDefaultRecordType(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestWithAllPageParamsStartsFromRequestedPage(t *testing.T) {
	got := withAllPageParams(true, `{"number":3,"size":20}`, map[string]string{"sort": "x"})
	if got["page[number]"] != "3" || got["page[size]"] != "20" || got["sort"] != "x" {
		t.Fatalf("unexpected params: %v", got)
	}
	if got := withAllPageParams(false, `{"number":3}`, map[string]string{}); len(got) != 0 {
		t.Fatalf("without --all params must be unchanged, got %v", got)
	}
}
