# Role: sales-manager

**Team oversight.** A sales manager monitors the pipeline, reassigns ownership, cleans up data, and runs end-of-period reviews. They need write/delete across sales entities but do not manage the product catalog or the cache.

- **Tools exposed:** 47
- **Tokens (tools/list):** 7,987
- **Savings vs baseline:** 965 tokens (10.8 %)

## Pipedrive mapping

| Pipedrive | Used here |
|---|---|
| Permission set | Regular with most Deals / Contacts / Leads / Activities add-edit-delete checkboxes on |
| OAuth scopes | `deals:full` · `contacts:full` · `activities:full` · `leads:full` · `products:read` |

Products are read-only — catalog management stays with admins.

## Flags

```env
PIPEDRIVE_ALLOW_WRITE=true
PIPEDRIVE_ALLOW_DELETE=true
PIPEDRIVE_ENABLE_ADMIN_TOOLS=false
```

## What it can do

- Full CRUD on deals, persons, organizations, leads and activities (including deletes for cleanup).
- Attach / detach products on deals, manage deal/person/org followers, write notes.
- Read the product catalog to pick items for deals.
- Read cache stats for observability.

## What it can't do

- Create, update or delete products (catalog stays with admins).
- Clear or invalidate the cache.

## `PIPEDRIVE_ALLOWED_TOOLS`

```
pipedrive.context.get,pipedrive.deals.list,pipedrive.deals.get,pipedrive.deals.search,pipedrive.deals.create,pipedrive.deals.update,pipedrive.deals.delete,pipedrive.deals.products.list,pipedrive.deals.products.attach,pipedrive.deals.products.update,pipedrive.deals.products.detach,pipedrive.deals.followers.list,pipedrive.deals.followers.add,pipedrive.deals.followers.remove,pipedrive.persons.list,pipedrive.persons.get,pipedrive.persons.search,pipedrive.persons.create,pipedrive.persons.update,pipedrive.persons.followers.list,pipedrive.persons.followers.add,pipedrive.persons.followers.remove,pipedrive.organizations.list,pipedrive.organizations.get,pipedrive.organizations.search,pipedrive.organizations.create,pipedrive.organizations.update,pipedrive.organizations.followers.list,pipedrive.organizations.followers.add,pipedrive.organizations.followers.remove,pipedrive.products.list,pipedrive.products.get,pipedrive.products.search,pipedrive.leads.list,pipedrive.leads.get,pipedrive.leads.search,pipedrive.leads.create,pipedrive.leads.update,pipedrive.leads.delete,pipedrive.activities.list,pipedrive.activities.get,pipedrive.activities.create,pipedrive.activities.update,pipedrive.activities.delete,pipedrive.notes.list,pipedrive.notes.create,pipedrive.cache.stats
```
