## Customer model
- P1 Property ops lead texting guests/vendors from owned numbers; frustration: no per-counterpart conversation view, failures/spend stitched manually.
- P2 Small-operator number admin (5-50 numbers); frustration: number/profile/10DLC campaign data never joined; missing campaign fails silently.
- P3 Agent builder sending SMS via LLM agent; frustration: no dry-run/confirm, no offline history check before send.

## Candidates (pre-cut)
conversation, numbers audit, spend, failures, numbers map, inbox unanswered, send (safe), numbers provision, contacts, webhooks failing, 10dlc status, volume, conversation summarize, balance runway.

## Survivors and kills
### Survivors
conversation 9, numbers audit 9, spend 8, failures 7, inbox unanswered 7, send 7 (all hand-code).
### Killed candidates
| Feature | Kill reason | Closest sibling |
|---|---|---|
| numbers map | redundant with audit --all | numbers audit |
| contacts | analytics/search query | conversation |
| numbers provision | chained paid writes, scope creep | numbers audit |
| webhooks failing | thin persona; generated list suffices | failures |
| 10dlc status | folded into audit | numbers audit |
| volume | analytics --group-by | spend |
| conversation summarize | LLM dependency | conversation |
| balance runway | folded into spend --runway | spend |
