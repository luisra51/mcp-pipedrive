# Role-based MCP configurations

This folder contains ready-to-paste configurations that expose only the subset of MCP tools a given Pipedrive role actually needs. Because `PIPEDRIVE_ALLOWED_TOOLS` filters at **registration time** (not just at the handler), unlisted tools never appear in `tools/list` — a direct reduction in the tokens the LLM consumes every time it loads the tool catalog.

## Why this matters

Every time an LLM connects to this MCP it ingests the full tool schema. Listing all 58 tools costs ~7,800 tokens (cl100k_base). Scoping the surface to a single role can cut that by **20-65 %** without losing any real capability for that role.

| Role | Tools | Tokens | Saved | Doc |
|---|---:|---:|---:|---|
| [admin](roles/admin.md) | 58 | 7,765 | — | full surface |
| [sales-manager](roles/sales-manager.md) | 47 | 6,212 | 20.0 % | team oversight |
| [sales-rep](roles/sales-rep.md) | 33 | 4,682 | 39.7 % | account executive |
| [sdr](roles/sdr.md) | 24 | 3,473 | 55.3 % | top-of-funnel / BDR |
| [customer-success](roles/customer-success.md) | 19 | 2,816 | 63.7 % | post-sale account mgmt |
| [read-only](roles/read-only.md) | 28 | 3,995 | 48.6 % | analyst / reporting |
| [marketing](roles/marketing.md) | 22 | 3,302 | 57.5 % | campaigns / nurturing |

> Token counts measured with cl100k_base (OpenAI tokenizer). Claude's tokenizer is typically within ±10 % of that.

## How Pipedrive roles map to this MCP

Pipedrive ships **two built-in user types** — `admin` and `regular` — and lets higher plans define custom permission sets:

| Plan | Custom permission sets |
|---|---|
| Lite / Growth | — (admin + regular only) |
| Premium | up to 15 |
| Ultimate | up to 25 |
| Enterprise | up to 150 |

Pipedrive itself does **not** ship role archetypes (SDR, CS, etc.) — each account defines its own. The configurations here are common industry archetypes aligned with Pipedrive's [permission set](https://support.pipedrive.com/en/article/permission-sets) and [OAuth scope](https://pipedrive.readme.io/docs/marketplace-scopes-and-permissions-explanations) structure:

| Pipedrive OAuth scope | MCP tool prefix |
|---|---|
| `deals:read` / `deals:full` | `pipedrive.deals.*`, `pipedrive.deals.products.*`, `pipedrive.deals.followers.*` |
| `contacts:read` / `contacts:full` | `pipedrive.persons.*`, `pipedrive.organizations.*`, their `.followers.*` |
| `products:read` / `products:full` | `pipedrive.products.*` |
| `activities:read` / `activities:full` | `pipedrive.activities.*` |
| `leads:read` / `leads:full` | `pipedrive.leads.*` |
| any `:full` scope | `pipedrive.notes.*` (notes require a parent entity) |

Note that Pipedrive's **visibility groups** (row-level data visibility) are orthogonal to the MCP allowlist — the allowlist restricts *which operations* the LLM can invoke; the underlying API token or OAuth scope still enforces *which rows* come back. Always use a Pipedrive credential that matches the role's intended scope; the MCP does not widen access beyond what the credential grants.

## How to apply a role config

1. Open the role's file under `roles/`.
2. Copy the `PIPEDRIVE_ALLOWED_TOOLS` value.
3. Set the recommended `PIPEDRIVE_ALLOW_WRITE` / `PIPEDRIVE_ALLOW_DELETE` flags.
4. (Optional) Keep `PIPEDRIVE_ENABLE_ADMIN_TOOLS=false` unless the caller needs `cache.clear` / `cache.invalidate`.

Example for `sales-rep`:

```env
PIPEDRIVE_ALLOWED_TOOLS=pipedrive.context.get,pipedrive.deals.list,pipedrive.deals.get,…
PIPEDRIVE_ALLOW_WRITE=true
PIPEDRIVE_ALLOW_DELETE=false
PIPEDRIVE_ENABLE_ADMIN_TOOLS=false
```

## Defense in depth

Even with a tight allowlist, the handler-level guards (`ensureToolAllowed`) still run on every call. A role that omits `pipedrive.deals.delete` from the allowlist won't see it in `tools/list`, **and** if something (a misconfiguration, a renamed tool, a direct JSON-RPC probe) ever surfaced the name, the handler would still block writes/deletes without the corresponding flag. The two layers compose.

## Sources

- [Pipedrive permission sets (official)](https://support.pipedrive.com/en/article/permission-sets)
- [Pipedrive user types](https://support.pipedrive.com/en/article/types-of-users-in-pipedrive)
- [Pipedrive visibility groups](https://support.pipedrive.com/en/article/visibility-groups)
- [Pipedrive OAuth scopes](https://pipedrive.readme.io/docs/marketplace-scopes-and-permissions-explanations)
