package tools

// Followers expose list/add/remove semantics for deals, persons and
// organizations under a shared handler core. Each resource gets its own three
// tools so LLMs can discover the capability per-entity, but the plumbing is
// identical.

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"

	mcppipedrive "mcp-pipedrive"
	"mcp-pipedrive/internal"
	"mcp-pipedrive/pipedrive"
)

type FollowersListParams struct {
	ID        int64  `json:"id" jsonschema:"description=Parent resource ID (deal id / person id / organization id)"`
	CacheMode string `json:"cache_mode,omitempty" jsonschema:"description=Cache mode: default|bypass|refresh|only"`
}

type FollowersAddParams struct {
	ID     int64 `json:"id" jsonschema:"description=Parent resource ID"`
	UserID int64 `json:"user_id" jsonschema:"description=User ID to add as a follower"`
}

type FollowersRemoveParams struct {
	ID         int64 `json:"id" jsonschema:"description=Parent resource ID"`
	FollowerID int64 `json:"follower_id" jsonschema:"description=Follower record ID returned by the list endpoint"`
}

func followersBasePath(resource string, parentID int64) string {
	return "/" + resource + "/" + strconv.FormatInt(parentID, 10) + "/followers"
}

func followersList(ctx context.Context, resource, toolName string, args FollowersListParams) (any, error) {
	if d, err := ensureToolAllowed(ctx, toolName, guardRead); err != nil {
		return nil, err
	} else if d != nil {
		return d, nil
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
	path := followersBasePath(resource, args.ID)
	req, err := client.NewRequest(pipedrive.V2, http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	key := pipedrive.Key(client, pipedrive.V2, http.MethodGet, path, nil, nil)
	payload, cacheMeta, err := client.CachedGet(req.WithContext(ctx), key, client.TTLs.Follower, mode)
	if err != nil {
		return nil, wrapAPIError(err)
	}
	_, arr := extractListData(payload)
	return internal.Wrap(
		map[string]any{"followers": pipedrive.NormalizeFollowerList(arr)},
		map[string]any{"cache": cacheMeta},
	), nil
}

func followersAdd(ctx context.Context, resource, toolName string, args FollowersAddParams) (any, error) {
	if d, err := ensureToolAllowed(ctx, toolName, guardWrite); err != nil {
		return nil, err
	} else if d != nil {
		return d, nil
	}
	if args.ID <= 0 {
		return nil, fmt.Errorf("id is required and must be > 0")
	}
	if args.UserID <= 0 {
		return nil, fmt.Errorf("user_id is required and must be > 0")
	}
	client, err := clientOrError(ctx)
	if err != nil {
		return nil, err
	}
	path := followersBasePath(resource, args.ID)
	body := map[string]any{"user_id": args.UserID}
	req, err := client.NewRequest(pipedrive.V2, http.MethodPost, path, nil, body)
	if err != nil {
		return nil, err
	}
	var payload any
	if err := client.DoJSON(req.WithContext(ctx), &payload); err != nil {
		return nil, wrapAPIError(err)
	}
	client.InvalidatePath(pipedrive.V2, http.MethodGet, path)
	return internal.Wrap(map[string]any{"added": true, "raw": internal.MaskSensitive(payload)}, nil), nil
}

func followersRemove(ctx context.Context, resource, toolName string, args FollowersRemoveParams) (any, error) {
	// Removing a follower is a write, not a delete — the entity itself
	// persists, only the follower link is torn down. Stay on guardWrite.
	if d, err := ensureToolAllowed(ctx, toolName, guardWrite); err != nil {
		return nil, err
	} else if d != nil {
		return d, nil
	}
	if args.ID <= 0 {
		return nil, fmt.Errorf("id is required and must be > 0")
	}
	if args.FollowerID <= 0 {
		return nil, fmt.Errorf("follower_id is required and must be > 0")
	}
	client, err := clientOrError(ctx)
	if err != nil {
		return nil, err
	}
	path := followersBasePath(resource, args.ID) + "/" + strconv.FormatInt(args.FollowerID, 10)
	req, err := client.NewRequest(pipedrive.V2, http.MethodDelete, path, nil, nil)
	if err != nil {
		return nil, err
	}
	var payload any
	if err := client.DoJSON(req.WithContext(ctx), &payload); err != nil {
		return nil, wrapAPIError(err)
	}
	client.InvalidatePath(pipedrive.V2, http.MethodGet, followersBasePath(resource, args.ID))
	return internal.Wrap(map[string]any{"removed": true, "follower_id": args.FollowerID, "raw": internal.MaskSensitive(payload)}, nil), nil
}

// ---------------------------------------------------------------------------
// Per-resource tool declarations
// ---------------------------------------------------------------------------

var (
	DealFollowersList = mcppipedrive.MustTool(
		"pipedrive.deals.followers.list",
		"List followers of a deal (Pipedrive API v2).",
		func(ctx context.Context, p FollowersListParams) (any, error) {
			return followersList(ctx, "deals", "pipedrive.deals.followers.list", p)
		},
		mcp.WithTitleAnnotation("List deal followers"), mcp.WithIdempotentHintAnnotation(true), mcp.WithReadOnlyHintAnnotation(true),
	)

	DealFollowersAdd = mcppipedrive.MustTool(
		"pipedrive.deals.followers.add",
		"Add a user as follower of a deal (write). Requires PIPEDRIVE_ALLOW_WRITE=true.",
		func(ctx context.Context, p FollowersAddParams) (any, error) {
			return followersAdd(ctx, "deals", "pipedrive.deals.followers.add", p)
		},
		mcp.WithTitleAnnotation("Add deal follower"),
	)

	DealFollowersRemove = mcppipedrive.MustTool(
		"pipedrive.deals.followers.remove",
		"Remove a follower from a deal (write). Requires PIPEDRIVE_ALLOW_WRITE=true.",
		func(ctx context.Context, p FollowersRemoveParams) (any, error) {
			return followersRemove(ctx, "deals", "pipedrive.deals.followers.remove", p)
		},
		mcp.WithTitleAnnotation("Remove deal follower"),
	)

	PersonFollowersList = mcppipedrive.MustTool(
		"pipedrive.persons.followers.list",
		"List followers of a person (Pipedrive API v2).",
		func(ctx context.Context, p FollowersListParams) (any, error) {
			return followersList(ctx, "persons", "pipedrive.persons.followers.list", p)
		},
		mcp.WithTitleAnnotation("List person followers"), mcp.WithIdempotentHintAnnotation(true), mcp.WithReadOnlyHintAnnotation(true),
	)

	PersonFollowersAdd = mcppipedrive.MustTool(
		"pipedrive.persons.followers.add",
		"Add a user as follower of a person (write). Requires PIPEDRIVE_ALLOW_WRITE=true.",
		func(ctx context.Context, p FollowersAddParams) (any, error) {
			return followersAdd(ctx, "persons", "pipedrive.persons.followers.add", p)
		},
		mcp.WithTitleAnnotation("Add person follower"),
	)

	PersonFollowersRemove = mcppipedrive.MustTool(
		"pipedrive.persons.followers.remove",
		"Remove a follower from a person (write). Requires PIPEDRIVE_ALLOW_WRITE=true.",
		func(ctx context.Context, p FollowersRemoveParams) (any, error) {
			return followersRemove(ctx, "persons", "pipedrive.persons.followers.remove", p)
		},
		mcp.WithTitleAnnotation("Remove person follower"),
	)

	OrgFollowersList = mcppipedrive.MustTool(
		"pipedrive.organizations.followers.list",
		"List followers of an organization (Pipedrive API v2).",
		func(ctx context.Context, p FollowersListParams) (any, error) {
			return followersList(ctx, "organizations", "pipedrive.organizations.followers.list", p)
		},
		mcp.WithTitleAnnotation("List organization followers"), mcp.WithIdempotentHintAnnotation(true), mcp.WithReadOnlyHintAnnotation(true),
	)

	OrgFollowersAdd = mcppipedrive.MustTool(
		"pipedrive.organizations.followers.add",
		"Add a user as follower of an organization (write). Requires PIPEDRIVE_ALLOW_WRITE=true.",
		func(ctx context.Context, p FollowersAddParams) (any, error) {
			return followersAdd(ctx, "organizations", "pipedrive.organizations.followers.add", p)
		},
		mcp.WithTitleAnnotation("Add organization follower"),
	)

	OrgFollowersRemove = mcppipedrive.MustTool(
		"pipedrive.organizations.followers.remove",
		"Remove a follower from an organization (write). Requires PIPEDRIVE_ALLOW_WRITE=true.",
		func(ctx context.Context, p FollowersRemoveParams) (any, error) {
			return followersRemove(ctx, "organizations", "pipedrive.organizations.followers.remove", p)
		},
		mcp.WithTitleAnnotation("Remove organization follower"),
	)
)


