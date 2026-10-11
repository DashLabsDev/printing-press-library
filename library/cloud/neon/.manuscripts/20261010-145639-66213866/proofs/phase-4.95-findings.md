# Phase 4.95 local code review
- Review path: direct subagent dispatch (general-purpose reviewer; correctness/safety/security/reliability/maintainability in one persona). Codex Sol 6.1 was attempted first and failed to launch (config load error).
- Autofix summary: 17 findings autofixed in-place across 3 rounds (11 + 5 + 1); see commits c269732 7625845 94ce62c 15c38cd 2bd03b8 21b39be 
- Convergence: round 3 left one high finding (preview up 409 path waited on other branches' operations); fixed with a targeted test (TestNeonPreviewUpConflictWaitsOnlyOnThisBranch). Cap reached; no outstanding in-scope findings known.
- Post-fix /simplify: not run.

## Template-shape / out-of-scope retro candidates
- Generated sync treats Neon's echoed cursor on an empty final page as stuck_pagination (sync.go cursor loops); fixed locally, generator bug.
- defaultSyncResources picked org-key-incompatible resources (/users/me, /regions, /api_keys) and omitted projects.
- verify-skill resolves novel subcommands registered via registerNovelCommand by file basename only (context set collided with client source set).
- Generated MCP tests (internal/mcp) fail when the auth env var is exported (credential-scoped store path).
- Regen with hand edits to generated sync.go is not durable; patch records correctly block but require manual re-apply.
