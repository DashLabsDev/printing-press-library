Manifest transcendence rows: 6 planned, 6 built. Phase 3 will not pass until all 6 ship.

# Telnyx CLI build log

## Built
- P0: sync works live for all 25 default resources (1,343 records, 0 errors). Fixes to generated sync:
  - Telnyx pagination uses page[number]/page[size]; generator emitted bare `page` (25 resources) so only page 1 synced. Fixed in paginationDefaults.
  - detail_records silently caps page[size] at 50; limit set to 50 so short-page termination works (1,105/1,105 MDRs).
  - GET /detail_records requires filter[record_type]; added applyTelnyxSyncDefaults (telnyx_sync_defaults.go) defaulting to messaging, overridable via --resource-param.
  - Removed active_calls dependent (422 for non-call-control connections; transient data).
- Auth: TELNYX_API_TOKEN accepted as alias for TELNYX_API_KEY.
- P1: generated endpoints for messages, messaging profiles, phone numbers, available numbers, number orders/reservations, calls + call actions, connections, call-control/TeXML apps, balance, detail records, 10DLC, recordings, faxes, verifications/verify profiles, webhook deliveries, short codes, media. Review gate: messaging-profiles list, balance, phone-numbers list pass --help/--dry-run/--json.
- P2 (all hand-code, verified on live-synced data):
  1. conversation (local) — per-counterpart timeline.
  2. numbers audit (local) — joins phone-numbers, phone-numbers-messaging, messaging-profiles, 10dlc assignments; correctly flagged 3 of 4 real numbers.
  3. spend (local; --runway makes one live GET /balance).
  4. failures (local).
  5. inbox unanswered (local).
  6. send (live; preview by default, --yes to send, harness refusal).
- Unit tests: telnyx_mdr_test.go (normalizePhone, smsSegments, decodeMDR, isFailedStatus).

## Deferred / limitations
- Telnyx MDRs contain no message bodies; conversation shows delivery metadata only.
- numbers audit checks 10DLC assignment status, not brand/campaign approval state (would need live per-campaign lookups).
- Spec scoped to core families (211 ops); AI, storage, IoT, porting, legacy excluded by user approval.
- Long nested call configs (answering_machine_detection_config, conversation_relay_config) flattened to free-form objects (MCP key length limit); /messages/rcs dropped.

## Generator limitations found (for retro)
- OpenAPI deepObject `page` param not expanded into page[number]/page[size] for pagination.
- dogfood collectRegisteredCommands only detects `rootCmd.AddCommand`, not generator-emitted `addNovelCommandIfAbsent`; novel `send` falsely reported as depth mismatch with `messages send` and attributed messages.send host.
- Nested request-body flag names >64 chars hard-fail generate instead of auto-truncating.
