Searches and reads Notion content scoped to the **registered Sources** of this workspace. A Source is a Notion page or database that the workspace owner has explicitly registered via the WebUI; the tools never see Notion content outside those roots, so picking the right Source matters more than crafting the perfect query.

{{- if .Sources }}

Registered Notion sources for this workspace:
{{- range .Sources }}
- `{{ .ID }}` — {{ .Title }} ({{ .ObjectType }}){{ if .UserDescription }} — {{ .UserDescription }}{{ end }}
{{- end }}

Workflow: call `notion_list_sources` to confirm IDs at runtime, then `notion_search` (full-text within sources) or `notion_query_database` (structured rows from a database source). Use `notion_get_page` to read a specific page's body.

{{- else }}

**No Notion sources are currently registered for this workspace.** `notion_search`, `notion_query_database`, and `notion_get_page` will return empty results. If a workspace owner has not registered the relevant page/database via the WebUI, do not propose Notion-based investigation subtasks.
{{- end }}
