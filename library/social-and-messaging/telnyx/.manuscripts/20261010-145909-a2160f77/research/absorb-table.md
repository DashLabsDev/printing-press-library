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
| 17 | AI assistants manage | MCP create_assistant | (generated endpoint) ai assistants (if in scoped spec) | |
| 18 | Sync / search / analytics framework | none | telnyx-pp-cli sync / search / analytics | offline FTS |
