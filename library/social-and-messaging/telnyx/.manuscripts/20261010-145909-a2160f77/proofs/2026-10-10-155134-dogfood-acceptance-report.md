Acceptance Report: telnyx
  Level: Full Dogfood (writes dry-run only per user)
  Tests: 686/686 passed (721 skipped: mutating dry-run-only / --allow-destructive not granted, blocked fixtures for account-specific IDs, no-positional, destructive-at-auth)
  Loops: 4 runs (156 -> 35 -> 2 -> 0 failures)
  Fixes applied:
    - CLI fix: 124 generated commands lacked Examples (underivable IDs); added examples and `<id>=example-value` happy-args (blocked-fixture) instead of spec doc IDs that 404 on a real account.
    - CLI fix: detail-records defaults filter[record_type]=messaging (API 400 without it); --filter help now says JSON.
    - CLI fix: conversation rejects non-phone input with exit 2.
    - CLI fix: spec path /10dlc/campaign/usecase_cost 404s live; get-campaign-cost now calls /10dlc/campaign/usecase/cost (verified 200).
    - CLI fix: 4 generated list-by-parent endpoints that return 200 for unknown IDs annotated pp:no-error-path-probe.
    - CLI fix (template-shaped): jobs list/prune and media download dry-run envelopes; tail examples named a real resource.
  Printing Press issues (retro):
    - Generated endpoint commands omit Example when IDs are underivable, which then fails dogfood help checks (124 rows).
    - Spec doc example IDs used as happy-path fixtures guarantee 404 on real accounts.
    - jobs list/prune templates do not short-circuit --dry-run; binary-download dry-run sentinel lacks `action`.
    - tail template example `tail --interval 10s` errors (resource required).
    - Cross-spec regen (--force after a spec path removal) falls back to novel-only preservation and silently drops hand edits to generated files.
    - deepObject `page` not expanded to page[number]/page[size]; detail_records silent page-size cap.
    - dogfood novel-command scanner ignores addNovelCommandIfAbsent wiring (forced rename send -> text).
  Gate: PASS
