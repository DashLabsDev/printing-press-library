package cli

import "encoding/json"

// applyTelnyxSyncDefaults fills per-resource query params the Telnyx API
// requires but the spec does not mark as defaults. GET /detail_records
// returns 400 "Record type is missing" without filter[record_type], and the
// messaging records are what conversation, spend, failures and inbox read.
// An explicit --resource-param detail-records:filter[record_type]=... wins.
func applyTelnyxSyncDefaults(p *syncUserParams) {
	if p.perResource == nil {
		p.perResource = map[string]map[string]string{}
	}
	dr := p.perResource["detail-records"]
	if dr == nil {
		dr = map[string]string{}
		p.perResource["detail-records"] = dr
	}
	if _, ok := dr["filter[record_type]"]; !ok {
		dr["filter[record_type]"] = "messaging"
	}
}

// withDefaultRecordType adds record_type=messaging to a --filter JSON object
// when the caller did not choose one. Non-object JSON is returned unchanged
// so the generated parser reports the error.
func withDefaultRecordType(raw string) string {
	if raw == "" {
		return `{"record_type":"messaging"}`
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil || m == nil {
		// Not a JSON object (including literal null): leave it for the
		// generated parser to accept or reject; never index a nil map.
		if m == nil && err == nil {
			return `{"record_type":"messaging"}`
		}
		return raw
	}
	if _, ok := m["record_type"]; ok {
		return raw
	}
	m["record_type"] = "messaging"
	b, err := json.Marshal(m)
	if err != nil {
		return raw
	}
	return string(b)
}

// withAllPageParams moves a --page JSON object ({"number":3,"size":20}) into
// the paging loop's page[number]/page[size] params when --all is set, so the
// loop starts from the requested page instead of page 1 with the page
// embedded in a fixed path. Without --all, params are returned unchanged.
func withAllPageParams(fetchAll bool, rawPage string, params map[string]string) map[string]string {
	if !fetchAll || rawPage == "" {
		return params
	}
	var page map[string]any
	if err := json.Unmarshal([]byte(rawPage), &page); err != nil || page == nil {
		return params
	}
	if params == nil {
		params = map[string]string{}
	}
	for field, key := range map[string]string{"number": "page[number]", "size": "page[size]"} {
		if v, ok := page[field]; ok && v != nil {
			params[key] = formatCLIParamValue(v)
		}
	}
	return params
}
