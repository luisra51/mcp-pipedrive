package tools

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"testing"

	"mcp-pipedrive/pipedrive"
)

// captureTransport records the most recent outgoing request and returns a
// canned empty-list response. Lets us assert which query parameters a list
// handler actually sends upstream without hitting Pipedrive.
type captureTransport struct {
	last *http.Request
}

func (c *captureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	c.last = req
	body := bytes.NewBufferString(`{"success":true,"data":[]}`)
	return &http.Response{
		StatusCode: 200,
		Status:     "200 OK",
		Body:       io.NopCloser(body),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Request:    req,
	}, nil
}

func newTestCtx(t *testing.T) (context.Context, *captureTransport) {
	t.Helper()
	transport := &captureTransport{}
	client := &pipedrive.Client{
		BaseURL:  "https://test.pipedrive.com",
		HTTP:     &http.Client{Transport: transport},
		APIToken: "test-token",
		AuthMode: pipedrive.AuthModeToken,
		// Cache nil → bypasses caching path entirely.
	}
	ctx := pipedrive.WithConfig(context.Background(), pipedrive.Config{})
	ctx = pipedrive.WithClient(ctx, client)
	return ctx, transport
}

func mustQuery(t *testing.T, transport *captureTransport) url.Values {
	t.Helper()
	if transport.last == nil {
		t.Fatal("handler did not issue an HTTP request")
	}
	return transport.last.URL.Query()
}

func assertHas(t *testing.T, q url.Values, key, want string) {
	t.Helper()
	if got := q.Get(key); got != want {
		t.Errorf("query %s = %q, want %q (full query: %v)", key, got, want, q)
	}
}

func assertAbsent(t *testing.T, q url.Values, key string) {
	t.Helper()
	if got, ok := q[key]; ok {
		t.Errorf("query %s should be absent but was %v (full query: %v)", key, got, q)
	}
}

func TestDealsList_QueryPropagation(t *testing.T) {
	ctx, transport := newTestCtx(t)
	if _, err := dealsList(ctx, DealsListParams{
		FilterID:     42,
		UpdatedSince: "2026-05-01T00:00:00Z",
		UpdatedUntil: "2026-05-31T00:00:00Z",
	}); err != nil {
		t.Fatalf("dealsList: %v", err)
	}
	q := mustQuery(t, transport)
	assertHas(t, q, "filter_id", "42")
	assertHas(t, q, "updated_since", "2026-05-01T00:00:00Z")
	assertHas(t, q, "updated_until", "2026-05-31T00:00:00Z")

	ctx, transport = newTestCtx(t)
	if _, err := dealsList(ctx, DealsListParams{}); err != nil {
		t.Fatalf("dealsList zero: %v", err)
	}
	q = mustQuery(t, transport)
	assertAbsent(t, q, "filter_id")
	assertAbsent(t, q, "updated_since")
	assertAbsent(t, q, "updated_until")
}

func TestPersonsList_QueryPropagation(t *testing.T) {
	ctx, transport := newTestCtx(t)
	if _, err := personsList(ctx, PersonsListParams{
		FilterID:     7,
		UpdatedSince: "2026-01-01T00:00:00Z",
		UpdatedUntil: "2026-12-31T00:00:00Z",
	}); err != nil {
		t.Fatalf("personsList: %v", err)
	}
	q := mustQuery(t, transport)
	assertHas(t, q, "filter_id", "7")
	assertHas(t, q, "updated_since", "2026-01-01T00:00:00Z")
	assertHas(t, q, "updated_until", "2026-12-31T00:00:00Z")

	ctx, transport = newTestCtx(t)
	if _, err := personsList(ctx, PersonsListParams{}); err != nil {
		t.Fatalf("personsList zero: %v", err)
	}
	q = mustQuery(t, transport)
	assertAbsent(t, q, "filter_id")
	assertAbsent(t, q, "updated_since")
	assertAbsent(t, q, "updated_until")
}

func TestOrganizationsList_QueryPropagation(t *testing.T) {
	ctx, transport := newTestCtx(t)
	if _, err := organizationsList(ctx, OrganizationsListParams{
		FilterID:     99,
		UpdatedSince: "2026-03-01T00:00:00Z",
		UpdatedUntil: "2026-04-01T00:00:00Z",
	}); err != nil {
		t.Fatalf("organizationsList: %v", err)
	}
	q := mustQuery(t, transport)
	assertHas(t, q, "filter_id", "99")
	assertHas(t, q, "updated_since", "2026-03-01T00:00:00Z")
	assertHas(t, q, "updated_until", "2026-04-01T00:00:00Z")

	ctx, transport = newTestCtx(t)
	if _, err := organizationsList(ctx, OrganizationsListParams{}); err != nil {
		t.Fatalf("organizationsList zero: %v", err)
	}
	q = mustQuery(t, transport)
	assertAbsent(t, q, "filter_id")
	assertAbsent(t, q, "updated_since")
	assertAbsent(t, q, "updated_until")
}

func TestActivitiesList_QueryPropagation(t *testing.T) {
	ctx, transport := newTestCtx(t)
	if _, err := activitiesList(ctx, ActivitiesListParams{
		FilterID:     11,
		DueDate:      "2026-05-22",
		UpdatedSince: "2026-05-01T00:00:00Z",
		UpdatedUntil: "2026-05-31T00:00:00Z",
	}); err != nil {
		t.Fatalf("activitiesList: %v", err)
	}
	q := mustQuery(t, transport)
	assertHas(t, q, "filter_id", "11")
	assertHas(t, q, "due_date", "2026-05-22")
	assertHas(t, q, "updated_since", "2026-05-01T00:00:00Z")
	assertHas(t, q, "updated_until", "2026-05-31T00:00:00Z")

	ctx, transport = newTestCtx(t)
	if _, err := activitiesList(ctx, ActivitiesListParams{}); err != nil {
		t.Fatalf("activitiesList zero: %v", err)
	}
	q = mustQuery(t, transport)
	assertAbsent(t, q, "filter_id")
	assertAbsent(t, q, "due_date")
	assertAbsent(t, q, "updated_since")
	assertAbsent(t, q, "updated_until")
}

func TestProductsList_QueryPropagation(t *testing.T) {
	ctx, transport := newTestCtx(t)
	if _, err := productsList(ctx, ProductsListParams{
		FilterID:     3,
		UpdatedSince: "2026-02-01T00:00:00Z",
		UpdatedUntil: "2026-03-01T00:00:00Z",
	}); err != nil {
		t.Fatalf("productsList: %v", err)
	}
	q := mustQuery(t, transport)
	assertHas(t, q, "filter_id", "3")
	assertHas(t, q, "updated_since", "2026-02-01T00:00:00Z")
	assertHas(t, q, "updated_until", "2026-03-01T00:00:00Z")

	ctx, transport = newTestCtx(t)
	if _, err := productsList(ctx, ProductsListParams{}); err != nil {
		t.Fatalf("productsList zero: %v", err)
	}
	q = mustQuery(t, transport)
	assertAbsent(t, q, "filter_id")
	assertAbsent(t, q, "updated_since")
	assertAbsent(t, q, "updated_until")
}

func TestLeadsList_QueryPropagation(t *testing.T) {
	ctx, transport := newTestCtx(t)
	if _, err := leadsList(ctx, LeadsListParams{FilterID: 55}); err != nil {
		t.Fatalf("leadsList: %v", err)
	}
	q := mustQuery(t, transport)
	assertHas(t, q, "filter_id", "55")

	ctx, transport = newTestCtx(t)
	if _, err := leadsList(ctx, LeadsListParams{}); err != nil {
		t.Fatalf("leadsList zero: %v", err)
	}
	assertAbsent(t, mustQuery(t, transport), "filter_id")
}

func TestNotesList_QueryPropagation(t *testing.T) {
	ctx, transport := newTestCtx(t)
	if _, err := notesList(ctx, NotesListParams{
		FilterID:  17,
		StartDate: "2026-05-01",
		EndDate:   "2026-05-31",
	}); err != nil {
		t.Fatalf("notesList: %v", err)
	}
	q := mustQuery(t, transport)
	assertHas(t, q, "filter_id", "17")
	assertHas(t, q, "start_date", "2026-05-01")
	assertHas(t, q, "end_date", "2026-05-31")

	ctx, transport = newTestCtx(t)
	if _, err := notesList(ctx, NotesListParams{}); err != nil {
		t.Fatalf("notesList zero: %v", err)
	}
	q = mustQuery(t, transport)
	assertAbsent(t, q, "filter_id")
	assertAbsent(t, q, "start_date")
	assertAbsent(t, q, "end_date")
}

func TestFiltersList_TypePassthrough(t *testing.T) {
	ctx, transport := newTestCtx(t)
	if _, err := filtersList(ctx, FiltersListParams{Type: "deals"}); err != nil {
		t.Fatalf("filtersList: %v", err)
	}
	assertHas(t, mustQuery(t, transport), "type", "deals")

	ctx, transport = newTestCtx(t)
	if _, err := filtersList(ctx, FiltersListParams{}); err != nil {
		t.Fatalf("filtersList zero: %v", err)
	}
	assertAbsent(t, mustQuery(t, transport), "type")
}
