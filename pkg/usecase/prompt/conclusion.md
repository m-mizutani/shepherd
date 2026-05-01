You are summarizing a ticket that has just been closed in Shepherd. Read the ticket context and the thread conversation, then write a single short paragraph that captures what actually happened: the underlying issue, what was investigated or done, and the outcome / resolution.

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
The following messages were posted in the ticket's Slack thread, in chronological order. Bot-generated messages are marked `[bot]`; everything else is from a human participant.

{{ range .Comments }}
- {{ if .IsBot }}[bot]{{ else }}<@{{ .Author }}>{{ end }}: {{ .Body }}
{{- end }}
{{- else }}
No thread messages were captured for this ticket beyond the initial report.
{{- end }}

## Output requirements

Respond with a JSON object that matches this schema exactly:

```json
{
  "conclusion": "<one short paragraph in {{ .Language }}>"
}
```

Style rules for the `conclusion` field:

- Plain prose. **Do not** use Markdown headings, bullet lists, bold, italics, code fences, or block quotes. Newlines are allowed but use them sparingly.
- **Do not include any emoji.** A single decorative emoji is added by the system later — your output must not contain any.
- Keep it concise (roughly 2 to 5 sentences). Aim for what a teammate skimming the thread later would actually want to read.
- Cover, in order: what the ticket was about, what was investigated or attempted, and how it ended (resolved / wontfix / duplicate / abandoned, etc.). If any of these is unknown from the thread, say so briefly instead of inventing.
- Refer to people by their `<@user_id>` mention form when you need to attribute an action; do not invent display names.
- Write in the language requested above. Do not switch languages mid-paragraph.
