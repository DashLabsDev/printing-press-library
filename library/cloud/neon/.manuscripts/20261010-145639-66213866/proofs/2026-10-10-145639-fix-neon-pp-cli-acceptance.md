Acceptance Report: neon
  Level: Full Dogfood (live, read-only org API key; mutations dry-run only)
  Tests: 370/370 passed (runs: 613→169 fail, 374→40, 383→4, 370→0)
  Hollow features: preview up (mutating; dry-run only by user's read-only approval), wait (needs an account-specific project; covered by httptest unit tests and a manual live run on the test workspace: exit 0)
  Fixes applied: 6
    - x-happy-args on ID-path operations so the runner resolves real IDs from the synced store instead of spec example IDs (CLI fix)
    - Neon error hints: org-API-key and plan-gated refusals no longer say "resource not found" (CLI fix, internal/cli/neon_errors.go hooked from classifyAPIError)
    - tail: Neon examples + runnable happy-args (CLI fix, hook file)
    - Example synthesis for nested-body write commands (CLI fix, hook file) + required body field examples in spec
    - consumption v2 happy-args placeholders for account-specific org/project IDs (CLI fix)
    - Regen dropped generic pm_* commands (generator choice changed with spec)
  Printing Press issues: 4
    - Spec param examples become authoritative pp:happy-args, overriding store/companion ID resolution
    - Feature-absent phrase list misses Neon's wording ("not allowed for organization API keys", "included with Launch plans and above")
    - Companion list resolution only matches leaf "list"; operationId-derived names like list-project never match
    - tail template examples reference non-existent resources (messages/events)
  Gate: PASS
