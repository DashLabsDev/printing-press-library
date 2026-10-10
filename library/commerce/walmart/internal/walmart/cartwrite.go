// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.

package walmart

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Cart and slot READ operations captured from the walmart.com cart page.
// getSlots is a GET. Cart mutations (updateItems, reserveSlotMutation) are
// never sent by this CLI — cart changes are link-only via affil.walmart.com.
// Hashes rotate on Walmart deploys; override with WALMART_HASH_<OPERATION>.
const (
	SubgraphCartXO      = "cartxo"
	OpGetSlots          = "getSlots"
	HashGetSlots        = "439ba3e04fdfda26874c6bfafbf301e684b674d819a4c7d9b03e0c348d9cdf18"
	MaxCartLineQuantity = 99
)

// SlotHeaders are the extra headers the cart page sends with getSlots.
func SlotHeaders() map[string]string {
	return map[string]string{"x-o-default-slot-intent": "true", "x-o-in-home-slots": "true"}
}

// ValidCartID reports whether s looks like a Walmart cart id.
func ValidCartID(s string) bool { return cartIDRe.MatchString(strings.TrimSpace(s)) }

// SlotMode returns the getSlots fulfillment option for the cart's current
// intent. Only the current mode is ever queried; nothing switches it.
func SlotMode(intent string) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(intent)) {
	case "PICKUP":
		return "PICKUP", nil
	case "DELIVERY":
		return "DELIVERY", nil
	case "":
		return "", errors.New("the cart did not report a pickup/delivery mode; choose one on walmart.com first")
	default:
		return "", fmt.Errorf("cart mode %q has no time slots in this CLI (only PICKUP and DELIVERY)", intent)
	}
}

// SlotsVariables builds getSlots variables for the cart in its current mode.
func SlotsVariables(cartID, mode string) (string, error) {
	if !ValidCartID(cartID) {
		return "", errors.New("a cart id (36-character UUID) is required")
	}
	m, err := SlotMode(mode)
	if err != nil {
		return "", err
	}
	return fillTemplate(OpGetSlots, map[string]string{"CART_ID": strings.TrimSpace(cartID), "MODE": m})
}

type Slot struct {
	ID              string  `json:"id"`
	Day             string  `json:"day"`
	Start           string  `json:"start"`
	End             string  `json:"end"`
	Window          string  `json:"window"`
	Available       bool    `json:"available"`
	Fee             string  `json:"fee,omitempty"`
	FeeValue        float64 `json:"feeValue"`
	Kind            string  `json:"kind,omitempty"`
	AccessType      string  `json:"accessType,omitempty"`
	FulfillmentType string  `json:"fulfillmentType,omitempty"`
	HoldUntil       string  `json:"holdUntil,omitempty"`
	Popular         bool    `json:"popular,omitempty"`
}

// SlotDay groups slots by date.
type SlotDay struct {
	Day          string `json:"day"`
	HasAvailable bool   `json:"hasAvailable"`
	Slots        []Slot `json:"slots"`
}

// SlotsView is a parsed getSlots response. Pickup store address, delivery
// address and membership data are never decoded.
type SlotsView struct {
	Mode           string    `json:"mode"`
	SelectedSlotID string    `json:"selectedSlotId,omitempty"`
	Days           []SlotDay `json:"days"`
}

type slotsEnvelope struct {
	Data struct {
		Slots *struct {
			SelectedSlotID *string `json:"selectedSlotId"`
			SlotDays       []struct {
				Day               string           `json:"day"`
				HasAvailableSlots bool             `json:"hasAvailableSlots"`
				EachDaySlots      []map[string]any `json:"eachDaySlots"`
			} `json:"slotDays"`
		} `json:"slots"`
	} `json:"data"`
}

func decodeSlots(raw []byte) (*slotsEnvelope, error) {
	var env slotsEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("walmart: decoding slots: %w", err)
	}
	if env.Data.Slots == nil {
		if err := gqlErrors(raw); err != nil {
			return nil, err
		}
		return nil, errors.New("walmart: slots missing from response")
	}
	return &env, nil
}

// ParseSlots parses a getSlots response.
func ParseSlots(raw []byte, mode string) (*SlotsView, error) {
	env, err := decodeSlots(raw)
	if err != nil {
		return nil, err
	}
	v := &SlotsView{Mode: mode, Days: []SlotDay{}}
	if env.Data.Slots.SelectedSlotID != nil {
		v.SelectedSlotID = *env.Data.Slots.SelectedSlotID
	}
	for _, d := range env.Data.Slots.SlotDays {
		day := SlotDay{Day: d.Day, HasAvailable: d.HasAvailableSlots, Slots: []Slot{}}
		for _, s := range d.EachDaySlots {
			day.Slots = append(day.Slots, slotFromRaw(s, d.Day))
		}
		v.Days = append(v.Days, day)
	}
	return v, nil
}

func str(m map[string]any, k string) string {
	if s, ok := m[k].(string); ok {
		return s
	}
	return ""
}

func boolean(m map[string]any, k string) bool {
	b, _ := m[k].(bool)
	return b
}

func money(m map[string]any, k string) (string, float64) {
	sub, _ := m[k].(map[string]any)
	if sub == nil {
		return "", 0
	}
	f, _ := sub["value"].(float64)
	return str(sub, "displayValue"), f
}

func slotFromRaw(s map[string]any, day string) Slot {
	out := Slot{
		ID: str(s, "id"), Day: day, Start: str(s, "startTime"), End: str(s, "endTime"),
		Available: boolean(s, "available"), Kind: str(s, "__typename"), AccessType: str(s, "nodeAccessType"),
		FulfillmentType: str(s, "fulfillmentType"), HoldUntil: str(s, "slotExpiryTime"), Popular: boolean(s, "isPopular"),
	}
	out.Window = TimeWindow(out.Start, out.End)
	if price, ok := s["price"].(map[string]any); ok {
		out.Fee, out.FeeValue = money(price, "total")
	}
	return out
}

// TimeWindow formats a slot's start/end in the slot's own time zone the way
// the web app labels it ("10am-11am", "10:30am-11am").
func TimeWindow(start, end string) string {
	a, err1 := time.Parse(time.RFC3339, start)
	b, err2 := time.Parse(time.RFC3339, end)
	if err1 != nil || err2 != nil {
		return ""
	}
	return clock(a) + "-" + clock(b)
}

func clock(t time.Time) string {
	if t.Minute() == 0 {
		return strings.ToLower(t.Format("3PM"))
	}
	return strings.ToLower(t.Format("3:04PM"))
}

func GQLErrors(raw []byte) error { return gqlErrors(raw) }
