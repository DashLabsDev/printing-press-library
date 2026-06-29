# DEVONthink CLI

**Local-first DEVONthink automation with safer shell workflows than raw AppleScript or MCP alone.**

This CLI treats DEVONthink as a local knowledge database, not a cloud API. It wraps core record/search operations, adds a SQLite mirror for repeatable analysis, and provides stable inventory, graph, batch, and ledger contracts for higher-level maintenance plugins.

## Install

The recommended path installs both the `devonthink-pp-cli` binary and the `pp-devonthink` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install devonthink
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install devonthink --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install devonthink --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install devonthink --agent claude-code
npx -y @mvanhorn/printing-press-library install devonthink --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/devonthink/cmd/devonthink-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/devonthink-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install devonthink --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-devonthink --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-devonthink --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install devonthink --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/devonthink-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/devonthink/cmd/devonthink-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "devonthink": {
      "command": "devonthink-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Default operation uses local macOS automation and requires no API key. Optional official MCP passthrough uses DEVONthink's local MCP server when you enable it in DEVONthink; keep it bound to localhost or your own LAN and set any bearer token through DEVONthink's MCP settings.

## Quick Start

```bash
# Check local runtime readiness without touching DEVONthink.
devonthink-pp-cli runtime doctor --json

# Preview the stable inventory contract consumed by maintenance workflows.
devonthink-pp-cli inventory export --format maintenance --query "kind:document" --limit 500 --output devonthink-inventory.json

# Find records while keeping agent output compact.
devonthink-pp-cli records search "kind:pdf" --limit 5 --agent --select uuid,name,item_link

# Scope a normal search to a Smart Group by UUID, exact name, or DEVONthink path.
devonthink-pp-cli records search "tags:waiting/rueckerstattung" --smart-group "Offene Rückerstattungen" --agent --select uuid,name,item_link,tags,databaseName

# Capture the current GUI selection as a repeatable workflow seed.
devonthink-pp-cli selection snapshot --agent

# Build a bounded evidence packet for local reasoning.
devonthink-pp-cli context pack --query "project alpha" --token-budget 6000 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`context pack`** — Build a compact evidence packet from records, selections, highlights, links, and related items.

  _Use this when an agent needs enough DEVONthink context to reason without dumping whole documents._

  ```bash
  devonthink-pp-cli context pack --query "project alpha" --token-budget 6000 --agent --select markdown,records
  ```
- **`graph audit`** — Detect orphans, broken links, unresolved wiki links, weak hubs, and tag-only clusters.

  _Use this when DEVONthink should behave like a maintained knowledge graph instead of a folder pile._

  ```bash
  devonthink-pp-cli graph audit --database Research --agent --select issues,type,count,samples
  ```
- **`mirror search`** — Query a local SQLite mirror for repeatable fast analysis without repeated app calls.

  _Use this for repeated analysis, dashboards, and low-token agent workflows._

  ```bash
  devonthink-pp-cli mirror search "tag:tax" --limit 20 --agent --select uuid,name,path
  ```

### Local-first safety
- **`privacy audit`** — Preview what a workflow may expose before content leaves the local machine.

  _Use this before sending DEVONthink-derived context to an external model or shared MCP endpoint._

  ```bash
  devonthink-pp-cli privacy audit --query "invoice" --limit 10 --agent
  ```
- **`agent-context`** — Emit an agent contract that enforces local-machine and own-LAN DEVONthink access only.

  _Use this before handing DEVONthink access to an agent that must avoid remote control paths._

  ```bash
  devonthink-pp-cli agent-context --local-only --agent
  ```

### Safe automation
- **`batch plan`** — Stage multi-record edits as validated dry-run plans before applying them.

  _Use this for multi-record writes where each target must be checked before mutation._

  ```bash
  devonthink-pp-cli batch plan --from selection --add-tag reviewed --move-to /Archive --agent
  ```
- **`ledger list`** — Review CLI-driven mutation plans, applies, target proofs, and rollback hints.

  _Use this to audit or explain what recent automation did to DEVONthink._

  ```bash
  devonthink-pp-cli ledger list --since 7d --agent --select time,action,count,status
  ```
- **`selection snapshot`** — Turn the current GUI selection into a reusable JSON workflow seed.

  _Use this when the human has curated records in the GUI and wants an agent-safe handoff._

  ```bash
  devonthink-pp-cli selection snapshot --note "review these PDFs" --agent
  ```

### Agent-native plumbing
- **`inventory export`** — Export DEVONthink databases, groups, tags, and document metadata for maintenance plugins.

  _Use this when structure-audit or inbox-triage tooling needs a stable local inventory contract._

  ```bash
  devonthink-pp-cli inventory export --format maintenance --query "kind:document" --limit 500 --output devonthink-inventory.json --agent --select databases,documents
  ```
- **`mcp call`** — Call DEVONthink's official local MCP tools from scripts when the local MCP server is enabled.

  _Use this when the official MCP exposes a new tool before the CLI has a promoted command._

  ```bash
  devonthink-pp-cli mcp call search_records --args '{"query":"kind:pdf","limit":5}' --agent
  ```

## Recipes


### Compact search for an agent

```bash
devonthink-pp-cli records search "tags:review AND kind:pdf" --limit 10 --agent --select uuid,name,item_link,tags
```

Returns only the fields an agent needs to pick the next record.

### Search within a Smart Group

```bash
devonthink-pp-cli records search "tags:waiting/rueckerstattung" \
  --smart-group "Offene Rückerstattungen" \
  --agent \
  --select uuid,name,item_link,tags,databaseName
```

Smart Groups are search scopes only. They do not define action workflow policy; downstream maintenance tools should still apply their own review and transition rules.

### Feed the maintenance plugin

```bash
devonthink-pp-cli inventory export --format maintenance --query "kind:document" --limit 500 --output devonthink-inventory.json --agent --select databases.name,documents.name,documents.tags
```

Produces the stable inventory JSON that structure-audit and inbox-triage workflows can consume.

### Create a local evidence packet

```bash
devonthink-pp-cli context pack --query "family invoices" --token-budget 5000 --agent --select markdown,records.item_link
```

Builds a bounded context bundle without dumping entire records.

### Plan a safe tag cleanup

```bash
devonthink-pp-cli batch plan --query "tags:todo" --add-tag reviewed --dry-run --agent
```

Stages a batch change for review before any record is mutated.

### Audit graph health

```bash
devonthink-pp-cli graph audit --database Research --agent --select issues,type,count,samples
```

Finds link and organization gaps from the local mirror.

## Usage

Run `devonthink-pp-cli --help` for the full command reference and flag list.

## Commands

### ai

DEVONthink AI and summary helpers

- **`devonthink-pp-cli ai ask`** - Ask DEVONthink AI about selected local records with explicit cloud-use warnings
- **`devonthink-pp-cli ai summarize`** - Summarize records or highlights

### batch

Dry-run-first multi-record mutation plans

- **`devonthink-pp-cli batch apply`** - Apply a previously reviewed local JSON plan
- **`devonthink-pp-cli batch plan`** - Stage multi-record changes as a local JSON plan

### context

Agent context bundles

- **`devonthink-pp-cli context`** - Build a compact local context pack from records, selection, or search

### databases

Open DEVONthink databases

- **`devonthink-pp-cli databases`** - List open databases

### graph

Links, mentions, and knowledge graph health

- **`devonthink-pp-cli graph audit`** - Detect orphans, unresolved wiki links, weak hubs, and tag-only clusters
- **`devonthink-pp-cli graph links`** - List item links, wiki links, mentions, and unresolved wiki names

### groups

DEVONthink groups and folders

- **`devonthink-pp-cli groups`** - Render a bounded group tree

### ingest

File and URL ingestion

- **`devonthink-pp-cli ingest file`** - Import or index a file or folder
- **`devonthink-pp-cli ingest url`** - Capture a URL as Markdown, HTML, PDF, bookmark, or webarchive

### inventory

Stable inventory export contracts

- **`devonthink-pp-cli inventory`** - Export databases, groups, tags, and selected document metadata for downstream tools

### ledger

Local operation ledger

- **`devonthink-pp-cli ledger list`** - List recent CLI operation ledger entries
- **`devonthink-pp-cli ledger show`** - Show one ledger entry with target proofs and rollback hints

### mcp

Optional local official MCP passthrough

- **`devonthink-pp-cli mcp call`** - Call a local official DEVONthink MCP tool by name
- **`devonthink-pp-cli mcp schema`** - Emit cached MCP tool schemas
- **`devonthink-pp-cli mcp tools`** - List official DEVONthink MCP tools when local MCP HTTP is enabled

### media

OCR and transcription

- **`devonthink-pp-cli media ocr`** - OCR an image or scanned PDF
- **`devonthink-pp-cli media transcribe`** - Transcribe audio, video, image, or PDF content

### mirror

Local SQLite mirror

- **`devonthink-pp-cli mirror search`** - Search the local mirror with FTS
- **`devonthink-pp-cli mirror sync`** - Refresh the local SQLite mirror from open DEVONthink databases

### privacy

Local privacy and exposure reports

- **`devonthink-pp-cli privacy`** - Preview database scope, content-size budget, and cloud/MCP exposure before handoff

### records

DEVONthink records

- **`devonthink-pp-cli records content`** - Extract text content with length and redaction controls
- **`devonthink-pp-cli records create`** - Create a record or group after validating destination
- **`devonthink-pp-cli records get`** - Get record metadata
- **`devonthink-pp-cli records highlights`** - Extract highlights and annotations
- **`devonthink-pp-cli records lookup`** - Look up records by exact name, URL, path, filename, location, or comment
- **`devonthink-pp-cli records move`** - Move, duplicate, replicate, or trash a record with dry-run proof
- **`devonthink-pp-cli records related`** - Find related records using DEVONthink similarity
- **`devonthink-pp-cli records search`** - Search records using DEVONthink query syntax or local mirror fallback
- **`devonthink-pp-cli records update`** - Update record text, properties, tags, comment, URL, aliases, or rating
- **`devonthink-pp-cli records versions`** - List saved record versions

### runtime

Local DEVONthink runtime health

- **`devonthink-pp-cli runtime`** - Check DEVONthink app, AppleScript, optional MCP, and local mirror readiness

### selection

Current DEVONthink GUI selection

- **`devonthink-pp-cli selection get`** - Return currently selected records
- **`devonthink-pp-cli selection snapshot`** - Capture the current selection as a reusable workflow seed

### sheets

DEVONthink sheets

- **`devonthink-pp-cli sheets <uuid>`** - Read a sheet as structured rows

### tags

Tag taxonomy and hygiene

- **`devonthink-pp-cli tags`** - Analyze tags for duplicates, case drift, action tags, and maintenance tags


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
devonthink-pp-cli databases

# JSON for scripting and agents
devonthink-pp-cli databases --json

# Filter to specific fields
devonthink-pp-cli databases --json --select id,name,status

# Dry run — show the request without sending
devonthink-pp-cli databases --dry-run

# Agent mode — JSON + compact + no prompts in one flag
devonthink-pp-cli databases --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
devonthink-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/devonthink-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **doctor reports DEVONthink is not running** — Open DEVONthink locally, then rerun devonthink-pp-cli doctor.
- **MCP passthrough commands fail** — Enable DEVONthink Settings > AI > MCP and verify the local endpoint or use native CLI commands instead.
- **mirror search returns no rows** — Run devonthink-pp-cli mirror sync before querying the local mirror.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**dvcrn/devonthink-cli**](https://github.com/dvcrn/devonthink-cli) — TypeScript (15 stars)
- [**2b3pro/Devonthink-MCP-CLI**](https://github.com/2b3pro/Devonthink-MCP-CLI) — JavaScript (3 stars)
- [**TomBener/dtx**](https://github.com/TomBener/dtx) — TypeScript (3 stars)
- [**fenrick/devonthink-cli**](https://github.com/fenrick/devonthink-cli) — Swift (2 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
