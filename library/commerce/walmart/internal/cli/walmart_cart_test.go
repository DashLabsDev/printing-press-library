// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/walmart/internal/walmart"
)

func TestResolveCartIDOrder(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WALMART_DATA_DIR", dir)
	t.Setenv("XDG_DATA_HOME", dir)
	t.Setenv("WALMART_CART_ID", "")
	const a = "00000000-0000-4000-8000-00000000000a"
	const b = "00000000-0000-4000-8000-00000000000b"
	if _, _, err := resolveCartID(""); err == nil {
		t.Fatal("no id anywhere must error")
	}
	rememberCartID(a)
	p, err := cartIDStatePath()
	if err != nil {
		t.Fatal(err)
	}
	if rel, err := filepath.Rel(dir, p); err != nil || rel == "" || rel[0] == '.' {
		t.Fatalf("state file %s must live under the temp data dir", p)
	}
	if st, err := os.Stat(p); err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("state file mode: %v %v", st, err)
	}
	if id, src, _ := resolveCartID(""); id != a || src != "remembered" {
		t.Fatalf("got %s %s", id, src)
	}
	t.Setenv("WALMART_CART_ID", b)
	if id, src, _ := resolveCartID(""); id != b || src != "env" {
		t.Fatalf("env must win over remembered: %s %s", id, src)
	}
	if id, src, _ := resolveCartID(a); id != a || src != "flag" {
		t.Fatalf("flag must win: %s %s", id, src)
	}
	if _, _, err := resolveCartID("bad"); err == nil {
		t.Fatal("malformed flag must error")
	}
}

func findSub(parent *cobra.Command, name string) *cobra.Command {
	for _, c := range parent.Commands() {
		if c.Name() == name {
			return c
		}
	}
	return nil
}

func TestCartLinkIsReadOnlyAnnotation(t *testing.T) {
	flags := &rootFlags{}
	cart := newCartCmd(flags)
	for _, name := range []string{"link", "add", "view"} {
		c := findSub(cart, name)
		if c == nil {
			t.Fatalf("missing cart %s", name)
		}
		if c.Annotations["mcp:read-only"] != "true" {
			t.Fatalf("cart %s must be mcp:read-only", name)
		}
		if c.Annotations["mcp:hidden"] == "true" {
			t.Fatalf("cart %s must not be mcp:hidden", name)
		}
	}
	if findSub(newSlotsCmd(flags), "reserve") != nil {
		t.Fatal("slots reserve must be removed")
	}
	if findSub(cart, "update") != nil || findSub(cart, "remove") != nil {
		t.Fatal("cart update/remove must be removed")
	}
}

func TestParseLinkItemArgs(t *testing.T) {
	items, err := parseLinkItemArgs([]string{"51259338", "44391152:2", "https://www.walmart.com/ip/Example/10450114"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 || items[0].Quantity != 1 || items[1].Quantity != 2 || items[2].USItemID != "10450114" {
		t.Fatalf("%+v", items)
	}
	if _, err := parseLinkItemArgs([]string{"51259338:0"}, 1); err == nil {
		t.Fatal("qty 0 must fail")
	}
}

func TestCartLinkURL(t *testing.T) {
	u, err := walmart.AffilAddToCartURL([]walmart.AffilCartItem{{USItemID: "51259338", Quantity: 1}, {USItemID: "44391152", Quantity: 2}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(u, "affil.walmart.com/cart/addToCart") {
		t.Fatal(u)
	}
	if !strings.Contains(u, "51259338") || !strings.Contains(u, "44391152") {
		t.Fatal(u)
	}
	// | is query-escaped as %7C
	if !strings.Contains(u, "%7C1") && !strings.Contains(u, "|1") {
		t.Fatal(u)
	}
}
