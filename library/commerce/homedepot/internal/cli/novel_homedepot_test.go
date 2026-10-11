package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/commerce/homedepot/internal/client"
	"github.com/spf13/cobra"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func fakeHomeDepotCookie() homeDepotCookie {
	value := base64.RawURLEncoding.EncodeToString([]byte(`{"u":"sample-user","i":"sample-token","t":"sample-account","c":"B2C","r":"ADMIN"}`)) + ".sample-signature"
	return homeDepotCookie{Name: "THD_CUSTOMER", Value: value, Domain: ".homedepot.com", Path: "/", Expires: time.Now().Add(time.Hour), Secure: true}
}

func TestHomeDepotCookieValidation(t *testing.T) {
	for _, tc := range []struct {
		name, domain   string
		expired, valid bool
	}{
		{"root", "homedepot.com", false, true},
		{"subdomain", ".apionline.homedepot.com", false, true},
		{"export_without_domain", "", false, true},
		{"similar_name", "evilhomedepot.com", false, false},
		{"foreign_suffix", "homedepot.com.example.org", false, false},
		{"expired", ".homedepot.com", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ck := fakeHomeDepotCookie()
			ck.Domain = tc.domain
			if tc.expired {
				ck.Expires = time.Now().Add(-time.Hour)
			}
			cookies, err := validHomeDepotCookies([]homeDepotCookie{ck})
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, err=%v", tc.valid, err)
			}
			if tc.valid && len(cookies) != 1 {
				t.Fatal("valid identity cookie was dropped")
			}
		})
	}
}

func TestHomeDepotCookieExpiryFormats(t *testing.T) {
	for _, raw := range []string{
		`{"name":"test","value":"value","expires":"2030-01-01T00:00:00Z","http_only":true}`,
		`{"name":"test","value":"value","expires":1893456000,"httpOnly":true}`,
		`{"name":"test","value":"value","expirationDate":1893456000,"httpOnly":true}`,
	} {
		var cookie homeDepotCookie
		if err := json.Unmarshal([]byte(raw), &cookie); err != nil {
			t.Fatal(err)
		}
		if cookie.Expires.Unix() != 1893456000 || !cookie.HTTPOnly {
			t.Fatal("expiry or HttpOnly metadata lost")
		}
	}
}

func TestHomeDepotPrivateCookieFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cookies.json")
	data, _ := json.Marshal([]homeDepotCookie{fakeHomeDepotCookie()})
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		if out, err := exec.Command("icacls", path, "/inheritance:r", "/grant:r", os.Getenv("USERNAME")+":F").CombinedOutput(); err != nil {
			t.Skipf("cannot set an owner-only test ACL: %v %s", err, out)
		}
	}
	if _, err := readHomeDepotCookies(path, true); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		// NTFS privacy is an ACL property; mode bits do not represent it.
		if out, err := exec.Command("icacls", path, "/grant", "*S-1-5-32-545:R").CombinedOutput(); err != nil {
			t.Skipf("cannot set a broad test ACL: %v %s", err, out)
		}
	} else if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := readHomeDepotCookies(path, true); err == nil {
		t.Fatal("public cookie file accepted")
	}
	if runtime.GOOS == "windows" {
		return // Windows symlink creation needs a privilege not required by this CLI.
	}
	link := filepath.Join(t.TempDir(), "link.json")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := readHomeDepotCookies(link, true); err == nil {
		t.Fatal("symlink cookie file accepted")
	}
}

func TestHomeDepotHistoryShape(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		valid     bool
		count     int
	}{
		{"empty_omits_orders", `{"userId":"u","customerAccountId":"a","orderCount":0}`, true, 0},
		{"one_order", `{"userId":"u","customerAccountId":"a","orderCount":1,"orders":[{"orderNumber":"sample-order"}]}`, true, 1},
		{"wrong_account", `{"userId":"u","customerAccountId":"other","orderCount":0}`, false, 0},
		{"missing_identity", `{"orderCount":0}`, false, 0},
		{"silent_error", `{"userId":"u","customerAccountId":"a","orderCount":0,"error":{"message":"denied"}}`, false, 0},
		{"positive_without_orders", `{"userId":"u","customerAccountId":"a","orderCount":1}`, false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			orders, count, err := decodeHomeDepotHistory([]byte(tc.raw), "u", "a")
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, err=%v", tc.valid, err)
			}
			if tc.valid && (orders == nil || count != tc.count) {
				t.Fatal("empty collection or count was wrong")
			}
		})
	}
}

func TestHomeDepotCartDecodeAndTargeting(t *testing.T) {
	raw := []byte(`{"data":{"cartInfo":{"cartId":"sample-cart","itemCount":1,"items":[{"id":"sample-line","quantity":"2","selectedFulfillment":"bopis","product":{"itemId":"300951230","identifiers":{"productLabel":"Pencil","canonicalUrl":"/p/sample/300951230"},"fulfillment":{"fulfillmentOptions":[{"services":[{"type":"bopis","selected":true,"locations":[{"locationId":"8964"}]}]}]}}}],"localization":{"primaryStoreId":"8964"}}}}`)
	cart, err := decodeHomeDepotCart(raw, "cartInfo")
	if err != nil {
		t.Fatal(err)
	}
	line := cart.Items[0]
	if line.Quantity != 2 || line.Fulfillment != "BOPIS" || line.Location != "8964" {
		t.Fatal("wire strings or fulfillment were not decoded")
	}
	if _, err := findHomeDepotCartLine(cart, line.ItemID); err == nil {
		t.Fatal("product ID was accepted instead of exact line ID")
	}
	if _, err := findHomeDepotCartLine(cart, line.LineID); err != nil {
		t.Fatal(err)
	}
	request, err := cartLineRequest(line, 3)
	if err != nil {
		t.Fatal(err)
	}
	item := request["items"].(map[string]any)["pickup"].([]any)[0].(map[string]any)
	if item["id"] != line.LineID || item["quantity"] != "3" || item["store"] != "8964" {
		t.Fatal("update request did not preserve exact line/store")
	}
	for _, bad := range []string{`{}`, `{"data":{"cartInfo":null}}`, `{"data":{"cartInfo":{"cartId":"sample"}},"errors":[{}]}`} {
		if _, err := decodeHomeDepotCart([]byte(bad), "cartInfo"); err == nil {
			t.Fatal("invalid cart response was accepted")
		}
	}
}

func TestHomeDepotCartOtherLinesUnchanged(t *testing.T) {
	a := homeDepotCartLine{LineID: "a", ItemID: "100000001", Quantity: 1, Fulfillment: "BOPIS", Location: "8964"}
	b := homeDepotCartLine{LineID: "b", ItemID: "100000002", Quantity: 1, Fulfillment: "BOPIS", Location: "8964"}
	before := homeDepotCart{Items: []homeDepotCartLine{a, b}}
	if !homeDepotCartOtherLinesUnchanged(before, homeDepotCart{Items: []homeDepotCartLine{a}}, "b") {
		t.Fatal("correct removal failed")
	}
	a.Quantity = 2
	if homeDepotCartOtherLinesUnchanged(before, homeDepotCart{Items: []homeDepotCartLine{a}}, "b") {
		t.Fatal("unrelated quantity change was accepted")
	}
}

func TestHomeDepotSearchParser(t *testing.T) {
	for _, tc := range []struct {
		name, refs string
		count      int
	}{
		{"one", `[{"__ref":"Product:1"}]`, 1},
		{"zero", `[]`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			page := `window.__APOLLO_STATE__ = {"ROOT_QUERY":{"searchModel({})":{"__ref":"Search:1"}},"Search:1":{"products({})":` + tc.refs + `},"Product:1":{"identifiers":{"itemId":"300951230","productLabel":"Pencil","canonicalUrl":"/p/sample/300951230"},"pricing({})":{"value":0.33}}};`
			products, err := parseHomeDepotSearchPage([]byte(page))
			if err != nil {
				t.Fatal(err)
			}
			if products == nil || len(products) != tc.count {
				t.Fatal("product count or empty array was wrong")
			}
			if tc.count > 0 && (products[0].Availability != "unknown" || products[0].Price == nil || *products[0].Price != 0.33) {
				t.Fatal("price or honest availability was wrong")
			}
		})
	}
	if _, err := parseHomeDepotSearchPage([]byte("Access Denied")); err == nil {
		t.Fatal("clearance denial became empty products")
	}
}

func TestHomeDepotCartUnexpectedNewLine(t *testing.T) {
	original := homeDepotCartLine{LineID: "original", ItemID: "100001123", Quantity: 1}
	extra := homeDepotCartLine{LineID: "unexpected", ItemID: "100001124", Quantity: 1}
	before := homeDepotCart{Items: []homeDepotCartLine{original}}
	after := homeDepotCart{Items: []homeDepotCartLine{original, extra}}
	if homeDepotCartOtherLinesUnchanged(before, after, "target") {
		t.Fatal("unexpected new line accepted")
	}
}

func TestHomeDepotOrdersLimitValidatesBeforeIO(t *testing.T) {
	for _, dry := range []bool{false, true} {
		flags := &rootFlags{dryRun: dry}
		cmd := newNovelOrdersListCmd(flags)
		if err := cmd.Flags().Set("limit", "0"); err != nil {
			t.Fatal(err)
		}
		if err := cmd.RunE(cmd, nil); err == nil || !strings.Contains(err.Error(), "--limit") {
			t.Fatalf("invalid limit was not a preflight usage error: %v", err)
		}
	}
}

func TestHomeDepotSafeProviderError(t *testing.T) {
	private := "private-account-path private-response"
	err := homeDepotRequestError("read order history", 0, &client.APIError{StatusCode: 403, Path: private, Body: private})
	if strings.Contains(err.Error(), private) || ExitCode(err) != 4 {
		t.Fatalf("unsafe/unclassified provider error: %v", err)
	}
}

func TestHomeDepotOrderDetailValidation(t *testing.T) {
	order := observedOrderSummary{OrderOrigin: "store", SalesDate: "2030-01-01", StoreNumber: "1", RegisterNumber: "2", TransactionID: "3"}
	for _, tc := range []struct {
		raw   string
		valid bool
	}{
		{`{"userId":"u","customerAccountId":"a","salesDate":"2030-01-01","storeNumber":1,"registerNumber":2,"transactionId":3,"grandTotalAmount":0,"fulfillmentGroups":[]}`, true},
		{`{"userId":"u","customerAccountId":"a"}`, false},
		{`{"userId":"u","customerAccountId":"a","salesDate":"2030-01-01","storeNumber":1,"registerNumber":2,"transactionId":4,"grandTotalAmount":0,"fulfillmentGroups":[]}`, false},
		{`{"userId":"u","customerAccountId":"a","salesDate":"2030-01-01","storeNumber":1,"registerNumber":2,"transactionId":3,"grandTotalAmount":0,"fulfillmentGroups":[],"error":{}}`, false},
	} {
		var detail observedOrderDetail
		if err := json.Unmarshal([]byte(tc.raw), &detail); err != nil {
			t.Fatal(err)
		}
		if err := validateHomeDepotOrderDetail([]byte(tc.raw), detail, order, "u", "a"); (err == nil) != tc.valid {
			t.Fatalf("valid=%v error=%v", tc.valid, err)
		}
	}
}

func TestHomeDepotChromeDuplicateExpiredIdentity(t *testing.T) {
	cookie := fakeHomeDepotCookie()
	now := time.Now()
	rows := []homeDepotChromeCookieMetadata{
		{Name: cookie.Name, Domain: ".homedepot.com", Path: "/", Expires: now.Add(-time.Hour).UnixMicro() + 11644473600000000},
		{Name: cookie.Name, Domain: ".homedepot.com", Path: "/oms", Expires: now.Add(time.Hour).UnixMicro() + 11644473600000000},
	}
	if _, err := homeDepotCookiesFromChromeMetadata(rows, map[string]string{cookie.Name: cookie.Value}, now); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("ambiguous expired cookie accepted: %v", err)
	}
}

func TestHomeDepotCookiePathScope(t *testing.T) {
	for _, path := range []string{"/", "/s", "/cart", "/oms", "/federation-gateway"} {
		if !homeDepotCookiePathInScope(path) {
			t.Fatalf("request scope omitted: %s", path)
		}
	}
	for _, path := range []string{"/services/c/install", "/omsevil", "/cartoon"} {
		if homeDepotCookiePathInScope(path) {
			t.Fatalf("unrelated scope included: %s", path)
		}
	}
}

func TestHomeDepotOrderItemNamesSurviveCompactOutput(t *testing.T) {
	var detail observedOrderDetail
	if err := json.Unmarshal([]byte(`{"userId":"sample-user-private","customerAccountId":"sample-account-private","grandTotalAmount":4.50,"fulfillmentGroups":[{"lineItems":[{"description":"Sample pencil pack","currentQuantity":2,"cancelledQuantity":0,"totalPrice":4.50}]}]}`), &detail); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"json", "compact", "agent"} {
		t.Run(mode, func(t *testing.T) {
			flags := &rootFlags{asJSON: true, compact: mode != "json", agent: mode == "agent", agentSource: "live"}
			cmd := &cobra.Command{Use: "get"}
			var out bytes.Buffer
			cmd.SetOut(&out)
			if err := printHomeDepotOrderDetail(cmd, flags, observedOrderSummary{OrderOrigin: "online", OrderNumber: "sample-order"}, detail); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), `"description":"Sample pencil pack"`) && !strings.Contains(out.String(), `"description": "Sample pencil pack"`) {
				t.Fatalf("product description was dropped in %s output", mode)
			}
			if strings.Contains(out.String(), "sample-user-private") || strings.Contains(out.String(), "sample-account-private") {
				t.Fatal("private identity fields leaked into order output")
			}
		})
	}
}
