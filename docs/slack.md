# Slack App Setup Guide

Shepherd integrates with Slack for authentication (OIDC) and event-driven ticket management. This guide covers how to create and configure a Slack app for use with Shepherd.

For a top-level walkthrough that ties Slack setup together with LLM, storage, and workspace configuration, see [docs/setup.md](setup.md). The full reference for every CLI flag and environment variable mentioned below is in [docs/configuration.md](configuration.md).

## Prerequisites

- A Slack workspace where you have admin permissions
- Shepherd deployed and accessible via a public URL (for OAuth callback and event webhooks)

## 1. Create a Slack App

1. Go to [https://api.slack.com/apps](https://api.slack.com/apps)
2. Click **Create New App** > **From scratch**
3. Enter an app name (e.g., "Shepherd") and select your workspace
4. Click **Create App**

## 2. Configure OAuth & Permissions

### Redirect URL

1. Navigate to **OAuth & Permissions** in the sidebar
2. Under **Redirect URLs**, add: `{SHEPHERD_BASE_URL}/api/auth/callback`
   - Example: `https://shepherd.example.com/api/auth/callback`

### Bot Token Scopes

Add the following **Bot Token Scopes**:

| Scope | Purpose |
|-------|---------|
| `chat:write` | Post ticket creation replies in threads |
| `app_mentions:read` | Receive `app_mention` events for LLM-assisted replies |
| `channels:history` | Read messages in public channels |
| `groups:history` | Read messages in private channels |
| `users:read` | Fetch user profile (name) for NoAuthn mode and `slack_get_user_info` LLM tool |
| `users:read.email` | Fetch user email for NoAuthn mode |
| `search:read` | Required by the `slack_search_messages` LLM tool (Slack `search.messages` API). User-token scope on classic apps; on Slack Marketplace apps this scope must be approved for bot tokens or the search tool returns "not_allowed_token_type". |

### User Token Scopes

Add the following **User Token Scopes** (required for OpenID Connect authentication):

| Scope | Purpose |
|-------|---------|
| `openid` | Required for OIDC flow |
| `email` | Retrieve user's email address |
| `profile` | Retrieve user's display name |

## 3. Configure OpenID Connect (Sign in with Slack)

1. Navigate to **OpenID Connect** in the sidebar (under Features)
2. Verify the **Redirect URL** matches the one configured in OAuth & Permissions

Shepherd uses Slack's OIDC for user authentication. The following claims are used:
- `sub` — Slack User ID
- `email` — User's email address
- `name` — User's display name

## 4. Enable Event Subscriptions

1. Navigate to **Event Subscriptions** in the sidebar
2. Toggle **Enable Events** to On
3. Set **Request URL** to: `{SHEPHERD_BASE_URL}/hooks/slack/event`
   - Example: `https://shepherd.example.com/hooks/slack/event`
   - Slack will send a challenge request to verify the endpoint

### Subscribe to Bot Events

Add the following events under **Subscribe to bot events**:

| Event | Purpose |
|-------|---------|
| `message.channels` | Detect new messages in public channels → auto-create tickets |
| `message.groups` | Detect new messages in private channels → auto-create tickets |
| `app_mention` | Trigger an LLM-generated reply when the bot is mentioned in a ticket thread |

## 4a. Enable Interactivity (required for triage)

The triage agent uses Slack Block Kit input blocks plus a Submit button to ask the reporter follow-up questions. The reporter's submit click is delivered as a `block_actions` interactive callback, which arrives on a different endpoint than Events API.

1. Navigate to **Interactivity & Shortcuts** in the sidebar
2. Toggle **Interactivity** to On
3. Set **Request URL** to: `{SHEPHERD_BASE_URL}/hooks/slack/interaction`
   - Example: `https://shepherd.example.com/hooks/slack/interaction`

This endpoint is signed with the same signing secret as `/hooks/slack/event` and handles the following Slack interactivity types:

### `block_actions`

| action_id | Purpose |
|-----------|---------|
| `triage_submit_answers` | Reporter clicked Submit on the triage question form. Bot reads `state.values` (keyed by `block_id == question.id`), records the answers in agent history, and resumes the planner loop. |
| `triage_retry` | Reporter clicked the retry button on a failure-recovery message. Bot re-dispatches the planner loop for the ticket. |
| `triage_review_edit` | Anyone clicked **Edit** on the triage review message. Bot opens the Edit modal synchronously via `views.open` (Slack's `trigger_id` is only valid for ~3 seconds). |
| `triage_review_submit` | Anyone clicked **Submit** on the triage review message. Bot finalises the planner's latest `PlanComplete` proposal as-is, posts a follow-up "Submitted" message into the thread mentioning every selected assignee. The original review message is not rewritten. |
| `triage_review_reinvestigate` | Anyone clicked **Re-investigate** on the triage review message. Bot opens the instruction modal synchronously via `views.open`. |

### `view_submission`

| callback_id | Purpose |
|-------------|---------|
| `triage_review_edit_modal` | Edit modal submitted. Bot parses summary / assignees (multi-user select) / custom-field inputs, persists field values to the ticket, finalises with the edited proposal, and posts the "Submitted" follow-up. Required-field violations come back as `response_action: errors` so the modal stays open. |
| `triage_review_reinvestigate_modal` | Re-investigate modal submitted. Bot appends the user's instruction to the planner's gollem history as a user-role message, posts a "Re-investigating…" follow-up message into the thread, and re-dispatches the planner. The review message itself is left untouched. |

**Permission model.** All three review buttons and both modals are open to any user in the channel — there is no reporter-only restriction. Idempotency falls out of the `Triaged` flag: once a triage is finalised, subsequent button clicks no-op with an ephemeral notice.

**Configuration.** Whether the planner pauses for review (vs. finalising immediately) is controlled per workspace by `[triage] auto` in the workspace TOML config. The default is `false` (review required). Set to `true` to opt into the legacy behaviour where `PlanComplete` finalises the ticket immediately with no human review step.

```toml
[triage]
auto = false   # default; set to true to skip the review step and auto-finalise
```

**State model.** The review flow does not introduce a `PendingTriage` field on the ticket. The "current proposal" is the latest assistant `PlanComplete` in the planner's gollem history; the "review pending" state is `Triaged == false` while that latest plan is a `PlanComplete`. Edit submissions feed the edited values directly into `FinalizeTriage` — there is no intermediate "edited but unsubmitted" snapshot to reconcile.

## 5. Install the App

1. Navigate to **Install App** in the sidebar
2. Click **Install to Workspace** and authorize

## 6. Collect Credentials

After installation, gather the following values from the Slack app settings:

| Value | Location | Env Var |
|-------|----------|---------|
| **Client ID** | Basic Information > App Credentials | `SHEPHERD_SLACK_CLIENT_ID` |
| **Client Secret** | Basic Information > App Credentials | `SHEPHERD_SLACK_CLIENT_SECRET` |
| **Signing Secret** | Basic Information > App Credentials | `SHEPHERD_SLACK_SIGNING_SECRET` |
| **Bot User OAuth Token** | OAuth & Permissions | `SHEPHERD_SLACK_BOT_TOKEN` |

## 7. Configure Shepherd

### Environment Variables

```bash
# Required for Slack OIDC authentication
SHEPHERD_SLACK_CLIENT_ID=<your-client-id>
SHEPHERD_SLACK_CLIENT_SECRET=<your-client-secret>

# Required for Slack event webhooks
SHEPHERD_SLACK_BOT_TOKEN=xoxb-...
SHEPHERD_SLACK_SIGNING_SECRET=<your-signing-secret>

# Required when Slack OAuth is enabled
SHEPHERD_BASE_URL=https://shepherd.example.com
```

### CLI Flags

The same values can be passed as CLI flags:

```bash
shepherd serve \
  --slack-client-id <client-id> \
  --slack-client-secret <client-secret> \
  --slack-bot-token xoxb-... \
  --slack-signing-secret <signing-secret> \
  --base-url https://shepherd.example.com
```

### Workspace Configuration

Each workspace's TOML config must specify the Slack channel ID to monitor:

```toml
[slack]
channel = "C0123456789"       # Channel ID (must match ^[CDG][A-Z0-9]{8,}$)
```

The `#channel-name` form is **not** supported. To find a channel's ID:

1. Open the channel in the Slack desktop or web client
2. Click the channel name in the header to open **View channel details**
3. Scroll to **About** at the bottom; the **Channel ID** is shown there with a copy button

If you previously used the `#channel-name` form, replace it with the channel ID copied this way — Shepherd will refuse to start with a clear error message otherwise.

## 8. Invite the Bot

Invite the Shepherd bot to each channel configured in your workspace TOML files:

```
/invite @Shepherd
```

## LLM-Assisted Replies (required)

When a user mentions the bot (`@Shepherd ...`) inside a ticket thread, Shepherd generates a reply using an LLM. The bot reads the ticket title, description, prior comments, and the latest mention, then posts a generated answer in the thread.

Configuring an LLM provider is **required** — `serve` aborts at startup when no provider is set. Choose one of the providers below:

| Flag | Env Var | Notes |
|------|---------|-------|
| `--llm-provider` | `SHEPHERD_LLM_PROVIDER` | `openai` / `claude` / `gemini`. Required. |
| `--llm-model` | `SHEPHERD_LLM_MODEL` | Optional model name override. |
| `--llm-openai-api-key` | `SHEPHERD_LLM_OPENAI_API_KEY` | Required when provider is `openai`. |
| `--llm-claude-api-key` | `SHEPHERD_LLM_CLAUDE_API_KEY` | Used when provider is `claude` and you want direct Anthropic access. |
| `--llm-gemini-project-id` | `SHEPHERD_LLM_GEMINI_PROJECT_ID` | Google Cloud project. Required for `gemini`, or for `claude` via Gemini Enterprise Agent Platform (formerly Vertex AI). |
| `--llm-gemini-location` | `SHEPHERD_LLM_GEMINI_LOCATION` | Google Cloud location, e.g. `us-central1`. |

For Claude on Google Cloud, set `--llm-provider=claude` together with `--llm-gemini-project-id` and `--llm-gemini-location` (instead of `--llm-claude-api-key`).

### Agent execution model

`@Shepherd` mentions are handled by a [gollem](https://github.com/m-mizutani/gollem) `Agent` (`agent.Execute`), not a single round-trip `GenerateContent` call. This means each mention can issue multiple LLM turns (and, in the future, tool calls) before producing the final reply, while `gollem` automatically maintains a per-thread conversation history.

## Agent Storage (required)

The agent persists two kinds of data and **one of the two backends below must be configured** (they are mutually exclusive):

- **History** — the gollem conversation history per Slack ticket. Lets a follow-up mention pick up where the previous one left off.
- **Trace** — one execution-trace JSON per `agent.Execute` call. Useful for debugging and for the [`gollem view`](https://github.com/m-mizutani/gollem) trace viewer.

| Flag | Env Var | Notes |
|------|---------|-------|
| `--agent-storage-fs-dir` | `SHEPHERD_AGENT_STORAGE_FS_DIR` | Local filesystem directory. Mutually exclusive with `--agent-storage-gcs-bucket`. |
| `--agent-storage-gcs-bucket` | `SHEPHERD_AGENT_STORAGE_GCS_BUCKET` | GCS bucket name. Mutually exclusive with `--agent-storage-fs-dir`. |
| `--agent-storage-gcs-prefix` | `SHEPHERD_AGENT_STORAGE_GCS_PREFIX` | Object name prefix under the bucket. Default: `shepherd/`. |

Layout under the chosen base (`{fs-dir}` or `gs://{bucket}/{prefix}`):

```
history/v1/{workspaceID}/{ticketID}.json   # one file per ticket, overwritten each turn
trace/v1/{traceID}.json                    # one file per agent.Execute call
```

Each trace's metadata `Labels` include `workspace_id`, `ticket_id`, `channel_id`, and `seq` (the 1-based index of the bot reply within the ticket thread), so traces can be ordered by mention even though `traceID` is a UUID v7.

Save failures from either side are logged but never abort agent execution; load failures (including `ErrHistoryVersionMismatch`) start the agent with a fresh session.

## How It Works

1. **New message in a monitored channel** → Shepherd creates a ticket with the message as the title/description, then replies in a thread with a link to the ticket in the Web UI
2. **Thread reply on a ticket message** → Shepherd records the reply as a comment on the corresponding ticket
3. **`@Shepherd` mention in a ticket thread** → Shepherd generates a reply based on the ticket context and posts it in the thread
4. Bot messages and subtypes (join/leave/etc.) are ignored

## Development Mode (NoAuthn)

For local development without Slack OAuth:

```bash
shepherd serve --no-authn U_DEV --repository-backend memory --config examples/config.toml
```

This skips all OAuth flows and authenticates as a test user. The `--no-authn` flag accepts a Slack User ID that will be used as the authenticated user's `sub` claim.
