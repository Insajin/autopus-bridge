// sprint.go는 스프린트 관련 CLI 명령어를 구현합니다.
// sprint list/show/create/update/delete/start/complete/add-issue/remove-issue/issues 서브커맨드
package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/spf13/cobra"
)

// Sprint은 스프린트 기본 정보를 나타냅니다.
type Sprint struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Goal       string `json:"goal,omitempty"`
	Status     string `json:"status,omitempty"`
	StartDate  string `json:"start_date,omitempty"`
	EndDate    string `json:"end_date,omitempty"`
	IssueCount int    `json:"issue_count,omitempty"`
}

// SprintIssue는 스프린트에 포함된 이슈 정보를 나타냅니다.
type SprintIssue struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Status   string `json:"status,omitempty"`
	Priority int    `json:"priority,omitempty"`
}

var (
	sprintJSONOutput bool
	sprintName       string
	sprintGoal       string
	sprintStartDate  string
	sprintEndDate    string
)

// sprintCmd는 sprint 서브커맨드의 루트입니다.
var sprintCmd = &cobra.Command{
	Use:   "sprint",
	Short: "스프린트 관련 명령어",
	Long:  `스프린트 목록 조회, 상세 조회, 생성, 수정, 삭제, 시작, 완료, 이슈 관리 기능을 제공합니다.`,
}

// sprintListCmd는 스프린트 목록을 조회합니다.
var sprintListCmd = &cobra.Command{
	Use:   "list <project-id>",
	Short: "스프린트 목록 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runSprintList(client, os.Stdout, args[0], json)
	},
}

// sprintShowCmd는 스프린트 상세 정보를 조회합니다.
var sprintShowCmd = &cobra.Command{
	Use:   "show <sprint-id>",
	Short: "스프린트 상세 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runSprintShow(client, os.Stdout, args[0], json)
	},
}

// sprintCreateCmd는 새 스프린트를 생성합니다.
var sprintCreateCmd = &cobra.Command{
	Use:   "create <project-id>",
	Short: "스프린트 생성",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		name, _ := cmd.Flags().GetString("name")
		goal, _ := cmd.Flags().GetString("goal")
		startDate, _ := cmd.Flags().GetString("start-date")
		endDate, _ := cmd.Flags().GetString("end-date")
		client.SetJSONOutput(json)
		return runSprintCreate(client, os.Stdout, args[0], name, goal, startDate, endDate, json)
	},
}

// sprintUpdateCmd는 스프린트를 수정합니다.
var sprintUpdateCmd = &cobra.Command{
	Use:   "update <sprint-id>",
	Short: "스프린트 수정",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		name, _ := cmd.Flags().GetString("name")
		goal, _ := cmd.Flags().GetString("goal")
		startDate, _ := cmd.Flags().GetString("start-date")
		endDate, _ := cmd.Flags().GetString("end-date")
		client.SetJSONOutput(json)
		return runSprintUpdate(client, os.Stdout, args[0], name, goal, startDate, endDate, json)
	},
}

// sprintDeleteCmd는 스프린트를 삭제합니다.
var sprintDeleteCmd = &cobra.Command{
	Use:   "delete <sprint-id>",
	Short: "스프린트 삭제",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runSprintDelete(client, os.Stdout, args[0])
	},
}

// sprintStartCmd는 스프린트를 시작합니다.
var sprintStartCmd = &cobra.Command{
	Use:   "start <sprint-id>",
	Short: "스프린트 시작",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runSprintStart(client, os.Stdout, args[0])
	},
}

// sprintCompleteCmd는 스프린트를 완료합니다.
var sprintCompleteCmd = &cobra.Command{
	Use:   "complete <sprint-id>",
	Short: "스프린트 완료",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runSprintComplete(client, os.Stdout, args[0])
	},
}

// sprintAddIssueCmd는 스프린트에 이슈를 추가합니다.
var sprintAddIssueCmd = &cobra.Command{
	Use:   "add-issue <sprint-id> <issue-id>",
	Short: "스프린트에 이슈 추가",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runSprintAddIssue(client, os.Stdout, args[0], args[1])
	},
}

// sprintRemoveIssueCmd는 스프린트에서 이슈를 제거합니다.
var sprintRemoveIssueCmd = &cobra.Command{
	Use:   "remove-issue <sprint-id> <issue-id>",
	Short: "스프린트에서 이슈 제거",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runSprintRemoveIssue(client, os.Stdout, args[0], args[1])
	},
}

// sprintIssuesCmd는 스프린트의 이슈 목록을 조회합니다.
var sprintIssuesCmd = &cobra.Command{
	Use:   "issues <sprint-id>",
	Short: "스프린트 이슈 목록 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runSprintIssues(client, os.Stdout, args[0], json)
	},
}

func init() {
	rootCmd.AddCommand(sprintCmd)
	sprintCmd.AddCommand(sprintListCmd)
	sprintCmd.AddCommand(sprintShowCmd)
	sprintCmd.AddCommand(sprintCreateCmd)
	sprintCmd.AddCommand(sprintUpdateCmd)
	sprintCmd.AddCommand(sprintDeleteCmd)
	sprintCmd.AddCommand(sprintStartCmd)
	sprintCmd.AddCommand(sprintCompleteCmd)
	sprintCmd.AddCommand(sprintAddIssueCmd)
	sprintCmd.AddCommand(sprintRemoveIssueCmd)
	sprintCmd.AddCommand(sprintIssuesCmd)

	// --json 플래그를 출력이 있는 서브커맨드에 추가
	for _, sub := range []*cobra.Command{sprintListCmd, sprintShowCmd, sprintCreateCmd, sprintUpdateCmd, sprintIssuesCmd} {
		sub.Flags().BoolVar(&sprintJSONOutput, "json", false, "JSON 형식으로 출력")
	}

	// create/update 전용 플래그
	for _, sub := range []*cobra.Command{sprintCreateCmd, sprintUpdateCmd} {
		sub.Flags().StringVar(&sprintName, "name", "", "스프린트 이름")
		sub.Flags().StringVar(&sprintGoal, "goal", "", "스프린트 목표")
		sub.Flags().StringVar(&sprintStartDate, "start-date", "", "시작일 (YYYY-MM-DD)")
		sub.Flags().StringVar(&sprintEndDate, "end-date", "", "종료일 (YYYY-MM-DD)")
	}
}

// runSprintList는 스프린트 목록을 조회하고 출력합니다.
func runSprintList(client *apiclient.Client, out io.Writer, projectID string, jsonOutput bool) error {
	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	sprints, err := apiclient.DoList[Sprint](client, ctx, "GET",
		"/api/v1/projects/"+projectID+"/sprints", nil)
	if err != nil {
		return fmt.Errorf("스프린트 목록 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, sprints)
	}

	headers := []string{"ID", "NAME", "STATUS", "GOAL", "START", "END", "ISSUES"}
	rows := make([][]string, len(sprints))
	for i, sp := range sprints {
		// ID는 첫 8자만 표시
		shortID := sp.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		rows[i] = []string{
			shortID,
			sp.Name,
			sp.Status,
			sp.Goal,
			sp.StartDate,
			sp.EndDate,
			fmt.Sprintf("%d", sp.IssueCount),
		}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runSprintShow는 스프린트 상세 정보를 출력합니다.
func runSprintShow(client *apiclient.Client, out io.Writer, sprintID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(sprintID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	sp, err := apiclient.Do[Sprint](client, ctx, "GET", "/api/v1/sprints/"+sprintID, nil)
	if err != nil {
		return fmt.Errorf("스프린트 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, sp)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: sp.ID},
		{Key: "Name", Value: sp.Name},
		{Key: "Goal", Value: sp.Goal},
		{Key: "Status", Value: sp.Status},
		{Key: "StartDate", Value: sp.StartDate},
		{Key: "EndDate", Value: sp.EndDate},
		{Key: "IssueCount", Value: fmt.Sprintf("%d", sp.IssueCount)},
	})
	return nil
}

// runSprintCreate는 새 스프린트를 생성합니다.
func runSprintCreate(client *apiclient.Client, out io.Writer, projectID, name, goal, startDate, endDate string, jsonOutput bool) error {
	body := map[string]interface{}{}
	if name != "" {
		body["name"] = name
	}
	if goal != "" {
		body["goal"] = goal
	}
	if startDate != "" {
		body["start_date"] = startDate
	}
	if endDate != "" {
		body["end_date"] = endDate
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	sp, err := apiclient.Do[Sprint](client, ctx, "POST",
		"/api/v1/projects/"+projectID+"/sprints", body)
	if err != nil {
		return fmt.Errorf("스프린트 생성 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, sp)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: sp.ID},
		{Key: "Name", Value: sp.Name},
		{Key: "Status", Value: sp.Status},
	})
	return nil
}

// runSprintUpdate는 스프린트 정보를 수정합니다.
func runSprintUpdate(client *apiclient.Client, out io.Writer, sprintID, name, goal, startDate, endDate string, jsonOutput bool) error {
	if err := apiclient.ValidateID(sprintID); err != nil {
		return err
	}

	// 비어 있지 않은 필드만 PATCH 바디에 포함
	body := map[string]interface{}{}
	if name != "" {
		body["name"] = name
	}
	if goal != "" {
		body["goal"] = goal
	}
	if startDate != "" {
		body["start_date"] = startDate
	}
	if endDate != "" {
		body["end_date"] = endDate
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	sp, err := apiclient.Do[Sprint](client, ctx, "PATCH", "/api/v1/sprints/"+sprintID, body)
	if err != nil {
		return fmt.Errorf("스프린트 수정 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, sp)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: sp.ID},
		{Key: "Name", Value: sp.Name},
		{Key: "Status", Value: sp.Status},
	})
	return nil
}

// runSprintDelete는 스프린트를 삭제합니다.
func runSprintDelete(client *apiclient.Client, out io.Writer, sprintID string) error {
	if err := apiclient.ValidateID(sprintID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	_, err := apiclient.Do[map[string]interface{}](client, ctx, "DELETE",
		"/api/v1/sprints/"+sprintID, nil)
	if err != nil {
		return fmt.Errorf("스프린트 삭제 실패: %w", err)
	}

	fmt.Fprintf(out, "스프린트 삭제 완료: %s\n", sprintID)
	return nil
}

// runSprintStart는 스프린트를 시작합니다.
func runSprintStart(client *apiclient.Client, out io.Writer, sprintID string) error {
	if err := apiclient.ValidateID(sprintID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	sp, err := apiclient.Do[Sprint](client, ctx, "POST", "/api/v1/sprints/"+sprintID+"/start", nil)
	if err != nil {
		return fmt.Errorf("스프린트 시작 실패: %w", err)
	}

	fmt.Fprintf(out, "스프린트 시작됨: %s (%s)\n", sp.Name, sp.Status)
	return nil
}

// runSprintComplete는 스프린트를 완료합니다.
func runSprintComplete(client *apiclient.Client, out io.Writer, sprintID string) error {
	if err := apiclient.ValidateID(sprintID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	sp, err := apiclient.Do[Sprint](client, ctx, "POST", "/api/v1/sprints/"+sprintID+"/complete", nil)
	if err != nil {
		return fmt.Errorf("스프린트 완료 실패: %w", err)
	}

	fmt.Fprintf(out, "스프린트 완료됨: %s (%s)\n", sp.Name, sp.Status)
	return nil
}

// runSprintAddIssue는 스프린트에 이슈를 추가합니다.
func runSprintAddIssue(client *apiclient.Client, out io.Writer, sprintID, issueID string) error {
	if err := apiclient.ValidateID(sprintID); err != nil {
		return err
	}
	if err := apiclient.ValidateID(issueID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	_, err := apiclient.Do[map[string]interface{}](client, ctx, "POST",
		"/api/v1/sprints/"+sprintID+"/issues/"+issueID, nil)
	if err != nil {
		return fmt.Errorf("스프린트 이슈 추가 실패: %w", err)
	}

	fmt.Fprintf(out, "이슈 추가 완료: %s → 스프린트 %s\n", issueID, sprintID)
	return nil
}

// runSprintRemoveIssue는 스프린트에서 이슈를 제거합니다.
func runSprintRemoveIssue(client *apiclient.Client, out io.Writer, sprintID, issueID string) error {
	if err := apiclient.ValidateID(sprintID); err != nil {
		return err
	}
	if err := apiclient.ValidateID(issueID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	_, err := apiclient.Do[map[string]interface{}](client, ctx, "DELETE",
		"/api/v1/sprints/"+sprintID+"/issues/"+issueID, nil)
	if err != nil {
		return fmt.Errorf("스프린트 이슈 제거 실패: %w", err)
	}

	fmt.Fprintf(out, "이슈 제거 완료: %s ← 스프린트 %s\n", issueID, sprintID)
	return nil
}

// runSprintIssues는 스프린트의 이슈 목록을 출력합니다.
func runSprintIssues(client *apiclient.Client, out io.Writer, sprintID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(sprintID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	issues, err := apiclient.DoList[SprintIssue](client, ctx, "GET",
		"/api/v1/sprints/"+sprintID+"/issues", nil)
	if err != nil {
		return fmt.Errorf("스프린트 이슈 목록 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, issues)
	}

	headers := []string{"ID", "TITLE", "STATUS", "PRIORITY"}
	rows := make([][]string, len(issues))
	for i, iss := range issues {
		// ID는 첫 8자만 표시
		shortID := iss.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		rows[i] = []string{
			shortID,
			iss.Title,
			iss.Status,
			fmt.Sprintf("%d", iss.Priority),
		}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}
