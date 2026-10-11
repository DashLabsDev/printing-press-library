# Final-build authorized cart lifecycle

Owner approved a temporary disposable pencil in the signed-in test cart. TEST_LINE was bound to the unique new line returned by cart add; its value is omitted as private state. Command arguments below preserve that binding rather than publishing account-specific IDs.

- `homedepot-pp-cli cart show --json` — exit 0
- `homedepot-pp-cli cart add 300951230 --store 8964 --quantity 1 --confirm --json` — exit 0
- `homedepot-pp-cli cart update "$TEST_LINE" --quantity 2 --confirm --json` — exit 0
- `homedepot-pp-cli cart show --json` — exit 0
- `homedepot-pp-cli cart remove "$TEST_LINE" --confirm --json` — exit 0
- `homedepot-pp-cli cart show --json` — exit 0

Add quantity 1 and update quantity 2 were confirmed. Cleanup removed the unique test line. Final cart show exactly matched the private baseline (all original lines, quantities, fulfillment, locations, cart identity, store and total). Original two lines and $9.45 restored: True. No order placed.


---

# Home Depot acceptance

- Level: full dogfood.
- Gate: PASS. 110 mandatory checks passed, zero failed.
- 103 additional probes were explicitly skipped/unverified by the safety and fixture rules. These are not counted as passed.
- The runner-owned marker binds the passing result to the current source.
- Real signed-in product search, cart reads and protected authentication checks passed.
- Authorized cart add/update/remove passed with an inexpensive disposable item. The original two lines and total were restored exactly; no order was placed. See cart-lifecycle-proof.md.
- Separate live online and in-store order detail reads passed after history discovery (one and four items respectively). Account-specific references and response bodies remain in private local files.
- CLI fixes: relevant cookie-path scoping for Chrome import, realistic runnable examples and safe happy-path annotations.
- Printing Press issues corrected in the isolated v4.33.4 checker: configuration-field parsing, bare host detection, inclusion of explicitly read-only auth status, and boolean happy-argument overlays. Regression checks passed. These local tooling fixes have not been contributed upstream.
- Full Printing Press source tests had unrelated fixture failures; the CLI's own Go tests and vet passed. No claim of a clean full Printing Press test suite is made.

