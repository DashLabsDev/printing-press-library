Manifest transcendence rows: 5 planned, 5 built; no approved rows missing.

## Build completion
5 planned, 5 built: cart add, cart update, products search, orders list, auth status.

All approved shopper command paths are implemented: browser authentication, product search, cart show/add/update/remove, and order list/get. The static browser GraphQL add contract is retained for gateway compatibility; the CLI discards its broad response and emits only a fresh minimal cart read. No response caching is used for cart/order operations.

Authentication uses the generated private cookie jar format and HTTP transport. Safari and Chrome imports preserve cookie domains, paths, expiry, Secure and HttpOnly metadata. Candidate sessions pass an isolated protected account read before replacing stored credentials. Status distinguishes presence from live verification. Cart writes preview by default and require --yes or --confirm, do not retry ambiguous writes, and confirm the resulting cart.

Provider discoveries: empty history pages omit orders; orderCount is page-specific and can differ from grouped rows; online order references may repeat in a single page. History therefore checks account identity and page fingerprints, scans through a protected empty page, and fails on repeated pages or scan caps.

## Live cart lifecycle
Owner authorized test and restoration. Public test product 300951230 (33-cent pencil), pickup store 8964. CLI add quantity 1, update to 2, and remove passed. Final cart compared exactly equal with private baseline: original two line IDs, products, quantities, fulfillment, locations, store and total preserved. A browser capture helped resolve missing localization/store and delete itemReq contracts. No order placed. Baseline and cookies remain outside manuscripts.

## Checks
Go build, go test ./..., and go vet ./... passed. Product search returned priced products. Auth import/status verified. Recent order listing returned 9 provider summary rows; online and in-store details both returned items and totals.

## Tooling findings
Stock Printing Press dogfood misclassifies file names and dotted JavaScript identifiers as hosts, and its config regex reads Go source fragments between quotes as field names. Command wrappers also confused static path discovery. Runtime command factories were aligned with static discovery. Checker corrections are tracked separately from the printed CLI, with regression checks; they will not alter the shopper source provenance.

## Scope
Checkout, payment, delivery scheduling, inventory sweeps, monitoring and local purchase-history storage are outside this print.

## Final replay status
After the earlier complete restored lifecycle, the final build received HTTP 403 on a new add attempt. The cart remained identical to the original baseline. Browser-session refresh and replay investigation are in progress; the earlier lifecycle is not final-build release proof.

## Final replay resolution
Browser-observed tracing headers were missing. Cart writes now create fresh trace and parent IDs. With the final build, add succeeded; update and remove confirmed state and original cart was restored. Cookie import also now uses the allowlisted extractor executable name. Cart changes preview by default; per-command --confirm is mirrored into MCP.

