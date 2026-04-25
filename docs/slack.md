# Slack App Setup Guide

Shepherd integrates with Slack for authentication (OIDC) and event-driven ticket management. This guide covers how to create and configure a Slack app for use with Shepherd.

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
| `channels:read` | Resolve `#channel-name` to channel ID |
| `groups:history` | Read messages in private channels |
| `groups:read` | Resolve private `#channel-name` to channel ID |
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

Each workspace's TOML config must specify the Slack channel to monitor:

```toml
[slack]
channel = "#my-channel"       # Channel name (resolved to ID at startup via Slack API)
```

Or use a raw channel ID directly:

```toml
[slack]
channel = "C0123456789"       # Channel ID (used as-is)
```

When using `#channel-name` format, `--slack-bot-token` must be set so Shepherd can resolve the name to a channel ID at startup. The bot also needs the `channels:read` and `groups:read` scopes.

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
