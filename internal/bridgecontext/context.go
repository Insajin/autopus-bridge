package bridgecontext

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	DefaultManifestVersion = "2026-03-12"
	DefaultManifestPath    = ".autopus/source-manifest.json"
	DefaultIgnoreRulesPath = ".autopus/ignore"
)

type KnowledgeSourceBinding struct {
	SourceID   string   `json:"source_id"`
	SourceType string   `json:"source_type,omitempty"`
	SourceRoot string   `json:"source_root"`
	SyncMode   string   `json:"sync_mode,omitempty"`
	WriteScope []string `json:"write_scope,omitempty"`
}

type SourceManifest struct {
	Version       string                   `json:"version"`
	WorkspaceID   string                   `json:"workspace_id,omitempty"`
	WorkspaceRoot string                   `json:"workspace_root"`
	SyncMode      string                   `json:"sync_mode,omitempty"`
	UpdatedAt     string                   `json:"updated_at,omitempty"`
	Bindings      []KnowledgeSourceBinding `json:"bindings"`
}

type RuntimeContext struct {
	WorkspaceRoot           string                   `json:"workspace_root,omitempty"`
	KnowledgeSourceBindings []KnowledgeSourceBinding `json:"knowledge_source_bindings,omitempty"`
	SyncMode                string                   `json:"sync_mode,omitempty"`
	IgnoreRulesLoaded       bool                     `json:"ignore_rules_loaded,omitempty"`
	PendingLocalChanges     bool                     `json:"pending_local_changes,omitempty"`
	WriteScope              []string                 `json:"write_scope,omitempty"`
}

func ManifestPath(root string) string {
	return filepath.Join(root, DefaultManifestPath)
}

func IgnoreRulesPath(root string) string {
	return filepath.Join(root, DefaultIgnoreRulesPath)
}

func LoadManifest(root string) (*SourceManifest, error) {
	data, err := os.ReadFile(ManifestPath(root))
	if err != nil {
		return nil, err
	}

	var manifest SourceManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("manifest 파싱 실패: %w", err)
	}

	if strings.TrimSpace(manifest.WorkspaceRoot) == "" {
		manifest.WorkspaceRoot = root
	}
	if strings.TrimSpace(manifest.Version) == "" {
		manifest.Version = DefaultManifestVersion
	}

	return &manifest, nil
}

func SaveManifest(root string, manifest *SourceManifest) error {
	if manifest == nil {
		return fmt.Errorf("manifest is nil")
	}
	if strings.TrimSpace(manifest.WorkspaceRoot) == "" {
		manifest.WorkspaceRoot = root
	}
	if strings.TrimSpace(manifest.Version) == "" {
		manifest.Version = DefaultManifestVersion
	}
	manifest.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	sort.Slice(manifest.Bindings, func(i, j int) bool {
		return manifest.Bindings[i].SourceID < manifest.Bindings[j].SourceID
	})

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("manifest 직렬화 실패: %w", err)
	}

	manifestDir := filepath.Dir(ManifestPath(root))
	if err := os.MkdirAll(manifestDir, 0755); err != nil {
		return fmt.Errorf("manifest 디렉토리 생성 실패: %w", err)
	}

	if err := os.WriteFile(ManifestPath(root), append(data, '\n'), 0644); err != nil {
		return fmt.Errorf("manifest 저장 실패: %w", err)
	}

	return nil
}

func UpsertBinding(root string, workspaceID string, binding KnowledgeSourceBinding, syncMode string) (*SourceManifest, error) {
	manifest, err := LoadManifest(root)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		manifest = &SourceManifest{
			Version:       DefaultManifestVersion,
			WorkspaceID:   workspaceID,
			WorkspaceRoot: root,
		}
	}

	if workspaceID != "" {
		manifest.WorkspaceID = workspaceID
	}
	manifest.WorkspaceRoot = root
	if strings.TrimSpace(syncMode) != "" {
		manifest.SyncMode = syncMode
	}

	updated := false
	for i := range manifest.Bindings {
		if manifest.Bindings[i].SourceID == binding.SourceID {
			manifest.Bindings[i] = binding
			updated = true
			break
		}
	}
	if !updated {
		manifest.Bindings = append(manifest.Bindings, binding)
	}

	if err := SaveManifest(root, manifest); err != nil {
		return nil, err
	}

	return manifest, nil
}

func BuildRuntimeContext(root string, manifest *SourceManifest) *RuntimeContext {
	if manifest == nil {
		return nil
	}

	writeScopeSet := make(map[string]struct{})
	for _, binding := range manifest.Bindings {
		for _, scope := range binding.WriteScope {
			scope = strings.TrimSpace(scope)
			if scope == "" {
				continue
			}
			writeScopeSet[scope] = struct{}{}
		}
	}

	writeScope := make([]string, 0, len(writeScopeSet))
	for scope := range writeScopeSet {
		writeScope = append(writeScope, scope)
	}
	sort.Strings(writeScope)

	syncMode := strings.TrimSpace(manifest.SyncMode)
	if syncMode == "" && len(manifest.Bindings) > 0 {
		syncMode = manifest.Bindings[0].SyncMode
	}
	if syncMode == "" {
		syncMode = "read_only"
	}

	_, ignoreErr := os.Stat(IgnoreRulesPath(root))

	return &RuntimeContext{
		WorkspaceRoot:           root,
		KnowledgeSourceBindings: append([]KnowledgeSourceBinding(nil), manifest.Bindings...),
		SyncMode:                syncMode,
		IgnoreRulesLoaded:       ignoreErr == nil,
		PendingLocalChanges:     false,
		WriteScope:              writeScope,
	}
}
