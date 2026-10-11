package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/cloud/neon/internal/store"
)

var neonMetrics = []string{"compute", "storage", "transfer", "written"}

type neonUsagePoint struct {
	ProjectID   string             `json:"project_id"`
	ProjectName string             `json:"project_name"`
	Start       string             `json:"period_start"`
	End         string             `json:"period_end"`
	Values      map[string]float64 `json:"values"`
	Basis       string             `json:"basis,omitempty"`
}

func consumptionPoints(db *store.Store) ([]neonUsagePoint, error) {
	projects, err := neonRows(db, "projects")
	if err != nil {
		return nil, err
	}
	names := map[string]string{}
	for _, p := range projects {
		names[ns(p, "id")] = ns(p, "name")
	}
	result := make([]neonUsagePoint, 0)
	merged := map[string]*neonUsagePoint{}
	for _, sources := range [][]string{{"consumption-history-projects", "consumption_history_projects", "consumption-history/projects"}, {"consumption-history-v2-projects", "consumption_history_v2_projects", "consumption-history/v2/projects"}} {
		rows, err := neonRows(db, sources...)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			p := ns(row, "project_id")
			emit := func(o neonObject) {
				start := ns(o, "timeframe_start")
				if start == "" {
					start = ns(o, "period_start")
				}
				end := ns(o, "timeframe_end")
				if end == "" {
					end = ns(o, "period_end")
				}
				if p == "" || start == "" {
					return
				}
				key := p + "/" + start
				point := merged[key]
				if point == nil {
					point = &neonUsagePoint{ProjectID: p, ProjectName: names[p], Start: start, End: end, Values: map[string]float64{}}
					merged[key] = point
				}
				for field, metric := range map[string]string{"compute_time_seconds": "compute", "data_transfer_bytes": "transfer", "written_data_bytes": "written", "compute": "compute", "storage": "storage", "transfer": "transfer", "written": "written"} {
					if v, ok := o[field].(float64); ok {
						point.Values[metric] = v
					}
				}
				// The current API declares v1 storage deprecated and always 0.
				// Retain older nonzero byte-hour observations; prefer v2 below.
				if v, ok := o["data_storage_bytes_hour"].(float64); ok && v > 0 {
					point.Values["storage"] = v
				}
				if metrics, ok := o["metrics"].([]any); ok {
					storage, transfer := 0.0, 0.0
					hasStorage, hasTransfer := false, false
					for _, raw := range metrics {
						m, ok := raw.(map[string]any)
						if !ok {
							continue
						}
						v, valueOK := m["value"].(float64)
						if !valueOK {
							continue
						}
						switch ns(m, "metric_name") {
						case "compute_unit_seconds":
							point.Values["compute"] = v
						case "root_branch_bytes_month", "child_branch_bytes_month":
							storage += v
							hasStorage = true
						case "public_network_transfer_bytes", "private_network_transfer_bytes":
							transfer += v
							hasTransfer = true
						}
					}
					if hasStorage {
						point.Values["storage"] = storage
					}
					if hasTransfer {
						point.Values["transfer"] = transfer
					}
				}
			}
			if periods, ok := row["periods"].([]any); ok {
				for _, r := range periods {
					period, ok := r.(map[string]any)
					if !ok {
						continue
					}
					if consumption, ok := period["consumption"].([]any); ok {
						for _, c := range consumption {
							if o, ok := c.(map[string]any); ok {
								emit(o)
							}
						}
					} else {
						emit(period)
					}
				}
			} else {
				emit(row)
			}
		}
	}
	for _, p := range merged {
		result = append(result, *p)
	}
	// Plans below Launch/Scale have no consumption history. Choose the source
	// per project: history when the project has any, sync snapshots otherwise,
	// never both for the same project.
	withHistory := map[string]bool{}
	for _, p := range result {
		withHistory[p.ProjectID] = true
	}
	snaps, err := snapshotPoints(db, names)
	if err != nil {
		return nil, err
	}
	for _, p := range snaps {
		if !withHistory[p.ProjectID] {
			result = append(result, p)
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].ProjectID != result[j].ProjectID {
			return result[i].ProjectID < result[j].ProjectID
		}
		return result[i].Start < result[j].Start
	})
	sort.Slice(result, func(i, j int) bool {
		if result[i].ProjectID != result[j].ProjectID {
			return result[i].ProjectID < result[j].ProjectID
		}
		return result[i].Start < result[j].Start
	})
	return result, nil
}

type neonUsageRank struct {
	ProjectID   string  `json:"project_id"`
	ProjectName string  `json:"project_name"`
	Metric      string  `json:"metric"`
	Value       float64 `json:"value"`
	Basis       string  `json:"basis,omitempty"`
}

func rankNeonUsage(points []neonUsagePoint, metric string, since, now time.Time) []neonUsageRank {
	sums := map[string]*neonUsageRank{}
	for _, p := range points {
		t, ok := parseRestSyncTimestamp(p.Start)
		if !ok || t.Before(since) || t.After(now) {
			continue
		}
		value, ok := p.Values[metric]
		if !ok {
			continue
		}
		r := sums[p.ProjectID]
		if r == nil {
			r = &neonUsageRank{ProjectID: p.ProjectID, ProjectName: p.ProjectName, Metric: metric, Basis: p.Basis}
			sums[p.ProjectID] = r
		}
		// Snapshot storage is a point-in-time size, not usage: take the peak.
		if metric == "storage" && strings.HasPrefix(p.Basis, neonSnapshotBasis) {
			if value > r.Value {
				r.Value = value
			}
			continue
		}
		r.Value += value
	}
	result := make([]neonUsageRank, 0)
	for _, r := range sums {
		result = append(result, *r)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Value != result[j].Value {
			return result[i].Value > result[j].Value
		}
		return result[i].ProjectID < result[j].ProjectID
	})
	return result
}

type neonUsageDelta struct {
	ProjectID   string   `json:"project_id"`
	ProjectName string   `json:"project_name"`
	Metric      string   `json:"metric"`
	Start       string   `json:"period_start"`
	End         string   `json:"period_end"`
	Value       float64  `json:"value"`
	Previous    *float64 `json:"previous"`
	Delta       *float64 `json:"delta"`
	Percent     *float64 `json:"delta_percent"`
	Basis       string   `json:"basis,omitempty"`
}

func trendNeonUsage(points []neonUsagePoint, since, now time.Time) []neonUsageDelta {
	result := make([]neonUsageDelta, 0)
	previous := map[string]neonUsagePoint{}
	for _, p := range points {
		t, ok := parseRestSyncTimestamp(p.Start)
		if !ok || t.Before(since) || t.After(now) {
			continue
		}
		for _, metric := range neonMetrics {
			value, ok := p.Values[metric]
			if !ok {
				continue
			}
			r := neonUsageDelta{ProjectID: p.ProjectID, ProjectName: p.ProjectName, Metric: metric, Start: p.Start, End: p.End, Value: value, Basis: p.Basis}
			key := p.ProjectID + "/" + metric
			if last, ok := previous[key]; ok && last.End == p.Start && last.Basis == p.Basis {
				v := last.Values[metric]
				delta := value - v
				r.Previous = &v
				r.Delta = &delta
				if v != 0 {
					percent := delta / v * 100
					r.Percent = &percent
				}
			}
			previous[key] = p
			result = append(result, r)
		}
	}
	return result
}
func validateNeonMetric(metric string) error {
	for _, m := range neonMetrics {
		if m == metric {
			return nil
		}
	}
	return usageErr(fmt.Errorf("--metric must be compute, storage, transfer or written"))
}

// storageRankPoints returns points for ranking storage on one measurement.
// History reports byte-months while snapshots report current bytes, so they
// must not be mixed: every sync snapshots every project, so rank on snapshots
// when any exist and fall back to history only when none do.
func storageRankPoints(db *store.Store) ([]neonUsagePoint, error) {
	projects, err := neonRows(db, "projects")
	if err != nil {
		return nil, err
	}
	names := map[string]string{}
	for _, p := range projects {
		names[ns(p, "id")] = ns(p, "name")
	}
	snaps, err := snapshotPoints(db, names)
	if err != nil {
		return nil, err
	}
	if len(snaps) > 0 {
		// A storage reading belongs to the moment its snapshot was taken, not
		// to the interval since the previous sync, so window it by End.
		for i := range snaps {
			snaps[i].Start = snaps[i].End
		}
		return snaps, nil
	}
	return consumptionPoints(db)
}
