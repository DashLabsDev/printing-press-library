// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/walmart/internal/client"
)

// All cookie values are fake placeholders.
func TestClassifyWalmartSession(t *testing.T) {
	now := time.Date(2030, 1, 2, 12, 0, 0, 0, time.UTC)
	signedIn := map[string]string{"CID": "fake-cid", "SPID": "fake-spid", "customer": "fake-customer", "vtc": "fake"}
	withAuth := map[string]string{"auth": "fake-auth"}
	for k, v := range signedIn {
		withAuth[k] = v
	}
	cases := []struct {
		name    string
		cookies map[string]string
		expiry  time.Time
		want    sessionState
	}{
		{"active with future expiry", withAuth, now.Add(20 * time.Minute), sessionActive},
		{"active, expiry unknown", withAuth, time.Time{}, sessionActive},
		{"auth expired", withAuth, now.Add(-time.Minute), sessionLapsed},
		{"auth missing but signed in", signedIn, time.Time{}, sessionLapsed},
		{"anonymous: tracking cookies only", map[string]string{"vtc": "fake", "_pxvid": "fake"}, time.Time{}, sessionAnonymous},
		{"anonymous even with a stray auth", map[string]string{"auth": "fake-auth", "vtc": "fake"}, now.Add(time.Hour), sessionAnonymous},
		{"anonymous: CID only", map[string]string{"CID": "fake-cid", "auth": "fake-auth"}, now.Add(time.Hour), sessionAnonymous},
		{"empty signed-in value", map[string]string{"CID": "fake-cid", "SPID": " ", "customer": "fake", "auth": "fake"}, time.Time{}, sessionAnonymous},
	}
	for _, c := range cases {
		got := classifyWalmartSession(c.cookies, c.expiry, now)
		if got.State != c.want {
			t.Errorf("%s: state %d, want %d (%s)", c.name, got.State, c.want, got.Detail)
		}
		if strings.Contains(got.Detail, "fake") {
			t.Errorf("%s: detail echoes a cookie value", c.name)
		}
	}
}

func TestAuthCookieExpiry(t *testing.T) {
	exp := time.Date(2030, 1, 2, 12, 30, 0, 0, time.UTC)
	rows := []client.ImportedCookie{{Name: "vtc", Value: "fake"}, {Name: "auth", Value: "fake", Expires: exp}}
	if got := authCookieExpiry(rows); !got.Equal(exp) {
		t.Fatalf("got %v", got)
	}
	if !authCookieExpiry(nil).IsZero() {
		t.Fatal("no rows, no expiry")
	}
}
