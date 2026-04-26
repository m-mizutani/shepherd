# Ticket Triage Agent

When a new ticket is created from a Slack message, Shepherd's triage agent automatically:

1. Investigates the ticket in parallel using existing tool providers (Slack search, past tickets, Notion, etc.).
2. Asks the reporter follow-up questions via an inline Slack form when information is missing.
3. Hands off to a (possibly auto-assigned) owner with a structured summary.

The triage state is intentionally minimal: only `Ticket.Triaged` (`bool`) is added to the ticket model. Everything else — past plans, subtask results, reporter answers — lives in the gollem agent history under session id `{workspaceID}/{ticketID}/plan`.

## When does triage run?

- Triggered automatically right after a ticket is created from a Slack parent thread message (`HandleNewMessage` in `pkg/usecase/slack.go`).
- Skipped when `Ticket.Triaged == true` (already finished, including aborted runs).
- Resumed when the reporter clicks Submit on a triage question form (`/hooks/slack/interactions`).

## What triage produces in Slack

| Phase | Slack message |
|---|---|
| Investigate | Single thread message with one `context` block per subtask. Each block updates as the subtask progresses (queued / running / done / failed). |
| Ask | Thread message with input blocks (radio or checkboxes) per question, an "if none of the above" free-text input per question, and a single Submit button. |
| Submit ack | The question message is replaced with a "thanks, received" notice; the planner resumes in the background. |
| Complete | A summary message tagging the assignee (or noting why none was picked), with sections for summary, key findings, reporter answers, similar tickets, and recommended next steps. |
| Abort | A short message stating the abort reason (`iteration cap exceeded`, internal error, etc.). The ticket is still marked `Triaged = true`. |

## Tools used by child agents

The planner picks per-subtask `allowed_tools` from the workspace's enabled tool providers (`tool.Catalog.For`). All providers configured for the workspace are eligible:

- `slack_search_messages`, `slack_get_thread`, `slack_get_channel_history`, `slack_get_user_info` (when Slack provider enabled)
- `ticket_search`, `ticket_get`, `ticket_get_comments`, `ticket_get_history` (when Ticket provider enabled)
- `notion_*` (when Notion provider enabled, see `docs/notion.md`)
- `workspace_describe`, `current_time` (Meta provider, always available)

## CLI configuration

| Flag | Default | Effect |
|------|---------|--------|
| `--triage-iteration-cap` | `10` | Maximum number of `propose_*` planner turns per ticket. Includes investigate / ask / complete proposals. Exceeding the cap aborts triage with a brief Slack notice. |

## Slack configuration

Triage requires the Interactivity endpoint at `{SHEPHERD_BASE_URL}/hooks/slack/interactions` to be configured in the Slack app. See `docs/slack.md` section 4a.

## Out of scope (planned for follow-up issues)

- Workspace-level toggle to disable triage.
- Ability for reporters to send free-form messages while the form is open (currently ignored; existing thread-reply handler still records them as comments).
- Buttons on the completion message to reassign / take over / re-triage.
- A startup sweep that aborts plans whose owning instance died mid-run.
- Buffering of Slack messages received during the self-running window so the planner can incorporate them mid-iteration. Tracked in `.cckiro/issues/buffer-inflight-triage-messages.md`.
