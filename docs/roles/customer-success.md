# Role: customer-success

**Post-sale account management.** Reads won deals for context, updates contact/organization details, logs activities (renewals, QBRs, escalations) and notes. Does not create or modify deals — those are handled by AEs or sales ops.

- **Tools exposed:** 19
- **Tokens (tools/list):** 3,623
- **Savings vs baseline:** 5,329 tokens (59.5 %)

## Pipedrive mapping

| Pipedrive | Used here |
|---|---|
| Permission set | Regular with Deals **read**, Contacts edit, Activities full |
| OAuth scopes | `deals:read` · `contacts:full` · `activities:full` |

## Flags

```env
PIPEDRIVE_ALLOW_WRITE=true
PIPEDRIVE_ALLOW_DELETE=false
PIPEDRIVE_ENABLE_ADMIN_TOOLS=false
```

## What it can do

- Read deals (any status) for account context.
- Update person / organization profiles (keep contact data fresh).
- Create/update activities (renewals, check-ins, escalations).
- Read and write notes on deals/persons/orgs.

## What it can't do

- Create or modify deals, lead pipeline stages, or won/lost status.
- Create or delete persons / organizations (can only update existing records).
- Delete anything.
- Touch products, leads, followers or cache admin.

## `PIPEDRIVE_ALLOWED_TOOLS`

```
pipedrive.context.get,pipedrive.deals.list,pipedrive.deals.get,pipedrive.deals.search,pipedrive.persons.list,pipedrive.persons.get,pipedrive.persons.search,pipedrive.persons.update,pipedrive.organizations.list,pipedrive.organizations.get,pipedrive.organizations.search,pipedrive.organizations.update,pipedrive.activities.list,pipedrive.activities.get,pipedrive.activities.create,pipedrive.activities.update,pipedrive.notes.list,pipedrive.notes.create,pipedrive.cache.stats
```
