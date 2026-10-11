# Neon CLI

**Neon API commands in one static binary, plus a local fleet mirror for stale-branch cleanup, usage trends and compute audits.**

Manage Neon projects, branches, computes, roles and databases from the terminal with agent-native JSON and no Node runtime. Sync your account locally to get stale, tree, usage trend and compute audit, and use preview up and wait for per-PR branch workflows.

## Install

The recommended path installs both the `neon-pp-cli` binary and the `pp-neon` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install neon
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install neon --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install neon --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install neon --agent claude-code
npx -y @mvanhorn/printing-press-library install neon --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.9 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/neon/cmd/neon-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/neon-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install neon --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-neon --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-neon --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install neon --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/neon-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `NEON_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/neon/cmd/neon-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "neon": {
      "command": "neon-pp-mcp",
      "env": {
        "NEON_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

```bash
# Check configuration without touching the network
neon-pp-cli doctor --dry-run

# Confirm your NEON_API_KEY works
neon-pp-cli projects list --json

# Mirror projects, branches and endpoints locally
neon-pp-cli sync

# See the whole fleet at once
neon-pp-cli tree

# Find branches nobody is using
neon-pp-cli stale --idle 14d

```

## Unique Features

### Fleet hygiene
- **`stale`** — Find idle or expired branches across every project from the local mirror (run sync first), with delete commands for non-default branches.

  _Reach for this before cleanup to see which preview branches are safe to remove._

  ```bash
  neon-pp-cli stale --idle 14d --agent
  ```
- **`tree`** — See every org, project, branch and compute in one hierarchy from the local mirror (run sync first).

  _Use it to orient in an unfamiliar Neon account in one call._

  ```bash
  neon-pp-cli tree --json
  ```
- **`prune`** — Preview idle or expired branches to delete (from the local mirror); --yes actually deletes them, always skipping default and protected branches.

  _Use after stale to actually clean up._

  ```bash
  neon-pp-cli prune --idle 14d
  ```

### Cost visibility
- **`usage trend`** — Track compute, storage and transfer per project with period-over-period deltas; on plans without consumption history (below Launch/Scale) it uses per-sync snapshots (basis: sync-snapshots).

  _Use for questions about how usage changed over time._

  ```bash
  neon-pp-cli usage trend --since 30d --agent
  ```
- **`usage top`** — Rank projects by compute, storage, transfer or written data; on plans without consumption history it ranks period-to-date totals from sync snapshots (basis: sync-snapshots).

  _Use to find which project drove a bill spike._

  ```bash
  neon-pp-cli usage top --metric compute --since 7d --agent
  ```
- **`compute audit`** — Flag computes that never scale to zero, have fixed autoscaling, or are oversized on non-default branches, from the local mirror (run sync first).

  _Use to find the cheapest cost wins in a Neon fleet._

  ```bash
  neon-pp-cli compute audit --agent
  ```

### Preview branch workflow
- **`wait`** — Block until a branch's async operations finish; exits 0 when done, 6 if an operation failed, 8 on timeout.

  _Use in CI or agent flows before connecting to a freshly created branch._

  ```bash
  neon-pp-cli wait --branch pr-123 --project-id lucky-sun-123456 --timeout 5m
  ```
- **`preview up`** — Create a real branch (or reuse an existing one by name), wait until ready and print its connection string; --expires applies only to newly created branches.

  _Use for per-PR databases; safe to re-run._

  ```bash
  neon-pp-cli preview up pr-123 --project-id lucky-sun-123456 --expires 7d --agent
  ```

## Recipes

### Per-PR database

```bash
neon-pp-cli preview up pr-123 --project-id lucky-sun-123456 --expires 7d --agent
```

Creates or reuses the branch and prints its connection string.

### Cleanup preview

```bash
neon-pp-cli stale --idle 14d --agent --select branch_name,project_id,last_active_at
```

Narrow output to the fields you need for a cleanup decision.

### Bill spike

```bash
neon-pp-cli usage top --metric compute --since 7d
```

Rank projects by compute over the last week.

### Clean up idle branches

```bash
neon-pp-cli prune --idle 14d
```

Previews what would be deleted; re-run with --yes to delete (default and protected branches are always skipped).

### Save a default project

```bash
neon-pp-cli context set --project-id lucky-sun-123456
```

connection-string, preview up and wait then work without --project-id.

## Usage

Run `neon-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `NEON_CONFIG_DIR`, `NEON_DATA_DIR`, `NEON_STATE_DIR`, or `NEON_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `NEON_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export NEON_HOME=/srv/neon
neon-pp-cli doctor
```

Under `NEON_HOME=/srv/neon`, the four dirs resolve to `/srv/neon/config`, `/srv/neon/data`, `/srv/neon/state`, and `/srv/neon/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "neon": {
      "command": "neon-pp-mcp",
      "env": {
        "NEON_HOME": "/srv/neon"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `NEON_DATA_DIR` overrides an explicit `--home` for that kind. Use `NEON_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `NEON_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `neon-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### api-keys

These methods allow you to create and manage API keys for your Neon account. For related information, see [Manage API keys](https://neon.com/docs/manage/api-keys).

- **`neon-pp-cli api-keys create`** - Creates an API key.
The `key_name` is a user-specified name for the key.
Returns an `id` and `key`; the `key` is a randomly generated, 64-bit token required to access the Neon API.
Store the key securely — it is only returned once.
API keys can also be managed in the Neon Console.
See [Manage API keys](https://neon.com/docs/manage/api-keys/).
- **`neon-pp-cli api-keys list`** - Retrieves the API keys for your Neon account.
The response does not include API key tokens. A token is only provided when creating an API key.
API keys can also be managed in the Neon Console.
For more information, see [Manage API keys](https://neon.com/docs/manage/api-keys/).
- **`neon-pp-cli api-keys revoke`** - Revokes the specified API key.
An API key that is no longer needed can be revoked.
This action cannot be reversed.
API keys can also be managed in the Neon Console.
See [Manage API keys](https://neon.com/docs/manage/api-keys/).

### auth_details

Manage auth details

- **`neon-pp-cli auth-details`** - Returns authentication details for the credentials used in the request,
including the credential type (API key, Bearer token, or OAuth session)
and the associated identity.

### consumption-history

Manage consumption history

- **`neon-pp-cli consumption-history get-per-branch-v2`** - Returns consumption metrics for each branch across one or more projects listed in
`project_ids` (1 to 100 projects). Available for accounts on paid usage-based Launch, Scale,
Agent, and Enterprise plans.

History starts when the account first ingests branch-level consumption data.

The `metrics` query parameter is required. Only these six values are supported on this
endpoint:
`compute_unit_seconds`, `root_branch_bytes_month`, `child_branch_bytes_month`,
`instant_restore_bytes_month`, `public_network_transfer_bytes`, `private_network_transfer_bytes`.

This endpoint does not support `extra_branches_month` or `snapshot_storage_bytes_month`.
Use `GET /consumption_history/v2/projects` for those.

Consumption metrics within each branch are returned in ascending time order (oldest first).
This request does not wake project computes.
- **`neon-pp-cli consumption-history get-per-project`** - Retrieves consumption metrics for Scale, Business, and Enterprise plan projects. History begins at the time of upgrade.
Results are ordered by time in ascending order (oldest to newest).
Issuing a call to this API does not wake a project's compute endpoint.
- **`neon-pp-cli consumption-history get-per-project-v2`** - Returns consumption metrics for up to `limit` projects per page. If `project_ids` is omitted,
projects in the organization are included across pages (use `cursor`). If `project_ids` is
provided, the response is limited to those projects (up to 100). Available for accounts on
Launch, Scale, Agent, Business, and Enterprise plans.

History starts when the account upgrades to an eligible plan.

The `metrics` query parameter is required. Supported values:
`compute_unit_seconds`, `root_branch_bytes_month`, `child_branch_bytes_month`,
`instant_restore_bytes_month`, `public_network_transfer_bytes`, `private_network_transfer_bytes`,
`extra_branches_month`, `snapshot_storage_bytes_month`.

Consumption metrics within each project are returned in ascending time order (oldest first).
This request does not wake project computes.

### organizations

These methods allow you to manage your Neon organizations.

- **`neon-pp-cli organizations <org_id>`** - Retrieves details for the specified organization, including its name, plan, and configuration.

### projects

These methods allow you to create and manage Neon projects. For related information, see [Manage projects](https://neon.com/docs/manage/projects).

- **`neon-pp-cli projects create`** - Creates a Neon project within an organization.
If using a personal API key, include the `org_id` parameter to specify which organization to create the project in.
If using an org API key, `org_id` is automatically inferred from the key.
Plan limits define how many projects you can create.
For more information, see [Manage projects](https://neon.com/docs/manage/projects/).

You can specify a region and Postgres version in the request body.
Neon supports Postgres 14 through 18, with 19 rolling out to enabled regions.
For supported regions and `region_id` values, see [Regions](https://neon.com/docs/introduction/regions/).
- **`neon-pp-cli projects create-neon-auth-integration`** - Deprecated. Use `/projects/{project_id}/branches/{branch_id}/auth` instead. Removal scheduled for March 1, 2026.
Use this endpoint if the frontend integration flow can't be used.
- **`neon-pp-cli projects create-neon-auth-new-user`** - Deprecated. Use `/projects/{project_id}/branches/{branch_id}/auth/users` instead. Removal scheduled for March 1, 2026.
The user will be created in your neon_auth.users_sync table and automatically propagated to your auth project, whether Neon-managed or provider-owned.
- **`neon-pp-cli projects create-neon-auth-provider-sdkkeys`** - Generates SDK or API Keys for the auth provider. These might be called different things depending
on the auth provider you're using, but are generally used for setting up the frontend and backend SDKs.
- **`neon-pp-cli projects delete`** - Deletes the specified project and all its endpoints, branches, databases, and users.
Deleted projects can be recovered within 7 days using `POST /projects/{project_id}/recover`.
To list recoverable projects, use `GET /projects?recoverable=true`.
- **`neon-pp-cli projects get`** - Retrieves information about the specified project.
Returned details include the project settings, compute configuration, history retention, owner information, and current usage metrics.
- **`neon-pp-cli projects list`** - Retrieves a list of projects for the specified organization.
If using a personal API key, include the `org_id` parameter to specify which organization to work with.
If using an org API key, `org_id` is automatically inferred from the key.
For more information, see [Manage organizations using the Neon API](https://neon.com/docs/manage/orgs-api)
and [Manage projects](https://neon.com/docs/manage/projects/).
- **`neon-pp-cli projects list-shared`** - Retrieves a list of projects shared with your Neon account.
For more information, see [Manage projects](https://neon.com/docs/manage/projects/).
- **`neon-pp-cli projects transfer-neon-auth-provider`** - Transfers ownership of your Neon-managed auth project to your own auth provider account.
- **`neon-pp-cli projects update`** - Updates the specified project.
Configurable properties include the project name, default compute settings, history retention period, and IP allowlist.

### regions

These methods allow you to inspect Neon regions.

- **`neon-pp-cli regions`** - Lists supported Neon regions.

**Note:** Not all regions are available to all organizations. Pass the `org_id`
parameter to get an accurate list of regions available to your organization.

### users

These methods allow you to manage your Neon user account.

- **`neon-pp-cli users get-current-info`** - Retrieves information about the currently authenticated Neon user,
including account identifiers, plan details, and linked auth accounts.
- **`neon-pp-cli users get-current-organizations`** - Retrieves the organizations that the currently authenticated user belongs to.

When called with an organization- or project-scoped API key (which is not
tied to a user), this returns the single organization that owns the key.
- **`neon-pp-cli users transfer-projects-from-to-org`** - Deprecated. Personal accounts have been migrated to organizations, so this operation no longer applies. Removal scheduled for July 1, 2026.


### compute


- **`neon-pp-cli compute audit`** - Flag computes that never scale to zero, have fixed autoscaling, or are oversized on non-default branches, from the local mirror (run sync first).


### preview


- **`neon-pp-cli preview up`** - Create a real branch (or reuse an existing one by name), wait until ready and print its connection string; --expires applies only to newly created branches.


### prune


- **`neon-pp-cli prune`** - Preview idle or expired branches to delete (from the local mirror); --yes actually deletes them, always skipping default and protected branches.


### stale


- **`neon-pp-cli stale`** - Find idle or expired branches across every project from the local mirror (run sync first), with delete commands for non-default branches.


### tree


- **`neon-pp-cli tree`** - See every org, project, branch and compute in one hierarchy from the local mirror (run sync first).


### usage


- **`neon-pp-cli usage trend`** - Track compute, storage and transfer per project with period-over-period deltas; on plans without consumption history (below Launch/Scale) it uses per-sync snapshots (basis: sync-snapshots).

- **`neon-pp-cli usage top`** - Rank projects by compute, storage, transfer or written data; on plans without consumption history it ranks period-to-date totals from sync snapshots (basis: sync-snapshots).


### wait


- **`neon-pp-cli wait`** - Block until a branch's async operations finish; exits 0 when done, 6 if an operation failed, 8 on timeout.




### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`neon-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`neon-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`neon-pp-cli learnings list`** - Inspect taught rows
- **`neon-pp-cli learnings forget <query>`** - Undo a teach
- **`neon-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`neon-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`neon-pp-cli teach-pattern`** - Install a query/resource template up front
- **`neon-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `NEON_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `neon-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
neon-pp-cli projects list

# JSON for scripting and agents
neon-pp-cli projects list --json
# Filter to specific fields
neon-pp-cli projects list --json --select projects.id,projects.name

# Dry run — show the request without sending
neon-pp-cli projects list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
neon-pp-cli projects list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `6` partial failure or failed Neon operation, `7` rate limited, `8` operation wait timed out, `10` config error.

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `NEON_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:

- `neon-pp-cli projects list`
- `neon-pp-cli projects get`
- `neon-pp-cli stale`, `neon-pp-cli tree`, `neon-pp-cli compute audit`, `neon-pp-cli usage top`, `neon-pp-cli usage trend` and `neon-pp-cli prune` read the local mirror only; run `neon-pp-cli sync` first.

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Health Check

```bash
neon-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `neon-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/neon-pp-cli/config.toml`; `--home`, `NEON_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `NEON_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `neon-pp-cli doctor` reports `agentcookie: detected` and `auth status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `neon-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $NEON_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized** — Create an API key in the Neon console (Account settings > API keys), then: export NEON_API_KEY=napi_xxx
- **404 not allowed for organization API keys** — Org API keys cannot call /users/me, /regions or /api_keys; use projects list and organizations commands instead
- **423 Locked when creating or deleting a branch** — Another operation is running: neon-pp-cli wait --branch pr-123 --project-id lucky-sun-123456, then retry
- **Local commands show no data** — Run neon-pp-cli sync first

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**mcp-server-neon**](https://github.com/neondatabase/mcp-server-neon) — TypeScript (650 stars)
- [**neonctl**](https://github.com/neondatabase/neonctl) — TypeScript (117 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
