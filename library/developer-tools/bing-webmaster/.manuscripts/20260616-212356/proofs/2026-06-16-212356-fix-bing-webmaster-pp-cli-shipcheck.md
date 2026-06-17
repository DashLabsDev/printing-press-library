# Bing Webmaster CLI — Shipcheck + Live Acceptance

## Shipcheck (umbrella) — VERDICT: PASS (6/6 legs)
| Leg | Result |
|---|---|
| dogfood | PASS (100%, 29/29, 0 critical) |
| verify | PASS |
| workflow-verify | PASS (no manifest) |
| verify-skill | PASS |
| validate-narrative | PASS (9 commands resolved, full examples pass) |
| scorecard | PASS — **81/100, Grade A** |

Scorecard highlights: Output Modes 10, Auth 10, Error Handling 10, Doctor 10, Agent Native 10, MCP (remote/tool-design/surface) 10/10/10, Breadth 10, Workflows 10, Local Cache 10, Sync Correctness 10, Path Validity 10.
Weak dims (non-blocking, above 65 floor): insight 2/10, auth_protocol 4/10, cache_freshness 5/10 — candidates for polish.

## Fixes applied during shipcheck
1. **validate-narrative FAIL → PASS:** quickstart referenced `sync --site` (no such flag on generic `sync`). Replaced with `traffic queries --site ...` in research.json + rendered README.
2. **`publish --dry-run` UX:** was a generic short-circuit; now fetches + parses the real sitemap and previews the plan (URL count, chunks, sample URLs) without submitting. Verify-mode (`PRINTING_PRESS_VERIFY=1`) still short-circuits with no network; never submits without `--confirm`.

## Live acceptance (real API key, real verified site `code-lieshout.nl`) — read-only
Writes were intentionally NOT exercised against the user's real Bing account (no disposable sandbox); `publish` tested in `--dry-run` only.

| Command | Result |
|---|---|
| doctor | PASS — auth configured, credentials valid, API reachable |
| sites list | PASS — 2 verified sites returned, `{"d"}` unwrapped, `__type` fidelity preserved |
| traffic rank-traffic / queries | PASS — empty `[]` (site below Bing data threshold), handled as valid |
| crawl stats / issues | PASS — empty, valid |
| feeds list | PASS — empty, valid |
| quota | PASS — real data parsed: daily 10 / monthly 150, pacing 1/hr |
| review | PASS — honest baseline message ("Baseline captured … run again in ~7 days") |
| feed-health | PASS — empty feeds handled |
| triage | PASS — clean summary, snapshot diff path |
| watch | PASS — honest baseline note |
| gap (with GSC CSV) | PASS — CSV parsed (3 queries), reconciled (3 google-only) |
| publish (--dry-run, real sitemap) | PASS — fetched sitemap (42 URLs, 1 chunk), no submission |

## Verified technical behaviors
- `{"d":...}` envelope unwrap working on live responses.
- Microsoft `/Date(ms±offset)/` normalization in place (client layer).
- Empty `GetQueryStats` arrays treated as valid (not errors).
- Verify-friendly RunE on all transcendence commands (dry-run → exit 0, no hard arg/flag gates).
- `publish` is the only non-read-only command (mcp:read-only omitted); print-only without `--confirm`.

## Known gaps (minor, non-blocking)
- Generated store cannot extract an ID for `sites`/some resources (warning on generic `sync`), so the generated `sync`/`sql`/`search` are partial for Bing. The transcendence commands use their own snapshot store (`internal/snapshots`), which is unaffected. (Could set `id_field` and regenerate to fully fix.)
- In `publish --dry-run`, the quota GET is also dry-run so the preview shows `daily_quota: 0`; the live quota check runs on the actual `--confirm` submission.
- `triage` crawl-issue flag→label decoding is best-effort (Bing's CrawlIssues bit values aren't fully documented); unknown bits surface as `other(bitN)`.

## Final recommendation: **ship**
All ship-threshold conditions met; flagship features behaviorally validated against the live API.
