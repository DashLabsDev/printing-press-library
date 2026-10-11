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
