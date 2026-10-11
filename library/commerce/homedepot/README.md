# Home Depot CLI

**Search products, manage your cart, and retrieve purchases using your own signed-in session.**

Read products and past purchases through direct HTTP. Cart changes target explicit items and return a verified resulting cart.

Learn more at [Home Depot](https://www.homedepot.com).

Contributors: [@gregmushen](https://github.com/gregmushen) (Greg Mushen).
## Local build and publication status

This CLI is currently a local build. Public installation, release downloads and pre-built bundles below become available only after library publication.

```bash
go build -o ./build/stage/bin/homedepot-pp-cli ./cmd/homedepot-pp-cli
go build -o ./build/stage/bin/homedepot-pp-mcp ./cmd/homedepot-pp-mcp
```

## Install

The recommended path installs both the `homedepot-pp-cli` binary and the `pp-homedepot` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install homedepot
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install homedepot --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install homedepot --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install homedepot --agent claude-code
npx -y @mvanhorn/printing-press-library install homedepot --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.9 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/homedepot/cmd/homedepot-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/homedepot-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine homedepot-pp-cli`. On Unix, mark it executable: `chmod +x homedepot-pp-cli`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install homedepot --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-homedepot --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-homedepot --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install homedepot --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
homedepot-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/homedepot-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/homedepot/cmd/homedepot-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "homedepot": {
      "command": "homedepot-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Sign in to homedepot.com in Safari, then run homedepot-pp-cli auth import. For Chrome, run homedepot-pp-cli auth login --chrome. Session cookies preserve domain, path and expiry in a private local file (0600 on POSIX; owner-only ACL on Windows) at ~/.local/share/homedepot-pp-cli/cookies.json, or the directory selected by --home. An absolute HOMEDEPOT_COOKIE_FILE selects another private file. Imports verify a protected request before replacing the stored session. Run homedepot-pp-cli auth status --verify for a live check; auth logout removes the stored session. No developer API key is needed.

## Quick Start

```bash
# Inspect configuration safely.
homedepot-pp-cli doctor --dry-run

# Find product IDs and prices.
homedepot-pp-cli products search pencils --agent

# Inspect current line IDs and quantities.
homedepot-pp-cli cart show --agent

# Find recent online and in-store purchases.
homedepot-pp-cli orders list --agent

```

## Unique Features

### Signed-in shopping
- **`cart add`** — Reports the resulting cart after the requested change.

  _Reports the resulting cart after the requested change._

  ```bash
  homedepot-pp-cli cart add 100001123 --store 4706 --dry-run --agent
  ```
- **`cart update`** — Updates one explicit cart line and confirms its quantity.

  _Updates one explicit cart line and confirms its quantity._

  ```bash
  homedepot-pp-cli cart update sample-line --quantity 2 --dry-run --agent
  ```
- **`products search`** — Returns concrete products and marks absent availability unknown.

  _Returns concrete products and marks absent availability unknown._

  ```bash
  homedepot-pp-cli products search pencils --agent
  ```
- **`orders list`** — Lists orders across bounded pages in the requested date range.

  _Lists orders across bounded pages in the requested date range._

  ```bash
  homedepot-pp-cli orders list --agent
  ```
- **`auth status`** — Distinguishes stored session presence from a verified signed-in read.

  _Distinguishes stored session presence from a verified signed-in read._

  ```bash
  homedepot-pp-cli auth status --agent
  ```

## Recipes

### Find products

```bash
homedepot-pp-cli products search pencils --agent --select item_id,name,price
```

Return only shopping identifiers, names and prices.

### Inspect cart

```bash
homedepot-pp-cli cart show --agent
```

Get line IDs before changing quantities.

### Find recent purchases

```bash
homedepot-pp-cli orders list --agent
```

Return recent order references.

## Usage

Run `homedepot-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `HOMEDEPOT_CONFIG_DIR`, `HOMEDEPOT_DATA_DIR`, `HOMEDEPOT_STATE_DIR`, or `HOMEDEPOT_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `HOMEDEPOT_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export HOMEDEPOT_HOME=/srv/homedepot
homedepot-pp-cli doctor
```

Under `HOMEDEPOT_HOME=/srv/homedepot`, the four dirs resolve to `/srv/homedepot/config`, `/srv/homedepot/data`, `/srv/homedepot/state`, and `/srv/homedepot/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "homedepot": {
      "command": "homedepot-pp-mcp",
      "env": {
        "HOMEDEPOT_HOME": "/srv/homedepot"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `HOMEDEPOT_DATA_DIR` overrides an explicit `--home` for that kind. Use `HOMEDEPOT_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `HOMEDEPOT_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `homedepot-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### Shopping and account

| Task | Command | Main inputs and result |
| --- | --- | --- |
| Import Safari session | `auth import` | Verify and save the signed-in session; `--file` accepts a cookie export |
| Import Chrome session | `auth login --chrome` | Optional `--profile`; verify and save cookie metadata |
| Verify authentication | `auth status --verify` | Report session presence and protected-read verification |
| Find products | `products search <query>` | `--limit` 1–24; return product IDs, names, URLs and prices |
| Inspect cart | `cart show` | Return current line IDs, quantities, fulfillment and total |
| Add product | `cart add <item-id>` | Quantity and pickup store or delivery ZIP; preview by default |
| Set quantity | `cart update <line-id>` | Required `--quantity`; preview by default |
| Remove line | `cart remove <line-id>` | Exact current line ID; preview by default |
| Find purchases | `orders list` | Date range, result limit and page cap; return detail references |
| Read purchase | `orders get <reference>` | Optional date range; return purchased items and totals |

Cart changes require `--confirm` or `--yes` to apply. None of these commands places an order.

### federation-gateway

Developer transport for observed cart operations; use cart commands for shopping.

- **`homedepot-pp-cli federation-gateway`** - Replay an observed Home Depot GraphQL request.

### oms

Developer transport for observed order requests; use orders commands for purchases.

- **`homedepot-pp-cli oms get-order-details`** - Read one purchase with a complete observed body.
- **`homedepot-pp-cli oms list-order-history`** - Read purchase history with a complete observed body.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`homedepot-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`homedepot-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`homedepot-pp-cli learnings list`** - Inspect taught rows
- **`homedepot-pp-cli learnings forget <query>`** - Undo a teach
- **`homedepot-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`homedepot-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`homedepot-pp-cli teach-pattern`** - Install a query/resource template up front
- **`homedepot-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `HOMEDEPOT_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `homedepot-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
homedepot-pp-cli products search hammer

# JSON for scripting and agents
homedepot-pp-cli products search hammer --json
# Filter to specific fields by name
homedepot-pp-cli products search hammer --json --select item_id,name,price

# Dry run — report the skipped action without network access
homedepot-pp-cli products search hammer --dry-run

# Agent mode — JSON + compact + no prompts in one flag
homedepot-pp-cli products search hammer --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select item_id,name,price` returns only fields you need
- **Previewable** - shopper `--dry-run` reports the skipped action without network access; generated API commands can preview HTTP requests
- **Cart retries** - writes are not retried automatically; inspect `cart show` before retrying an uncertain change
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Live shopping** - product search, cart and purchase-history commands use live HTTP; the local store supports agent learning, not offline shopping
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Cookbook

Use current product, line and purchase references from this account. `sample-line` and `sample-order` below are placeholders.

### 1. Inspect local configuration and homepage reachability

```bash
homedepot-pp-cli doctor --json
```

### 2. Import Chrome's Default profile

```bash
homedepot-pp-cli auth login --chrome --profile Default
```

### 3. Confirm the session with a protected read

```bash
homedepot-pp-cli auth status --verify --json
```

### 4. Return five product matches

```bash
homedepot-pp-cli products search pencils --limit 5 --json --select item_id,name,price
```

### 5. Export product matches as CSV

```bash
homedepot-pp-cli products search pencils --limit 5 --csv
```

### 6. Read cart line IDs before changing anything

```bash
homedepot-pp-cli cart show --json
```

### 7. Check add arguments without contacting Home Depot

```bash
homedepot-pp-cli cart add 300951230 --store 8964 --quantity 1 --dry-run --json
```

To see a live read-only preview, omit `--dry-run`. After approving that specific change, add `--confirm` to apply it.

### 8. Check quantity-update arguments

```bash
homedepot-pp-cli cart update sample-line --quantity 2 --dry-run --json
```

Replace `sample-line` with a current `line_id`; omit `--dry-run` for a read-only preview, then add `--confirm` after approving the change.

### 9. Check removal arguments

```bash
homedepot-pp-cli cart remove sample-line --dry-run --json
```

Use the exact current `line_id` and `--confirm` only after approving its removal.

### 10. Find purchases and inspect a chosen reference

```bash
homedepot-pp-cli orders list --limit 10 --json
homedepot-pp-cli orders get sample-order --dry-run --json
```

Replace `sample-order` with a returned reference and omit `--dry-run` to read its items and totals. Add `--from YYYY-MM-DD` and `--to YYYY-MM-DD` when the purchase falls outside the default history window.

## Health Check

```bash
homedepot-pp-cli doctor
```

Checks local configuration, credential presence and homepage reachability. Run `homedepot-pp-cli auth status --verify` to verify the signed-in session with a protected request.

## Configuration

Run `homedepot-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/homedepot-pp-cli/config.toml`; `--home`, `HOMEDEPOT_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `homedepot-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run `orders list` for order references or `cart show` for cart line IDs

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

TLS certificates are verified by default. For a trusted development or self-signed endpoint only, pass `--insecure` for one invocation, set `HOMEDEPOT_SKIP_TLS_VERIFY=true` for the current environment, or set `skip_tls_verify = true` in the config file for a persistent override.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**homedepot-go**](https://github.com/fnziman/homedepot-go) — Go
- [**mcp-homedepot**](https://github.com/markswendsen-code/mcp-homedepot) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

## Cart confirmation and limits

Cart mutations preview by default. After the user approves a specific change, pass `--confirm` (available to MCP agents) or `--yes` (CLI only). `--dry-run` makes no network request. Use the current `line_id` from `cart show` for updates and removals. If an outcome is uncertain, inspect the cart before retrying; writes are never automatically retried.

```bash
homedepot-pp-cli cart add 300951230 --store 8964 --quantity 1 --dry-run --json
homedepot-pp-cli cart update sample-line --quantity 2 --dry-run --json
homedepot-pp-cli cart remove sample-line --dry-run --json
```

Product search reads at most 24 results from the website's first search page and reports unobserved availability as `unknown`. Purchase history defaults to three months, supports `--from`, `--to`, `--limit` and `--max-scan-pages`, and reports failed or page-capped scans as errors. An intentional output limit succeeds with `complete:false` and `scanned_orders` showing the full scanned count. Home Depot's private website API and clearance cookies can change; re-import a signed-in browser session after expiration or an authentication error. Cart-write headers and the add selection query follow the observed browser contract; the generated TLS transport is unchanged. Checkout and payment are outside this CLI's scope.

## Known Gaps

- The generated `tail` framework has no observed polling resources for this private website API. Use `cart show` and `orders list` for current state.
