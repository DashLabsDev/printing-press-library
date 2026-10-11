# Neon CLI Brief

## API Identity
- Domain: Neon serverless Postgres control plane (console.neon.tech/api/v2). Official OpenAPI 3.0.3 at https://neon.tech/api_spec/release/v2.json (redirects to neon.com), 125 paths / 184 operations.
- Tag breakdown: Auth 29, Branch 28, Project 24, Organizations 19, Auth(legacy) 16, Functions 13, Buckets 8, Endpoint 8, Snapshot 7, Credentials 5, Realtime 5, DataAPI 4, Users 4, API Key 3, Consumption 3, Logs 3, Operation 2, AI Gateway 1, Region 1, Storage 1.
- Users: app developers using branch-per-PR/preview workflows, platform teams managing many projects/orgs, agents provisioning databases.
- Data profile: low-volume, high-value control-plane metadata (projects, branches, endpoints, databases, roles, operations) plus time-series consumption metrics.
- Auth: Bearer API key (`NEON_API_KEY`, same env var as neonctl). Cookie auth schemes in spec are console-only; ignore.

## Reachability Risk
- None. Official, documented, stable v2 API; spec fetched 200. Local key available (user-approved, read-only smoke only).

## Top Workflows
1. Branch-per-PR lifecycle: create branch from parent (optionally at timestamp/LSN), get connection string, reset to parent, delete/expire.
2. Fleet inventory: list projects across orgs → branches → endpoints/computes, with state, last-active, size.
3. Cost/usage: consumption history per project/branch (compute-hours, storage, transfer) — neonctl lacks this (open issue "Add `neonctl consumption`").
4. Operation tracking: wait on async operations after create/reset/restore.
5. Snapshots / point-in-time restore and schema-diff between branches.

## Table Stakes (neonctl parity)
- projects list/create/get/update/delete; branches list/create/reset/restore/rename/set-default/set-expiration/add-compute/delete/get/schema-diff; databases; roles (incl. reset password); connection-string; operations list; orgs list; ip-allow; vpc endpoints; set-context; --output json/yaml/table; --api-key / NEON_API_KEY.
- Neon MCP server (650★): run_sql, describe schema, prepare/complete migration on temp branch, slow queries, query tuning, compare schema, snapshots, logs.

## Pain points (neonctl issues)
- Requires Node; Windows binary hangs; no macOS arm64 prebuilt; Node version breakages (v24 unsupported, crash on 18.5) → a single static Go binary is a real win.
- No consumption/usage history command (open).
- Can't edit computes (open feature request).
- "Last Active" missing from branch list; wanted tree view of projects→branches.
- Exit codes not distinct for "already exists".

## Data Layer
- Primary entities: projects, branches, endpoints, databases, roles, operations, snapshots, consumption samples.
- Sync cursor: projects pagination cursor; branches per project; consumption by `from`/`to` window, stored incrementally.
- FTS/search: projects/branches/databases by name across all orgs offline.

## Product Thesis
- Name: neon-pp-cli
- Why: zero-dependency Go binary covering the whole v2 API (incl. consumption, snapshots, data API, auth), agent-native JSON, plus a local mirror enabling fleet tree, stale-branch cleanup and cost trends neonctl can't do.

## Build Priorities
1. Full generated v2 surface with NEON_API_KEY bearer auth and correct pagination.
2. connection-string + branch lifecycle ergonomics (by name, not id) and `operations wait`.
3. sync + local store: `tree`, `search`, `stale` (idle/expired branches), `usage` trends.
4. Safe-by-default destructive commands (--dry-run / confirmation).

## Reachability Gate
- Decision: PASS. GET /projects with bearer key -> 200; unauthenticated GET /regions -> 401.
