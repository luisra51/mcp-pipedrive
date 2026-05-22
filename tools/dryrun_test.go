//go:build dryrun

package tools

// Live read-only sweep against a real Pipedrive workspace. Excluded from
// default builds via the `dryrun` build tag — run with:
//
//   go test -tags dryrun -run DryRun -v ./tools/...
//
// Requires PIPEDRIVE_API_TOKEN in the environment. Performs no writes.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"mcp-pipedrive/pipedrive"
)

func liveCtx(t *testing.T) context.Context {
	t.Helper()
	token := strings.TrimSpace(os.Getenv("PIPEDRIVE_API_TOKEN"))
	if token == "" {
		t.Skip("PIPEDRIVE_API_TOKEN not set; skipping dry run")
	}
	domain := strings.TrimSpace(os.Getenv("PIPEDRIVE_DOMAIN"))
	if domain == "" {
		domain = "api.pipedrive.com"
	}
	cfg := pipedrive.Config{
		Domain:   domain,
		APIToken: token,
		Timeout:  30 * time.Second,
	}
	client := &pipedrive.Client{
		BaseURL:  "https://" + domain,
		HTTP:     &http.Client{Timeout: cfg.Timeout, Transport: http.DefaultTransport},
		APIToken: token,
		AuthMode: pipedrive.AuthModeToken,
	}
	ctx := pipedrive.WithConfig(context.Background(), cfg)
	ctx = pipedrive.WithClient(ctx, client)
	return ctx
}

// dump pretty-prints a tool result truncated to ~600 chars so the log stays
// readable.
func dump(t *testing.T, label string, v any) {
	t.Helper()
	b, _ := json.MarshalIndent(v, "", "  ")
	s := string(b)
	if len(s) > 800 {
		s = s[:800] + "\n…(truncated)"
	}
	t.Logf("%s:\n%s", label, s)
}

// countItems pulls the slice under a known key (deals/persons/...) and returns
// its length. Returns -1 if the shape is unexpected.
func countItems(result any, key string) int {
	m, ok := result.(map[string]any)
	if !ok {
		return -1
	}
	data, ok := m["data"].(map[string]any)
	if !ok {
		return -1
	}
	arr, ok := data[key].([]any)
	if !ok {
		// Try concrete slice types we emit from NormalizeXList helpers.
		switch arr2 := data[key].(type) {
		case []pipedrive.NormalizedDeal:
			return len(arr2)
		case []pipedrive.NormalizedPerson:
			return len(arr2)
		case []pipedrive.NormalizedOrganization:
			return len(arr2)
		case []pipedrive.NormalizedActivity:
			return len(arr2)
		case []pipedrive.NormalizedProduct:
			return len(arr2)
		case []pipedrive.NormalizedLead:
			return len(arr2)
		case []pipedrive.NormalizedNote:
			return len(arr2)
		case []pipedrive.NormalizedFilter:
			return len(arr2)
		}
		return -1
	}
	return len(arr)
}

// firstFilterIDByType returns the first filter id matching the given Pipedrive
// filter `type`. Pipedrive uses these type strings on /filters:
//   deals, people, org, product, activity, lead
func firstFilterIDByType(result any, kind string) int64 {
	for _, f := range filtersOfType(result, kind) {
		return f.ID
	}
	return 0
}

// filtersOfType returns every active filter matching `kind`.
func filtersOfType(result any, kind string) []pipedrive.NormalizedFilter {
	m, ok := result.(map[string]any)
	if !ok {
		return nil
	}
	data, ok := m["data"].(map[string]any)
	if !ok {
		return nil
	}
	filters, ok := data["filters"].([]pipedrive.NormalizedFilter)
	if !ok {
		return nil
	}
	out := make([]pipedrive.NormalizedFilter, 0, len(filters))
	for _, f := range filters {
		if f.Active && f.Type == kind {
			out = append(out, f)
		}
	}
	return out
}

func TestDryRun_FiltersAndAllLists(t *testing.T) {
	ctx := liveCtx(t)

	// Use the API's hard cap to maximize the chance of catching a filter
	// that's actually narrowing — at limit=20 a filter matching 1000 items
	// would still return 20 and look "no-op".
	const pageLimit = 500

	// 1. filters.list — discover saved filters.
	filtersRes, err := filtersList(ctx, FiltersListParams{CacheMode: "bypass"})
	if err != nil {
		t.Fatalf("filters.list: %v", err)
	}
	filterCount := countItems(filtersRes, "filters")
	t.Logf("filters.list returned %d filters", filterCount)

	type resource struct {
		name       string
		key        string
		filterType string
		unfiltered func() (any, error)
		withFilter func(id int64) (any, error)
		withSince  func(since string) (any, error)
	}

	since := time.Now().UTC().Add(-7 * 24 * time.Hour).Format(time.RFC3339)

	resources := []resource{
		{
			name: "deals.list", key: "deals", filterType: "deals",
			unfiltered: func() (any, error) {
				return dealsList(ctx, DealsListParams{Limit: pageLimit, CacheMode: "bypass"})
			},
			withFilter: func(id int64) (any, error) {
				return dealsList(ctx, DealsListParams{Limit: pageLimit, FilterID: id, CacheMode: "bypass"})
			},
			withSince: func(s string) (any, error) {
				return dealsList(ctx, DealsListParams{Limit: pageLimit, UpdatedSince: s, CacheMode: "bypass"})
			},
		},
		{
			name: "persons.list", key: "persons", filterType: "people",
			unfiltered: func() (any, error) {
				return personsList(ctx, PersonsListParams{Limit: pageLimit, CacheMode: "bypass"})
			},
			withFilter: func(id int64) (any, error) {
				return personsList(ctx, PersonsListParams{Limit: pageLimit, FilterID: id, CacheMode: "bypass"})
			},
			withSince: func(s string) (any, error) {
				return personsList(ctx, PersonsListParams{Limit: pageLimit, UpdatedSince: s, CacheMode: "bypass"})
			},
		},
		{
			name: "organizations.list", key: "organizations", filterType: "org",
			unfiltered: func() (any, error) {
				return organizationsList(ctx, OrganizationsListParams{Limit: pageLimit, CacheMode: "bypass"})
			},
			withFilter: func(id int64) (any, error) {
				return organizationsList(ctx, OrganizationsListParams{Limit: pageLimit, FilterID: id, CacheMode: "bypass"})
			},
			withSince: func(s string) (any, error) {
				return organizationsList(ctx, OrganizationsListParams{Limit: pageLimit, UpdatedSince: s, CacheMode: "bypass"})
			},
		},
		{
			name: "activities.list", key: "activities", filterType: "activity",
			unfiltered: func() (any, error) {
				return activitiesList(ctx, ActivitiesListParams{Limit: pageLimit, CacheMode: "bypass"})
			},
			withFilter: func(id int64) (any, error) {
				return activitiesList(ctx, ActivitiesListParams{Limit: pageLimit, FilterID: id, CacheMode: "bypass"})
			},
			withSince: func(s string) (any, error) {
				return activitiesList(ctx, ActivitiesListParams{Limit: pageLimit, UpdatedSince: s, CacheMode: "bypass"})
			},
		},
		{
			name: "products.list", key: "products", filterType: "product",
			unfiltered: func() (any, error) {
				return productsList(ctx, ProductsListParams{Limit: pageLimit, CacheMode: "bypass"})
			},
			withFilter: func(id int64) (any, error) {
				return productsList(ctx, ProductsListParams{Limit: pageLimit, FilterID: id, CacheMode: "bypass"})
			},
			withSince: func(s string) (any, error) {
				return productsList(ctx, ProductsListParams{Limit: pageLimit, UpdatedSince: s, CacheMode: "bypass"})
			},
		},
		{
			name: "leads.list", key: "leads", filterType: "lead",
			unfiltered: func() (any, error) {
				return leadsList(ctx, LeadsListParams{Limit: pageLimit, CacheMode: "bypass"})
			},
			withFilter: func(id int64) (any, error) {
				return leadsList(ctx, LeadsListParams{Limit: pageLimit, FilterID: id, CacheMode: "bypass"})
			},
			withSince: nil, // /leads has no date params in v1
		},
		{
			name: "notes.list", key: "notes", filterType: "", // filters.type for notes is not exposed in the UI
			unfiltered: func() (any, error) {
				return notesList(ctx, NotesListParams{Limit: pageLimit, CacheMode: "bypass"})
			},
			withFilter: nil, // no notes filter type to match
			withSince: func(s string) (any, error) {
				return notesList(ctx, NotesListParams{
					Limit:     pageLimit,
					StartDate: time.Now().UTC().Add(-7 * 24 * time.Hour).Format("2006-01-02"),
					CacheMode: "bypass",
				})
			},
		},
	}

	for _, r := range resources {
		t.Run(r.name, func(t *testing.T) {
			res, err := r.unfiltered()
			if err != nil {
				t.Errorf("unfiltered %s: %v", r.name, err)
				return
			}
			baseCount := countItems(res, r.key)
			t.Logf("%s unfiltered (limit=%d) → %d items", r.name, pageLimit, baseCount)

			if r.withFilter != nil && r.filterType != "" {
				// Scan every active filter of this type and report the
				// narrowest non-empty result. Proves filter_id is doing
				// real work even when the first one is permissive.
				candidates := filtersOfType(filtersRes, r.filterType)
				if len(candidates) == 0 {
					t.Logf("%s: no active saved filter of type=%q — skipping", r.name, r.filterType)
				} else {
					t.Logf("%s: scanning %d active %q filters for narrowing effect", r.name, len(candidates), r.filterType)
					type filterHit struct {
						id    int64
						name  string
						count int
					}
					var (
						hits       []filterHit
						narrowest  *filterHit
						probed     int
						maxToProbe = 25 // bound the API calls per type
					)
					for _, f := range candidates {
						if probed >= maxToProbe {
							break
						}
						probed++
						fr, err := r.withFilter(f.ID)
						if err != nil {
							t.Logf("  filter_id=%d (%s): error %v", f.ID, f.Name, err)
							continue
						}
						c := countItems(fr, r.key)
						h := filterHit{id: f.ID, name: f.Name, count: c}
						hits = append(hits, h)
						if c < baseCount && (narrowest == nil || c < narrowest.count) {
							hCopy := h
							narrowest = &hCopy
						}
					}
					// Log compact line per filter probed.
					for _, h := range hits {
						mark := "  "
						if narrowest != nil && h.id == narrowest.id {
							mark = "★ "
						}
						t.Logf("%s filter_id=%-5d → %4d items  (%s)", mark, h.id, h.count, h.name)
					}
					if narrowest != nil {
						t.Logf("%s NARROWING PROVEN: filter_id=%d (%s) returned %d < %d unfiltered",
							r.name, narrowest.id, narrowest.name, narrowest.count, baseCount)
					} else if baseCount > 0 {
						t.Logf("%s: none of %d probed filters narrowed below %d items (workspace data may be small or filters permissive)",
							r.name, probed, baseCount)
					}
				}
			}

			if r.withSince != nil {
				sinceRes, err := r.withSince(since)
				if err != nil {
					t.Errorf("with since %s: %v", r.name, err)
				} else {
					sc := countItems(sinceRes, r.key)
					t.Logf("%s with date-bound (since 7d) → %d items (vs %d unfiltered)", r.name, sc, baseCount)
				}
			}
		})
	}

	fmt.Println() // visual gap in log
}

// Negative check: applying a filter of the wrong type must surface an upstream
// error via wrapAPIError rather than silently returning unfiltered data.
func TestDryRun_FilterIDTypeMismatch(t *testing.T) {
	ctx := liveCtx(t)
	filtersRes, err := filtersList(ctx, FiltersListParams{CacheMode: "bypass"})
	if err != nil {
		t.Fatalf("filters.list: %v", err)
	}
	personFilter := firstFilterIDByType(filtersRes, "people")
	if personFilter == 0 {
		t.Skip("no active person filter available — cannot run mismatch test")
	}
	_, err = dealsList(ctx, DealsListParams{Limit: 5, FilterID: personFilter, CacheMode: "bypass"})
	if err == nil {
		t.Errorf("expected error applying people-typed filter_id=%d on deals.list; got none", personFilter)
	} else {
		t.Logf("mismatch correctly errored: %v", err)
	}
}
