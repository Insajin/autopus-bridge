// content.go는 콘텐츠 캘린더 관련 CLI 명령어를 구현합니다.
// content list/show/create/update/delete/schedule/approve 서브커맨드
package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/spf13/cobra"
)

// ContentItem은 콘텐츠 캘린더 항목을 나타냅니다.
type ContentItem struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	ContentType   string `json:"content_type,omitempty"`
	Platform      string `json:"platform,omitempty"`
	Status        string `json:"status,omitempty"`
	ContentBody   string `json:"content_body,omitempty"`
	AuthorAgentID string `json:"author_agent_id,omitempty"`
	GoalID        string `json:"goal_id,omitempty"`
	CreatedAt     string `json:"created_at,omitempty"`
}

var (
	contentJSONOutput  bool
	contentListLimit   int
	contentListOffset  int
	contentTitle       string
	contentType        string
	contentPlatform    string
	contentBody        string
	contentAuthorAgent string
	contentGoal        string
)

// contentCmd는 content 서브커맨드의 루트입니다.
var contentCmd = &cobra.Command{
	Use:   "content",
	Short: "콘텐츠 캘린더 관련 명령어",
	Long:  `콘텐츠 목록 조회, 상세 조회, 생성, 수정, 삭제, 예약, 승인 기능을 제공합니다.`,
}

var contentListCmd = &cobra.Command{
	Use:   "list",
	Short: "콘텐츠 목록 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		limit, _ := cmd.Flags().GetInt("limit")
		offset, _ := cmd.Flags().GetInt("offset")
		client.SetJSONOutput(jsonOut)
		return runContentList(client, os.Stdout, limit, offset, jsonOut)
	},
}

var contentShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "콘텐츠 상세 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(jsonOut)
		return runContentShow(client, os.Stdout, args[0], jsonOut)
	},
}

var contentCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "콘텐츠 생성",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		title, _ := cmd.Flags().GetString("title")
		ctype, _ := cmd.Flags().GetString("type")
		platform, _ := cmd.Flags().GetString("platform")
		body, _ := cmd.Flags().GetString("body")
		author, _ := cmd.Flags().GetString("author")
		goal, _ := cmd.Flags().GetString("goal")
		client.SetJSONOutput(jsonOut)
		return runContentCreate(client, os.Stdout, title, ctype, platform, body, author, goal, jsonOut)
	},
}

var contentUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "콘텐츠 수정",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		title, _ := cmd.Flags().GetString("title")
		ctype, _ := cmd.Flags().GetString("type")
		platform, _ := cmd.Flags().GetString("platform")
		body, _ := cmd.Flags().GetString("body")
		author, _ := cmd.Flags().GetString("author")
		goal, _ := cmd.Flags().GetString("goal")
		client.SetJSONOutput(jsonOut)
		return runContentUpdate(client, os.Stdout, args[0], title, ctype, platform, body, author, goal, jsonOut)
	},
}

var contentDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "콘텐츠 삭제",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runContentDelete(client, os.Stdout, args[0])
	},
}

var contentScheduleCmd = &cobra.Command{
	Use:   "schedule <id>",
	Short: "콘텐츠 예약",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(jsonOut)
		return runContentSchedule(client, os.Stdout, args[0], jsonOut)
	},
}

var contentApproveCmd = &cobra.Command{
	Use:   "approve <id>",
	Short: "콘텐츠 승인",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(jsonOut)
		return runContentApprove(client, os.Stdout, args[0], jsonOut)
	},
}

func init() {
	rootCmd.AddCommand(contentCmd)
	contentCmd.AddCommand(contentListCmd)
	contentCmd.AddCommand(contentShowCmd)
	contentCmd.AddCommand(contentCreateCmd)
	contentCmd.AddCommand(contentUpdateCmd)
	contentCmd.AddCommand(contentDeleteCmd)
	contentCmd.AddCommand(contentScheduleCmd)
	contentCmd.AddCommand(contentApproveCmd)

	// --json 플래그를 서브커맨드에 추가
	for _, sub := range []*cobra.Command{
		contentListCmd, contentShowCmd, contentCreateCmd,
		contentUpdateCmd, contentScheduleCmd, contentApproveCmd,
	} {
		sub.Flags().BoolVar(&contentJSONOutput, "json", false, "JSON 형식으로 출력")
	}

	// list 전용 플래그
	contentListCmd.Flags().IntVar(&contentListLimit, "limit", 0, "최대 조회 수")
	contentListCmd.Flags().IntVar(&contentListOffset, "offset", 0, "조회 시작 오프셋")

	// create/update 공통 플래그
	for _, sub := range []*cobra.Command{contentCreateCmd, contentUpdateCmd} {
		sub.Flags().StringVar(&contentTitle, "title", "", "콘텐츠 제목")
		sub.Flags().StringVar(&contentType, "type", "", "콘텐츠 유형")
		sub.Flags().StringVar(&contentPlatform, "platform", "", "플랫폼")
		sub.Flags().StringVar(&contentBody, "body", "", "콘텐츠 본문")
		sub.Flags().StringVar(&contentAuthorAgent, "author", "", "작성자 에이전트 ID")
		sub.Flags().StringVar(&contentGoal, "goal", "", "목표 ID")
	}
}

// runContentList는 콘텐츠 목록을 조회하고 출력합니다.
func runContentList(client *apiclient.Client, out io.Writer, limit, offset int, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	path := "/api/v1/workspaces/" + workspaceID + "/content-calendar"
	sep := "?"
	if limit > 0 {
		path += sep + fmt.Sprintf("limit=%d", limit)
		sep = "&"
	}
	if offset > 0 {
		path += sep + fmt.Sprintf("offset=%d", offset)
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	items, err := apiclient.DoList[ContentItem](client, ctx, "GET", path, nil)
	if err != nil {
		return fmt.Errorf("콘텐츠 목록 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, items)
	}

	headers := []string{"ID", "TITLE", "TYPE", "PLATFORM", "STATUS"}
	rows := make([][]string, len(items))
	for i, item := range items {
		shortID := item.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		rows[i] = []string{shortID, item.Title, item.ContentType, item.Platform, item.Status}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runContentShow는 콘텐츠 상세 정보를 출력합니다.
func runContentShow(client *apiclient.Client, out io.Writer, id string, jsonOutput bool) error {
	if err := apiclient.ValidateID(id); err != nil {
		return err
	}

	workspaceID := client.WorkspaceID()

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	item, err := apiclient.Do[ContentItem](client, ctx, "GET",
		"/api/v1/workspaces/"+workspaceID+"/content-calendar/"+id, nil)
	if err != nil {
		return fmt.Errorf("콘텐츠 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, item)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: item.ID},
		{Key: "Title", Value: item.Title},
		{Key: "ContentType", Value: item.ContentType},
		{Key: "Platform", Value: item.Platform},
		{Key: "Status", Value: item.Status},
		{Key: "AuthorAgentID", Value: item.AuthorAgentID},
		{Key: "GoalID", Value: item.GoalID},
		{Key: "CreatedAt", Value: item.CreatedAt},
	})
	return nil
}

// runContentCreate는 새 콘텐츠를 생성합니다.
func runContentCreate(client *apiclient.Client, out io.Writer, title, contentType, platform, body, author, goal string, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	reqBody := map[string]string{}
	if title != "" {
		reqBody["title"] = title
	}
	if contentType != "" {
		reqBody["content_type"] = contentType
	}
	if platform != "" {
		reqBody["platform"] = platform
	}
	if body != "" {
		reqBody["content_body"] = body
	}
	if author != "" {
		reqBody["author_agent_id"] = author
	}
	if goal != "" {
		reqBody["goal_id"] = goal
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	item, err := apiclient.Do[ContentItem](client, ctx, "POST",
		"/api/v1/workspaces/"+workspaceID+"/content-calendar", reqBody)
	if err != nil {
		return fmt.Errorf("콘텐츠 생성 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, item)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: item.ID},
		{Key: "Title", Value: item.Title},
		{Key: "ContentType", Value: item.ContentType},
		{Key: "Platform", Value: item.Platform},
		{Key: "Status", Value: item.Status},
	})
	return nil
}

// runContentUpdate는 콘텐츠를 수정합니다.
func runContentUpdate(client *apiclient.Client, out io.Writer, id, title, contentType, platform, body, author, goal string, jsonOutput bool) error {
	if err := apiclient.ValidateID(id); err != nil {
		return err
	}

	workspaceID := client.WorkspaceID()

	reqBody := map[string]string{}
	if title != "" {
		reqBody["title"] = title
	}
	if contentType != "" {
		reqBody["content_type"] = contentType
	}
	if platform != "" {
		reqBody["platform"] = platform
	}
	if body != "" {
		reqBody["content_body"] = body
	}
	if author != "" {
		reqBody["author_agent_id"] = author
	}
	if goal != "" {
		reqBody["goal_id"] = goal
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	item, err := apiclient.Do[ContentItem](client, ctx, "PUT",
		"/api/v1/workspaces/"+workspaceID+"/content-calendar/"+id, reqBody)
	if err != nil {
		return fmt.Errorf("콘텐츠 수정 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, item)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: item.ID},
		{Key: "Title", Value: item.Title},
		{Key: "ContentType", Value: item.ContentType},
		{Key: "Platform", Value: item.Platform},
		{Key: "Status", Value: item.Status},
	})
	return nil
}

// runContentDelete는 콘텐츠를 삭제합니다.
func runContentDelete(client *apiclient.Client, out io.Writer, id string) error {
	if err := apiclient.ValidateID(id); err != nil {
		return err
	}

	workspaceID := client.WorkspaceID()

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	_, err := apiclient.Do[map[string]interface{}](client, ctx, "DELETE",
		"/api/v1/workspaces/"+workspaceID+"/content-calendar/"+id, nil)
	if err != nil {
		return fmt.Errorf("콘텐츠 삭제 실패: %w", err)
	}

	fmt.Fprintf(out, "콘텐츠 삭제 완료: %s\n", id)
	return nil
}

// runContentSchedule는 콘텐츠를 예약합니다.
func runContentSchedule(client *apiclient.Client, out io.Writer, id string, jsonOutput bool) error {
	if err := apiclient.ValidateID(id); err != nil {
		return err
	}

	workspaceID := client.WorkspaceID()

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	item, err := apiclient.Do[ContentItem](client, ctx, "POST",
		"/api/v1/workspaces/"+workspaceID+"/content-calendar/"+id+"/schedule", nil)
	if err != nil {
		return fmt.Errorf("콘텐츠 예약 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, item)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: item.ID},
		{Key: "Title", Value: item.Title},
		{Key: "Status", Value: item.Status},
	})
	return nil
}

// runContentApprove는 콘텐츠를 승인합니다.
func runContentApprove(client *apiclient.Client, out io.Writer, id string, jsonOutput bool) error {
	if err := apiclient.ValidateID(id); err != nil {
		return err
	}

	workspaceID := client.WorkspaceID()

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	item, err := apiclient.Do[ContentItem](client, ctx, "POST",
		"/api/v1/workspaces/"+workspaceID+"/content-calendar/"+id+"/approve", nil)
	if err != nil {
		return fmt.Errorf("콘텐츠 승인 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, item)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: item.ID},
		{Key: "Title", Value: item.Title},
		{Key: "Status", Value: item.Status},
	})
	return nil
}
