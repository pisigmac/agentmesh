# Empty states

UX copy and API behavior when no data exists—frontend and API consumers.

## No registered agents

**UI (`/` dashboard):**

> No agents registered yet. Connect Cursor, Claude Code, or another adapter to start routing.

**CTA:** Link to docs [04-AGENTS.md](../docs/04-AGENTS.md) with sample `curl` register command.

**API `GET /v1/agents/discover`:** `{ "agents": [] }` — **200**, not 404.

## No route match

**API `POST /v1/route`:** May return **500** `ROUTE_FAILED` when no candidate matches capability/cost filters.

**UI `/routes`:**

> No routes matched your filters. Register agents with capability `{{capability}}` or relax max cost.

## Circuit closed (healthy)

**UI `/circuits`:** Show green badge "Closed" — not empty; if no circuits ever triggered, show:

> No circuits have opened in the last 7 days. That's a good sign.

## Open circuit

Show agent ID, failure count, link to `GET /v1/circuit/{agent_id}` JSON for support.

## Empty analytics

ClickHouse with zero rows: charts display "No data in range" with date picker default last 24h.

## Unauthorized org

401 page: "Invalid API key or org" — link to settings for key rotation (future).

## Discover with wrong capability

Empty list + hint:

> Try capabilities: `code_gen`, `refactor`, `completion`, `browser_verify`.

## Settings defaults

Org name unset: display workspace ID from header; prompt user to confirm `X-Org-ID`.

## API keys (future UI)

Empty state: "Generate your first API key" with scope explanation.

## Mobile / narrow layout

Empty states stack vertically; preserve CTA button full width.

## Accessibility

Empty illustrations require `alt=""` decorative; status text in headings for screen readers.
