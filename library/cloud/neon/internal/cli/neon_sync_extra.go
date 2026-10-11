// Hand-authored Neon sync adapter. The small dispatch points in sync.go are
// recorded in .printing-press-patches so reprints cannot silently drop them.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/cloud/neon/internal/store"
)

type neonSyncGetter interface {
	Get(context.Context, string, map[string]string) (json.RawMessage, error)
}

func syncNeonConsumption(ctx context.Context, c neonSyncGetter, db *store.Store, resource, sinceTS string, maxPages int, user *syncUserParams, events io.Writer) (result syncResult) {
	started := time.Now()
	result = syncResult{Resource: resource}
	defer func() { result.Duration = time.Since(started) }()
	if preview, ok := c.(interface{ IsDryRun() bool }); ok && preview.IsDryRun() {
		fmt.Fprintf(events, "{\"event\":\"sync_dryrun\",\"resource\":%q}\n", resource)
		return result
	}
	now := time.Now().UTC().Truncate(24 * time.Hour)
	params := map[string]string{"from": now.Add(-30 * 24 * time.Hour).Format(time.RFC3339), "to": now.Format(time.RFC3339), "granularity": "daily", "limit": "100"}
	if sinceTS != "" {
		params["from"] = sinceTS
	}
	path := "/consumption_history/projects"
	if resource == "consumption-history-v2-projects" {
		path = "/consumption_history/v2/projects"
		params["metrics"] = "compute_unit_seconds,root_branch_bytes_month,child_branch_bytes_month,public_network_transfer_bytes,private_network_transfer_bytes"
	}
	user.applyTo(resource, params, false)
	orgs := make([]string, 0)
	if org := params["org_id"]; org != "" {
		orgs = append(orgs, org)
	} else {
		// GET /projects works for organization keys; /users/me does not. Resolve org
		// scope from API-owned projects, without racing the parallel project sync.
		query := map[string]string{"limit": "100"}
		seen := map[string]bool{}
		orgSet := map[string]bool{}
		for {
			raw, err := c.Get(ctx, "/projects", query)
			if err != nil {
				result.Err = err
				return result
			}
			var env struct {
				Projects   []neonObject `json:"projects"`
				Pagination struct {
					Cursor string `json:"cursor"`
				} `json:"pagination"`
			}
			if err := json.Unmarshal(raw, &env); err != nil {
				result.Err = err
				return result
			}
			if env.Projects == nil {
				result.Err = fmt.Errorf("missing projects in org discovery response")
				return result
			}
			for _, p := range env.Projects {
				if org := ns(p, "org_id"); org != "" {
					orgSet[org] = true
				}
			}
			next := env.Pagination.Cursor
			// Neon echoes the last cursor on an empty final page.
			if next == "" || len(env.Projects) == 0 {
				break
			}
			if seen[next] {
				result.Err = fmt.Errorf("repeated project cursor in org discovery")
				return result
			}
			seen[next] = true
			query["cursor"] = next
		}
		for org := range orgSet {
			orgs = append(orgs, org)
		}
		sort.Strings(orgs)
	}
	if len(orgs) == 0 {
		result.Warn = fmt.Errorf("no project organization available for consumption; pass --resource-param %s:org_id=...", resource)
		return result
	}
	if err := db.SaveSyncProgress(resource, "", 0); err != nil {
		result.Err = err
		return result
	}
	for _, org := range orgs {
		query := map[string]string{}
		for k, v := range params {
			query[k] = v
		}
		query["org_id"] = org
		seen := map[string]bool{}
		pages := 0
		for {
			raw, err := c.Get(ctx, path, query)
			if neonPlanGated(err) {
				// Keep going: other organizations may be on a plan with history.
				result.Warn = fmt.Errorf("%s not available on this Neon plan for some organizations; their projects fall back to sync snapshots", resource)
				fmt.Fprintf(events, "{\"event\":\"sync_warning\",\"resource\":%q,\"org_id\":%q,\"reason\":\"plan_gated\",\"message\":\"consumption history requires a Launch/Scale plan; usage commands use sync snapshots for this organization\"}\n", resource, org)
				break
			}
			if err != nil {
				result.Err = err
				return result
			}
			var env struct {
				Projects   []neonObject `json:"projects"`
				Pagination struct {
					Cursor string `json:"cursor"`
				} `json:"pagination"`
			}
			if err := json.Unmarshal(raw, &env); err != nil {
				result.Err = err
				return result
			}
			if env.Projects == nil {
				result.Err = fmt.Errorf("missing projects in consumption response")
				return result
			}
			items, err := flattenNeonConsumption(env.Projects)
			if err != nil {
				result.Err = err
				return result
			}
			for _, item := range items {
				data, err := json.Marshal(item)
				if err != nil {
					result.Err = err
					return result
				}
				if err := db.Upsert(resource, ns(item, "id"), data); err != nil {
					result.Err = err
					return result
				}
				result.Count++
			}
			pages++
			next := env.Pagination.Cursor
			// Neon echoes the last cursor on an empty final page.
			if next == "" || len(env.Projects) == 0 {
				break
			}
			if seen[next] {
				result.Err = fmt.Errorf("repeated consumption cursor")
				return result
			}
			if maxPages > 0 && pages >= maxPages {
				result.Warn = fmt.Errorf("consumption sync incomplete: max-pages cap")
				return result
			}
			seen[next] = true
			query["cursor"] = next
		}
	}
	if err := db.SaveSyncState(resource, "", result.Count); err != nil {
		result.Err = err
		return result
	}
	result.Duration = time.Since(started)
	fmt.Fprintf(events, "{\"event\":\"sync_complete\",\"resource\":%q,\"total\":%d}\n", resource, result.Count)
	return result
}
func flattenNeonConsumption(projects []neonObject) ([]neonObject, error) {
	out := make([]neonObject, 0)
	for _, p := range projects {
		project := ns(p, "project_id")
		if project == "" {
			return nil, fmt.Errorf("consumption project missing project_id")
		}
		periods, ok := p["periods"].([]any)
		if !ok {
			return nil, fmt.Errorf("consumption project missing periods")
		}
		for _, raw := range periods {
			period, ok := raw.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("invalid consumption period")
			}
			consumption, ok := period["consumption"].([]any)
			if !ok {
				return nil, fmt.Errorf("consumption period missing consumption")
			}
			for _, raw := range consumption {
				point, ok := raw.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("invalid consumption timeframe")
				}
				start := ns(point, "timeframe_start")
				if _, ok := parseRestSyncTimestamp(start); !ok {
					return nil, fmt.Errorf("consumption missing valid timeframe_start")
				}
				point["project_id"] = project
				point["period_id"] = ns(period, "period_id")
				point["period_start"] = ns(period, "period_start")
				point["id"] = project + "/" + ns(period, "period_id") + "/" + start
				out = append(out, point)
			}
		}
	}
	return out, nil
}
