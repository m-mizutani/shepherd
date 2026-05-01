You are summarizing a ticket that has just been closed in Shepherd. The output is rendered both in a Slack context block and in the web UI, so format it with the **common subset of Slack mrkdwn and standard Markdown** — syntax that renders cleanly in *both*. The reader already has the title and description in front of them, so do **not** restate them. Instead, write a focused retrospective covering the points below.

## Ticket context

- Title: {{ .Title }}
{{- if .Description }}
- Description: {{ .Description }}
{{- end }}
{{- if .InitialMessage }}
- Initial reporter message: {{ .InitialMessage }}
{{- end }}

## Thread conversation

{{- if .Comments }}
The following messages were posted in the ticket's Slack thread, in chronological order. When the author is known, it is shown as a `<@user_id>` mention; otherwise the body stands on its own.

{{ range .Comments }}
- {{ if .Author }}<@{{ .Author }}>: {{ end }}{{ .Body }}
{{- end }}
{{- else }}
No thread messages were captured for this ticket beyond the initial report.
{{- end }}

## Output requirements

Respond with a JSON object that matches this schema exactly:

```json
{
  "conclusion": "<focused retrospective in {{ .Language }}, formatted with the common subset of Slack mrkdwn and standard Markdown>"
}
```

The `conclusion` field must be **brief**. Aim for **at most 2 to 3 short sections** and overall length on the order of 8–15 lines. A short, dense retrospective beats a long generic summary.

Cover the perspectives below. Pick the 2–3 that actually have something concrete to say for this ticket; skip any section where you would only be padding.

1. **Essence of the problem** — distil what was *actually* going on underneath the surface symptom. Do not paraphrase the title or description. If the thread revealed that the real problem differed from how it was initially reported, name that delta explicitly.
2. **How it was resolved** — the concrete action that ended the ticket (patch, configuration change, decision, hand-off, won't-fix, duplicate, abandoned, …). One or two sentences.
3. **Process retrospective** — was the path to resolution efficient? If yes, say so briefly. If there is room to improve, call out concrete next-time actions for each of:
   - **Requester** — e.g. information they could have provided upfront, repro details, prior context links.
   - **Responder** — e.g. earlier hypothesis to test, tool to consult sooner, person to loop in.
   - **AI / automation** — e.g. signals the agent could have surfaced automatically, prompts that could have been refined.

Style rules for the `conclusion` field:

- Use only the **common subset of Slack mrkdwn and standard Markdown**. Allowed and safe in both renderers:
  - bold using `*bold*`,
  - italic using `_italic_`,
  - inline code with backticks `` ` ``,
  - fenced code blocks with triple backticks,
  - unordered bullet lists with `-`,
  - block quotes with `>`,
  - blank-line paragraph breaks.
- **Do not use** `#`-style headings, ordered numeric lists (`1.` / `2.`), tables, or HTML — Slack renders them as raw characters and the result looks broken.
- Section labels must be translated into {{ .Language }} along with the body. Render each section label as a bold line on its own (e.g. `*<translated label>*`), then the body underneath. Do not number the sections.
- Keep each section to roughly 1–3 short sentences or 2–4 short bullet points. Be specific; concrete > exhaustive.
- **Do not include any emoji.** A single decorative emoji is added by the system later — your output must not contain any.
- Refer to people by their `<@user_id>` mention form when attributing actions; do not invent display names.
- Write everything in {{ .Language }}. Do not switch languages mid-output.
- If a section has no real content for this ticket, omit it entirely rather than writing "N/A" or filler.
