# Configuration Reference

This document is the authoritative reference for every CLI flag, environment
variable, and workspace TOML field that Shepherd accepts. It is paired with
[docs/setup.md](setup.md), which walks through how these settings fit together.

## Configuration sources

Shepherd reads each setting from, in order of precedence:

1. The CLI flag (e.g. `--llm-provider`)
2. The corresponding environment variable (e.g. `SHEPHERD_LLM_PROVIDER`)
3. The default value baked into the binary (when one exists)

The Shepherd binary has two layers of flags:

- **Root flags** (`shepherd <flag> <subcommand> ...`) — currently logging only
- **Subcommand flags** (`shepherd serve <flag>`) — everything else

Subcommands: `serve` (run the HTTP server), `migrate` (placeholder for future
data migrations), `validate` (validate workspace TOML files).

## Logging (root flags)

These flags apply to all subcommands and must appear before the subcommand
name on the command line.

| Flag | Env var | Default | Description |
|---|---|---|---|
| `--log-level`, `-l` | `SHEPHERD_LOG_LEVEL` | `info` | One of `debug`, `info`, `warn`, `error`. |
| `--log-format`, `-f` | `SHEPHERD_LOG_FORMAT` | `console` | `console` for human-readable output, `json` for structured logs. |
| `--log-output` | `SHEPHERD_LOG_OUTPUT` | `stdout` | `stdout`, `stderr`, `-` (stdout), or a file path. Files are opened with append mode. |
| `--log-quiet`, `-q` | `SHEPHERD_LOG_QUIET` | `false` | Suppress all log output. |
| `--log-stacktrace` | `SHEPHERD_LOG_STACKTRACE` | `true` | Include stack traces in console-formatted error logs. |

## `serve` subcommand

### Server

| Flag | Env var | Default | Description |
|---|---|---|---|
| `--addr` | `SHEPHERD_ADDR` | `localhost:8080` | Listen address. |
| `--base-url` | `SHEPHERD_BASE_URL` | _(empty)_ | External base URL of the Shepherd deployment, e.g. `https://shepherd.example.com`. Required when Slack OAuth is configured (`--slack-client-id` + `--slack-client-secret`); also used to build the OAuth callback URL `<base-url>/api/auth/callback`. |
| `--lang` | `SHEPHERD_LANG` | `en` | Backend message language for end-user copy (Slack notifications, etc.). One of `en`, `ja`. |
| `--config` | `SHEPHERD_CONFIG` | `./config.toml` | Workspace TOML file or directory. May be specified multiple times. When a directory is given, every `*.toml` file under it is loaded. See [Workspace TOML](#workspace-toml). |
| `--triage-iteration-cap` | `SHEPHERD_TRIAGE_ITERATION_CAP` | `10` | Maximum number of triage planner turns per ticket before aborting. |

### Repository backend

Shepherd persists workspaces, tickets, and related entities in either
Firestore or an in-memory store.

| Flag | Env var | Default | Description |
|---|---|---|---|
| `--repository-backend` | `SHEPHERD_REPOSITORY_BACKEND` | `memory` | `memory` or `firestore`. |
| `--firestore-project-id` | `SHEPHERD_FIRESTORE_PROJECT_ID` | _(empty)_ | Required when `--repository-backend=firestore`. |
| `--firestore-database-id` | `SHEPHERD_FIRESTORE_DATABASE_ID` | _(empty)_ | Optional Firestore database ID. Leave empty to use the project's default database. |

When `firestore` is selected, the process must be able to obtain Google
Application Default Credentials with read/write access to Firestore on the
target project. The `memory` backend is for development only — all data is
lost when the process exits.

### Slack

These flags govern Slack OIDC sign-in, the Slack bot token, and event
verification. The full provisioning walkthrough lives in [docs/slack.md](slack.md).

| Flag | Env var | Description |
|---|---|---|
| `--slack-client-id` | `SHEPHERD_SLACK_CLIENT_ID` | Slack OAuth client ID for Sign in with Slack. |
| `--slack-client-secret` | `SHEPHERD_SLACK_CLIENT_SECRET` | Slack OAuth client secret. |
| `--slack-bot-token` | `SHEPHERD_SLACK_BOT_TOKEN` | Bot token (`xoxb-...`) used to read channels, post replies, resolve `#channel-name` to channel IDs, and invoke the Slack-backed LLM tools. |
| `--slack-signing-secret` | `SHEPHERD_SLACK_SIGNING_SECRET` | Signing secret used to verify incoming events on `/hooks/slack/event` and interactive payloads on `/hooks/slack/interaction`. |
| `--no-authn` | `SHEPHERD_NO_AUTHN` | Development-only: bypass OAuth and authenticate every request as the given Slack User ID. Mutually intended-exclusive with the OAuth flags. |

The Slack event/interaction handlers register only when both
`--slack-bot-token` and `--slack-signing-secret` are set. When OAuth is
configured (`--slack-client-id` + `--slack-client-secret`), `--base-url` is
required so Shepherd can build the callback URL.

### LLM

An LLM provider is **required** — `serve` aborts at startup when
`--llm-provider` is empty.

| Flag | Env var | Description |
|---|---|---|
| `--llm-provider` | `SHEPHERD_LLM_PROVIDER` | One of `openai`, `claude`, `gemini`. Required. |
| `--llm-model` | `SHEPHERD_LLM_MODEL` | Optional model name override (provider default is used when empty). |
| `--llm-openai-api-key` | `SHEPHERD_LLM_OPENAI_API_KEY` | OpenAI API key. Required when `--llm-provider=openai`. |
| `--llm-claude-api-key` | `SHEPHERD_LLM_CLAUDE_API_KEY` | Anthropic API key. Used when `--llm-provider=claude` against Anthropic directly. |
| `--llm-gemini-project-id` | `SHEPHERD_LLM_GEMINI_PROJECT_ID` | Google Cloud project ID. Required for Gemini, and for Claude on Google Cloud. |
| `--llm-gemini-location` | `SHEPHERD_LLM_GEMINI_LOCATION` | Google Cloud location (e.g. `us-central1`). Required for Gemini, and for Claude on Google Cloud. |

Required combinations per provider:

| `--llm-provider` | Required additional flags |
|---|---|
| `openai` | `--llm-openai-api-key` |
| `claude` | Either `--llm-claude-api-key`, **or** both `--llm-gemini-project-id` and `--llm-gemini-location` (Claude on Google Cloud / Gemini Enterprise Agent Platform). The two routes are mutually exclusive. |
| `gemini` | Both `--llm-gemini-project-id` and `--llm-gemini-location`. |

For Claude on Google Cloud, leave `--llm-claude-api-key` unset and provide
the Google Cloud project + location instead. Application Default Credentials
must be available to the process.

### Agent storage

Shepherd persists per-ticket gollem conversation history and per-call
execution traces. Exactly one backend must be configured — both unset is an
error, and both set is also an error.

| Flag | Env var | Default | Description |
|---|---|---|---|
| `--agent-storage-fs-dir` | `SHEPHERD_AGENT_STORAGE_FS_DIR` | _(empty)_ | Local filesystem directory. Mutually exclusive with `--agent-storage-gcs-bucket`. |
| `--agent-storage-gcs-bucket` | `SHEPHERD_AGENT_STORAGE_GCS_BUCKET` | _(empty)_ | GCS bucket name. Mutually exclusive with `--agent-storage-fs-dir`. |
| `--agent-storage-gcs-prefix` | `SHEPHERD_AGENT_STORAGE_GCS_PREFIX` | `shepherd/` | Object name prefix under the GCS bucket. |

Object layout under the chosen base (`{fs-dir}` or `gs://{bucket}/{prefix}`):

```
history/v1/{workspaceID}/{ticketID}.json   # one file per ticket, overwritten each turn
trace/v1/{traceID}.json                    # one file per agent.Execute call
```

The GCS backend requires Google Application Default Credentials with object
read/write access to the bucket.

### Notion

Notion integration is optional. When `--notion-token` is unset, all Notion
tools and the Sources WebUI section are inert. Detailed setup is in
[docs/notion.md](notion.md).

| Flag | Env var | Description |
|---|---|---|
| `--notion-token` | `SHEPHERD_NOTION_TOKEN` | Notion Internal Integration Secret. |

### Sentry

| Flag | Env var | Default | Description |
|---|---|---|---|
| `--sentry-dsn` | `SHEPHERD_SENTRY_DSN` | _(empty)_ | Sentry DSN. When empty, Sentry reporting is disabled. |
| `--sentry-env` | `SHEPHERD_SENTRY_ENV` | `development` | Sentry environment label. |

## Workspace TOML

Each TOML file passed via `--config` describes one workspace: its identity,
the Slack channel that feeds it, the available statuses, and the custom
fields the Web UI exposes.

When `--config` points at a directory, every `*.toml` file under it is
loaded. Two files cannot share the same `workspace.id` or the same
`slack.channel`; both collisions are detected at startup and abort the
server.

A minimal valid file:

```toml
[workspace]
id = "support"
name = "Support Team"

[ticket]
default_status = "open"
closed_statuses = ["resolved"]

[slack]
channel = "#team-support"

[[statuses]]
id = "open"
name = "Open"
color = "#22c55e"

[[statuses]]
id = "resolved"
name = "Resolved"
color = "#6b7280"
```

A fuller example, including custom fields and labels, lives at
[`examples/config.toml`](../examples/config.toml).

### `[workspace]`

| Key | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | Workspace ID. Must match `^[a-z0-9]+(-[a-z0-9]+)*$` and be at most 63 characters. Used in URLs and Firestore document IDs. |
| `name` | string | no | Display name. Falls back to `id` when empty. |

### `[ticket]`

| Key | Type | Required | Description |
|---|---|---|---|
| `default_status` | string | no | Status ID assigned to newly created tickets. Defaults to the first entry of `[[statuses]]`. |
| `closed_statuses` | string list | no | Status IDs treated as terminal (used by the Web UI's open/closed filter). |

### `[slack]`

| Key | Type | Required | Description |
|---|---|---|---|
| `channel` | string | yes | Slack channel to monitor. Either a channel ID (`C0123456789`) or a channel name prefixed with `#` (`#team-support`). When the `#name` form is used, `--slack-bot-token` is required at startup so Shepherd can resolve the name to an ID. |

### `[[statuses]]`

Each entry is one selectable status. Order in the file determines the
display order in the Web UI.

| Key | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | Status ID. Referenced by `ticket.default_status` and `ticket.closed_statuses`. |
| `name` | string | yes | Display name. |
| `color` | string | yes | Hex color string (e.g. `#22c55e`). |

### `[labels]`

Optional. Lets you re-label core entities in the Web UI (for example,
calling tickets "Issues" instead).

| Key | Type | Default | Description |
|---|---|---|---|
| `ticket` | string | `Ticket` | Display name for the ticket entity. |
| `title` | string | `Title` | Display name for the ticket title field. |
| `description` | string | `Description` | Display name for the ticket description field. |

### `[[fields]]`

Custom fields surfaced on each ticket. Each entry defines one field.

| Key | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | Field ID. Used as the Firestore key. |
| `name` | string | yes | Display name. |
| `type` | string | yes | One of `select`, `date`, `url`. |
| `required` | bool | no | Whether the Web UI requires a value before saving. |
| `description` | string | no | Help text shown in the Web UI. |
| `options` | array of tables | required for `select` | See `[[fields.options]]` below. |

### `[[fields.options]]`

Options for `select`-typed fields.

| Key | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | Option ID. |
| `name` | string | yes | Display name. |
| `color` | string | no | Hex color string. |
| `metadata` | table | no | Free-form key/value metadata for advanced use cases. |

## Validating workspace TOML

The `validate` subcommand checks one or more TOML files without starting
the server:

```bash
shepherd validate --config ./config.toml
shepherd validate --config ./workspaces/
```

Use it in CI or before rolling out a new workspace file.

## Common startup errors

Pointers for the most frequent misconfigurations:

| Symptom | Likely cause |
|---|---|
| `--llm-provider is required` | No LLM provider set. Pick `openai`, `claude`, or `gemini`. |
| `agent storage is required: set either --agent-storage-fs-dir or --agent-storage-gcs-bucket` | Neither agent-storage backend was configured. |
| `--agent-storage-fs-dir and --agent-storage-gcs-bucket are mutually exclusive` | Both backends were set; pick one. |
| `--base-url is required when Slack OAuth is enabled` | Slack OAuth flags are set but `--base-url` is empty. |
| `channel name resolution requires --slack-bot-token` | Workspace TOML uses `#channel-name` form but no bot token is configured. |
| `firestore-project-id is required when using firestore backend` | `--repository-backend=firestore` was set without `--firestore-project-id`. |
