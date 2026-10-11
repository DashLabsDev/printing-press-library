---
name: pp-telnyx
description: "Telnyx messaging, numbers, and voice endpoints, plus a synced message-history mirror, number setup audits, and spend rollups. Trigger phrases: `text a guest from telnyx`, `check telnyx message history`, `which telnyx numbers can't send`, `telnyx sms spend`, `use telnyx`."
author: "Greg Mushen"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - telnyx-pp-cli
    install:
      - kind: go
        bins: [telnyx-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/social-and-messaging/telnyx/cmd/telnyx-pp-cli
---

# Telnyx — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `telnyx-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install telnyx --cli-only
   ```
2. Verify: `telnyx-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.9 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/telnyx/cmd/telnyx-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Sync your Telnyx messaging detail records into a local database, then see any number's delivery timeline with conversation, who is waiting with inbox unanswered, and costs with spend. numbers audit catches numbers missing a messaging profile or 10DLC campaign assignment before texts fail silently.

## When to Use This CLI

Use for Telnyx SMS/MMS operations from owned numbers: reading conversation history, auditing number setup, tracking spend and failures, and sending texts safely from an agent.

## Anti-triggers

Do not use this CLI for:
- Building real-time call flows (use webhooks and the Telnyx SDK)
- Managing Telnyx AI assistants, storage, or IoT SIMs (out of scope)

## Unique Capabilities

### Synced message history
- **`conversation`** — See the delivery timeline with one guest or vendor number: direction, status, parts and cost (Telnyx records carry no message bodies).

  _Check when you last texted someone and whether it was delivered before sending anything new._

  ```bash
  telnyx-pp-cli conversation +12075551234 --since 30d --agent
  ```
- **`spend`** — Total messaging cost by number, profile, direction, or day, with balance runway.

  _Answer what texting cost this month without the portal._

  ```bash
  telnyx-pp-cli spend --since 30d --group-by number --agent
  ```
- **`failures`** — Failed and undelivered texts grouped by error code and recipient.

  _Find guests who never got their message._

  ```bash
  telnyx-pp-cli failures --since 7d --agent
  ```
- **`inbox unanswered`** — Contacts whose latest text was inbound with no reply yet.

  _Find who is still waiting on a reply._

  ```bash
  telnyx-pp-cli inbox unanswered --since 48h --agent
  ```

### Number health
- **`numbers audit`** — Find owned numbers that cannot text: missing messaging profile or missing 10DLC campaign assignment.

  _Run before blaming carriers for silent send failures._

  ```bash
  telnyx-pp-cli numbers audit --agent
  ```

### Agent-native plumbing
- **`text`** — Send a text after checking the sender is wired correctly and showing recent history; requires --yes.

  _Agents should send through this, not the raw endpoint._

  ```bash
  telnyx-pp-cli text --from +12075550100 --to +12075551234 --message 'Your door code is ready' --dry-run
  ```

## Command Reference

**10dlc** — Manage 10dlc

- `telnyx-pp-cli 10dlc accept-campaign` — Manually accept a campaign shared with Telnyx
- `telnyx-pp-cli 10dlc appeal-campaign` — Submits an appeal for rejected native campaigns in TELNYX_FAILED or MNO_REJECTED status.
- `telnyx-pp-cli 10dlc create-brand-post` — This endpoint is used to create a new brand.
- `telnyx-pp-cli 10dlc create-phone-number-campaign` — Assigns a phone number to a 10DLC campaign.
- `telnyx-pp-cli 10dlc deactivate-campaign` — Terminate a campaign. Note that once deactivated, a campaign cannot be restored.
- `telnyx-pp-cli 10dlc delete-brand` — Delete Brand. This endpoint is used to delete a brand.
- `telnyx-pp-cli 10dlc delete-phone-number-campaign` — This endpoint allows you to remove a campaign assignment from the supplied `phoneNumber`.
- `telnyx-pp-cli 10dlc get-all-phone-number-campaigns` — Returns phone-number-to-campaign assignments for the authenticated account.
- `telnyx-pp-cli 10dlc get-assignment-task-status` — Check the status of the task associated with assigning all phone numbers on a messaging profile to a campaign by
- `telnyx-pp-cli 10dlc get-brand` — Returns the details of a 10DLC brand by its brandId, including the count of campaigns associated with the brand.
- `telnyx-pp-cli 10dlc get-brand-feedback-by-id` — Get feedback about a brand by ID. This endpoint can be used after creating or revetting a brand. Possible values for `.
- `telnyx-pp-cli 10dlc get-brand-feedback-by-id-brand` — Get feedback about a brand by ID. This endpoint can be used after creating or revetting a brand. Possible values for `.
- `telnyx-pp-cli 10dlc get-brand-sms-otp-status` — Query the status of an SMS OTP (One-Time Password) for Sole Proprietor brand verification.
- `telnyx-pp-cli 10dlc get-brand-sms-otp-status-by-brand-id` — Query the status of an SMS OTP (One-Time Password) for Sole Proprietor brand verification using the Brand ID.
- `telnyx-pp-cli 10dlc get-brands` — This endpoint is used to list all brands associated with your organization.
- `telnyx-pp-cli 10dlc get-campaign` — Retrieve campaign details by `campaignId`.
- `telnyx-pp-cli 10dlc get-campaign-cost` — Retrieve the campaign cost associated with a given 10DLC use case, including any carrier fees.
- `telnyx-pp-cli 10dlc get-campaign-cost-campaign` — Returns the upfront and monthly cost associated with the selected 10DLC campaign use case.
- `telnyx-pp-cli 10dlc get-campaign-mno-metadata` — Get the campaign metadata for each MNO it was submitted to.
- `telnyx-pp-cli 10dlc get-campaign-operation-status` — Retrieve campaign's operation status at MNO level.
- `telnyx-pp-cli 10dlc get-campaign-osr-attributes` — Retrieve the OSR attributes assigned to a 10DLC campaign.
- `telnyx-pp-cli 10dlc get-campaign-osr-attributes-campaign` — Returns the optional shared-responsibility attributes recorded for the campaign.
- `telnyx-pp-cli 10dlc get-campaign-sharing-status` — Returns whether the campaign is configured for partner sharing and the current sharing state.
- `telnyx-pp-cli 10dlc get-campaigns` — Retrieve a list of campaigns associated with a supplied `brandId`.
- `telnyx-pp-cli 10dlc get-enum-endpoint` — Returns the accepted values for the selected 10DLC enumeration endpoint.
- `telnyx-pp-cli 10dlc get-partner-campaign-sharing-status` — Returns the partner-sharing state for the specified campaign.
- `telnyx-pp-cli 10dlc get-partner-campaigns-shared-by-user` — Get all partner campaigns you have shared to Telnyx in a paginated fashion This endpoint is currently limited to only
- `telnyx-pp-cli 10dlc get-phone-number-status` — Check the status of the individual phone number/campaign assignments associated with the supplied `taskId`.
- `telnyx-pp-cli 10dlc get-shared-campaign` — Retrieve campaign details by `campaignId`.
- `telnyx-pp-cli 10dlc get-shared-campaigns` — Retrieve all partner campaigns you have shared to Telnyx in a paginated fashion.
- `telnyx-pp-cli 10dlc get-single-phone-number-campaign` — Retrieve an individual phone number/campaign assignment by `phoneNumber`.
- `telnyx-pp-cli 10dlc get-usecase-qualification` — This endpoint allows you to see whether or not the supplied brand is suitable for your desired campaign use case.
- `telnyx-pp-cli 10dlc list-external-vettings` — Get list of valid external vetting record for a given brand
- `telnyx-pp-cli 10dlc post-assign-messaging-profile-to-campaign` — This endpoint allows you to link all phone numbers associated with a Messaging Profile to a campaign.
- `telnyx-pp-cli 10dlc post-campaign` — Before creating a campaign, use the [Qualify By Usecase endpoint](https://developers.telnyx.
- `telnyx-pp-cli 10dlc post-order-external-vetting` — Order new external vetting for a brand.
- `telnyx-pp-cli 10dlc put-external-vetting-record` — This operation can be used to import an external vetting record from a TCR-approved vetting provider.
- `telnyx-pp-cli 10dlc put-phone-number-campaign` — Replaces the 10DLC campaign assignment for the specified phone number.
- `telnyx-pp-cli 10dlc resend-brand2fa-email` — Requests a new two-factor authentication email for the specified 10DLC brand.
- `telnyx-pp-cli 10dlc revet-brand` — This operation allows you to revet the brand.
- `telnyx-pp-cli 10dlc trigger-brand-sms-otp` — Trigger or re-trigger an SMS OTP (One-Time Password) for Sole Proprietor brand verification.
- `telnyx-pp-cli 10dlc update-brand` — Update a brand's attributes by `brandId`.
- `telnyx-pp-cli 10dlc update-campaign` — Update a campaign's properties by `campaignId`. **Please note:** only sample messages are editable.
- `telnyx-pp-cli 10dlc update-shared-campaign` — Update campaign details by `campaignId`. **Please note:** Only webhook urls are editable.
- `telnyx-pp-cli 10dlc verify-brand-sms-otp` — Verify the SMS OTP (One-Time Password) for Sole Proprietor brand verification. **Verification Flow:** 1.

**available-phone-numbers** — Manage available phone numbers

- `telnyx-pp-cli available-phone-numbers` — Searches the Telnyx inventory for available phone numbers.

**balance** — Manage balance

- `telnyx-pp-cli balance` — Retrieve the current balance, available credit, and credit limit for your account.

**call-control-applications** — Call Control applications operations

- `telnyx-pp-cli call-control-applications create` — Creates a call control application
- `telnyx-pp-cli call-control-applications delete` — Permanently deletes the specified call control application and its webhook configuration.
- `telnyx-pp-cli call-control-applications list` — Return a list of call control applications.
- `telnyx-pp-cli call-control-applications retrieve` — Retrieves the details of an existing call control application.
- `telnyx-pp-cli call-control-applications update` — Updates settings of an existing call control application.

**calls** — Manage calls

- `telnyx-pp-cli calls dial` — Dial a number or SIP URI from a given connection.
- `telnyx-pp-cli calls retrieve-status` — Returns the status of a call (data is available 10 minutes after call ended).

**connections** — Connections operations

- `telnyx-pp-cli connections count` — Returns the number of connections associated with the authenticated user, grouped by connection type
- `telnyx-pp-cli connections list` — Returns a list of your connections irrespective of type.
- `telnyx-pp-cli connections retrieve` — Retrieves the high-level details of an existing connection.

**detail-records** — Detail Records operations

- `telnyx-pp-cli detail-records` — Search for any detail record across the Telnyx Platform

**faxes** — Manage faxes

- `telnyx-pp-cli faxes delete-fax` — Delete a fax resource from your account.
- `telnyx-pp-cli faxes list` — Retrieve a paginated list of faxes sent or received on your account.
- `telnyx-pp-cli faxes send-fax` — Send a fax. Files have size limits and page count limit validations.
- `telnyx-pp-cli faxes view-fax` — Returns the details of a single fax, including its current status.

**media** — Manage media

- `telnyx-pp-cli media create-storage` — Upload media file to Telnyx so it can be used with other Telnyx services
- `telnyx-pp-cli media delete-storage` — Permanently deletes the specified media file from storage.
- `telnyx-pp-cli media get-storage` — Returns the information about a stored media file.
- `telnyx-pp-cli media list-storage` — Returns a list of the media files stored on your account, with support for filtering.
- `telnyx-pp-cli media update-storage` — Updates the specified stored media file and returns the updated resource.

**messages** — Messages

- `telnyx-pp-cli messages cancel` — Cancel a scheduled message that has not yet been sent.
- `telnyx-pp-cli messages create-group-mms` — Queues an MMS addressed to multiple recipients as a group conversation.
- `telnyx-pp-cli messages create-long-code` — Queues an outbound SMS or MMS using a long-code sender.
- `telnyx-pp-cli messages create-number-pool` — Queues an outbound message using a number pool.
- `telnyx-pp-cli messages create-short-code` — Queues an outbound SMS or MMS using a short-code sender.
- `telnyx-pp-cli messages generate-rcsdeeplink` — Generate a deeplink URL that can be used to start an RCS conversation with a specific agent.
- `telnyx-pp-cli messages generate-rcsdeeplink-rcs` — Generate a deeplink URL that can be used to start an RCS conversation with a specific agent.
- `telnyx-pp-cli messages get` — Note: This API endpoint can only retrieve messages that are no older than 10 days since their creation.
- `telnyx-pp-cli messages get-group-mms` — Retrieve all messages in a group MMS conversation by the group message ID.
- `telnyx-pp-cli messages schedule` — Schedule a message with a Phone Number, Alphanumeric Sender ID, Short Code or Number Pool.
- `telnyx-pp-cli messages send` — Send a message with a Phone Number, Alphanumeric Sender ID, Short Code or Number Pool.
- `telnyx-pp-cli messages send-alphanumeric-sender-id` — Send an SMS message using an alphanumeric sender ID. This is SMS only.
- `telnyx-pp-cli messages send-alphanumeric-sender-id-alphanumeric` — Send an SMS message using an alphanumeric sender ID. This is SMS only.
- `telnyx-pp-cli messages send-whatsapp` — Sends a WhatsApp message using a Telnyx WhatsApp-enabled number.

**messaging-profiles** — Manage messaging profiles

- `telnyx-pp-cli messaging-profiles create` — Creates a messaging profile that controls outbound sender selection, webhook delivery
- `telnyx-pp-cli messaging-profiles delete` — Deletes the specified messaging profile and returns the profile's final configuration.
- `telnyx-pp-cli messaging-profiles list` — Lists messaging profiles owned by the authenticated account.
- `telnyx-pp-cli messaging-profiles retrieve` — Returns the complete configuration of the specified messaging profile, including webhook and sender-selection settings.
- `telnyx-pp-cli messaging-profiles update` — Updates the supplied settings on the specified messaging profile. Settings omitted from the request remain unchanged.

**number-orders** — Manage number orders

- `telnyx-pp-cli number-orders create` — Creates an order to purchase the specified phone numbers and returns the created order.
- `telnyx-pp-cli number-orders list` — Returns a paginated list of your phone number orders, with support for filtering.
- `telnyx-pp-cli number-orders retrieve` — Returns the details of an existing phone number order, including its status and the numbers included.
- `telnyx-pp-cli number-orders update` — Updates an existing phone number order, for example to satisfy regulatory requirements attached to the order

**number-reservations** — Manage number reservations

- `telnyx-pp-cli number-reservations create` — Creates a Phone Number Reservation for multiple numbers.
- `telnyx-pp-cli number-reservations list` — Gets a paginated list of phone number reservations.
- `telnyx-pp-cli number-reservations retrieve` — Returns the details of a single phone number reservation, including its status and the reserved numbers.

**outbound-voice-profiles** — Outbound voice profiles operations

- `telnyx-pp-cli outbound-voice-profiles create-voice-profile` — Creates a new outbound voice profile defining calling permissions, destinations, and limits for outbound calls
- `telnyx-pp-cli outbound-voice-profiles delete` — Deletes an existing outbound voice profile.
- `telnyx-pp-cli outbound-voice-profiles get` — Retrieves the details of an existing outbound voice profile.
- `telnyx-pp-cli outbound-voice-profiles list` — Get all outbound voice profiles belonging to the user that match the given filters.
- `telnyx-pp-cli outbound-voice-profiles update` — Updates an existing outbound voice profile.

**phone-numbers** — Associate phone numbers with a verified DIR so calls from those numbers carry the DIR's display identity.

- `telnyx-pp-cli phone-numbers create-csv-download` — Starts generation of a CSV export for phone numbers matching the supplied filters.
- `telnyx-pp-cli phone-numbers create-delete-job` — Creates a new background job to delete a batch of numbers. At most one thousand numbers can be updated per API call.
- `telnyx-pp-cli phone-numbers create-job-update-emergency-settings` — Creates a background job to update the emergency settings of a collection of phone numbers.
- `telnyx-pp-cli phone-numbers create-update-job` — Creates a new background job to update a batch of numbers. At most one thousand numbers can be updated per API call.
- `telnyx-pp-cli phone-numbers delete` — Deletes the specified phone number from the account.
- `telnyx-pp-cli phone-numbers get-csv-download` — Returns the current status and download details for the CSV export identified by `id`.
- `telnyx-pp-cli phone-numbers list` — Returns phone numbers associated with the account.
- `telnyx-pp-cli phone-numbers list-csv-downloads` — Returns CSV export jobs created for account phone numbers
- `telnyx-pp-cli phone-numbers list-jobs` — Returns background jobs that operate on phone numbers.
- `telnyx-pp-cli phone-numbers list-regulatory-requirements` — Retrieve the regulatory requirements that apply to a list of phone numbers.
- `telnyx-pp-cli phone-numbers list-with-messaging-settings` — Returns phone numbers with their current messaging product and messaging-profile assignments.
- `telnyx-pp-cli phone-numbers list-with-voice-settings` — Returns account phone numbers together with their voice settings.
- `telnyx-pp-cli phone-numbers retrieve` — Returns the detailed configuration and current state of the phone number identified by `id`.
- `telnyx-pp-cli phone-numbers retrieve-job` — Returns the status and details of the phone-number background job identified by `id`.
- `telnyx-pp-cli phone-numbers slim-list` — List phone numbers
- `telnyx-pp-cli phone-numbers update` — Updates the configurable settings of the specified phone number.
- `telnyx-pp-cli phone-numbers verify-ownership` — Verifies ownership of the provided phone numbers and returns a mapping of numbers to their IDs

**recordings** — Manage recordings

- `telnyx-pp-cli recordings delete` — Permanently deletes the specified call recording and returns the deleted recording resource.
- `telnyx-pp-cli recordings delete-actions` — Permanently deletes a list of call recordings.
- `telnyx-pp-cli recordings get` — Returns a paginated list of your call recordings, with support for filtering to locate specific recordings.
- `telnyx-pp-cli recordings get-recordingid` — Retrieves the details of an existing call recording.

**short-codes** — Short codes

- `telnyx-pp-cli short-codes list` — Returns short codes owned by the authenticated account.
- `telnyx-pp-cli short-codes retrieve` — Returns the messaging configuration and assignment details for the specified short code.
- `telnyx-pp-cli short-codes update` — Update the settings for a specific short code.

**texml-applications** — Manage texml applications

- `telnyx-pp-cli texml-applications create` — Creates a TeXML application, which defines the voice URLs and settings used to serve TeXML instructions for calls
- `telnyx-pp-cli texml-applications delete` — Permanently deletes the specified TeXML application from your account.
- `telnyx-pp-cli texml-applications find` — Returns a list of your TeXML Applications.
- `telnyx-pp-cli texml-applications get` — Retrieves the details of an existing TeXML Application.
- `telnyx-pp-cli texml-applications update` — Updates settings of an existing TeXML Application.

**verifications** — Manage verifications

- `telnyx-pp-cli verifications create-call` — Starts a verification for the specified phone number and delivers its code in a voice call using the selected Verify
- `telnyx-pp-cli verifications create-flashcall` — Starts a verification for the specified phone number and places a brief call with the code embedded in the caller ID.
- `telnyx-pp-cli verifications create-sms` — Starts a verification for the specified phone number and sends its code by SMS using the selected Verify profile.
- `telnyx-pp-cli verifications create-whatsapp` — Starts a verification for the specified phone number and sends its code over WhatsApp using the selected Verify profile.
- `telnyx-pp-cli verifications list` — Returns a paginated list of verifications associated with the specified phone number.
- `telnyx-pp-cli verifications retrieve` — Returns the verification identified by ID, including its channel, phone number, Verify profile, timeout
- `telnyx-pp-cli verifications verify-code-by-phone-number` — Submits a verification code for the specified phone number and Verify profile.

**verify-profiles** — Manage verify profiles

- `telnyx-pp-cli verify-profiles create` — Creates a new Verify profile to associate verifications with.
- `telnyx-pp-cli verify-profiles create-message-template` — Create a new Verify profile message template.
- `telnyx-pp-cli verify-profiles delete-profile` — Deletes the specified Verify profile and returns the deleted profile record.
- `telnyx-pp-cli verify-profiles get` — Returns the details of a single Verify profile by its ID, including its verification channel configuration.
- `telnyx-pp-cli verify-profiles list-profile-message-templates` — List all Verify profile message templates.
- `telnyx-pp-cli verify-profiles list-profiles` — Gets a paginated list of Verify profiles.
- `telnyx-pp-cli verify-profiles update` — Updates the specified Verify profile's name, webhook destinations, language, daily spend limits
- `telnyx-pp-cli verify-profiles update-message-template` — Update an existing Verify profile message template.

**webhook-deliveries** — Manage webhook deliveries

- `telnyx-pp-cli webhook-deliveries get` — Lists webhook_deliveries for the authenticated user
- `telnyx-pp-cli webhook-deliveries get-webhook-delivery` — Provides webhook_delivery debug data, such as timestamps, delivery status and attempts.


**conversation**


- `telnyx-pp-cli conversation` — See the delivery timeline with one guest or vendor number: direction, status, parts and cost (Telnyx records carry no message bodies).


**failures**


- `telnyx-pp-cli failures` — Failed and undelivered texts grouped by error code and recipient.


**inbox**


- `telnyx-pp-cli inbox unanswered` — Contacts whose latest text was inbound with no reply yet.


**numbers**


- `telnyx-pp-cli numbers audit` — Find owned numbers that cannot text: missing messaging profile or missing 10DLC campaign assignment.


**spend**


- `telnyx-pp-cli spend` — Total messaging cost by number, profile, direction, or day, with balance runway.


**text**


- `telnyx-pp-cli text` — Send a text after checking the sender is wired correctly and showing recent history; requires --yes.




### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
telnyx-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

## Recipes

### Check a thread before replying

```bash
telnyx-pp-cli conversation +12075551234 --since 14d --agent --select messages.direction,messages.our_number,messages.status,messages.created_at
```

Narrow the delivery timeline to the fields an agent needs (metadata only, no message bodies).

### Monthly cost by number

```bash
telnyx-pp-cli spend --since 30d --group-by number
```

Cost per owned number from synced history.

### Preview a send

```bash
telnyx-pp-cli text --from +12075550100 --to +12075551234 --message 'Checking in' --dry-run
```

Runs preflight checks without sending.

## Auth Setup

Set TELNYX_API_KEY to a Telnyx v2 API key (starts with KEY); TELNYX_API_TOKEN is accepted as an alias. Or run `telnyx-pp-cli auth set-token`, and check with `telnyx-pp-cli auth status`.

Run `telnyx-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color`.

Global format flags share one contract on promoted, novel, sync, and `--deliver` paths:

- `--json` — one JSON document on stdout (sync progress events go to stderr)
- `--compact` — keep identity/status/timestamp fields; does not change the document vs stream shape
- `--csv` / `--plain` — tabular rows (collection envelopes unwrap to the row array)
- `--quiet` — one identity value per row, no envelope

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  telnyx-pp-cli available-phone-numbers --agent --select best_effort,cost_information,features
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit confirmation** — `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and use `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `TELNYX_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `TELNYX_CONFIG_DIR`, `TELNYX_DATA_DIR`, `TELNYX_STATE_DIR`, `TELNYX_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `TELNYX_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `telnyx-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "telnyx": {
        "command": "telnyx-pp-mcp",
        "env": {
          "TELNYX_HOME": "/srv/telnyx"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `TELNYX_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `TELNYX_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, pass the question as an argv or MCP tool argument to `recall --agent`. Do not interpolate user-controlled text into a shell command line.

Quoted `recall "<question>"` breaks on an apostrophe, which is ordinary English. A quoted heredoc breaks when a body line equals the delimiter, and that delimiter is published in these docs. Write the question with a non-shell file-writing tool, then read it back as data:

```bash
# Write the question verbatim with your file-writing tool (no shell involved).
# Command substitution on a file only ever yields data — the shell never
# parses the file's bytes as syntax.
QUERY=$(cat /path/to/question.txt)
telnyx-pp-cli recall "$QUERY" --agent
```

Prefer MCP: pass the question as the tool's query argument. `"$QUERY"` after a file read is argv-safe; putting the question itself in the command text is not.

The response envelope:

```json
{
  "query": "...",
  "normalized": "<normalized form>",
  "query_entities": ["..."],
  "found": true | false,
  "match_score": 0.0,
  "results": [
    { "resource_id": "...", "resource_type": "...", "venue": "...",
      "confidence": 2, "entity_match": "exact|partial|unknown",
      "source": "taught|preseed|pattern", "warnings": ["..."] }
  ],
  "mismatches": [ /* only when --debug-mismatches */ ],
  "warnings": [ /* top-level */ ],
  "candidates": [
    { "id": 12, "class": "flag_alias | playbook_candidate",
      "summary": "...", "sightings": 3, "last_seen": "...",
      "rationale": "...",
      "next_action": ["<trial command>", "telnyx-pp-cli learnings confirm 12"] }
  ],
  "playbook": {
    "query_family": "...",
    "playbook": {
      "steps": [ { "cmd": "<command with {slot} substitution>", "purpose": "..." } ],
      "entity_slots": ["$ENTITY"],
      "expected_tool_calls": 3
    },
    "slots_resolved": { "$ENTITY": { "token": "<live token>", "canonical": "<canonical>" } },
    "notes": "<workarounds + gotchas for this query family>"
  },
  "notes": "<duplicate surface for non-playbook callers>"
}
```

Empty-store short-circuit: if the store has no learnings, playbooks, or candidates yet (recall finds nothing and `learnings list` and `learnings candidates` are both empty), skip recall for the rest of this session instead of taxing every query; resume recall-first once something has been taught.

### Step 2: decision tree

Read `candidates`, `playbook`, `notes`, `results[0]`, and warnings in that order:

```
if Candidates present (warnings include "candidates_present"):
    -> candidates are try-then-confirm, never facts. Follow each candidate's
       two-step next_action verbatim: run the trial command first, then run
       `learnings confirm <id>` only after the trial verified the behavior.
       Reject a wrong candidate with `learnings reject <id>`.
    -> NEVER re-teach something recall surfaced as a candidate; confirm or
       reject that candidate instead of teaching a duplicate.
    -> candidates ride alongside playbooks and resource hits, not instead of
       them; continue with the branches below after acting on them.

if Playbook present:
    -> READ Playbook.notes verbatim FIRST (workarounds + gotchas the CLI surface doesn't expose)
    -> replay Playbook.steps in order, substituting Playbook.slots_resolved entries
       for the entity slot tokens. If a step's slot is unresolved, fall back to
       discovery for that step only.
    -> the Playbook's expected_tool_calls is a budget; if you find yourself running
       materially more, record the divergence via `telnyx-pp-cli playbook amend`
       at end-of-session.

elif Notes present (no Playbook):
    -> read Notes verbatim before any discovery step; they carry known gotchas
       for this query family even when no structured choreography exists yet.

elif Found AND Results[0].EntityMatch == "exact" AND Results[0].Confidence >= 2:
    -> skip discovery; fetch live data for Results[*].ResourceID in parallel

elif Found AND Results[0].EntityMatch == "partial":
    -> candidate hint, NOT a hit; read the resource title to validate before trusting

elif (any row in Mismatches[] when --debug-mismatches was passed):
    -> treat as cold start; the stored learning is for a different entity
       (different canonical resolved from query_entities)

else:  // Found == false, no playbook, no notes
    -> cold start; run discovery normally; teach the answer afterward (Step 4).
       If the family has no playbook yet, that teach auto-synthesizes a
       playbook candidate from this session's journal - you do not need to
       record one by hand.
```

Playbook and Notes are orthogonal to the per-resource path. A recall response can carry both a Playbook AND a `Results[]` hit - use both: the Playbook tells you which choreography to run; the resource hits short-circuit specific steps. Default to skipping `mismatches`; pass `--debug-mismatches` only when investigating cold-start surprises.

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `telnyx-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `telnyx-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately. Pass the query the same way as recall — argv/MCP, or file-then-`$QUERY`. Do not splice the question into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
telnyx-pp-cli teach --query "$QUERY" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
QUERY=$(cat /path/to/question.txt)
telnyx-pp-cli teach \
  --query "$QUERY" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
QUERY=$(cat /path/to/question.txt)
telnyx-pp-cli teach-playbook \
  --query "$QUERY" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`. Pass the query and note as argv/MCP arguments, or write each with a non-shell file tool and read them back (`QUERY=$(cat ...)`, `NOTE=$(cat ...)`). Do not interpolate either string into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
NOTE=$(cat /path/to/note.txt)
telnyx-pp-cli playbook amend \
  --query "$QUERY" \
  --add-note "$NOTE"
# (append shell `&` to background it)
```

What counts as worth amending: a behavior you OBSERVED this session that future-you would benefit from knowing. Examples worth amending:

- A workaround for a CLI surface that silently drops or misorders a flag.
- An undocumented endpoint shape (response wrapped in `{meta, results}`, payload nested two levels deeper than the docs claim).
- Observed schema drift (a field renamed, an index that shifted between seasons, a category label that the API now returns lower-cased).

What does NOT belong in notes:

- The year-specific or entity-specific answer to the user's question. That's the response, not a learning.
- Per-team / per-athlete / per-row data the playbook already retrieves at runtime.
- Statements that paraphrase what the existing notes already say.

The amend command appends to the family's existing notes with a timestamped marker (`[amend YYYY-MM-DDTHH:MMZ]: <text>`). Multiple amends accumulate; the audit trail is visible. If no playbook exists yet for the family, amend creates a notes-only one (so cold-start corrections still land).

#### PII discipline for amend notes

`playbook amend` notes are designed to potentially flow upstream as shared knowledge in future versions of the Printing Press. Keep them clean of user-identifying content so the upstream-contribution path stays open without retroactive scrubbing:

- **Do NOT embed** paths to user filesystems, personal API keys or tokens, user email addresses, user GitHub handles, or specific query histories tied to a single user.
- **Acceptable**: endpoint shapes, undocumented field names, API gotchas, observed schema drift, workarounds for CLI surfaces, generalizable pagination or retry tactics.

If a correction is only meaningful with user-specific context, it belongs in a personal note, not in the playbook amend.

### Measuring the loop

`telnyx-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `TELNYX_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
telnyx-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
telnyx-pp-cli feedback --stdin < notes.txt
telnyx-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `TELNYX_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TELNYX_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename). Binary-response commands write decoded payload bytes (not the base64 JSON envelope) and print a small JSON receipt on stdout; `--json`/`--csv` do not refuse when this sink is set. |
| `webhook:<url>` | POST the output body to the URL (`application/json`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
telnyx-pp-cli profile save briefing --json
telnyx-pp-cli --profile briefing available-phone-numbers
telnyx-pp-cli profile list --json
telnyx-pp-cli profile show briefing
telnyx-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Async Jobs

For endpoints that submit long-running work, the generator detects the submit-then-poll pattern (a `job_id`/`task_id`/`operation_id` field in the response plus a sibling status endpoint) and wires up three extra flags on the submitting command:

| Flag | Purpose |
|------|---------|
| `--wait` | Block until the job reaches a terminal status instead of returning the job ID immediately |
| `--wait-timeout` | Maximum wait duration (default 10m, 0 means no timeout) |
| `--wait-interval` | Initial poll interval (default 2s; grows with exponential backoff up to 30s) |

Use async submission without `--wait` when you want to fire-and-forget; use `--wait` when you want one command to return the finished artifact.

If `--wait` times out or polling fails after the job was accepted, the command exits `8` and prints the job ID plus a recovery command that fetches the result. The job may still be running and may already be billed, so run the recovery command instead of resubmitting.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 6 | Partial failure |
| 7 | Rate limited (wait and retry) |
| 8 | Async job submitted but not finished (run the printed recovery command; do not resubmit) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `telnyx-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/telnyx/cmd/telnyx-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add telnyx-pp-mcp -- telnyx-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which telnyx-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   telnyx-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `telnyx-pp-cli <command> --help`.
