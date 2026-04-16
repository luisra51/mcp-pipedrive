package pipedrive

import (
	"path/filepath"
	"testing"
	"time"
)

func newTestCache(t *testing.T) *Cache {
	t.Helper()
	dir := t.TempDir()
	c, err := OpenCache(filepath.Join(dir, "c.bbolt"))
	if err != nil {
		t.Fatalf("OpenCache: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestCachePutGet(t *testing.T) {
	c := newTestCache(t)
	e := newEntry(map[string]any{"foo": "bar"}, time.Minute)
	if err := c.Put("k1", e); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, ok := c.Get("k1")
	if !ok {
		t.Fatalf("Get missed")
	}
	v, err := got.Decode()
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	m, ok := v.(map[string]any)
	if !ok || m["foo"] != "bar" {
		t.Fatalf("decoded mismatch: %+v", v)
	}
}

func TestCacheMissOnUnknownKey(t *testing.T) {
	c := newTestCache(t)
	if _, ok := c.Get("missing"); ok {
		t.Fatalf("expected miss")
	}
}

func TestCacheDeletePrefix(t *testing.T) {
	c := newTestCache(t)
	for _, k := range []string{"v2:xx:GET:/deals:a", "v2:xx:GET:/deals:b", "v2:xx:GET:/persons:c"} {
		if err := c.Put(k, newEntry(k, time.Minute)); err != nil {
			t.Fatalf("Put(%s): %v", k, err)
		}
	}
	n := c.DeletePrefix("v2:xx:GET:/deals")
	if n != 2 {
		t.Fatalf("DeletePrefix count = %d, want 2", n)
	}
	if _, ok := c.Get("v2:xx:GET:/persons:c"); !ok {
		t.Fatalf("sibling key was incorrectly deleted")
	}
	if _, ok := c.Get("v2:xx:GET:/deals:a"); ok {
		t.Fatalf("key should have been deleted")
	}
}

func TestCacheClear(t *testing.T) {
	c := newTestCache(t)
	for _, k := range []string{"a", "b", "c"} {
		_ = c.Put(k, newEntry(k, time.Minute))
	}
	n := c.Clear()
	if n != 3 {
		t.Fatalf("Clear count = %d, want 3", n)
	}
	if _, ok := c.Get("a"); ok {
		t.Fatalf("expected clear to remove a")
	}
}

func TestValidCacheMode(t *testing.T) {
	cases := []struct {
		in  string
		out CacheMode
		err bool
	}{
		{"", CacheModeDefault, false},
		{"default", CacheModeDefault, false},
		{"BYPASS", CacheModeBypass, false},
		{"refresh", CacheModeRefresh, false},
		{"only", CacheModeOnly, false},
		{"nope", "", true},
	}
	for _, c := range cases {
		got, err := ValidCacheMode(c.in)
		if (err != nil) != c.err {
			t.Fatalf("ValidCacheMode(%q) err=%v want err=%v", c.in, err, c.err)
		}
		if !c.err && got != c.out {
			t.Fatalf("ValidCacheMode(%q)=%q want %q", c.in, got, c.out)
		}
	}
}

func TestCacheKeyOmitsAPIToken(t *testing.T) {
	client := &Client{BaseURL: "https://example.com", AuthMode: AuthModeToken, APIToken: "secret"}
	// api_token in query should not affect the key.
	k1 := Key(client, V2, "GET", "/deals", map[string][]string{"api_token": {"secret"}, "limit": {"50"}}, nil)
	k2 := Key(client, V2, "GET", "/deals", map[string][]string{"api_token": {"other"}, "limit": {"50"}}, nil)
	if k1 != k2 {
		t.Fatalf("key changed with api_token; k1=%s k2=%s", k1, k2)
	}
	if containsSecret(k1) {
		t.Fatalf("key leaks secret: %s", k1)
	}
}

func containsSecret(s string) bool {
	return len(s) > 0 && (contains(s, "secret"))
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
