// execution.go는 실행(Execution) 관련 CLI 명령어를 구현합니다.
// AC26-AC30: execution list/show/watch/stats 서브커맨드
package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/spf13/cobra"
)

// ExecutionSummary는 실행 목록 항목을 나타냅니다.
type ExecutionSummary struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	AgentID   string `json:"agent_id,omitempty"`
	AgentName string `json:"agent_name,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// ExecutionDetail은 실행 상세 정보를 나타냅니다.
type ExecutionDetail struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	AgentID   string `json:"agent_id,omitempty"`
	AgentName string `json:"agent_name,omitempty"`
	Prompt    string `json:"prompt,omitempty"`
	Result    string `json:"result,omitempty"`
	Error     string `json:"error,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// executionListOpts는 execution list 명령의 옵션을 담습니다.
type executionListOpts struct {
	AgentName    string
	StatusFilter string
	Limit        int
}

var (
	executionJSONOutput   bool
	executionAgentFilter  string
	executionStatusFilter string
	executionLimit        int
)

// executionCmd는 execution 서브커맨드의 루트입니다.
var executionCmdRoot = &cobra.Command{
	Use:   "execution",
	Short: "실행(Execution) 관련 명령어",
	Long:  `실행 목록 조회, 상세 조회, SSE 스트리밍 감시, 통계 조회 기능을 제공합니다.`,
}

// executionListCmd는 실행 목록을 조회합니다.
var executionListCmd = &cobra.Command{
	Use:   "list",
	Short: "실행 목록 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)

		opts := executionListOpts{
			AgentName:    executionAgentFilter,
			StatusFilter: executionStatusFilter,
			Limit:        executionLimit,
		}
		return runExecutionList(client, os.Stdout, opts, json)
	},
}

// executionShowCmd는 실행 상세 정보를 조회합니다.
var executionShowCmd = &cobra.Command{
	Use:   "show <execution-id>",
	Short: "실행 상세 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runExecutionShow(client, os.Stdout, args[0], json)
	},
}

// executionWatchCmd는 실행 SSE 스트림을 감시합니다.
var executionWatchCmd = &cobra.Command{
	Use:   "watch <execution-id>",
	Short: "실행 SSE 스트리밍 감시",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runExecutionWatch(cmd.Context(), client, os.Stdout, args[0])
	},
}

// executionStatsCmd는 실행 통계를 조회합니다.
var executionStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "실행 통계 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runExecutionStats(client, os.Stdout, executionAgentFilter, json)
	},
}

func init() {
	rootCmd.AddCommand(executionCmdRoot)
	executionCmdRoot.AddCommand(executionListCmd)
	executionCmdRoot.AddCommand(executionShowCmd)
	executionCmdRoot.AddCommand(executionWatchCmd)
	executionCmdRoot.AddCommand(executionStatsCmd)

	// --json 플래그
	for _, sub := range []*cobra.Command{executionListCmd, executionShowCmd, executionStatsCmd} {
		sub.Flags().BoolVar(&executionJSONOutput, "json", false, "JSON 형식으로 출력")
	}

	// list 전용 플래그
	executionListCmd.Flags().StringVar(&executionAgentFilter, "agent", "", "에이전트 이름으로 필터링")
	executionListCmd.Flags().StringVar(&executionStatusFilter, "status", "", "상태로 필터링 (running|completed|failed)")
	executionListCmd.Flags().IntVar(&executionLimit, "limit", 20, "최대 조회 건수")

	// stats 전용 플래그
	executionStatsCmd.Flags().StringVar(&executionAgentFilter, "agent", "", "에이전트 이름으로 필터링")
}

// runExecutionList는 실행 목록을 출력합니다.
// --agent가 지정되면 /agents/:agentId/executions를 호출하고,
// 그렇지 않으면 /workspaces/:id/executions/audit를 호출합니다.
func runExecutionList(client *apiclient.Client, out io.Writer, opts executionListOpts, jsonOutput bool) error {
	var path string

	if opts.AgentName != "" {
		// 에이전트 이름으로 ID 조회
		agentID, err := resolveAgentRef(client, opts.AgentName)
		if err != nil {
			return err
		}
		path = "/api/v1/agents/" + agentID + "/executions"
	} else {
		workspaceID := client.WorkspaceID()
		path = "/api/v1/workspaces/" + workspaceID + "/executions/audit"
	}

	// limit 쿼리 파라미터 추가
	if opts.Limit > 0 {
		path += "?limit=" + strconv.Itoa(opts.Limit)
	}

	executions, err := apiclient.DoList[ExecutionSummary](client, context.Background(), "GET", path, nil)
	if err != nil {
		return fmt.Errorf("실행 목록 조회 실패: %w", err)
	}

	// 상태 필터 적용 (클라이언트 사이드)
	if opts.StatusFilter != "" {
		filtered := executions[:0]
		for _, e := range executions {
			if e.Status == opts.StatusFilter {
				filtered = append(filtered, e)
			}
		}
		executions = filtered
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, executions)
	}

	headers := []string{"ID", "STATUS", "AGENT", "CREATED_AT"}
	rows := make([][]string, len(executions))
	for i, e := range executions {
		agentLabel := e.AgentID
		if e.AgentName != "" {
			agentLabel = e.AgentName
		}
		rows[i] = []string{e.ID, e.Status, agentLabel, e.CreatedAt}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runExecutionShow는 실행 상세 정보를 출력합니다.
func runExecutionShow(client *apiclient.Client, out io.Writer, executionID string, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()
	path := "/api/v1/workspaces/" + workspaceID + "/executions/" + executionID

	detail, err := apiclient.Do[ExecutionDetail](client, context.Background(), "GET", path, nil)
	if err != nil {
		return fmt.Errorf("실행 상세 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, detail)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: detail.ID},
		{Key: "Status", Value: detail.Status},
		{Key: "AgentID", Value: detail.AgentID},
		{Key: "AgentName", Value: detail.AgentName},
		{Key: "Prompt", Value: detail.Prompt},
		{Key: "Error", Value: detail.Error},
		{Key: "CreatedAt", Value: detail.CreatedAt},
		{Key: "UpdatedAt", Value: detail.UpdatedAt},
	})
	return nil
}

// runExecutionWatch는 실행 SSE 스트림을 감시합니다.
// execute.go의 streamExecution 패턴을 참조합니다.
func runExecutionWatch(ctx context.Context, client *apiclient.Client, out io.Writer, executionID string) error {
	workspaceID := client.WorkspaceID()
	if workspaceID == "" {
		return fmt.Errorf("워크스페이스가 선택되지 않았습니다")
	}

	token, err := client.Token()
	if err != nil {
		return fmt.Errorf("인증 토큰 획득 실패: %w", err)
	}

	fmt.Fprintf(out, "실행 %s 감시 시작...\n", executionID)

	// SSESubscriber를 사용하여 스트림 구독
	// 엔드포인트: GET /workspaces/:id/executions/:executionId/stream
	// 현재는 Unified SSE 엔드포인트를 통해 agent_typing 이벤트만 필터링
	subscriber := apiclient.NewSSESubscriber(client.BaseURL(), workspaceID, token)
	eventCh, errCh := subscriber.Subscribe(ctx)

	// agent_typing 이벤트 필터링 (executionID와 연결된 채널의 이벤트)
	for {
		select {
		case event, ok := <-eventCh:
			if !ok {
				return nil
			}
			// 실행 관련 이벤트 출력
			fmt.Fprintf(out, "[%s] %s\n", event.Type, string(event.Data))

		case err, ok := <-errCh:
			if !ok {
				return nil
			}
			if err != nil {
				return fmt.Errorf("SSE 스트림 오류: %w", err)
			}
			return nil

		case <-ctx.Done():
			return nil
		}
	}
}

// runExecutionStats는 실행 통계를 출력합니다.
func runExecutionStats(client *apiclient.Client, out io.Writer, agentFilter string, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()
	path := "/api/v1/workspaces/" + workspaceID + "/executions/audit"

	executions, err := apiclient.DoList[ExecutionSummary](client, context.Background(), "GET", path, nil)
	if err != nil {
		return fmt.Errorf("실행 통계 조회 실패: %w", err)
	}

	// 에이전트 필터 적용
	if agentFilter != "" {
		agentID, resolveErr := resolveAgentRef(client, agentFilter)
		if resolveErr == nil {
			filtered := executions[:0]
			for _, e := range executions {
				if e.AgentID == agentID {
					filtered = append(filtered, e)
				}
			}
			executions = filtered
		}
	}

	// 통계 집계
	stats := map[string]int{
		"total":     len(executions),
		"completed": 0,
		"failed":    0,
		"running":   0,
		"other":     0,
	}
	for _, e := range executions {
		switch e.Status {
		case "completed":
			stats["completed"]++
		case "failed", "rejected", "cancelled":
			stats["failed"]++
		case "running", "pending":
			stats["running"]++
		default:
			stats["other"]++
		}
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, stats)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "Total", Value: strconv.Itoa(stats["total"])},
		{Key: "Completed", Value: strconv.Itoa(stats["completed"])},
		{Key: "Failed", Value: strconv.Itoa(stats["failed"])},
		{Key: "Running", Value: strconv.Itoa(stats["running"])},
	})
	return nil
}
