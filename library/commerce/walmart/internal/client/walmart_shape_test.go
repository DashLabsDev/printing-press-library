// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestWalmartBrowserShape(t *testing.T) {
	t.Setenv("WALMART_USER_AGENT", "")
	t.Setenv("WALMART_PLATFORM_VERSION", "")
	req, _ := http.NewRequest("GET", "https://www.walmart.com/orchestra/pdp/graphql/ItemById/abc/ip/10000000001?variables=%7B%7D", nil)
	req.Header.Set("User-Agent", "something-else")
	setWalmartBrowserShape(req)
	if req.Header.Get("User-Agent") != walmartBrowserUA || req.Header.Get("sec-ch-ua-platform") != `"Linux"` {
		t.Fatalf("UA/client hints not aligned: %v", req.Header)
	}
	if req.Header.Get("x-o-platform-version") != defaultWalmartPlatformVersion {
		t.Fatal("platform version")
	}
	if v, ok := req.Header["Ip-Session-Traffic-Type"]; !ok || len(v) != 1 || v[0] != "" {
		t.Fatal("ip-session-traffic-type should be present and empty on ItemById")
	}
	for _, h := range []string{"cache-control", "pragma", "priority", "dpr", "device-memory", "Accept-Language"} {
		if req.Header.Get(h) == "" {
			t.Fatalf("missing %s", h)
		}
	}
	t.Setenv("WALMART_PLATFORM_VERSION", "usweb-test")
	t.Setenv("WALMART_USER_AGENT", "custom")
	req2, _ := http.NewRequest("GET", "https://www.walmart.com/orchestra/home/graphql/getCart/abc", nil)
	req2.Header.Set("User-Agent", "custom")
	setWalmartBrowserShape(req2)
	if req2.Header.Get("User-Agent") != "custom" || req2.Header.Get("sec-ch-ua") != "" || req2.Header.Get("x-o-platform-version") != "usweb-test" {
		t.Fatal("env overrides not honored")
	}
	if _, ok := req2.Header["Ip-Session-Traffic-Type"]; ok {
		t.Fatal("ItemById-only header leaked to getCart")
	}
}

func TestReplaceCookieJarMirrorsBrowser(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WALMART_DATA_DIR", dir)
	t.Setenv("XDG_DATA_HOME", dir)
	if cookieJarPath() == "" {
		t.Skip("no data dir")
	}
	exp := time.Now().Add(time.Hour)
	err := ReplaceCookieJar([]ImportedCookie{
		{Name: "a", Value: "1", Domain: ".walmart.com", Path: "/", Secure: true, Expires: exp},
		{Name: "dup", Value: "x", Domain: ".walmart.com", Path: "/", Secure: true, Expires: exp},
		{Name: "dup", Value: "y", Domain: ".walmart.com", Path: "/", Secure: true, Expires: exp},
		{Name: "host", Value: "h", Domain: "www.walmart.com", Path: "/", Secure: true},
		{Name: "old", Value: "z", Domain: ".walmart.com", Path: "/", Expires: time.Now().Add(-time.Hour)},
	})
	if err != nil {
		t.Fatal(err)
	}
	jar := LoadCookieJar()
	if !PersistedJarLoaded(jar) {
		t.Fatal("persisted jar not loaded")
	}
	u, _ := url.Parse("https://www.walmart.com/orchestra/x")
	got := map[string]int{}
	for _, c := range jar.Cookies(u) {
		got[c.Name]++
	}
	if got["a"] != 1 || got["dup"] != 2 || got["host"] != 1 || got["old"] != 0 {
		t.Fatalf("cookies sent = %v", got)
	}
	// A second login replaces, never merges.
	if err := ReplaceCookieJar([]ImportedCookie{{Name: "b", Value: "2", Domain: ".walmart.com", Path: "/", Secure: true}}); err != nil {
		t.Fatal(err)
	}
	got = map[string]int{}
	for _, c := range LoadCookieJar().Cookies(u) {
		got[c.Name]++
	}
	if len(got) != 1 || got["b"] != 1 {
		t.Fatalf("jar not replaced: %v", got)
	}
}
