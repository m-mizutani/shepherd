# Shepherd
[![CI](https://github.com/m-mizutani/shepherd/actions/workflows/ci.yml/badge.svg)](https://github.com/m-mizutani/shepherd/actions/workflows/ci.yml)
[![Lint](https://github.com/m-mizutani/shepherd/actions/workflows/lint.yml/badge.svg)](https://github.com/m-mizutani/shepherd/actions/workflows/lint.yml)
[![Gosec](https://github.com/m-mizutani/shepherd/actions/workflows/gosec.yml/badge.svg)](https://github.com/m-mizutani/shepherd/actions/workflows/gosec.yml)
[![Trivy](https://github.com/m-mizutani/shepherd/actions/workflows/trivy.yml/badge.svg)](https://github.com/m-mizutani/shepherd/actions/workflows/trivy.yml)

Slack-native ticket management for team workflows — messages become tickets, threads become conversations.

<p align="center">
  <img src="./docs/images/logo.png" height="128" />
</p>

## What it does

- **Slack-first** — Tickets are created from Slack messages; status updates flow back to threads.
- **Workspace-based** — Each Slack channel maps to a workspace with its own statuses and custom fields.
- **Triage built in** — A background agent investigates new tickets, asks the reporter follow-up questions in-thread, and produces a summary for the assignee.
- **Web UI** — A React SPA for browsing tickets, configuring workspaces, and tracking status.

See [docs/slack.md § How It Works](docs/slack.md#how-it-works) for the user-facing flow.

## Getting started

Read [docs/setup.md](docs/setup.md) for the end-to-end setup walkthrough — from getting a binary to running against a real Slack workspace.

## Documentation

| Document | Purpose |
|---|---|
| [docs/setup.md](docs/setup.md) | End-to-end setup walkthrough (build, configure, run, verify). |
| [docs/configuration.md](docs/configuration.md) | Reference for every CLI flag, environment variable, and workspace TOML field. |
| [docs/slack.md](docs/slack.md) | Slack app provisioning and event/interactivity wiring. |
| [docs/notion.md](docs/notion.md) | Notion integration setup and Source registration. |

## License

Apache License 2.0
