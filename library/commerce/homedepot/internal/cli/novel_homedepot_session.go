package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/browserutils/kooky"
	"github.com/browserutils/kooky/browser/safari"
	homedepot "github.com/fnziman/homedepot-go"
	"github.com/mvanhorn/printing-press-library/library/commerce/homedepot/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/homedepot/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/homedepot/internal/config"
)

// Match the generated persistent cookie jar so Safari and Chrome share storage.
type homeDepotCookie struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Domain   string    `json:"domain"`
	Path     string    `json:"path"`
	Expires  time.Time `json:"expires,omitempty"`
	Secure   bool      `json:"secure"`
	HTTPOnly bool      `json:"http_only"`
}

func (c *homeDepotCookie) UnmarshalJSON(data []byte) error {
	var raw struct {
		Name           string          `json:"name"`
		Value          string          `json:"value"`
		Domain         string          `json:"domain"`
		Path           string          `json:"path"`
		Expires        json.RawMessage `json:"expires"`
		ExpirationDate float64         `json:"expirationDate"`
		Secure         bool            `json:"secure"`
		HTTPOnly       bool            `json:"httpOnly"`
		HTTP           bool            `json:"http_only"`
	}
	if json.Unmarshal(data, &raw) != nil {
		return fmt.Errorf("invalid cookie JSON")
	}
	*c = homeDepotCookie{Name: raw.Name, Value: raw.Value, Domain: raw.Domain, Path: raw.Path, Secure: raw.Secure, HTTPOnly: raw.HTTPOnly || raw.HTTP}
	if len(raw.Expires) > 0 && string(raw.Expires) != "null" {
		if json.Unmarshal(raw.Expires, &c.Expires) != nil {
			var seconds float64
			if json.Unmarshal(raw.Expires, &seconds) != nil {
				return fmt.Errorf("invalid cookie expiry")
			}
			if seconds > 0 {
				c.Expires = time.Unix(int64(seconds), 0)
			}
		}
	} else if raw.ExpirationDate > 0 {
		c.Expires = time.Unix(int64(raw.ExpirationDate), 0)
	}
	return nil
}

func sessionPath() (string, error) {
	if custom := os.Getenv("HOMEDEPOT_COOKIE_FILE"); custom != "" {
		if !filepath.IsAbs(custom) {
			return "", fmt.Errorf("HOMEDEPOT_COOKIE_FILE must be absolute")
		}
		return filepath.Clean(custom), nil
	}
	dir, err := cliutil.DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "cookies.json"), nil
}

func homeDepotDomain(domain string) bool {
	domain = strings.ToLower(strings.TrimPrefix(domain, "."))
	return domain == "homedepot.com" || strings.HasSuffix(domain, ".homedepot.com")
}

func readHomeDepotCookies(path string, private bool) ([]homeDepotCookie, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("cookie file must be a regular file")
	}
	if private {
		if err := cliutil.VerifyCredsPerms(path); err != nil {
			return nil, fmt.Errorf("cookie file must be private: %w", err)
		}
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	var cookies []homeDepotCookie
	if json.Unmarshal(data, &cookies) != nil {
		var wrapped struct {
			Cookies []homeDepotCookie `json:"cookies"`
		}
		if json.Unmarshal(data, &wrapped) != nil {
			return nil, fmt.Errorf("invalid cookie export")
		}
		cookies = wrapped.Cookies
	}
	return cookies, nil
}

func credentialsFromHomeDepotCookies(cookies []homeDepotCookie) (homedepot.Credentials, error) {
	for _, ck := range cookies {
		if ck.Name != "THD_CUSTOMER" || !homeDepotDomain(ck.Domain) {
			continue
		}
		if !ck.Expires.IsZero() && !ck.Expires.After(time.Now()) {
			continue
		}
		creds, err := homedepot.DecodeCustomerCookie(ck.Value)
		if err == nil && creds.UserID != "" && creds.CustomerAccountID != "" && strings.IndexAny(creds.UserID, "/?#\\") < 0 {
			return creds, nil
		}
	}
	return homedepot.Credentials{}, authErr(fmt.Errorf("no valid signed-in Home Depot session; sign in and run 'homedepot-pp-cli auth import' or 'auth login --chrome'"))
}

func validHomeDepotCookies(cookies []homeDepotCookie) ([]homeDepotCookie, error) {
	valid := make([]homeDepotCookie, 0, len(cookies))
	for _, ck := range cookies {
		if ck.Domain == "" {
			ck.Domain = ".homedepot.com"
		}
		if !homeDepotDomain(ck.Domain) {
			continue
		}
		if !ck.Expires.IsZero() && !ck.Expires.After(time.Now()) {
			continue
		}
		if ck.Path == "" {
			ck.Path = "/"
		}
		ck.Secure = true
		// #nosec G124 -- Validation-only cookie: never sent or exposed to a browser; transport cookies below force Secure and retain browser HttpOnly metadata.
		if (&http.Cookie{Name: ck.Name, Value: ck.Value, Domain: ck.Domain, Path: ck.Path}).Valid() != nil {
			continue
		}
		valid = append(valid, ck)
	}
	if _, err := credentialsFromHomeDepotCookies(valid); err != nil {
		return nil, err
	}
	return valid, nil
}

func loadHomeDepotSession() ([]homeDepotCookie, homedepot.Credentials, error) {
	path, err := sessionPath()
	if err != nil {
		return nil, homedepot.Credentials{}, err
	}
	cookies, err := readHomeDepotCookies(path, true)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, homedepot.Credentials{}, authErr(fmt.Errorf("no Home Depot session; run 'homedepot-pp-cli auth import' or 'auth login --chrome'"))
		}
		return nil, homedepot.Credentials{}, authErr(fmt.Errorf("load private Home Depot session: %w", err))
	}
	cookies, err = validHomeDepotCookies(cookies)
	if err != nil {
		return nil, homedepot.Credentials{}, err
	}
	creds, err := credentialsFromHomeDepotCookies(cookies)
	return cookies, creds, err
}

func safariHomeDepotCookies(ctx context.Context) ([]homeDepotCookie, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(home, "Library", "Containers", "com.apple.Safari", "Data", "Library", "Cookies", "Cookies.binarycookies")
	found, err := safari.ReadCookies(ctx, path, kooky.DomainHasSuffix("homedepot.com"), kooky.Valid)
	if err != nil {
		return nil, fmt.Errorf("read Safari session: %w", err)
	}
	cookies := make([]homeDepotCookie, 0, len(found))
	for _, c := range found {
		cookies = append(cookies, homeDepotCookie{Name: c.Name, Value: c.Value, Domain: c.Domain, Path: c.Path, Expires: c.Expires, Secure: c.Secure, HTTPOnly: c.HttpOnly})
	}
	return validHomeDepotCookies(cookies)
}

func saveHomeDepotCookies(cookies []homeDepotCookie) error {
	path, err := sessionPath()
	if err != nil {
		return err
	}
	data, err := json.Marshal(cookies)
	if err != nil {
		return err
	}
	return cliutil.AtomicWritePrivateFile(path, data, 0600, 0700)
}

func homeDepotClientForCookies(flags *rootFlags, cookies []homeDepotCookie) (*client.Client, error) {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return nil, err
	}
	// A previous account must not make a candidate session look valid.
	cfg.BaseURL = "https://www.homedepot.com"
	cfg.AuthHeaderVal = ""
	cfg.AccessToken = ""
	cfg.Headers = nil
	c := client.New(cfg, flags.timeout, flags.rateLimit)
	jar, _ := cookiejar.New(nil)
	for _, host := range []string{"www.homedepot.com", "apionline.homedepot.com"} {
		u := &url.URL{Scheme: "https", Host: host}
		for _, ck := range cookies {
			// #nosec G124 -- Outbound HTTPS client jar; preserve the browser's HttpOnly metadata, which applies to browser scripts rather than this Go client.
			jar.SetCookies(u, []*http.Cookie{{Name: ck.Name, Value: ck.Value, Domain: ck.Domain, Path: ck.Path, Expires: ck.Expires, Secure: true, HttpOnly: ck.HTTPOnly}})
		}
	}
	c.HTTPClient.Jar = jar
	c.NoCache = true
	return c, nil
}

// Custom shopping reads and writes use an in-memory jar. Response cookies
// cannot overwrite the private metadata-preserving browser export.
func newHomeDepotSessionClient(flags *rootFlags) (*client.Client, error) {
	cookies, _, err := loadHomeDepotSession()
	if err != nil {
		return nil, err
	}
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	fresh, err := homeDepotClientForCookies(flags, cookies)
	if err != nil {
		return nil, err
	}
	c.HTTPClient.Jar = fresh.HTTPClient.Jar
	c.NoCache = true
	return c, nil
}

func verifyHomeDepotSession(ctx context.Context, flags *rootFlags, cookies []homeDepotCookie) error {
	creds, err := credentialsFromHomeDepotCookies(cookies)
	if err != nil {
		return err
	}
	c, err := homeDepotClientForCookies(flags, cookies)
	if err != nil {
		return err
	}
	now := time.Now().Format("2006-01-02")
	body := map[string]any{"orderHistoryRequest": map[string]any{"customerAccountId": creds.CustomerAccountID, "pageSize": 1, "pageNumber": 1, "startDate": now, "endDate": now, "sortBy": "salesDate", "sortOrder": "desc", "searchType": "ORDERS", "resultsFilter": "allOrders", "timezone": time.Now().Location().String(), "searchValue": ""}}
	raw, status, err := c.PostQueryWithParamsAndHeaders(ctx, "/oms/customer/order/v1/user/"+url.PathEscape(creds.UserID)+"/orderhistory", nil, body, homeDepotOrderHeaders())
	if err != nil {
		return authErr(fmt.Errorf("signed-in session check failed; sign in and import again"))
	}
	if status != 200 {
		return authErr(fmt.Errorf("Home Depot did not confirm the signed-in session; sign in and import again"))
	}
	if _, _, err := decodeHomeDepotHistory(raw, creds.UserID, creds.CustomerAccountID); err != nil {
		return authErr(fmt.Errorf("Home Depot did not confirm the signed-in session; sign in and import again"))
	}
	return nil
}

func init() {
	doctorAuthConfiguredHook = func() (bool, string) {
		_, _, err := loadHomeDepotSession()
		return err == nil, "private browser cookie jar"
	}
	registerClientHook(func(c *client.Client) error {
		if os.Getenv("HOMEDEPOT_COOKIE_FILE") == "" {
			return nil
		}
		cookies, _, err := loadHomeDepotSession()
		if err != nil {
			return err
		}
		fresh, err := homeDepotClientForCookies(&rootFlags{timeout: c.HTTPClient.Timeout}, cookies)
		if err != nil {
			return err
		}
		c.HTTPClient.Jar = fresh.HTTPClient.Jar
		return nil
	})
}

func chromeHomeDepotCookies(cmd *cobra.Command, profileName string) ([]homeDepotCookie, error) {
	tool := cookieTool{}
	var err error
	if _, e := exec.LookPath("pycookiecheat"); e == nil {
		tool = cookieTool{name: "pycookiecheat-cli", pyBin: "pycookiecheat"}
	} else {
		tool, err = detectCookieTool()
	}
	if err != nil {
		return nil, authErr(fmt.Errorf("install pycookiecheat or cookie-scoop-cli to read Chrome cookies"))
	}
	if !cookieToolSupportsProfiles(tool.name) {
		return nil, authErr(fmt.Errorf("use pycookiecheat or cookie-scoop-cli to preserve Chrome cookie metadata"))
	}
	profile, err := resolveChromeProfile(cmd.ErrOrStderr(), cmd.InOrStdin(), "homedepot.com", profileName, requiredAuthCookies())
	if err != nil {
		return nil, authErr(err)
	}
	header, err := extractCookies(tool, "homedepot.com", profile)
	if err != nil {
		return nil, authErr(fmt.Errorf("cannot read Chrome cookies; sign in and retry"))
	}
	if validateExtractedCookieHeader(header) != nil {
		return nil, authErr(fmt.Errorf("Chrome cookie extractor returned an invalid export; update the extractor"))
	}
	values := parseCookieString(header)
	path := filepath.Join(profile.DataDir, profile.Dir, "Cookies")
	// #nosec G204 -- Fixed sqlite3 executable and read-only static query; the resolved Chrome profile database path is passed as one argument without a shell.
	raw, err := exec.CommandContext(cmd.Context(), "sqlite3", "-readonly", "-json", path, `SELECT name,host_key AS domain,path,expires_utc,is_secure AS secure,is_httponly AS http_only FROM cookies WHERE host_key='homedepot.com' OR host_key='.homedepot.com'`).Output()
	if err != nil {
		return nil, authErr(fmt.Errorf("cannot read Chrome cookie metadata"))
	}
	var rows []homeDepotChromeCookieMetadata
	if json.Unmarshal(raw, &rows) != nil {
		return nil, authErr(fmt.Errorf("invalid Chrome cookie metadata"))
	}
	return homeDepotCookiesFromChromeMetadata(rows, values, time.Now())
}

type homeDepotChromeCookieMetadata struct {
	Name     string `json:"name"`
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	Expires  int64  `json:"expires_utc"`
	Secure   int    `json:"secure"`
	HTTPOnly int    `json:"http_only"`
}

func homeDepotCookiesFromChromeMetadata(rows []homeDepotChromeCookieMetadata, values map[string]string, now time.Time) ([]homeDepotCookie, error) {
	cookies := make([]homeDepotCookie, 0, len(rows))
	seenNames := map[string]bool{}
	for _, row := range rows {
		if !homeDepotCookiePathInScope(row.Path) {
			continue
		}
		value, ok := values[row.Name]
		if !ok {
			continue
		}
		if seenNames[row.Name] {
			return nil, authErr(fmt.Errorf("Chrome export contains ambiguous same-name cookies; use a JSON cookie export preserving domain and path"))
		}
		seenNames[row.Name] = true
		if row.Expires > 0 && !time.UnixMicro(row.Expires-11644473600000000).After(now) {
			continue
		}
		ck := homeDepotCookie{Name: row.Name, Value: value, Domain: row.Domain, Path: row.Path, Secure: row.Secure != 0, HTTPOnly: row.HTTPOnly != 0}
		if row.Expires > 0 {
			ck.Expires = time.UnixMicro(row.Expires - 11644473600000000)
		}
		cookies = append(cookies, ck)
	}
	return validHomeDepotCookies(cookies)
}

// Ignore unrelated service-page cookies before joining flattened extractor
// values. They cannot ride any shopping/auth request this CLI makes.
func homeDepotCookiePathInScope(path string) bool {
	if path == "" || path == "/" {
		return true
	}
	for _, route := range []string{"/s", "/cart", "/oms/customer/order", "/federation-gateway/graphql"} {
		if path == route || strings.HasPrefix(path, route+"/") || strings.HasPrefix(route, strings.TrimSuffix(path, "/")+"/") {
			return true
		}
	}
	return false
}
