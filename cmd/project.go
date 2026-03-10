// project.go는 프로젝트 관련 CLI 명령어를 구현합니다.
// project list/show/create/update/delete/add-member/remove-member 서브커맨드
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
	projectJSONOutput  bool
	projectCreateName  string
	projectPrefix      string
	projectUpdateName  string
	projectAddMemberID string
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

// projectUpdateCmd는 프로젝트 정보를 수정합니다.
var projectUpdateCmd = &cobra.Command{
	Use:   "update <project-id>",
	Short: "프로젝트 수정",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runProjectUpdate(client, os.Stdout, args[0], projectUpdateName, json)
	},
}

// projectDeleteCmd는 프로젝트를 삭제합니다.
var projectDeleteCmd = &cobra.Command{
	Use:   "delete <project-id>",
	Short: "프로젝트 삭제",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runProjectDelete(client, os.Stdout, args[0])
	},
}

// projectAddMemberCmd는 프로젝트에 멤버를 추가합니다.
var projectAddMemberCmd = &cobra.Command{
	Use:   "add-member <project-id>",
	Short: "프로젝트 멤버 추가",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runProjectAddMember(client, os.Stdout, args[0], projectAddMemberID)
	},
}

// projectRemoveMemberCmd는 프로젝트에서 멤버를 제거합니다.
var projectRemoveMemberCmd = &cobra.Command{
	Use:   "remove-member <project-id> <user-id>",
	Short: "프로젝트 멤버 제거",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runProjectRemoveMember(client, os.Stdout, args[0], args[1])
	},
}

func init() {
	rootCmd.AddCommand(projectCmd)
	projectCmd.AddCommand(projectListCmd, projectShowCmd, projectCreateCmd)
	projectCmd.AddCommand(projectUpdateCmd, projectDeleteCmd, projectAddMemberCmd, projectRemoveMemberCmd)

	// --json 플래그를 서브커맨드에 추가
	for _, sub := range []*cobra.Command{projectListCmd, projectShowCmd, projectCreateCmd, projectUpdateCmd} {
		sub.Flags().BoolVar(&projectJSONOutput, "json", false, "JSON 형식으로 출력")
	}

	// project create 전용 플래그
	projectCreateCmd.Flags().StringVar(&projectCreateName, "name", "", "프로젝트 이름 (필수)")
	projectCreateCmd.Flags().StringVar(&projectPrefix, "prefix", "", "이슈 접두사 (예: PRJ)")
	projectCreateCmd.MarkFlagRequired("name")

	// project update 전용 플래그
	projectUpdateCmd.Flags().StringVar(&projectUpdateName, "name", "", "프로젝트 이름")

	// project add-member 전용 플래그
	projectAddMemberCmd.Flags().StringVar(&projectAddMemberID, "user-id", "", "사용자 ID (필수)")
	projectAddMemberCmd.MarkFlagRequired("user-id")
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

// runProjectUpdate는 프로젝트 정보를 수정합니다.
func runProjectUpdate(client *apiclient.Client, out io.Writer, projectID, name string, jsonOutput bool) error {
	if err := apiclient.ValidateID(projectID); err != nil {
		return err
	}

	body := map[string]string{}
	if name != "" {
		body["name"] = name
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	proj, err := apiclient.Do[Project](client, ctx, "PATCH", "/api/v1/projects/"+projectID, body)
	if err != nil {
		return fmt.Errorf("프로젝트 수정 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, proj)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: proj.ID},
		{Key: "Name", Value: proj.Name},
		{Key: "Slug", Value: proj.Slug},
		{Key: "Status", Value: proj.Status},
	})
	return nil
}

// runProjectDelete는 프로젝트를 삭제합니다.
func runProjectDelete(client *apiclient.Client, out io.Writer, projectID string) error {
	if err := apiclient.ValidateID(projectID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	_, err := apiclient.Do[map[string]interface{}](client, ctx, "DELETE", "/api/v1/projects/"+projectID, nil)
	if err != nil {
		return fmt.Errorf("프로젝트 삭제 실패: %w", err)
	}

	fmt.Fprintln(out, "프로젝트 삭제 완료")
	return nil
}

// runProjectAddMember는 프로젝트에 멤버를 추가합니다.
func runProjectAddMember(client *apiclient.Client, out io.Writer, projectID, userID string) error {
	if err := apiclient.ValidateID(projectID); err != nil {
		return err
	}

	body := map[string]string{"user_id": userID}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	member, err := apiclient.Do[WorkspaceMember](client, ctx, "POST",
		"/api/v1/projects/"+projectID+"/members", body)
	if err != nil {
		return fmt.Errorf("프로젝트 멤버 추가 실패: %w", err)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: member.ID},
		{Key: "Name", Value: member.Name},
		{Key: "Role", Value: member.Role},
	})
	return nil
}

// runProjectRemoveMember는 프로젝트에서 멤버를 제거합니다.
func runProjectRemoveMember(client *apiclient.Client, out io.Writer, projectID, userID string) error {
	if err := apiclient.ValidateID(projectID); err != nil {
		return err
	}
	if err := apiclient.ValidateID(userID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	_, err := apiclient.Do[map[string]interface{}](client, ctx, "DELETE",
		"/api/v1/projects/"+projectID+"/members/"+userID, nil)
	if err != nil {
		return fmt.Errorf("프로젝트 멤버 제거 실패: %w", err)
	}

	fmt.Fprintf(out, "프로젝트 멤버 제거 완료: %s\n", userID)
	return nil
}
