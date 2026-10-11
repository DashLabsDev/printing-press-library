# Telnyx shipcheck

## Loop 1 (FAIL, 6/7)
- scorecard 98/100 A; verify PASS; dogfood FAIL:
  - novel 'send' depth mismatch vs generated 'messages send' (dogfood scanner ignores addNovelCommandIfAbsent wiring)
  - novel host check attributed op-id literal "messages.send" in messages_send.go to novel 'send' (leaf collision)
- sample probe: conversation empty-mirror output lacked counterpart token.

## Fixes
- Renamed novel 'send' -> 'text' (body flag --message) with user approval; manifest + research.json updated; regenerated with --force (hand edits preserved; stale send_test.go removed).
- Shaped empty results when no local mirror exists (conversation echoes counterpart + note).

## Loop 2 (PASS, 7/7)
- verify PASS, validate-narrative PASS, dogfood PASS (6/6 novel survived), workflow-verify PASS (no manifest), apify-audit PASS, verify-skill PASS, scorecard 98/100 Grade A; live sample probe 5/5.
Pass Rate: 100% (203/203 passed, 0 critical)
  Passed: 5/5  (100% pass rate, 1 skipped)

## Behavioral samples (live-synced data, read-only)
- numbers audit: flagged 3 of 4 owned numbers (2 missing messaging profile + 10DLC assignment, 1 missing 10DLC assignment); 1 OK.
- spend --since 30d --group-by number --runway: totals per number, live balance, runway days.
- failures --since 30d: 0 failed among 541 records (empty groups, valid JSON).
- inbox unanswered, conversation <number>: correct direction/counterpart normalization.
- text (preview): sender audit row, last 3 messages, no send without --yes.

Recommendation: ship
