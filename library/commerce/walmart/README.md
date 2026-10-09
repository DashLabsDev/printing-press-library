# Walmart CLI

**Walmart Pickup & Delivery from the terminal — reads via orchestra, cart changes via affiliate link only.**

Purchase history, grocery search, product and store lookups, cart view and slot listing, plus a cart link builder that prints one Walmart add-to-cart URL for you to open. The CLI never mutates the cart or checks out.

Learn more at [Walmart](https://www.walmart.com).

Created by [@DashLabsDev](https://github.com/DashLabsDev) (Thomas McCormick).

## Install

The recommended path installs both the `walmart-pp-cli` binary and the `pp-walmart` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install walmart
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install walmart --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install walmart --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install walmart --agent claude-code
npx -y @mvanhorn/printing-press-library install walmart --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/walmart/cmd/walmart-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/walmart-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install walmart --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-walmart --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-walmart --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install walmart --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
walmart-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/walmart-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/walmart/cmd/walmart-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "walmart": {
      "command": "walmart-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Import a signed-in walmart.com browser session with auth login --chrome (or --cookies-file with Playwright storage-state JSON, a browser-extension cookie export, a Netscape cookies.txt file or a raw Cookie header), then run doctor. auth status shows whether the session is active and when the short-lived auth cookie expires.

## Quick Start

```bash
# Verify cookie session
walmart-pp-cli doctor

# Find the search command
walmart-pp-cli which "search products"

# Print a multi-item add-to-cart URL (ADDS; open it yourself)
walmart-pp-cli cart link 51259338:1 44391152:1 14584700257:1

# Show CLI version
walmart-pp-cli version

```

## Unique Features

These capabilities aren't available in any other tool for this API.
- **`orders list`** — PurchaseHistoryV3 persisted query with --type in-store/online, --status completed/in-progress, --search and cursor paging; read-only.
- **`orders get`** — getOrder line items (name, item id, quantity, line price) plus subtotal, tax, fees and total; customer, address and payment fields are never emitted.
- **`search`** — Parses the server-rendered search page for price, unit price, aisle, availability and pickup/delivery fulfillment at the session's store.
- **`orders list`** — Client refuses non-GET requests and denylisted cart/checkout/slot/order mutations, and stops (no retry) on PerimeterX 412/418/429/456 or /blocked.
- **`product get`** — ItemById persisted query: price, unit price, stock, pickup/delivery/shipping status and seller; shopper location fields are never emitted.
- **`store list`** — nearByNodes query for a ZIP: store id, distance, address, pickup types and hours; view only, never sets a store.
- **`cart view`** — getCart GET with a supplied cart id: items, mode, store and slot-reserved flag; MergeAndGetCart and every cart mutation stay blocked.
- **`search`** — --filter type:value (e.g. brand:Great Value), --sort and --list-filters from the page's sort/filter facets.

## Recipes

### Build a grocery add-to-cart link

```bash
walmart-pp-cli cart link 51259338:1 44391152:1 14584700257:1
```

### Check session health

```bash
walmart-pp-cli doctor
```

## Usage

Run `walmart-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `WALMART_CONFIG_DIR`, `WALMART_DATA_DIR`, `WALMART_STATE_DIR`, or `WALMART_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `WALMART_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export WALMART_HOME=/srv/walmart
walmart-pp-cli doctor
```

Under `WALMART_HOME=/srv/walmart`, the four dirs resolve to `/srv/walmart/config`, `/srv/walmart/data`, `/srv/walmart/state`, and `/srv/walmart/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "walmart": {
      "command": "walmart-pp-mcp",
      "env": {
        "WALMART_HOME": "/srv/walmart"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `WALMART_DATA_DIR` overrides an explicit `--home` for that kind. Use `WALMART_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `WALMART_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `walmart-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### account

Account landing data used to verify the session, read only

- **`walmart-pp-cli account`** - Account landing page (accountLandingPage persisted query)

### orders

Walmart purchase history (online and in-store), read only

- **`walmart-pp-cli orders detail`** - One order with line items and price details (getOrder persisted query)
- **`walmart-pp-cli orders history`** - Purchase history page (PurchaseHistoryV3 persisted query)
- **`walmart-pp-cli orders items`** - Recently purchased items across orders (ItemHistory persisted query)


### Shopping commands

These hand-built commands parse Walmart responses into compact shapes. Customer names, addresses, phone numbers and payment data are never emitted.

**Link-only cart.** Reads (history, search, product, store, cart view, slots list) call Walmart's orchestra GraphQL over HTTPS with your imported cookies. The CLI never POSTs cart mutations. To add items, `cart link` builds Walmart's public affiliate add-to-cart URL (`https://affil.walmart.com/cart/addToCart?items=id|qty,...`) and prints it; you open it in your own browser. The link *ADDS / increments* quantity on whatever is already in that browser cart. It also works signed out: items go into a guest cart and carry over into the account cart when you sign in. Multiple items are comma-separated `id|qty` pairs (US item ids). There is no cart update/remove, no slots reserve, and no CDP/browser driver. Checkout, place order and Pickup/Delivery mode switching stay blocked.

Read only:

- **`walmart-pp-cli orders list`** / **`orders get`** / **`search`** / **`product get`** / **`store list`** — as before
- **`walmart-pp-cli cart view`** — items, quantities, prices, mode, reserved slot
- **`walmart-pp-cli slots list [--available]`** — slots for the cart's current mode only

Cart link (prints a URL; optional `--open` launches your browser; never mutates via API):

- **`walmart-pp-cli cart link <item[:qty]>…`** — build `https://affil.walmart.com/cart/addToCart?items=ID|qty,…`. **ADDS / increments** quantity on whatever is already in the browser cart. Works signed out (guest cart; items carry over on sign-in) or signed in. `cart add` is an alias.
- Extra items can also go in one comma-separated `--items` value (e.g. `--items 44391152:2,10450114`); MCP clients use this field for multi-item links.
- Open the URL yourself, then pick a time and check out on walmart.com. The CLI does not update, remove, reserve, or check out.

```bash
walmart-pp-cli cart link 51259338:2 44391152
walmart-pp-cli cart link 51259338 --open
```

### Safety

The client refuses every non-GET orchestra request and every denylisted operation (including updateItems, reserveSlotMutation, checkout, place order, cart merge, mode switching, payment, order changes). Cart changes are link-only: the CLI only prints (or optionally opens) an affiliate URL; it never mutates the cart. If Walmart answers with a bot challenge (HTTP 412/418/429/456, `/blocked`, or a dropped connection) the CLI stops without retrying.

If Walmart rotates a persisted-query hash, override it with `WALMART_HASH_<OPERATION>` (for example `WALMART_HASH_PURCHASEHISTORYV3`). After a Walmart web release, set `WALMART_PLATFORM_VERSION` to the new `x-o-platform-version` value. Requests carry the same user agent and client hints as desktop Chrome on Linux; if you import cookies from a different browser, set `WALMART_USER_AGENT` to that browser's user agent. Each command makes one attempt per request and never retries against Walmart.

### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`walmart-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`walmart-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`walmart-pp-cli learnings list`** - Inspect taught rows
- **`walmart-pp-cli learnings forget <query>`** - Undo a teach
- **`walmart-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`walmart-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`walmart-pp-cli teach-pattern`** - Install a query/resource template up front
- **`walmart-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `WALMART_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `walmart-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
walmart-pp-cli account

# JSON for scripting and agents
walmart-pp-cli account --json
# Filter to specific fields by name
walmart-pp-cli account --json --select <field>[,<field>...]

# Dry run — show the request without sending
walmart-pp-cli account --dry-run

# Agent mode — JSON + compact + no prompts in one flag
walmart-pp-cli account --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Link-only cart** - `cart link` prints (or `--open`s) an affiliate add-to-cart URL; the CLI never mutates the cart; checkout is never sent
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
walmart-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `walmart-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/walmart-pp-cli/config.toml`; `--home`, `WALMART_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `walmart-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **HTTP 412 / bot challenge / connection closed (EOF)** — Stop. Open walmart.com in a browser, clear any challenge, then re-run walmart-pp-cli auth login --chrome. The CLI does not retry challenges.
- **cart link URL opens but items do not appear** — Refresh /cart after the redirect finishes. The link ADDS to the existing cart; signed-out browsers get a guest cart that carries over when you sign in.

## HTTP Transport

This CLI uses standard HTTP transport with HTTP/2 disabled for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
