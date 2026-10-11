# Telnyx CLI Brief

## API Identity
- Domain: CPaaS (SMS/MMS, voice, phone numbers, 10DLC compliance, fax, SIM/IoT, AI assistants, storage, email).
- Users: developers and small operators running SMS/voice on owned numbers (e.g. property ops texting guests/vendors), plus agent builders.
- Data profile: REST v2 at https://api.telnyx.com/v2, OpenAPI 3.1 (team-telnyx/openapi spec3.json, 942 paths / 1,394 ops). Bearer auth (`Authorization: Bearer <key>`). JSON:API-ish `{data, meta}` envelopes; page[number]/page[size] pagination, some cursor endpoints (page[after]).

## Reachability Risk
- Low. Official, actively maintained spec (SDKs pushed 2026-10-09). Key available locally as TELNYX_API_TOKEN (official tools use TELNYX_API_KEY — accept both).

## Top Workflows
1. Send an SMS/MMS from an owned number and check delivery status (messages create/get).
2. Inventory: list owned phone numbers, which messaging profile / connection each is on, and monthly cost.
3. Search and buy numbers (available_phone_numbers → number_orders).
4. Audit history: message and call detail records (detail_records, MDR/CDR reports) — who texted whom, failures, spend.
5. 10DLC compliance: brand/campaign status and number-to-campaign assignment.

## Table Stakes (official Stainless `telnyx` CLI + MCP server)
- Full resource coverage, json/yaml/jsonl output, --debug, base-url override.
- Official remote MCP at api.telnyx.com/v2/mcp; Agent Skills plugin.
- Gaps: no local store, no offline history/search, no cross-entity views (number ↔ profile ↔ campaign), no spend rollups, no dry-run safety on send/buy.

## Data Layer
- Primary entities: phone_numbers, messaging_profiles, messages/detail records, calls/CDRs, connections, 10DLC brands/campaigns, balance.
- Sync cursor: detail-record date filters (filter[created_at][gte]); full refresh for small inventories.
- FTS/search: message bodies and counterpart numbers in synced MDRs.

## Product Thesis
- Name: telnyx-pp-cli
- Why: the official CLI is a thin 1:1 wrapper. This one adds a local SQLite mirror of numbers and message/call history with FTS, conversation views per counterpart number, spend and failure rollups, a number health/compliance audit (unassigned profile, missing 10DLC campaign), and agent-safe sends (--dry-run, explicit confirm).

## Build Priorities
1. Scope the 1,394-op spec to core resources (messaging, numbers, ordering, calls/CDR, connections, 10DLC, balance, recordings) — avoid 118 AI and legacy endpoints bloating the surface.
2. Sync + FTS over message detail records; `conversation <number>` and `search`.
3. `numbers audit` and `spend` rollups.
4. Safe `send` with dry-run.

## Reachability Gate
- Decision: PASS. GET https://api.telnyx.com/v2/balance -> 200 with TELNYX_API_TOKEN (PAPERROUTE_TELNYX key -> 403, unused).

## SDK Surface Notes
- Official SDK send method: `messages.send` (telnyx-node `client.messages.send`, POST /messages). The generated `messages send` endpoint mirrors it; the novel `send` wraps it with preflight checks.
