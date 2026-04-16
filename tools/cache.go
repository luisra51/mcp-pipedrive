package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	mcppipedrive "mcp-pipedrive"
	"mcp-pipedrive/internal"
	"mcp-pipedrive/pipedrive"
)

type CacheStatsParams struct{}

type CacheClearParams struct {
	Confirm bool `json:"confirm" jsonschema:"description=Must be true to proceed. Safety switch."`
}

type CacheInvalidateParams struct {
	Prefix string `json:"prefix" jsonschema:"description=Key prefix to invalidate (e.g. 'v2:{identity}:GET:/deals')"`
}

func cacheStats(ctx context.Context, _ CacheStatsParams) (any, error) {
	if disabled, err := ensureToolAllowed(ctx, "pipedrive.cache.stats", guardRead); err != nil {
		return nil, err
	} else if disabled != nil {
		return disabled, nil
	}
	cache := pipedrive.CacheFromContext(ctx)
	stats := cache.Stats()
	return internal.Wrap(stats, nil), nil
}

func cacheClear(ctx context.Context, args CacheClearParams) (any, error) {
	if disabled, err := ensureToolAllowed(ctx, "pipedrive.cache.clear", guardAdmin); err != nil {
		return nil, err
	} else if disabled != nil {
		return disabled, nil
	}
	if !args.Confirm {
		return nil, fmt.Errorf("pass confirm=true to clear the cache")
	}
	cache := pipedrive.CacheFromContext(ctx)
	if cache == nil {
		return nil, &mcppipedrive.HardError{Err: fmt.Errorf("cache is disabled")}
	}
	n := cache.Clear()
	return internal.Wrap(map[string]any{"cleared": n}, nil), nil
}

func cacheInvalidate(ctx context.Context, args CacheInvalidateParams) (any, error) {
	if disabled, err := ensureToolAllowed(ctx, "pipedrive.cache.invalidate", guardAdmin); err != nil {
		return nil, err
	} else if disabled != nil {
		return disabled, nil
	}
	if err := internal.RequireID(args.Prefix, "prefix"); err != nil {
		return nil, err
	}
	cache := pipedrive.CacheFromContext(ctx)
	if cache == nil {
		return nil, &mcppipedrive.HardError{Err: fmt.Errorf("cache is disabled")}
	}
	n := cache.DeletePrefix(args.Prefix)
	return internal.Wrap(map[string]any{"invalidated": n, "prefix": args.Prefix}, nil), nil
}

var CacheStats = mcppipedrive.MustTool(
	"pipedrive.cache.stats",
	"Return local cache statistics (keys, path, size).",
	cacheStats,
	mcp.WithTitleAnnotation("Cache stats"),
	mcp.WithIdempotentHintAnnotation(true),
	mcp.WithReadOnlyHintAnnotation(true),
)

var CacheClear = mcppipedrive.MustTool(
	"pipedrive.cache.clear",
	"Clear the entire local cache. Admin-only. Requires PIPEDRIVE_ENABLE_ADMIN_TOOLS=true.",
	cacheClear,
	mcp.WithTitleAnnotation("Clear cache"),
	mcp.WithDestructiveHintAnnotation(true),
)

var CacheInvalidate = mcppipedrive.MustTool(
	"pipedrive.cache.invalidate",
	"Invalidate every cache entry whose key starts with the given prefix. Admin-only.",
	cacheInvalidate,
	mcp.WithTitleAnnotation("Invalidate cache prefix"),
)

