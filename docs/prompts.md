# Prompts

The **Prompts** settings page lets workspace admins customize the system
prompts that Shepherd uses when reasoning about tickets. The current page is
at `/ws/{workspaceId}/settings/prompts` (Settings → Prompts in the left nav).

For now there is exactly one customizable slot:

| Slot     | When it runs                                                                         |
| -------- | ------------------------------------------------------------------------------------ |
| `triage` | Planner system prompt for the triage agent — invoked when a Slack message becomes a new ticket. |

Additional slots (Summarize, Reply Drafts, etc.) are planned. The data model,
API, and UI all key by `PromptID`, so adding a new slot only requires
registering it on both sides.

## Editing a prompt

1. Open Settings → Prompts and click the **Triage** card.
2. Edit the Markdown source in the editor. The footer lists the template
   variables you can interpolate via Go `text/template` syntax — e.g.
   `{{ .Title }}`, `{{ .Description }}`, `{{ .InitialMessage }}`,
   `{{ .Reporter }}`.
3. Click **Save**.

When you save, Shepherd:

- Parses the content as a Go `text/template` with `missingkey=error`.
- Executes it against representative test inputs to catch typos in field
  names (`{{ .NonExistent }}`) and bad pipeline targets
  (`{{ range .Title }}…{{ end }}`).
- Rejects the save with **422 Unprocessable Entity** if either step fails.
  The UI shows the underlying parse / execute error so you can fix the
  template without leaving the page.

If a save fails because **another user already saved a newer version** while
you were editing, Shepherd returns **409 Conflict**. The UI tells you which
version is now current and offers "Discard my edits and reload" so you can
re-apply your changes against the latest content. There is no implicit
last-write-wins.

## History and Restore

Every save appends a new immutable version. Click **History** in the editor
header to open the side panel:

- The left column lists every saved version (newest first), with the author,
  timestamp, and additions/deletions vs. the previous version.
- Clicking a version compares it (left) against the live current version
  (right). The diff is line-level, similar to GitHub.
- **Restore** copies the chosen version's content into a brand-new version on
  top of `current`. Versions are append-only — the version number always
  moves forward, even on restore.

History is retained indefinitely.

## Fallback safety

If at runtime the workspace's saved prompt fails to render (for instance,
the embedded default's variable shape changes in a future release in a way
the override didn't account for), the triage agent logs the failure and
falls back to the embedded default for that turn. Triage never stops
because of a bad override.
