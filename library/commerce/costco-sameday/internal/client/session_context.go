// Copyright 2026 dashlabsdev and contributors. Licensed under Apache-2.0.
// Hand-authored session-context resolution (shopId / cartId / zoneId /
// postalCode / userLocation) and GraphQL-errors-as-failures.
// Preserved across regenerate via .printing-press-patches records.

package client

import (
	"context"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/costco-sameday/internal/cliutil"
)

// Session-context operations (persisted queries captured from the web app).
const (
	geolocationOperation          = "Geolocation"
	shopCollectionScopedOperation = "ShopCollectionScoped"
	activeCartIDOperation         = "ActiveCartId"
	personalActiveCartsOperation  = "PersonalActiveCarts"
	costcoRetailerSlug            = "costco"

	// SessionZipEnv overrides the default delivery ZIP (below --zip).
	SessionZipEnv = "COSTCO_SAMEDAY_ZIP"

	sessionContextFile    = "session-context.json"
	sessionLocationMaxAge = 24 * time.Hour
	sessionCartMaxAge     = 10 * time.Minute
)

// Context variable names the resolver knows how to supply.
const (
	ctxVarShopID       = "shopId"
	ctxVarCartID       = "cartId"
	ctxVarZoneID       = "zoneId"
	ctxVarPostalCode   = "postalCode"
	ctxVarUserLocation = "userLocation"
)

// sessionContextOperations maps read operations to the session-derived
// variables they declare (taken from captured web traffic). Only variables the
// caller left empty are filled; explicit flags always win.
var sessionContextOperations = map[string][]string{
	"ActiveCartId":                  {ctxVarShopID},
	"CartTotals":                    {ctxVarShopID, ctxVarCartID},
	"CartCheckoutValidation":        {ctxVarShopID, ctxVarCartID},
	"CartOnloadPlacementQuery":      {ctxVarShopID, ctxVarCartID, ctxVarPostalCode},
	"CartBottomBannerQuery":         {ctxVarShopID, ctxVarPostalCode},
	"CartRecommendationsPlacements": {ctxVarShopID, ctxVarZoneID, ctxVarPostalCode},
	"CartProductsRecomendation":     {ctxVarShopID, ctxVarCartID, ctxVarZoneID, ctxVarPostalCode},
	"FloatingCartMessages":          {ctxVarShopID, ctxVarCartID, ctxVarPostalCode},
	"TreatmentCartMessages":         {ctxVarShopID, ctxVarCartID, ctxVarPostalCode},
	"UserCartEnriched":              {ctxVarShopID, ctxVarPostalCode},
	"CurrentRetailer":               {ctxVarCartID},
	"AvailableServices":             {ctxVarShopID, ctxVarCartID, ctxVarUserLocation},
	"CheckoutCmd":                   {ctxVarShopID},
	"Items":                         {ctxVarShopID, ctxVarZoneID, ctxVarPostalCode},
	"ItemDetailData":                {ctxVarShopID},
	"ItemDetailsRetailerProduct":    {ctxVarZoneID},
	"Autosuggestions":               {ctxVarShopID},
	"SearchFacets":                  {ctxVarShopID, ctxVarPostalCode},
	"SearchResultsPlacements":       {ctxVarShopID, ctxVarZoneID, ctxVarPostalCode},
	"SlotCampaignPlacement":         {ctxVarShopID, ctxVarZoneID, ctxVarPostalCode},
	"ComplementaryProductItems":     {ctxVarShopID, ctxVarZoneID, ctxVarPostalCode},
}

var postalCodeRe = regexp.MustCompile(`^\d{5}$`)

// ValidatePostalCode accepts a 5-digit US ZIP (empty means "use default").
func ValidatePostalCode(zip string) error {
	zip = strings.TrimSpace(zip)
	if zip == "" || postalCodeRe.MatchString(zip) {
		return nil
	}
	return fmt.Errorf("invalid --zip %q: expected a 5-digit US ZIP code", zip)
}

// SessionContext is the shop/cart/location context a signed-in web session
// carries implicitly and the CLI must derive explicitly.
type SessionContext struct {
	PostalCode         string   `json:"postal_code,omitempty"`
	PostalCodeSource   string   `json:"postal_code_source,omitempty"`
	ZoneID             string   `json:"zone_id,omitempty"`
	ShopID             string   `json:"shop_id,omitempty"`
	RetailerLocationID string   `json:"retailer_location_id,omitempty"`
	Latitude           *float64 `json:"latitude,omitempty"`
	Longitude          *float64 `json:"longitude,omitempty"`
	CartID             string   `json:"cart_id,omitempty"`
}

// UserLocationJSON renders CheckoutInputsUserLocation from the context.
func (s SessionContext) UserLocationJSON() string {
	loc := map[string]any{}
	if s.PostalCode != "" {
		loc["postalCode"] = s.PostalCode
	}
	if s.Latitude != nil && s.Longitude != nil {
		loc["coordinates"] = map[string]any{"latitude": *s.Latitude, "longitude": *s.Longitude}
	}
	if len(loc) == 0 {
		return ""
	}
	b, _ := json.Marshal(loc)
	return string(b)
}

// SessionNeeds selects which parts of the context to resolve.
type SessionNeeds struct {
	Shop     bool
	Zone     bool
	Cart     bool
	Location bool
	// Refresh ignores cached values (they are still rewritten).
	Refresh bool
}

func needsForVars(vars []string) SessionNeeds {
	var n SessionNeeds
	for _, v := range vars {
		switch v {
		case ctxVarShopID:
			n.Shop = true
		case ctxVarCartID:
			n.Shop, n.Cart = true, true
		case ctxVarZoneID:
			n.Zone = true
		case ctxVarPostalCode:
			n.Location = true
		case ctxVarUserLocation:
			n.Location = true
		}
	}
	return n
}

type cachedGeolocation struct {
	PostalCode string    `json:"postal_code,omitempty"`
	ZoneID     string    `json:"zone_id,omitempty"`
	Latitude   *float64  `json:"latitude,omitempty"`
	Longitude  *float64  `json:"longitude,omitempty"`
	ResolvedAt time.Time `json:"resolved_at"`
}

type cachedShop struct {
	ShopID             string    `json:"shop_id"`
	RetailerLocationID string    `json:"retailer_location_id,omitempty"`
	ResolvedAt         time.Time `json:"resolved_at"`
}

type cachedCart struct {
	CartID     string    `json:"cart_id"`
	ResolvedAt time.Time `json:"resolved_at"`
}

// sessionContextCache is persisted (0600) in the state dir. Geolocation and
// carts are keyed by a hash of the active credential so a different login
// never inherits another account's cart.
type sessionContextCache struct {
	LastPostalCode string                       `json:"last_postal_code,omitempty"`
	Geolocation    map[string]cachedGeolocation `json:"geolocation,omitempty"`
	Shops          map[string]cachedShop        `json:"shops,omitempty"`
	Carts          map[string]cachedCart        `json:"carts,omitempty"`
}

type sessionState struct {
	mu  sync.Mutex
	zip string
}

type sessionResolvingKey struct{}

func withSessionResolving(ctx context.Context) context.Context {
	return context.WithValue(ctx, sessionResolvingKey{}, true)
}

func isSessionResolving(ctx context.Context) bool {
	v, _ := ctx.Value(sessionResolvingKey{}).(bool)
	return v
}

// SetSessionPostalCode sets the explicit delivery ZIP (from --zip). An explicit
// ZIP is remembered as the default for later runs.
func (c *Client) SetSessionPostalCode(zip string) {
	if c == nil {
		return
	}
	c.sessionMu().Lock()
	defer c.sessionMu().Unlock()
	c.session.zip = strings.TrimSpace(zip)
}

func (c *Client) sessionMu() *sync.Mutex {
	return &c.session.mu
}

// invalidateSessionCart drops cached cart ids so the next lookup re-resolves.
func (c *Client) invalidateSessionCart() {
	if c == nil || cliutil.IsVerifyEnv() {
		return
	}
	c.sessionMu().Lock()
	defer c.sessionMu().Unlock()
	cache := c.loadSessionCache()
	if len(cache.Carts) == 0 {
		return
	}
	cache.Carts = nil
	c.saveSessionCache(cache)
}

func (c *Client) sessionCachePath() string {
	dir, err := cliutil.StateDir()
	if err != nil || dir == "" {
		return ""
	}
	return filepath.Join(dir, sessionContextFile)
}

func (c *Client) loadSessionCache() sessionContextCache {
	var cache sessionContextCache
	path := c.sessionCachePath()
	if path == "" {
		return cache
	}
	data, err := os.ReadFile(filepath.Clean(path)) // #nosec G304 -- app-owned state file.
	if err == nil {
		_ = json.Unmarshal(data, &cache)
	}
	return cache
}

func (c *Client) saveSessionCache(cache sessionContextCache) {
	path := c.sessionCachePath()
	if path == "" {
		return
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

func (c *Client) credentialScopeKey() string {
	if c == nil || c.Config == nil {
		return "anonymous"
	}
	cred := c.Config.StoreScopeCredential()
	if cred == "" {
		return "anonymous"
	}
	sum := sha256.Sum256([]byte(cred))
	return hex.EncodeToString(sum[:])[:12]
}

// ResolveSessionContext derives the delivery ZIP, zone, Costco shop and the
// active cart for the signed-in session. ZIP precedence: --zip, then
// COSTCO_SAMEDAY_ZIP, then the last explicit ZIP (state cache), then the
// account/IP geolocation. Errors name the part that could not be resolved;
// any parts resolved before the failure are still returned.
func (c *Client) ResolveSessionContext(ctx context.Context, needs SessionNeeds) (SessionContext, error) {
	var out SessionContext
	if c == nil {
		return out, errors.New("session context: nil client")
	}
	ctx = withSessionResolving(ctx)
	c.sessionMu().Lock()
	defer c.sessionMu().Unlock()

	cache := c.loadSessionCache()
	dirty := false
	scope := c.credentialScopeKey()
	now := time.Now()
	useCache := !needs.Refresh && !c.NoCache

	explicit := strings.TrimSpace(c.session.zip)
	switch {
	case explicit != "":
		out.PostalCode, out.PostalCodeSource = explicit, "flag"
		if cache.LastPostalCode != explicit {
			cache.LastPostalCode = explicit
			dirty = true
		}
	case strings.TrimSpace(os.Getenv(SessionZipEnv)) != "":
		out.PostalCode, out.PostalCodeSource = strings.TrimSpace(os.Getenv(SessionZipEnv)), "env"
	case cache.LastPostalCode != "":
		out.PostalCode, out.PostalCodeSource = cache.LastPostalCode, "saved"
	}
	if err := ValidatePostalCode(out.PostalCode); err != nil {
		return out, err
	}

	defer func() {
		if dirty {
			c.saveSessionCache(cache)
		}
	}()

	// Geolocation supplies zoneId and coordinates (and the ZIP fallback).
	var geo *cachedGeolocation
	if g, ok := cache.Geolocation[scope]; ok && useCache && now.Sub(g.ResolvedAt) < sessionLocationMaxAge && g.ZoneID != "" {
		geo = &g
	}
	if geo == nil && (needs.Zone || needs.Shop || needs.Cart || needs.Location || out.PostalCode == "") {
		g, err := c.fetchGeolocation(ctx)
		if err == nil {
			geo = &g
			if cache.Geolocation == nil {
				cache.Geolocation = map[string]cachedGeolocation{}
			}
			cache.Geolocation[scope] = g
			dirty = true
		} else if needs.Zone || out.PostalCode == "" {
			return out, fmt.Errorf("could not resolve delivery location (Geolocation): %w", err)
		}
	}
	if geo != nil {
		out.ZoneID = geo.ZoneID
		out.Latitude, out.Longitude = geo.Latitude, geo.Longitude
		if out.PostalCode == "" && geo.PostalCode != "" {
			out.PostalCode, out.PostalCodeSource = geo.PostalCode, "geolocation"
		}
		if geo.PostalCode != "" && geo.PostalCode != out.PostalCode {
			// Coordinates belong to a different ZIP; do not send them.
			out.Latitude, out.Longitude = nil, nil
		}
	}
	if out.PostalCode == "" {
		return out, errors.New("could not determine a delivery ZIP; pass --zip <ZIP> or set " + SessionZipEnv)
	}

	if needs.Shop || needs.Cart {
		if s, ok := cache.Shops[out.PostalCode]; ok && useCache && now.Sub(s.ResolvedAt) < sessionLocationMaxAge && s.ShopID != "" {
			out.ShopID, out.RetailerLocationID = s.ShopID, s.RetailerLocationID
		} else {
			shopID, locID, err := c.fetchCostcoShop(ctx, out.PostalCode, out.Latitude, out.Longitude, needs.Refresh)
			if err != nil {
				return out, fmt.Errorf("could not resolve Costco shopId for ZIP %s: %w", out.PostalCode, err)
			}
			out.ShopID, out.RetailerLocationID = shopID, locID
			if cache.Shops == nil {
				cache.Shops = map[string]cachedShop{}
			}
			cache.Shops[out.PostalCode] = cachedShop{ShopID: shopID, RetailerLocationID: locID, ResolvedAt: now}
			dirty = true
		}
	}

	if needs.Cart {
		key := scope + ":" + out.ShopID
		if cc, ok := cache.Carts[key]; ok && useCache && now.Sub(cc.ResolvedAt) < sessionCartMaxAge && cc.CartID != "" {
			out.CartID = cc.CartID
		} else {
			cartID, err := c.fetchActiveCartID(ctx, out.ShopID)
			if err != nil {
				return out, fmt.Errorf("could not resolve the active cartId (requires a signed-in session; run 'costco-sameday-pp-cli auth login --chrome' if it expired): %w", err)
			}
			out.CartID = cartID
			if cache.Carts == nil {
				cache.Carts = map[string]cachedCart{}
			}
			cache.Carts[key] = cachedCart{CartID: cartID, ResolvedAt: now}
			dirty = true
		}
	}
	return out, nil
}

func (c *Client) graphQLGet(ctx context.Context, op string, vars map[string]any, noCache bool) (map[string]any, error) {
	vb, err := json.Marshal(vars)
	if err != nil {
		return nil, err
	}
	params := map[string]string{"operationName": op, "variables": string(vb)}
	var raw json.RawMessage
	if noCache {
		raw, err = c.GetNoCache(ctx, "/graphql", params)
	} else {
		raw, err = c.Get(ctx, "/graphql", params)
	}
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("%s: decoding response: %w", op, err)
	}
	return payload, nil
}

func digJSON(v any, path ...string) any {
	cur := v
	for _, p := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[p]
	}
	return cur
}

func jsonString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strings.TrimSuffix(strings.TrimRight(fmt.Sprintf("%f", t), "0"), ".")
	case json.Number:
		return t.String()
	default:
		return ""
	}
}

func jsonFloat(v any) *float64 {
	if f, ok := v.(float64); ok {
		return &f
	}
	return nil
}

func (c *Client) fetchGeolocation(ctx context.Context) (cachedGeolocation, error) {
	var g cachedGeolocation
	payload, err := c.graphQLGet(ctx, geolocationOperation, map[string]any{}, true)
	if err != nil {
		return g, err
	}
	geo := digJSON(payload, "data", "geolocationWithUserLocation", "geolocation")
	g.PostalCode = jsonString(digJSON(geo, "postalCode"))
	g.ZoneID = jsonString(digJSON(geo, "zoneId"))
	g.Latitude = jsonFloat(digJSON(geo, "coordinates", "latitude"))
	g.Longitude = jsonFloat(digJSON(geo, "coordinates", "longitude"))
	g.ResolvedAt = time.Now()
	if g.ZoneID == "" && g.PostalCode == "" {
		return g, errors.New("Geolocation returned no zoneId or postalCode")
	}
	return g, nil
}

// fetchCostcoShop maps a ZIP to the Costco delivery shop via
// ShopCollectionScoped. The server keys on postalCode; coordinates are a
// required input, so zero coordinates are sent when none match the ZIP.
func (c *Client) fetchCostcoShop(ctx context.Context, zip string, lat, lng *float64, noCache bool) (string, string, error) {
	coords := map[string]any{"latitude": 0, "longitude": 0}
	if lat != nil && lng != nil {
		coords = map[string]any{"latitude": *lat, "longitude": *lng}
	}
	payload, err := c.graphQLGet(ctx, shopCollectionScopedOperation, map[string]any{
		"retailerSlug":           costcoRetailerSlug,
		"postalCode":             zip,
		"coordinates":            coords,
		"allowCanonicalFallback": true,
	}, noCache)
	if err != nil {
		return "", "", err
	}
	shops, _ := digJSON(payload, "data", "shopCollection", "shops").([]any)
	var fallbackID, fallbackLoc string
	for _, s := range shops {
		id := jsonString(digJSON(s, "id"))
		if id == "" {
			continue
		}
		loc := jsonString(digJSON(s, "retailerLocationId"))
		if strings.EqualFold(jsonString(digJSON(s, "serviceType")), "delivery") {
			return id, loc, nil
		}
		if fallbackID == "" {
			fallbackID, fallbackLoc = id, loc
		}
	}
	if fallbackID != "" {
		return fallbackID, fallbackLoc, nil
	}
	return "", "", fmt.Errorf("no Costco shop serves ZIP %s", zip)
}

// fetchActiveCartID asks ActiveCartId for the shop basket, falling back to the
// Costco cart in PersonalActiveCarts.
func (c *Client) fetchActiveCartID(ctx context.Context, shopID string) (string, error) {
	payload, err := c.graphQLGet(ctx, activeCartIDOperation, map[string]any{"shopId": shopID}, true)
	if err == nil {
		if id := jsonString(digJSON(payload, "data", "shopBasket", "cartId")); id != "" {
			return id, nil
		}
	}
	firstErr := err
	payload, err = c.graphQLGet(ctx, personalActiveCartsOperation, map[string]any{}, true)
	if err != nil {
		if firstErr != nil {
			return "", firstErr
		}
		return "", err
	}
	carts, _ := digJSON(payload, "data", "userCarts", "carts").([]any)
	for _, cart := range carts {
		if strings.EqualFold(jsonString(digJSON(cart, "retailer", "slug")), costcoRetailerSlug) {
			if id := jsonString(digJSON(cart, "id")); id != "" {
				return id, nil
			}
		}
	}
	return "", errors.New("no active Costco cart found for this account")
}

// graphQLVariablePresent reports whether name is already supplied as a flat
// param or inside the variables JSON.
func graphQLVariablePresent(params map[string]string, name string) bool {
	if strings.TrimSpace(params[name]) != "" {
		return true
	}
	if raw := params["variables"]; raw != "" {
		var vars map[string]any
		if json.Unmarshal([]byte(raw), &vars) == nil {
			if v, ok := vars[name]; ok && v != nil && v != "" {
				return true
			}
		}
	}
	return false
}

// fillSessionContextParams fills session-derived variables a known read
// operation declares but the caller left empty. It never overrides explicit
// values, never runs in verify/dry-run mode, and never fails the request: a
// resolution error is returned so a later GraphQL error can explain it.
func (c *Client) fillSessionContextParams(ctx context.Context, method, path string, params map[string]string) (map[string]string, error) {
	if c == nil || c.DryRun || !strings.Contains(path, "graphql") || !strings.EqualFold(method, "GET") {
		return params, nil
	}
	if isSessionResolving(ctx) || cliutil.IsVerifyEnv() {
		return params, nil
	}
	op := strings.TrimSpace(params["operationName"])
	vars, ok := sessionContextOperations[op]
	if !ok {
		return params, nil
	}
	var missing []string
	for _, v := range vars {
		if !graphQLVariablePresent(params, v) {
			missing = append(missing, v)
		}
	}
	if len(missing) == 0 {
		return params, nil
	}
	sc, resolveErr := c.ResolveSessionContext(ctx, needsForVars(missing))
	updated := make(map[string]string, len(params)+len(missing))
	for k, v := range params {
		updated[k] = v
	}
	for _, v := range missing {
		var val string
		switch v {
		case ctxVarShopID:
			val = sc.ShopID
		case ctxVarCartID:
			val = sc.CartID
		case ctxVarZoneID:
			val = sc.ZoneID
		case ctxVarPostalCode:
			val = sc.PostalCode
		case ctxVarUserLocation:
			val = sc.UserLocationJSON()
		}
		if val != "" {
			updated[v] = val
		}
	}
	return updated, resolveErr
}

// GraphQLError reports a GraphQL response that carried errors and no data.
type GraphQLError struct {
	Operation  string
	Messages   []string
	Missing    []string
	ResolveErr error
}

var missingVariableRe = regexp.MustCompile(`Variable \$(\w+) of type \S+ was provided invalid value`)

func (e *GraphQLError) Error() string {
	op := e.Operation
	if op == "" {
		op = "request"
	}
	var b strings.Builder
	if len(e.Missing) > 0 {
		fmt.Fprintf(&b, "GraphQL %s failed: missing %s", op, strings.Join(e.Missing, ", "))
		for _, m := range e.Missing {
			if hint := missingVariableHint(m); hint != "" {
				fmt.Fprintf(&b, "\nhint: %s", hint)
			}
		}
	} else {
		fmt.Fprintf(&b, "GraphQL %s failed: %s", op, strings.Join(e.Messages, "; "))
	}
	if e.ResolveErr != nil {
		fmt.Fprintf(&b, "\nauto-resolve: %v", e.ResolveErr)
	}
	return b.String()
}

func (e *GraphQLError) Unwrap() error { return e.ResolveErr }

func missingVariableHint(name string) string {
	switch name {
	case ctxVarShopID:
		return "shopId is looked up from your delivery ZIP; pass --zip <ZIP> (or --shop-id). Run 'costco-sameday-pp-cli session context --zip <ZIP>' to check."
	case ctxVarCartID:
		return "cartId is your active cart and needs a signed-in session; run 'costco-sameday-pp-cli session context' (or 'auth login --chrome' if the session expired), or pass --cart-id."
	case ctxVarZoneID, ctxVarPostalCode:
		return "pass --zip <ZIP>; run 'costco-sameday-pp-cli session context --zip <ZIP>' to check the resolved location."
	case ctxVarUserLocation:
		return "pass --zip <ZIP> (or --user-location '{\"postalCode\":\"<ZIP>\"}')."
	}
	return "pass --" + kebabFlagName(name) + "."
}

// kebabFlagName maps a GraphQL variable (shopId) to its CLI flag (shop-id).
func kebabFlagName(name string) string {
	var b strings.Builder
	for i, r := range name {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('-')
			}
			r += 'a' - 'A'
		}
		b.WriteRune(r)
	}
	return b.String()
}

// graphQLResponseError turns a GraphQL body with errors and no usable data
// into a *GraphQLError. Partial data (some non-null field) is left alone.
func graphQLResponseError(op string, body []byte, resolveErr error) error {
	var payload map[string]any
	if json.Unmarshal(body, &payload) != nil || payload == nil {
		return nil
	}
	if isVerifySynthetic(payload) {
		return nil
	}
	errs, _ := payload["errors"].([]any)
	if len(errs) == 0 {
		return nil
	}
	if data, ok := payload["data"].(map[string]any); ok {
		for _, v := range data {
			if v != nil {
				return nil
			}
		}
	}
	ge := &GraphQLError{Operation: op, ResolveErr: resolveErr}
	seen := map[string]bool{}
	for _, raw := range errs {
		m, _ := raw.(map[string]any)
		msg := strings.TrimSpace(jsonString(m["message"]))
		if msg == "" {
			continue
		}
		ge.Messages = append(ge.Messages, msg)
		if sub := missingVariableRe.FindStringSubmatch(msg); sub != nil {
			ext, _ := m["extensions"].(map[string]any)
			value, hasValue := ext["value"]
			if (!hasValue || value == nil) && !seen[sub[1]] {
				seen[sub[1]] = true
				ge.Missing = append(ge.Missing, sub[1])
			}
		}
	}
	if len(ge.Messages) == 0 {
		ge.Messages = []string{"unknown GraphQL error"}
	}
	sort.Strings(ge.Missing)
	return ge
}

// graphQLOperationName returns the operationName from params or a JSON body.
func graphQLOperationName(params map[string]string, body []byte) string {
	names := collectGraphQLOperationNames(params, body)
	if len(names) == 0 {
		return ""
	}
	return names[0]
}

// NewPageViewID returns a random UUIDv4 for search pageViewId.
func NewPageViewID() string {
	var b [16]byte
	if _, err := cryptorand.Read(b[:]); err != nil {
		return "00000000-0000-4000-8000-000000000000"
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}
