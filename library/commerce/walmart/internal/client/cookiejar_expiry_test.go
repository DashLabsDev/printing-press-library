// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.

package client

// Max-Age / expiry handling for cookies persisted from responses. Fake values only.

import (
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestPersistedExpiryUsesMaxAge(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	exp := now.Add(48 * time.Hour)
	if got := persistedExpiry(&http.Cookie{MaxAge: 1800}, now); !got.Equal(now.Add(30 * time.Minute)) {
		t.Fatalf("Max-Age=1800: got %v", got)
	}
	if got := persistedExpiry(&http.Cookie{MaxAge: 60, Expires: exp}, now); !got.Equal(now.Add(time.Minute)) {
		t.Fatalf("Max-Age must win over Expires: got %v", got)
	}
	if got := persistedExpiry(&http.Cookie{MaxAge: -1}, now); !got.Before(now) {
		t.Fatalf("Max-Age<=0 must be expired: got %v", got)
	}
	if got := persistedExpiry(&http.Cookie{Expires: exp}, now); !got.Equal(exp) {
		t.Fatalf("Expires only: got %v", got)
	}
	if got := persistedExpiry(&http.Cookie{}, now); !got.IsZero() {
		t.Fatalf("session cookie must stay zero: got %v", got)
	}
}

func TestSetCookiesPersistsMaxAgeAndDropsDeletions(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WALMART_DATA_DIR", dir)
	t.Setenv("XDG_DATA_HOME", dir)
	if err := ReplaceCookieJar([]ImportedCookie{
		{Name: "fake_keep", Value: "k", Domain: ".walmart.com", Path: "/"},
		{Name: "fake_gone", Value: "g", Domain: ".walmart.com", Path: "/", Expires: time.Now().Add(time.Hour)},
	}); err != nil {
		t.Fatal(err)
	}
	jar := newCookieJar()
	jar.loadFromDisk()
	u := &url.URL{Scheme: "https", Host: "www.walmart.com", Path: "/"}
	jar.SetCookies(u, []*http.Cookie{
		{Name: "fake_short", Value: "s", Domain: ".walmart.com", Path: "/", MaxAge: 1800},
		{Name: "fake_gone", Value: "", Domain: ".walmart.com", Path: "/", MaxAge: -1},
	})
	byName := map[string]ImportedCookie{}
	for _, c := range PersistedCookies() {
		byName[c.Name] = c
	}
	if _, ok := byName["fake_gone"]; ok {
		t.Fatal("a Max-Age=0 deletion must remove the persisted cookie")
	}
	short, ok := byName["fake_short"]
	if !ok || short.Expires.IsZero() || short.Expires.Sub(time.Now()) > 31*time.Minute || short.Expires.Sub(time.Now()) < 29*time.Minute {
		t.Fatalf("Max-Age cookie must persist with ~30m expiry: %+v", short)
	}
	if keep, ok := byName["fake_keep"]; !ok || !keep.Expires.IsZero() {
		t.Fatalf("session cookie must be kept with no expiry: %+v", keep)
	}
}
