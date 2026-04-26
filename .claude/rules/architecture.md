---
paths:
  - "pkg/**/*.go"
---

# Architecture & layer responsibilities

The codebase is laid out as a classic layered architecture
(`controller → usecase → repository / service`). Each layer's job is
narrowly defined; cross-layer leakage is the most common code-review failure
mode in this repo, so the boundaries below are non-negotiable. Apply them
even when no rule explicitly calls them out — this section is the
authoritative checklist.

## controller (`pkg/controller/http/`)

**Responsibility:** translate transport-level concerns to usecase calls and
back. Nothing else.

The controller may:

- Parse the inbound request (body, headers, query/path params, signed
  payload verification, multipart, form decoding).
- Bound the request (size limits, auth checks, content-type validation).
- Pick which usecase method to call and marshal the request into that
  method's input struct.
- Translate the usecase's return value into an HTTP response (status code,
  body encoding, redirect, header).
- Acknowledge async / fire-and-forget contracts (e.g. write 200 to Slack
  before dispatching, since Slack enforces a 3-second deadline).

The controller MUST NOT:

- Touch repositories. No `repo.Ticket().Get`, no `repo.User().List`. If you
  need a ticket loaded to decide what to do, that decision belongs in the
  usecase.
- Resolve domain identifiers (channel id → workspace id, slack user id →
  internal user, etc.). These mappings are domain logic.
- Call external services (Slack API, LLM, Notion, Firestore). Even
  "innocent" status pings belong in a service or usecase wrapper.
- Build domain blocks / messages (Slack Block Kit, email bodies, LLM
  prompts). Rendering belongs in `pkg/service/<name>/` or `pkg/usecase/`.
- Hold business invariants ("if Triaged, invalidate the form"). Invariants
  belong inside the usecase that owns the entity.

If the controller needs information to make a decision (e.g. "is this
ticket triaged?"), the answer is *not* "load the ticket here". The answer
is "make the usecase method idempotent and let it decide". The controller
hands off raw payload values; the usecase resolves and decides.

## usecase (`pkg/usecase/`)

**Responsibility:** orchestrate the business operation end-to-end.

The usecase:

- Resolves identifiers (channel → workspace, ticket id → ticket, etc.).
- Loads / mutates persistent state through `interfaces.Repository`.
- Calls external services through their respective service interfaces
  (`SlackTriageClient`, `gollem.LLMClient`, etc.).
- Enforces invariants and idempotency (re-deliveries, duplicate clicks,
  already-finalised entities).
- Dispatches background work via `pkg/utils/async.Dispatch` when the
  operation has a sync entry point and an async tail.
- Returns *domain* errors / states; never HTTP status codes.

A usecase method's signature should take only domain primitives and the
raw payload values the entry point captured. If a method takes both
`workspaceID` and `channelID` "for convenience", the controller is
probably resolving identifiers it shouldn't be.

## repository (`pkg/repository/`) and service (`pkg/service/`)

**Responsibility:** narrow adapters over a single backend.

- `repository/` only knows how to read/write entities. No business
  decisions, no Slack calls, no fan-out to other repositories.
- `service/<name>/` wraps a single external system (Slack, Notion). It
  builds the protocol-level payloads (e.g. Block Kit blocks) and calls the
  third-party SDK. It does not load tickets, does not consult the registry.

## domain (`pkg/domain/`)

Pure types, interfaces, and validation. No I/O, no logging, no goroutines.
Models in `pkg/domain/model/` are also the Firestore wire format (see
CLAUDE.md), so additions there must remain serialisable.

## Quick smell tests

When reviewing or writing code, run these tests in your head:

- *"Could I move this code into the controller / out of the controller
  without changing behavior?"* If yes, it is in the wrong layer.
- *"Does this controller import `repository` or `gollem` or
  `service/slack` for anything other than passing to a usecase
  constructor?"* If yes, push it down.
- *"Does this usecase return `http.StatusBadRequest` or
  `errutil.HandleHTTP`?"* If yes, the layering is leaking up.
- *"If I rewrote the transport (HTTP → gRPC → CLI), how much usecase code
  would I need to change?"* The answer should be "zero".
