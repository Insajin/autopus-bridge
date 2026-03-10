// observability.go는 관측성 관련 CLI 명령어를 구현합니다.
// observability agents/agent/executions/cost/health/trends 서브커맨드
package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/spf13/cobra"
)

// ObservabilityAgent는 관측 대상 에이전트 정보를 나타냅니다.
type ObservabilityAgent struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Status      string  `json:"status,omitempty"`
	TaskCount   int     `json:"task_count,omitempty"`
	SuccessRate float64 `json:"success_rate,omitempty"`
}

// ObservabilityExecution은 에이전트 실행 정보를 나타냅니다.
type ObservabilityExecution struct {
	ID        string `json:"id"`
	AgentID   string `json:"agent_id,omitempty"`
	AgentName string `json:"agent_name,omitempty"`
	Status    string `json:"status,omitempty"`
	Duration  string `json:"duration,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

// ObservabilityCost는 비용 정보를 나타냅니다.
type ObservabilityCost struct {
	TotalCost float64 `json:"total_cost"`
	ByAgent   string  `json:"by_agent,omitempty"`
	Period    string  `json:"period,omitempty"`
}

// ObservabilityHealth는 시스템 건강 상태를 나타냅니다.
type ObservabilityHealth struct {
	Status    string  `json:"status"`
	Score     float64 `json:"score"`
	Issues    int     `json:"issues,omitempty"`
	CheckedAt string  `json:"checked_at,omitempty"`
}

// ObservabilityTrend는 트렌드 데이터를 나타냅니다.
type ObservabilityTrend struct {
	Period string  `json:"period"`
	Metric string  `json:"metric,omitempty"`
	Value  float64 `json:"value"`
}

var observabilityJSONOutput bool

// observabilityCmd는 observability 서브커맨드의 루트입니다.
var observabilityCmd = &cobra.Command{
	Use:   "observability",
	Short: "관측성 관련 명령어",
	Long:  `에이전트, 실행, 비용, 건강 상태, 트렌드 관측 기능을 제공합니다.`,
}

// observabilityAgentsCmd는 관측 에이전트 목록을 조회합니다.
var observabilityAgentsCmd = &cobra.Command{
	Use:   "agents",
	Short: "관측 에이전트 목록 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runObservabilityAgents(client, os.Stdout, json)
	},
}

// observabilityAgentCmd는 특정 관측 에이전트를 조회합니다.
var observabilityAgentCmd = &cobra.Command{
	Use:   "agent <agent-id>",
	Short: "관측 에이전트 상세 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runObservabilityAgent(client, os.Stdout, args[0], json)
	},
}

// observabilityExecutionsCmd는 실행 목록을 조회합니다.
var observabilityExecutionsCmd = &cobra.Command{
	Use:   "executions",
	Short: "실행 목록 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runObservabilityExecutions(client, os.Stdout, json)
	},
}

// observabilityCostCmd는 비용 정보를 조회합니다.
var observabilityCostCmd = &cobra.Command{
	Use:   "cost",
	Short: "비용 정보 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runObservabilityCost(client, os.Stdout, json)
	},
}

// observabilityHealthCmd는 시스템 건강 상태를 조회합니다.
var observabilityHealthCmd = &cobra.Command{
	Use:   "health",
	Short: "시스템 건강 상태 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runObservabilityHealth(client, os.Stdout, json)
	},
}

// observabilityTrendsCmd는 트렌드 데이터를 조회합니다.
var observabilityTrendsCmd = &cobra.Command{
	Use:   "trends",
	Short: "트렌드 데이터 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runObservabilityTrends(client, os.Stdout, json)
	},
}

func init() {
	rootCmd.AddCommand(observabilityCmd)
	observabilityCmd.AddCommand(observabilityAgentsCmd)
	observabilityCmd.AddCommand(observabilityAgentCmd)
	observabilityCmd.AddCommand(observabilityExecutionsCmd)
	observabilityCmd.AddCommand(observabilityCostCmd)
	observabilityCmd.AddCommand(observabilityHealthCmd)
	observabilityCmd.AddCommand(observabilityTrendsCmd)

	// --json 플래그를 서브커맨드에 추가
	for _, sub := range []*cobra.Command{
		observabilityAgentsCmd,
		observabilityAgentCmd,
		observabilityExecutionsCmd,
		observabilityCostCmd,
		observabilityHealthCmd,
		observabilityTrendsCmd,
	} {
		sub.Flags().BoolVar(&observabilityJSONOutput, "json", false, "JSON 형식으로 출력")
	}
}

// runObservabilityAgents는 관측 에이전트 목록을 조회하고 출력합니다.
func runObservabilityAgents(client *apiclient.Client, out io.Writer, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	agents, err := apiclient.DoList[ObservabilityAgent](client, ctx, "GET",
		"/api/v1/workspaces/"+workspaceID+"/observability/agents", nil)
	if err != nil {
		return fmt.Errorf("관측 에이전트 목록 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, agents)
	}

	headers := []string{"ID", "NAME", "STATUS", "TASK_COUNT", "SUCCESS_RATE"}
	rows := make([][]string, len(agents))
	for i, a := range agents {
		// ID는 첫 8자만 표시
		shortID := a.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		rows[i] = []string{
			shortID,
			a.Name,
			a.Status,
			fmt.Sprintf("%d", a.TaskCount),
			fmt.Sprintf("%.2f", a.SuccessRate),
		}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runObservabilityAgent는 특정 관측 에이전트를 조회하고 출력합니다.
func runObservabilityAgent(client *apiclient.Client, out io.Writer, agentID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(agentID); err != nil {
		return err
	}

	workspaceID := client.WorkspaceID()

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	agent, err := apiclient.Do[ObservabilityAgent](client, ctx, "GET",
		"/api/v1/workspaces/"+workspaceID+"/observability/agents/"+agentID, nil)
	if err != nil {
		return fmt.Errorf("관측 에이전트 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, agent)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: agent.ID},
		{Key: "Name", Value: agent.Name},
		{Key: "Status", Value: agent.Status},
		{Key: "TaskCount", Value: fmt.Sprintf("%d", agent.TaskCount)},
		{Key: "SuccessRate", Value: fmt.Sprintf("%.2f", agent.SuccessRate)},
	})
	return nil
}

// runObservabilityExecutions는 실행 목록을 조회하고 출력합니다.
func runObservabilityExecutions(client *apiclient.Client, out io.Writer, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	execs, err := apiclient.DoList[ObservabilityExecution](client, ctx, "GET",
		"/api/v1/workspaces/"+workspaceID+"/observability/executions", nil)
	if err != nil {
		return fmt.Errorf("실행 목록 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, execs)
	}

	headers := []string{"ID", "AGENT_NAME", "STATUS", "DURATION", "CREATED_AT"}
	rows := make([][]string, len(execs))
	for i, e := range execs {
		// ID는 첫 8자만 표시
		shortID := e.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		rows[i] = []string{shortID, e.AgentName, e.Status, e.Duration, e.CreatedAt}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runObservabilityCost는 비용 정보를 조회하고 출력합니다.
func runObservabilityCost(client *apiclient.Client, out io.Writer, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	cost, err := apiclient.Do[ObservabilityCost](client, ctx, "GET",
		"/api/v1/workspaces/"+workspaceID+"/observability/cost", nil)
	if err != nil {
		return fmt.Errorf("비용 정보 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, cost)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "TotalCost", Value: fmt.Sprintf("%.2f", cost.TotalCost)},
		{Key: "ByAgent", Value: cost.ByAgent},
		{Key: "Period", Value: cost.Period},
	})
	return nil
}

// runObservabilityHealth는 시스템 건강 상태를 조회하고 출력합니다.
func runObservabilityHealth(client *apiclient.Client, out io.Writer, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	health, err := apiclient.Do[ObservabilityHealth](client, ctx, "GET",
		"/api/v1/workspaces/"+workspaceID+"/observability/health", nil)
	if err != nil {
		return fmt.Errorf("시스템 건강 상태 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, health)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "Status", Value: health.Status},
		{Key: "Score", Value: fmt.Sprintf("%.2f", health.Score)},
		{Key: "Issues", Value: fmt.Sprintf("%d", health.Issues)},
		{Key: "CheckedAt", Value: health.CheckedAt},
	})
	return nil
}

// runObservabilityTrends는 트렌드 데이터를 조회하고 출력합니다.
func runObservabilityTrends(client *apiclient.Client, out io.Writer, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	trends, err := apiclient.DoList[ObservabilityTrend](client, ctx, "GET",
		"/api/v1/workspaces/"+workspaceID+"/observability/trends", nil)
	if err != nil {
		return fmt.Errorf("트렌드 데이터 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, trends)
	}

	headers := []string{"PERIOD", "METRIC", "VALUE"}
	rows := make([][]string, len(trends))
	for i, tr := range trends {
		rows[i] = []string{tr.Period, tr.Metric, fmt.Sprintf("%.2f", tr.Value)}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}
