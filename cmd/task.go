// task.go는 에이전트 태스크 관련 CLI 명령어를 구현합니다.
// task list/show/create/assign/start/complete/fail/cancel/stats 서브커맨드
package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/spf13/cobra"
)

// AgentTask는 에이전트 태스크 기본 정보를 나타냅니다.
type AgentTask struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	TaskType    string `json:"task_type,omitempty"`
	Priority    int    `json:"priority,omitempty"`
	Status      string `json:"status,omitempty"`
	AgentID     string `json:"agent_id,omitempty"`
	AgentName   string `json:"agent_name,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

// TaskQueueStats는 태스크 큐 통계 정보를 나타냅니다.
type TaskQueueStats struct {
	Total    int            `json:"total"`
	ByStatus map[string]int `json:"by_status,omitempty"`
	AvgTime  string         `json:"avg_processing_time,omitempty"`
}

var (
	taskJSONOutput bool
	taskTitle      string
	taskType       string
	taskPriority   int
	taskAgentID    string
	taskDesc       string
	taskStatus     string
	taskOutput     string
	taskReason     string
)

// taskCmd는 task 서브커맨드의 루트입니다.
var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "에이전트 태스크 관련 명령어",
	Long:  `에이전트 태스크 목록 조회, 상세 조회, 생성, 할당, 시작, 완료, 실패, 취소, 통계 조회 기능을 제공합니다.`,
}

// taskListCmd는 태스크 목록을 조회합니다.
var taskListCmd = &cobra.Command{
	Use:   "list",
	Short: "태스크 목록 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		status, _ := cmd.Flags().GetString("status")
		ttype, _ := cmd.Flags().GetString("type")
		priority, _ := cmd.Flags().GetInt("priority")
		client.SetJSONOutput(json)
		return runTaskList(client, os.Stdout, status, ttype, priority, json)
	},
}

// taskShowCmd는 태스크 상세 정보를 조회합니다.
var taskShowCmd = &cobra.Command{
	Use:   "show <task-id>",
	Short: "태스크 상세 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runTaskShow(client, os.Stdout, args[0], json)
	},
}

// taskCreateCmd는 새 태스크를 생성합니다.
var taskCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "태스크 생성",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		title, _ := cmd.Flags().GetString("title")
		ttype, _ := cmd.Flags().GetString("type")
		priority, _ := cmd.Flags().GetInt("priority")
		agentID, _ := cmd.Flags().GetString("agent")
		desc, _ := cmd.Flags().GetString("description")
		client.SetJSONOutput(json)
		return runTaskCreate(client, os.Stdout, title, ttype, priority, agentID, desc, json)
	},
}

// taskAssignCmd는 태스크에 에이전트를 할당합니다.
var taskAssignCmd = &cobra.Command{
	Use:   "assign <task-id>",
	Short: "태스크 에이전트 할당",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		agentID, _ := cmd.Flags().GetString("agent")
		client.SetJSONOutput(json)
		return runTaskAssign(client, os.Stdout, args[0], agentID, json)
	},
}

// taskStartCmd는 태스크를 시작합니다.
var taskStartCmd = &cobra.Command{
	Use:   "start <task-id>",
	Short: "태스크 시작",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runTaskStart(client, os.Stdout, args[0])
	},
}

// taskCompleteCmd는 태스크를 완료합니다.
var taskCompleteCmd = &cobra.Command{
	Use:   "complete <task-id>",
	Short: "태스크 완료",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		output, _ := cmd.Flags().GetString("output")
		return runTaskComplete(client, os.Stdout, args[0], output)
	},
}

// taskFailCmd는 태스크를 실패 처리합니다.
var taskFailCmd = &cobra.Command{
	Use:   "fail <task-id>",
	Short: "태스크 실패 처리",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		reason, _ := cmd.Flags().GetString("reason")
		return runTaskFail(client, os.Stdout, args[0], reason)
	},
}

// taskCancelCmd는 태스크를 취소합니다.
var taskCancelCmd = &cobra.Command{
	Use:   "cancel <task-id>",
	Short: "태스크 취소",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runTaskCancel(client, os.Stdout, args[0])
	},
}

// taskStatsCmd는 태스크 큐 통계를 조회합니다.
var taskStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "태스크 큐 통계 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runTaskStats(client, os.Stdout)
	},
}

func init() {
	rootCmd.AddCommand(taskCmd)
	taskCmd.AddCommand(taskListCmd)
	taskCmd.AddCommand(taskShowCmd)
	taskCmd.AddCommand(taskCreateCmd)
	taskCmd.AddCommand(taskAssignCmd)
	taskCmd.AddCommand(taskStartCmd)
	taskCmd.AddCommand(taskCompleteCmd)
	taskCmd.AddCommand(taskFailCmd)
	taskCmd.AddCommand(taskCancelCmd)
	taskCmd.AddCommand(taskStatsCmd)

	// --json 플래그를 출력이 있는 서브커맨드에 추가
	for _, sub := range []*cobra.Command{taskListCmd, taskShowCmd, taskCreateCmd, taskAssignCmd} {
		sub.Flags().BoolVar(&taskJSONOutput, "json", false, "JSON 형식으로 출력")
	}

	// list 전용 필터 플래그
	taskListCmd.Flags().StringVar(&taskStatus, "status", "", "상태 필터 (pending|in_progress|completed|failed|cancelled)")
	taskListCmd.Flags().StringVar(&taskType, "type", "", "태스크 유형 필터")
	taskListCmd.Flags().IntVar(&taskPriority, "priority", 0, "우선순위 필터")

	// create 전용 플래그
	taskCreateCmd.Flags().StringVar(&taskTitle, "title", "", "태스크 제목 (필수)")
	taskCreateCmd.Flags().StringVar(&taskType, "type", "", "태스크 유형")
	taskCreateCmd.Flags().IntVar(&taskPriority, "priority", 0, "우선순위")
	taskCreateCmd.Flags().StringVar(&taskAgentID, "agent", "", "할당할 에이전트 ID")
	taskCreateCmd.Flags().StringVar(&taskDesc, "description", "", "태스크 설명")

	// assign 전용 플래그
	taskAssignCmd.Flags().StringVar(&taskAgentID, "agent", "", "할당할 에이전트 ID (필수)")

	// complete 전용 플래그
	taskCompleteCmd.Flags().StringVar(&taskOutput, "output", "", "완료 결과 (JSON 문자열)")

	// fail 전용 플래그
	taskFailCmd.Flags().StringVar(&taskReason, "reason", "", "실패 이유 (필수)")
}

// runTaskList는 태스크 목록을 조회하고 출력합니다.
// status, taskType, priority가 있으면 쿼리 스트링에 포함합니다.
func runTaskList(client *apiclient.Client, out io.Writer, status, taskType string, priority int, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	// 쿼리 파라미터 구성
	params := []string{}
	if status != "" {
		params = append(params, "status="+status)
	}
	if taskType != "" {
		params = append(params, "type="+taskType)
	}
	if priority != 0 {
		params = append(params, fmt.Sprintf("priority=%d", priority))
	}

	path := "/api/v1/workspaces/" + workspaceID + "/agent-tasks"
	if len(params) > 0 {
		path += "?" + strings.Join(params, "&")
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	tasks, err := apiclient.DoList[AgentTask](client, ctx, "GET", path, nil)
	if err != nil {
		return fmt.Errorf("태스크 목록 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, tasks)
	}

	headers := []string{"ID", "TITLE", "TYPE", "PRIORITY", "STATUS", "AGENT"}
	rows := make([][]string, len(tasks))
	for i, t := range tasks {
		// ID는 첫 8자만 표시
		shortID := t.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		rows[i] = []string{
			shortID,
			t.Title,
			t.TaskType,
			fmt.Sprintf("%d", t.Priority),
			t.Status,
			t.AgentName,
		}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runTaskShow는 태스크 상세 정보를 출력합니다.
func runTaskShow(client *apiclient.Client, out io.Writer, taskID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(taskID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	task, err := apiclient.Do[AgentTask](client, ctx, "GET", "/api/v1/agent-tasks/"+taskID, nil)
	if err != nil {
		return fmt.Errorf("태스크 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, task)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: task.ID},
		{Key: "Title", Value: task.Title},
		{Key: "Description", Value: task.Description},
		{Key: "TaskType", Value: task.TaskType},
		{Key: "Priority", Value: fmt.Sprintf("%d", task.Priority)},
		{Key: "Status", Value: task.Status},
		{Key: "AgentID", Value: task.AgentID},
		{Key: "AgentName", Value: task.AgentName},
		{Key: "CreatedAt", Value: task.CreatedAt},
	})
	return nil
}

// runTaskCreate는 새 태스크를 생성합니다.
func runTaskCreate(client *apiclient.Client, out io.Writer, title, taskType string, priority int, agentID, desc string, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	body := map[string]interface{}{}
	if title != "" {
		body["title"] = title
	}
	if taskType != "" {
		body["task_type"] = taskType
	}
	if priority != 0 {
		body["priority"] = priority
	}
	if agentID != "" {
		body["agent_id"] = agentID
	}
	if desc != "" {
		body["description"] = desc
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	task, err := apiclient.Do[AgentTask](client, ctx, "POST",
		"/api/v1/workspaces/"+workspaceID+"/agent-tasks", body)
	if err != nil {
		return fmt.Errorf("태스크 생성 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, task)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: task.ID},
		{Key: "Title", Value: task.Title},
		{Key: "Status", Value: task.Status},
	})
	return nil
}

// runTaskAssign는 태스크에 에이전트를 할당합니다.
func runTaskAssign(client *apiclient.Client, out io.Writer, taskID, agentID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(taskID); err != nil {
		return err
	}

	body := map[string]interface{}{}
	if agentID != "" {
		body["agent_id"] = agentID
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	task, err := apiclient.Do[AgentTask](client, ctx, "PATCH",
		"/api/v1/agent-tasks/"+taskID+"/assign", body)
	if err != nil {
		return fmt.Errorf("태스크 할당 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, task)
	}

	fmt.Fprintf(out, "태스크 할당 완료: %s → 에이전트 %s\n", taskID, task.AgentID)
	return nil
}

// runTaskStart는 태스크를 시작합니다.
func runTaskStart(client *apiclient.Client, out io.Writer, taskID string) error {
	if err := apiclient.ValidateID(taskID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	task, err := apiclient.Do[AgentTask](client, ctx, "POST",
		"/api/v1/agent-tasks/"+taskID+"/start", nil)
	if err != nil {
		return fmt.Errorf("태스크 시작 실패: %w", err)
	}

	fmt.Fprintf(out, "태스크 시작됨: %s (%s)\n", task.Title, task.Status)
	return nil
}

// runTaskComplete는 태스크를 완료합니다.
func runTaskComplete(client *apiclient.Client, out io.Writer, taskID, outputJSON string) error {
	if err := apiclient.ValidateID(taskID); err != nil {
		return err
	}

	var body map[string]interface{}
	if outputJSON != "" {
		body = map[string]interface{}{"output": outputJSON}
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	task, err := apiclient.Do[AgentTask](client, ctx, "POST",
		"/api/v1/agent-tasks/"+taskID+"/complete", body)
	if err != nil {
		return fmt.Errorf("태스크 완료 실패: %w", err)
	}

	fmt.Fprintf(out, "태스크 완료됨: %s (%s)\n", task.Title, task.Status)
	return nil
}

// runTaskFail는 태스크를 실패 처리합니다.
func runTaskFail(client *apiclient.Client, out io.Writer, taskID, reason string) error {
	if err := apiclient.ValidateID(taskID); err != nil {
		return err
	}

	body := map[string]interface{}{}
	if reason != "" {
		body["reason"] = reason
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	task, err := apiclient.Do[AgentTask](client, ctx, "POST",
		"/api/v1/agent-tasks/"+taskID+"/fail", body)
	if err != nil {
		return fmt.Errorf("태스크 실패 처리 오류: %w", err)
	}

	fmt.Fprintf(out, "태스크 실패 처리됨: %s (%s)\n", task.Title, task.Status)
	return nil
}

// runTaskCancel는 태스크를 취소합니다.
func runTaskCancel(client *apiclient.Client, out io.Writer, taskID string) error {
	if err := apiclient.ValidateID(taskID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	task, err := apiclient.Do[AgentTask](client, ctx, "POST",
		"/api/v1/agent-tasks/"+taskID+"/cancel", nil)
	if err != nil {
		return fmt.Errorf("태스크 취소 실패: %w", err)
	}

	fmt.Fprintf(out, "태스크 취소됨: %s (%s)\n", task.Title, task.Status)
	return nil
}

// runTaskStats는 태스크 큐 통계를 조회합니다.
func runTaskStats(client *apiclient.Client, out io.Writer) error {
	workspaceID := client.WorkspaceID()

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	stats, err := apiclient.Do[TaskQueueStats](client, ctx, "GET",
		"/api/v1/workspaces/"+workspaceID+"/agent-tasks/queue-stats", nil)
	if err != nil {
		return fmt.Errorf("태스크 통계 조회 실패: %w", err)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "Total", Value: fmt.Sprintf("%d", stats.Total)},
		{Key: "AvgTime", Value: stats.AvgTime},
	})

	if len(stats.ByStatus) > 0 {
		fmt.Fprintf(out, "\n상태별 집계:\n")
		for status, count := range stats.ByStatus {
			fmt.Fprintf(out, "  %-20s %d\n", status+":", count)
		}
	}

	return nil
}
