// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored tests for `slots windows`, `slots pick`, `address list`, the
// CartSignaledEta addressId/subtotal auto-fill and the ETA no-fee note.
// Local stub only; every value is fake.

package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/costco-sameday/internal/client"
)

func fmtStr(text string) string {
	return `{"__typename":"ViewFormattedAttributesString","sections":[{"text":"` + text + `"}]}`
}

func fakeLine(token, date, tm, price string) string {
	p := "null"
	if price != "" {
		p = fmtStr(price)
	}
	return `{"serviceOptionId":"so-fake","serviceOptionToken":"` + token + `","viewSection":{"postCheckoutDateString":"` + date +
		`","postCheckoutTimeString":"` + tm + `","postCheckoutWindowString":"` + date + `, ` + tm + `","priceFormattedAttributesString":` + p + `}}`
}

// Same structure as the live CheckoutPricedServiceOptionsForDateRange reply.
var fakeWindows = `{"data":{"checkoutPricedServiceOptionsForDateRange":{"pricedServiceOptions":{"__typename":"CheckoutPricedServiceOptionsPresentableFormat","presentableFormat":{"layeredOptions":[` +
	`{"layeredOption":{"__typename":"CheckoutPricedServiceOptionsSingleSelectionOption","disabled":false,"viewSection":{"lineOneFormattedAttributesString":` + fmtStr("Standard") + `},` +
	`"serviceOption":{"serviceOptionToken":"tok-std-fake","viewSection":{"postCheckoutDateString":"Today, Jan 1","postCheckoutTimeString":"1pm–2pm","priceFormattedAttributesString":null}}}},` +
	`{"layeredOption":{"__typename":"CheckoutPricedServiceOptionsScheduleSelectionOption","disabled":false,"viewSection":{"lineOneFormattedAttributesString":` + fmtStr("Schedule and save") + `},"dateGroups":[` +
	`{"serviceOptionLines":[` + fakeLine("tok-a-fake", "Tomorrow, Jan 2", "11am–1pm", "") + `,` + fakeLine("tok-b-fake", "Tomorrow, Jan 2", "11am–2pm", "$2 off order") + `]}]}}]}}}}}`

func TestParseDeliveryWindowsLiveShape(t *testing.T) {
	var payload any
	if err := json.Unmarshal([]byte(fakeWindows), &payload); err != nil {
		t.Fatal(err)
	}
	wins, _, err := parseDeliveryWindows(payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(wins) != 3 {
		t.Fatalf("windows = %+v", wins)
	}
	if wins[0].Option != "Standard" || wins[0].Window != "Today, Jan 1, 1pm–2pm" || wins[0].Token != "tok-std-fake" {
		t.Fatalf("standard = %+v", wins[0])
	}
	if wins[2].Option != "Schedule and save" || wins[2].Price != "$2 off order" || wins[2].Index != 3 {
		t.Fatalf("scheduled = %+v", wins[2])
	}
}

func TestParseDeliveryWindowsErrorTypename(t *testing.T) {
	var payload any
	_ = json.Unmarshal([]byte(`{"data":{"checkoutPricedServiceOptionsForDateRange":{"pricedServiceOptions":{"__typename":"CheckoutCheckoutErrorsServiceOptionError","viewSection":{"toastDescriptionString":"No windows fake"}}}}}`), &payload)
	if _, _, err := parseDeliveryWindows(payload); err == nil || !strings.Contains(err.Error(), "No windows fake") {
		t.Fatalf("err = %v", err)
	}
}

func TestSlotsWindowsReadOnlyHidesTokens(t *testing.T) {
	rec := routedSameDay(t, withReplies(map[string]string{windowsOperation: fakeWindows}))
	out, _, err := runRootSplit(t, "slots", "windows", "--checkout-session-id", "sess-fake", "--date", "2030-01-01", "--json")
	if err != nil {
		t.Fatalf("windows: %v\n%s", err, out)
	}
	if rec.methods[windowsOperation] != "GET" {
		t.Fatalf("method = %s", rec.methods[windowsOperation])
	}
	if v := rec.vars[windowsOperation]; v["checkoutSessionId"] != "sess-fake" || v["startDate"] != "2030-01-01" {
		t.Fatalf("vars = %v", v)
	}
	if strings.Contains(out, "tok-") {
		t.Fatalf("tokens printed without --show-tokens:\n%s", out)
	}
	if !strings.Contains(out, "11am–2pm") || !strings.Contains(out, "$2 off order") {
		t.Fatalf("windows missing:\n%s", out)
	}
	if _, called := rec.methods[updateCheckoutOp]; called {
		t.Fatal("windows must not call UpdateCheckout")
	}
	out, _, _ = runRootSplit(t, "slots", "windows", "--checkout-session-id", "sess-fake", "--show-tokens", "--json")
	if !strings.Contains(out, "tok-b-fake") {
		t.Fatalf("--show-tokens did not print tokens:\n%s", out)
	}
}

func TestSlotsWindowsNeedsSession(t *testing.T) {
	routedSameDay(t, sessionReplies)
	t.Setenv(checkoutSessionEnvName, "")
	if _, _, err := runRootSplit(t, "slots", "windows"); err == nil || !strings.Contains(err.Error(), "checkout-session-id") {
		t.Fatalf("err = %v", err)
	}
}

func TestSlotsPickGatedByYes(t *testing.T) {
	rec := routedSameDay(t, withReplies(map[string]string{windowsOperation: fakeWindows, updateCheckoutOp: `{"data":{"updateCheckout":{"__typename":"X"}}}`}))
	if _, _, err := runRootSplit(t, "slots", "pick", "--checkout-session-id", "sess-fake", "--index", "3"); err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("pick without --yes: %v", err)
	}
	if _, called := rec.methods[updateCheckoutOp]; called {
		t.Fatal("UpdateCheckout sent without --yes")
	}
	out, _, err := runRootSplit(t, "slots", "pick", "--checkout-session-id", "sess-fake", "--index", "3", "--dry-run")
	if err != nil || !strings.Contains(out, "NOT sent") || strings.Contains(out, "tok-") {
		t.Fatalf("dry-run: %v\n%s", err, out)
	}
	if _, called := rec.methods[updateCheckoutOp]; called {
		t.Fatal("UpdateCheckout sent on --dry-run")
	}
	out, _, err = runRootSplit(t, "slots", "pick", "--checkout-session-id", "sess-fake", "--index", "3", "--yes", "--json")
	if err != nil {
		t.Fatalf("pick --yes: %v\n%s", err, out)
	}
	if rec.methods[updateCheckoutOp] != "POST" || rec.query[updateCheckoutOp] != "" {
		t.Fatalf("UpdateCheckout method=%s query=%q", rec.methods[updateCheckoutOp], rec.query[updateCheckoutOp])
	}
	v := rec.vars[updateCheckoutOp]
	upd, _ := v["checkoutUpdates"].(map[string]any)
	so, _ := upd["serviceOptionUpdate"].(map[string]any)
	if v["checkoutSessionId"] != "sess-fake" || so["serviceOptionToken"] != "tok-b-fake" {
		t.Fatalf("UpdateCheckout vars = %v", v)
	}
	if strings.Contains(out, "tok-") {
		t.Fatalf("token printed:\n%s", out)
	}
	if _, called := rec.methods["FinalizeCheckout"]; called {
		t.Fatal("pick must never finalize")
	}
}

func TestSlotsPickRejectsBadIndex(t *testing.T) {
	routedSameDay(t, withReplies(map[string]string{windowsOperation: fakeWindows}))
	if _, _, err := runRootSplit(t, "slots", "pick", "--checkout-session-id", "sess-fake", "--index", "9", "--yes"); err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("err = %v", err)
	}
	if _, _, err := runRootSplit(t, "slots", "pick", "--checkout-session-id", "sess-fake", "--yes"); err == nil {
		t.Fatal("pick with neither --index nor --token should fail")
	}
}

const fakeAddresses = `{"data":{"userAddresses":[` +
	`{"id":"addr-1-fake","postalCode":"98027","streetAddress":"1234 Maple Ave"},` +
	`{"id":"addr-2-fake","postalCode":"10001","streetAddress":"55 Fake St"}]}}`

func TestMaskedAddressLabelHidesStreet(t *testing.T) {
	got := client.MaskedAddressLabel(client.SavedAddress{PostalCode: "98027", Street: "1234 Maple Ave"})
	if got != "1*** M**** A**, 980xx" {
		t.Fatalf("label = %q", got)
	}
}

func TestPickAddressID(t *testing.T) {
	addrs := []client.SavedAddress{{ID: "a1", PostalCode: "98027"}, {ID: "a2", PostalCode: "10001"}}
	if id, how, err := client.PickAddressID(addrs, "10001"); id != "a2" || how != "zip-match" || err != nil {
		t.Fatalf("zip match = %s %s %v", id, how, err)
	}
	if id, _, err := client.PickAddressID(addrs, "33333"); id != "" || err != nil {
		t.Fatalf("no match should be empty, got %s %v", id, err)
	}
	if id, how, err := client.PickAddressID(addrs[:1], "33333"); id != "a1" || how != "only-address" || err != nil {
		t.Fatalf("only = %s %s %v", id, how, err)
	}
}

func TestAddressListShowsIDsAndMaskedLabelOnly(t *testing.T) {
	routedSameDay(t, withReplies(map[string]string{"UserAddresses": fakeAddresses}))
	out, _, err := runRootSplit(t, "address", "list", "--zip", "10001", "--json")
	if err != nil {
		t.Fatalf("address list: %v\n%s", err, out)
	}
	for _, leak := range []string{"Maple", "Fake St", "1234"} {
		if strings.Contains(out, leak) {
			t.Fatalf("street leaked (%q):\n%s", leak, out)
		}
	}
	if !strings.Contains(out, "addr-1-fake") || !strings.Contains(out, "5* F*** S*, 100xx") {
		t.Fatalf("ids/labels missing:\n%s", out)
	}
	if !strings.Contains(out, `"auto_address_id": "addr-2-fake"`) && !strings.Contains(out, `"auto_address_id":"addr-2-fake"`) {
		t.Fatalf("auto pick wrong:\n%s", out)
	}
}

func TestCartSignaledEtaAutoFillsAddressAndSubtotal(t *testing.T) {
	rec := routedSameDay(t, withReplies(map[string]string{
		"UserAddresses":   fakeAddresses,
		"CartTotals":      `{"data":{"cartTotals":{"retailerTotal":{"amount":"12.34","currencyCode":"USD"}}}}`,
		"CartSignaledEta": `{"data":{"cartSignaledEta":{"viewSection":{"etaString":"1:00-1:30pm"}}}}`,
	}))
	out, _, err := runRootSplit(t, "cart", "cartsignaledeta", "--zip", "98027", "--json")
	if err != nil {
		t.Fatalf("cartsignaledeta: %v\n%s", err, out)
	}
	v := rec.vars["CartSignaledEta"]
	if v["addressId"] != "addr-1-fake" {
		t.Fatalf("addressId = %v (vars %v)", v["addressId"], v)
	}
	sub, _ := v["subtotal"].(map[string]any)
	if sub["amount"] != "12.34" || sub["currencyCode"] != "USD" {
		t.Fatalf("subtotal = %v", v["subtotal"])
	}
	// explicit flags win; a bare number becomes a USD money input
	_, _, _ = runRootSplit(t, "cart", "cartsignaledeta", "--zip", "98027", "--address-id", "addr-x-fake", "--subtotal", "50", "--json")
	v = rec.vars["CartSignaledEta"]
	sub, _ = v["subtotal"].(map[string]any)
	if v["addressId"] != "addr-x-fake" || sub["amount"] != "50" || sub["currencyCode"] != "USD" {
		t.Fatalf("explicit vars = %v", v)
	}
}

func TestSlotsEtasNotesMissingFee(t *testing.T) {
	routedSameDay(t, withReplies(map[string]string{
		"ShopCollectionScoped":    fakeShops,
		"UserType":                `{"data":{"currentUser":{"guest":false}}}`,
		"GetAccurateRetailerEtas": `{"data":{"getAccurateRetailerEtas":[{"retailerId":"ret-fake","etaSeconds":3141.8,"viewSection":{"etaString":"Delivery by 1:30pm","condensedEtaString":"By 1:30pm"}}]}}`,
	}))
	out, _, err := runRootSplit(t, "slots", "etas", "--zip", "98027", "--service", "delivery", "--json")
	if err != nil {
		t.Fatalf("etas: %v\n%s", err, out)
	}
	m := etaOutput(t, out)
	notes, _ := json.Marshal(m["notes"])
	if !strings.Contains(string(notes), "return no fee") {
		t.Fatalf("notes = %s", notes)
	}
	services, _ := m["services"].([]any)
	if len(services) != 1 || services[0].(map[string]any)["label"] != "Delivery by 1:30pm" {
		t.Fatalf("services = %v", m["services"])
	}
}
