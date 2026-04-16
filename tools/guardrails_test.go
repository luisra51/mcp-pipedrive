package tools

import (
	"context"
	"testing"

	"mcp-pipedrive/pipedrive"
)

func ctxWith(cfg pipedrive.Config) context.Context {
	return pipedrive.WithConfig(context.Background(), cfg)
}

func TestEnsureToolAllowed_AllowlistBlocks(t *testing.T) {
	ctx := ctxWith(pipedrive.Config{
		AllowedTools: map[string]struct{}{"pipedrive.deals.get": {}},
	})
	if _, err := ensureToolAllowed(ctx, "pipedrive.deals.list", guardRead); err == nil {
		t.Fatal("expected allowlist block for unlisted tool")
	}
	if res, err := ensureToolAllowed(ctx, "pipedrive.deals.get", guardRead); err != nil || res != nil {
		t.Fatalf("allowed tool should pass: res=%v err=%v", res, err)
	}
}

func TestEnsureToolAllowed_WriteDisabled(t *testing.T) {
	ctx := ctxWith(pipedrive.Config{AllowWrite: false})
	res, err := ensureToolAllowed(ctx, "pipedrive.deals.create", guardWrite)
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	d, ok := res.(disabledResult)
	if !ok {
		t.Fatalf("expected disabledResult, got %T", res)
	}
	if d.Error != "write_disabled" {
		t.Fatalf("expected write_disabled, got %q", d.Error)
	}
}

func TestEnsureToolAllowed_WriteEnabledPasses(t *testing.T) {
	ctx := ctxWith(pipedrive.Config{AllowWrite: true})
	res, err := ensureToolAllowed(ctx, "pipedrive.deals.create", guardWrite)
	if err != nil || res != nil {
		t.Fatalf("write with flag should pass: res=%v err=%v", res, err)
	}
}

func TestEnsureToolAllowed_DeleteRequiresBothFlags(t *testing.T) {
	// Only write flag on: delete still blocked.
	ctx := ctxWith(pipedrive.Config{AllowWrite: true, AllowDelete: false})
	res, err := ensureToolAllowed(ctx, "pipedrive.deals.delete", guardDelete)
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	d, ok := res.(disabledResult)
	if !ok || d.Error != "delete_disabled" {
		t.Fatalf("expected delete_disabled, got %+v", res)
	}

	// Both flags: passes.
	ctx = ctxWith(pipedrive.Config{AllowWrite: true, AllowDelete: true})
	if res, err := ensureToolAllowed(ctx, "pipedrive.deals.delete", guardDelete); err != nil || res != nil {
		t.Fatalf("delete with both flags should pass: res=%v err=%v", res, err)
	}
}

func TestEnsureToolAllowed_DeleteBlocksWhenWriteOff(t *testing.T) {
	// write off must block delete even if delete=true — layered defense.
	ctx := ctxWith(pipedrive.Config{AllowWrite: false, AllowDelete: true})
	res, err := ensureToolAllowed(ctx, "pipedrive.deals.delete", guardDelete)
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	d, ok := res.(disabledResult)
	if !ok || d.Error != "write_disabled" {
		t.Fatalf("delete should hit write_disabled first: %+v", res)
	}
}

func TestEnsureToolAllowed_AdminToolsGate(t *testing.T) {
	ctx := ctxWith(pipedrive.Config{AdminToolsEnabled: false})
	res, err := ensureToolAllowed(ctx, "pipedrive.cache.clear", guardAdmin)
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	d, ok := res.(disabledResult)
	if !ok || d.Error != "admin_tools_disabled" {
		t.Fatalf("expected admin_tools_disabled, got %+v", res)
	}
}
