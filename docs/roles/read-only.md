# Role: read-only

**Analyst / reporting / executive dashboards.** Pure consumer of CRM data — nothing is created, updated or deleted. Safest configuration when pointing an autonomous agent at a production Pipedrive account.

- **Tools exposed:** 24
- **Tokens (tools/list):** 4,025
- **Savings vs baseline:** 4,927 tokens (55.0 %)

## Pipedrive mapping

| Pipedrive | Used here |
|---|---|
| Permission set | Regular with **no** add/edit/delete checkboxes on any category |
| OAuth scopes | `deals:read` · `contacts:read` · `products:read` · `activities:read` · `leads:read` |

## Flags

```env
PIPEDRIVE_ALLOW_WRITE=false
PIPEDRIVE_ALLOW_DELETE=false
PIPEDRIVE_ENABLE_ADMIN_TOOLS=false
```

All three flags default to false. Even if a hypothetical write tool leaked into the allowlist, the handler would still reject it.

## What it can do

- List / get / search every first-class entity (deals, persons, organizations, products, leads, activities).
- Read followers lists.
- Read product catalog, attached deal products, and notes.
- Read cache stats (introspection only).

## What it can't do

- Anything that mutates state, period.
- Manage followers.
- Touch the cache.

## `PIPEDRIVE_ALLOWED_TOOLS`

```
pipedrive.context.get,pipedrive.deals.list,pipedrive.deals.get,pipedrive.deals.search,pipedrive.deals.products.list,pipedrive.deals.followers.list,pipedrive.persons.list,pipedrive.persons.get,pipedrive.persons.search,pipedrive.persons.followers.list,pipedrive.organizations.list,pipedrive.organizations.get,pipedrive.organizations.search,pipedrive.organizations.followers.list,pipedrive.products.list,pipedrive.products.get,pipedrive.products.search,pipedrive.leads.list,pipedrive.leads.get,pipedrive.leads.search,pipedrive.activities.list,pipedrive.activities.get,pipedrive.notes.list,pipedrive.cache.stats
```
