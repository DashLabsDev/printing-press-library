# neon-pp-cli shipcheck

## Loop 1 (pipeline/shipcheck1.log)
- verify PASS, validate-narrative PASS, workflow-verify PASS, apify-audit PASS, verify-skill PASS, scorecard 96/100 A.
- dogfood FAIL: examples 2/10 — generated endpoint commands omitted examples because path params (project_id, branch_id, ...) had no spec examples.

## Fix
- Added realistic `example` values to 248 parameters in the overlay spec (project_id lucky-sun-123456, branch_id br-wispy-meadow-a1b2c3d4, ...), regenerated. Regen dropped hand edits to generated sync.go (caught by .printing-press-patches guard); re-applied from pipeline/sync-hand.patch. Full test suite green.

## Loop 2 (pipeline/shipcheck2.log)
- 7/7 legs PASS. verify PASS; dogfood PASS (examples 8/10, novel 8/8, 0 dead flags/functions); scorecard 96/100 Grade A.

## Behavioral check (live, read-only, org API key)
- tree: org → 2 projects → 6 branches nested by ancestry → 6 endpoints. Correct.
- stale --idle 3d: production (default, no delete command) + local-dev. prune --idle 3d (preview): local-dev only (default skipped). Correct.
- compute audit: [] — all 6 endpoints 0.25–2 CU with default suspend. Correct.
- usage top/trend: plan lacks consumption history (Launch/Scale only) → snapshot fallback, basis labeled; the test workspace 52,424 compute-seconds period-to-date. Trend deltas null with one snapshot (no fabrication).
- wait --branch local-dev: exit 0, status finished.
- connection-string: resolves branch by name, infers db/role, --format env; unknown branch → exit 3.
- preview up: dry-run only (no live writes, per read-only approval); create/idempotency covered by httptest unit tests.

## Verdict: ship
