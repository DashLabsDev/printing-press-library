Manifest transcendence rows: 8 planned, 8 built. Phase 3 will not pass until all 8 ship.

## Built
- Generated: full Neon API v2 surface (184 ops), NEON_API_KEY bearer auth, Cloudflare MCP pattern (code orchestration), learn loop seeds, cache freshness 24h.
- Transcendence (8/8): stale, tree, usage trend, usage top, wait, preview up, prune, compute audit. Implemented by Codex (gpt-6.1-sol), reviewed by Claude.
- Absorbed clean paths: connection-string (branch/db/role by name, --pooled, --format uri|env|psql), context set/show/clear (NEON_CONTEXT_FILE override; default project for connection-string, preview up, wait). Built by Claude after completion gate caught them missing.

## Data layer fixes (Priority 0)
- sync default list now projects + users-me-organizations + consumption (org keys 404 on /users/me, /regions, /api_keys).
- endpoints added as a projects dependent; branch ancestry preserved as branch_parent_id.
- Neon pagination: empty final page echoes the last cursor; generated sync treated it as stuck (projects/operations never marked complete). Fixed in syncResource, dependent loop, neonLiveList and consumption paging.
- Consumption history is plan-gated (Launch/Scale). Gate now a sync warning; each sync records per-project usage snapshots (neon_usage_snapshots) and usage top/trend fall back to period-to-date baseline + increments, labeled basis=sync-snapshots.

## Review fixes on Codex output
- stale no longer prints a delete command for default/protected branches.
- usage commands hinted "not synced" against the consumption resource; now keyed to projects.

## Deferred / limitations
- Generated typed command names are operationId-derived and verbose (e.g. projects branches list-project); not renamed.
- projects_members 404 for org keys (sync error, non-fatal).
- Generated MCP tests (internal/mcp TestMCPPathResolution*, TestMCP*MissingStore*) fail when NEON_API_KEY is exported: credential-scoped store filename. Pass with key unset. Generator issue.
- Edits to generated sync.go (default list, dependents, pagination end, snapshot hook) are not regen-durable; retro item.
- Complex body fields fall back to --body-json: auth email_server, email_provider, branch triggers.
