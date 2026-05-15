package tools

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	mcppipedrive "mcp-pipedrive"
	"mcp-pipedrive/internal"
	"mcp-pipedrive/pipedrive"
)

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const (
	// mailThreadsDefaultLimit is the agent-facing default page size on
	// pipedrive.mail.threads.list — small enough that scanning the inbox
	// doesn't burn LLM context.
	mailThreadsDefaultLimit = 20

	// mailDealDefaultLimit is the per-call default for
	// pipedrive.deals.mail.list. Most deal conversations fit in one page.
	mailDealDefaultLimit = 50

	// mailFilteredPageSize is the per-page Pipedrive request size used
	// when iterating to satisfy a `since` / `unread_only` filter.
	mailFilteredPageSize = 100

	// mailFilteredHardCap bounds the total returned items under filters so
	// a wide `since` doesn't run away.
	mailFilteredHardCap = 200

	// dealBodyConcurrency caps in-flight body backfills per
	// pipedrive.deals.mail.list call. Roughly half of Pipedrive's typical
	// 80-req/2s burst budget for personal tokens.
	dealBodyConcurrency = 5
)

// ---------------------------------------------------------------------------
// Params
// ---------------------------------------------------------------------------

type MailThreadsListParams struct {
	Folder     string `json:"folder,omitempty" jsonschema:"description=inbox (default) | sent | drafts | archive"`
	DealID     int64  `json:"deal_id,omitempty" jsonschema:"description=Filter by linked deal id"`
	PersonID   int64  `json:"person_id,omitempty" jsonschema:"description=Filter by linked person id"`
	Start      int    `json:"start,omitempty" jsonschema:"description=Pagination offset"`
	Limit      int    `json:"limit,omitempty" jsonschema:"description=Max threads (default 20 hard cap 200 with filters)"`
	Since      string `json:"since,omitempty" jsonschema:"description=RFC3339 cutoff; keep threads with last_message_timestamp >= this"`
	UnreadOnly bool   `json:"unread_only,omitempty" jsonschema:"description=Drop threads with read_flag != 0"`
	Fields     string `json:"fields,omitempty" jsonschema:"description=compact (default; primary-from only snippet <=120 chars) | full (adds parties tree)"`
	CacheMode  string `json:"cache_mode,omitempty" jsonschema:"description=Cache mode: default|bypass|refresh|only"`
}

type MailThreadsGetParams struct {
	ID              int64  `json:"id" jsonschema:"description=Thread id"`
	IncludeMessages *bool  `json:"include_messages,omitempty" jsonschema:"description=Include per-message metadata (default true)"`
	Fields          string `json:"fields,omitempty" jsonschema:"description=compact (default) | full"`
	CacheMode       string `json:"cache_mode,omitempty" jsonschema:"description=Cache mode: default|bypass|refresh|only"`
}

type MailMessagesGetParams struct {
	ID          int64  `json:"id" jsonschema:"description=Message id"`
	IncludeBody *bool  `json:"include_body,omitempty" jsonschema:"description=Include the message body (default true)"`
	BodyFormat  string `json:"body_format,omitempty" jsonschema:"description=text (default HTML-stripped) | html (raw) | none"`
	CacheMode   string `json:"cache_mode,omitempty" jsonschema:"description=Cache mode: default|bypass|refresh|only"`
}

type DealsMailListParams struct {
	DealID        int64  `json:"deal_id" jsonschema:"description=Deal id"`
	Start         int    `json:"start,omitempty" jsonschema:"description=Pagination offset"`
	Limit         int    `json:"limit,omitempty" jsonschema:"description=Max messages (default 50 hard cap 200 with since)"`
	Since         string `json:"since,omitempty" jsonschema:"description=RFC3339 cutoff; keep messages with message_time >= this"`
	IncludeBodies *bool  `json:"include_bodies,omitempty" jsonschema:"description=Backfill bodies for messages that lack one (default true)"`
	MaxBodies     int    `json:"max_bodies,omitempty" jsonschema:"description=Cap per-call body backfills; 0 = no cap"`
	BodyFormat    string `json:"body_format,omitempty" jsonschema:"description=text (default HTML-stripped) | html | none"`
	CacheMode     string `json:"cache_mode,omitempty" jsonschema:"description=Cache mode: default|bypass|refresh|only"`
}

// boolOrDefault deref's a *bool with a default for the nil case.
func boolOrDefault(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func mailThreadsList(ctx context.Context, args MailThreadsListParams) (any, error) {
	if disabled, err := ensureToolAllowed(ctx, "pipedrive.mail.threads.list", guardRead); err != nil {
		return nil, err
	} else if disabled != nil {
		return disabled, nil
	}
	mode, err := pipedrive.ValidCacheMode(args.CacheMode)
	if err != nil {
		return nil, err
	}
	client, err := clientOrError(ctx)
	if err != nil {
		return nil, err
	}

	folder := args.Folder
	if folder == "" {
		folder = "inbox"
	}
	fields := pipedrive.NormalizeMailFields(args.Fields)
	limit := args.Limit
	if limit <= 0 {
		limit = mailThreadsDefaultLimit
	}

	baseQuery := func(start, pageLimit int) url.Values {
		q := url.Values{}
		q.Set("folder", folder)
		if args.DealID != 0 {
			q.Set("deal_id", strconv.FormatInt(args.DealID, 10))
		}
		if args.PersonID != 0 {
			q.Set("person_id", strconv.FormatInt(args.PersonID, 10))
		}
		if start > 0 {
			q.Set("start", strconv.Itoa(start))
		}
		if pageLimit > 0 {
			q.Set("limit", strconv.Itoa(pageLimit))
		}
		return q
	}

	// Fast path: no filter → single upstream page.
	if args.Since == "" && !args.UnreadOnly {
		q := baseQuery(args.Start, limit)
		payload, cacheMeta, err := mailCachedGet(ctx, client, pipedrive.V1Legacy, "/mailbox/mailThreads", q, client.TTLs.MailList, mode)
		if err != nil {
			return nil, err
		}
		_, rawArr := extractListData(payload)
		threads := pipedrive.NormalizeMailThreadList(rawArr, fields)
		more, nextStart := extractMailPagination(payload)
		return internal.Wrap(
			map[string]any{"threads": threads},
			map[string]any{"cache": cacheMeta, "more": more, "next_start": nextStart},
		), nil
	}

	// Filtered path: iterate upstream pages until cutoff or cap.
	hardCap := limit
	if hardCap > mailFilteredHardCap {
		hardCap = mailFilteredHardCap
	}
	collected := make([]any, 0, hardCap)
	cursor := args.Start
	pastCutoff := false
	more := false
	nextStart := 0

iteratePages:
	for {
		q := baseQuery(cursor, mailFilteredPageSize)
		payload, _, err := mailCachedGet(ctx, client, pipedrive.V1Legacy, "/mailbox/mailThreads", q, client.TTLs.MailList, mode)
		if err != nil {
			return nil, err
		}
		_, items := extractListData(payload)
		for _, item := range items {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			ts := toStringRaw(m["last_message_timestamp"])
			if args.Since != "" && ts != "" && ts < args.Since {
				pastCutoff = true
				continue
			}
			if args.UnreadOnly && toInt64Raw(m["read_flag"]) != 0 {
				continue
			}
			collected = append(collected, item)
			if len(collected) >= hardCap {
				pageMore, pageNext := extractMailPagination(payload)
				more = pageMore || pastCutoff
				if pageMore {
					nextStart = pageNext
				}
				break iteratePages
			}
		}
		pageMore, pageNext := extractMailPagination(payload)
		if pastCutoff || !pageMore {
			break
		}
		cursor = pageNext
	}

	threads := pipedrive.NormalizeMailThreadList(collected, fields)
	return internal.Wrap(
		map[string]any{"threads": threads},
		map[string]any{
			"more":          more,
			"next_start":    nextStart,
			"applied_since": args.Since,
			"unread_only":   args.UnreadOnly,
			"matched":       len(threads),
		},
	), nil
}

func mailThreadsGet(ctx context.Context, args MailThreadsGetParams) (any, error) {
	if disabled, err := ensureToolAllowed(ctx, "pipedrive.mail.threads.get", guardRead); err != nil {
		return nil, err
	} else if disabled != nil {
		return disabled, nil
	}
	if args.ID <= 0 {
		return nil, fmt.Errorf("id is required and must be > 0")
	}
	mode, err := pipedrive.ValidCacheMode(args.CacheMode)
	if err != nil {
		return nil, err
	}
	client, err := clientOrError(ctx)
	if err != nil {
		return nil, err
	}
	fields := pipedrive.NormalizeMailFields(args.Fields)
	includeMessages := boolOrDefault(args.IncludeMessages, true)

	path := "/mailbox/mailThreads/" + strconv.FormatInt(args.ID, 10)
	payload, cacheMeta, err := mailCachedGet(ctx, client, pipedrive.V1Legacy, path, nil, client.TTLs.MailList, mode)
	if err != nil {
		return nil, err
	}
	_, rawThread := extractItemData(payload)
	thread := pipedrive.NormalizeMailThread(rawThread, fields)

	data := map[string]any{"thread": thread}
	meta := map[string]any{"cache": cacheMeta}

	if includeMessages {
		msgsPath := path + "/mailMessages"
		msgsPayload, msgsCache, err := mailCachedGet(ctx, client, pipedrive.V1Legacy, msgsPath, nil, client.TTLs.MailList, mode)
		if err != nil {
			return nil, err
		}
		_, msgArr := extractListData(msgsPayload)
		// /mailbox/mailThreads/{id}/mailMessages returns metadata only.
		data["messages"] = pipedrive.NormalizeMailMessageList(msgArr, pipedrive.BodyFormatNone)
		meta["messages_cache"] = msgsCache
	}
	return internal.Wrap(data, meta), nil
}

func mailMessagesGet(ctx context.Context, args MailMessagesGetParams) (any, error) {
	if disabled, err := ensureToolAllowed(ctx, "pipedrive.mail.messages.get", guardRead); err != nil {
		return nil, err
	} else if disabled != nil {
		return disabled, nil
	}
	if args.ID <= 0 {
		return nil, fmt.Errorf("id is required and must be > 0")
	}
	mode, err := pipedrive.ValidCacheMode(args.CacheMode)
	if err != nil {
		return nil, err
	}
	client, err := clientOrError(ctx)
	if err != nil {
		return nil, err
	}

	includeBody := boolOrDefault(args.IncludeBody, true)
	format := pipedrive.NormalizeBodyFormat(args.BodyFormat)
	if !includeBody {
		format = pipedrive.BodyFormatNone
	}

	q := url.Values{}
	if includeBody {
		q.Set("include_body", "1")
	}
	path := "/mailbox/mailMessages/" + strconv.FormatInt(args.ID, 10)
	payload, cacheMeta, err := mailCachedGet(ctx, client, pipedrive.V1Legacy, path, q, client.TTLs.MailMessage, mode)
	if err != nil {
		return nil, err
	}
	_, rawMsg := extractItemData(payload)
	msg := pipedrive.NormalizeMailMessage(rawMsg, format)
	return internal.Wrap(
		map[string]any{"message": msg},
		map[string]any{"cache": cacheMeta},
	), nil
}

func dealsMailList(ctx context.Context, args DealsMailListParams) (any, error) {
	if disabled, err := ensureToolAllowed(ctx, "pipedrive.deals.mail.list", guardRead); err != nil {
		return nil, err
	} else if disabled != nil {
		return disabled, nil
	}
	if args.DealID <= 0 {
		return nil, fmt.Errorf("deal_id is required and must be > 0")
	}
	mode, err := pipedrive.ValidCacheMode(args.CacheMode)
	if err != nil {
		return nil, err
	}
	client, err := clientOrError(ctx)
	if err != nil {
		return nil, err
	}

	includeBodies := boolOrDefault(args.IncludeBodies, true)
	format := pipedrive.NormalizeBodyFormat(args.BodyFormat)
	if !includeBodies {
		format = pipedrive.BodyFormatNone
	}
	limit := args.Limit
	if limit <= 0 {
		limit = mailDealDefaultLimit
	}

	rawMsgs, more, nextStart, err := dealMailFetch(ctx, client, args.DealID, args.Start, limit, args.Since, mode)
	if err != nil {
		return nil, err
	}

	// Bodies live in /mailbox/mailMessages/{id}; the deal endpoint
	// returns metadata only. Backfill any message lacking a body with
	// bounded concurrency. Cached bodies skip the HTTP call via
	// CachedGet.
	bodiesFetched := 0
	bodiesSkipped := 0
	bodyErrors := map[int64]string{}
	bodiesRequested := includeBodies && format != pipedrive.BodyFormatNone

	if bodiesRequested {
		type need struct {
			idx int
			id  int64
		}
		var needs []need
		for i, m := range rawMsgs {
			body := toStringRaw(m["body"])
			if body == "" {
				body = toStringRaw(m["body_content"])
			}
			if body == "" {
				if id := toInt64Raw(m["id"]); id > 0 {
					needs = append(needs, need{idx: i, id: id})
				}
			}
		}
		if args.MaxBodies > 0 && len(needs) > args.MaxBodies {
			bodiesSkipped = len(needs) - args.MaxBodies
			needs = needs[:args.MaxBodies]
		}

		if len(needs) > 0 {
			var mu sync.Mutex
			sem := make(chan struct{}, dealBodyConcurrency)
			var wg sync.WaitGroup
			for _, n := range needs {
				wg.Add(1)
				sem <- struct{}{}
				go func(n need) {
					defer wg.Done()
					defer func() { <-sem }()
					path := "/mailbox/mailMessages/" + strconv.FormatInt(n.id, 10)
					q := url.Values{}
					q.Set("include_body", "1")
					payload, _, err := mailCachedGet(ctx, client, pipedrive.V1Legacy, path, q, client.TTLs.MailMessage, mode)
					if err != nil {
						mu.Lock()
						bodyErrors[n.id] = err.Error()
						mu.Unlock()
						return
					}
					_, full := extractItemData(payload)
					mu.Lock()
					if b := toStringRaw(full["body"]); b != "" {
						rawMsgs[n.idx]["body"] = b
					}
					if bc := toStringRaw(full["body_content"]); bc != "" {
						rawMsgs[n.idx]["body_content"] = bc
					}
					bodiesFetched++
					mu.Unlock()
				}(n)
			}
			wg.Wait()
		}
	}

	asAny := make([]any, 0, len(rawMsgs))
	for _, m := range rawMsgs {
		asAny = append(asAny, m)
	}
	messages := pipedrive.NormalizeMailMessageList(asAny, format)

	meta := map[string]any{
		"more":             more,
		"next_start":       nextStart,
		"bodies_requested": bodiesRequested,
		"bodies_fetched":   bodiesFetched,
		"bodies_skipped":   bodiesSkipped,
	}
	if len(bodyErrors) > 0 {
		meta["body_errors"] = bodyErrors
	}
	return internal.Wrap(map[string]any{"messages": messages}, meta), nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// mailCachedGet wraps client.NewRequest + pipedrive.Key + client.CachedGet.
// Mail endpoints live in v1 only; v2 doesn't cover the mailbox surface.
// The version argument lets callers choose V1Legacy (bare /v1/, required
// for /mailbox/*) vs V1 (the standard /api/v1/, accepted by deal
// sub-resources like /deals/{id}/mailMessages).
func mailCachedGet(
	ctx context.Context,
	client *pipedrive.Client,
	version pipedrive.APIVersion,
	path string,
	q url.Values,
	ttl time.Duration,
	mode pipedrive.CacheMode,
) (any, any, error) {
	req, err := client.NewRequest(version, http.MethodGet, path, q, nil)
	if err != nil {
		return nil, nil, err
	}
	key := pipedrive.Key(client, version, http.MethodGet, path, q, nil)
	payload, cacheMeta, err := client.CachedGet(req.WithContext(ctx), key, ttl, mode)
	if err != nil {
		return nil, nil, wrapAPIError(err)
	}
	return payload, cacheMeta, nil
}

// dealMailFetch returns the (possibly Since-filtered) raw mail-message
// maps for a deal, paging upstream as needed and unwrapping the
// {object, timestamp, data: {...}} envelope Pipedrive uses on this endpoint.
func dealMailFetch(
	ctx context.Context,
	client *pipedrive.Client,
	dealID int64,
	start, limit int,
	since string,
	mode pipedrive.CacheMode,
) ([]map[string]any, bool, int, error) {
	path := "/deals/" + strconv.FormatInt(dealID, 10) + "/mailMessages"

	pageLimit := limit
	if since != "" && pageLimit < mailFilteredPageSize {
		pageLimit = mailFilteredPageSize
	}
	hardCap := limit
	if since != "" && hardCap > mailFilteredHardCap {
		hardCap = mailFilteredHardCap
	}

	collected := make([]map[string]any, 0, hardCap)
	cursor := start
	pastCutoff := false
	more := false
	nextStart := 0

iteratePages:
	for {
		q := url.Values{}
		if cursor > 0 {
			q.Set("start", strconv.Itoa(cursor))
		}
		if pageLimit > 0 {
			q.Set("limit", strconv.Itoa(pageLimit))
		}
		payload, _, err := mailCachedGet(ctx, client, pipedrive.V1Legacy, path, q, client.TTLs.MailList, mode)
		if err != nil {
			return nil, false, 0, err
		}
		items, _ := unwrapDealMailItems(payload)
		for _, m := range items {
			if since != "" {
				ts := toStringRaw(m["message_time"])
				if ts != "" && ts < since {
					pastCutoff = true
					continue
				}
			}
			collected = append(collected, m)
			if len(collected) >= hardCap {
				pageMore, pageNext := extractMailPagination(payload)
				more = pageMore || pastCutoff
				if pageMore {
					nextStart = pageNext
				}
				break iteratePages
			}
		}
		pageMore, pageNext := extractMailPagination(payload)
		if since == "" || pastCutoff || !pageMore {
			more = pageMore
			if pageMore {
				nextStart = pageNext
			}
			break
		}
		cursor = pageNext
	}
	return collected, more, nextStart, nil
}

// unwrapDealMailItems pulls the inner data object out of each
// {object, timestamp, data: {...}} envelope that /deals/{id}/mailMessages
// returns. Tolerates flat shapes too in case Pipedrive changes the wire
// format.
func unwrapDealMailItems(payload any) ([]map[string]any, bool) {
	root, ok := payload.(map[string]any)
	if !ok {
		return nil, false
	}
	arr, ok := root["data"].([]any)
	if !ok {
		return nil, false
	}
	out := make([]map[string]any, 0, len(arr))
	for _, v := range arr {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if inner, ok := m["data"].(map[string]any); ok && (m["object"] != nil || m["timestamp"] != nil) {
			out = append(out, inner)
			continue
		}
		out = append(out, m)
	}
	return out, true
}

// extractMailPagination reads additional_data.pagination from a v1
// response and returns (more_items_in_collection, next_start).
func extractMailPagination(payload any) (bool, int) {
	m, ok := payload.(map[string]any)
	if !ok {
		return false, 0
	}
	add, ok := m["additional_data"].(map[string]any)
	if !ok {
		return false, 0
	}
	p, ok := add["pagination"].(map[string]any)
	if !ok {
		return false, 0
	}
	more, _ := p["more_items_in_collection"].(bool)
	next := int(toInt64Raw(p["next_start"]))
	return more, next
}

func toInt64Raw(v any) int64 {
	switch x := v.(type) {
	case float64:
		return int64(x)
	case int64:
		return x
	case int:
		return int64(x)
	}
	return 0
}

func toStringRaw(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// ---------------------------------------------------------------------------
// Tool declarations
// ---------------------------------------------------------------------------

var MailThreadsList = mcppipedrive.MustTool(
	"pipedrive.mail.threads.list",
	"Threads in the token owner's connected mailbox (single-user scope). Use since+unread_only for 'what needs attention?' scans. fields=compact (default) drops the parties tree and truncates snippets.",
	mailThreadsList,
	mcp.WithTitleAnnotation("List mail threads"),
	mcp.WithIdempotentHintAnnotation(true),
	mcp.WithReadOnlyHintAnnotation(true),
)

var MailThreadsGet = mcppipedrive.MustTool(
	"pipedrive.mail.threads.get",
	"One thread + optional message metadata (no bodies — call pipedrive.mail.messages.get). Single-user scope.",
	mailThreadsGet,
	mcp.WithTitleAnnotation("Get mail thread"),
	mcp.WithIdempotentHintAnnotation(true),
	mcp.WithReadOnlyHintAnnotation(true),
)

var MailMessagesGet = mcppipedrive.MustTool(
	"pipedrive.mail.messages.get",
	"One message with body. Cross-user scope: any message id resolves regardless of owning mailbox. body_format toggles text|html|none. Body cached for PIPEDRIVE_CACHE_MAIL_MESSAGE_TTL (default 24h); mutable fields can lag — use pipedrive.cache.invalidate to bust.",
	mailMessagesGet,
	mcp.WithTitleAnnotation("Get mail message"),
	mcp.WithIdempotentHintAnnotation(true),
	mcp.WithReadOnlyHintAnnotation(true),
)

var DealsMailList = mcppipedrive.MustTool(
	"pipedrive.deals.mail.list",
	"All mail tied to a deal across the workspace (cross-user — includes teammates' mail). Listing is live; bodies backfill from the mailbox endpoint with bounded concurrency, cached bodies skip the upstream call.",
	dealsMailList,
	mcp.WithTitleAnnotation("List deal mail"),
	mcp.WithIdempotentHintAnnotation(true),
	mcp.WithReadOnlyHintAnnotation(true),
)
