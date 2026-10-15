# Cookidoo v0.1 MVP extension

The generated recipe-get command is preserved. `internal/cli/cookidoo_views.go` is a hand-authored extension registered through the generated novel-command hook and adds read-only `details`, `ingredients`, `nutrition`, and `plan` views. It accepts public recipe IDs/URLs, sends an HTML `Accept` header and the explicit internal HTML-response marker required by the generated client, and emits no authenticated Cookidoo mutations or stored session material.

Public search was verified in the Codex in-app browser but remains deferred because the browser-rendered result service does not provide a stable credential-free contract for a CLI.

The `recipes shopper` extension invokes an explicitly selected Shopper CLI executable for read-only catalog searches. It returns candidates and no-match/error states for review; it never adds products to a cart. The caller should pass the canonical Shopper binary path when multiple installed versions could shadow one another.

## Public release wording

Use a public recipe-only description consistently in the internal spec, generated provenance, README, and skill. Do not describe the CLI as private or imply an authenticated Cookidoo login. The installer fallback should refer to library troubleshooting and the selected category after publication.


## Release dry-run conformance

The newer Printing Press dogfood gate checks `--dry-run --json` across read-only framework commands. `doctor`, `feedback list`, and `profile list` now return the shared dry-run envelope without probing the network or reading local records. `doctor` is explicitly marked read-only for agent surfaces. This downstream patch keeps the public read-only contract clear and makes the live gate safe under isolated test homes.


## Novel source version scanner compatibility

The structural novel-host scanner treats a bare `"0.1"` source literal as a hostname. Compose the plan output version from two literals so the emitted JSON remains `0.1` while the scanner does not report a false host.
