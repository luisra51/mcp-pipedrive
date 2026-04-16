package pipedrive

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"go.etcd.io/bbolt"
)

// CacheMode controls whether a cached response is used, refreshed or bypassed.
type CacheMode string

const (
	CacheModeDefault CacheMode = "default"
	CacheModeBypass  CacheMode = "bypass"
	CacheModeRefresh CacheMode = "refresh"
	CacheModeOnly    CacheMode = "only"
)

// ValidCacheMode normalizes a user-supplied mode, defaulting to "default".
func ValidCacheMode(m string) (CacheMode, error) {
	switch CacheMode(strings.TrimSpace(strings.ToLower(m))) {
	case "", CacheModeDefault:
		return CacheModeDefault, nil
	case CacheModeBypass:
		return CacheModeBypass, nil
	case CacheModeRefresh:
		return CacheModeRefresh, nil
	case CacheModeOnly:
		return CacheModeOnly, nil
	default:
		return "", fmt.Errorf("invalid cache_mode %q: must be one of default|bypass|refresh|only", m)
	}
}

var responseBucket = []byte("responses")

// CacheEntry is the persisted payload plus cache bookkeeping.
type CacheEntry struct {
	StoredAt   time.Time       `json:"stored_at"`
	ExpiresAt  time.Time       `json:"expires_at"`
	APIVersion string          `json:"api_version,omitempty"`
	Body       json.RawMessage `json:"body"`
}

func newEntry(payload any, ttl time.Duration) *CacheEntry {
	now := time.Now().UTC()
	b, _ := json.Marshal(payload)
	return &CacheEntry{
		StoredAt:  now,
		ExpiresAt: now.Add(ttl),
		Body:      b,
	}
}

// Decode re-hydrates the cached body as a generic JSON value.
func (e *CacheEntry) Decode() (any, error) {
	var v any
	if err := json.Unmarshal(e.Body, &v); err != nil {
		return nil, err
	}
	return v, nil
}

// Meta produces the cache metadata block for a response. When stale is true,
// the caller has consumed an expired entry as a fallback.
func (e *CacheEntry) Meta(stale bool) *CacheMeta {
	ttl := int(time.Until(e.ExpiresAt).Seconds())
	if ttl < 0 {
		ttl = 0
	}
	return &CacheMeta{
		Hit:        true,
		Stale:      stale,
		StoredAt:   e.StoredAt,
		ExpiresAt:  e.ExpiresAt,
		TTLSeconds: ttl,
	}
}

// CacheMeta is attached to each tool response so callers can reason about
// freshness and decide whether to ask for fresh data.
type CacheMeta struct {
	Hit        bool      `json:"hit"`
	Stale      bool      `json:"stale,omitempty"`
	StoredAt   time.Time `json:"stored_at,omitempty"`
	ExpiresAt  time.Time `json:"expires_at,omitempty"`
	Reason     string    `json:"reason,omitempty"`
	TTLSeconds int       `json:"ttl_seconds,omitempty"`
}

// Cache is a file-backed KV store for HTTP responses.
type Cache struct {
	db   *bbolt.DB
	path string
	mu   sync.Mutex
}

// OpenCache opens or creates the bbolt file and ensures the bucket exists.
// Returns nil if path is empty.
func OpenCache(path string) (*Cache, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}
	db, err := bbolt.Open(path, 0o600, &bbolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open bbolt: %w", err)
	}
	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(responseBucket)
		return err
	})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Cache{db: db, path: path}, nil
}

func (c *Cache) Close() error {
	if c == nil || c.db == nil {
		return nil
	}
	return c.db.Close()
}

// Get returns a cached entry if present. A zero-value entry with ok=true is
// never returned.
func (c *Cache) Get(key string) (*CacheEntry, bool) {
	if c == nil || c.db == nil {
		return nil, false
	}
	var out *CacheEntry
	_ = c.db.View(func(tx *bbolt.Tx) error {
		raw := tx.Bucket(responseBucket).Get([]byte(key))
		if raw == nil {
			return nil
		}
		e := &CacheEntry{}
		if err := json.Unmarshal(raw, e); err != nil {
			return nil
		}
		out = e
		return nil
	})
	if out == nil {
		return nil, false
	}
	return out, true
}

// Put stores an entry. Errors from disk writes are returned but should not
// block the request path.
func (c *Cache) Put(key string, entry *CacheEntry) error {
	if c == nil || c.db == nil {
		return nil
	}
	b, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return c.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(responseBucket).Put([]byte(key), b)
	})
}

// Delete removes a single key. Returns true if the key existed.
func (c *Cache) Delete(key string) bool {
	if c == nil || c.db == nil {
		return false
	}
	existed := false
	_ = c.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(responseBucket)
		if b.Get([]byte(key)) != nil {
			existed = true
		}
		return b.Delete([]byte(key))
	})
	return existed
}

// DeletePrefix deletes every key starting with prefix. Returns count deleted.
func (c *Cache) DeletePrefix(prefix string) int {
	if c == nil || c.db == nil {
		return 0
	}
	count := 0
	_ = c.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(responseBucket)
		cur := b.Cursor()
		var keys [][]byte
		p := []byte(prefix)
		for k, _ := cur.Seek(p); k != nil && strings.HasPrefix(string(k), prefix); k, _ = cur.Next() {
			keys = append(keys, append([]byte(nil), k...))
		}
		for _, k := range keys {
			if err := b.Delete(k); err == nil {
				count++
			}
		}
		return nil
	})
	return count
}

// Clear removes every entry in the cache and returns the deleted count.
func (c *Cache) Clear() int {
	if c == nil || c.db == nil {
		return 0
	}
	count := 0
	_ = c.db.Update(func(tx *bbolt.Tx) error {
		if b := tx.Bucket(responseBucket); b != nil {
			count = b.Stats().KeyN
		}
		if err := tx.DeleteBucket(responseBucket); err != nil && err != bbolt.ErrBucketNotFound {
			return err
		}
		_, err := tx.CreateBucket(responseBucket)
		return err
	})
	return count
}

// Stats returns counts and disk path. Does not block the network path.
func (c *Cache) Stats() map[string]any {
	if c == nil || c.db == nil {
		return map[string]any{"enabled": false}
	}
	out := map[string]any{"enabled": true, "path": c.path}
	_ = c.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(responseBucket)
		s := b.Stats()
		out["keys"] = s.KeyN
		return nil
	})
	if fi, err := os.Stat(c.path); err == nil {
		out["size_bytes"] = fi.Size()
	}
	return out
}

// Key builds a deterministic cache key from request-affecting inputs. The
// company identity is hashed so the raw API token never appears in keys.
func Key(c *Client, version APIVersion, method, path string, q url.Values, body any) string {
	h := sha256.New()
	_, _ = h.Write([]byte(method))
	_, _ = h.Write([]byte{'|'})
	_, _ = h.Write([]byte(path))
	_, _ = h.Write([]byte{'|'})

	if q != nil {
		sorted := make([]string, 0, len(q))
		for k := range q {
			if k == "api_token" {
				continue
			}
			sorted = append(sorted, k)
		}
		sort.Strings(sorted)
		for _, k := range sorted {
			vals := append([]string(nil), q[k]...)
			sort.Strings(vals)
			for _, v := range vals {
				_, _ = h.Write([]byte(k))
				_, _ = h.Write([]byte{'='})
				_, _ = h.Write([]byte(v))
				_, _ = h.Write([]byte{'&'})
			}
		}
	}

	if body != nil {
		if b, err := json.Marshal(body); err == nil {
			_, _ = h.Write([]byte{'|'})
			_, _ = h.Write(b)
		}
	}
	queryHash := hex.EncodeToString(h.Sum(nil))[:16]

	identity := identityHash(c)
	return fmt.Sprintf("%s:%s:%s:%s:%s", version, identity, method, path, queryHash)
}

func identityHash(c *Client) string {
	if c == nil {
		return "anon"
	}
	h := sha256.New()
	_, _ = h.Write([]byte(c.BaseURL))
	_, _ = h.Write([]byte{'|'})
	switch c.AuthMode {
	case AuthModeOAuth:
		_, _ = h.Write([]byte("oauth:"))
		_, _ = h.Write([]byte(c.OAuthToken))
	case AuthModeToken:
		_, _ = h.Write([]byte("token:"))
		_, _ = h.Write([]byte(c.APIToken))
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
}
