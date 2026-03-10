// workspace.go는 워크스페이스 관련 CLI 명령어를 구현합니다.
// AC08-AC12: workspace list/show/switch/members/create/update/delete/mission/add-member/remove-member/update-role 서브커맨드
package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/insajin/autopus-bridge/internal/auth"
	"github.com/spf13/cobra"
)

// Workspace는 워크스페이스 정보를 나타냅니다.
type Workspace struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Role        string `json:"role,omitempty"`
	MemberCount int    `json:"member_count,omitempty"`
}

// WorkspaceMember는 워크스페이스 멤버 정보를 나타냅니다.
type WorkspaceMember struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

var (
	workspaceJSONOutput   bool
	workspaceCreateName   string
	workspaceUpdateName   string
	workspaceUpdateDesc   string
	workspaceMissionText  string
	workspaceVisionText   string
	workspaceAddUserID    string
	workspaceAddRole      string
	workspaceUpdateRoleVal string
)

// workspaceCmd는 workspace 서브커맨드의 루트입니다.
var workspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "워크스페이스 관련 명령어",
	Long:  `워크스페이스 목록 조회, 상세 조회, 전환, 멤버 조회 기능을 제공합니다.`,
}

// workspaceListCmd는 워크스페이스 목록을 조회합니다.
var workspaceListCmd = &cobra.Command{
	Use:   "list",
	Short: "워크스페이스 목록 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runWorkspaceList(client, os.Stdout, json)
	},
}

// workspaceShowCmd는 워크스페이스 상세 정보를 조회합니다.
var workspaceShowCmd = &cobra.Command{
	Use:   "show [workspace-id]",
	Short: "워크스페이스 상세 조회",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)

		workspaceID := ""
		if len(args) > 0 {
			workspaceID = args[0]
		}
		return runWorkspaceShow(client, os.Stdout, workspaceID, json)
	},
}

// workspaceSwitchCmd는 워크스페이스를 선택적으로 전환합니다.
var workspaceSwitchCmd = &cobra.Command{
	Use:   "switch",
	Short: "워크스페이스 전환 (인터랙티브)",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}

		// credentials도 로드하여 WorkspaceID 업데이트
		creds, loadErr := auth.Load()
		if loadErr != nil {
			return loadErr
		}
		if creds == nil {
			return errors.New("로그인이 필요합니다")
		}

		return runWorkspaceSwitch(cmd.Context(), client, creds, os.Stdin, os.Stdout)
	},
}

// workspaceCreateCmd는 새 워크스페이스를 생성합니다.
var workspaceCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "워크스페이스 생성",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runWorkspaceCreate(client, os.Stdout, workspaceCreateName, json)
	},
}

// workspaceUpdateCmd는 워크스페이스 정보를 수정합니다.
var workspaceUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "워크스페이스 수정",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runWorkspaceUpdate(client, os.Stdout, workspaceUpdateName, workspaceUpdateDesc, json)
	},
}

// workspaceDeleteCmd는 워크스페이스를 삭제합니다.
var workspaceDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "워크스페이스 삭제",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runWorkspaceDelete(client, os.Stdout)
	},
}

// workspaceMissionCmd는 워크스페이스 미션/비전을 설정합니다.
var workspaceMissionCmd = &cobra.Command{
	Use:   "mission",
	Short: "워크스페이스 미션/비전 설정",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runWorkspaceMission(client, os.Stdout, workspaceMissionText, workspaceVisionText, json)
	},
}

// workspaceAddMemberCmd는 워크스페이스에 멤버를 추가합니다.
var workspaceAddMemberCmd = &cobra.Command{
	Use:   "add-member",
	Short: "워크스페이스 멤버 추가",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runWorkspaceAddMember(client, os.Stdout, workspaceAddUserID, workspaceAddRole)
	},
}

// workspaceRemoveMemberCmd는 워크스페이스에서 멤버를 제거합니다.
var workspaceRemoveMemberCmd = &cobra.Command{
	Use:   "remove-member <user-id>",
	Short: "워크스페이스 멤버 제거",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runWorkspaceRemoveMember(client, os.Stdout, args[0])
	},
}

// workspaceUpdateRoleCmd는 워크스페이스 멤버의 역할을 변경합니다.
var workspaceUpdateRoleCmd = &cobra.Command{
	Use:   "update-role <user-id>",
	Short: "워크스페이스 멤버 역할 변경",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runWorkspaceUpdateRole(client, os.Stdout, args[0], workspaceUpdateRoleVal)
	},
}

// workspaceMembersCmd는 워크스페이스 멤버 목록을 조회합니다.
var workspaceMembersCmd = &cobra.Command{
	Use:   "members [workspace-id]",
	Short: "워크스페이스 멤버 조회",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)

		workspaceID := client.WorkspaceID()
		if len(args) > 0 {
			workspaceID = args[0]
		}
		return runWorkspaceMembers(client, os.Stdout, workspaceID, json)
	},
}

func init() {
	rootCmd.AddCommand(workspaceCmd)
	workspaceCmd.AddCommand(workspaceListCmd)
	workspaceCmd.AddCommand(workspaceShowCmd)
	workspaceCmd.AddCommand(workspaceSwitchCmd)
	workspaceCmd.AddCommand(workspaceMembersCmd)
	workspaceCmd.AddCommand(workspaceCreateCmd)
	workspaceCmd.AddCommand(workspaceUpdateCmd)
	workspaceCmd.AddCommand(workspaceDeleteCmd)
	workspaceCmd.AddCommand(workspaceMissionCmd)
	workspaceCmd.AddCommand(workspaceAddMemberCmd)
	workspaceCmd.AddCommand(workspaceRemoveMemberCmd)
	workspaceCmd.AddCommand(workspaceUpdateRoleCmd)

	// --json 플래그를 서브커맨드에 추가
	for _, sub := range []*cobra.Command{workspaceListCmd, workspaceShowCmd, workspaceMembersCmd, workspaceCreateCmd, workspaceUpdateCmd, workspaceMissionCmd} {
		sub.Flags().BoolVar(&workspaceJSONOutput, "json", false, "JSON 형식으로 출력")
	}

	// workspace create 전용 플래그
	workspaceCreateCmd.Flags().StringVar(&workspaceCreateName, "name", "", "워크스페이스 이름 (필수)")
	workspaceCreateCmd.MarkFlagRequired("name")

	// workspace update 전용 플래그
	workspaceUpdateCmd.Flags().StringVar(&workspaceUpdateName, "name", "", "새 워크스페이스 이름")
	workspaceUpdateCmd.Flags().StringVar(&workspaceUpdateDesc, "description", "", "워크스페이스 설명")

	// workspace mission 전용 플래그
	workspaceMissionCmd.Flags().StringVar(&workspaceMissionText, "mission", "", "미션")
	workspaceMissionCmd.Flags().StringVar(&workspaceVisionText, "vision", "", "비전")

	// workspace add-member 전용 플래그
	workspaceAddMemberCmd.Flags().StringVar(&workspaceAddUserID, "user-id", "", "사용자 ID (필수)")
	workspaceAddMemberCmd.Flags().StringVar(&workspaceAddRole, "role", "", "역할 (필수)")
	workspaceAddMemberCmd.MarkFlagRequired("user-id")
	workspaceAddMemberCmd.MarkFlagRequired("role")

	// workspace update-role 전용 플래그
	workspaceUpdateRoleCmd.Flags().StringVar(&workspaceUpdateRoleVal, "role", "", "새 역할 (필수)")
	workspaceUpdateRoleCmd.MarkFlagRequired("role")
}

// runWorkspaceList는 워크스페이스 목록을 조회하고 출력합니다.
func runWorkspaceList(client *apiclient.Client, out io.Writer, jsonOutput bool) error {
	workspaces, err := apiclient.DoList[Workspace](client, context.Background(), "GET", "/api/v1/workspaces", nil)
	if err != nil {
		return fmt.Errorf("워크스페이스 목록 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, workspaces)
	}

	// 테이블 형식으로 출력
	headers := []string{"ID", "NAME", "SLUG", "ROLE"}
	rows := make([][]string, len(workspaces))
	for i, ws := range workspaces {
		rows[i] = []string{ws.ID, ws.Name, ws.Slug, ws.Role}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runWorkspaceShow는 워크스페이스 상세 정보를 출력합니다.
// workspaceID가 비어있으면 현재 로그인된 워크스페이스를 사용합니다.
func runWorkspaceShow(client *apiclient.Client, out io.Writer, workspaceID string, jsonOutput bool) error {
	// workspaceID 미지정 시 현재 workspace 사용
	if workspaceID == "" {
		workspaceID = client.WorkspaceID()
	}
	if workspaceID == "" {
		return errors.New("워크스페이스 ID를 지정하거나 먼저 'autopus workspace switch'로 선택하세요")
	}

	ws, err := apiclient.Do[Workspace](client, context.Background(), "GET", "/api/v1/workspaces/"+workspaceID, nil)
	if err != nil {
		return fmt.Errorf("워크스페이스 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, ws)
	}

	// 상세 형식으로 출력
	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: ws.ID},
		{Key: "Name", Value: ws.Name},
		{Key: "Slug", Value: ws.Slug},
		{Key: "Role", Value: ws.Role},
	})
	return nil
}

// runWorkspaceSwitch는 인터랙티브 워크스페이스 전환을 수행합니다.
// 선택 후 credentials 파일을 업데이트합니다.
func runWorkspaceSwitch(ctx context.Context, client *apiclient.Client, creds *auth.Credentials, in io.Reader, out io.Writer) error {
	workspaces, err := apiclient.DoList[Workspace](client, ctx, "GET", "/api/v1/workspaces", nil)
	if err != nil {
		return fmt.Errorf("워크스페이스 목록 조회 실패: %w", err)
	}
	if len(workspaces) == 0 {
		return errors.New("사용 가능한 워크스페이스가 없습니다")
	}

	// 현재 선택된 워크스페이스 표시
	fmt.Fprintln(out, "워크스페이스를 선택하세요:")
	for i, ws := range workspaces {
		marker := " "
		if ws.ID == creds.WorkspaceID {
			marker = "*"
		}
		fmt.Fprintf(out, " %s %d. %s (%s)\n", marker, i+1, ws.Name, ws.Slug)
	}
	fmt.Fprint(out, "> ")

	reader := bufio.NewReader(in)
	line, readErr := reader.ReadString('\n')
	if readErr != nil && readErr != io.EOF {
		return fmt.Errorf("입력 읽기 실패: %w", readErr)
	}

	choice, parseErr := strconv.Atoi(strings.TrimSpace(line))
	if parseErr != nil {
		return errors.New("유효한 번호를 입력하세요")
	}
	if choice < 1 || choice > len(workspaces) {
		return fmt.Errorf("선택 범위를 벗어났습니다: %d", choice)
	}

	selected := workspaces[choice-1]

	// credentials 업데이트
	creds.WorkspaceID = selected.ID
	creds.WorkspaceSlug = selected.Slug
	creds.WorkspaceName = selected.Name

	if saveErr := auth.Save(creds); saveErr != nil {
		return fmt.Errorf("credentials 저장 실패: %w", saveErr)
	}

	fmt.Fprintf(out, "워크스페이스 전환 완료: %s (%s)\n", selected.Name, selected.ID)
	return nil
}

// runWorkspaceCreate는 새 워크스페이스를 생성하고 결과를 출력합니다.
func runWorkspaceCreate(client *apiclient.Client, out io.Writer, name string, jsonOutput bool) error {
	body := map[string]string{"name": name}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	ws, err := apiclient.Do[Workspace](client, ctx, "POST", "/api/v1/workspaces", body)
	if err != nil {
		return fmt.Errorf("워크스페이스 생성 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, ws)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: ws.ID},
		{Key: "Name", Value: ws.Name},
		{Key: "Slug", Value: ws.Slug},
	})
	return nil
}

// runWorkspaceUpdate는 현재 워크스페이스 정보를 수정합니다.
func runWorkspaceUpdate(client *apiclient.Client, out io.Writer, name, description string, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()
	if workspaceID == "" {
		return errors.New("워크스페이스 ID를 지정하거나 먼저 'autopus workspace switch'로 선택하세요")
	}

	body := map[string]string{}
	if name != "" {
		body["name"] = name
	}
	if description != "" {
		body["description"] = description
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	ws, err := apiclient.Do[Workspace](client, ctx, "PATCH", "/api/v1/workspaces/"+workspaceID, body)
	if err != nil {
		return fmt.Errorf("워크스페이스 수정 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, ws)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: ws.ID},
		{Key: "Name", Value: ws.Name},
		{Key: "Slug", Value: ws.Slug},
	})
	return nil
}

// runWorkspaceDelete는 현재 워크스페이스를 삭제합니다.
func runWorkspaceDelete(client *apiclient.Client, out io.Writer) error {
	workspaceID := client.WorkspaceID()
	if workspaceID == "" {
		return errors.New("워크스페이스 ID를 지정하거나 먼저 'autopus workspace switch'로 선택하세요")
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	_, err := apiclient.Do[map[string]interface{}](client, ctx, "DELETE", "/api/v1/workspaces/"+workspaceID, nil)
	if err != nil {
		return fmt.Errorf("워크스페이스 삭제 실패: %w", err)
	}

	fmt.Fprintln(out, "워크스페이스 삭제 완료")
	return nil
}

// runWorkspaceMission은 워크스페이스 미션/비전을 설정합니다.
func runWorkspaceMission(client *apiclient.Client, out io.Writer, mission, vision string, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()
	if workspaceID == "" {
		return errors.New("워크스페이스 ID를 지정하거나 먼저 'autopus workspace switch'로 선택하세요")
	}

	body := map[string]string{
		"mission": mission,
		"vision":  vision,
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	ws, err := apiclient.Do[Workspace](client, ctx, "PATCH", "/api/v1/workspaces/"+workspaceID+"/mission", body)
	if err != nil {
		return fmt.Errorf("미션/비전 설정 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, ws)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: ws.ID},
		{Key: "Name", Value: ws.Name},
	})
	return nil
}

// runWorkspaceAddMember는 워크스페이스에 멤버를 추가합니다.
func runWorkspaceAddMember(client *apiclient.Client, out io.Writer, userID, role string) error {
	workspaceID := client.WorkspaceID()
	if workspaceID == "" {
		return errors.New("워크스페이스 ID를 지정하거나 먼저 'autopus workspace switch'로 선택하세요")
	}

	body := map[string]string{
		"user_id": userID,
		"role":    role,
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	member, err := apiclient.Do[WorkspaceMember](client, ctx, "POST",
		"/api/v1/workspaces/"+workspaceID+"/members", body)
	if err != nil {
		return fmt.Errorf("멤버 추가 실패: %w", err)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: member.ID},
		{Key: "Name", Value: member.Name},
		{Key: "Role", Value: member.Role},
	})
	return nil
}

// runWorkspaceRemoveMember는 워크스페이스에서 멤버를 제거합니다.
func runWorkspaceRemoveMember(client *apiclient.Client, out io.Writer, userID string) error {
	if err := apiclient.ValidateID(userID); err != nil {
		return err
	}

	workspaceID := client.WorkspaceID()
	if workspaceID == "" {
		return errors.New("워크스페이스 ID를 지정하거나 먼저 'autopus workspace switch'로 선택하세요")
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	_, err := apiclient.Do[map[string]interface{}](client, ctx, "DELETE",
		"/api/v1/workspaces/"+workspaceID+"/members/"+userID, nil)
	if err != nil {
		return fmt.Errorf("멤버 제거 실패: %w", err)
	}

	fmt.Fprintf(out, "멤버 제거 완료: %s\n", userID)
	return nil
}

// runWorkspaceUpdateRole은 워크스페이스 멤버의 역할을 변경합니다.
func runWorkspaceUpdateRole(client *apiclient.Client, out io.Writer, userID, role string) error {
	if err := apiclient.ValidateID(userID); err != nil {
		return err
	}

	workspaceID := client.WorkspaceID()
	if workspaceID == "" {
		return errors.New("워크스페이스 ID를 지정하거나 먼저 'autopus workspace switch'로 선택하세요")
	}

	body := map[string]string{"role": role}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	member, err := apiclient.Do[WorkspaceMember](client, ctx, "PATCH",
		"/api/v1/workspaces/"+workspaceID+"/members/"+userID, body)
	if err != nil {
		return fmt.Errorf("역할 변경 실패: %w", err)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: member.ID},
		{Key: "Name", Value: member.Name},
		{Key: "Role", Value: member.Role},
	})
	return nil
}

// runWorkspaceMembers는 워크스페이스 멤버 목록을 출력합니다.
func runWorkspaceMembers(client *apiclient.Client, out io.Writer, workspaceID string, jsonOutput bool) error {
	if workspaceID == "" {
		workspaceID = client.WorkspaceID()
	}
	if workspaceID == "" {
		return errors.New("워크스페이스 ID를 지정하세요")
	}

	members, err := apiclient.DoList[WorkspaceMember](client, context.Background(), "GET",
		"/api/v1/workspaces/"+workspaceID+"/members", nil)
	if err != nil {
		return fmt.Errorf("멤버 목록 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, members)
	}

	headers := []string{"ID", "NAME", "EMAIL", "ROLE"}
	rows := make([][]string, len(members))
	for i, m := range members {
		rows[i] = []string{m.ID, m.Name, m.Email, m.Role}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}
