// report.go는 예약 리포트 관련 CLI 명령어를 구현합니다.
// report list/show/create/update/delete/trigger/toggle 서브커맨드
package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/spf13/cobra"
)

// ScheduledReport는 예약 리포트 정보를 나타냅니다.
type ScheduledReport struct {
	ID             string `json:"id"`
	AgentID        string `json:"agent_id,omitempty"`
	ChannelID      string `json:"channel_id,omitempty"`
	ReportType     string `json:"report_type,omitempty"`
	CronExpression string `json:"cron_expression,omitempty"`
	IsActive       bool   `json:"is_active"`
	CreatedAt      string `json:"created_at,omitempty"`
}

var (
	reportJSONOutput     bool
	reportListAgentID    string
	reportAgentID        string
	reportChannelID      string
	reportType           string
	reportCron           string
	reportConfig         string
	reportUpdateChannel  string
	reportUpdateType     string
	reportUpdateCron     string
	reportUpdateActive   bool
	reportUpdateConfig   string
)

// reportCmd는 report 서브커맨드의 루트입니다.
var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "예약 리포트 관련 명령어",
	Long:  `예약 리포트 목록 조회, 상세 조회, 생성, 업데이트, 삭제, 트리거, 토글 기능을 제공합니다.`,
}

var reportListCmd = &cobra.Command{
	Use:   "list",
	Short: "예약 리포트 목록 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		agentID, _ := cmd.Flags().GetString("agent")
		client.SetJSONOutput(jsonOut)
		return runReportList(client, os.Stdout, agentID, jsonOut)
	},
}

var reportShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "예약 리포트 상세 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(jsonOut)
		return runReportShow(client, os.Stdout, args[0], jsonOut)
	},
}

var reportCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "예약 리포트 생성",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		agentID, _ := cmd.Flags().GetString("agent")
		channelID, _ := cmd.Flags().GetString("channel")
		rType, _ := cmd.Flags().GetString("type")
		cron, _ := cmd.Flags().GetString("cron")
		config, _ := cmd.Flags().GetString("config")
		client.SetJSONOutput(jsonOut)
		return runReportCreate(client, os.Stdout, agentID, channelID, rType, cron, config, jsonOut)
	},
}

var reportUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "예약 리포트 업데이트",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		channelID, _ := cmd.Flags().GetString("channel")
		rType, _ := cmd.Flags().GetString("type")
		cron, _ := cmd.Flags().GetString("cron")
		active, _ := cmd.Flags().GetBool("active")
		_ , _ = cmd.Flags().GetString("config")
		return runReportUpdate(client, os.Stdout, args[0], channelID, rType, cron, active, false)
	},
}

var reportDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "예약 리포트 삭제",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runReportDelete(client, os.Stdout, args[0])
	},
}

var reportTriggerCmd = &cobra.Command{
	Use:   "trigger <id>",
	Short: "예약 리포트 즉시 실행",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runReportTrigger(client, os.Stdout, args[0])
	},
}

var reportToggleCmd = &cobra.Command{
	Use:   "toggle <id>",
	Short: "예약 리포트 활성화 토글",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runReportToggle(client, os.Stdout, args[0])
	},
}

func init() {
	rootCmd.AddCommand(reportCmd)
	reportCmd.AddCommand(reportListCmd)
	reportCmd.AddCommand(reportShowCmd)
	reportCmd.AddCommand(reportCreateCmd)
	reportCmd.AddCommand(reportUpdateCmd)
	reportCmd.AddCommand(reportDeleteCmd)
	reportCmd.AddCommand(reportTriggerCmd)
	reportCmd.AddCommand(reportToggleCmd)

	// --json 플래그
	for _, sub := range []*cobra.Command{reportListCmd, reportShowCmd, reportCreateCmd} {
		sub.Flags().BoolVar(&reportJSONOutput, "json", false, "JSON 형식으로 출력")
	}

	// list 전용 플래그
	reportListCmd.Flags().StringVar(&reportListAgentID, "agent", "", "에이전트 ID로 필터링")

	// create 전용 플래그
	reportCreateCmd.Flags().StringVar(&reportAgentID, "agent", "", "에이전트 ID (필수)")
	reportCreateCmd.Flags().StringVar(&reportChannelID, "channel", "", "채널 ID")
	reportCreateCmd.Flags().StringVar(&reportType, "type", "", "리포트 유형")
	reportCreateCmd.Flags().StringVar(&reportCron, "cron", "", "Cron 표현식")
	reportCreateCmd.Flags().StringVar(&reportConfig, "config", "", "리포트 설정 (JSON 문자열)")

	// update 전용 플래그
	reportUpdateCmd.Flags().StringVar(&reportUpdateChannel, "channel", "", "채널 ID")
	reportUpdateCmd.Flags().StringVar(&reportUpdateType, "type", "", "리포트 유형")
	reportUpdateCmd.Flags().StringVar(&reportUpdateCron, "cron", "", "Cron 표현식")
	reportUpdateCmd.Flags().BoolVar(&reportUpdateActive, "active", false, "활성화 여부")
	reportUpdateCmd.Flags().StringVar(&reportUpdateConfig, "config", "", "리포트 설정 (JSON 문자열)")
}

// runReportList는 예약 리포트 목록을 조회하고 출력합니다.
// agentID가 제공된 경우 해당 에이전트의 리포트만 조회합니다.
func runReportList(client *apiclient.Client, out io.Writer, agentID string, jsonOutput bool) error {
	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	var path string
	if agentID != "" {
		path = "/api/v1/agents/" + agentID + "/scheduled-reports"
	} else {
		path = "/api/v1/workspaces/" + client.WorkspaceID() + "/scheduled-reports"
	}

	reports, err := apiclient.DoList[ScheduledReport](client, ctx, "GET", path, nil)
	if err != nil {
		return fmt.Errorf("리포트 목록 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, reports)
	}

	headers := []string{"ID", "AGENT_ID", "REPORT_TYPE", "CRON", "ACTIVE"}
	rows := make([][]string, len(reports))
	for i, r := range reports {
		shortID := r.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		rows[i] = []string{shortID, r.AgentID, r.ReportType, r.CronExpression, fmt.Sprintf("%v", r.IsActive)}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runReportShow는 예약 리포트 상세 정보를 출력합니다.
func runReportShow(client *apiclient.Client, out io.Writer, reportID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(reportID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	report, err := apiclient.Do[ScheduledReport](client, ctx, "GET",
		"/api/v1/scheduled-reports/"+reportID, nil)
	if err != nil {
		return fmt.Errorf("리포트 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, report)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: report.ID},
		{Key: "AgentID", Value: report.AgentID},
		{Key: "ChannelID", Value: report.ChannelID},
		{Key: "ReportType", Value: report.ReportType},
		{Key: "CronExpression", Value: report.CronExpression},
		{Key: "IsActive", Value: fmt.Sprintf("%v", report.IsActive)},
		{Key: "CreatedAt", Value: report.CreatedAt},
	})
	return nil
}

// runReportCreate는 새 예약 리포트를 생성합니다.
func runReportCreate(client *apiclient.Client, out io.Writer, agentID, channelID, rType, cron, config string, jsonOutput bool) error {
	body := map[string]interface{}{}
	if channelID != "" {
		body["channel_id"] = channelID
	}
	if rType != "" {
		body["report_type"] = rType
	}
	if cron != "" {
		body["cron_expression"] = cron
	}
	if config != "" {
		body["config"] = config
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	report, err := apiclient.Do[ScheduledReport](client, ctx, "POST",
		"/api/v1/agents/"+agentID+"/scheduled-reports", body)
	if err != nil {
		return fmt.Errorf("리포트 생성 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, report)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: report.ID},
		{Key: "AgentID", Value: report.AgentID},
		{Key: "ReportType", Value: report.ReportType},
		{Key: "CronExpression", Value: report.CronExpression},
		{Key: "IsActive", Value: fmt.Sprintf("%v", report.IsActive)},
	})
	return nil
}

// runReportUpdate는 예약 리포트를 업데이트합니다 (PUT).
func runReportUpdate(client *apiclient.Client, out io.Writer, reportID, channelID, rType, cron string, active bool, jsonOutput bool) error {
	if err := apiclient.ValidateID(reportID); err != nil {
		return err
	}

	body := map[string]interface{}{
		"is_active": active,
	}
	if channelID != "" {
		body["channel_id"] = channelID
	}
	if rType != "" {
		body["report_type"] = rType
	}
	if cron != "" {
		body["cron_expression"] = cron
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	report, err := apiclient.Do[ScheduledReport](client, ctx, "PUT",
		"/api/v1/scheduled-reports/"+reportID, body)
	if err != nil {
		return fmt.Errorf("리포트 업데이트 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, report)
	}

	fmt.Fprintf(out, "리포트 업데이트 완료: %s\n", report.ID)
	return nil
}

// runReportDelete는 예약 리포트를 삭제합니다.
func runReportDelete(client *apiclient.Client, out io.Writer, reportID string) error {
	if err := apiclient.ValidateID(reportID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	_, err := apiclient.Do[map[string]interface{}](client, ctx, "DELETE",
		"/api/v1/scheduled-reports/"+reportID, nil)
	if err != nil {
		return fmt.Errorf("리포트 삭제 실패: %w", err)
	}

	fmt.Fprintf(out, "리포트 삭제 완료: %s\n", reportID)
	return nil
}

// runReportTrigger는 예약 리포트를 즉시 실행합니다.
func runReportTrigger(client *apiclient.Client, out io.Writer, reportID string) error {
	if err := apiclient.ValidateID(reportID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	_, err := apiclient.Do[map[string]interface{}](client, ctx, "POST",
		"/api/v1/scheduled-reports/"+reportID+"/trigger", nil)
	if err != nil {
		return fmt.Errorf("리포트 트리거 실패: %w", err)
	}

	fmt.Fprintf(out, "리포트 트리거 완료: %s\n", reportID)
	return nil
}

// runReportToggle은 예약 리포트 활성화 상태를 토글합니다.
func runReportToggle(client *apiclient.Client, out io.Writer, reportID string) error {
	if err := apiclient.ValidateID(reportID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	report, err := apiclient.Do[ScheduledReport](client, ctx, "PATCH",
		"/api/v1/scheduled-reports/"+reportID+"/toggle", nil)
	if err != nil {
		return fmt.Errorf("리포트 토글 실패: %w", err)
	}

	fmt.Fprintf(out, "리포트 토글 완료: %s (활성: %v)\n", report.ID, report.IsActive)
	return nil
}
