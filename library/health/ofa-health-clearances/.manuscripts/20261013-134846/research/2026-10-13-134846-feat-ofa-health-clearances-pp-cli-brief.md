# OFA CLI Brief

> Renamed 2026-10-14: this CLI was first printed as `ofa` and renamed to `ofa-health-clearances` (slug, binary, module) so the name says what it returns. Commands and behavior are unchanged.

Rebuild of the 2026-10-10 sandbox print (lost: local branch `feat/ofa` @ 742b2d4, never pushed).
Sources: `~/dev-bank/QA/2026-10-10-ofa-*.md` (David-confirmed spec + findings) and live recon 2026-10-13
(`scratchpad/ofa-recon/`: js.php?id=as client, server-rendered form, one search response, one dog-detail page).

## API Identity
- Domain: Orthopedic Foundation for Animals (ofa.org) — US canine/feline health-clearance registry
  (hips, elbows, eyes/CAER, cardiac, patella, thyroid, DNA panels), plus CHIC certification status.
- Users:
  - Health-first puppy buyers vetting a breed and its breeders before committing (David's 2026-10-10 Kelpie/Koolie/Aussie search is the motivating case).
  - Breeders checking a prospective stud/dam's clearances and its sire/dam before a breeding.
  - Breed-club health committees tracking how many dogs in their breed are being tested, by year.
- Data profile: read-only public records. No owner, breeder, or location fields. Registered names begin with the kennel prefix, which is the only breeder signal.

## Reachability Risk
- Low. No auth. Backend `https://api.ofa.org/api/as.php` returns HTML fragments. Requires browser-like `Origin: https://ofa.org` and `Referer: https://ofa.org/advanced-search/` headers (GET without them returns an empty 200 body). Verified 2026-10-13: search POST 200 (25 matches Koolie/2026), detail GET 200.
- Terms: OFA states results are copyrighted and public reproduction requires permission. CLI is for personal research; README must say "don't republish bulk data". Default rate limit 1 req/s.

## Wire contract (from recon)
- Search: `POST /api/as.php`, `application/x-www-form-urlencoded` (multipart also accepted). Required: `api_action=as_action`. Paging: `api_page=N` (20 rows/page; header text `N matches.  Page X of <span id='api_page_count'>Y</span>`).
- Filters (`as_filter[...]`): `brdvar[]` (breed code, array, e.g. AUK=Australian Koolie, AKP=Australian Kelpie, WK=Working Kelpie), `test[]` (registry code, array, e.g. HD=hips, EL=elbow), `reportyear` (scalar), `testyear` (scalar), `birthyear` (scalar) or `birthyear_from`/`birthyear_thru`, `special][chic]=Y` (CHIC only), `sex` (M/F), `registrar` (AKC/CKC/UKC), `regname` (prefix; `fullpart=A` for any-part), `regnum`, `ofanum`.
- **Repeated-field gotcha:** scalar fields (`reportyear`, `birthyear`, `testyear`) keep only the LAST value because the backend is PHP (no `[]`). Verified live: reportyear=2025&reportyear=2026 → identical 25 rows as 2026 alone. CLI must run one query per value and merge.
- Result rows: `<tr class='as_results_row' data-appnum='N' data-chic='cur|no'>` then cells: name, registration, breed, sex, color, birthdate, test date, age (months), OFA number, test. Summary layout has no conclusion column; hip OFA numbers encode the grade (E/G/F, e.g. `AUK-50E24M-C-NOPI` = Excellent).
- Detail: `GET /api/as.php?a=/advanced-search/&appnum=N` → per-registry rows: registry, test date, report date, age, conclusion, OFA number; plus vertical-pedigree block (sire/dam).
- Embedded lookups: ~480 breed codes, ~242 test registries (scraped from the form; ship as embedded tables).

## Top Workflows
1. "Which kennels in breed X are actively health-testing?" — filter breed + dogs aged 0–5 + reported last N years, group by kennel prefix (the records-first breeder search, QA 2026-10-10).
2. "Show me this dog's full clearances and its parents'" — dog detail by appnum incl. sire/dam.
3. "How deep is the health-testing record for breed X?" — counts per report year (the Koolie 12/17/22 correction).
4. "CHIC-certified dogs in breed X born in the last N years" — shortlist.
5. Look up breed / test codes.

## Table Stakes
- Search with every filter the web form offers; paging; JSON output.
- Correct multi-year handling (the gotcha). Nobody else does this; the website forces one year at a time too.

## Data Layer
- Primary entities: result rows (dog × test), dogs (appnum).
- Sync cursor: none useful (no updated-since); no local store needed beyond optional caching. Cache disabled.
- FTS/search: not needed.

## Product Thesis
- Name: ofa-health-clearances-pp-cli
- Why: OFA has no API, no CLI, no MCP. The website is one-year-at-a-time, 20 rows/page, with no kennel rollup. A buyer or breeder answering "who is actually testing" today does it by hand across dozens of pages.

## Build Priorities
1. `results` — structured search, multi-value year fan-out, paging, CHIC/age/sex filters.
2. `kennels` — kennel-prefix rollup (dogs, CHIC dogs, tests, latest test, non-normal flags).
3. `dog <appnum>` — full record + sire/dam.
4. `breeds`, `tests` — embedded lookup tables.
