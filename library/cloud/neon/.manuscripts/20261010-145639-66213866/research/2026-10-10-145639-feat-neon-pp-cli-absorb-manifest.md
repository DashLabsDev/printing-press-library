# Neon CLI Absorb Manifest

Sources: neonctl (official TS CLI, 117★), mcp-server-neon (official MCP, 650★), Neon API v2 OpenAPI (184 ops).

### Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | List/get/create/update/delete projects | neonctl projects; MCP list_projects/create_project/describe_project | (generated endpoint) projects list/get/create/update/delete | Static Go binary, --json/--select, --dry-run |
| 2 | List/get/create/delete branches | neonctl branches; MCP create_branch/delete_branch/describe_branch | (generated endpoint) branches list/get/create/delete | Name-or-id resolution, typed exit codes |
| 3 | Reset branch to parent / restore to timestamp/LSN | neonctl branches reset/restore; MCP reset_from_parent | (generated endpoint) branches restore | --dry-run preview |
| 4 | Rename / set-default / set-expiration | neonctl branches | (generated endpoint) branches update / set-as-default | agent-native output |
| 5 | Branch schema diff | neonctl branches schema-diff; MCP compare_database_schema | (generated endpoint) branches compare-schema | JSON diff output |
| 6 | Add compute / read replica; edit computes | neonctl branches add-compute (edit requested in open issue) | (generated endpoint) endpoints create/update/start/suspend/restart | Fills open neonctl gap (edit computes) |
| 7 | Connection string | neonctl connection-string; MCP get_connection_string | neon-pp-cli connection-string | Branch/db/role by name, --pooled, psql/env formats |
| 8 | Databases CRUD | neonctl databases; MCP create/delete_postgres_database | (generated endpoint) databases list/get/create/update/delete | |
| 9 | Roles CRUD + reset password + reveal | neonctl roles; MCP create_postgres_role/reset_postgres_role_password | (generated endpoint) roles list/create/delete/reset-password/reveal-password | |
| 10 | Operations list/get | neonctl operations; MCP list_operations/get_operation | (generated endpoint) operations list/get | |
| 11 | Orgs, members, invitations, org API keys | neonctl orgs; MCP list_organizations | (generated endpoint) organizations ... | |
| 12 | API keys list/create/revoke | Neon API | (generated endpoint) api-keys list/create/revoke | |
| 13 | IP allowlist / VPC endpoints | neonctl ip-allow / vpc | (behavior in neon-pp-cli projects update) allowed_ips; (generated endpoint) vpc endpoints | |
| 14 | Snapshots create/list/restore/schedule | MCP create_snapshot/restore_snapshot/set_snapshot_schedule | (generated endpoint) snapshots ... | |
| 15 | Consumption history (account/projects) | Neon API; open neonctl issue | (generated endpoint) consumption ... | Missing from neonctl entirely |
| 16 | Regions list | Neon API | (generated endpoint) regions list | |
| 17 | Neon Auth / Data API / Functions / Buckets | neonctl neon_auth/data_api/functions/bucket; MCP provision_neon_auth etc. | (generated endpoint) auth / data-api / functions / buckets | |
| 18 | Current user / me | neonctl me | (generated endpoint) users me | |
| 19 | Set context (default project/org) | neonctl set-context | neon-pp-cli context set | Persisted defaults in config |
| 20 | Output json/yaml/table | neonctl --output | (behavior in neon-pp-cli projects list) --json/--csv/--select/--agent | |
| 21 | Logs query | MCP query_logs | (generated endpoint) logs ... | |

### Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Stale branches (9/10) | neon-pp-cli stale --idle 14d | hand-code | Local SQLite join of branches (last_active_at, expires_at, default) + endpoints | Use this command for finding idle or expired branches. Do NOT use this command for ranking cost or consumption; use 'usage top' instead. |
| 2 | Fleet tree (8/10) | neon-pp-cli tree | hand-code | Local join orgs→projects→branches→endpoints; one view instead of N×M calls | none |
| 3 | Usage trends (8/10) | neon-pp-cli usage trend --since 30d | hand-code | Synced consumption history with period-over-period deltas | Use this command for usage change over time. Do NOT use this command for ranking which projects consume most; use 'usage top' instead. |
| 4 | Operation wait (8/10) | neon-pp-cli wait --branch pr-123 | hand-code | Polls async operations to completion with typed exit codes and timeout | none |
| 5 | Usage top (7/10) | neon-pp-cli usage top --metric compute --since 7d | hand-code | Aggregate synced consumption over window, rank projects | Use this command for ranking projects by consumption. Do NOT use this command for time series; use 'usage trend' instead. |
| 6 | Preview branch up (7/10) | neon-pp-cli preview up pr-123 --expires 7d | hand-code | Idempotent find-or-create + wait + expiration + connection string in one step | Use this command for idempotently creating or reusing a named preview branch and getting its connection string. Do NOT use this command for removing branches; use 'prune' instead. |
| 7 | Branch prune (7/10) | neon-pp-cli prune --idle 14d --yes | hand-code | Same selection as stale, dry-run default, protected/default skipped | Use this command for deleting idle or expired branches. Do NOT use this command for just listing them; use 'stale' instead. |
| 8 | Compute audit (6/10) | neon-pp-cli compute audit | hand-code | Scans synced endpoints for suspend disabled / fixed autoscaling / oversized CU | none |

Stubs: none.
