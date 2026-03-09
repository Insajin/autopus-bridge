// project.go는 프로젝트 관련 CLI 명령어를 구현합니다.
// project list/show/create 서브커맨드
package cmd

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/spf13/cobra"
)

// Project는 프로젝트 정보를 나타냅니다.
type Project struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Slug         string `json:"slug,omitempty"`
	Status       string `json:"status,omitempty"`
	Prefix       string `json:"prefix,omitempty"`
	Description  string `json:"description,omitempty"`
	IssueCounter int    `json:"issue_counter,omitempty"`
	CreatedAt    string `json:"created_at,omitempty"`
}

var (
	projectJSONOutput bool
	projectCreateName string
	projectPrefix     string
)

// projectCmd는 project 서브커맨드의 루트입니다.
var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "프로젝트 관련 명령어",
	Long:  `프로젝트 목록 조회, 상세 조회, 생성 기능을 제공합니다.`,
}

// projectListCmd는 프로젝트 목록을 조회합니다.
var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "프로젝트 목록 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runProjectList(client, os.Stdout, json)
	},
}

// projectShowCmd는 프로젝트 상세 정보를 조회합니다.
var projectShowCmd = &cobra.Command{
	Use:   "show <project-id>",
	Short: "프로젝트 상세 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runProjectShow(client, os.Stdout, args[0], json)
	},
}

// projectCreateCmd는 새 프로젝트를 생성합니다.
var projectCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "프로젝트 생성",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runProjectCreate(client, os.Stdout, projectCreateName, projectPrefix, json)
	},
}

func init() {
	rootCmd.AddCommand(projectCmd)
	projectCmd.AddCommand(projectListCmd, projectShowCmd, projectCreateCmd)

	// --json 플래그를 모든 서브커맨드에 추가
	for _, sub := range []*cobra.Command{projectListCmd, projectShowCmd, projectCreateCmd} {
		sub.Flags().BoolVar(&projectJSONOutput, "json", false, "JSON 형식으로 출력")
	}

	// project create 전용 플래그
	projectCreateCmd.Flags().StringVar(&projectCreateName, "name", "", "프로젝트 이름 (필수)")
	projectCreateCmd.Flags().StringVar(&projectPrefix, "prefix", "", "이슈 접두사 (예: PRJ)")
	projectCreateCmd.MarkFlagRequired("name")
}

// runProjectList는 워크스페이스의 프로젝트 목록을 조회하고 출력합니다.
func runProjectList(client *apiclient.Client, out io.Writer, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()
	path := "/api/v1/workspaces/" + workspaceID + "/projects"

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	projects, err := apiclient.DoList[Project](client, ctx, "GET", path, nil)
	if err != nil {
		return fmt.Errorf("프로젝트 목록 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, projects)
	}

	// 테이블 형식으로 출력 (ID는 앞 8자만 표시)
	headers := []string{"ID", "NAME", "SLUG", "STATUS", "PREFIX", "ISSUES"}
	rows := make([][]string, len(projects))
	for i, p := range projects {
		// ID 앞 8자 처리
		id := p.ID
		if len(id) > 8 {
			id = id[:8]
		}
		rows[i] = []string{id, p.Name, p.Slug, p.Status, p.Prefix, strconv.Itoa(p.IssueCounter)}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runProjectShow는 프로젝트 상세 정보를 출력합니다.
func runProjectShow(client *apiclient.Client, out io.Writer, projectID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(projectID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	proj, err := apiclient.Do[Project](client, ctx, "GET", "/api/v1/projects/"+projectID, nil)
	if err != nil {
		return fmt.Errorf("프로젝트 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, proj)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: proj.ID},
		{Key: "Name", Value: proj.Name},
		{Key: "Slug", Value: proj.Slug},
		{Key: "Status", Value: proj.Status},
		{Key: "Prefix", Value: proj.Prefix},
		{Key: "Description", Value: proj.Description},
		{Key: "IssueCounter", Value: strconv.Itoa(proj.IssueCounter)},
		{Key: "CreatedAt", Value: proj.CreatedAt},
	})
	return nil
}

// runProjectCreate는 새 프로젝트를 생성하고 결과를 출력합니다.
func runProjectCreate(client *apiclient.Client, out io.Writer, name, prefix string, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()
	path := "/api/v1/workspaces/" + workspaceID + "/projects"

	// 요청 본문 구성
	body := map[string]string{"name": name}
	if prefix != "" {
		body["prefix"] = prefix
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	proj, err := apiclient.Do[Project](client, ctx, "POST", path, body)
	if err != nil {
		return fmt.Errorf("프로젝트 생성 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, proj)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: proj.ID},
		{Key: "Name", Value: proj.Name},
		{Key: "Slug", Value: proj.Slug},
		{Key: "Status", Value: proj.Status},
		{Key: "Prefix", Value: proj.Prefix},
	})
	return nil
}
