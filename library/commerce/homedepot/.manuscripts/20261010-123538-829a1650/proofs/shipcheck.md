# Shipcheck

Verdict: ship. All umbrella legs passed: verify, validate-narrative, dogfood, workflow-verify, apify-audit, verify-skill, scorecard.

Go tests and vet pass. Scoped govulncheck: no reachable vulnerabilities after updating Go to 1.26.9 and x/net to 0.60.0. Live search, session verification, cart reads and authorized restored mutation lifecycle passed; online and in-store purchase details passed.

Tool provenance: v4.33.4 with two isolated checker corrections: config-field regex excludes code fragments between quotes; bare-host scanning excludes filenames and non-DNS dotted identifiers. Focused regression checks pass. Full machine suite was run but fails in unrelated existing generated MCP store fixtures, environment-dependent Apify auth fixture and macOS /var versus /private/var path expectations; this is not a claim that the printer full suite is green. Stock checker false positives remain an upstream follow-up. No public PR was created.

First umbrella failed on static command-source ambiguity, old Go install prose, and unrelated printer fixture actor references accidentally under the research tree. Factories now use canonical command files; installer prose and toolchain are updated; tooling fixtures are outside research. Final umbrella passed.

## Final review and live fixes

All three code reviewers converged at round 3; focused regressions and full Go tests passed. A subsequent final cart retest returned a 403 with stale browser clearance; private Chrome re-import succeeded, then the same final binary passed add/update/remove and restored the cart exactly. Chrome extraction discards service-page cookie scopes that cannot ride any CLI request and rejects ambiguous relevant cookie names before expiry filtering.

The isolated local v4.33.4 tool additionally includes read-only auth status in live dogfood while retaining exclusions for login/logout/import. Its focused safety regression passed. This is a local checker patch, not an official release or a public PR.


---

README/SKILL/AGENTS audited against source and recursive help. Semantic errors corrected; concrete shopping examples replace shell placeholders. Learning templates are explicitly labeled non-executable. verify-skill: 39 recipes, zero findings. Public install instructions disclose future publication. Checkout/payment excluded.


---

PASS: one eligible public products-search sample reviewed for relevance, URLs, format and ordering. No findings. Private cart/order/auth samples deliberately withheld; plausibility not assessed for those samples. Live rule checks passed search/orders/auth; cart writes excluded from scorecard and separately proven.


---

# Local code review

Autofix summary: 13 distinct findings and follow-up guards fixed in place across three rounds; see .printing-press-patches/shopper-session-orders.json and focused regression tests. Working tree is not a Git repository, so no commit hashes exist.

Convergence outcome: all correctness, security and maintainability reviewers PASS at round 3.
Review path: direct subagent dispatch (correctness, security, maintainability). All reviewer testing was offline.

## Template retro candidates
- internal/client/cookiejar.go:87,308 (medium; cookiejar.go.tmpl): generated persistence uses direct write-to-clear/non-atomic writes and follows links. Custom shopping clients use verified private exports with in-memory jars; custom logout removes inodes. Reserved generated behavior was not patched.
- internal/client/client.go:1304 (medium; client.go.tmpl): native retry diagnostics can include account identifiers in request URLs. Custom final errors are sanitized; template diagnostic correction belongs upstream.
- Generated tail resource resolver has no emitted paths for this POST-only spec (verify static critical label, resource-path:tail). No approved shopper feature depends on it. README/SKILL disclose absence; template-shaped resolver was not patched.

No pending user tradeoffs or in-scope findings.


---

# Polish Results for homedepot-pp-cli

Mid-pipeline forked polish completed against the current working CLI with Printing Press 4.33.4 plus the parent's disclosed local checker fixes. Public publication and promotion were not performed.

## Preflight and divergence

The upstream-remote-validated public clone at `Scratch/printing-press-library` contains no Home Depot CLI. This first-print working directory is canonical. The parent released the build lock; lock status reported not held. An attempted required heartbeat correctly reported no existing lock and did not create one. Disk space was sufficient. The system Go 1.26.2 is older than the printer; all Go quality checks used cached Go 1.26.9.

## Diagnostic delta

| Check | Before | After | Delta |
| --- | --- | --- | --- |
| Scorecard | 78/100 (B) | 78/100 (B) | 0 |
| Static verify | 37/38, 97.37% | 37/38, 97.37% | 0 |
| Full live dogfood | 110/110 passed | 110/110 passed | 0 |
| Explicit skips / unverified checks | 103 | 103 | 0 |
| Live scorecard samples | 3 passed, 0 failed | 3 passed, 0 failed | Exercised |
| Handwritten security scan sites requiring disposition | 3 | 0 | -3 |
| Go vet | 0 issues | 0 issues | 0 |
| Tools audit | No findings | No findings | 0 |
| PII audit including research manuscripts | No findings | No findings | 0 |
| Workflow verification | PASS | PASS | 0 |
| SKILL verification | PASS | PASS | 0 |
| Publish validation | Skipped: mid-pipeline | Skipped: mid-pipeline | Parent-owned |

The before full live result was already established by the parent. Polish reran the complete live matrix after its final source edits and wrote a fresh source-bound acceptance marker. The 103 explicit skips are not additional successful live requests: they include protected mutation/no-op paths, required-input endpoint fixtures, and intentionally unsupported probes. The scoped cart lifecycle and restoration were proved separately by the parent; polish made no cart writes.

## Fixes applied

- Expanded six shopper command descriptions with meaningful inputs, scope and confirmation behavior so MCP discovery clearly distinguishes preview from applying cart changes.
- Added a shopping/account command table and ten practical cookbook entries to README; checked every newly documented command's actual help. Removed the ambiguous claim that custom cart writes automatically gain retries from `--idempotent`.
- Added three local, documented gosec suppressions for confirmed safe constructs: a validation-only cookie, an outbound HTTPS cookie jar retaining imported HttpOnly metadata, and fixed `sqlite3` execution with a static read-only query and a path argument without a shell. Browser metadata and runtime behavior were preserved.
- Confirmed every modified source/document path is already covered by `.printing-press-patches/shopper-session-orders.json`; no new Go files were introduced.

All modified Go files are formatted. `go test ./...` and `go vet ./...` pass. Both CLI and MCP command packages are included in the tests.

## Agentic output review

An independent required reviewer assessed the three eligible passing samples: product search, orders list and auth status. PASS, no findings. Search results matched the pencil query and used canonical product URLs; purchases reported count/completeness; authentication distinguished stored presence from live verification. No aggregation or ranking claims required further assessment. Additional live samples after the fixes passed 3/3; these edits changed descriptions and comments, not output behavior.

## Skipped findings and known limits

- Static verify marks the generated `tail` resolver critical because this private website spec has no observed polling resource paths. This is a generator/framework limit, with a user-facing workaround already disclosed under README `Known Gaps`: use `cart show` and `orders list`. No custom shopper command failed static verification.
- Static dogfood reports four unused generator helper functions: `handleBinaryResponseDelivery`, `isDryRunResponseForClient`, `retainCLIQueryParams`, and `retainExplicitQueryParams`. The printed API has no corresponding binary/query endpoint surface. Retained as generator retro candidates rather than changing reserved reusable framework code.
- The new bare `auth import` cookbook sample triggered a static verifier collision with root `import <resource>`. Runtime auth import correctly takes no positional arguments. The new cookbook entry was changed to `doctor --json`; Safari import remains accurately documented in the account table and authentication narrative. Final verify-skill is clean. This collision is a checker retro candidate.
- MCP description/tool-design/surface-strategy dimensions are unscored for this endpoint-hidden, runtime Cobra-mirrored shopping surface; the generated typed tools manifest intentionally has no tools. Runtime mirror parity passes. No scaffolding was added to improve these metrics.
- The low static auth-protocol score reflects the custom browser-cookie flow rather than a bot/bearer/basic scheme. Safari/Chrome session metadata validation and protected live verification are implemented and proved separately.
- Shopping data uses uncached live requests. Product search is limited to the first 24 results and returns availability as `unknown`; order paging is bounded. Checkout and payment remain outside the approved scope.
- Raw gosec emits 71 remaining findings in generator/reserved code after custom triage. These are scan candidates, not 71 confirmed vulnerabilities. They were not patched in printed output under the skill's generator ownership rule. A complete location/rule inventory follows for later Printing Press retro work.

## Generator / reserved static-analysis candidates

| Rule | Location | Severity | Scanner report | Ownership evidence |
| --- | --- | --- | --- | --- |
| G402 | internal/client/chrome.go:193 | HIGH | TLS InsecureSkipVerify may be set to true. | generator header |
| G101 | internal/platform/gate.go:22 | HIGH | Potential hardcoded credentials | generator header |
| G101 | cmd/homedepot-pp-mcp/main.go:36 | HIGH | Potential hardcoded credentials | generator header |
| G703 | internal/cli/teach.go:210 | HIGH | Path traversal via taint analysis | generator header |
| G124 | internal/client/cookiejar.go:335 | MEDIUM | http.Cookie missing or has insecure Secure, HttpOnly, or SameSite attribute | generator header |
| G124 | internal/client/cookiejar.go:144 | MEDIUM | http.Cookie missing or has insecure Secure, HttpOnly, or SameSite attribute | generator header |
| G124 | internal/cli/auth.go:1088 | MEDIUM | http.Cookie missing or has insecure Secure, HttpOnly, or SameSite attribute | generator header |
| G124 | internal/cli/auth.go:1046 | MEDIUM | http.Cookie missing or has insecure Secure, HttpOnly, or SameSite attribute | generator header |
| G204 | internal/cli/auth.go:1376 | MEDIUM | Subprocess launched with variable | generator header |
| G204 | internal/cli/auth.go:1374 | MEDIUM | Subprocess launched with variable | generator header |
| G204 | internal/cli/auth.go:1372 | MEDIUM | Subprocess launched with variable | generator header |
| G204 | internal/cli/auth.go:1370 | MEDIUM | Subprocess launched with variable | generator header |
| G204 | internal/cli/auth.go:1368 | MEDIUM | Subprocess launched with variable | generator header |
| G204 | internal/cli/auth.go:1366 | MEDIUM | Subprocess launched with variable | generator header |
| G204 | internal/cli/auth.go:1364 | MEDIUM | Subprocess launched with variable | generator header |
| G204 | internal/cli/auth.go:1362 | MEDIUM | Subprocess launched with variable | generator header |
| G204 | internal/cli/auth.go:1360 | MEDIUM | Subprocess launched with variable | generator header |
| G204 | internal/cli/auth.go:1358 | MEDIUM | Subprocess launched with variable | generator header |
| G204 | internal/cli/auth.go:1356 | MEDIUM | Subprocess launched with variable | generator header |
| G204 | internal/cli/auth.go:1354 | MEDIUM | Subprocess launched with variable | generator header |
| G204 | internal/cli/auth.go:1352 | MEDIUM | Subprocess launched with variable | generator header |
| G204 | internal/cli/auth.go:1350 | MEDIUM | Subprocess launched with variable | generator header |
| G204 | internal/cli/auth.go:1348 | MEDIUM | Subprocess launched with variable | generator header |
| G204 | internal/cli/auth.go:1346 | MEDIUM | Subprocess launched with variable | generator header |
| G204 | internal/cli/auth.go:1201 | MEDIUM | Subprocess launched with variable | generator header |
| G204 | internal/cli/auth.go:1175 | MEDIUM | Subprocess launched with variable | generator header |
| G204 | internal/cli/auth.go:1127 | MEDIUM | Subprocess launched with variable | generator header |
| G204 | internal/cli/auth.go:603 | MEDIUM | Subprocess launched with variable | generator header |
| G204 | internal/cli/auth.go:995 | MEDIUM | Subprocess launched with a potential tainted input or cmd arguments | generator header |
| G201 | internal/store/store.go:3763-3766 | MEDIUM | SQL string formatting | generator header |
| G201 | internal/store/store.go:3460-3463 | MEDIUM | SQL string formatting | generator header |
| G201 | internal/store/store.go:1504 | MEDIUM | SQL string formatting | generator header |
| G201 | internal/store/store.go:1480 | MEDIUM | SQL string formatting | generator header |
| G202 | internal/store/store.go:1031 | MEDIUM | SQL string concatenation | generator header |
| G202 | internal/store/learnings.go:609 | MEDIUM | SQL string concatenation | generator header |
| G202 | internal/platform/migration.go:156 | MEDIUM | SQL string concatenation | generator emit map; platform_migration.go.tmpl |
| G304 | internal/store/store.go:254 | MEDIUM | Potential file inclusion via variable | generator header |
| G304 | internal/platform/ratelimit.go:148 | MEDIUM | Potential file inclusion via variable | generator header |
| G304 | internal/platform/profile.go:510 | MEDIUM | Potential file inclusion via variable | generator header |
| G304 | internal/learn/teach_log.go:112 | MEDIUM | Potential file inclusion via variable | generator header |
| G304 | internal/learn/playbooks.go:73 | MEDIUM | Potential file inclusion via variable | generator header |
| G304 | internal/config/config.go:225 | MEDIUM | Potential file inclusion via variable | generator header |
| G304 | internal/client/cookiejar.go:267 | MEDIUM | Potential file inclusion via variable | generator header |
| G304 | internal/cli/teach_playbook.go:376 | MEDIUM | Potential file inclusion via variable | generator header |
| G304 | internal/cli/teach.go:150 | MEDIUM | Potential file inclusion via variable | generator header |
| G304 | internal/cli/teach.go:127 | MEDIUM | Potential file inclusion via variable | generator header |
| G304 | internal/cli/feedback.go:204 | MEDIUM | Potential file inclusion via variable | generator header |
| G304 | internal/cli/feedback.go:66 | MEDIUM | Potential file inclusion via variable | generator header |
| G304 | internal/cli/auth.go:1013 | MEDIUM | Potential file inclusion via variable | generator header |
| G304 | internal/cli/auth.go:660 | MEDIUM | Potential file inclusion via variable | generator header |
| G304 | internal/cli/auth.go:652 | MEDIUM | Potential file inclusion via variable | generator header |
| G304 | internal/cli/auth.go:567 | MEDIUM | Potential file inclusion via variable | generator header |
| G112 | cmd/homedepot-pp-mcp/main.go:86-89 | MEDIUM | Potential Slowloris Attack because ReadHeaderTimeout is not configured in the http.Server | generator header |
| G117 | internal/learn/journal.go:265 | MEDIUM | Marshaled struct field "SessionKey" (JSON key "session_key") matches secret pattern | generator header |
| G302 | internal/platform/profile.go:302 | MEDIUM | Expect file permissions to be 0600 or less | generator header |
| G302 | internal/cliutil/testenv/testenv.go:76 | MEDIUM | Expect file permissions to be 0600 or less | reserved cliutil_testenv.go.tmpl |
| G302 | internal/cliutil/testenv/sandbox_unix.go:15 | MEDIUM | Expect file permissions to be 0600 or less | generator header |
| G302 | internal/client/client.go:810 | MEDIUM | Expect file permissions to be 0600 or less | generator header |
| G104 | internal/store/store.go:3551 | LOW | Errors unhandled | generator header |
| G104 | internal/store/store.go:3294 | LOW | Errors unhandled | generator header |
| G104 | internal/learn/journal.go:278 | LOW | Errors unhandled | generator header |
| G104 | internal/client/client.go:1486 | LOW | Errors unhandled | generator header |
| G104 | internal/client/chrome.go:248 | LOW | Errors unhandled | generator header |
| G104 | internal/client/chrome.go:202 | LOW | Errors unhandled | generator header |
| G104 | internal/client/chrome.go:195 | LOW | Errors unhandled | generator header |
| G104 | internal/client/chrome.go:190 | LOW | Errors unhandled | generator header |
| G104 | internal/client/chrome.go:155 | LOW | Errors unhandled | generator header |
| G104 | internal/client/chrome.go:151 | LOW | Errors unhandled | generator header |
| G104 | internal/client/chrome.go:141 | LOW | Errors unhandled | generator header |
| G104 | internal/client/chrome.go:124 | LOW | Errors unhandled | generator header |
| G104 | internal/cli/auth.go:767 | LOW | Errors unhandled | generator header |

## Final acceptance

The final full live dogfood command used the parent's patched absolute printer binary, the current spec and research directory, `--level full`, `--timeout 45s`, `--auth-env HOMEDEPOT_COOKIE_FILE`, and `--write-acceptance <run>/proofs/phase5-acceptance.json`. Raw live stdout/stderr and scan reports remain in private files with mode 0600 under the private Home Depot data directory. No cookies, account identifiers, purchase references, cart identifiers, addresses or payment details appear in this response.

Final acceptance: status pass; full; 110 tests passed; 0 failures; 103 explicit skipped/unverified checks. Source fingerprint: `25b7c241a2df587ad5a2a7af4f04b01a721bd88cc0adda3eafe75763f1c92975`.

Recommendation: **ship-with-gaps**, for the already documented generated polling limitation. The approved shopping/authentication features pass their quality and live gates. Further polish would repeat checks against the same generator/private-API limitation; any polling expansion requires new discovery or generator work. Parent owns archive consolidation, final staged PII gate and promotion.

---POLISH-RESULT---
scorecard_before: 78
scorecard_after: 78
verify_before: 97.36842105263158
verify_after: 97.36842105263158
dogfood_before: PASS
dogfood_after: PASS
dogfood_live_matrix_before: exercised
dogfood_live_matrix_after: exercised
govet_before: 0
govet_after: 0
gosec_before: 3
gosec_after: 0
tools_audit_before: 0 pending
tools_audit_after: 0 pending
publish_validate_before: skipped (mid-pipeline)
publish_validate_after: skipped (mid-pipeline)
fixes_applied:
- Clarified six shopper command descriptions with inputs, scope and confirmation behavior.
- Added a shopping/account command table and ten verified cookbook entries; corrected cart retry guidance.
- Documented three narrowly scoped safe gosec false positives at their source sites.
- Reran the full live matrix after final source changes and refreshed source-bound acceptance.
skipped_findings:
- Generated tail resolver lacks observed polling resource paths; README Known Gaps names live cart/orders alternatives.
- Four unused generated framework helpers retained as Printing Press retro candidates.
- Static auth-import positional collision routed to checker retro; final documentation verification passes.
- Seventy-one generated/reserved gosec scan candidates routed to retro under generator ownership rules.
- Unscored MCP mirror dimensions and browser-cookie auth-protocol scoring are structural; no fake scaffolding added.
remaining_issues:
- Generated tail polling is unavailable; use cart show and orders list, as README Known Gaps explains.
ship_recommendation: ship-with-gaps
further_polish_recommended: no
further_polish_reasoning: All approved shopping features pass; the remaining polling limitation requires new API discovery or generator work rather than another polish pass.
---END-POLISH-RESULT---


## Publish verification, October 10, 2026

Publication validation passed all required gates. Full live dogfood reran against the final local source: 110 passed, 0 failed, 103 explicitly skipped/unverified guarded or unsupported cases. Raw response reports were kept private and deleted after extracting counts. The runner refreshed the source-bound Phase 5 acceptance marker in embedded and archived manuscripts. Creator metadata and public attribution were synchronized to Greg Mushen (@gregmushen) using the Printing Press promotion and contributor tools. A local publisher metadata repair preserved known creator handles while completing a verified name-only identity; focused regression checks passed. No checkout or payment is exposed. The generated tail command remains unsupported; use cart show or orders list for fresh reads.

### Subsequent registration correction and live challenge

The public-library documentation verifier required explicit final auth registration and separate cart-child attachment; the final registration passed the repository verifier and focused CLI tests. The subsequent full live run recorded 106 passed, 4 failed product-search checks, and 103 skipped/unverified cases. Search returned a security-challenge HTML page rather than product state. A refreshed Chrome cookie import verified order access, but did not clear the search challenge. That source-bound acceptance marker recorded the failed run; publication was held pending a fresh full live pass. No public PR or commit was created.

### Final publish gate after browser refresh

The browser session refresh cleared the temporary search security page. The CLI returned five actual products, then the final full live matrix passed 110/110 checks, with 0 failures and 103 explicitly skipped/unverified cases. The runner refreshed the acceptance marker for the corrected registration source, and embedded and archived copies were synchronized. Public-library verify-skill passed with no errors or likely false positives; final scoped Go tests and vet passed. No additional cart mutation or order placement occurred.

### Public review corrections

Compact and agent order output now explicitly retains each item description. The synthetic regression failed before the correction and passed afterward; read-only live checks confirmed preserved descriptions for both online and in-store order details (1 and 4 items respectively), without publishing identifiers or response bodies. Browser-cookie reads now use the shared platform credential permission guard: POSIX file modes and Windows owner/DACL validation. Native macOS permission tests pass; the Windows test executable cross-compiles, but native Windows ACL execution was not available on this host. Full scoped Go tests and vet passed. Full live dogfood reran after both source corrections: 110 passed, 0 failed, 103 explicitly skipped/unverified cases. The current runner marker is synchronized in embedded and archived manuscripts.
