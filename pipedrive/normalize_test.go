package pipedrive

import "testing"

func TestNormalizeDealV2Shape(t *testing.T) {
	raw := map[string]any{
		"id":       float64(42),
		"title":    "Big deal",
		"status":   "open",
		"value":    float64(1000),
		"currency": "EUR",
		"owner_id": float64(7),
		"person_id": float64(11),
		"org_id":    float64(13),
		"pipeline_id": float64(1),
		"stage_id":    float64(2),
		"custom_fields": map[string]any{"color": "blue"},
		"add_time":    "2026-01-01T10:00:00Z",
	}
	d := NormalizeDeal(raw)
	if d.ID != 42 || d.Title != "Big deal" || d.Status != "open" {
		t.Fatalf("scalar mismatch: %+v", d)
	}
	if d.Value != 1000 || d.Currency != "EUR" {
		t.Fatalf("value/currency mismatch: %+v", d)
	}
	if d.OwnerID != 7 || d.PersonID != 11 || d.OrganizationID != 13 {
		t.Fatalf("related ID mismatch: %+v", d)
	}
	if d.PipelineID != 1 || d.StageID != 2 {
		t.Fatalf("pipeline/stage mismatch: %+v", d)
	}
	if d.CustomFields["color"] != "blue" {
		t.Fatalf("custom_fields mismatch: %+v", d)
	}
}

func TestNormalizeDealV1Shape(t *testing.T) {
	// v1 returns some related objects as {value: id, name: "..."}; make sure
	// the normalizer can peel those.
	raw := map[string]any{
		"id":    float64(99),
		"title": "Legacy",
		"status": "won",
		"user_id": map[string]any{"value": float64(5), "name": "owner"},
		"org_id":  map[string]any{"value": float64(17)},
	}
	d := NormalizeDeal(raw)
	if d.OwnerID != 5 {
		t.Fatalf("OwnerID from nested user_id expected 5, got %d", d.OwnerID)
	}
	if d.OrganizationID != 17 {
		t.Fatalf("OrganizationID from nested org_id expected 17, got %d", d.OrganizationID)
	}
}

func TestNormalizePersonEmailsPhones(t *testing.T) {
	raw := map[string]any{
		"id":   float64(1),
		"name": "Jane Doe",
		"emails": []any{
			map[string]any{"value": "jane@example.com", "primary": true, "label": "work"},
			map[string]any{"value": "jane@gmail.com", "primary": false, "label": "home"},
		},
		"phones": []any{
			map[string]any{"value": "+1555", "primary": true, "label": "mobile"},
		},
	}
	p := NormalizePerson(raw)
	if len(p.Emails) != 2 || p.Emails[0].Value != "jane@example.com" || !p.Emails[0].Primary {
		t.Fatalf("emails not normalized: %+v", p.Emails)
	}
	if len(p.Phones) != 1 || p.Phones[0].Label != "mobile" {
		t.Fatalf("phones not normalized: %+v", p.Phones)
	}
}

func TestNormalizeOrganizationAddressVariants(t *testing.T) {
	scalar := NormalizeOrganization(map[string]any{"id": float64(1), "name": "Foo", "address": "1 Main"})
	if scalar.Address != "1 Main" {
		t.Fatalf("scalar address not kept: %+v", scalar)
	}
	obj := NormalizeOrganization(map[string]any{"id": float64(2), "name": "Bar", "address": map[string]any{"value": "2 Elm"}})
	if obj.Address != "2 Elm" {
		t.Fatalf("object address.value not extracted: %+v", obj)
	}
}

func TestNormalizeActivityDone(t *testing.T) {
	a := NormalizeActivity(map[string]any{"id": float64(1), "subject": "Call", "done": true, "type": "call"})
	if !a.Done || a.Type != "call" {
		t.Fatalf("activity mismatch: %+v", a)
	}
}

func TestNormalizeNote(t *testing.T) {
	n := NormalizeNote(map[string]any{
		"id":      float64(1),
		"content": "hello",
		"deal_id": float64(42),
	})
	if n.Content != "hello" || n.DealID != 42 {
		t.Fatalf("note mismatch: %+v", n)
	}
}
