package pipedrive

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/server"
)

type configKey struct{}
type clientKey struct{}
type cacheKey struct{}

type httpContextFunc func(ctx context.Context, req *http.Request) context.Context

func WithConfig(ctx context.Context, cfg Config) context.Context {
	return context.WithValue(ctx, configKey{}, cfg)
}

func ConfigFromContext(ctx context.Context) Config {
	if cfg, ok := ctx.Value(configKey{}).(Config); ok {
		return cfg
	}
	return Config{}
}

func WithClient(ctx context.Context, client *Client) context.Context {
	return context.WithValue(ctx, clientKey{}, client)
}

func ClientFromContext(ctx context.Context) *Client {
	c, ok := ctx.Value(clientKey{}).(*Client)
	if !ok {
		return nil
	}
	return c
}

func WithCache(ctx context.Context, cache *Cache) context.Context {
	return context.WithValue(ctx, cacheKey{}, cache)
}

func CacheFromContext(ctx context.Context) *Cache {
	c, ok := ctx.Value(cacheKey{}).(*Cache)
	if !ok {
		return nil
	}
	return c
}

// Process-wide cache singleton. bbolt only allows a single writer per file,
// and the server is a single process, so we keep one Cache open for the
// lifetime of the server and let every request share it.
var (
	sharedCacheOnce sync.Once
	sharedCache     *Cache
)

func ensureSharedCache(cfg Config) *Cache {
	if !cfg.CacheEnabled || strings.TrimSpace(cfg.CachePath) == "" {
		return nil
	}
	sharedCacheOnce.Do(func() {
		c, err := OpenCache(cfg.CachePath)
		if err != nil {
			slog.Warn("failed to open bbolt cache; continuing without cache", "err", err, "path", cfg.CachePath)
			return
		}
		sharedCache = c
	})
	return sharedCache
}

// SharedCache returns the opened cache if any. Useful for tests and graceful shutdown.
func SharedCache() *Cache { return sharedCache }

func baseConfig(cfg Config) Config {
	cfg.AllowWrite = envBool(allowWriteEnvVar)
	cfg.AllowDelete = envBool(allowDeleteEnvVar)
	cfg.AllowedTools = allowedToolsFromEnv()
	cfg.Timeout = timeoutFromEnv()
	cfg.Debug = envBool(debugEnvVar)
	cfg.DisableRateLimit = envBool(disableRateLimitEnvVar)
	cfg.CacheEnabled = envBoolDefault(cacheEnabledEnvVar, true)
	cfg.CachePath = cachePathFromEnv()
	cfg.CacheStaleOn429 = envBoolDefault(cacheStaleOn429EnvVar, true)
	cfg.TTLs = ttlsFromEnv()
	cfg.AdminToolsEnabled = envBool(adminToolsEnvVar)
	return cfg
}

func ExtractInfoFromEnv(ctx context.Context) context.Context {
	cfg := ConfigFromContext(ctx)
	cfg.Domain = domainFromEnv()
	cfg.APIToken = apiTokenFromEnv()
	cfg.OAuthToken = oauthTokenFromEnv()
	cfg = baseConfig(cfg)
	return WithConfig(ctx, cfg)
}

func ExtractInfoFromHeaders(ctx context.Context, req *http.Request) context.Context {
	cfg := ConfigFromContext(ctx)

	domain := strings.TrimRight(strings.TrimSpace(req.Header.Get(domainHeader)), "/")
	if domain == "" {
		domain = domainFromEnv()
	}
	cfg.Domain = strings.TrimPrefix(strings.TrimPrefix(domain, "https://"), "http://")

	apiToken := strings.TrimSpace(req.Header.Get(apiTokenHeader))
	if apiToken == "" {
		apiToken = apiTokenFromEnv()
	}
	cfg.APIToken = apiToken

	oauthToken := strings.TrimSpace(req.Header.Get(oauthTokenHeader))
	if oauthToken == "" {
		oauthToken = oauthTokenFromEnv()
	}
	cfg.OAuthToken = oauthToken

	cfg = baseConfig(cfg)
	return WithConfig(ctx, cfg)
}

func ExtractClientFromEnv(ctx context.Context) context.Context {
	cfg := ConfigFromContext(ctx)
	logCfgWarnings(cfg)
	cache := ensureSharedCache(cfg)
	ctx = WithCache(ctx, cache)
	return WithClient(ctx, NewClient(cfg, cache))
}

func ExtractClientFromHeaders(ctx context.Context, _ *http.Request) context.Context {
	cfg := ConfigFromContext(ctx)
	logCfgWarnings(cfg)
	cache := ensureSharedCache(cfg)
	ctx = WithCache(ctx, cache)
	return WithClient(ctx, NewClient(cfg, cache))
}

func logCfgWarnings(cfg Config) {
	if cfg.ResolvedAuthMode() == AuthModeNone {
		slog.Warn("no Pipedrive credential configured", "api_token_set", cfg.APIToken != "", "oauth_token_set", cfg.OAuthToken != "")
	}
	if cfg.Domain == "" {
		slog.Warn("no Pipedrive domain configured; defaulting to api.pipedrive.com")
	}
}

func ComposeStdioContextFuncs(funcs ...server.StdioContextFunc) server.StdioContextFunc {
	return func(ctx context.Context) context.Context {
		for _, f := range funcs {
			ctx = f(ctx)
		}
		return ctx
	}
}

func ComposeSSEContextFuncs(funcs ...httpContextFunc) server.SSEContextFunc {
	return func(ctx context.Context, req *http.Request) context.Context {
		for _, f := range funcs {
			ctx = f(ctx, req)
		}
		return ctx
	}
}

func ComposeHTTPContextFuncs(funcs ...httpContextFunc) server.HTTPContextFunc {
	return func(ctx context.Context, req *http.Request) context.Context {
		for _, f := range funcs {
			ctx = f(ctx, req)
		}
		return ctx
	}
}

func ComposedStdioContextFunc() server.StdioContextFunc {
	return ComposeStdioContextFuncs(
		ExtractInfoFromEnv,
		ExtractClientFromEnv,
	)
}

func ComposedSSEContextFunc() server.SSEContextFunc {
	return ComposeSSEContextFuncs(
		ExtractInfoFromHeaders,
		ExtractClientFromHeaders,
	)
}

func ComposedHTTPContextFunc() server.HTTPContextFunc {
	return ComposeHTTPContextFuncs(
		ExtractInfoFromHeaders,
		ExtractClientFromHeaders,
	)
}
