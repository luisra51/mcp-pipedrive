package tools

// Deal products are attached to a deal with a quantity, price and optional
// discount/tax. Endpoints live under /deals/{id}/products (v2).

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"

	mcppipedrive "mcp-pipedrive"
	"mcp-pipedrive/internal"
	"mcp-pipedrive/pipedrive"
)

type DealProductsListParams struct {
	DealID     int64  `json:"deal_id" jsonschema:"description=Deal ID"`
	Limit      int    `json:"limit,omitempty" jsonschema:"description=Max results (1-500 default 50)"`
	Cursor     string `json:"cursor,omitempty" jsonschema:"description=Cursor from a previous page"`
	IncludeRaw bool   `json:"include_raw,omitempty" jsonschema:"description=If true also include raw v2 payload"`
	CacheMode  string `json:"cache_mode,omitempty" jsonschema:"description=Cache mode: default|bypass|refresh|only"`
}

type DealProductsAttachParams struct {
	DealID       int64   `json:"deal_id" jsonschema:"description=Deal ID"`
	ProductID    int64   `json:"product_id" jsonschema:"description=Product ID to attach"`
	Quantity     float64 `json:"quantity" jsonschema:"description=Quantity (required)"`
	ItemPrice    float64 `json:"item_price,omitempty" jsonschema:"description=Unit price (defaults to product price)"`
	Discount     float64 `json:"discount,omitempty" jsonschema:"description=Discount amount or percentage"`
	DiscountType string  `json:"discount_type,omitempty" jsonschema:"description=Discount type: percentage|amount"`
	Tax          float64 `json:"tax,omitempty" jsonschema:"description=Tax amount or percentage"`
	Comments     string  `json:"comments,omitempty" jsonschema:"description=Free-form notes about this attachment"`
}

type DealProductsUpdateParams struct {
	DealID         int64    `json:"deal_id" jsonschema:"description=Deal ID"`
	AttachmentID   int64    `json:"attachment_id" jsonschema:"description=Deal-product attachment ID (from list response)"`
	Quantity       *float64 `json:"quantity,omitempty" jsonschema:"description=New quantity"`
	ItemPrice      *float64 `json:"item_price,omitempty" jsonschema:"description=New unit price"`
	Discount       *float64 `json:"discount,omitempty" jsonschema:"description=New discount value"`
	DiscountType   string   `json:"discount_type,omitempty" jsonschema:"description=New discount type"`
	Tax            *float64 `json:"tax,omitempty" jsonschema:"description=New tax value"`
	Comments       string   `json:"comments,omitempty" jsonschema:"description=New comments"`
}

type DealProductsDetachParams struct {
	DealID       int64 `json:"deal_id" jsonschema:"description=Deal ID"`
	AttachmentID int64 `json:"attachment_id" jsonschema:"description=Deal-product attachment ID to remove"`
}

func dealProductsPath(dealID int64) string {
	return "/deals/" + strconv.FormatInt(dealID, 10) + "/products"
}

func dealProductsList(ctx context.Context, args DealProductsListParams) (any, error) {
	if d, err := ensureToolAllowed(ctx, "pipedrive.deals.products.list", guardRead); err != nil {
		return nil, err
	} else if d != nil {
		return d, nil
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
	q, err := internal.AddV2Pagination(url.Values{}, args.Limit, args.Cursor)
	if err != nil {
		return nil, err
	}
	path := dealProductsPath(args.DealID)
	req, err := client.NewRequest(pipedrive.V2, http.MethodGet, path, q, nil)
	if err != nil {
		return nil, err
	}
	key := pipedrive.Key(client, pipedrive.V2, http.MethodGet, path, q, nil)
	payload, cacheMeta, err := client.CachedGet(req.WithContext(ctx), key, client.TTLs.List, mode)
	if err != nil {
		return nil, wrapAPIError(err)
	}
	raw, arr := extractListData(payload)
	data := map[string]any{"products": pipedrive.NormalizeDealProductList(arr)}
	if args.IncludeRaw {
		data["raw"] = internal.MaskSensitive(raw)
	}
	meta := map[string]any{"limit": effectiveLimit(args.Limit), "cache": cacheMeta, "deal_id": args.DealID}
	if nc := extractNextCursor(raw); nc != "" {
		meta["next_cursor"] = nc
	}
	return internal.Wrap(data, meta), nil
}

func dealProductsAttach(ctx context.Context, args DealProductsAttachParams) (any, error) {
	if d, err := ensureToolAllowed(ctx, "pipedrive.deals.products.attach", guardWrite); err != nil {
		return nil, err
	} else if d != nil {
		return d, nil
	}
	if args.DealID <= 0 {
		return nil, fmt.Errorf("deal_id is required and must be > 0")
	}
	if args.ProductID <= 0 {
		return nil, fmt.Errorf("product_id is required and must be > 0")
	}
	if args.Quantity <= 0 {
		return nil, fmt.Errorf("quantity is required and must be > 0")
	}
	client, err := clientOrError(ctx)
	if err != nil {
		return nil, err
	}
	body := map[string]any{
		"product_id": args.ProductID,
		"quantity":   args.Quantity,
	}
	if args.ItemPrice != 0 {
		body["item_price"] = args.ItemPrice
	}
	if args.Discount != 0 {
		body["discount"] = args.Discount
	}
	setIfNonZero(body, "discount_type", args.DiscountType)
	if args.Tax != 0 {
		body["tax"] = args.Tax
	}
	setIfNonZero(body, "comments", args.Comments)

	path := dealProductsPath(args.DealID)
	req, err := client.NewRequest(pipedrive.V2, http.MethodPost, path, nil, body)
	if err != nil {
		return nil, err
	}
	var payload any
	if err := client.DoJSON(req.WithContext(ctx), &payload); err != nil {
		return nil, wrapAPIError(err)
	}
	invalidateDealProductsCache(client, args.DealID)
	raw, m := extractItemData(payload)
	return internal.Wrap(map[string]any{"attachment": pipedrive.NormalizeDealProduct(m), "raw": internal.MaskSensitive(raw)}, nil), nil
}

func dealProductsUpdate(ctx context.Context, args DealProductsUpdateParams) (any, error) {
	if d, err := ensureToolAllowed(ctx, "pipedrive.deals.products.update", guardWrite); err != nil {
		return nil, err
	} else if d != nil {
		return d, nil
	}
	if args.DealID <= 0 || args.AttachmentID <= 0 {
		return nil, fmt.Errorf("deal_id and attachment_id are required and must be > 0")
	}
	client, err := clientOrError(ctx)
	if err != nil {
		return nil, err
	}
	body := map[string]any{}
	if args.Quantity != nil {
		body["quantity"] = *args.Quantity
	}
	if args.ItemPrice != nil {
		body["item_price"] = *args.ItemPrice
	}
	if args.Discount != nil {
		body["discount"] = *args.Discount
	}
	setIfNonZero(body, "discount_type", args.DiscountType)
	if args.Tax != nil {
		body["tax"] = *args.Tax
	}
	setIfNonZero(body, "comments", args.Comments)
	if len(body) == 0 {
		return nil, fmt.Errorf("no fields to update")
	}
	path := dealProductsPath(args.DealID) + "/" + strconv.FormatInt(args.AttachmentID, 10)
	req, err := client.NewRequest(pipedrive.V2, http.MethodPatch, path, nil, body)
	if err != nil {
		return nil, err
	}
	var payload any
	if err := client.DoJSON(req.WithContext(ctx), &payload); err != nil {
		return nil, wrapAPIError(err)
	}
	invalidateDealProductsCache(client, args.DealID)
	raw, m := extractItemData(payload)
	return internal.Wrap(map[string]any{"attachment": pipedrive.NormalizeDealProduct(m), "raw": internal.MaskSensitive(raw)}, nil), nil
}

func dealProductsDetach(ctx context.Context, args DealProductsDetachParams) (any, error) {
	// Detaching a product from a deal does not delete the product entity
	// itself — only the attachment link — so it stays under guardWrite, not
	// guardDelete. Use pipedrive.products.delete to remove the actual product.
	if d, err := ensureToolAllowed(ctx, "pipedrive.deals.products.detach", guardWrite); err != nil {
		return nil, err
	} else if d != nil {
		return d, nil
	}
	if args.DealID <= 0 || args.AttachmentID <= 0 {
		return nil, fmt.Errorf("deal_id and attachment_id are required and must be > 0")
	}
	client, err := clientOrError(ctx)
	if err != nil {
		return nil, err
	}
	path := dealProductsPath(args.DealID) + "/" + strconv.FormatInt(args.AttachmentID, 10)
	req, err := client.NewRequest(pipedrive.V2, http.MethodDelete, path, nil, nil)
	if err != nil {
		return nil, err
	}
	var payload any
	if err := client.DoJSON(req.WithContext(ctx), &payload); err != nil {
		return nil, wrapAPIError(err)
	}
	invalidateDealProductsCache(client, args.DealID)
	return internal.Wrap(map[string]any{"detached": true, "attachment_id": args.AttachmentID, "deal_id": args.DealID, "raw": internal.MaskSensitive(payload)}, nil), nil
}

func invalidateDealProductsCache(client *pipedrive.Client, dealID int64) {
	if client == nil || dealID <= 0 {
		return
	}
	client.InvalidatePath(pipedrive.V2, http.MethodGet, dealProductsPath(dealID))
	// A deal's own cached record may also contain products_count/totals.
	client.InvalidatePath(pipedrive.V2, http.MethodGet, "/deals/"+strconv.FormatInt(dealID, 10))
}

var DealProductsList = mcppipedrive.MustTool("pipedrive.deals.products.list",
	"List products attached to a deal (Pipedrive API v2).",
	dealProductsList,
	mcp.WithTitleAnnotation("List deal products"), mcp.WithIdempotentHintAnnotation(true), mcp.WithReadOnlyHintAnnotation(true))

var DealProductsAttach = mcppipedrive.MustTool("pipedrive.deals.products.attach",
	"Attach a product to a deal with quantity and optional price/discount/tax (write). Requires PIPEDRIVE_ALLOW_WRITE=true.",
	dealProductsAttach,
	mcp.WithTitleAnnotation("Attach product to deal"))

var DealProductsUpdate = mcppipedrive.MustTool("pipedrive.deals.products.update",
	"Update an existing deal-product attachment (write). Requires PIPEDRIVE_ALLOW_WRITE=true.",
	dealProductsUpdate,
	mcp.WithTitleAnnotation("Update deal product"))

var DealProductsDetach = mcppipedrive.MustTool("pipedrive.deals.products.detach",
	"Remove a product attachment from a deal (write). Does not delete the product itself. Requires PIPEDRIVE_ALLOW_WRITE=true.",
	dealProductsDetach,
	mcp.WithTitleAnnotation("Detach product from deal"))

