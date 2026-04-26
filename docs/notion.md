# Notion Integration

Shepherd can let the LLM agent search and read Notion content. Access is
scoped per workspace by registering Notion pages or databases as **Sources**
through the WebUI.

## 1. Create a Notion Internal Integration

1. Go to <https://www.notion.so/profile/integrations>.
2. Click **New integration**.
3. Choose the workspace, give it a name (e.g. "Shepherd"), and select the
   **Internal** type.
4. Capabilities: enable at least **Read content**.
5. Save and copy the **Internal Integration Secret**. This is the token
   Shepherd will use.

## 2. Configure Shepherd

Pass the token to the server:

```sh
export SHEPHERD_NOTION_TOKEN=secret_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
shepherd serve ...
```

Or via flag:

```sh
shepherd serve --notion-token=secret_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

When the token is unset, all Notion tools and the Sources WebUI section are
inert (not exposed to the agent or visible as enabled).

## 3. Invite the integration to pages / databases

The Notion API never sees content the integration has not been explicitly
invited to. For each top-level page or database you want Shepherd to index:

1. Open it in Notion.
2. **Share / Connections** → **Add connections** → pick the Shepherd integration.

Sub-pages and rows inherit the parent's connection.

## 4. Register a Source in Shepherd

1. Open the workspace in Shepherd's WebUI.
2. **Settings → Integration → Sources**.
3. Paste the page or database URL (e.g.
   `https://www.notion.so/myws/Project-Plan-1f2e3d4c5b6a7980abcd1234567890ef`).
4. Click **Add source**. Shepherd will:
   - parse the URL,
   - call the Notion API to verify the integration can read the object,
   - record its title and persist the Source.

If verification fails:

- **Forbidden** — the integration was not invited to the page/database.
  Re-do step 3 in Notion.
- **Not found** — the URL is wrong, or the object was deleted.
- **Invalid URL** — Shepherd could not extract a Notion object id from the
  URL.

## 5. Tool Catalog

Adding at least one Source enables the Notion tools for that workspace:

| Tool | What it does |
|---|---|
| `notion_search` | Search Notion content; results are filtered to objects reachable from a registered Source. |
| `notion_get_page` | Fetch a page as Markdown (Notion Markdown Content API, `Notion-Version: 2026-03-11`). With `recursive=true` it walks linked child pages within scope, capped by `max_depth` and `max_pages`. |
| `notion_query_database` | Query rows of a registered database. With `include_body=true` each row's page Markdown is also fetched (capped at 10 bodies/call). |

You can disable Notion entirely for a specific workspace via
**Settings → Integration → Tools**, even when a Source is registered.

## 6. URL formats accepted

- Page: `https://www.notion.so/<workspace>/<slug>-<32hex>`
- Database: `https://www.notion.so/<workspace>/<32hex>?v=<view_id>`
- Bare ID: 32 hex chars or hyphenated UUID

## Notes

- Notion tokens are never logged. Shepherd records only `token_set: true|false`.
- The Markdown Content API does not currently support some object kinds (e.g.
  certain database-row pages); Shepherd surfaces these as
  `notion: markdown endpoint unsupported for this object`.
