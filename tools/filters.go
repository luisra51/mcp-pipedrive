package tools

// Saved filters are Pipedrive API v1 only. This tool lets callers discover
// filter IDs so they can be passed as `filter_id` to any list tool and
// reproduce a UI view exactly.

import (
	"context"
	"net/http"
	"net/url"

	"github.com/mark3labs/mcp-go/mcp"

	mcppipedrive "mcp-pipedrive"
	"mcp-pipedrive/internal"
	"mcp-pipedrive/pipedrive"
)

type FiltersListParams struct {
	Type       string `json:"type,omitempty" jsonschema:"description=Filter type: deals|people|org|product|activity|lead (omit for all)"`
	IncludeRaw bool   `json:"include_raw,omitempty" jsonschema:"description=If true also include raw v1 payload (with conditions tree)"`
	CacheMode  string `json:"cache_mode,omitempty" jsonschema:"description=Cache mode: default|bypass|refresh|only"`
}

func filtersList(ctx context.Context, args FiltersListParams) (any, error) {
	if d, err := ensureToolAllowed(ctx, "pipedrive.filters.list", guardRead); err != nil {
		return nil, err
	} else if d != nil {
		return d, nil
	}
	mode, err := pipedrive.ValidCacheMode(args.CacheMode)
	if err != nil {
		return nil, err
	}
	client, err := clientOrError(ctx)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	if args.Type != "" {
		q.Set("type", args.Type)
	}
	req, err := client.NewRequest(pipedrive.V1, http.MethodGet, "/filters", q, nil)
	if err != nil {
		return nil, err
	}
	key := pipedrive.Key(client, pipedrive.V1, http.MethodGet, "/filters", q, nil)
	payload, cacheMeta, err := client.CachedGet(req.WithContext(ctx), key, client.TTLs.Metadata, mode)
	if err != nil {
		return nil, wrapAPIError(err)
	}
	raw, arr := extractListData(payload)
	data := map[string]any{"filters": pipedrive.NormalizeFilterList(arr)}
	if args.IncludeRaw {
		data["raw"] = internal.MaskSensitive(raw)
	}
	return internal.Wrap(data, map[string]any{"cache": cacheMeta}), nil
}

var FiltersList = mcppipedrive.MustTool("pipedrive.filters.list",
	"List saved filters (Pipedrive API v1). Use the returned id with filter_id on any list tool to reproduce a UI view.",
	filtersList,
	mcp.WithTitleAnnotation("List saved filters"),
	mcp.WithIdempotentHintAnnotation(true),
	mcp.WithReadOnlyHintAnnotation(true))
