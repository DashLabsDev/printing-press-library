// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.

package walmart

import (
	"encoding/json"
	"strings"
	"testing"
)

// All values below are synthetic placeholders.
const (
	fakeCart  = "00000000-0000-4000-8000-000000000000"
	fakeOffer = "0123456789ABCDEF0123456789ABCDEF"
	fakeSlot  = "11111111-1111-4111-8111-111111111111-2030-01-02"
)

const fakeSlotsJSON = `{"data":{"slots":{"selectedSlotId":null,"slotDays":[
 {"day":"2030-01-02","hasAvailableSlots":true,"eachDaySlots":[
  {"__typename":"RegularSlot","id":"` + fakeSlot + `","available":true,"startTime":"2030-01-02T10:00:00-06:00","endTime":"2030-01-02T11:00:00-06:00",
   "slotExpiryTime":"2030-01-02T09:00:00-06:00","slotMetadata":"fake-metadata","accessPointId":"22222222-2222-4222-8222-222222222222",
   "fulfillmentType":"INSTORE_PICKUP","nodeAccessType":"PICKUP_CURBSIDE","isPrimary":true,"isAlcoholRestricted":false,"isPopular":null,
   "ineligibleItemCount":0,"pharmacyIneligibleItemCount":0,"serviceType":null,"slotIndicator":null,"statusReason":null,
   "price":{"baseFee":{"displayValue":"$0"},"expressFee":null,"memberBaseFee":null,"total":{"value":0,"displayValue":"$0"},"optedInTotal":null,"memberExpressDiscount":null,"originaltotal":{"value":0,"displayValue":"$0"},"totalSavings":{"displayValue":"","value":null}}},
  {"__typename":"RegularSlot","id":"unavailable-slot","available":false,"startTime":"2030-01-02T10:30:00-06:00","endTime":"2030-01-02T11:30:00-06:00","slotMetadata":"x","price":{"total":{"value":0,"displayValue":"$0"}}},
  {"__typename":"ExpressSlot","id":"express-slot","available":true,"startTime":"2030-01-02T12:00:00-06:00","endTime":"2030-01-02T13:00:00-06:00","slotMetadata":"x"}
 ]}]}}}`


func TestTemplatesCarryNoCapturedIdentifiers(t *testing.T) {
	for _, n := range []string{OpGetSlots} {
		b, err := templates.ReadFile("templates/" + n + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), "{{CART_ID}}") {
			t.Errorf("%s: cart id must be a placeholder", n)
		}
	}
}

func TestSlotsVariablesCurrentModeOnly(t *testing.T) {
	s, err := SlotsVariables(fakeCart, "PICKUP")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	_ = json.Unmarshal([]byte(s), &m)
	if m["fulfillmentOption"] != "PICKUP" || m["cartFulfillmentOption"] != "PICKUP" || m["cartId"] != fakeCart {
		t.Fatalf("bad vars %v", m)
	}
	if _, err := SlotMode("SHIPPING"); err == nil {
		t.Error("unknown modes are rejected")
	}
	if _, err := SlotMode(""); err == nil {
		t.Error("empty mode is rejected")
	}
}

func TestParseSlotsAndTimeWindow(t *testing.T) {
	v, err := ParseSlots([]byte(fakeSlotsJSON), "PICKUP")
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Days) != 1 || len(v.Days[0].Slots) != 3 {
		t.Fatalf("got %+v", v)
	}
	s := v.Days[0].Slots[0]
	if s.Window != "10am-11am" || !s.Available || s.Fee != "$0" || s.ID != fakeSlot {
		t.Fatalf("slot %+v", s)
	}
	if w := v.Days[0].Slots[1].Window; w != "10:30am-11:30am" {
		t.Errorf("window %q", w)
	}
	if strings.Contains(fakeSlotsJSON, "address") {
		t.Fatal("fixture must not carry addresses")
	}
}


func TestParseCartFromMutationResponses(t *testing.T) {
	line := `{"quantity":2,"product":{"usItemId":"10000000001","offerId":"` + fakeOffer + `","name":"Example Item"},"priceInfo":{"linePrice":{"value":5.5,"displayValue":"$5.50"},"unitPrice":{"value":2.75,"displayValue":"$2.75"}}}`
	for _, key := range []string{"cart", "updateItems", "reserveSlot"} {
		raw := `{"data":{"` + key + `":{"id":"` + fakeCart + `","checkoutable":true,"customer":{"firstName":"Placeholder"},"lineItems":[` + line + `],
		"fulfillment":{"intent":"PICKUP","storeId":9999,"deliveryAddress":{"addressLineOne":"1 Placeholder St"},
		"reservation":{"expiryTime":"2030-01-02T09:00:00-06:00","expired":false,"reservedSlot":{"startTime":"2030-01-02T10:00:00-06:00","endTime":"2030-01-02T11:00:00-06:00"}}},
		"priceDetails":{"subTotal":{"value":5.5},"grandTotal":{"value":6}}}}}`
		v, err := ParseCart([]byte(raw))
		if err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		if v.CartID != fakeCart || v.ItemCount != 2 || v.Items[0].OfferID != fakeOffer || v.Items[0].UnitPrice != "$2.75" || !v.SlotReserved ||
			v.Reservation.Window != "10am-11am" || v.StoreID != "9999" || v.Total != 6 {
			t.Fatalf("%s: %+v", key, v)
		}
		if l := v.FindLine("10000000001"); l == nil || l.Quantity != 2 {
			t.Fatalf("%s: FindLine", key)
		}
		b, _ := json.Marshal(v)
		if strings.Contains(string(b), "Placeholder") {
			t.Fatalf("%s: customer/address data leaked into output", key)
		}
	}
}

func TestGetSlotsTemplate(t *testing.T) {
	if _, err := SlotsVariables("00000000-0000-4000-8000-000000000001", "PICKUP"); err != nil {
		t.Fatal(err)
	}
}
