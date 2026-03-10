// schedule.go는 스케줄 관련 CLI 명령어를 구현합니다.
// schedule list/show/create/update/delete/toggle/logs 서브커맨드
package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/spf13/cobra"
)

// Schedule은 스케줄 기본 정보를 나타냅니다.
type Schedule struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description,omitempty"`
	CronExpression string `json:"cron_expression,omitempty"`
	Timezone       string `json:"timezone,omitempty"`
	TargetAgentID  string `json:"target_agent_id,omitempty"`
	IsActive       bool   `json:"is_active"`
	CreatedAt      string `json:"created_at,omitempty"`
}

// ScheduleLog은 스케줄 실행 로그 정보를 나타냅니다.
type ScheduleLog struct {
	ID         string `json:"id"`
	ScheduleID string `json:"schedule_id"`
	Status     string `json:"status"`
	StartedAt  string `json:"started_at,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
}

var (
	scheduleJSONOutput    bool
	scheduleName          string
	scheduleDesc          string
	scheduleCron          string
	scheduleTimezone      string
	scheduleAgentID       string
	scheduleTaskTemplate  string
	scheduleLogsLimit     int
	scheduleLogsOffset    int
)

// scheduleCmd는 schedule 서브커맨드의 루트입니다.
var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "스케줄 관련 명령어",
	Long:  `스케줄 목록 조회, 상세 조회, 생성, 수정, 삭제, 토글, 로그 조회 기능을 제공합니다.`,
}

// scheduleListCmd는 워크스페이스의 스케줄 목록을 조회합니다.
var scheduleListCmd = &cobra.Command{
	Use:   "list",
	Short: "스케줄 목록 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runScheduleList(client, os.Stdout, json)
	},
}

// scheduleShowCmd는 스케줄 상세 정보를 조회합니다.
var scheduleShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "스케줄 상세 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runScheduleShow(client, os.Stdout, args[0], json)
	},
}

// scheduleCreateCmd는 새 스케줄을 생성합니다.
var scheduleCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "스케줄 생성",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		name, _ := cmd.Flags().GetString("name")
		cron, _ := cmd.Flags().GetString("cron")
		timezone, _ := cmd.Flags().GetString("timezone")
		agentID, _ := cmd.Flags().GetString("agent")
		taskTemplate, _ := cmd.Flags().GetString("task-template")
		client.SetJSONOutput(json)
		return runScheduleCreate(client, os.Stdout, name, cron, timezone, agentID, taskTemplate, json)
	},
}

// scheduleUpdateCmd는 스케줄을 수정합니다.
var scheduleUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "스케줄 수정",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		name, _ := cmd.Flags().GetString("name")
		cron, _ := cmd.Flags().GetString("cron")
		timezone, _ := cmd.Flags().GetString("timezone")
		agentID, _ := cmd.Flags().GetString("agent")
		taskTemplate, _ := cmd.Flags().GetString("task-template")
		client.SetJSONOutput(json)
		return runScheduleUpdate(client, os.Stdout, args[0], name, cron, timezone, agentID, taskTemplate, json)
	},
}

// scheduleDeleteCmd는 스케줄을 삭제합니다.
var scheduleDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "스케줄 삭제",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runScheduleDelete(client, os.Stdout, args[0])
	},
}

// scheduleToggleCmd는 스케줄 활성/비활성 상태를 토글합니다.
var scheduleToggleCmd = &cobra.Command{
	Use:   "toggle <id>",
	Short: "스케줄 활성/비활성 토글",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runScheduleToggle(client, os.Stdout, args[0], json)
	},
}

// scheduleLogsCmd는 스케줄 실행 로그를 조회합니다.
var scheduleLogsCmd = &cobra.Command{
	Use:   "logs <id>",
	Short: "스케줄 실행 로그 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		limit, _ := cmd.Flags().GetInt("limit")
		offset, _ := cmd.Flags().GetInt("offset")
		client.SetJSONOutput(json)
		return runScheduleLogs(client, os.Stdout, args[0], limit, offset, json)
	},
}

func init() {
	rootCmd.AddCommand(scheduleCmd)
	scheduleCmd.AddCommand(scheduleListCmd)
	scheduleCmd.AddCommand(scheduleShowCmd)
	scheduleCmd.AddCommand(scheduleCreateCmd)
	scheduleCmd.AddCommand(scheduleUpdateCmd)
	scheduleCmd.AddCommand(scheduleDeleteCmd)
	scheduleCmd.AddCommand(scheduleToggleCmd)
	scheduleCmd.AddCommand(scheduleLogsCmd)

	// --json 플래그를 출력이 있는 서브커맨드에 추가
	for _, sub := range []*cobra.Command{
		scheduleListCmd, scheduleShowCmd, scheduleCreateCmd,
		scheduleUpdateCmd, scheduleToggleCmd, scheduleLogsCmd,
	} {
		sub.Flags().BoolVar(&scheduleJSONOutput, "json", false, "JSON 형식으로 출력")
	}

	// create 전용 플래그
	scheduleCreateCmd.Flags().StringVar(&scheduleName, "name", "", "스케줄 이름 (필수)")
	scheduleCreateCmd.Flags().StringVar(&scheduleCron, "cron", "", "크론 표현식")
	scheduleCreateCmd.Flags().StringVar(&scheduleTimezone, "timezone", "", "타임존 (예: Asia/Seoul)")
	scheduleCreateCmd.Flags().StringVar(&scheduleAgentID, "agent", "", "대상 에이전트 ID")
	scheduleCreateCmd.Flags().StringVar(&scheduleTaskTemplate, "task-template", "", "태스크 템플릿")

	// update 전용 플래그
	scheduleUpdateCmd.Flags().StringVar(&scheduleName, "name", "", "스케줄 이름")
	scheduleUpdateCmd.Flags().StringVar(&scheduleCron, "cron", "", "크론 표현식")
	scheduleUpdateCmd.Flags().StringVar(&scheduleTimezone, "timezone", "", "타임존")
	scheduleUpdateCmd.Flags().StringVar(&scheduleAgentID, "agent", "", "대상 에이전트 ID")
	scheduleUpdateCmd.Flags().StringVar(&scheduleTaskTemplate, "task-template", "", "태스크 템플릿")

	// logs 전용 플래그
	scheduleLogsCmd.Flags().IntVar(&scheduleLogsLimit, "limit", 0, "조회할 로그 수")
	scheduleLogsCmd.Flags().IntVar(&scheduleLogsOffset, "offset", 0, "조회 시작 위치")
}

// runScheduleList는 워크스페이스의 스케줄 목록을 조회하고 출력합니다.
func runScheduleList(client *apiclient.Client, out io.Writer, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	schedules, err := apiclient.DoList[Schedule](client, ctx, "GET",
		"/api/v1/workspaces/"+workspaceID+"/schedules", nil)
	if err != nil {
		return fmt.Errorf("스케줄 목록 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, schedules)
	}

	headers := []string{"ID", "NAME", "CRON", "TIMEZONE", "ACTIVE"}
	rows := make([][]string, len(schedules))
	for i, s := range schedules {
		// ID는 첫 8자만 표시
		shortID := s.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		rows[i] = []string{
			shortID,
			s.Name,
			s.CronExpression,
			s.Timezone,
			fmt.Sprintf("%v", s.IsActive),
		}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runScheduleShow는 스케줄 상세 정보를 출력합니다.
func runScheduleShow(client *apiclient.Client, out io.Writer, scheduleID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(scheduleID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	s, err := apiclient.Do[Schedule](client, ctx, "GET", "/api/v1/schedules/"+scheduleID, nil)
	if err != nil {
		return fmt.Errorf("스케줄 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, s)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: s.ID},
		{Key: "Name", Value: s.Name},
		{Key: "Description", Value: s.Description},
		{Key: "CronExpression", Value: s.CronExpression},
		{Key: "Timezone", Value: s.Timezone},
		{Key: "TargetAgentID", Value: s.TargetAgentID},
		{Key: "IsActive", Value: fmt.Sprintf("%v", s.IsActive)},
		{Key: "CreatedAt", Value: s.CreatedAt},
	})
	return nil
}

// runScheduleCreate는 새 스케줄을 생성합니다.
func runScheduleCreate(client *apiclient.Client, out io.Writer, name, cron, timezone, agentID, taskTemplate string, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	// 빈 값을 제외한 body 구성
	body := map[string]interface{}{}
	if name != "" {
		body["name"] = name
	}
	if cron != "" {
		body["cron_expression"] = cron
	}
	if timezone != "" {
		body["timezone"] = timezone
	}
	if agentID != "" {
		body["target_agent_id"] = agentID
	}
	if taskTemplate != "" {
		body["task_template"] = taskTemplate
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	s, err := apiclient.Do[Schedule](client, ctx, "POST",
		"/api/v1/workspaces/"+workspaceID+"/schedules", body)
	if err != nil {
		return fmt.Errorf("스케줄 생성 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, s)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: s.ID},
		{Key: "Name", Value: s.Name},
		{Key: "CronExpression", Value: s.CronExpression},
		{Key: "IsActive", Value: fmt.Sprintf("%v", s.IsActive)},
	})
	return nil
}

// runScheduleUpdate는 스케줄을 수정합니다.
func runScheduleUpdate(client *apiclient.Client, out io.Writer, scheduleID, name, cron, timezone, agentID, taskTemplate string, jsonOutput bool) error {
	if err := apiclient.ValidateID(scheduleID); err != nil {
		return err
	}

	// 제공된 필드만 포함하는 body 구성
	body := map[string]interface{}{}
	if name != "" {
		body["name"] = name
	}
	if cron != "" {
		body["cron_expression"] = cron
	}
	if timezone != "" {
		body["timezone"] = timezone
	}
	if agentID != "" {
		body["target_agent_id"] = agentID
	}
	if taskTemplate != "" {
		body["task_template"] = taskTemplate
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	s, err := apiclient.Do[Schedule](client, ctx, "PATCH", "/api/v1/schedules/"+scheduleID, body)
	if err != nil {
		return fmt.Errorf("스케줄 수정 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, s)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: s.ID},
		{Key: "Name", Value: s.Name},
		{Key: "CronExpression", Value: s.CronExpression},
		{Key: "IsActive", Value: fmt.Sprintf("%v", s.IsActive)},
	})
	return nil
}

// runScheduleDelete는 스케줄을 삭제합니다.
func runScheduleDelete(client *apiclient.Client, out io.Writer, scheduleID string) error {
	if err := apiclient.ValidateID(scheduleID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	_, err := apiclient.Do[map[string]interface{}](client, ctx, "DELETE",
		"/api/v1/schedules/"+scheduleID, nil)
	if err != nil {
		return fmt.Errorf("스케줄 삭제 실패: %w", err)
	}

	fmt.Fprintf(out, "스케줄 삭제 완료: %s\n", scheduleID)
	return nil
}

// runScheduleToggle은 스케줄 활성/비활성 상태를 토글합니다.
func runScheduleToggle(client *apiclient.Client, out io.Writer, scheduleID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(scheduleID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	s, err := apiclient.Do[Schedule](client, ctx, "PATCH",
		"/api/v1/schedules/"+scheduleID+"/toggle", nil)
	if err != nil {
		return fmt.Errorf("스케줄 토글 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, s)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: s.ID},
		{Key: "Name", Value: s.Name},
		{Key: "IsActive", Value: fmt.Sprintf("%v", s.IsActive)},
	})
	return nil
}

// runScheduleLogs는 스케줄 실행 로그를 조회하고 출력합니다.
// limit, offset이 0보다 클 때만 쿼리 파라미터로 포함합니다.
func runScheduleLogs(client *apiclient.Client, out io.Writer, scheduleID string, limit, offset int, jsonOutput bool) error {
	if err := apiclient.ValidateID(scheduleID); err != nil {
		return err
	}

	// 쿼리 파라미터 구성
	path := "/api/v1/schedules/" + scheduleID + "/logs"
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

	logs, err := apiclient.DoList[ScheduleLog](client, ctx, "GET", path, nil)
	if err != nil {
		return fmt.Errorf("스케줄 로그 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, logs)
	}

	headers := []string{"ID", "STATUS", "STARTED", "FINISHED"}
	rows := make([][]string, len(logs))
	for i, l := range logs {
		// ID는 첫 8자만 표시
		shortID := l.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		rows[i] = []string{
			shortID,
			l.Status,
			l.StartedAt,
			l.FinishedAt,
		}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}
