// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: saved delivery addresses (read-only UserAddresses query) for
// `address list` and the addressId auto-fill on CartSignaledEta.
//
// Shape (sameday.costco.com public rspack chunk 95722, module
// UserAddressManager/AddressSelection/queries.ts; operation:"query", no
// variables): data.userAddresses[] {id, postalCode, streetAddress,
// apartmentNumber, viewSection{lineOneString, lineTwoString, cityStateString}}.
// The store header passes the user location's addressId to CartSignaledEta
// {cartId, subtotal, shopId, addressId, postalCode, pageType}.
//
// Addresses are never written to the cache or state dir.

package client

import (
	"context"
	"fmt"
	"strings"
	"unicode"
)

const userAddressesOperation = "UserAddresses"

// SavedAddress is one saved delivery address. Street is kept only so callers
// can render a masked label; never print it.
type SavedAddress struct {
	ID         string
	PostalCode string
	Street     string
}

// UserAddresses lists the signed-in user's saved delivery addresses.
func (c *Client) UserAddresses(ctx context.Context) ([]SavedAddress, error) {
	payload, err := c.graphQLGet(withSessionResolving(ctx), userAddressesOperation, map[string]any{}, false)
	if err != nil {
		return nil, err
	}
	raw, _ := digJSON(payload, "data", "userAddresses").([]any)
	var out []SavedAddress
	for _, a := range raw {
		id := jsonString(digJSON(a, "id"))
		if id == "" {
			continue
		}
		out = append(out, SavedAddress{
			ID:         id,
			PostalCode: jsonString(digJSON(a, "postalCode")),
			Street:     jsonString(digJSON(a, "streetAddress")),
		})
	}
	return out, nil
}

// AmbiguousAddressError reports several saved addresses sharing the ZIP. A
// ZIP cannot tell them apart, so nothing is picked automatically.
type AmbiguousAddressError struct {
	PostalCode string
	Candidates []SavedAddress
}

func (e *AmbiguousAddressError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d saved addresses match ZIP %s; pass --address-id with one of:", len(e.Candidates), maskZIP(e.PostalCode))
	for _, a := range e.Candidates {
		fmt.Fprintf(&b, "\n  %s  %s", a.ID, MaskedAddressLabel(a))
	}
	b.WriteString("\n(run 'costco-sameday-pp-cli address list' to see all saved addresses)")
	return b.String()
}

// PickAddressID chooses the saved address for zip: the single one whose
// postal code matches, else the only saved address. Several matches return
// an *AmbiguousAddressError (never an arbitrary pick); no safe choice
// returns "" with a nil error.
func PickAddressID(addrs []SavedAddress, zip string) (string, string, error) {
	zip = strings.TrimSpace(zip)
	var match []SavedAddress
	for _, a := range addrs {
		if zip != "" && a.PostalCode == zip {
			match = append(match, a)
		}
	}
	switch {
	case len(match) == 1:
		return match[0].ID, "zip-match", nil
	case len(match) > 1:
		return "", "", &AmbiguousAddressError{PostalCode: zip, Candidates: match}
	case len(addrs) == 1:
		return addrs[0].ID, "only-address", nil
	}
	return "", "", nil
}

func maskZIP(z string) string {
	if len(z) < 3 {
		return z
	}
	return z[:3] + strings.Repeat("x", len(z)-3)
}

// MaskedAddressLabel renders a street as a recognisable but masked label:
// the house number keeps its first digit and each word keeps its first
// letter ("1234 Maple Ave" -> "1*** M**** A**"); ZIP keeps three digits.
func MaskedAddressLabel(a SavedAddress) string {
	var words []string
	for _, w := range strings.Fields(a.Street) {
		r := []rune(w)
		if len(r) == 0 {
			continue
		}
		masked := []rune{r[0]}
		for _, ch := range r[1:] {
			if unicode.IsLetter(ch) || unicode.IsDigit(ch) {
				masked = append(masked, '*')
			} else {
				masked = append(masked, ch)
			}
		}
		words = append(words, string(masked))
	}
	label := strings.Join(words, " ")
	if z := a.PostalCode; len(z) >= 3 {
		zipMask := maskZIP(z)
		if label == "" {
			return zipMask
		}
		label += ", " + zipMask
	}
	return label
}
