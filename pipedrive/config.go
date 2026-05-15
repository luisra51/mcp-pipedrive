// Package pipedrive contains the Pipedrive API client, config, cache, and
// context plumbing. The client supports both API-token and OAuth bearer
// authentication and routes requests to either Pipedrive API v2 (preferred)
// or v1 (fallback) depending on endpoint coverage.
package pipedrive

import (
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultDomain = "api.pipedrive.com"

	domainEnvVar           = "PIPEDRIVE_DOMAIN"
	apiTokenEnvVar         = "PIPEDRIVE_API_TOKEN"
	oauthTokenEnvVar       = "PIPEDRIVE_OAUTH_ACCESS_TOKEN"
	timeoutMsEnvVar        = "PIPEDRIVE_TIMEOUT_MS"
	allowWriteEnvVar       = "PIPEDRIVE_ALLOW_WRITE"
	allowDeleteEnvVar      = "PIPEDRIVE_ALLOW_DELETE"
	allowedToolsEnvVar     = "PIPEDRIVE_ALLOWED_TOOLS"
	debugEnvVar            = "PIPEDRIVE_DEBUG"
	disableRateLimitEnvVar = "PIPEDRIVE_RATE_LIMIT_DISABLE"
	adminToolsEnvVar       = "PIPEDRIVE_ENABLE_ADMIN_TOOLS"

	cacheEnabledEnvVar     = "PIPEDRIVE_CACHE_ENABLED"
	cachePathEnvVar        = "PIPEDRIVE_CACHE_PATH"
	cacheMetadataTTLEnvVar = "PIPEDRIVE_CACHE_METADATA_TTL"
	cacheDealTTLEnvVar     = "PIPEDRIVE_CACHE_DEAL_TTL"
	cachePersonTTLEnvVar   = "PIPEDRIVE_CACHE_PERSON_TTL"
	cacheOrgTTLEnvVar      = "PIPEDRIVE_CACHE_ORGANIZATION_TTL"
	cacheActivityTTLEnvVar = "PIPEDRIVE_CACHE_ACTIVITY_TTL"
	cacheProductTTLEnvVar  = "PIPEDRIVE_CACHE_PRODUCT_TTL"
	cacheListTTLEnvVar     = "PIPEDRIVE_CACHE_LIST_TTL"
	cacheSearchTTLEnvVar   = "PIPEDRIVE_CACHE_SEARCH_TTL"
	cacheFollowerTTLEnvVar    = "PIPEDRIVE_CACHE_FOLLOWER_TTL"
	cacheMailListTTLEnvVar    = "PIPEDRIVE_CACHE_MAIL_LIST_TTL"
	cacheMailMessageTTLEnvVar = "PIPEDRIVE_CACHE_MAIL_MESSAGE_TTL"
	cacheStaleOn429EnvVar     = "PIPEDRIVE_CACHE_ALLOW_STALE_ON_429"
	cacheStaleTTLEnvVar       = "PIPEDRIVE_CACHE_STALE_TTL"

	domainHeader     = "X-Pipedrive-Domain"
	apiTokenHeader   = "X-Pipedrive-API-Token"
	oauthTokenHeader = "X-Pipedrive-OAuth-Token"
)

// AuthMode indicates which credential the client should attach to requests.
type AuthMode string

const (
	AuthModeNone  AuthMode = ""
	AuthModeToken AuthMode = "api_token"
	AuthModeOAuth AuthMode = "oauth"
)

// CacheTTLs groups per-resource TTLs resolved from env vars.
type CacheTTLs struct {
	Metadata     time.Duration
	Deal         time.Duration
	Person       time.Duration
	Organization time.Duration
	Activity     time.Duration
	Product      time.Duration
	Follower     time.Duration
	List         time.Duration
	Search       time.Duration
	Stale        time.Duration
	// Mailbox endpoints. MailList is for /mailbox/mailThreads and
	// /deals/{id}/mailMessages (short — listings drift). MailMessage is
	// for /mailbox/mailMessages/{id} (long — message contents are
	// immutable once delivered; mutable fields like read_flag can be
	// busted via pipedrive.cache.invalidate).
	MailList    time.Duration
	MailMessage time.Duration
}

type Config struct {
	Debug                   bool
	IncludeArgumentsInSpans bool

	// Domain is the Pipedrive host without scheme, e.g. "mycompany.pipedrive.com"
	// or "api.pipedrive.com" for OAuth-scoped requests.
	Domain string

	// Exactly one of APIToken or OAuthToken should be set. When both are set,
	// OAuth wins (explicit scope beats global token).
	APIToken   string
	OAuthToken string

	AllowWrite       bool
	AllowDelete      bool
	AllowedTools     map[string]struct{}
	Timeout          time.Duration
	DisableRateLimit bool

	CacheEnabled    bool
	CachePath       string
	CacheStaleOn429 bool
	TTLs            CacheTTLs

	AdminToolsEnabled bool
}

// BaseURL returns the scheme + host for HTTP requests.
func (c Config) BaseURL() string {
	d := strings.TrimRight(c.Domain, "/")
	if d == "" {
		d = defaultDomain
	}
	return "https://" + d
}

// ResolvedAuthMode picks the credential to use, preferring OAuth.
func (c Config) ResolvedAuthMode() AuthMode {
	if strings.TrimSpace(c.OAuthToken) != "" {
		return AuthModeOAuth
	}
	if strings.TrimSpace(c.APIToken) != "" {
		return AuthModeToken
	}
	return AuthModeNone
}

func envBool(key string) bool {
	val := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	return val == "1" || val == "true" || val == "yes"
}

func envBoolDefault(key string, def bool) bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if raw == "" {
		return def
	}
	return raw == "1" || raw == "true" || raw == "yes"
}

func allowedToolsFromEnv() map[string]struct{} {
	val := strings.TrimSpace(os.Getenv(allowedToolsEnvVar))
	if val == "" {
		return nil
	}
	set := make(map[string]struct{})
	for _, t := range strings.Split(val, ",") {
		name := strings.TrimSpace(t)
		if name != "" {
			set[name] = struct{}{}
		}
	}
	return set
}

func timeoutFromEnv() time.Duration {
	val := strings.TrimSpace(os.Getenv(timeoutMsEnvVar))
	if val == "" {
		return 30 * time.Second
	}
	ms, err := strconv.Atoi(val)
	if err != nil || ms <= 0 {
		return 30 * time.Second
	}
	return time.Duration(ms) * time.Millisecond
}

func domainFromEnv() string {
	d := strings.TrimSpace(strings.TrimRight(os.Getenv(domainEnvVar), "/"))
	if d == "" {
		return defaultDomain
	}
	// Tolerate users passing a full URL.
	d = strings.TrimPrefix(d, "https://")
	d = strings.TrimPrefix(d, "http://")
	return d
}

func apiTokenFromEnv() string   { return strings.TrimSpace(os.Getenv(apiTokenEnvVar)) }
func oauthTokenFromEnv() string { return strings.TrimSpace(os.Getenv(oauthTokenEnvVar)) }

func cachePathFromEnv() string {
	v := strings.TrimSpace(os.Getenv(cachePathEnvVar))
	if v == "" {
		return ".cache/pipedrive-mcp.bbolt"
	}
	return v
}

func durationFromEnv(key string, def time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return def
	}
	return d
}

func ttlsFromEnv() CacheTTLs {
	return CacheTTLs{
		Metadata:     durationFromEnv(cacheMetadataTTLEnvVar, 12*time.Hour),
		Deal:         durationFromEnv(cacheDealTTLEnvVar, 180*time.Second),
		Person:       durationFromEnv(cachePersonTTLEnvVar, 600*time.Second),
		Organization: durationFromEnv(cacheOrgTTLEnvVar, 600*time.Second),
		Activity:     durationFromEnv(cacheActivityTTLEnvVar, 60*time.Second),
		Product:      durationFromEnv(cacheProductTTLEnvVar, 30*time.Minute),
		Follower:     durationFromEnv(cacheFollowerTTLEnvVar, 300*time.Second),
		List:         durationFromEnv(cacheListTTLEnvVar, 120*time.Second),
		Search:       durationFromEnv(cacheSearchTTLEnvVar, 45*time.Second),
		Stale:        durationFromEnv(cacheStaleTTLEnvVar, 24*time.Hour),
		MailList:     durationFromEnv(cacheMailListTTLEnvVar, 60*time.Second),
		MailMessage:  durationFromEnv(cacheMailMessageTTLEnvVar, 24*time.Hour),
	}
}
