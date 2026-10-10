// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored `slots etas`: the read-only header ETA queries the store
// header and retailer cards use (GetAccurate[Visitor]Retailer[Pickup]Etas),
// plus the empty-result note for the checkout-scoped `slots availableservices`.
// Preserved across regenerate via registerNovelCommand and
// .printing-press-patches/costco-sameday-slots-etas.json.
//
// Variable shapes (sameday.costco.com public rspack chunks, 2026-10-10):
//   GetAccurateRetailerEtas         {addressId, homeLoadUuid, postalCode, retailerIds, serviceType: "DELIVERY"}
//   GetAccurateVisitorRetailerEtas  {postalCode, retailerIds, explicitRetailerLocationIds, serviceType: "DELIVERY"}
//     explicitRetailerLocationIds elements are AvailabilityRetailerLocationInput
//     objects {retailerId, retailerLocationId} (server error confirmed the type).
// Live response (2026-10): items carry etaSeconds (float), retailerId and
// viewSection.{etaString, condensedEtaString, homeCondensedEtaString, ...};
// there is no fee field and no window end, so earliest_end stays empty and the
// fee is reported only if a future response adds one.
//   GetAccurate[Visitor]RetailerPickupEtas {shopIds (pickup shops), retailerLocations}
// Responses: data.getAccurateRetailerEtas[] (delivery, both audiences) and
// data.pickupEtas[] (pickup); items carry retailerId, retailerLocationId,
// etaSeconds and viewSection{...String}. The exact viewSection fields are
// server-defined by the persisted query, so labels are read generically.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/costco-sameday/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/costco-sameday/internal/cliutil"
	"github.com/spf13/cobra"
)

// availableServicesEmptyNote is printed (stderr, and as JSON "note") when the
// checkout service picker returns no services.
const availableServicesEmptyNote = "AvailableServices is the checkout service picker: it lists services only for a cart that is in checkout, so an empty list outside checkout does not mean Same-Day is unavailable. For delivery/pickup ETAs at a ZIP run: costco-sameday-pp-cli slots etas --zip <ZIP>"

const availableServicesLong = `AvailableServices is the checkout service picker. It returns the delivery
and pickup services for the active cart once that cart is in checkout, so
outside checkout it usually returns an empty "services" list even when
Same-Day delivers to the ZIP. For the store-header delivery/pickup ETAs at a
ZIP (no cart or checkout needed), use 'slots etas'.`

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		for _, child := range root.Commands() {
			if child.Name() == "slots" {
				addNovelCommandIfAbsent(child, newSlotsEtasCmd(flags))
			}
		}
	})
}

// availableServicesEmpty reports whether an AvailableServices reply carried
// an explicit, empty services list.
func availableServicesEmpty(data json.RawMessage) bool {
	var payload struct {
		Data *struct {
			CheckoutAvailableServices *struct {
				Services []json.RawMessage `json:"services"`
			} `json:"checkoutAvailableServices"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &payload) != nil || payload.Data == nil || payload.Data.CheckoutAvailableServices == nil {
		return false
	}
	return len(payload.Data.CheckoutAvailableServices.Services) == 0
}

// withJSONNote adds a top-level "note" to a JSON object; other JSON is
// returned unchanged.
func withJSONNote(data json.RawMessage, note string) json.RawMessage {
	var m map[string]any
	if json.Unmarshal(data, &m) != nil || m == nil {
		return data
	}
	m["note"] = note
	out, err := json.Marshal(m)
	if err != nil {
		return data
	}
	return out
}

type etaRow struct {
	ServiceType   string            `json:"service_type"`
	EtaSeconds    *int64            `json:"eta_seconds,omitempty"`
	EarliestStart string            `json:"earliest_start,omitempty"`
	EarliestEnd   string            `json:"earliest_end,omitempty"`
	Label         string            `json:"label,omitempty"`
	Fee           string            `json:"fee,omitempty"`
	Strings       map[string]string `json:"strings,omitempty"`
}

type etaQueryPlan struct {
	Service   string
	Operation string
	Vars      map[string]any
}

func newSlotsEtasCmd(flags *rootFlags) *cobra.Command {
	var service, audience, retailerID string
	cmd := &cobra.Command{
		Use:   "etas",
		Short: "Earliest delivery/pickup ETA, fee and label for a ZIP (read-only header ETA queries)",
		Long: strings.TrimSpace(`
Shows the earliest Same-Day delivery and pickup times the store header shows
for a ZIP, using the read-only ETA queries. No cart or checkout is needed and
nothing is changed.

Signed-in members use GetAccurateRetailerEtas / GetAccurateRetailerPickupEtas;
guests use GetAccurateVisitorRetailerEtas / GetAccurateVisitorRetailerPickupEtas.
--as auto (default) picks from the session (see 'auth status'); --as member or
--as guest forces one. Output has the service type, earliest window start and
end (end only when the server sends one), and any fee or label text. Ids are
not printed.`),
		Example: `  costco-sameday-pp-cli slots etas --zip 98027
  costco-sameday-pp-cli slots etas --zip 10001 --service delivery --json
  costco-sameday-pp-cli slots etas --as guest`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:narrative": "slots.etas"},
		RunE: func(cmd *cobra.Command, args []string) error {
			service = strings.ToLower(strings.TrimSpace(service))
			audience = strings.ToLower(strings.TrimSpace(audience))
			switch service {
			case "all", "delivery", "pickup":
			default:
				return usageErr(fmt.Errorf("--service must be all, delivery or pickup (got %q)", service))
			}
			switch audience {
			case "auto", accountTypeMember, accountTypeGuest:
			case "visitor":
				audience = accountTypeGuest
			default:
				return usageErr(fmt.Errorf("--as must be auto, member or guest (got %q)", audience))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if c.IsDryRun() || cliutil.IsVerifyEnv() {
				return flags.printJSON(cmd, map[string]any{"dry_run": true, "would": "GET /graphql Geolocation, ShopCollectionScoped, then GetAccurate[Visitor]Retailer[Pickup]Etas"})
			}
			return runSlotsEtas(cmd, flags, c, service, audience, strings.TrimSpace(retailerID))
		},
	}
	cmd.Flags().StringVar(&service, "service", "all", "Which ETAs: all, delivery or pickup")
	cmd.Flags().StringVar(&audience, "as", "auto", "Query set: auto (from the session), member, or guest")
	cmd.Flags().StringVar(&retailerID, "retailer-id", "", "Costco retailer id override (default: from the shop lookup)")
	return cmd
}

func runSlotsEtas(cmd *cobra.Command, flags *rootFlags, c *client.Client, service, audience, retailerID string) error {
	ctx := cmd.Context()
	sc, err := c.ResolveSessionContext(ctx, client.SessionNeeds{Location: true})
	if err != nil {
		return classifyAPIErrorOnly(err)
	}
	shops, err := c.CostcoShops(ctx, sc.PostalCode, sc.Latitude, sc.Longitude)
	if err != nil {
		return classifyAPIErrorOnly(err)
	}
	var notes []string
	ridSource := "flag"
	if retailerID == "" {
		retailerID, ridSource = c.CostcoRetailerID(ctx, shops)
		if ridSource == "assumed" {
			notes = append(notes, "Costco retailer id was not in the shop lookup; assumed the known Costco id (override with --retailer-id)")
		}
	}

	acct := accountTypeResult{Type: audience, Source: "flag"}
	if audience == "auto" {
		acct = probeAccountType(ctx, c)
	}

	out := map[string]any{
		"postal_code":         sc.PostalCode,
		"postal_code_source":  sc.PostalCodeSource,
		"account_type":        acct.Type,
		"account_type_source": acct.Source,
		"retailer_id_source":  ridSource,
	}
	var rows []etaRow
	var queried []string
	now := time.Now()
	for _, plan := range etaPlans(service, acct.Type, sc.PostalCode, retailerID, shops) {
		got, op, err := runEtaPlan(ctx, c, plan, acct.Type, retailerID, now)
		if op != "" {
			queried = append(queried, op)
		}
		if err != nil {
			notes = append(notes, fmt.Sprintf("%s ETAs: %v", plan.Service, err))
			continue
		}
		if len(got) == 0 {
			notes = append(notes, fmt.Sprintf("%s: %s returned no ETA for Costco at this ZIP", plan.Service, op))
		}
		rows = append(rows, got...)
	}
	if service != "delivery" && !hasServiceShop(shops, "pickup") {
		notes = append(notes, "pickup: no Costco pickup shop is listed for this ZIP")
	}
	if len(rows) > 0 && !anyFee(rows) {
		notes = append(notes, etaNoFeeNote)
	}
	out["queried"] = queried
	out["services"] = rows
	if len(notes) > 0 {
		out["notes"] = notes
	}

	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		for _, n := range notes {
			fmt.Fprintln(cmd.ErrOrStderr(), "note: "+n)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "ZIP %s (%s) as %s via %s\n", sc.PostalCode, sc.PostalCodeSource, acct.Type, strings.Join(queried, ", "))
		if len(rows) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No ETAs returned.")
			return nil
		}
		table := [][]string{}
		for _, r := range rows {
			table = append(table, []string{r.ServiceType, humanTime(r.EarliestStart), humanTime(r.EarliestEnd), r.Fee, r.Label})
		}
		return flags.printTable(cmd, []string{"SERVICE", "EARLIEST", "END", "FEE", "LABEL"}, table)
	}
	return flags.printJSON(cmd, out)
}

func hasServiceShop(shops []client.CostcoShop, serviceType string) bool {
	for _, s := range shops {
		if s.ServiceType == serviceType {
			return true
		}
	}
	return false
}

// etaPlans builds the queries for the selected services. An unknown account
// type starts with the member query; runEtaPlan falls back to the visitor one.
func etaPlans(service, accountType, zip, retailerID string, shops []client.CostcoShop) []etaQueryPlan {
	visitor := accountType == accountTypeGuest
	var pickupShops []string
	var deliveryLocs []map[string]any
	for _, s := range shops {
		switch s.ServiceType {
		case "pickup":
			pickupShops = append(pickupShops, s.ID)
		default:
			if s.RetailerLocationID != "" {
				// explicitRetailerLocationIds is [AvailabilityRetailerLocationInput!]:
				// objects, not bare ids (confirmed live; the site builds
				// {retailerId, retailerLocationId} the same way).
				rid := s.RetailerID
				if rid == "" {
					rid = retailerID
				}
				deliveryLocs = append(deliveryLocs, map[string]any{"retailerId": rid, "retailerLocationId": s.RetailerLocationID})
			}
		}
	}
	var plans []etaQueryPlan
	if service == "all" || service == "delivery" {
		plans = append(plans, deliveryEtaPlan(visitor, zip, retailerID, deliveryLocs))
	}
	if (service == "all" || service == "pickup") && len(pickupShops) > 0 {
		plans = append(plans, pickupEtaPlan(visitor, pickupShops))
	}
	return plans
}

func deliveryEtaPlan(visitor bool, zip, retailerID string, locs []map[string]any) etaQueryPlan {
	if visitor {
		if locs == nil {
			locs = []map[string]any{}
		}
		return etaQueryPlan{Service: "delivery", Operation: client.EtaDeliveryVisitorOperation, Vars: map[string]any{
			"postalCode":                  zip,
			"retailerIds":                 []string{retailerID},
			"explicitRetailerLocationIds": locs,
			"serviceType":                 "DELIVERY",
		}}
	}
	return etaQueryPlan{Service: "delivery", Operation: client.EtaDeliveryMemberOperation, Vars: map[string]any{
		"addressId":    nil,
		"homeLoadUuid": client.NewPageViewID(),
		"postalCode":   zip,
		"retailerIds":  []string{retailerID},
		"serviceType":  "DELIVERY",
	}}
}

func pickupEtaPlan(visitor bool, shopIDs []string) etaQueryPlan {
	op := client.EtaPickupMemberOperation
	if visitor {
		op = client.EtaPickupVisitorOperation
	}
	return etaQueryPlan{Service: "pickup", Operation: op, Vars: map[string]any{"shopIds": shopIDs}}
}

// runEtaPlan runs one ETA query. When the account type is unknown and the
// member query fails or is empty, the visitor query is tried.
func runEtaPlan(ctx context.Context, c *client.Client, plan etaQueryPlan, accountType, retailerID string, now time.Time) ([]etaRow, string, error) {
	payload, err := c.GraphQLQuery(ctx, plan.Operation, plan.Vars)
	var rows []etaRow
	if err == nil {
		rows = parseEtaRows(plan.Service, payload, retailerID, now)
	}
	if accountType == accountTypeUnknown && (err != nil || len(rows) == 0) {
		var alt etaQueryPlan
		if plan.Service == "pickup" {
			ids, _ := plan.Vars["shopIds"].([]string)
			alt = pickupEtaPlan(true, ids)
		} else {
			zip, _ := plan.Vars["postalCode"].(string)
			alt = deliveryEtaPlan(true, zip, retailerID, nil)
		}
		if p2, err2 := c.GraphQLQuery(ctx, alt.Operation, alt.Vars); err2 == nil {
			return parseEtaRows(plan.Service, p2, retailerID, now), plan.Operation + "→" + alt.Operation, nil
		} else if err == nil {
			return rows, plan.Operation, nil
		}
	}
	return rows, plan.Operation, err
}

// parseEtaRows reads data.getAccurateRetailerEtas / data.pickupEtas, or any
// list of objects carrying etaSeconds, keeping Costco's entries.
func parseEtaRows(service string, payload map[string]any, retailerID string, now time.Time) []etaRow {
	data, _ := payload["data"].(map[string]any)
	if data == nil {
		return nil
	}
	var list []any
	for _, k := range []string{"getAccurateRetailerEtas", "pickupEtas", "accurateRetailerEtas", "etas"} {
		if l, ok := data[k].([]any); ok {
			list = l
			break
		}
	}
	if list == nil {
		list = findEtaList(data)
	}
	// Keep only Costco's entries when the reply names Costco's retailer id;
	// if none match (for example an assumed id), keep every entry.
	filter := false
	for _, x := range list {
		if item, ok := x.(map[string]any); ok && retailerID != "" && etaString(item["retailerId"]) == retailerID {
			filter = true
			break
		}
	}
	var rows []etaRow
	for _, x := range list {
		item, ok := x.(map[string]any)
		if !ok {
			continue
		}
		if filter && etaString(item["retailerId"]) != retailerID {
			continue
		}
		rows = append(rows, etaRowFrom(service, item, now))
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i].EtaSeconds, rows[j].EtaSeconds
		if a == nil || b == nil {
			return a != nil
		}
		return *a < *b
	})
	return rows
}

func findEtaList(v any) []any {
	switch t := v.(type) {
	case []any:
		for _, x := range t {
			if m, ok := x.(map[string]any); ok {
				if _, has := m["etaSeconds"]; has {
					return t
				}
			}
		}
		for _, x := range t {
			if l := findEtaList(x); l != nil {
				return l
			}
		}
	case map[string]any:
		for _, k := range sortedKeys(t) {
			if l := findEtaList(t[k]); l != nil {
				return l
			}
		}
	}
	return nil
}

func etaRowFrom(service string, item map[string]any, now time.Time) etaRow {
	r := etaRow{ServiceType: service}
	if st := strings.ToLower(etaString(item["serviceType"])); st != "" {
		r.ServiceType = st
	}
	if f, ok := item["etaSeconds"].(float64); ok && f > 0 && !math.IsInf(f, 0) {
		s := int64(f)
		r.EtaSeconds = &s
		r.EarliestStart = now.Add(time.Duration(s) * time.Second).Format(time.RFC3339)
	}
	strs := map[string]string{}
	collectEtaStrings("", item, strs)
	if len(strs) > 0 {
		r.Strings = strs
	}
	if end := etaWindowEnd(item, now); end != "" {
		r.EarliestEnd = end
	}
	if start := etaWindowStart(item, now); start != "" && r.EarliestStart == "" {
		r.EarliestStart = start
	}
	r.Label = firstString(strs, "etaString", "deliveryString", "pickupString", "etaLabelString", "titleString", "labelString", "windowString")
	if r.Label == "" {
		for _, k := range sortedStringKeys(strs) {
			if lk := strings.ToLower(k); strings.Contains(lk, "eta") || strings.Contains(lk, "window") || strings.Contains(lk, "time") {
				r.Label = strs[k]
				break
			}
		}
	}
	for _, k := range sortedStringKeys(strs) {
		if lk := strings.ToLower(k); strings.Contains(lk, "fee") || strings.Contains(lk, "price") || strings.Contains(lk, "cost") {
			r.Fee = strs[k]
			break
		}
	}
	return r
}

// collectEtaStrings gathers human text fields (…String, label, title) from an
// ETA item, skipping ids, tracking payloads and URLs.
func collectEtaStrings(prefix string, v any, out map[string]string) {
	m, ok := v.(map[string]any)
	if !ok {
		return
	}
	for _, k := range sortedKeys(m) {
		lk := strings.ToLower(k)
		if isIDKey(k) || strings.Contains(lk, "tracking") || strings.Contains(lk, "image") || strings.Contains(lk, "url") || k == "__typename" {
			continue
		}
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch t := m[k].(type) {
		case string:
			if strings.TrimSpace(t) == "" {
				continue
			}
			if strings.HasSuffix(k, "String") || strings.Contains(lk, "label") || strings.Contains(lk, "title") || strings.Contains(lk, "fee") || strings.Contains(lk, "window") {
				out[key] = t
			}
		case map[string]any:
			collectEtaStrings(key, t, out)
		}
	}
}

func isIDKey(k string) bool {
	return k == "id" || strings.HasSuffix(k, "Id") || strings.HasSuffix(k, "Ids") || strings.HasSuffix(k, "ID") || strings.HasSuffix(k, "Uuid") || strings.EqualFold(k, "uuid")
}

func etaWindowEnd(item map[string]any, now time.Time) string {
	return findTimeField(item, now, "windowEndsAt", "windowEnd", "endsAt", "endAt", "etaWindowEnd", "latestEtaSeconds", "maxEtaSeconds")
}

func etaWindowStart(item map[string]any, now time.Time) string {
	return findTimeField(item, now, "windowStartsAt", "windowStart", "startsAt", "startAt")
}

func findTimeField(v any, now time.Time, keys ...string) string {
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	for _, k := range keys {
		switch t := m[k].(type) {
		case string:
			if ts, err := time.Parse(time.RFC3339, t); err == nil {
				return ts.Local().Format(time.RFC3339)
			}
		case float64:
			if strings.HasSuffix(k, "Seconds") && t > 0 {
				return now.Add(time.Duration(int64(t)) * time.Second).Format(time.RFC3339)
			}
		}
	}
	for _, k := range sortedKeys(m) {
		if s := findTimeField(m[k], now, keys...); s != "" {
			return s
		}
	}
	return ""
}

func firstString(m map[string]string, keys ...string) string {
	for _, want := range keys {
		for _, k := range sortedStringKeys(m) {
			if k == want || strings.HasSuffix(k, "."+want) {
				return m[k]
			}
		}
	}
	return ""
}

func sortedStringKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func etaString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return fmt.Sprintf("%d", int64(t))
	}
	return ""
}

func humanTime(rfc string) string {
	if rfc == "" {
		return "-"
	}
	t, err := time.Parse(time.RFC3339, rfc)
	if err != nil {
		return rfc
	}
	return t.Local().Format("Mon 3:04 PM MST")
}

// etaNoFeeNote explains the missing fee: the header ETA queries carry only
// etaSeconds and display strings (checked live, 2026-10).
const etaNoFeeNote = "fee: the store-header ETA queries return no fee; Costco shows the delivery fee at checkout (see slots windows)"

func anyFee(rows []etaRow) bool {
	for _, r := range rows {
		if r.Fee != "" {
			return true
		}
	}
	return false
}
