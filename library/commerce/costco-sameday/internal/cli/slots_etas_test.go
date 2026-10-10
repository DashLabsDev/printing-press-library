// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored tests for `slots etas`, the AvailableServices empty note and
// the guest/member probe (synthetic fixtures and fake values only).

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// runRootSplit runs the CLI with separate stdout and stderr buffers.
func runRootSplit(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	root := RootCmd()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(append(args, "--no-cache"))
	err := root.Execute()
	return stdout.String(), stderr.String(), err
}

const fakeShops = `{"data":{"shopCollection":{"shops":[` +
	`{"id":"shop-d-fake","serviceType":"delivery","retailerLocationId":"loc-d-fake","retailerId":"ret-fake"},` +
	`{"id":"shop-p-fake","serviceType":"pickup","retailerLocationId":"loc-p-fake","retailerId":"ret-fake"}]}}}`

const fakeDeliveryEtas = `{"data":{"getAccurateRetailerEtas":[` +
	`{"retailerId":"ret-other","retailerLocationId":"loc-x","etaSeconds":60,"viewSection":{"etaString":"other store"}},` +
	`{"retailerId":"ret-fake","retailerLocationId":"loc-d-fake","etaSeconds":5400,"viewSection":{"etaString":"by 1:30pm","deliveryFeeString":"$3.99 delivery","trackingProperties":{"eta_id":"trk-fake"}}}]}}`

const fakePickupEtas = `{"data":{"pickupEtas":[{"retailerId":"ret-fake","retailerLocationId":"loc-p-fake","etaSeconds":7200,"viewSection":{"pickupString":"Ready by 2pm"}}]}}`

func etaOutput(t *testing.T, out string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if d, ok := m["data"].(map[string]any); ok && m["services"] == nil {
		m = d
	}
	return m
}

func assertNoFakeIDs(t *testing.T, out string) {
	t.Helper()
	for _, id := range []string{"ret-fake", "loc-d-fake", "loc-p-fake", "shop-d-fake", "shop-p-fake", "trk-fake", "ret-other"} {
		if strings.Contains(out, id) {
			t.Fatalf("output leaks id %q:\n%s", id, out)
		}
	}
}

func TestSlotsEtasGuestUsesVisitorQueries(t *testing.T) {
	rec := routedSameDay(t, withReplies(map[string]string{
		"ShopCollectionScoped":                 fakeShops,
		"UserType":                             `{"data":{"currentUser":{"guest":true}}}`,
		"GetAccurateVisitorRetailerEtas":       fakeDeliveryEtas,
		"GetAccurateVisitorRetailerPickupEtas": fakePickupEtas,
	}))
	out, _, err := runRootSplit(t, "slots", "etas", "--zip", "98027", "--json")
	if err != nil {
		t.Fatalf("etas: %v\n%s", err, out)
	}
	dv := rec.vars["GetAccurateVisitorRetailerEtas"]
	if dv["postalCode"] != "98027" || dv["serviceType"] != "DELIVERY" {
		t.Fatalf("visitor delivery vars = %v", dv)
	}
	if !reflect.DeepEqual(dv["retailerIds"], []any{"ret-fake"}) || !reflect.DeepEqual(dv["explicitRetailerLocationIds"], []any{"loc-d-fake"}) {
		t.Fatalf("visitor delivery ids = %v", dv)
	}
	if pv := rec.vars["GetAccurateVisitorRetailerPickupEtas"]; !reflect.DeepEqual(pv["shopIds"], []any{"shop-p-fake"}) {
		t.Fatalf("visitor pickup vars = %v", pv)
	}
	if _, called := rec.vars["GetAccurateRetailerEtas"]; called {
		t.Fatal("guest session must not call the member delivery query")
	}
	m := etaOutput(t, out)
	if m["account_type"] != "guest" {
		t.Fatalf("account_type = %v", m["account_type"])
	}
	services, _ := m["services"].([]any)
	if len(services) != 2 {
		t.Fatalf("services = %v", m["services"])
	}
	d := services[0].(map[string]any)
	if d["service_type"] != "delivery" || d["label"] != "by 1:30pm" || d["fee"] != "$3.99 delivery" || d["eta_seconds"] != float64(5400) || d["earliest_start"] == "" {
		t.Fatalf("delivery row = %v", d)
	}
	p := services[1].(map[string]any)
	if p["service_type"] != "pickup" || p["label"] != "Ready by 2pm" {
		t.Fatalf("pickup row = %v", p)
	}
	assertNoFakeIDs(t, out)
}

func TestSlotsEtasMemberUsesMemberQueries(t *testing.T) {
	rec := routedSameDay(t, withReplies(map[string]string{
		"ShopCollectionScoped":          fakeShops,
		"UserType":                      `{"data":{"currentUser":{"guest":false}}}`,
		"GetAccurateRetailerEtas":       fakeDeliveryEtas,
		"GetAccurateRetailerPickupEtas": fakePickupEtas,
	}))
	out, _, err := runRootSplit(t, "slots", "etas", "--zip", "10001", "--json")
	if err != nil {
		t.Fatalf("etas: %v\n%s", err, out)
	}
	mv := rec.vars["GetAccurateRetailerEtas"]
	if mv["postalCode"] != "10001" || mv["serviceType"] != "DELIVERY" || mv["homeLoadUuid"] == "" || !reflect.DeepEqual(mv["retailerIds"], []any{"ret-fake"}) {
		t.Fatalf("member delivery vars = %v", mv)
	}
	if _, ok := mv["addressId"]; !ok {
		t.Fatalf("member delivery vars miss addressId: %v", mv)
	}
	if pv := rec.vars["GetAccurateRetailerPickupEtas"]; !reflect.DeepEqual(pv["shopIds"], []any{"shop-p-fake"}) {
		t.Fatalf("member pickup vars = %v", pv)
	}
	if m := etaOutput(t, out); m["account_type"] != "member" {
		t.Fatalf("account_type = %v", m["account_type"])
	}
	assertNoFakeIDs(t, out)
}

func TestSlotsEtasAsOverrideSkipsProbe(t *testing.T) {
	rec := routedSameDay(t, withReplies(map[string]string{
		"ShopCollectionScoped":           fakeShops,
		"GetAccurateVisitorRetailerEtas": fakeDeliveryEtas,
	}))
	out, _, err := runRootSplit(t, "slots", "etas", "--zip", "98027", "--as", "guest", "--service", "delivery", "--json")
	if err != nil {
		t.Fatalf("etas: %v\n%s", err, out)
	}
	for _, op := range accountTypeProbeOperations {
		if _, called := rec.vars[op]; called {
			t.Fatalf("--as guest must not probe (%s called)", op)
		}
	}
	if _, called := rec.vars["GetAccurateVisitorRetailerPickupEtas"]; called {
		t.Fatal("--service delivery must not query pickup")
	}
}

func TestSlotsEtasUnknownFallsBackToVisitor(t *testing.T) {
	rec := routedSameDay(t, withReplies(map[string]string{
		"ShopCollectionScoped":           fakeShops,
		"GetAccurateVisitorRetailerEtas": fakeDeliveryEtas,
	}))
	out, _, err := runRootSplit(t, "slots", "etas", "--zip", "98027", "--service", "delivery", "--json")
	if err != nil {
		t.Fatalf("etas: %v\n%s", err, out)
	}
	if _, called := rec.vars["GetAccurateRetailerEtas"]; !called {
		t.Fatal("unknown account type should try the member query first")
	}
	m := etaOutput(t, out)
	if m["account_type"] != "unknown" || len(m["services"].([]any)) != 1 {
		t.Fatalf("fallback output = %v", m)
	}
}

func TestSlotsEtasRetailerIDFromSlugThenAssumed(t *testing.T) {
	noRetailer := `{"data":{"shopCollection":{"shops":[{"id":"shop-d-fake","serviceType":"delivery","retailerLocationId":"loc-d-fake"}]}}}`
	rec := routedSameDay(t, withReplies(map[string]string{
		"ShopCollectionScoped":           noRetailer,
		"GetRetailerIdBySlug":            `{"data":{"retailerBySlug":{"id":"ret-slug-fake"}}}`,
		"GetAccurateVisitorRetailerEtas": `{"data":{"getAccurateRetailerEtas":[]}}`,
	}))
	if out, _, err := runRootSplit(t, "slots", "etas", "--zip", "98027", "--as", "guest", "--json"); err != nil {
		t.Fatalf("etas: %v\n%s", err, out)
	}
	if got := rec.vars["GetAccurateVisitorRetailerEtas"]["retailerIds"]; !reflect.DeepEqual(got, []any{"ret-slug-fake"}) {
		t.Fatalf("retailerIds from slug = %v", got)
	}

	rec = routedSameDay(t, withReplies(map[string]string{
		"ShopCollectionScoped":           noRetailer,
		"GetAccurateVisitorRetailerEtas": `{"data":{"getAccurateRetailerEtas":[]}}`,
	}))
	out, _, err := runRootSplit(t, "slots", "etas", "--zip", "98027", "--as", "guest", "--json")
	if err != nil {
		t.Fatalf("etas: %v\n%s", err, out)
	}
	if got := rec.vars["GetAccurateVisitorRetailerEtas"]["retailerIds"]; !reflect.DeepEqual(got, []any{"5"}) {
		t.Fatalf("assumed retailerIds = %v", got)
	}
	m := etaOutput(t, out)
	if m["retailer_id_source"] != "assumed" || !strings.Contains(out, "pickup: no Costco pickup shop") || !strings.Contains(out, "returned no ETA") {
		t.Fatalf("assumed output = %s", out)
	}
}

func TestSlotsEtasRejectsBadFlags(t *testing.T) {
	routedSameDay(t, sessionReplies)
	if _, _, err := runRootSplit(t, "slots", "etas", "--service", "boat"); err == nil {
		t.Fatal("bad --service accepted")
	}
	if _, _, err := runRootSplit(t, "slots", "etas", "--as", "admin"); err == nil {
		t.Fatal("bad --as accepted")
	}
}

func TestAvailableServicesEmptyWarnsAndNotes(t *testing.T) {
	routedSameDay(t, withReplies(map[string]string{
		"AvailableServices": `{"data":{"checkoutAvailableServices":{"services":[]}}}`,
	}))
	out, errOut, err := runRootSplit(t, "slots", "availableservices", "--zip", "10001", "--json")
	if err != nil {
		t.Fatalf("availableservices: %v\n%s", err, out)
	}
	if !strings.Contains(errOut, "warning:") || !strings.Contains(errOut, "slots etas") {
		t.Fatalf("stderr warning missing: %q", errOut)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("not JSON: %s", out)
	}
	if note, _ := m["note"].(string); !strings.Contains(note, "checkout service picker") || !strings.Contains(note, "slots etas") {
		t.Fatalf("JSON note = %v", m["note"])
	}
}

func TestAvailableServicesNonEmptyHasNoNote(t *testing.T) {
	routedSameDay(t, withReplies(map[string]string{
		"AvailableServices": `{"data":{"checkoutAvailableServices":{"services":[{"serviceType":"delivery"}]}}}`,
	}))
	out, errOut, err := runRootSplit(t, "slots", "availableservices", "--zip", "10001", "--json")
	if err != nil {
		t.Fatalf("availableservices: %v\n%s", err, out)
	}
	if strings.Contains(errOut, "warning:") || strings.Contains(out, `"note"`) {
		t.Fatalf("unexpected note: stdout=%s stderr=%s", out, errOut)
	}
}

func TestAvailableServicesHelpNamesCheckoutPicker(t *testing.T) {
	routedSameDay(t, sessionReplies)
	out, _, err := runRootSplit(t, "slots", "availableservices", "--help")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "checkout service picker") || !strings.Contains(out, "slots etas") {
		t.Fatalf("help = %s", out)
	}
}

func TestClassifyAccountPayload(t *testing.T) {
	cases := []struct {
		op, body, want, wantSource string
	}{
		{"UserType", `{"data":{"currentUser":{"guest":true}}}`, "guest", "UserType"},
		{"UserType", `{"data":{"currentUser":{"isGuest":false}}}`, "member", "UserType"},
		{"UserType", `{"data":{"currentUser":{"userType":"IMPLICIT_GUEST"}}}`, "guest", "UserType"},
		{"UserType", `{"data":{"currentUser":{"userType":"REGISTERED"}}}`, "member", "UserType"},
		{"GetCurrentUserName", `{"data":{"currentUser":{"firstName":"Fakename"}}}`, "member", "GetCurrentUserName"},
		{"HouseholdByUser", `{"data":{"householdByUser":{"__typename":"HouseholdsNotFound"}}}`, "guest", "HouseholdByUser (weak)"},
		{"HouseholdByUser", `{"data":{"householdByUser":{"household":{"id":"hh-fake"}}}}`, "member", "HouseholdByUser"},
		{"GetCurrentUserName", `{"data":{"currentUser":null}}`, "unknown", ""},
	}
	for _, tc := range cases {
		var p map[string]any
		if err := json.Unmarshal([]byte(tc.body), &p); err != nil {
			t.Fatal(err)
		}
		got := classifyAccountPayload(tc.op, p)
		if got.Type != tc.want || got.Source != tc.wantSource {
			t.Fatalf("%s %s = %+v, want %s/%s", tc.op, tc.body, got, tc.want, tc.wantSource)
		}
		if strings.Contains(got.Evidence, "Fakename") || strings.Contains(got.Evidence, "hh-fake") {
			t.Fatalf("evidence leaks a value: %q", got.Evidence)
		}
	}
}

func TestProbePrefersStrongOverWeak(t *testing.T) {
	routedSameDay(t, map[string]string{
		"HouseholdByUser":    `{"data":{"householdByUser":{"__typename":"HouseholdsNotFound"}}}`,
		"GetCurrentUserName": `{"data":{"currentUser":{"firstName":"Fakename"}}}`,
	})
	out, _, err := runRootSplit(t, "session", "context", "--zip", "98027", "--cart=false", "--account-type", "--json")
	if err != nil && !strings.Contains(out, "account_type") {
		t.Fatalf("session context: %v\n%s", err, out)
	}
	if !strings.Contains(out, `"account_type": "member"`) && !strings.Contains(out, `"account_type":"member"`) {
		t.Fatalf("expected member from GetCurrentUserName, got %s", out)
	}
	if strings.Contains(out, "Fakename") {
		t.Fatal("output leaks the user name")
	}
}

func TestCookieNameHintsNamesOnly(t *testing.T) {
	h := cookieNameHintsFrom("Cookie: __Host-instacart_sid=FAKESID; WC_AUTHENTICATION_123=FAKEAUTH; ahoy_visit=FAKEV; WC_USERACTIVITY_123=FAKEUA")
	if !h.SessionCookie || h.CookieCount != 4 || !reflect.DeepEqual(h.MemberCookieNames, []string{"WC_AUTHENTICATION_123", "WC_USERACTIVITY_123"}) {
		t.Fatalf("hints = %+v", h)
	}
	b, _ := json.Marshal(h)
	if strings.Contains(string(b), "FAKE") {
		t.Fatalf("hints leak values: %s", b)
	}
}

func TestAuthStatusReportsAccountType(t *testing.T) {
	routedSameDay(t, map[string]string{
		"UserType": `{"data":{"currentUser":{"guest":true}}}`,
	})
	writeFakeLogin(t)
	out, _, err := runRootSplit(t, "auth", "status", "--json")
	if err != nil {
		t.Fatalf("status: %v\n%s", err, out)
	}
	if strings.Contains(out, "FAKESIDVALUE") || strings.Contains(out, "FAKEVISIT") {
		t.Fatalf("status leaks cookie values: %s", out)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("not JSON: %s", out)
	}
	if m["account_type"] != "guest" || m["account_type_source"] != "UserType" {
		t.Fatalf("status = %v", m)
	}
	if c, _ := m["cookies"].(map[string]any); c == nil || c["session_cookie_present"] != true {
		t.Fatalf("cookie hints = %v", m["cookies"])
	}

	human, _, err := runRootSplit(t, "auth", "status", "--no-probe")
	if err != nil {
		t.Fatalf("status --no-probe: %v", err)
	}
	if !strings.Contains(human, "Account: unknown") || !strings.Contains(human, "Session cookie (__Host-instacart_sid): present") {
		t.Fatalf("human status = %s", human)
	}
}

// writeFakeLogin saves a fake cookie session through auth login.
func writeFakeLogin(t *testing.T) {
	t.Helper()
	cookieFile := filepath.Join(t.TempDir(), "cookies.txt")
	if err := os.WriteFile(cookieFile, []byte("__Host-instacart_sid=FAKESIDVALUE; ahoy_visit=FAKEVISIT"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, _, err := runRootSplit(t, "auth", "login", "--cookies-file", cookieFile); err != nil {
		t.Fatalf("login: %v\n%s", err, out)
	}
}
