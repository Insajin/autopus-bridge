package bridgecontext

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUpsertBindingAndLoadManifest(t *testing.T) {
	root := t.TempDir()

	manifest, err := UpsertBinding(root, "ws-123", KnowledgeSourceBinding{
		SourceID:   "source-1",
		SourceType: "bridge_sync",
		SourceRoot: "docs",
		SyncMode:   "mirror",
		WriteScope: []string{"docs"},
	}, "mirror")
	if err != nil {
		t.Fatalf("UpsertBinding error: %v", err)
	}

	if manifest.WorkspaceRoot != root {
		t.Fatalf("WorkspaceRoot = %q, want %q", manifest.WorkspaceRoot, root)
	}

	loaded, err := LoadManifest(root)
	if err != nil {
		t.Fatalf("LoadManifest error: %v", err)
	}

	if len(loaded.Bindings) != 1 {
		t.Fatalf("bindings len = %d, want 1", len(loaded.Bindings))
	}
	if loaded.Bindings[0].SourceRoot != "docs" {
		t.Fatalf("SourceRoot = %q, want docs", loaded.Bindings[0].SourceRoot)
	}
}

func TestBuildRuntimeContext(t *testing.T) {
	root := t.TempDir()
	autopusDir := filepath.Join(root, ".autopus")
	if err := os.MkdirAll(autopusDir, 0755); err != nil {
		t.Fatalf("mkdir error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(autopusDir, "ignore"), []byte("node_modules/\n"), 0644); err != nil {
		t.Fatalf("write ignore error: %v", err)
	}

	ctx := BuildRuntimeContext(root, &SourceManifest{
		WorkspaceRoot: root,
		SyncMode:      "approved_write",
		Bindings: []KnowledgeSourceBinding{
			{SourceID: "a", SourceRoot: "docs", SyncMode: "approved_write", WriteScope: []string{"docs", "notes"}},
			{SourceID: "b", SourceRoot: "notes", SyncMode: "approved_write", WriteScope: []string{"notes"}},
		},
	})
	if ctx == nil {
		t.Fatal("BuildRuntimeContext returned nil")
	}
	if !ctx.IgnoreRulesLoaded {
		t.Fatal("IgnoreRulesLoaded should be true")
	}
	if ctx.SyncMode != "approved_write" {
		t.Fatalf("SyncMode = %q, want approved_write", ctx.SyncMode)
	}
	if len(ctx.WriteScope) != 2 {
		t.Fatalf("WriteScope len = %d, want 2", len(ctx.WriteScope))
	}
}
