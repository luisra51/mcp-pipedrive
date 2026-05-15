# Mailbox tools

Four Pipedrive Mailbox API endpoints are wrapped:

| Tool | Underlying endpoint | Data scope | Default TTL |
| --- | --- | --- | --- |
| `pipedrive.mail.threads.list` | `GET /v1/mailbox/mailThreads` | **Token owner only** — just the user's connected mailbox | `MAIL_LIST_TTL` 60s |
| `pipedrive.mail.threads.get` | `GET /v1/mailbox/mailThreads/{id}` (+`/mailMessages`) | **Token owner only** | `MAIL_LIST_TTL` 60s |
| `pipedrive.mail.messages.get` | `GET /v1/mailbox/mailMessages/{id}?include_body=1` | **Cross-user** — any message id resolves regardless of owning mailbox | `MAIL_MESSAGE_TTL` 24h |
| `pipedrive.deals.mail.list` | `GET /v1/deals/{id}/mailMessages` + per-message body backfill | **Cross-user** — aggregates every teammate's mail linked to that deal | `MAIL_LIST_TTL` 60s on the listing; bodies cached at `MAIL_MESSAGE_TTL` |

The mailbox endpoints live at bare `/v1/` (not `/api/v1/`), so they use the `V1Legacy` API version. The deal-mail endpoint accepts both prefixes — we route it through the same legacy version for cache-namespace consistency.

## Why the long body TTL?

Pipedrive's message bodies are immutable once delivered: the bytes never change after a message reaches your account. The other fields on a message (`read_flag`, `deleted_flag`, `update_time`) can drift, so a 24h cache hit on `pipedrive.mail.messages.get` will return mutable fields that may lag by up to a day.

To force a refresh when an agent needs a fresh `read_flag`:

```json
{
  "name": "pipedrive.cache.invalidate",
  "arguments": {
    "path": "/v1/mailbox/mailMessages/128140",
    "method": "GET"
  }
}
```

Or wipe every cached mailbox entry at once:

```json
{
  "name": "pipedrive.cache.invalidate",
  "arguments": { "path_prefix": "/v1/mailbox/" }
}
```

(See `pipedrive.cache.invalidate` documentation for the exact argument shape — it understands both single paths and prefixes.)

## Filter recipes

**What needs my attention right now?** (single call, ~5 KB response on a busy inbox)

```json
{
  "name": "pipedrive.mail.threads.list",
  "arguments": {
    "since": "2026-05-13T17:55:00Z",
    "unread_only": true,
    "fields": "compact"
  }
}
```

**Whole deal conversation across the team** (one call gets every email anyone on the team sent or received on a deal, bodies included)

```json
{
  "name": "pipedrive.deals.mail.list",
  "arguments": {
    "deal_id": 84369,
    "include_bodies": true,
    "max_bodies": 50,
    "body_format": "text"
  }
}
```

`body_format: "text"` is the default — HTML is stripped via `jaytaylor/html2text`, which preserves link URLs as `label ( url )` so agents can still follow them. Pass `body_format: "html"` for the raw payload or `body_format: "none"` to omit bodies entirely.

## Projection notes

- `fields: "compact"` (default) drops the full `parties` tree and keeps only the primary `from` address as a flat `{name, email}`. Snippet is truncated to 120 chars with an ellipsis. Cuts a 50-thread listing from ~64 KB to ~10 KB.
- `fields: "full"` brings back the `parties` tree (`from`/`to`/`cc`/`bcc`) and the untruncated snippet — use when an agent needs the full recipient context (cc'd parties, etc.).
- Pipedrive bookkeeping fields (`account_id`, `assigned_user_ids`, `external_*_flag`, `mail_link_tracking_*`, `nylas_id`, `s3_bucket*`) are stripped at the boundary in both modes.

## Search is intentionally absent

There is no `pipedrive.mail.search` tool. Pipedrive's v1 API does not expose a server-side mail-content search, and the bbolt cache is request-signature keyed (not content-indexed), so a local cache scan would only see whatever messages the agent already pulled. For now: use `pipedrive.deals.mail.list` to surface the conversation on a specific deal, or `pipedrive.mail.threads.list` with `unread_only` / `since` to find recent activity.
