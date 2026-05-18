# Role: sales-rep

**Account executive running their own book.** Creates and moves deals, manages contacts, attaches products, logs activities and notes. Deletes are off-limits — cleanup happens via the sales manager or admin.

- **Tools exposed:** 33
- **Tokens (tools/list):** 4,682
- **Savings vs baseline:** 3,083 tokens (39.7 %)

## Pipedrive mapping

| Pipedrive | Used here |
|---|---|
| Permission set | Regular with Deals / Contacts / Activities add-edit on, delete **off** |
| OAuth scopes | `deals:full` · `contacts:full` · `activities:full` · `products:read` |

Leads, followers admin and cache admin are excluded to keep the LLM's surface tight around day-to-day AE work.

## Flags

```env
PIPEDRIVE_ALLOW_WRITE=true
PIPEDRIVE_ALLOW_DELETE=false
PIPEDRIVE_ENABLE_ADMIN_TOOLS=false
```

## What it can do

- Create/update deals, persons, organizations, activities, notes.
- Attach / update / detach products on their own deals.
- Manage deal followers (CC collaborators).
- Read the product catalog.

## What it can't do

- Delete anything (no `*.delete` tools registered).
- Manage leads (`pipedrive.leads.*` not registered).
- Add/remove followers on persons or organizations.
- Create/update products.
- Clear/invalidate the cache.

## `PIPEDRIVE_ALLOWED_TOOLS`

```
pipedrive.context.get,pipedrive.deals.list,pipedrive.deals.get,pipedrive.deals.search,pipedrive.deals.create,pipedrive.deals.update,pipedrive.deals.products.list,pipedrive.deals.products.attach,pipedrive.deals.products.update,pipedrive.deals.products.detach,pipedrive.deals.followers.list,pipedrive.deals.followers.add,pipedrive.deals.followers.remove,pipedrive.persons.list,pipedrive.persons.get,pipedrive.persons.search,pipedrive.persons.create,pipedrive.persons.update,pipedrive.organizations.list,pipedrive.organizations.get,pipedrive.organizations.search,pipedrive.organizations.create,pipedrive.organizations.update,pipedrive.products.list,pipedrive.products.get,pipedrive.products.search,pipedrive.activities.list,pipedrive.activities.get,pipedrive.activities.create,pipedrive.activities.update,pipedrive.notes.list,pipedrive.notes.create,pipedrive.cache.stats
```
