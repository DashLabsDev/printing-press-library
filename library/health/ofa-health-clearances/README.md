# OFA Health Clearances CLI

**Look up dogs' OFA health clearances (hips, elbows, eyes, cardiac, DNA) and CHIC status by breed, age, and report year, and roll them up by kennel.**

Query OFA's hip, elbow, eye, cardiac, and DNA records from the terminal with every Advanced Search filter. Multi-year searches work correctly (the website keeps only one year per query), every page is fetched, and 'kennels' turns raw records into a ranked list of breeders who actually test.

## Install

The recommended path installs both the `ofa-health-clearances-pp-cli` binary and the `pp-ofa-health-clearances` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install ofa-health-clearances
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install ofa-health-clearances --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install ofa-health-clearances --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install ofa-health-clearances --agent claude-code
npx -y @mvanhorn/printing-press-library install ofa-health-clearances --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.9 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/health/ofa-health-clearances/cmd/ofa-health-clearances-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ofa-health-clearances-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install ofa-health-clearances --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-ofa-health-clearances --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-ofa-health-clearances --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install ofa-health-clearances --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ofa-health-clearances-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/health/ofa-health-clearances/cmd/ofa-health-clearances-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "ofa-health-clearances": {
      "command": "ofa-health-clearances-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Check that the OFA search backend is reachable.
ofa-health-clearances-pp-cli doctor

# Find the breed code OFA uses (AUK for Australian Koolie).
ofa-health-clearances-pp-cli breeds koolie

# List this year's Koolie test records.
ofa-health-clearances-pp-cli results --breed AUK --report-year 2026

# Rank kennels testing young Koolies over three report years.
ofa-health-clearances-pp-cli kennels --breed AUK --born-from 2021 --report-year 2024,2025,2026

# Open one dog's full clearance record, including sire and dam.
ofa-health-clearances-pp-cli dog 2682765

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Breeder vetting
- **`kennels`** — See which kennels in a breed are actively health-testing young dogs, ranked by tested dogs, CHIC dogs, and latest test date.

  _Reach for this when an agent needs to find breeders who test, since OFA exposes no breeder field and no kennel view._

  ```bash
  ofa-health-clearances-pp-cli kennels --breed AUK --born-from 2021 --report-year 2024,2025,2026 --agent
  ```

## Recipes

### Records-first breeder search

```bash
ofa-health-clearances-pp-cli kennels --breed AUK --born-from 2021 --report-year 2022,2023,2024,2025,2026
```

Kennel prefixes of young Koolies tested in the last five report years.

### CHIC dogs only

```bash
ofa-health-clearances-pp-cli results --breed AS --chic --report-year 2026 --agent --select name,ofa_number,test
```

Narrow a large breed to CHIC-certified records and keep only the identifying fields.

### Hip records for a kennel prefix

```bash
ofa-health-clearances-pp-cli results --breed AUK --name AVALON --test HD
```

Every hip record whose registered name starts with the kennel prefix.

### One dog and its parents

```bash
ofa-health-clearances-pp-cli dog 2682765 --json
```

Full registry conclusions plus sire and dam as structured JSON.

## Usage

Run `ofa-health-clearances-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `OFA_HEALTH_CLEARANCES_CONFIG_DIR`, `OFA_HEALTH_CLEARANCES_DATA_DIR`, `OFA_HEALTH_CLEARANCES_STATE_DIR`, or `OFA_HEALTH_CLEARANCES_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `OFA_HEALTH_CLEARANCES_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export OFA_HEALTH_CLEARANCES_HOME=/srv/ofa-health-clearances
ofa-health-clearances-pp-cli doctor
```

Under `OFA_HEALTH_CLEARANCES_HOME=/srv/ofa-health-clearances`, the four dirs resolve to `/srv/ofa-health-clearances/config`, `/srv/ofa-health-clearances/data`, `/srv/ofa-health-clearances/state`, and `/srv/ofa-health-clearances/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "ofa-health-clearances": {
      "command": "ofa-health-clearances-pp-mcp",
      "env": {
        "OFA_HEALTH_CLEARANCES_HOME": "/srv/ofa-health-clearances"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `OFA_HEALTH_CLEARANCES_DATA_DIR` overrides an explicit `--home` for that kind. Use `OFA_HEALTH_CLEARANCES_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `OFA_HEALTH_CLEARANCES_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `ofa-health-clearances-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### backend

Raw OFA Advanced Search backend calls (HTML). Prefer 'results', 'dog', and 'kennels'.

- **`ofa-health-clearances-pp-cli backend detail`** - Fetch one dog's raw detail page (HTML) by OFA application number
- **`ofa-health-clearances-pp-cli backend search`** - Run one raw Advanced Search query and return the result table rows (20 rows per page)


### kennels


- **`ofa-health-clearances-pp-cli kennels`** - See which kennels in a breed are actively health-testing young dogs, ranked by tested dogs, CHIC dogs, and latest test date.




### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`ofa-health-clearances-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`ofa-health-clearances-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`ofa-health-clearances-pp-cli learnings list`** - Inspect taught rows
- **`ofa-health-clearances-pp-cli learnings forget <query>`** - Undo a teach
- **`ofa-health-clearances-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`ofa-health-clearances-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`ofa-health-clearances-pp-cli teach-pattern`** - Install a query/resource template up front
- **`ofa-health-clearances-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `OFA_HEALTH_CLEARANCES_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `ofa-health-clearances-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
ofa-health-clearances-pp-cli backend search

# JSON for scripting and agents
ofa-health-clearances-pp-cli backend search --json
# Filter to specific fields by name
ofa-health-clearances-pp-cli backend search --json --select <field>[,<field>...]

# Dry run — show the request without sending
ofa-health-clearances-pp-cli backend search --dry-run

# Agent mode — JSON + compact + no prompts in one flag
ofa-health-clearances-pp-cli backend search --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited or blocked, `10` config error.

## Health Check

```bash
ofa-health-clearances-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `ofa-health-clearances-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is ``; `--home`, `OFA_HEALTH_CLEARANCES_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Multi-year search returns only one year's rows on the website** — Pass years to --report-year as a comma list; the CLI runs one query per year and merges them.
- **Empty results for a breed name** — Use the breed code from 'ofa-health-clearances-pp-cli breeds <name>', not the display name.
- **Requests slow or throttled** — The CLI rate-limits to 1 request/second by default; narrow --report-year or --born-from to reduce pages.
