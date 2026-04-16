package internal

import (
	"fmt"
	"net/url"
	"strconv"
)

const (
	MinLimit     = 1
	MaxLimit     = 500
	DefaultLimit = 50
)

// NormalizeLimit validates an upstream `limit` parameter, clamping to sane bounds.
func NormalizeLimit(limit int) (int, error) {
	if limit <= 0 {
		return DefaultLimit, nil
	}
	if limit > MaxLimit {
		return 0, fmt.Errorf("limit must be between %d and %d", MinLimit, MaxLimit)
	}
	if limit < MinLimit {
		return MinLimit, nil
	}
	return limit, nil
}

// NormalizeStart returns a non-negative offset (Pipedrive v1 uses `start`).
func NormalizeStart(start int) int {
	if start < 0 {
		return 0
	}
	return start
}

// AddV2Pagination sets `limit` and optional `cursor` for Pipedrive v2 endpoints.
func AddV2Pagination(q url.Values, limit int, cursor string) (url.Values, error) {
	limit, err := NormalizeLimit(limit)
	if err != nil {
		return nil, err
	}
	q.Set("limit", strconv.Itoa(limit))
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	return q, nil
}

// AddV1Pagination sets `start` + `limit` for Pipedrive v1 endpoints.
func AddV1Pagination(q url.Values, start, limit int) (url.Values, error) {
	limit, err := NormalizeLimit(limit)
	if err != nil {
		return nil, err
	}
	q.Set("start", strconv.Itoa(NormalizeStart(start)))
	q.Set("limit", strconv.Itoa(limit))
	return q, nil
}

// PaginationMeta produces the meta block for a list response.
func PaginationMeta(limit int, cursor string, start int) (map[string]any, error) {
	limit, err := NormalizeLimit(limit)
	if err != nil {
		return nil, err
	}
	meta := map[string]any{"limit": limit}
	if cursor != "" {
		meta["cursor"] = cursor
	} else {
		meta["start"] = NormalizeStart(start)
	}
	return meta, nil
}
