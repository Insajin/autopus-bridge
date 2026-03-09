// logs.go는 모니터링 관련 CLI 명령어를 구현합니다.
// logs, metrics, health 서브커맨드 제공
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/spf13/cobra"
)

// DashboardMetrics는 대시보드 메트릭 정보를 나타냅니다.
type DashboardMetrics struct {
	ActiveAgents    int     `json:"active_agents,omitempty"`
	TotalMessages   int     `json:"total_messages,omitempty"`
	TotalTasks      int     `json:"total_tasks,omitempty"`
	CompletedTasks  int     `json:"completed_tasks,omitempty"`
	AvgResponseTime float64 `json:"avg_response_time,omitempty"`
}

// OrgHealth는 조직 건강 상태를 나타냅니다.
type OrgHealth struct {
	Status     string            `json:"status"`
	Score      float64           `json:"score,omitempty"`
	Components map[string]string `json:"components,omitempty"`
}

var (
	logsAgentFilter string
	logsTypeFilter  string
	logsTail        int
	metricsJSON     bool
	healthJSON      bool
)

// logsCmd는 실시간 로그 스트리밍 명령어입니다.
var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "실시간 이벤트 로그 스트리밍",
	Long: `워크스페이스의 실시간 SSE 이벤트를 스트리밍합니다.

이벤트 타입:
  metric_update       - 메트릭 업데이트
  agent_typing        - 에이전트 응답 생성
  agent_status_change - 에이전트 상태 변경

예시:
  autopus logs
  autopus logs --agent my-agent
  autopus logs --type metric_update
  autopus logs --tail 20`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		agent, _ := cmd.Flags().GetString("agent")
		evtType, _ := cmd.Flags().GetString("type")
		tail, _ := cmd.Flags().GetInt("tail")
		return runLogs(client, os.Stdout, agent, evtType, tail)
	},
}

// metricsCmd는 워크스페이스 메트릭 조회 명령어입니다.
var metricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "워크스페이스 메트릭 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(jsonOut)
		return runMetrics(client, os.Stdout, jsonOut)
	},
}

// healthCmd는 조직 건강 상태 조회 명령어입니다.
var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "조직 건강 상태 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(jsonOut)
		return runHealth(client, os.Stdout, jsonOut)
	},
}

func init() {
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(metricsCmd)
	rootCmd.AddCommand(healthCmd)

	// logs 플래그
	logsCmd.Flags().StringVar(&logsAgentFilter, "agent", "", "에이전트 이름 또는 ID 필터")
	logsCmd.Flags().StringVar(&logsTypeFilter, "type", "", "이벤트 타입 필터 (예: metric_update)")
	logsCmd.Flags().IntVar(&logsTail, "tail", 0, "수신할 최대 이벤트 수 (0=무제한)")

	// metrics/health --json 플래그
	metricsCmd.Flags().BoolVar(&metricsJSON, "json", false, "JSON 형식으로 출력")
	healthCmd.Flags().BoolVar(&healthJSON, "json", false, "JSON 형식으로 출력")
}

// runLogs는 SSE 스트림에서 이벤트를 수신하여 출력합니다.
// agentFilter: 에이전트 이름/ID 필터 (agent_typing 이벤트에만 적용)
// typeFilter: 이벤트 타입 필터 (빈 문자열이면 모두 표시)
// tail: 수신 후 종료할 이벤트 수 (0이면 무제한)
// @MX:NOTE: signal.NotifyContext로 Ctrl+C 시 graceful shutdown 처리
func runLogs(client *apiclient.Client, out io.Writer, agentFilter, typeFilter string, tail int) error {
	token, err := client.Token()
	if err != nil {
		return fmt.Errorf("인증 토큰 획득 실패: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	sub := apiclient.NewSSESubscriber(client.BaseURL(), client.WorkspaceID(), token)
	events, errs := sub.Subscribe(ctx)

	count := 0
	for {
		select {
		case evt, ok := <-events:
			if !ok {
				// 채널 종료
				return nil
			}
			// 필터 적용
			if !shouldShowSSEEvent(evt.Type, evt.Data, agentFilter, typeFilter) {
				continue
			}
			// 이벤트 포맷 출력
			line := formatSSEEvent(evt.Type, evt.Data, agentFilter, typeFilter)
			if line != "" {
				fmt.Fprintln(out, line)
			}
			count++
			if tail > 0 && count >= tail {
				return nil
			}
		case err := <-errs:
			if err != nil {
				return fmt.Errorf("SSE 스트림 오류: %w", err)
			}
			return nil
		case <-ctx.Done():
			return nil
		}
	}
}

// shouldShowSSEEvent는 필터 조건에 따라 이벤트를 표시할지 여부를 반환합니다.
func shouldShowSSEEvent(eventType string, data json.RawMessage, agentFilter, typeFilter string) bool {
	// 타입 필터
	if typeFilter != "" && eventType != typeFilter {
		return false
	}

	// 에이전트 필터 (agent_typing 이벤트에만 적용)
	if agentFilter != "" && eventType == "agent_typing" {
		var payload map[string]interface{}
		if err := json.Unmarshal(data, &payload); err != nil {
			return false
		}
		agentID, _ := payload["agent_id"].(string)
		if agentID != agentFilter {
			return false
		}
	}

	return true
}

// formatSSEEvent는 SSE 이벤트를 사람이 읽기 쉬운 형식으로 변환합니다.
// @MX:NOTE: 이벤트 타입별로 다른 포맷 적용
func formatSSEEvent(eventType string, data json.RawMessage, _, _ string) string {
	switch eventType {
	case "metric_update":
		return fmt.Sprintf("[METRIC] %s", string(data))
	case "agent_typing":
		var payload map[string]interface{}
		if err := json.Unmarshal(data, &payload); err != nil {
			return fmt.Sprintf("[AGENT] %s", string(data))
		}
		agentID, _ := payload["agent_id"].(string)
		textDelta, _ := payload["text_delta"].(string)
		return fmt.Sprintf("[AGENT] %s: %s", agentID, textDelta)
	case "agent_status_change":
		var payload map[string]interface{}
		if err := json.Unmarshal(data, &payload); err != nil {
			return fmt.Sprintf("[STATUS] %s", string(data))
		}
		agentID, _ := payload["agent_id"].(string)
		status, _ := payload["status"].(string)
		return fmt.Sprintf("[STATUS] %s: %s", agentID, status)
	default:
		return fmt.Sprintf("[%s] %s", eventType, string(data))
	}
}

// runMetrics는 워크스페이스 메트릭을 조회하여 출력합니다.
func runMetrics(client *apiclient.Client, out io.Writer, jsonOutput bool) error {
	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	workspaceID := client.WorkspaceID()
	path := "/api/v1/workspaces/" + workspaceID + "/dashboard/metrics"

	metrics, err := apiclient.Do[DashboardMetrics](client, ctx, "GET", path, nil)
	if err != nil {
		return fmt.Errorf("메트릭 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, metrics)
	}

	// 테이블 출력
	headers := []string{"ACTIVE_AGENTS", "TOTAL_MESSAGES", "TOTAL_TASKS", "COMPLETED_TASKS", "AVG_RESPONSE_MS"}
	row := []string{
		fmt.Sprintf("%d", metrics.ActiveAgents),
		fmt.Sprintf("%d", metrics.TotalMessages),
		fmt.Sprintf("%d", metrics.TotalTasks),
		fmt.Sprintf("%d", metrics.CompletedTasks),
		fmt.Sprintf("%.2f", metrics.AvgResponseTime),
	}
	apiclient.PrintTable(out, headers, [][]string{row})
	return nil
}

// runHealth는 조직 건강 상태를 조회하여 출력합니다.
func runHealth(client *apiclient.Client, out io.Writer, jsonOutput bool) error {
	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	workspaceID := client.WorkspaceID()
	path := "/api/v1/workspaces/" + workspaceID + "/org-health"

	health, err := apiclient.Do[OrgHealth](client, ctx, "GET", path, nil)
	if err != nil {
		return fmt.Errorf("건강 상태 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, health)
	}

	// 테이블 출력
	fields := []apiclient.KeyValue{
		{Key: "Status", Value: health.Status},
		{Key: "Score", Value: fmt.Sprintf("%.1f", health.Score)},
	}
	// 컴포넌트 상태 추가
	for name, status := range health.Components {
		fields = append(fields, apiclient.KeyValue{
			Key:   "Component/" + name,
			Value: status,
		})
	}
	apiclient.PrintDetail(out, fields)
	return nil
}
