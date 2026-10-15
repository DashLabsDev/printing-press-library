
=== verify ===
Runtime Verification: <runstate>/documents-8236339c/runs/20261013-134846/working/ofa-health-clearances-pp-cli/ofa-health-clearances-pp-cli
Mode: mock

COMMAND                        KIND         HELP   DRY-RUN  EXEC     SCORE
agent-context                  write        PASS   PASS     PASS     3/3
api                            local        PASS   PASS     PASS     3/3
backend                        read         PASS   PASS     PASS     3/3
breeds                         write        PASS   PASS     FAIL     2/3
doctor                         local        PASS   PASS     PASS     3/3
dog                            read         PASS   PASS     FAIL     2/3
feedback                       write        PASS   PASS     PASS     3/3
kennels                        read         PASS   PASS     FAIL     2/3
learnings                      read         PASS   PASS     PASS     3/3
playbook                       write        PASS   PASS     PASS     3/3
profile                        write        PASS   PASS     PASS     3/3
recall                         read         PASS   PASS     PASS     3/3
results                        read         PASS   PASS     FAIL     2/3
teach                          write        PASS   PASS     PASS     3/3
teach-lookup                   write        PASS   PASS     PASS     3/3
teach-pattern                  write        PASS   PASS     PASS     3/3
teach-playbook                 write        PASS   PASS     PASS     3/3
tests                          write        PASS   PASS     FAIL     2/3
which                          read         PASS   PASS     PASS     3/3

Path-Param Probes (nested commands with <positional> args):
  PASS learnings confirm
  PASS learnings forget
  PASS learnings reject
  PASS profile delete
  PASS profile save
  PASS profile show
  PASS profile use

Data Pipeline: SKIP (CLI has no sync command)
Pass Rate: 100% (26/26 passed, 0 critical)
Verdict: PASS

=== validate-narrative ===
OK: 9 narrative commands resolved and full examples passed

=== dogfood ===
dogfood: using spec <runstate>/documents-8236339c/runs/20261013-134846/working/ofa-health-clearances-pp-cli/spec.yaml (bundled)
dogfood: caller --spec=<runstate>/documents-8236339c/runs/20261013-134846/research/ofa-health-clearances-spec.yaml overridden by bundled <runstate>/documents-8236339c/runs/20261013-134846/working/ofa-health-clearances-pp-cli/spec.yaml
Dogfood Report: ofa-health-clearances-pp-cli
================================

Path Validity:     0/0 valid (SKIP)
  Detail: internal-yaml spec: paths validated at parse time

Auth Protocol:     MATCH
  Generated: Uses "unknown" prefix
  Detail: no bot/bearer/basic scheme detected

OAuth Scope Cover: 0/0 endpoints covered (SKIP)
  Detail: no OAuth-scoped endpoints in spec

Dead Flags:        0 dead (PASS)

Dead Functions:    9 dead (WARN)
  - booleanNoValueFlags (defined, never called)
  - booleanSpaceFormPositionalErr (defined, never called)
  - compactFields (defined, never called)
  - endpointArgsUsageErr (defined, never called)
  - endpointPositionalArgs (defined, never called)
  - endpointUsageLine (defined, never called)
  - paginatedGetWithResponsePath (defined, never called)
  - retainCLIQueryParams (defined, never called)
  - retainExplicitQueryParams (defined, never called)

Data Pipeline:     PARTIAL
  Sync: SKIP (no sync command)
  Search: uses generic Search only or direct SQL
  Domain tables: 4

Examples:          10/10 commands have examples (PASS)

Novel Features:    1/1 survived (PASS)

MCP Surface:       PASS (MCP surface mirrors the Cobra tree at runtime)

Verdict: WARN
  - 9 dead helper functions found

=== workflow-verify ===
Workflow Verification: ofa-health-clearances-pp-cli
================================

Overall Verdict: workflow-pass
  - no workflow manifest found, skipping

=== apify-audit ===
Apify Actor Audit
=================

No Apify actor references found.

Verdict: pass
  - no Apify actor references found, skipping

=== verify-skill ===
=== ofa-health-clearances-pp-cli ===
  ✓ All checks passed (flag-names, flag-commands, positional-args, shell-var-quotes, unknown-command)
  ✓ canonical-sections passed

=== scorecard ===
Quality Scorecard: ofa-health-clearances

  Output Modes         10/10
  Auth                 10/10
  Error Handling       8/10
  Terminal UX          10/10
  README               10/10
  Doctor               10/10
  Agent Native         10/10
  MCP Quality          6/10
  MCP Desc Quality     N/A
  MCP Token Efficiency 10/10
  MCP Remote Transport 10/10
  MCP Tool Design      10/10
  MCP Surface Strategy N/A
  Local Cache          10/10
  Cache Freshness      3/10
  Breadth              7/10
  Vision               5/10
  Workflows            6/10
  Insight              4/10
  Agent Workflow       9/10

  Domain Correctness
  Path Validity           10/10
  Auth Protocol           N/A
  Data Pipeline Integrity 7/10
  Sync Correctness        7/10
  Live API Verification   N/A
  Type Fidelity           5/5
  Dead Code               1/5

  Total: 80/100 - Grade A
  Note: omitted from denominator: mcp_description_quality, mcp_surface_strategy, auth_protocol, live_api_verification

Sample Output Probe (live command sample)
  Binary refresh: rebuilt (staged binary was older than Go sources)
  Passed: 1/1  (100% pass rate, 0 skipped)

Gaps:
  - insight scored 4/10 - needs improvement
  - dead_code scored 1/5 - needs improvement
  - MCP: 2 tools (2 public, 0 auth-required) — readiness: full

Shipcheck Summary
=================
  LEG               RESULT  EXIT      ELAPSED
  verify            PASS    0         2.206s
  validate-narrative  PASS    0         135ms
  dogfood           PASS    0         1.269s
  workflow-verify   PASS    0         12ms
  apify-audit       PASS    0         23ms
  verify-skill      PASS    0         1.481s
  scorecard         PASS    0         3.97s

Verdict: PASS (7/7 legs passed)
