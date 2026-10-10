// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.

package client

// Max-Age / expiry handling for cookies persisted from responses. Fake values only.

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/walmart/internal/config"
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

// TestEmptySavedJarDoesNotReseedLoginCookies: once every saved cookie has
// expired or been deleted, the empty jar means logged out; client.New must not
// fall back to the stale cookie string captured at login.
func TestEmptySavedJarDoesNotReseedLoginCookies(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WALMART_DATA_DIR", dir)
	t.Setenv("XDG_DATA_HOME", dir)
	t.Setenv("XDG_CACHE_HOME", dir)
	cfg := &config.Config{BaseURL: "https://www.walmart.com", AccessToken: "fake_stale=old1; fake_auth=old2"}
	site := &url.URL{Scheme: "https", Host: "www.walmart.com", Path: "/"}
	names := func(c *Client) []string {
		var out []string
		for _, ck := range c.HTTPClient.Jar.Cookies(site) {
			out = append(out, ck.Name)
		}
		return out
	}

	// No saved jar yet (e.g. an env/credentials session): seeding still applies.
	if got := names(New(cfg, 5*time.Second, 0)); len(got) != 2 {
		t.Fatalf("no jar file: want the 2 seeded cookies, got %v", got)
	}

	// Login wrote a jar; Walmart then deletes the last cookie (Max-Age=0).
	if err := ReplaceCookieJar([]ImportedCookie{{Name: "fake_auth", Value: "new", Domain: ".walmart.com", Path: "/"}}); err != nil {
		t.Fatal(err)
	}
	c := New(cfg, 5*time.Second, 0)
	c.HTTPClient.Jar.SetCookies(site, []*http.Cookie{{Name: "fake_auth", Domain: ".walmart.com", Path: "/", MaxAge: -1}})
	if rows := PersistedCookies(); len(rows) != 0 {
		t.Fatalf("deletion must leave an empty saved jar, got %d rows", len(rows))
	}

	// Next command: empty saved jar = logged out, never the stale login string.
	next := New(cfg, 5*time.Second, 0)
	if got := names(next); len(got) != 0 {
		t.Fatalf("empty saved jar re-seeded stale login cookies: %v", got)
	}
	if !PersistedJarLoaded(next.HTTPClient.Jar) {
		t.Fatal("an existing empty jar file must count as the persisted jar")
	}
}
