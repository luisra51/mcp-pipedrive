# Role: marketing

**Campaign operator / nurturing.** Enriches contacts, runs lead generation, logs campaign touches. Reads organizations to attach leads to the right account, but does not create companies — that's sales ops territory.

- **Tools exposed:** 22
- **Tokens (tools/list):** 4,279
- **Savings vs baseline:** 4,673 tokens (52.2 %)

## Pipedrive mapping

| Pipedrive | Used here |
|---|---|
| Permission set | Regular with Leads / Contacts edit, Activities full, Deals **off** |
| OAuth scopes | `leads:full` · `contacts:full` · `activities:full` |

## Flags

```env
PIPEDRIVE_ALLOW_WRITE=true
PIPEDRIVE_ALLOW_DELETE=false
PIPEDRIVE_ENABLE_ADMIN_TOOLS=false
```

## What it can do

- Create/update leads during campaigns; update when they convert or disqualify.
- Create/update persons (enrichment from forms, events, imports).
- Update existing organizations (but not create — avoids duplicate company records from marketing funnels).
- Log activities and notes against leads/persons/orgs.

## What it can't do

- Touch deals or deal products.
- Delete anything.
- Manage followers on any entity.
- Touch products or cache admin.

## `PIPEDRIVE_ALLOWED_TOOLS`

```
pipedrive.context.get,pipedrive.persons.list,pipedrive.persons.get,pipedrive.persons.search,pipedrive.persons.create,pipedrive.persons.update,pipedrive.organizations.list,pipedrive.organizations.get,pipedrive.organizations.search,pipedrive.organizations.update,pipedrive.leads.list,pipedrive.leads.get,pipedrive.leads.search,pipedrive.leads.create,pipedrive.leads.update,pipedrive.activities.list,pipedrive.activities.get,pipedrive.activities.create,pipedrive.activities.update,pipedrive.notes.list,pipedrive.notes.create,pipedrive.cache.stats
```
