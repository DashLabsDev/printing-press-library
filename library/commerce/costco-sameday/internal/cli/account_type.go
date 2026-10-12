// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored guest-vs-member detection for `auth status`, `session
// context --account-type` and `slots etas`. Reports cookie NAMES only and
// never prints cookie values or account details.
// Preserved across regenerate via .printing-press-patches.

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/costco-sameday/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/costco-sameday/internal/cliutil"
	"github.com/spf13/cobra"
)

const (
	accountTypeMember  = "member"
	accountTypeGuest   = "guest"
	accountTypeUnknown = "unknown"
)

// accountTypeProbeOperations are read-only persisted queries tried in order.
// UserType and GetCurrentUserName answer "is this a guest?" directly when
// the server returns a guest flag or a user name; HouseholdByUser is the
// weak fallback (a member without a household may also get "not found").
var accountTypeProbeOperations = []string{"UserType", "GetCurrentUserName", "HouseholdByUser"}

// memberCookiePrefixes are costco.com member-login cookie name prefixes. They
// show up in an imported jar only when cookies were copied from costco.com
// as well as sameday.costco.com, so their absence proves nothing.
var memberCookiePrefixes = []string{"WC_AUTHENTICATION_", "WC_USERACTIVITY_", "WC_SESSION_ESTABLISHED", "WC_PERSISTENT"}

type accountTypeResult struct {
	Type     string `json:"account_type"`
	Source   string `json:"account_type_source,omitempty"`
	Evidence string `json:"account_type_evidence,omitempty"`
	// Errs holds probe errors when no probe answered (not serialized).
	Errs []error `json:"-"`
}

type cookieNameHints struct {
	SessionCookie     bool     `json:"session_cookie_present"`
	MemberCookieNames []string `json:"member_cookie_names,omitempty"`
	CookieCount       int      `json:"cookie_count"`
}

// cookieNameHintsFrom inspects a stored "name=value; ..." jar by name only.
func cookieNameHintsFrom(jar string) cookieNameHints {
	var h cookieNameHints
	seen := map[string]bool{}
	for _, part := range strings.Split(trimCookieHeaderPrefix(jar), ";") {
		name, _, ok := strings.Cut(strings.TrimSpace(part), "=")
		name = strings.TrimSpace(name)
		if !ok || name == "" || seen[name] {
			continue
		}
		seen[name] = true
		h.CookieCount++
		if name == sessionCookieName {
			h.SessionCookie = true
		}
		for _, p := range memberCookiePrefixes {
			if strings.HasPrefix(name, p) {
				h.MemberCookieNames = append(h.MemberCookieNames, name)
				break
			}
		}
	}
	sort.Strings(h.MemberCookieNames)
	return h
}

// probeAccountType asks the server whether the session is a guest. Each
// probe is a read-only GraphQL query; failures fall through to the next.
func probeAccountType(ctx context.Context, c *client.Client) accountTypeResult {
	var weak *accountTypeResult
	var errs []error
	for _, op := range accountTypeProbeOperations {
		payload, err := c.GraphQLQuery(ctx, op, map[string]any{})
		if err != nil {
			errs = append(errs, err)
			continue
		}
		r := classifyAccountPayload(op, payload)
		if r.Type == accountTypeUnknown {
			continue
		}
		if strings.HasSuffix(r.Source, "(weak)") {
			if weak == nil {
				weak = &r
			}
			continue
		}
		return r
	}
	if weak != nil {
		return *weak
	}
	return accountTypeResult{Type: accountTypeUnknown, Evidence: "no probe query answered (" + strings.Join(accountTypeProbeOperations, ", ") + ")", Errs: errs}
}

// classifyAccountPayload reads a probe response without knowing its exact
// shape: a boolean guest/isGuest/visitor field, a userType string, a user
// name (presence only), or a household id / not-found marker.
func classifyAccountPayload(op string, payload map[string]any) accountTypeResult {
	data := payload["data"]
	if data == nil {
		return accountTypeResult{Type: accountTypeUnknown}
	}
	if v, key, ok := findBool(data, "guest", "isGuest", "isVisitor", "visitor", "isAnonymous"); ok {
		if v {
			return accountTypeResult{Type: accountTypeGuest, Source: op, Evidence: key + "=true"}
		}
		return accountTypeResult{Type: accountTypeMember, Source: op, Evidence: key + "=false"}
	}
	if s, key, ok := findString(data, "userType", "accountType", "type"); ok && op != "HouseholdByUser" {
		ls := strings.ToLower(s)
		switch {
		case strings.Contains(ls, "guest"), strings.Contains(ls, "visitor"), strings.Contains(ls, "anonym"), strings.Contains(ls, "implicit"):
			return accountTypeResult{Type: accountTypeGuest, Source: op, Evidence: key + " names a guest type"}
		case op == "UserType":
			return accountTypeResult{Type: accountTypeMember, Source: op, Evidence: key + " is a non-guest type"}
		}
	}
	if op == "GetCurrentUserName" {
		if _, key, ok := findString(data, "firstName", "fullName", "name", "lastName"); ok {
			return accountTypeResult{Type: accountTypeMember, Source: op, Evidence: key + " present (value not shown)"}
		}
	}
	if op == "HouseholdByUser" {
		if containsNotFound(data) {
			return accountTypeResult{Type: accountTypeGuest, Source: op + " (weak)", Evidence: "household not found; a member without a household can also see this"}
		}
		if _, _, ok := findString(data, "id"); ok {
			return accountTypeResult{Type: accountTypeMember, Source: op, Evidence: "household present (id not shown)"}
		}
	}
	return accountTypeResult{Type: accountTypeUnknown}
}

func findBool(v any, keys ...string) (bool, string, bool) {
	switch t := v.(type) {
	case map[string]any:
		for _, k := range keys {
			if b, ok := t[k].(bool); ok {
				return b, k, true
			}
		}
		for _, k := range sortedKeys(t) {
			if b, key, ok := findBool(t[k], keys...); ok {
				return b, key, true
			}
		}
	case []any:
		for _, x := range t {
			if b, key, ok := findBool(x, keys...); ok {
				return b, key, true
			}
		}
	}
	return false, "", false
}

func findString(v any, keys ...string) (string, string, bool) {
	switch t := v.(type) {
	case map[string]any:
		for _, k := range keys {
			if s, ok := t[k].(string); ok && strings.TrimSpace(s) != "" {
				return s, k, true
			}
		}
		for _, k := range sortedKeys(t) {
			if s, key, ok := findString(t[k], keys...); ok {
				return s, key, true
			}
		}
	case []any:
		for _, x := range t {
			if s, key, ok := findString(x, keys...); ok {
				return s, key, true
			}
		}
	}
	return "", "", false
}

func containsNotFound(v any) bool {
	switch t := v.(type) {
	case string:
		ls := strings.ToLower(t)
		return strings.Contains(ls, "notfound") || strings.Contains(ls, "not_found") || strings.Contains(ls, "not found")
	case map[string]any:
		for _, k := range sortedKeys(t) {
			if strings.EqualFold(k, "notFound") {
				if b, ok := t[k].(bool); ok && b {
					return true
				}
			}
			if containsNotFound(t[k]) {
				return true
			}
		}
	case []any:
		for _, x := range t {
			if containsNotFound(x) {
				return true
			}
		}
	}
	return false
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// authStatusProbe runs the guest/member probe for `auth status`. Dry-run and
// verify mode never touch the network.
func authStatusProbe(cmd *cobra.Command, flags *rootFlags, expiry time.Time) sessionHealth {
	c, err := flags.newClient()
	if err != nil {
		c = nil
	}
	if c != nil && (c.IsDryRun() || cliutil.IsVerifyEnv()) {
		h := checkSessionHealth(cmd.Context(), nil, expiry, time.Now())
		h.Session = "not verified (dry-run/verify)"
		h.AccountType = accountTypeResult{Type: accountTypeUnknown, Evidence: "not probed (dry-run/verify)"}
		return h
	}
	return checkSessionHealth(cmd.Context(), c, expiry, time.Now())
}

func describeAccountType(r accountTypeResult) string {
	var label string
	switch r.Type {
	case accountTypeMember:
		label = "signed-in member"
	case accountTypeGuest:
		label = "guest (not signed in to a Costco member account)"
	default:
		label = "unknown"
	}
	var extra []string
	if r.Source != "" {
		extra = append(extra, "via "+r.Source)
	}
	if r.Evidence != "" {
		extra = append(extra, r.Evidence)
	}
	if len(extra) > 0 {
		label += " (" + strings.Join(extra, "; ") + ")"
	}
	return label
}

func presentWord(b bool) string {
	if b {
		return "present"
	}
	return "missing"
}

// sessionHealth is the live session verdict for `auth status` and `doctor`.
type sessionHealth struct {
	Session     string            `json:"session"`
	Valid       bool              `json:"session_valid"`
	AccountType accountTypeResult `json:"-"`
	Expires     string            `json:"session_expires"`
	Expired     bool              `json:"-"`
}

// checkSessionHealth runs the read-only probe and reads the saved expiry.
// Verdict strings start with OK-style text, "WARN" or "ERROR" so doctor can
// color them.
func checkSessionHealth(ctx context.Context, c *client.Client, expiry time.Time, now time.Time) sessionHealth {
	h := sessionHealth{}
	switch {
	case expiry.IsZero():
		h.Expires = "unknown (this import carried no expiry; 'auth login' records it)"
	case !expiry.After(now):
		h.Expires = "ERROR expired " + expiry.Local().Format(time.RFC3339) + "; " + hintRunAuthLogin
		h.Expired = true
	default:
		h.Expires = expiry.Local().Format(time.RFC3339) + " (in " + humanDuration(expiry.Sub(now)) + ")"
	}
	if c == nil {
		h.Session = "WARN not verified (no client)"
		return h
	}
	h.AccountType = probeAccountType(ctx, c)
	switch h.AccountType.Type {
	case accountTypeMember:
		h.Session, h.Valid = "valid (signed-in member)", true
	case accountTypeGuest:
		h.Session, h.Valid = "WARN valid but guest (not signed in to a Costco member account); "+hintRunAuthLogin, true
	default:
		if sessionErrorsLookExpired(h.AccountType.Errs) {
			h.Session = "ERROR expired or missing session; " + hintRunAuthLogin
			h.Expired = true
		} else {
			h.Session = "WARN not verified (probe queries did not answer)"
		}
	}
	return h
}

func sessionErrorsLookExpired(errs []error) bool {
	if len(errs) == 0 {
		return false
	}
	for _, err := range errs {
		m := strings.ToLower(err.Error())
		if !(strings.Contains(m, "http 401") || strings.Contains(m, "http 403") || strings.Contains(m, "unauthorized") || strings.Contains(m, "unauthenticated") || strings.Contains(m, "not logged in") || strings.Contains(m, "login required") || strings.Contains(m, "returned html")) {
			return false
		}
	}
	return true
}

func humanDuration(d time.Duration) string {
	switch {
	case d >= 48*time.Hour:
		return fmt.Sprintf("%d days", int(d.Hours()/24))
	case d >= time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
}
