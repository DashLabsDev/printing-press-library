# OpenRouter CLI

**The only OpenRouter CLI built for fleet operations: budgets, anomaly alarms, and failover maps over a local usage ledger — not another chat wrapper.**

Every other OpenRouter tool wraps chat; this one governs it. A synced local mirror of models, generations, and credits powers per-cron cost attribution (usage cost-by), pre-flight budget gates (budget check), degradation watches (providers degraded), and runway projections (credits runway) that no single API call can answer. The full endpoint surface — catalog, keys admin, activity, analytics — ships alongside as typed commands.

Learn more at [OpenRouter](https://openrouter.ai/docs).

Created by [@rvdlaar](https://github.com/rvdlaar) (Rick van de Laar).
Contributors: [@Quantman1974](https://github.com/Quantman1974) (Quantman1974), [@Bhumika-1432006](https://github.com/Bhumika-1432006) (Bhumika-1432006).

## Install

The recommended path installs both the `openrouter-pp-cli` binary and the `pp-openrouter` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install openrouter
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install openrouter --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install openrouter --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install openrouter --agent claude-code
npx -y @mvanhorn/printing-press-library install openrouter --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/openrouter-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install openrouter --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-openrouter --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-openrouter --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install openrouter --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/openrouter-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `OPENROUTER_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "openrouter": {
      "command": "openrouter-pp-mcp",
      "env": {
        "OPENROUTER_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Standard bearer auth with your inference key via OPENROUTER_API_KEY covers models, credits, key status, and chat. Two surfaces need the separate provisioning (management) key created in the dashboard: key administration (/keys) and account-wide usage rollups (/activity, analytics). A 401 or 403 on those commands with an otherwise-working key means you are holding the inference key where the management key is required — not a broken setup. Live-verified 2026-09: /activity returns 403 'Only management keys can fetch activity' on an inference key.

## Quick Start

```bash
# Health check: config, auth presence, and local store — works before any credentials
openrouter-pp-cli doctor --dry-run


# Build the local mirror the transcendence commands run on
openrouter-pp-cli sync --resources models,generations --since 7d


# Shortlist capable models offline instead of paging the full catalog
openrouter-pp-cli models query "tools=true cost.completion<5 ctx>=128k"


# Attribute the week's spend to the cron or agent that incurred it
openrouter-pp-cli usage cost-by --since 7d


# Project when prepaid credits hit zero at the current burn rate
openrouter-pp-cli credits runway

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Catalog intelligence

- **`models query`** — Shortlist models by capability, price, and context window with a structured query over the synced catalog — offline and instant.

  _Reach for this instead of fetching the full catalog: it answers a shortlist question in ~200 tokens instead of hundreds of KB._

  ```bash
  openrouter-pp-cli models query "tools=true cost.completion<5 ctx>=128k" --agent
  ```
- **`models churn`** — See what changed in the model catalog between syncs: additions, removals, and repricings with deltas.

  _Use this to catch a pinned model repricing or vanishing before the invoice does._

  ```bash
  openrouter-pp-cli models churn --since 7d --agent
  ```

### Cost governance for agent fleets

- **`usage cost-by`** — Attribute spend to the cron, agent, or lineage that incurred it, over any window.

  _Use this when the question is 'which job burned the money', not 'which model'._

  ```bash
  openrouter-pp-cli usage cost-by --since 7d --agent
  ```
- **`usage anomaly`** — Flag per-model cost spikes against the trailing baseline before they compound.

  _Run this in a daily cron to catch a misbehaving lineage the day it regresses, not at invoice time._

  ```bash
  openrouter-pp-cli usage anomaly --since 24h --agent
  ```
- **`budget`** — Set spend caps per cron or agent and enforce them pre-flight with typed exit codes.

  _Gate a scheduled run on exit code 0/8 so an over-budget lineage never fires._

  ```bash
  openrouter-pp-cli budget check nightly-drafter
  ```
- **`generation explain`** — Break one generation into its cost anatomy: tokens, latency, and the delta versus the cheapest provider.

  _The next command after an anomaly fires: it turns 'cost spiked' into 'this call, this provider, this delta'._

  ```bash
  openrouter-pp-cli generation explain gen-abc123 --agent
  ```
- **`usage reconcile`** — Verify the local usage mirror against upstream daily totals and flag days that disagree.

  _Run this before trusting any local cost analysis — it is the trust root for the other usage commands._

  ```bash
  openrouter-pp-cli usage reconcile --since 7d --agent
  ```

### Provider health and dispatch

- **`providers degraded`** — See which providers or models are currently degraded before dispatching work to them.

  _Check this before dispatch to route around trouble instead of retrying into it._

  ```bash
  openrouter-pp-cli providers degraded --agent
  ```
- **`endpoints failover`** — Rank the providers serving one model by status, price, and observed latency for dispatch order.

  _Feed this to a router to pre-empt degraded providers for a specific model._

  ```bash
  openrouter-pp-cli endpoints failover deepseek/deepseek-v4 --agent
  ```

### Runway and rate-limit headroom

- **`key eta`** — Project the date your key's spend cap trips, from the observed burn rate.

  _Use this to plan the week's runs against the cap instead of discovering it mid-run._

  ```bash
  openrouter-pp-cli key eta --agent
  ```
- **`limits status`** — One view of current headroom: key-cap remaining, free-tier daily quota, and today's free-model burn.

  _Check before a free-tier dev loop or fleet dispatch to avoid opaque 429s._

  ```bash
  openrouter-pp-cli limits status --agent
  ```
- **`credits runway`** — Project days-to-zero for prepaid credits at the trailing burn rate — the 402 leading indicator.

  _A daily cron on this answers 'when do we hit 402' while there is still time to top up._

  ```bash
  openrouter-pp-cli credits runway --agent
  ```

## Recipes


### Monday cost review

```bash
openrouter-pp-cli usage cost-by --since 7d --agent
```

The weekly attribution pass: spend per cron/agent tag, ready to paste into a review note.

### Anomaly to root cause in two commands

```bash
openrouter-pp-cli usage anomaly --since 24h --agent
```

When a model flags, follow with 'generation explain <id>' to get tokens, latency, and the cheapest-provider delta for the offending call.

### Pre-dispatch health gate

```bash
openrouter-pp-cli providers degraded --agent
```

Run before firing a lineage; pair with 'endpoints failover <model>' to pick the dispatch order for the model you are about to use.

### Narrow a deep endpoint payload

```bash
openrouter-pp-cli models endpoints list deepseek deepseek-v4 --agent --select data.endpoints.provider_name,data.endpoints.pricing.completion,data.endpoints.status
```

Per-model provider listings nest deeply; --select keeps the answer to the three fields a router actually needs.

### Budget-gated cron pattern

```bash
openrouter-pp-cli budget check nightly-drafter
```

Exit code 0 under cap, 8 over — gate a scheduled run on this before it fires (e.g. 'budget check X && systemctl --user start X.service').

## Usage

Run `openrouter-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `OPENROUTER_CONFIG_DIR`, `OPENROUTER_DATA_DIR`, `OPENROUTER_STATE_DIR`, or `OPENROUTER_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `OPENROUTER_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export OPENROUTER_HOME=/srv/openrouter
openrouter-pp-cli doctor
```

Under `OPENROUTER_HOME=/srv/openrouter`, the four dirs resolve to `/srv/openrouter/config`, `/srv/openrouter/data`, `/srv/openrouter/state`, and `/srv/openrouter/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "openrouter": {
      "command": "openrouter-pp-mcp",
      "env": {
        "OPENROUTER_HOME": "/srv/openrouter"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `OPENROUTER_DATA_DIR` overrides an explicit `--home` for that kind. Use `OPENROUTER_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `OPENROUTER_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `openrouter-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### activity

Manage activity

- **`openrouter-pp-cli activity`** - Returns user activity data grouped by endpoint for the last 30 (completed) UTC days. Pass `workspace_id` to scope the response to a single workspace. Pass `group_by=workspace` to split each row per workspace and include `workspace_id` on every item; by default rows are aggregated across workspaces and `workspace_id` is not returned. Activity recorded before workspace resolution existed is permanently attributed to the account default workspace (no backfill is possible). [Management key](/docs/guides/overview/auth/management-api-keys) required.

### audio

Manage audio

- **`openrouter-pp-cli audio create-speech`** - Synthesizes audio from the input text. Returns a raw audio bytestream in the requested format (e.g. mp3, pcm, wav).

Exit codes:
  0 success
  2 structured output refused: response is binary audio; redirect stdout or use --deliver file:<path>
- **`openrouter-pp-cli audio create-transcriptions`** - Transcribes audio into text. Accepts base64-encoded audio input as JSON or an OpenAI-style multipart/form-data file upload, and returns the transcribed text.

### benchmarks

Benchmarks endpoints

- **`openrouter-pp-cli benchmarks`** - Unified benchmark endpoint that aggregates scores from multiple benchmark sources (Artificial Analysis, Design Arena, and OpenRouter's own tau-bench, GPQA, and web-search evals). Filter by source to reproduce the exact shapes from the legacy per-source endpoints, or use task_type to find models suited for specific workloads. Use task_type=search (or a search_* benchmark_type) for OpenRouter's search benchmarks, which publish each model's highest-scoring eligible evaluation configuration with same-configuration runs combined by task-weighted mean. Authenticate with any valid OpenRouter API key. Rate-limited to 30 requests/minute per key and 500 requests/day per account.

### byok

BYOK endpoints

- **`openrouter-pp-cli byok create-byokkey`** - Create a new bring-your-own-key (BYOK) provider credential. The raw key is encrypted at rest and never returned in API responses. When `workspace_id` is omitted, the credential is created in the default workspace; if that default has been deleted, the request returns a 400 and you must pass `workspace_id` explicitly. Treat the raw key as write-only; it is never returned after creation. Use `allowed_api_key_hashes` to restrict the credential to specific OpenRouter API keys. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-pp-cli byok delete-byokkey`** - Delete (soft-delete) a bring-your-own-key (BYOK) provider credential by its `id`. The encrypted key material is wiped and the record is marked as deleted. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-pp-cli byok get-byokkey`** - Get a single bring-your-own-key (BYOK) provider credential by its `id`. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-pp-cli byok list-byokkeys`** - List the bring-your-own-key (BYOK) provider credentials for the authenticated entity's default workspace. Use the `workspace_id` query parameter to scope the result to a different workspace, or the `provider` query parameter to filter by upstream provider. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-pp-cli byok update-byokkey`** - Update an existing bring-your-own-key (BYOK) provider credential by its `id`. Include the `key` field to rotate the raw provider API key in-place (the previous key material is overwritten). Use `allowed_api_key_hashes` to restrict the credential to specific OpenRouter API keys (`null` clears the restriction). [Management key](/docs/guides/overview/auth/management-api-keys) required.

### chat

Chat completion endpoints

- **`openrouter-pp-cli chat`** - Sends a request for a model response for the given chat conversation. Supports both streaming and non-streaming modes.

### classifications

Task classification market-share endpoints

- **`openrouter-pp-cli classifications`** - Returns the market-share breakdown of OpenRouter traffic by task classification
(e.g. code generation, web search, summarization) over a trailing time window.

Each classification reports its share of classified sampled requests (`usage_share`)
and classified sampled token volume (`token_share`) as fractions between 0 and 1.
The unclassified `other` bucket is excluded. Absolute volumes are not exposed
because the underlying data is sampled.

Each classification also includes a `models` array listing the top models by
request volume within that classification, with their within-tag usage and token shares.

Classifications are grouped into macro-categories (Code, Data, Agent, General)
with aggregate shares provided for each.

Authenticate with any valid OpenRouter API key (same key used for inference).
Rate-limited to 30 requests/minute per key and 500 requests/day per account.

When republishing or quoting this data, cite as:
"Source: OpenRouter (openrouter.ai/rankings), as of {as_of}."

### containers

Containers endpoints


### credits

Credit management endpoints

- **`openrouter-pp-cli credits create-coinbase-charge`** - Deprecated. The Coinbase APIs used by this endpoint have been deprecated, so Coinbase Commerce charges have been removed. Use the web credits purchase flow instead.
- **`openrouter-pp-cli credits get`** - Get total credits purchased and used for the authenticated user. [Management key](/docs/guides/overview/auth/management-api-keys) required.

### datasets

Public OpenRouter usage datasets. Data returned by these endpoints is licensed under CC BY 4.0 (https://creativecommons.org/licenses/by/4.0/): reuse and republish it, including commercially, with attribution to OpenRouter.

- **`openrouter-pp-cli datasets get-app-rankings`** - Returns the top public apps on OpenRouter ranked by token usage inside the requested
date window, matching the public apps marketplace on openrouter.ai/apps. Token totals
are `prompt_tokens + completion_tokens`; hidden and private apps are excluded and
traffic from related app aliases is merged into the canonical visible app.

`sort=popular` (default) ranks by total token volume inside the window.
`sort=trending` ranks by absolute excess token growth: window volume minus the average
volume of the three equal-length periods immediately preceding the window. Apps with
no excess growth are omitted, so `trending` may return fewer than `limit` rows.

Filter with `category` (marketplace category group, e.g. `coding`) or `subcategory`
(e.g. `cli-agent`). Ranks are re-numbered 1..N after filtering. Page with `offset` —
`rank` stays absolute, so the first row of `offset=50` is `rank: 51`.

Authenticate with any valid OpenRouter API key (same key used for inference).
Rate-limited to 30 requests/minute per key and 500 requests/day per account.

When republishing or quoting this dataset, OpenRouter must be cited as:
"Source: OpenRouter (openrouter.ai/apps), as of {as_of}."

Token counts come from each upstream provider's own tokenizer, so a token attributed
to one app is not directly comparable to a token attributed to another app whose
traffic flows through a different provider.

Licensed under [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/): reuse and republish with attribution to OpenRouter.
- **`openrouter-pp-cli datasets get-rankings-daily`** - Returns the top 50 public models per day by total token usage on OpenRouter, plus a
single aggregated `other` row per day that sums every model outside that top 50.
Token totals are `prompt_tokens + completion_tokens`, matching the public rankings
chart on openrouter.ai/rankings.

Each row is a distinct `(date, model_permaslug)` pair. The `other` row uses the
reserved permaslug `other` and is always returned last within its date, so callers
can compute `top-50 traffic / total daily traffic` without a second request.

Optional filters slice the dataset. `period` (`day`/`week`/`month`) sets the time
grain. `modality` and `context_bucket` narrow the exact dataset by output/input
modality (or tool-calling activity) and request context length. `category` and
`language_type` instead read a sampled, upsampled dataset whose `total_tokens` are
weekly-grain estimates — they cannot be combined with each other or with the exact
filters, and reject `period=day` with a 400.

Authenticate with any valid OpenRouter API key (same key used for inference).
Rate-limited to 30 requests/minute per key and 500 requests/day per account.

When republishing or quoting this dataset, OpenRouter must be cited as:
"Source: OpenRouter (openrouter.ai/rankings), as of {as_of}."

Token counts come from each upstream provider's own tokenizer (Anthropic counts
are as reported by Anthropic, OpenAI counts are as reported by OpenAI, etc.), so
a token in one row is not directly comparable to a token in another row from a
different provider.

Licensed under [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/): reuse and republish with attribution to OpenRouter.
- **`openrouter-pp-cli datasets get-session-cost`** - Returns weekly refreshed, aggregated cost-per-session cells for the published harnesses.
Sessions are never pooled across apps. Medians are of per-session USD spend, and
privacy-preserving aggregation never exposes clerk_user_id values or per-session rows.

Filter by `app_slug`, `model`, or `turn_range`. Filtering by `model` alone works across apps
for harness-vs-harness comparison at a fixed model. Results refresh weekly and include the source snapshot
window in `meta`.

Licensed under [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/): reuse and republish with attribution to OpenRouter.

### embeddings

Text embedding endpoints

- **`openrouter-pp-cli embeddings create`** - Submits an embedding request to the embeddings router
- **`openrouter-pp-cli embeddings list-models`** - Returns a list of all available embeddings models and their properties

### endpoints

Endpoint information

- **`openrouter-pp-cli endpoints`** - Preview the impact of ZDR on the available endpoints

### files

Files endpoints

- **`openrouter-pp-cli files delete`** - Deletes a file owned by the requesting workspace. Deletion is irreversible.
- **`openrouter-pp-cli files get-metadata`** - Retrieves metadata for a single file owned by the requesting workspace.
- **`openrouter-pp-cli files list`** - Lists files belonging to the workspace of the authenticating API key.
- **`openrouter-pp-cli files upload`** - Uploads a file to be referenced in future API calls. The file is stored under the workspace of the authenticating API key. Maximum file size: 100 MB; empty files are rejected. The file type is determined from the file contents — not the filename or the declared content type — and must be a PDF, a PNG/JPEG/GIF/WebP image, a DOCX/XLSX/PPTX document, an MP3/WAV/FLAC/OGG audio file, or UTF-8 text. Text is reported by its structure as `application/json`, `application/x-ndjson`, `text/csv`, `text/markdown`, or `text/plain`.

### generation

Generation history endpoints

- **`openrouter-pp-cli generation get`** - Get request & usage metadata for a generation
- **`openrouter-pp-cli generation list-content`** - Get stored prompt, completion, and error content for a generation
- **`openrouter-pp-cli generation submit-feedback`** - Submit structured feedback on a generation the authenticated user made. [Management key](/docs/guides/overview/auth/management-api-keys) required.

### guardrails

Guardrails endpoints

- **`openrouter-pp-cli guardrails create`** - Create a new guardrail for the authenticated user. A newly created guardrail enforces nothing until it is assigned to API keys or organization members; `workspace_id` places the guardrail in a workspace but does not apply it to that workspace's traffic. To restrict all traffic in a workspace, update the workspace's default guardrail instead. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-pp-cli guardrails delete`** - Delete an existing guardrail. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-pp-cli guardrails get`** - Get a single guardrail by ID. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-pp-cli guardrails list`** - List all guardrails for the authenticated user. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-pp-cli guardrails list-key-assignments`** - List all API key guardrail assignments for the authenticated user. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-pp-cli guardrails list-member-assignments`** - List all organization member guardrail assignments for the authenticated user. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-pp-cli guardrails update`** - Update an existing guardrail, or materialize an unconfigured workspace default guardrail. Collection fields use replace semantics: send the full desired set on every update. [Management key](/docs/guides/overview/auth/management-api-keys) required.

### images

Images endpoints

- **`openrouter-pp-cli images create`** - Generates an image from a text prompt via the image generation router
- **`openrouter-pp-cli images list-model-endpoints`** - Returns the full per-endpoint records for an image model: each endpoint's definitive supported parameters, pricing, and passthrough allowlist.
- **`openrouter-pp-cli images list-models`** - Lists every image generation model with its top-level supported-parameter superset and a URL to its full per-endpoint records.

### key

Manage key

- **`openrouter-pp-cli key`** - Get information on the API key associated with the current authentication session

### keys

Manage keys

- **`openrouter-pp-cli keys create`** - Create a new API key for the authenticated user. The plaintext `key` is returned only in this response. Treat it as a write-only, sensitive value; it cannot be retrieved later. Authenticate with a [management key](/docs/guides/overview/auth/management-api-keys), or with a Connect client secret. `external_user` and `external_api_key` are accepted only with a client secret, and `external_user` is required there; supplying either field with a management key is rejected with 403.
- **`openrouter-pp-cli keys delete`** - Delete an existing API key. Authenticate with a [management key](/docs/guides/overview/auth/management-api-keys), or with a Connect client secret. A client secret reaches only the keys that same client created; any other key responds as if it does not exist.
- **`openrouter-pp-cli keys get`** - Get a single API key by hash. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-pp-cli keys list`** - List all API keys for the authenticated user. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-pp-cli keys update`** - Update an existing API key. Authenticate with a [management key](/docs/guides/overview/auth/management-api-keys), or with a Connect client secret. A client secret reaches only the keys that same client created; any other key responds as if it does not exist.

### messages

Manage messages

- **`openrouter-pp-cli messages`** - Creates a message using the Anthropic Messages API format. Supports text, images, PDFs, tools, and extended thinking.

### model

Model information endpoints

- **`openrouter-pp-cli model <author> <slug>`** - Returns full details for a single model identified by its author and slug (e.g. openai/gpt-4). Supports variant suffixes (e.g. openai/gpt-4:free) and resolves known slug aliases.

### models

Model information endpoints

- **`openrouter-pp-cli models get`** - List all models and their properties
- **`openrouter-pp-cli models list-count`** - Get total count of available models
- **`openrouter-pp-cli models list-user`** - List models filtered by user provider preferences, [privacy settings](https://openrouter.ai/docs/guides/privacy/provider-logging), and [guardrails](https://openrouter.ai/docs/guides/features/guardrails). Returns text-output models by default; pass `output_modalities` (a comma-separated list of `text`, `image`, `embeddings`, `audio`, `video`, `rerank`, `speech`, `transcription`, or `all`) to include other modalities. If requesting through a regional hostname, the results will be filtered to models that satisfy in-region routing for that region.

### observability

Observability endpoints

- **`openrouter-pp-cli observability create-destination`** - Create a new observability destination. A maximum of 5 destinations per type is allowed. Defaults to the authenticated entity's default workspace; use the `workspace_id` body field to scope to a different workspace. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-pp-cli observability delete-destination`** - Delete an existing observability destination. This performs a soft delete. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-pp-cli observability get-destination`** - Fetch a single observability destination by its UUID. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-pp-cli observability list-destinations`** - List the observability destinations configured for the authenticated entity's default workspace. Use the `workspace_id` query parameter to scope the result to a different workspace. Only destinations with stable release status are surfaced — destinations of other types are excluded. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-pp-cli observability update-destination`** - Update an existing observability destination. Only the fields provided in the request body are updated. [Management key](/docs/guides/overview/auth/management-api-keys) required.

### openrouter-analytics

Manage openrouter analytics

- **`openrouter-pp-cli openrouter-analytics get-meta`** - Returns the available metrics, dimensions, filter operators, and granularities for the analytics query endpoint. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-pp-cli openrouter-analytics query`** - Execute an analytics query with specified metrics, dimensions, filters, and time range. [Management key](/docs/guides/overview/auth/management-api-keys) required.

### openrouter-auth

Manage openrouter auth

- **`openrouter-pp-cli openrouter-auth create-keys-code`** - Create an authorization code for the PKCE flow to generate a user-controlled API key
- **`openrouter-pp-cli openrouter-auth exchange-code-for-apikey`** - Exchange an authorization code from the PKCE flow for a user-controlled API key

### organization

Organization endpoints

- **`openrouter-pp-cli organization`** - List all members of the organization associated with the authenticated management key. [Management key](/docs/guides/overview/auth/management-api-keys) required.

### presets

Presets endpoints

- **`openrouter-pp-cli presets get`** - Retrieves a preset by its slug with its currently designated version inline.
- **`openrouter-pp-cli presets list`** - Lists all presets for the authenticated user, ordered by most recently updated first.

### providers

Provider information endpoints

- **`openrouter-pp-cli providers`** - List all providers

### rerank

Rerank endpoints

- **`openrouter-pp-cli rerank`** - Submits a rerank request to the rerank router

### responses

OpenAI-compatible Responses API endpoints

- **`openrouter-pp-cli responses`** - Creates a streaming or non-streaming response using OpenResponses API format

### scim

SCIM endpoints

- **`openrouter-pp-cli scim create-group-mapping`** - Create a SCIM group-to-workspace role mapping. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-pp-cli scim delete-group-mapping`** - Delete a SCIM group-to-workspace mapping. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-pp-cli scim get-group-mapping`** - Get a SCIM group-to-workspace mapping. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-pp-cli scim list-group-mappings`** - List SCIM group-to-workspace mappings for the organization. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-pp-cli scim list-groups`** - List SCIM groups for the organization. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-pp-cli scim update-group-mapping`** - Update a SCIM group mapping role. [Management key](/docs/guides/overview/auth/management-api-keys) required.

### videos

Manage videos

- **`openrouter-pp-cli videos create`** - Submits a video generation request and returns a polling URL to check status
- **`openrouter-pp-cli videos get`** - Returns job status and content URLs when completed
- **`openrouter-pp-cli videos list-models`** - Returns a list of all available video generation models and their properties

### workspaces

Workspaces endpoints

- **`openrouter-pp-cli workspaces create`** - Create a new workspace for the authenticated user. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-pp-cli workspaces delete`** - Delete an existing workspace. Workspaces with active API keys cannot be deleted; remove the keys first. Deleting the default workspace requires confirm_default_workspace_deletion=true. Deleting any workspace permanently deletes its budgets and guardrails and disables its classifiers and broadcast destinations. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-pp-cli workspaces get`** - Get a single workspace by ID or slug. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-pp-cli workspaces list`** - List all workspaces for the authenticated user. [Management key](/docs/guides/overview/auth/management-api-keys) required.
- **`openrouter-pp-cli workspaces update`** - Update an existing workspace by ID or slug. [Management key](/docs/guides/overview/auth/management-api-keys) required.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`openrouter-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`openrouter-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`openrouter-pp-cli learnings list`** - Inspect taught rows
- **`openrouter-pp-cli learnings forget <query>`** - Undo a teach
- **`openrouter-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`openrouter-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`openrouter-pp-cli teach-pattern`** - Install a query/resource template up front
- **`openrouter-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `OPENROUTER_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `openrouter-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
openrouter-pp-cli benchmarks

# JSON for scripting and agents
openrouter-pp-cli benchmarks --json
# Filter to specific fields
openrouter-pp-cli benchmarks --json --select accuracy,accuracy_stddev,agentic_index

# Dry run — show the request without sending
openrouter-pp-cli benchmarks --dry-run

# Agent mode — JSON + compact + no prompts in one flag
openrouter-pp-cli benchmarks --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
openrouter-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `openrouter-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/openrouter-pp-cli/config.toml`; `--home`, `OPENROUTER_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `OPENROUTER_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `openrouter-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `openrouter-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $OPENROUTER_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **402 Payment Required on inference calls** — Credits are exhausted — run 'openrouter-pp-cli credits runway' to see the burn that got you here, then top up in the dashboard
- **429 on free-variant models while paid calls work** — Free-tier daily quota is separate and small — run 'openrouter-pp-cli limits status' to see today's free-model burn and your quota tier
- **401/403 on keys commands while other commands work** — Key administration needs a provisioning key, not the inference key — create one in the dashboard and set it per the auth docs
- **Local cost numbers look stale or wrong** — Run 'openrouter-pp-cli sync --resources generations --since 7d' then 'openrouter-pp-cli usage reconcile --since 7d' to verify the mirror against upstream
- **403 'Only management keys can fetch activity' on usage reconcile or activity sync** — These surfaces need the provisioning/management key, not the inference key — create one in the dashboard, or skip reconcile and rely on the generations ledger

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**mrgoonie/openrouter-cli**](https://github.com/mrgoonie/openrouter-cli) — TypeScript (31 stars)
- [**th3nolo/openrouter-mcp**](https://github.com/th3nolo/openrouter-mcp) — TypeScript (13 stars)
- [**physics91/openrouter-mcp**](https://github.com/physics91/openrouter-mcp) — Python (9 stars)
- [**jwill9999/openrouter-cli**](https://github.com/jwill9999/openrouter-cli) — TypeScript (5 stars)
- [**maxxie114/openrouter-cli**](https://github.com/maxxie114/openrouter-cli) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
