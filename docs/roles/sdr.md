# Role: sdr

**Top-of-funnel / BDR.** Sales Development Reps qualify leads, enrich contact data, schedule first touches. They work upstream of deals — deal tools are deliberately excluded to keep the model focused on the lead-to-qualified-deal handoff.

- **Tools exposed:** 24
- **Tokens (tools/list):** 3,473
- **Savings vs baseline:** 4,292 tokens (55.3 %)

## Pipedrive mapping

| Pipedrive | Used here |
|---|---|
| Permission set | Regular with Leads / Contacts / Activities full; Deals **off** |
| OAuth scopes | `leads:full` · `contacts:full` · `activities:full` |

## Flags

```env
PIPEDRIVE_ALLOW_WRITE=true
PIPEDRIVE_ALLOW_DELETE=true
PIPEDRIVE_ENABLE_ADMIN_TOOLS=false
```

`ALLOW_DELETE=true` because SDRs routinely discard disqualified leads; the allowlist keeps it scoped to `pipedrive.leads.delete` (no other `*.delete` is exposed).

## What it can do

- Full CRUD on leads (including delete).
- Create/update persons and organizations (enrichment).
- Log activities (calls, emails, meetings) and notes against leads/persons/orgs.

## What it can't do

- Touch deals in any way (`pipedrive.deals.*` not registered).
- Touch products.
- Manage followers on any entity.
- Clear/invalidate the cache.

## `PIPEDRIVE_ALLOWED_TOOLS`

```
pipedrive.context.get,pipedrive.leads.list,pipedrive.leads.get,pipedrive.leads.search,pipedrive.leads.create,pipedrive.leads.update,pipedrive.leads.delete,pipedrive.persons.list,pipedrive.persons.get,pipedrive.persons.search,pipedrive.persons.create,pipedrive.persons.update,pipedrive.organizations.list,pipedrive.organizations.get,pipedrive.organizations.search,pipedrive.organizations.create,pipedrive.organizations.update,pipedrive.activities.list,pipedrive.activities.get,pipedrive.activities.create,pipedrive.activities.update,pipedrive.notes.list,pipedrive.notes.create,pipedrive.cache.stats
```
