# Telnyx Absorb Manifest

Spec scope: core-families spec (161 paths / 211 ops) derived from team-telnyx/openapi spec3.json.

## Absorb Manifest

### Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Send SMS/MMS | official telnyx CLI `messages send`; MCP send_message | (generated endpoint) messages send | --dry-run, --stdin, typed exits |
| 2 | Get message by id | telnyx CLI messages retrieve | (generated endpoint) messages get | --select, agent output |
| 3 | List/manage messaging profiles | telnyx CLI messaging-profiles | (generated endpoint) messaging-profiles list/get/create/update/delete | local mirror |
| 4 | List owned phone numbers | telnyx CLI phone-numbers list; MCP list_phone_numbers | (generated endpoint) phone-numbers list | synced, offline |
| 5 | Update phone number (profile/connection/tags) | telnyx CLI phone-numbers update | (generated endpoint) phone-numbers update | --dry-run |
| 6 | Search available numbers | telnyx CLI available-phone-numbers list; MCP search | (generated endpoint) available-phone-numbers list | filters as flags |
| 7 | Order numbers | telnyx CLI number-orders create; MCP buy_number | (generated endpoint) number-orders create | --dry-run guard |
| 8 | Dial / control calls | telnyx CLI calls dial, actions | (generated endpoint) calls dial + actions | dry-run |
| 9 | Call control apps / connections | telnyx CLI | (generated endpoint) call-control-applications, connections list | local mirror |
| 10 | Account balance | telnyx CLI balance | (generated endpoint) balance get | agent output |
| 11 | Detail records (MDR/CDR) search | telnyx CLI detail-records | (generated endpoint) detail-records list | synced for offline history |
| 12 | 10DLC brands/campaigns | telnyx CLI 10dlc | (generated endpoint) 10dlc brand/campaign list/get | local mirror |
| 13 | Recordings list/get | telnyx CLI recordings | (generated endpoint) recordings list | |
| 14 | Fax send/list | telnyx CLI faxes | (generated endpoint) faxes | |
| 15 | Verify (2FA) send/verify | telnyx CLI verifications; MCP | (generated endpoint) verifications | |
| 16 | Webhook deliveries inspect | telnyx CLI webhook-deliveries | (generated endpoint) webhook-deliveries list | |
| 17 | AI assistants manage | MCP create_assistant | (excluded) out of approved spec scope | user approved exclusion |
| 18 | Sync / search / analytics framework | none | telnyx-pp-cli sync / search / analytics | offline FTS |

### Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Conversation view (9/10) | conversation | hand-code | Per-counterpart timeline from synced MDRs in SQLite; no API call groups by counterpart | none |
| 2 | Number audit (9/10) | numbers audit | hand-code | Joins phone numbers, messaging profiles, 10DLC campaign assignments and brand status | Use this command to find numbers that can't send or are non-compliant. Do NOT use it for cost; use 'spend' instead. |
| 3 | Spend rollup (8/10) | spend | hand-code | Aggregates MDR/CDR cost locally by number/profile/direction/day, --runway vs balance | Use this command for message/call cost totals. Do NOT use it for the account balance; use 'balance get' instead. |
| 4 | Failure report (7/10) | failures | hand-code | Groups failed/undelivered MDRs by error code and counterpart | none |
| 5 | Unanswered threads (7/10) | inbox unanswered | hand-code | Cross-direction join: latest message inbound with no later outbound | none |
| 6 | Agent-safe send (7/10) | text | hand-code | Preflight audit row + last 3 messages before POST /messages; --yes or --dry-run required | Use this command for an agent-safe send with preflight checks and recent history. Do NOT use it for raw API sends; use 'messages send' instead. (renamed from send at user request: printing-press dogfood leaf-collision false positive) |
