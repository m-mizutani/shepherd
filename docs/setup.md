# Shepherd Setup Guide

This guide walks through setting up Shepherd from scratch — from getting a
binary in your hands to running it in production with a real Slack
workspace. For the full list of every flag, environment variable, and TOML
field, see [docs/configuration.md](configuration.md).

## Prerequisites

You will need:

- **Slack workspace admin access** — required to create the Slack app and
  install it into the workspace.
- **A public HTTPS endpoint for Shepherd** — Slack delivers OAuth callbacks,
  events, and interactivity payloads to Shepherd, so the deployment must be
  reachable from Slack's servers. Local development can skip this by using
  the `--no-authn` mode described below.
- **An LLM provider** — OpenAI, Anthropic Claude (direct API or via Google
  Cloud), or Google Gemini. An LLM is **required**; the server refuses to
  start without one.
- **A persistent backend for production** — Firestore for the application
  data, plus either a local directory or a GCS bucket for the agent's
  conversation history and execution traces.

To build from source you also need:

- Go 1.26+
- Node.js 22+ and `pnpm` (managed via Corepack)
- [Task](https://taskfile.dev/) for the convenience targets in
  `Taskfile.yml`

## Get the binary

### Build from source

```bash
task build
```

This runs OpenAPI codegen, builds the React frontend, embeds it via
`go:embed`, and produces a `shepherd` binary at the repository root.

### Build with Docker

```bash
docker build -t shepherd:local .
```

The provided `Dockerfile` produces a distroless image with the frontend
embedded. The resulting image runs as a non-root user and uses
`/shepherd` as its entrypoint.

## Run in development mode

The fastest path to a running instance — no Slack app, no GCP project, no
Firestore.

```bash
task dev
```

That target runs both the Go server (`task dev:go`) and the frontend dev
server (`task dev:frontend`) in parallel.

`task dev:go` is equivalent to:

```bash
go run . serve \
  --no-authn U_DEV \
  --repository-backend memory \
  --config examples/config.toml \
  --log-format console
```

In this mode:

- Authentication is bypassed — every request is authenticated as Slack
  user `U_DEV`.
- All data is stored in memory and lost when the process exits.
- Slack events are not delivered (no `--slack-bot-token`); ticket creation
  and triage flows that depend on Slack will not run.

To exercise the LLM-backed features in development, add an LLM provider
flag (e.g. `--llm-provider=openai --llm-openai-api-key=$OPENAI_API_KEY`)
and an agent storage backend (e.g. `--agent-storage-fs-dir=./.dev/agent`).

## Production setup walkthrough

A production-grade deployment ties together five pieces. Work through them
in order; each step links to the relevant reference.

### 1. Pick and configure an LLM provider

Choose one of the supported providers and gather its credentials. The full
flag matrix and required combinations are documented in
[configuration.md → LLM](configuration.md#llm).

The shortest path is OpenAI:

```bash
export SHEPHERD_LLM_PROVIDER=openai
export SHEPHERD_LLM_OPENAI_API_KEY=sk-...
```

For Claude on Google Cloud (Gemini Enterprise Agent Platform), set
`SHEPHERD_LLM_PROVIDER=claude` together with `SHEPHERD_LLM_GEMINI_PROJECT_ID`
and `SHEPHERD_LLM_GEMINI_LOCATION`, and rely on Application Default
Credentials.

### 2. Configure persistent storage

Two independent backends need to be configured:

- **Repository backend** — stores workspaces, tickets, and Web UI state.
  Use `--repository-backend=firestore` with `--firestore-project-id` for
  production. The process needs Application Default Credentials with
  read/write access to Firestore on that project.
- **Agent storage** — stores per-ticket conversation history and execution
  traces. Set exactly one of `--agent-storage-fs-dir` (local filesystem)
  or `--agent-storage-gcs-bucket` (GCS). Both unset, or both set, is an
  error.

See [configuration.md → Repository backend](configuration.md#repository-backend)
and [configuration.md → Agent storage](configuration.md#agent-storage)
for credentials and object layout details.

### 3. Set up the Slack app

Create a Slack app, configure scopes, OAuth, Event Subscriptions, and
Interactivity, then collect the four credentials Shepherd needs:

- `SHEPHERD_SLACK_CLIENT_ID`
- `SHEPHERD_SLACK_CLIENT_SECRET`
- `SHEPHERD_SLACK_BOT_TOKEN`
- `SHEPHERD_SLACK_SIGNING_SECRET`

The full step-by-step procedure (scope list, redirect URLs, event types,
interactivity URL) is in [docs/slack.md](slack.md).

Set `SHEPHERD_BASE_URL` to the externally reachable URL of your deployment
— Shepherd uses it to build the OAuth callback URL.

### 4. Write the workspace TOML files

Each Slack channel that Shepherd monitors is described by one TOML file.
A minimal example:

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

The full schema (custom fields, labels, options) is documented in
[configuration.md → Workspace TOML](configuration.md#workspace-toml). A
fuller example lives at [`examples/config.toml`](../examples/config.toml).

Pass the file (or a directory containing several files) via `--config`:

```bash
shepherd serve --config /etc/shepherd/workspaces/
```

You can validate the files without starting the server:

```bash
shepherd validate --config /etc/shepherd/workspaces/
```

### 5. Run `shepherd serve`

Bringing it together, a representative production invocation looks like
this (split across env vars and flags as you prefer):

```bash
SHEPHERD_BASE_URL=https://shepherd.example.com \
SHEPHERD_LLM_PROVIDER=openai \
SHEPHERD_LLM_OPENAI_API_KEY=sk-... \
SHEPHERD_SLACK_CLIENT_ID=... \
SHEPHERD_SLACK_CLIENT_SECRET=... \
SHEPHERD_SLACK_BOT_TOKEN=xoxb-... \
SHEPHERD_SLACK_SIGNING_SECRET=... \
shepherd serve \
  --addr 0.0.0.0:8080 \
  --repository-backend firestore \
  --firestore-project-id my-gcp-project \
  --agent-storage-gcs-bucket my-shepherd-agent \
  --config /etc/shepherd/workspaces/
```

Finally, invite the Shepherd bot to each channel listed in the workspace
TOML files:

```
/invite @Shepherd
```

## Triage agent

When a new ticket is created from a Slack message, Shepherd's triage agent
investigates it automatically: it runs background subtasks against the
workspace's enabled tools (Slack search, past tickets, Notion, etc.),
asks the reporter follow-up questions via an inline Slack form when
information is missing, and finally posts a summary that hands the ticket
off to an assignee. The reporter's submit click arrives on the
Interactivity endpoint, so step 3 above must include the Interactivity URL
configuration described in [docs/slack.md § 4a](slack.md#4a-enable-interactivity-required-for-triage).

Tuning knob: `--triage-iteration-cap` (default `10`) bounds the number of
planner turns per ticket. See
[configuration.md → Server](configuration.md#server).

## Optional integrations

- **Notion** — lets the LLM agent search and read Notion pages registered
  as Sources for a workspace. See [docs/notion.md](notion.md).
- **Sentry** — when `--sentry-dsn` is set, runtime errors and panics are
  reported to Sentry. The environment label defaults to `development`;
  override it with `--sentry-env`. See
  [configuration.md → Sentry](configuration.md#sentry).
- **Backend message language** — set `--lang=ja` (or `SHEPHERD_LANG=ja`)
  to deliver Slack notifications and other end-user copy in Japanese.

## Verifying the setup

Once `shepherd serve` is running:

1. **Health check.** `GET {SHEPHERD_BASE_URL}/api/v1/health` should
   return `200 OK`.
2. **Web UI.** Open `{SHEPHERD_BASE_URL}/` in a browser and sign in via
   Slack. You should land on the workspace's ticket list.
3. **Slack ingest.** Post a message in a configured channel; Shepherd
   should reply in a thread with a link to the new ticket in the Web UI.
4. **LLM reply.** Mention `@Shepherd` inside that thread; the bot should
   produce an answer based on the ticket context.
5. **Triage.** Newly created tickets are flagged `Triaged` once triage
   finishes; if the agent asks follow-up questions, the reporter's Submit
   click should be acknowledged in-thread.

If any of these fail, check the server logs and consult the "Common
startup errors" table in
[configuration.md](configuration.md#common-startup-errors).
