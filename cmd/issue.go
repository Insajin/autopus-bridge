// issue.go는 이슈 관련 CLI 명령어를 구현합니다.
// issue list/show/create/update/assign/comment 서브커맨드
package cmd

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/spf13/cobra"
)

// Issue는 이슈 정보를 나타냅니다.
type Issue struct {
	ID          string `json:"id"`
	Number      int    `json:"number"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status,omitempty"`
	Priority    string `json:"priority,omitempty"`
	Type        string `json:"type,omitempty"`
	AssigneeID  string `json:"assignee_id,omitempty"`
	ReporterID  string `json:"reporter_id,omitempty"`
	DueDate     string `json:"due_date,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

// IssueComment는 이슈 댓글 정보를 나타냅니다.
type IssueComment struct {
	ID        string `json:"id"`
	Content   string `json:"content"`
	AuthorID  string `json:"author_id,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

var (
	issueJSONOutput    bool
	issueStatusFilter  string
	issuePriorityFilter string
	issueTypeFilter    string
	issueTitle         string
	issueDescription   string
	issuePriority      string
	issueType          string
	issueAssignAgent   string
)

// issueCmd는 issue 서브커맨드의 루트입니다.
var issueCmd = &cobra.Command{
	Use:   "issue",
	Short: "이슈 관련 명령어",
	Long:  `이슈 목록 조회, 상세 조회, 생성, 수정, 담당자 지정, 댓글 관리 기능을 제공합니다.`,
}

// issueListCmd는 프로젝트의 이슈 목록을 조회합니다.
var issueListCmd = &cobra.Command{
	Use:   "list <project-id>",
	Short: "이슈 목록 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		status, _ := cmd.Flags().GetString("status")
		priority, _ := cmd.Flags().GetString("priority")
		typ, _ := cmd.Flags().GetString("type")
		return runIssueList(client, os.Stdout, args[0], status, priority, typ, json)
	},
}

// issueShowCmd는 이슈 상세 정보를 조회합니다.
var issueShowCmd = &cobra.Command{
	Use:   "show <issue-id>",
	Short: "이슈 상세 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runIssueShow(client, os.Stdout, args[0], json)
	},
}

// issueCreateCmd는 새 이슈를 생성합니다.
var issueCreateCmd = &cobra.Command{
	Use:   "create <project-id>",
	Short: "이슈 생성",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		title, _ := cmd.Flags().GetString("title")
		desc, _ := cmd.Flags().GetString("description")
		pri, _ := cmd.Flags().GetString("priority")
		typ, _ := cmd.Flags().GetString("type")
		return runIssueCreate(client, os.Stdout, args[0], title, desc, pri, typ, json)
	},
}

// issueUpdateCmd는 이슈를 수정합니다.
var issueUpdateCmd = &cobra.Command{
	Use:   "update <issue-id>",
	Short: "이슈 수정",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		title, _ := cmd.Flags().GetString("title")
		desc, _ := cmd.Flags().GetString("description")
		pri, _ := cmd.Flags().GetString("priority")
		return runIssueUpdate(client, os.Stdout, args[0], title, desc, pri, json)
	},
}

// issueAssignCmd는 이슈 담당자를 지정합니다.
var issueAssignCmd = &cobra.Command{
	Use:   "assign <issue-id>",
	Short: "이슈 담당자 지정",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		agent, _ := cmd.Flags().GetString("agent")
		return runIssueAssign(client, os.Stdout, args[0], agent, json)
	},
}

// issueCommentCmd는 issue comment 서브커맨드의 루트입니다.
var issueCommentCmd = &cobra.Command{
	Use:   "comment",
	Short: "이슈 댓글 관리",
}

// issueCommentListCmd는 이슈 댓글 목록을 조회합니다.
var issueCommentListCmd = &cobra.Command{
	Use:   "list <issue-id>",
	Short: "이슈 댓글 목록 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runIssueCommentList(client, os.Stdout, args[0], json)
	},
}

// issueCommentAddCmd는 이슈에 댓글을 추가합니다.
var issueCommentAddCmd = &cobra.Command{
	Use:   "add <issue-id> <content>",
	Short: "이슈 댓글 추가",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runIssueCommentAdd(client, os.Stdout, args[0], args[1], json)
	},
}

func init() {
	rootCmd.AddCommand(issueCmd)
	issueCmd.AddCommand(issueListCmd, issueShowCmd, issueCreateCmd, issueUpdateCmd, issueAssignCmd)
	issueCmd.AddCommand(issueCommentCmd)
	issueCommentCmd.AddCommand(issueCommentListCmd, issueCommentAddCmd)

	// --json 플래그를 모든 서브커맨드에 추가
	for _, sub := range []*cobra.Command{
		issueListCmd, issueShowCmd, issueCreateCmd, issueUpdateCmd, issueAssignCmd,
		issueCommentListCmd, issueCommentAddCmd,
	} {
		sub.Flags().BoolVar(&issueJSONOutput, "json", false, "JSON 형식으로 출력")
	}

	// issue list 필터 플래그
	issueListCmd.Flags().StringVar(&issueStatusFilter, "status", "", "상태 필터 (open/closed/in_progress)")
	issueListCmd.Flags().StringVar(&issuePriorityFilter, "priority", "", "우선순위 필터 (low/medium/high/critical)")
	issueListCmd.Flags().StringVar(&issueTypeFilter, "type", "", "타입 필터 (bug/feature/task)")

	// issue create 플래그
	issueCreateCmd.Flags().StringVar(&issueTitle, "title", "", "이슈 제목 (필수)")
	issueCreateCmd.Flags().StringVar(&issueDescription, "description", "", "이슈 설명")
	issueCreateCmd.Flags().StringVar(&issuePriority, "priority", "", "우선순위")
	issueCreateCmd.Flags().StringVar(&issueType, "type", "", "이슈 타입")
	issueCreateCmd.MarkFlagRequired("title")

	// issue update 플래그
	issueUpdateCmd.Flags().String("title", "", "이슈 제목")
	issueUpdateCmd.Flags().String("description", "", "이슈 설명")
	issueUpdateCmd.Flags().String("priority", "", "우선순위")

	// issue assign 플래그
	issueAssignCmd.Flags().StringVar(&issueAssignAgent, "agent", "", "에이전트 ID (필수)")
	issueAssignCmd.MarkFlagRequired("agent")
}

// runIssueList는 프로젝트의 이슈 목록을 조회하고 출력합니다.
// status, priority, type 파라미터가 비어있지 않으면 쿼리 파라미터로 전달합니다.
func runIssueList(client *apiclient.Client, out io.Writer, projectID, status, priority, issueType string, jsonOutput bool) error {
	// 쿼리 파라미터 구성
	params := url.Values{}
	if status != "" {
		params.Set("status", status)
	}
	if priority != "" {
		params.Set("priority", priority)
	}
	if issueType != "" {
		params.Set("type", issueType)
	}

	path := "/api/v1/projects/" + projectID + "/issues"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	issues, err := apiclient.DoList[Issue](client, context.Background(), "GET", path, nil)
	if err != nil {
		return fmt.Errorf("이슈 목록 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, issues)
	}

	// 테이블 형식으로 출력
	headers := []string{"NUMBER", "TITLE", "STATUS", "PRIORITY", "TYPE", "ASSIGNEE"}
	rows := make([][]string, len(issues))
	for i, iss := range issues {
		// AssigneeID 앞 8자 또는 "-"
		assignee := "-"
		if len(iss.AssigneeID) >= 8 {
			assignee = iss.AssigneeID[:8]
		} else if iss.AssigneeID != "" {
			assignee = iss.AssigneeID
		}
		rows[i] = []string{strconv.Itoa(iss.Number), iss.Title, iss.Status, iss.Priority, iss.Type, assignee}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runIssueShow는 이슈 상세 정보를 출력합니다.
func runIssueShow(client *apiclient.Client, out io.Writer, issueID string, jsonOutput bool) error {
	iss, err := apiclient.Do[Issue](client, context.Background(), "GET", "/api/v1/issues/"+issueID, nil)
	if err != nil {
		return fmt.Errorf("이슈 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, iss)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: iss.ID},
		{Key: "Number", Value: strconv.Itoa(iss.Number)},
		{Key: "Title", Value: iss.Title},
		{Key: "Status", Value: iss.Status},
		{Key: "Priority", Value: iss.Priority},
		{Key: "Type", Value: iss.Type},
		{Key: "AssigneeID", Value: iss.AssigneeID},
		{Key: "DueDate", Value: iss.DueDate},
		{Key: "CreatedAt", Value: iss.CreatedAt},
		{Key: "Description", Value: iss.Description},
	})
	return nil
}

// runIssueCreate는 새 이슈를 생성하고 결과를 출력합니다.
func runIssueCreate(client *apiclient.Client, out io.Writer, projectID, title, description, priority, issueType string, jsonOutput bool) error {
	path := "/api/v1/projects/" + projectID + "/issues"

	// 요청 본문 구성 (비어있지 않은 필드만 포함)
	body := map[string]string{"title": title}
	if description != "" {
		body["description"] = description
	}
	if priority != "" {
		body["priority"] = priority
	}
	if issueType != "" {
		body["type"] = issueType
	}

	iss, err := apiclient.Do[Issue](client, context.Background(), "POST", path, body)
	if err != nil {
		return fmt.Errorf("이슈 생성 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, iss)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: iss.ID},
		{Key: "Number", Value: strconv.Itoa(iss.Number)},
		{Key: "Title", Value: iss.Title},
		{Key: "Status", Value: iss.Status},
		{Key: "Priority", Value: iss.Priority},
		{Key: "Type", Value: iss.Type},
	})
	return nil
}

// runIssueUpdate는 이슈를 수정하고 결과를 출력합니다.
// 비어있지 않은 필드만 요청 본문에 포함합니다.
func runIssueUpdate(client *apiclient.Client, out io.Writer, issueID, title, description, priority string, jsonOutput bool) error {
	// 수정할 필드만 포함
	body := map[string]string{}
	if title != "" {
		body["title"] = title
	}
	if description != "" {
		body["description"] = description
	}
	if priority != "" {
		body["priority"] = priority
	}

	iss, err := apiclient.Do[Issue](client, context.Background(), "PATCH", "/api/v1/issues/"+issueID, body)
	if err != nil {
		return fmt.Errorf("이슈 수정 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, iss)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: iss.ID},
		{Key: "Title", Value: iss.Title},
		{Key: "Status", Value: iss.Status},
	})
	return nil
}

// runIssueAssign은 이슈 담당자를 지정하고 결과를 출력합니다.
func runIssueAssign(client *apiclient.Client, out io.Writer, issueID, agentID string, jsonOutput bool) error {
	body := map[string]string{"assignee_id": agentID}

	iss, err := apiclient.Do[Issue](client, context.Background(), "PATCH", "/api/v1/issues/"+issueID+"/assignee", body)
	if err != nil {
		return fmt.Errorf("이슈 담당자 지정 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, iss)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: iss.ID},
		{Key: "AssigneeID", Value: iss.AssigneeID},
	})
	return nil
}

// runIssueCommentList는 이슈 댓글 목록을 조회하고 출력합니다.
func runIssueCommentList(client *apiclient.Client, out io.Writer, issueID string, jsonOutput bool) error {
	comments, err := apiclient.DoList[IssueComment](client, context.Background(), "GET", "/api/v1/issues/"+issueID+"/comments", nil)
	if err != nil {
		return fmt.Errorf("댓글 목록 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, comments)
	}

	// 테이블 형식으로 출력 (ID, AuthorID는 앞 8자)
	headers := []string{"ID", "AUTHOR", "CONTENT", "CREATED"}
	rows := make([][]string, len(comments))
	for i, c := range comments {
		// ID 앞 8자 처리
		id := c.ID
		if len(id) > 8 {
			id = id[:8]
		}
		// AuthorID 앞 8자 처리
		author := c.AuthorID
		if len(author) > 8 {
			author = author[:8]
		}
		rows[i] = []string{id, author, c.Content, c.CreatedAt}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runIssueCommentAdd는 이슈에 댓글을 추가하고 결과를 출력합니다.
func runIssueCommentAdd(client *apiclient.Client, out io.Writer, issueID, content string, jsonOutput bool) error {
	body := map[string]string{"content": content}

	comment, err := apiclient.Do[IssueComment](client, context.Background(), "POST", "/api/v1/issues/"+issueID+"/comments", body)
	if err != nil {
		return fmt.Errorf("댓글 추가 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, comment)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: comment.ID},
		{Key: "Content", Value: comment.Content},
		{Key: "AuthorID", Value: comment.AuthorID},
		{Key: "CreatedAt", Value: comment.CreatedAt},
	})
	return nil
}
