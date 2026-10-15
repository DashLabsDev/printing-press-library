# OFA CLI Absorb Manifest

Approval basis: user directive 2026-10-13 ("rebuild the OFA CLI properly") = the 2026-10-10 sandbox's approved
command set (results, kennels, dog incl. sire/dam, breeds, tests). Scope is that set; additional subagent survivors
are listed as PROPOSED and are NOT shipping scope until David approves them.

## Absorb Manifest

### Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Search health records by breed/test/year/CHIC/sex/registrar/name | ofa.org Advanced Search (web form) | ofa-health-clearances-pp-cli results | Multi-year fan-out fixes the PHP last-value gotcha; follows all pages; --json/--csv/--select |
| 2 | Dog detail record by appnum incl. sire/dam | ofa.org dog page | ofa-health-clearances-pp-cli dog | Structured per-registry conclusions + sire/dam, agent-native JSON, typed not-found exit |
| 3 | Breed code lookup | ofa.org form breed list | ofa-health-clearances-pp-cli breeds | Embedded table, offline, substring match |
| 4 | Test registry code lookup | ofa.org form test list | ofa-health-clearances-pp-cli tests | Embedded table, offline, filter by category/method |

### Transcendence (only possible with our approach)
| # | Feature | Command | Score | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|-------|--------------|------------------------|------------------|
| 1 | Kennel rollup | kennels | 10/10 | hand-code | One search per report year (PHP last-value workaround) across every page, grouped by registered-name kennel prefix: dogs, CHIC dogs, tests, latest test date, hip-grade mix, non-normal flags. No single OFA request or page does this. | Use this command to rank kennels across a breed by health-testing activity. Do NOT use it for a single dog's record; use 'dog' instead. |

### Proposed, not approved (subagent survivors — awaiting David)
| Feature | Command | Score | Note |
|---------|---------|-------|------|
| Testing depth grid | depth | 10/10 | year x test match-count grid from page-1 headers |
| Pedigree clearance matrix | pedigree | 9/10 | dog + sire/dam (+grandparents) registry matrix |
| Kennel drill-down | kennel | 8/10 | one breeder's dogs collapsed per dog |
| OFA number verify | verify | 7/10 | decode a quoted OFA number + live existence check |

### Out of scope
- Breed-level disease statistics (Apify actor scrapes separate OFA statistics pages, paid) — different backend; not absorbed.

Stubs: none.
