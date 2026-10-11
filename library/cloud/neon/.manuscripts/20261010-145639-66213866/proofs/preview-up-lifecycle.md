# preview up — live write lifecycle proof
Approved by the user in-session (one throwaway branch, 1h expiry, deleted immediately).
Sandbox: a throwaway branch named pp-dogfood-preview in the test workspace's project; connection-string secrets redacted.

1. `neon-pp-cli preview up pp-dogfood-preview --project-id <test-project> --expires 1h --json` → exit 0; created branch br-autumn-dream-…, waited on its create operations, returned connection_uri (database neondb, role neondb_owner).
2. Same command again → exit 0; same branch_id returned, no second create (idempotent). Live branch list showed exactly one pp-dogfood-preview.
3. `neon-pp-cli wait --branch pp-dogfood-preview --project-id <test-project> --timeout 2m --json` → exit 0, status finished.
Cleanup: `neon-pp-cli projects branches delete-project-branch <test-project> <branch-id> --yes --json` → exit 0; live branch list afterwards contained 0 branches named pp-dogfood-preview.
