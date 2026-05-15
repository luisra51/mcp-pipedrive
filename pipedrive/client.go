package pipedrive

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"mcp-pipedrive/internal"
)

// APIVersion selects the upstream Pipedrive API path prefix.
type APIVersion string

const (
	V1 APIVersion = "v1"
	V2 APIVersion = "v2"
	// V1Legacy targets the bare /v1/ path (no /api/ segment). Some
	// Pipedrive endpoints — notably /mailbox/* — are only published at
	// /v1/, while everything else accepts the /api/v1/ form. Use this
	// for those legacy paths.
	V1Legacy APIVersion = "v1-legacy"
)

func (v APIVersion) prefix() string {
	switch v {
	case V2:
		return "/api/v2"
	case V1Legacy:
		return "/v1"
	default:
		return "/api/v1"
	}
}

type Client struct {
	BaseURL    string
	HTTP       *http.Client
	APIToken   string
	OAuthToken string
	AuthMode   AuthMode
	Limiter    *internal.MultiLimiter
	Cache      *Cache
	TTLs       CacheTTLs
	StaleOn429 bool
}

func NewClient(cfg Config, cache *Cache) *Client {
	return &Client{
		BaseURL:    cfg.BaseURL(),
		HTTP:       &http.Client{Timeout: cfg.Timeout, Transport: http.DefaultTransport},
		APIToken:   cfg.APIToken,
		OAuthToken: cfg.OAuthToken,
		AuthMode:   cfg.ResolvedAuthMode(),
		Limiter:    limiterFromConfig(cfg),
		Cache:      cache,
		TTLs:       cfg.TTLs,
		StaleOn429: cfg.CacheStaleOn429,
	}
}

func limiterFromConfig(cfg Config) *internal.MultiLimiter {
	if cfg.DisableRateLimit {
		return nil
	}
	return internal.NewPipedriveLimiter()
}

// NewRequest builds a request against the given API version. When using API
// token auth, `api_token` is appended to the query string (Pipedrive's
// convention); for OAuth, the `Authorization: Bearer` header is set inside do().
func (c *Client) NewRequest(v APIVersion, method, path string, q url.Values, body any) (*http.Request, error) {
	if c.AuthMode == AuthModeNone {
		return nil, ErrMissingAuth
	}
	if q == nil {
		q = url.Values{}
	}
	if c.AuthMode == AuthModeToken {
		q.Set("api_token", c.APIToken)
	}

	full := strings.TrimRight(c.BaseURL, "/") + v.prefix() + ensureLeadingSlash(path)
	if len(q) > 0 {
		full += "?" + q.Encode()
	}

	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, full, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	return req, nil
}

func ensureLeadingSlash(p string) string {
	if p == "" || strings.HasPrefix(p, "/") {
		return p
	}
	return "/" + p
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	if c.Limiter != nil {
		if err := c.Limiter.Wait(req.Context()); err != nil {
			return nil, err
		}
	}
	if c.AuthMode == AuthModeOAuth {
		req.Header.Set("Authorization", "Bearer "+c.OAuthToken)
	}
	return c.HTTP.Do(req)
}

// DoJSON executes a request and decodes the JSON body into out. Non-2xx
// responses are returned as *APIError.
func (c *Client) DoJSON(req *http.Request, out any) error {
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return &APIError{StatusCode: resp.StatusCode, Status: resp.Status, Body: body}
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// DoRaw executes a request and returns the raw body + content-type.
func (c *Client) DoRaw(req *http.Request) ([]byte, string, error) {
	resp, err := c.do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", &APIError{StatusCode: resp.StatusCode, Status: resp.Status, Body: body}
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	return b, resp.Header.Get("Content-Type"), nil
}

// CachedGet performs an HTTP GET with cache integration. Returns the decoded
// payload, cache metadata, and any error. When the cache is disabled or the
// mode is `bypass`, this is equivalent to DoJSON.
func (c *Client) CachedGet(req *http.Request, cacheKey string, ttl time.Duration, mode CacheMode) (any, *CacheMeta, error) {
	now := time.Now().UTC()

	if c.Cache == nil || mode == CacheModeBypass {
		var payload any
		if err := c.DoJSON(req, &payload); err != nil {
			if c.Cache != nil && c.StaleOn429 {
				if meta, stale := c.staleFallback(cacheKey, err); stale != nil {
					return stale, meta, nil
				}
			}
			return nil, nil, err
		}
		return payload, &CacheMeta{Hit: false}, nil
	}

	if mode == "" {
		mode = CacheModeDefault
	}

	if mode == CacheModeDefault || mode == CacheModeOnly {
		if entry, ok := c.Cache.Get(cacheKey); ok && entry.ExpiresAt.After(now) {
			payload, err := entry.Decode()
			if err == nil {
				return payload, entry.Meta(false), nil
			}
		}
		if mode == CacheModeOnly {
			return nil, &CacheMeta{Hit: false, Reason: "cache_miss"}, errors.New("cache miss")
		}
	}

	var payload any
	if err := c.DoJSON(req, &payload); err != nil {
		if c.StaleOn429 {
			if meta, stale := c.staleFallback(cacheKey, err); stale != nil {
				return stale, meta, nil
			}
		}
		return nil, nil, err
	}

	if err := c.Cache.Put(cacheKey, newEntry(payload, ttl)); err != nil {
		// Cache write failures should not fail the request.
		return payload, &CacheMeta{Hit: false, Reason: "cache_store_failed"}, nil
	}
	return payload, &CacheMeta{Hit: false, StoredAt: now, ExpiresAt: now.Add(ttl), TTLSeconds: int(ttl.Seconds())}, nil
}

// InvalidatePath removes every cached entry keyed to the given (version,
// method, path). Pass an empty path to invalidate all entries under a version.
// No-op if the cache is disabled.
func (c *Client) InvalidatePath(version APIVersion, method, path string) int {
	if c == nil || c.Cache == nil {
		return 0
	}
	prefix := string(version) + ":" + identityHash(c) + ":" + method + ":" + path
	return c.Cache.DeletePrefix(prefix)
}

// InvalidatePrefix removes entries whose key starts with the given prefix.
// Useful for broad cleanups after writes (e.g. "v2:{id}:GET:/deals").
func (c *Client) InvalidatePrefix(prefix string) int {
	if c == nil || c.Cache == nil {
		return 0
	}
	return c.Cache.DeletePrefix(prefix)
}

// IdentityHash returns this client's opaque identity prefix used in cache keys.
// Exposed for admin tools that build prefixes explicitly.
func (c *Client) IdentityHash() string { return identityHash(c) }

func (c *Client) staleFallback(key string, cause error) (*CacheMeta, any) {
	var apiErr *APIError
	if !errors.As(cause, &apiErr) {
		return nil, nil
	}
	if !apiErr.IsRateLimited() && !apiErr.IsServerError() {
		return nil, nil
	}
	entry, ok := c.Cache.Get(key)
	if !ok {
		return nil, nil
	}
	payload, err := entry.Decode()
	if err != nil {
		return nil, nil
	}
	reason := "pipedrive_rate_limited"
	if apiErr.IsServerError() {
		reason = "pipedrive_server_error"
	}
	return &CacheMeta{Hit: true, Stale: true, StoredAt: entry.StoredAt, ExpiresAt: entry.ExpiresAt, Reason: reason}, payload
}
