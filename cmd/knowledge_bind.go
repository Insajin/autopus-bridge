package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/insajin/autopus-bridge/internal/bridgecontext"
	"github.com/insajin/autopus-bridge/internal/config"
	localproject "github.com/insajin/autopus-bridge/internal/project"
	"github.com/spf13/cobra"
)

type bridgeBindingResponse struct {
	WorkspaceID     string `json:"workspace_id"`
	SourceID        string `json:"source_id"`
	SourceType      string `json:"source_type"`
	BridgeDeviceID  string `json:"bridge_device_id,omitempty"`
	ManifestVersion string `json:"manifest_version"`
	ManifestPath    string `json:"manifest_path"`
	Binding         struct {
		ManifestVersion string   `json:"manifest_version"`
		WorkspaceRoot   string   `json:"workspace_root"`
		SourceRoot      string   `json:"source_root"`
		SyncMode        string   `json:"sync_mode"`
		WriteScope      []string `json:"write_scope,omitempty"`
		IgnoreRulesPath string   `json:"ignore_rules_path,omitempty"`
	} `json:"binding"`
}

var (
	knowledgeBindFolder     string
	knowledgeBindSourceID   string
	knowledgeBindSourceRoot string
	knowledgeBindSyncMode   string
	knowledgeBindWriteScope []string
	knowledgeBindActivate   bool
)

var knowledgeBindCmd = &cobra.Command{
	Use:   "bind",
	Short: "작업 폴더를 Knowledge Hub bridge source와 연결합니다",
	Long:  "backend bridge binding을 갱신하고 로컬 .autopus/source-manifest.json 및 active project를 함께 설정합니다.",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runKnowledgeBind(client, os.Stdout)
	},
}

func init() {
	knowledgeCmd.AddCommand(knowledgeBindCmd)

	knowledgeBindCmd.Flags().StringVar(&knowledgeBindFolder, "folder", "", "작업 폴더 루트 경로")
	knowledgeBindCmd.Flags().StringVar(&knowledgeBindSourceID, "source-id", "", "Knowledge Hub bridge source ID")
	knowledgeBindCmd.Flags().StringVar(&knowledgeBindSourceRoot, "source-root", ".", "workspace root 기준 상대 source 경로")
	knowledgeBindCmd.Flags().StringVar(&knowledgeBindSyncMode, "sync-mode", "read_only", "동기화 모드 (read_only, mirror, approved_write)")
	knowledgeBindCmd.Flags().StringArrayVar(&knowledgeBindWriteScope, "write-scope", nil, "허용 write scope (반복 가능)")
	knowledgeBindCmd.Flags().BoolVar(&knowledgeBindActivate, "activate", true, "이 폴더를 active project로 설정")

	_ = knowledgeBindCmd.MarkFlagRequired("folder")
	_ = knowledgeBindCmd.MarkFlagRequired("source-id")
}

func runKnowledgeBind(client *apiclient.Client, out io.Writer) error {
	folder, err := filepath.Abs(knowledgeBindFolder)
	if err != nil {
		return fmt.Errorf("폴더 경로 해석 실패: %w", err)
	}
	if stat, err := os.Stat(folder); err != nil || !stat.IsDir() {
		return fmt.Errorf("유효한 폴더가 아닙니다: %s", folder)
	}

	workspaceID := client.WorkspaceID()
	if workspaceID == "" {
		return fmt.Errorf("워크스페이스가 선택되지 않았습니다")
	}

	req := map[string]interface{}{
		"workspace_root": folder,
		"source_root":    knowledgeBindSourceRoot,
		"sync_mode":      knowledgeBindSyncMode,
	}
	if len(knowledgeBindWriteScope) > 0 {
		req["write_scope"] = knowledgeBindWriteScope
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	path := fmt.Sprintf("/api/v1/workspaces/%s/knowledge/sources/%s/bridge/binding", workspaceID, knowledgeBindSourceID)
	resp, err := apiclient.Do[bridgeBindingResponse](client, ctx, "PUT", path, req)
	if err != nil {
		return fmt.Errorf("bridge binding 저장 실패: %w", err)
	}

	manifest, err := bridgecontext.UpsertBinding(folder, workspaceID, bridgecontext.KnowledgeSourceBinding{
		SourceID:   resp.SourceID,
		SourceType: resp.SourceType,
		SourceRoot: resp.Binding.SourceRoot,
		SyncMode:   resp.Binding.SyncMode,
		WriteScope: append([]string(nil), resp.Binding.WriteScope...),
	}, resp.Binding.SyncMode)
	if err != nil {
		return fmt.Errorf("로컬 manifest 저장 실패: %w", err)
	}

	if knowledgeBindActivate {
		if err := upsertActiveProject(folder, workspaceID); err != nil {
			return fmt.Errorf("active project 갱신 실패: %w", err)
		}
	}

	fmt.Fprintf(out, "Knowledge Hub binding saved\n")
	fmt.Fprintf(out, "  Workspace: %s\n", workspaceID)
	fmt.Fprintf(out, "  Source ID: %s\n", resp.SourceID)
	fmt.Fprintf(out, "  Folder: %s\n", folder)
	fmt.Fprintf(out, "  Source Root: %s\n", resp.Binding.SourceRoot)
	fmt.Fprintf(out, "  Sync Mode: %s\n", resp.Binding.SyncMode)
	fmt.Fprintf(out, "  Manifest: %s\n", bridgecontext.ManifestPath(folder))
	if len(manifest.Bindings) > 0 {
		fmt.Fprintf(out, "  Bindings: %d\n", len(manifest.Bindings))
	}

	return nil
}

func upsertActiveProject(folder, workspaceID string) error {
	configDir := filepath.Dir(config.DefaultConfigPath())
	manager := localproject.NewManager(configDir)
	if err := manager.LoadProjects(); err != nil {
		return err
	}

	projectName := filepath.Base(folder)
	for _, existing := range manager.ListProjects() {
		if existing.Name == projectName && existing.Path != folder {
			if err := manager.RemoveProject(existing.Name); err != nil {
				return err
			}
			break
		}
		if strings.EqualFold(existing.Path, folder) && existing.Name != projectName {
			if err := manager.RemoveProject(existing.Name); err != nil {
				return err
			}
			break
		}
	}

	if _, ok := manager.GetProject(projectName); !ok {
		if err := manager.AddProject(localproject.Project{
			Name:      projectName,
			Path:      folder,
			Workspace: workspaceID,
			Active:    false,
		}); err != nil {
			return err
		}
	}

	if err := manager.SetActive(projectName); err != nil {
		return err
	}
	return manager.SaveProjects()
}
