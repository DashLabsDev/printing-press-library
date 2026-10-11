# Telnyx CLI

**Telnyx messaging, numbers, and voice endpoints, plus a synced message-history mirror, number setup audits, and spend rollups.**

Sync your Telnyx messaging detail records into a local database, then see any number's delivery timeline with conversation, who is waiting with inbox unanswered, and costs with spend. numbers audit catches numbers missing a messaging profile or 10DLC campaign assignment before texts fail silently.

## Install

The recommended path installs both the `telnyx-pp-cli` binary and the `pp-telnyx` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install telnyx
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install telnyx --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install telnyx --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install telnyx --agent claude-code
npx -y @mvanhorn/printing-press-library install telnyx --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.9 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/telnyx/cmd/telnyx-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/telnyx-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install telnyx --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-telnyx --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-telnyx --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install telnyx --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/telnyx-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `TELNYX_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/telnyx/cmd/telnyx-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "telnyx": {
      "command": "telnyx-pp-mcp",
      "env": {
        "TELNYX_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Set TELNYX_API_KEY to a Telnyx v2 API key (starts with KEY); TELNYX_API_TOKEN is accepted as an alias. Or run `telnyx-pp-cli auth set-token`, and check with `telnyx-pp-cli auth status`.

## Quick Start

```bash
# Confirm the CLI is installed and config resolves
telnyx-pp-cli doctor --dry-run

# Pull numbers, profiles, and recent history into the local store
telnyx-pp-cli sync --since 30d

# Check every number can actually text
telnyx-pp-cli numbers audit

# See who is waiting on a reply
telnyx-pp-cli inbox unanswered --since 48h

```

## Unique Features

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

## Usage

Run `telnyx-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `TELNYX_CONFIG_DIR`, `TELNYX_DATA_DIR`, `TELNYX_STATE_DIR`, or `TELNYX_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `TELNYX_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export TELNYX_HOME=/srv/telnyx
telnyx-pp-cli doctor
```

Under `TELNYX_HOME=/srv/telnyx`, the four dirs resolve to `/srv/telnyx/config`, `/srv/telnyx/data`, `/srv/telnyx/state`, and `/srv/telnyx/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

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

Precedence matters in fleets: an ambient per-kind variable such as `TELNYX_DATA_DIR` overrides an explicit `--home` for that kind. Use `TELNYX_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `TELNYX_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `telnyx-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### 10dlc

Manage 10dlc

- **`telnyx-pp-cli 10dlc accept-campaign`** - Manually accept a campaign shared with Telnyx
- **`telnyx-pp-cli 10dlc appeal-campaign`** - Submits an appeal for rejected native campaigns in TELNYX_FAILED or MNO_REJECTED status. The appeal is recorded for manual compliance team review and the campaign status is reset to TCR_ACCEPTED. Note: Appeal forwarding is handled manually to allow proper review before incurring upstream charges.
- **`telnyx-pp-cli 10dlc create-brand-post`** - This endpoint is used to create a new brand. A brand is an entity created by The Campaign Registry (TCR) that represents an organization or a company. It is this entity that TCR created campaigns will be associated with. Each brand creation will entail an upfront, non-refundable $4 expense.
- **`telnyx-pp-cli 10dlc create-phone-number-campaign`** - Assigns a phone number to a 10DLC campaign. The assignment controls which registered campaign is used for traffic from that number.
- **`telnyx-pp-cli 10dlc deactivate-campaign`** - Terminate a campaign. Note that once deactivated, a campaign cannot be restored.
- **`telnyx-pp-cli 10dlc delete-brand`** - Delete Brand. This endpoint is used to delete a brand. Note the brand cannot be deleted if it contains one or more active campaigns, the campaigns need to be inactive and at least 3 months old due to billing purposes.
- **`telnyx-pp-cli 10dlc delete-phone-number-campaign`** - This endpoint allows you to remove a campaign assignment from the supplied `phoneNumber`.
- **`telnyx-pp-cli 10dlc get-all-phone-number-campaigns`** - Returns phone-number-to-campaign assignments for the authenticated account. Apply the documented filters and pagination parameters to narrow the result set.
- **`telnyx-pp-cli 10dlc get-assignment-task-status`** - Check the status of the task associated with assigning all phone numbers on a messaging profile to a campaign by `taskId`.
- **`telnyx-pp-cli 10dlc get-brand`** - Returns the details of a 10DLC brand by its brandId, including the count of campaigns associated with the brand.
- **`telnyx-pp-cli 10dlc get-brand-feedback-by-id`** - Get feedback about a brand by ID. This endpoint can be used after creating or revetting
a brand.

Possible values for `.category[].id`:

* `TAX_ID` - Data mismatch related to tax id and its associated properties.
* `STOCK_SYMBOL` - Non public entity registered as a public for profit entity or
  the stock information mismatch.
* `GOVERNMENT_ENTITY` - Non government entity registered as a government entity.
  Must be a U.S. government entity.
* `NONPROFIT` - Not a recognized non-profit entity. No IRS tax-exempt status
  found.
* `OTHERS` - Details of the data misrepresentation if any.
- **`telnyx-pp-cli 10dlc get-brand-feedback-by-id-brand`** - Get feedback about a brand by ID. This endpoint can be used after creating or revetting
a brand.

Possible values for `.category[].id`:

* `TAX_ID` - Data mismatch related to tax id and its associated properties.
* `STOCK_SYMBOL` - Non public entity registered as a public for profit entity or
  the stock information mismatch.
* `GOVERNMENT_ENTITY` - Non government entity registered as a government entity.
  Must be a U.S. government entity.
* `NONPROFIT` - Not a recognized non-profit entity. No IRS tax-exempt status
  found.
* `OTHERS` - Details of the data misrepresentation if any.
- **`telnyx-pp-cli 10dlc get-brand-sms-otp-status`** - Query the status of an SMS OTP (One-Time Password) for Sole Proprietor brand verification.

This endpoint allows you to check the delivery and verification status of an OTP sent during the Sole Proprietor brand verification process. You can query by either:

* `referenceId` - The reference ID returned when the OTP was initially triggered
* `brandId` - Query parameter for portal users to look up OTP status by Brand ID

The response includes delivery status, verification dates, and detailed delivery information.
- **`telnyx-pp-cli 10dlc get-brand-sms-otp-status-by-brand-id`** - Query the status of an SMS OTP (One-Time Password) for Sole Proprietor brand verification using the Brand ID.

This endpoint allows you to check the delivery and verification status of an OTP sent during the Sole Proprietor brand verification process by looking it up with the brand ID.

The response includes delivery status, verification dates, and detailed delivery information.

**Note:** This is an alternative to the `/10dlc/brand/smsOtp/{referenceId}` endpoint when you have the Brand ID but not the reference ID.
- **`telnyx-pp-cli 10dlc get-brands`** - This endpoint is used to list all brands associated with your organization.
- **`telnyx-pp-cli 10dlc get-campaign`** - Retrieve campaign details by `campaignId`.
- **`telnyx-pp-cli 10dlc get-campaign-cost`** - Retrieve the campaign cost associated with a given 10DLC use case, including any carrier fees.
- **`telnyx-pp-cli 10dlc get-campaign-cost-campaign`** - Returns the upfront and monthly cost associated with the selected 10DLC campaign use case.
- **`telnyx-pp-cli 10dlc get-campaign-mno-metadata`** - Get the campaign metadata for each MNO it was submitted to.
- **`telnyx-pp-cli 10dlc get-campaign-operation-status`** - Retrieve campaign's operation status at MNO level.
- **`telnyx-pp-cli 10dlc get-campaign-osr-attributes`** - Retrieve the OSR attributes assigned to a 10DLC campaign.
- **`telnyx-pp-cli 10dlc get-campaign-osr-attributes-campaign`** - Returns the optional shared-responsibility attributes recorded for the campaign. Use these values to inspect the campaign configuration submitted to the registry.
- **`telnyx-pp-cli 10dlc get-campaign-sharing-status`** - Returns whether the campaign is configured for partner sharing and the current sharing state.
- **`telnyx-pp-cli 10dlc get-campaigns`** - Retrieve a list of campaigns associated with a supplied `brandId`.
- **`telnyx-pp-cli 10dlc get-enum-endpoint`** - Returns the accepted values for the selected 10DLC enumeration endpoint. Use these values when constructing brand and campaign requests.
- **`telnyx-pp-cli 10dlc get-partner-campaign-sharing-status`** - Returns the partner-sharing state for the specified campaign.
- **`telnyx-pp-cli 10dlc get-partner-campaigns-shared-by-user`** - Get all partner campaigns you have shared to Telnyx in a paginated fashion

This endpoint is currently limited to only returning shared campaigns that Telnyx
has accepted. In other words, shared but pending campaigns are currently omitted
from the response from this endpoint.
- **`telnyx-pp-cli 10dlc get-phone-number-status`** - Check the status of the individual phone number/campaign assignments associated with the supplied `taskId`.
- **`telnyx-pp-cli 10dlc get-shared-campaign`** - Retrieve campaign details by `campaignId`.
- **`telnyx-pp-cli 10dlc get-shared-campaigns`** - Retrieve all partner campaigns you have shared to Telnyx in a paginated fashion.

This endpoint is currently limited to only returning shared campaigns that Telnyx has accepted. In other words, shared but pending campaigns are currently omitted from the response from this endpoint.
- **`telnyx-pp-cli 10dlc get-single-phone-number-campaign`** - Retrieve an individual phone number/campaign assignment by `phoneNumber`.
- **`telnyx-pp-cli 10dlc get-usecase-qualification`** - This endpoint allows you to see whether or not the supplied brand is suitable for your desired campaign use case.
- **`telnyx-pp-cli 10dlc list-external-vettings`** - Get list of valid external vetting record for a given brand
- **`telnyx-pp-cli 10dlc post-assign-messaging-profile-to-campaign`** - This endpoint allows you to link all phone numbers associated with a Messaging Profile to a campaign. **Please note:** if you want to assign phone numbers to a campaign that you did not create with Telnyx 10DLC services, this endpoint allows that provided that you've shared the campaign with Telnyx. In this case, only provide the parameter, `tcrCampaignId`, and not `campaignId`. In all other cases (where the campaign you're assigning was created with Telnyx 10DLC services), only provide `campaignId`, not `tcrCampaignId`.
- **`telnyx-pp-cli 10dlc post-campaign`** - Before creating a campaign, use the [Qualify By Usecase endpoint](https://developers.telnyx.com/api-reference/campaign/qualify-by-usecase) to ensure that the brand you want to assign a new campaign to is qualified for the desired use case of that campaign. **Please note:** After campaign creation, you'll only be able to edit the campaign's sample messages. Creating a campaign will entail an upfront, non-refundable three month's cost that will depend on the campaign's use case ([see 10DLC Costs section for details](https://developers.telnyx.com/api-reference/campaign/get-campaign-cost)).
- **`telnyx-pp-cli 10dlc post-order-external-vetting`** - Order new external vetting for a brand.

Duplicate orders for the same `evpId` and `vettingClass` return `400` with code `10012` if a successful vetting exists within the last 180 days, or one is currently being processed. Failed vettings can be retried immediately.
- **`telnyx-pp-cli 10dlc put-external-vetting-record`** - This operation can be used to import an external vetting record from a TCR-approved
vetting provider. If the vetting provider confirms validity of the record, it will be
saved with the brand and will be considered for future campaign qualification.
- **`telnyx-pp-cli 10dlc put-phone-number-campaign`** - Replaces the 10DLC campaign assignment for the specified phone number.
- **`telnyx-pp-cli 10dlc resend-brand2fa-email`** - Requests a new two-factor authentication email for the specified 10DLC brand. Complete verification through the link delivered to the brand contact before continuing registration.
- **`telnyx-pp-cli 10dlc revet-brand`** - This operation allows you to revet the brand. However, revetting is allowed once after the successful brand registration and thereafter limited to once every 3 months.
- **`telnyx-pp-cli 10dlc trigger-brand-sms-otp`** - Trigger or re-trigger an SMS OTP (One-Time Password) for Sole Proprietor brand verification.

**Important Notes:**

* Only allowed for Sole Proprietor (`SOLE_PROPRIETOR`) brands
* Triggers generation of a one-time password sent to the `mobilePhone` number in the brand's profile
* Campaigns cannot be created until OTP verification is complete
* US/CA numbers only for real OTPs; mock brands can use non-US/CA numbers for testing
* Returns a `referenceId` that can be used to check OTP status via the GET `/10dlc/brand/smsOtp/{referenceId}` endpoint

**Use Cases:**

* Initial OTP trigger after Sole Proprietor brand creation
* Re-triggering OTP if the user didn't receive or needs a new code
- **`telnyx-pp-cli 10dlc update-brand`** - Update a brand's attributes by `brandId`.
- **`telnyx-pp-cli 10dlc update-campaign`** - Update a campaign's properties by `campaignId`. **Please note:** only sample messages are editable.
- **`telnyx-pp-cli 10dlc update-shared-campaign`** - Update campaign details by `campaignId`. **Please note:** Only webhook urls are editable.
- **`telnyx-pp-cli 10dlc verify-brand-sms-otp`** - Verify the SMS OTP (One-Time Password) for Sole Proprietor brand verification.

**Verification Flow:**

1. User receives OTP via SMS after triggering
2. User submits the OTP pin through this endpoint
3. Upon successful verification:
   - A `BRAND_OTP_VERIFIED` webhook event is sent to the CSP
   - The brand's `identityStatus` changes to `VERIFIED`
   - Campaigns can now be created for this brand

**Error Handling:**

Provides proper error responses for:
* Invalid OTP pins
* Expired OTPs
* OTP verification failures

### available-phone-numbers

Manage available phone numbers

- **`telnyx-pp-cli available-phone-numbers`** - Searches the Telnyx inventory for available phone numbers. Filters support number patterns, location, number type, features, reservability, and other inventory constraints; the response includes matching numbers and search metadata.

### balance

Manage balance

- **`telnyx-pp-cli balance`** - Retrieve the current balance, available credit, and credit limit for your account.

### call-control-applications

Call Control applications operations

- **`telnyx-pp-cli call-control-applications create`** - Creates a call control application, which defines the webhook endpoints and settings used to control calls on associated connections.
- **`telnyx-pp-cli call-control-applications delete`** - Permanently deletes the specified call control application and its webhook configuration.
- **`telnyx-pp-cli call-control-applications list`** - Return a list of call control applications.
- **`telnyx-pp-cli call-control-applications retrieve`** - Retrieves the details of an existing call control application.
- **`telnyx-pp-cli call-control-applications update`** - Updates settings of an existing call control application.

### calls

Manage calls

- **`telnyx-pp-cli calls dial`** - Dial a number or SIP URI from a given connection. A successful response will include a `call_leg_id` which can be used to correlate the command with subsequent webhooks.

**Expected Webhooks:**

- `call.initiated`
- `call.answered` or `call.hangup`
- `call.hold` and `call.unhold` if the call is held/unheld
- `call.machine.detection.ended` if `answering_machine_detection` was requested
- `call.machine.greeting.ended` if `answering_machine_detection` was requested to detect the end of machine greeting
- `call.machine.premium.detection.ended` if `answering_machine_detection=premium` was requested
- `call.machine.premium.greeting.ended` if `answering_machine_detection=premium` was requested and a beep was detected
- `call.deepfake_detection.result` if `deepfake_detection` was enabled
- `call.deepfake_detection.error` if `deepfake_detection` was enabled and an error occurred
- `streaming.started`, `streaming.stopped` or `streaming.failed` if `stream_url` was set

When the `record` parameter is set to `record-from-answer`, the response will include a `recording_id` field.
- **`telnyx-pp-cli calls retrieve-status`** - Returns the status of a call (data is available 10 minutes after call ended).

### connections

Connections operations

- **`telnyx-pp-cli connections count`** - Returns the number of connections associated with the authenticated user, grouped by connection type, together with the connection limits that apply to the user. Forward-only connections are excluded from the counts.
- **`telnyx-pp-cli connections list`** - Returns a list of your connections irrespective of type.
- **`telnyx-pp-cli connections retrieve`** - Retrieves the high-level details of an existing connection. To retrieve specific authentication information, use the endpoint for the specific connection type.

### detail-records

Detail Records operations

- **`telnyx-pp-cli detail-records`** - Search for any detail record across the Telnyx Platform

### faxes

Manage faxes

- **`telnyx-pp-cli faxes delete-fax`** - Delete a fax resource from your account.
- **`telnyx-pp-cli faxes list`** - Retrieve a paginated list of faxes sent or received on your account.
- **`telnyx-pp-cli faxes send-fax`** - Send a fax. Files have size limits and page count limit validations. If a file is bigger than 50MB or has more than 350 pages it will fail with `file_size_limit_exceeded` and `page_count_limit_exceeded` respectively.

**Supported file formats:**

- PDF (`application/pdf`)
- TIFF (`application/tiff`, `image/tiff`)
- JPEG (`image/jpeg`)
- PNG (`image/png`)
- Microsoft Word `.doc` (`application/msword`)
- Microsoft Word `.docx` (`application/vnd.openxmlformats-officedocument.wordprocessingml.document`)
- Rich Text Format `.rtf` (`application/rtf`)
- Plain text `.txt` (`text/plain`)

**Expected Webhooks:**

- `fax.queued`
- `fax.media.processed`
- `fax.sending.started`
- `fax.delivered`
- `fax.failed`
- **`telnyx-pp-cli faxes view-fax`** - Returns the details of a single fax, including its current status.

### media

Manage media

- **`telnyx-pp-cli media create-storage`** - Upload media file to Telnyx so it can be used with other Telnyx services
- **`telnyx-pp-cli media delete-storage`** - Permanently deletes the specified media file from storage.
- **`telnyx-pp-cli media get-storage`** - Returns the information about a stored media file.
- **`telnyx-pp-cli media list-storage`** - Returns a list of the media files stored on your account, with support for filtering.
- **`telnyx-pp-cli media update-storage`** - Updates the specified stored media file and returns the updated resource.

### messages

Messages

- **`telnyx-pp-cli messages cancel`** - Cancel a scheduled message that has not yet been sent. Only messages with `status=scheduled` and `send_at` more than a minute from now can be cancelled.
- **`telnyx-pp-cli messages create-group-mms`** - Queues an MMS addressed to multiple recipients as a group conversation. Delivery events are reported asynchronously through messaging webhooks.
- **`telnyx-pp-cli messages create-long-code`** - Queues an outbound SMS or MMS using a long-code sender. Delivery progress and final disposition are reported asynchronously through messaging webhooks.
- **`telnyx-pp-cli messages create-number-pool`** - Queues an outbound message using a number pool. Telnyx selects an eligible sender from the pool according to its messaging profile configuration.
- **`telnyx-pp-cli messages create-short-code`** - Queues an outbound SMS or MMS using a short-code sender. Delivery progress and final disposition are reported asynchronously through messaging webhooks.
- **`telnyx-pp-cli messages generate-rcsdeeplink`** - Generate a deeplink URL that can be used to start an RCS conversation with a specific agent.
- **`telnyx-pp-cli messages generate-rcsdeeplink-rcs`** - Generate a deeplink URL that can be used to start an RCS conversation with a specific agent.
- **`telnyx-pp-cli messages get`** - Note: This API endpoint can only retrieve messages that are no older than 10 days since their creation. If you require messages older than this, please generate an [MDR report.](https://developers.telnyx.com/api-reference/mdr-usage-reports/create-mdr-usage-report)
- **`telnyx-pp-cli messages get-group-mms`** - Retrieve all messages in a group MMS conversation by the group message ID.
- **`telnyx-pp-cli messages schedule`** - Schedule a message with a Phone Number, Alphanumeric Sender ID, Short Code or Number Pool.

This endpoint allows you to schedule a message with any messaging resource.
Current messaging resources include: long-code, short-code, number-pool, and
alphanumeric-sender-id.
- **`telnyx-pp-cli messages send`** - Send a message with a Phone Number, Alphanumeric Sender ID, Short Code or Number Pool.

This endpoint allows you to send a message with any messaging resource.
Current messaging resources include: long-code, short-code, number-pool, and
alphanumeric-sender-id.
- **`telnyx-pp-cli messages send-alphanumeric-sender-id`** - Send an SMS message using an alphanumeric sender ID. This is SMS only.
- **`telnyx-pp-cli messages send-alphanumeric-sender-id-alphanumeric`** - Send an SMS message using an alphanumeric sender ID. This is SMS only.
- **`telnyx-pp-cli messages send-whatsapp`** - Sends a WhatsApp message using a Telnyx WhatsApp-enabled number. The message body, interactive elements, media, location, and reaction content are specified in the `whatsapp_message` field. Delivery progress and final disposition are reported asynchronously through messaging webhooks.

### messaging-profiles

Manage messaging profiles

- **`telnyx-pp-cli messaging-profiles create`** - Creates a messaging profile that controls outbound sender selection, webhook delivery, and inbound message handling for associated numbers and short codes.
- **`telnyx-pp-cli messaging-profiles delete`** - Deletes the specified messaging profile and returns the profile's final configuration.
- **`telnyx-pp-cli messaging-profiles list`** - Lists messaging profiles owned by the authenticated account. Apply the documented filters and pagination parameters to narrow the result set.
- **`telnyx-pp-cli messaging-profiles retrieve`** - Returns the complete configuration of the specified messaging profile, including webhook and sender-selection settings.
- **`telnyx-pp-cli messaging-profiles update`** - Updates the supplied settings on the specified messaging profile. Settings omitted from the request remain unchanged.

### number-orders

Manage number orders

- **`telnyx-pp-cli number-orders create`** - Creates an order to purchase the specified phone numbers and returns the created order. Track fulfillment through the order's status.
- **`telnyx-pp-cli number-orders list`** - Returns a paginated list of your phone number orders, with support for filtering.
- **`telnyx-pp-cli number-orders retrieve`** - Returns the details of an existing phone number order, including its status and the numbers included.
- **`telnyx-pp-cli number-orders update`** - Updates an existing phone number order, for example to satisfy regulatory requirements attached to the order, and returns the updated order.

### number-reservations

Manage number reservations

- **`telnyx-pp-cli number-reservations create`** - Creates a Phone Number Reservation for multiple numbers.
- **`telnyx-pp-cli number-reservations list`** - Gets a paginated list of phone number reservations.
- **`telnyx-pp-cli number-reservations retrieve`** - Returns the details of a single phone number reservation, including its status and the reserved numbers.

### outbound-voice-profiles

Outbound voice profiles operations

- **`telnyx-pp-cli outbound-voice-profiles create-voice-profile`** - Creates a new outbound voice profile defining calling permissions, destinations, and limits for outbound calls, and returns the created profile.
- **`telnyx-pp-cli outbound-voice-profiles delete`** - Deletes an existing outbound voice profile.
- **`telnyx-pp-cli outbound-voice-profiles get`** - Retrieves the details of an existing outbound voice profile.
- **`telnyx-pp-cli outbound-voice-profiles list`** - Get all outbound voice profiles belonging to the user that match the given filters.
- **`telnyx-pp-cli outbound-voice-profiles update`** - Updates an existing outbound voice profile.

### phone-numbers

Associate phone numbers with a verified DIR so calls from those numbers carry the DIR's display identity.

- **`telnyx-pp-cli phone-numbers create-csv-download`** - Starts generation of a CSV export for phone numbers matching the supplied filters. The `csv_format` parameter selects the output format, and the response contains the resulting download record.
- **`telnyx-pp-cli phone-numbers create-delete-job`** - Creates a new background job to delete a batch of numbers. At most one thousand numbers can be updated per API call.
- **`telnyx-pp-cli phone-numbers create-job-update-emergency-settings`** - Creates a background job to update the emergency settings of a collection of phone numbers. At most one thousand numbers can be updated per API call.
- **`telnyx-pp-cli phone-numbers create-update-job`** - Creates a new background job to update a batch of numbers. At most one thousand numbers can be updated per API call. At least one of the updateable fields must be submitted. IMPORTANT: You must either specify filters (using the filter parameters) or specific phone numbers (using the phone_numbers parameter in the request body). If you specify filters, ALL phone numbers that match the given filters (up to 1000 at a time) will be updated. If you want to update only specific numbers, you must use the phone_numbers parameter in the request body. When using the phone_numbers parameter, ensure you follow the correct format as shown in the example (either phone number IDs or phone numbers in E164 format).
- **`telnyx-pp-cli phone-numbers delete`** - Deletes the specified phone number from the account. The response contains the phone number's final deleted representation.
- **`telnyx-pp-cli phone-numbers get-csv-download`** - Returns the current status and download details for the CSV export identified by `id`.
- **`telnyx-pp-cli phone-numbers list`** - Returns phone numbers associated with the account. Results support pagination, sorting, and filters for number attributes, status, source, connections, billing groups, emergency addresses, tags, and customer references.
- **`telnyx-pp-cli phone-numbers list-csv-downloads`** - Returns CSV export jobs created for account phone numbers, including each export's current status and pagination metadata.
- **`telnyx-pp-cli phone-numbers list-jobs`** - Returns background jobs that operate on phone numbers. Filter by job type, target phone numbers, or job status, and sort by creation time. Multiple phone-number or status values use OR semantics within that filter; different filter categories use AND semantics. Results include pagination metadata.
- **`telnyx-pp-cli phone-numbers list-regulatory-requirements`** - Retrieve the regulatory requirements that apply to a list of phone numbers.
- **`telnyx-pp-cli phone-numbers list-with-messaging-settings`** - Returns phone numbers with their current messaging product and messaging-profile assignments.
- **`telnyx-pp-cli phone-numbers list-with-voice-settings`** - Returns account phone numbers together with their voice settings. Results support pagination, sorting, and filters for number, connection name, customer reference, and voice usage payment method.
- **`telnyx-pp-cli phone-numbers retrieve`** - Returns the detailed configuration and current state of the phone number identified by `id`.
- **`telnyx-pp-cli phone-numbers retrieve-job`** - Returns the status and details of the phone-number background job identified by `id`.
- **`telnyx-pp-cli phone-numbers slim-list`** - List phone numbers, This endpoint is a lighter version of the /phone_numbers endpoint having higher performance and rate limit.
- **`telnyx-pp-cli phone-numbers update`** - Updates the configurable settings of the specified phone number. The response contains the complete updated phone-number representation.
- **`telnyx-pp-cli phone-numbers verify-ownership`** - Verifies ownership of the provided phone numbers and returns a mapping of numbers to their IDs, plus a list of numbers not found in the account.

### recordings

Manage recordings

- **`telnyx-pp-cli recordings delete`** - Permanently deletes the specified call recording and returns the deleted recording resource. The media is removed and can no longer be downloaded.
- **`telnyx-pp-cli recordings delete-actions`** - Permanently deletes a list of call recordings.
- **`telnyx-pp-cli recordings get`** - Returns a paginated list of your call recordings, with support for filtering to locate specific recordings.
- **`telnyx-pp-cli recordings get-recordingid`** - Retrieves the details of an existing call recording.

### short-codes

Short codes

- **`telnyx-pp-cli short-codes list`** - Returns short codes owned by the authenticated account. Apply the documented filters and pagination parameters to narrow the result set.
- **`telnyx-pp-cli short-codes retrieve`** - Returns the messaging configuration and assignment details for the specified short code.
- **`telnyx-pp-cli short-codes update`** - Update the settings for a specific short code. To unbind a short code from a profile, set the `messaging_profile_id` to `null` or an empty string.
To add or update tags, include the tags field as an array of strings.

### texml-applications

Manage texml applications

- **`telnyx-pp-cli texml-applications create`** - Creates a TeXML application, which defines the voice URLs and settings used to serve TeXML instructions for calls, and returns the created application.
- **`telnyx-pp-cli texml-applications delete`** - Permanently deletes the specified TeXML application from your account.
- **`telnyx-pp-cli texml-applications find`** - Returns a list of your TeXML Applications.
- **`telnyx-pp-cli texml-applications get`** - Retrieves the details of an existing TeXML Application.
- **`telnyx-pp-cli texml-applications update`** - Updates settings of an existing TeXML Application.

### verifications

Manage verifications

- **`telnyx-pp-cli verifications create-call`** - Starts a verification for the specified phone number and delivers its code in a voice call using the selected Verify profile. Returns the pending verification record.
- **`telnyx-pp-cli verifications create-flashcall`** - Starts a verification for the specified phone number and places a brief call with the code embedded in the caller ID. Returns the pending verification record.
- **`telnyx-pp-cli verifications create-sms`** - Starts a verification for the specified phone number and sends its code by SMS using the selected Verify profile. Returns the pending verification record.
- **`telnyx-pp-cli verifications create-whatsapp`** - Starts a verification for the specified phone number and sends its code over WhatsApp using the selected Verify profile. Returns the pending verification record.
- **`telnyx-pp-cli verifications list`** - Returns a paginated list of verifications associated with the specified phone number.
- **`telnyx-pp-cli verifications retrieve`** - Returns the verification identified by ID, including its channel, phone number, Verify profile, timeout, and current status.
- **`telnyx-pp-cli verifications verify-code-by-phone-number`** - Submits a verification code for the specified phone number and Verify profile. The response indicates whether the code was accepted or rejected.

### verify-profiles

Manage verify profiles

- **`telnyx-pp-cli verify-profiles create`** - Creates a new Verify profile to associate verifications with.
- **`telnyx-pp-cli verify-profiles create-message-template`** - Create a new Verify profile message template.
- **`telnyx-pp-cli verify-profiles delete-profile`** - Deletes the specified Verify profile and returns the deleted profile record.
- **`telnyx-pp-cli verify-profiles get`** - Returns the details of a single Verify profile by its ID, including its verification channel configuration.
- **`telnyx-pp-cli verify-profiles list-profile-message-templates`** - List all Verify profile message templates.
- **`telnyx-pp-cli verify-profiles list-profiles`** - Gets a paginated list of Verify profiles.
- **`telnyx-pp-cli verify-profiles update`** - Updates the specified Verify profile's name, webhook destinations, language, daily spend limits, or channel-specific settings. Returns the updated profile.
- **`telnyx-pp-cli verify-profiles update-message-template`** - Update an existing Verify profile message template.

### webhook-deliveries

Manage webhook deliveries

- **`telnyx-pp-cli webhook-deliveries get`** - Lists webhook_deliveries for the authenticated user
- **`telnyx-pp-cli webhook-deliveries get-webhook-delivery`** - Provides webhook_delivery debug data, such as timestamps, delivery status and attempts.


### conversation


- **`telnyx-pp-cli conversation`** - See the delivery timeline with one guest or vendor number: direction, status, parts and cost (Telnyx records carry no message bodies).


### failures


- **`telnyx-pp-cli failures`** - Failed and undelivered texts grouped by error code and recipient.


### inbox


- **`telnyx-pp-cli inbox unanswered`** - Contacts whose latest text was inbound with no reply yet.


### numbers


- **`telnyx-pp-cli numbers audit`** - Find owned numbers that cannot text: missing messaging profile or missing 10DLC campaign assignment.


### spend


- **`telnyx-pp-cli spend`** - Total messaging cost by number, profile, direction, or day, with balance runway.


### text


- **`telnyx-pp-cli text`** - Send a text after checking the sender is wired correctly and showing recent history; requires --yes.




### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`telnyx-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`telnyx-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`telnyx-pp-cli learnings list`** - Inspect taught rows
- **`telnyx-pp-cli learnings forget <query>`** - Undo a teach
- **`telnyx-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`telnyx-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`telnyx-pp-cli teach-pattern`** - Install a query/resource template up front
- **`telnyx-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `TELNYX_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `telnyx-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
telnyx-pp-cli available-phone-numbers

# JSON for scripting and agents
telnyx-pp-cli available-phone-numbers --json
# Filter to specific fields
telnyx-pp-cli available-phone-numbers --json --select best_effort,cost_information,features

# Dry run — show the request without sending
telnyx-pp-cli available-phone-numbers --dry-run

# Agent mode — JSON + compact + no prompts in one flag
telnyx-pp-cli available-phone-numbers --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `6` partial failure, `7` rate limited, `8` async job submitted but not finished (rerun the printed recovery command; do not resubmit), `10` config error.

## Health Check

```bash
telnyx-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `telnyx-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/telnyx-pp-cli/config.toml`; `--home`, `TELNYX_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `TELNYX_API_KEY` | per_call | No | Set to your API credential. |
| `TELNYX_BEARER_AUTH` | per_call | No | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `telnyx-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `telnyx-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $TELNYX_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized** — export TELNYX_API_KEY=<your v2 key> (keys start with KEY)
- **Texts to US numbers fail with carrier errors** — telnyx-pp-cli numbers audit to find numbers without an approved 10DLC campaign
- **conversation or spend shows nothing** — telnyx-pp-cli sync --since 30d first

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**telnyx-cli**](https://github.com/team-telnyx/telnyx-cli) — Go (127 stars)
- [**telnyx-mcp-server**](https://github.com/team-telnyx/telnyx-mcp-server) — Python (25 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
