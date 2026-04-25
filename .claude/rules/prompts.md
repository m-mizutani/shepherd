---
paths:
  - "**/prompt/**/*.go"
  - "**/prompt/**/*.md"
  - "**/*prompt*.go"
---

# LLM Prompts

- Place every LLM prompt as a `.md` file under a `prompt/` directory that sits alongside the Go file consuming it (e.g. `pkg/usecase/prompt/mention.md` next to `pkg/usecase/llm.go`).
- Embed the prompt files with `//go:embed` and parse them through `text/template` (`template.New(...).Parse(...)` or `template.ParseFS`). Cache the parsed `*template.Template` at package init / `sync.Once`; do not re-parse per request.
- Inject dynamic values exclusively via template parameters and `{{ . }}` actions. **Never** build prompts with `fmt.Sprintf`, `strings.ReplaceAll`, `+` concatenation, or any other string-level manipulation. If a value needs conditional inclusion, use `{{ if }}` / `{{ range }}` inside the template — not Go-side string assembly.
- Pass a single typed struct (`type MentionPromptInput struct { ... }`) to `tmpl.Execute`. Keep the struct definition next to the embed so the template's expected fields are discoverable.
- Markdown is for humans first: keep prompts readable with headings, bullet lists, and code fences. The template engine treats it as plain text, so no escaping is required beyond `{{` / `}}`.
- Every prompt template must have a `*_test.go` covering `tmpl.Execute` with at least:
  1. A representative happy-path input that exercises every template action (`{{ .Field }}`, `{{ range }}`, `{{ if }}`).
  2. Edge cases for empty / nil collections so the rendered output stays well-formed.
  3. A golden assertion on the rendered string (use `gt.Equal` or `gt.S(t, got).Contains(...)`) so prompt regressions are caught at CI time.
- When updating a prompt, edit the `.md` file and adjust the input struct + test — do not patch the rendered string in Go code.
