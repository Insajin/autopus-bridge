// agent.go는 에이전트 관련 CLI 명령어를 구현합니다.
// AC13-AC17: agent list/show/activity/performance 서브커맨드
package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/spf13/cobra"
)

// DashboardAgent는 대시보드 에이전트 목록 항목을 나타냅니다.
// GET /api/v1/workspaces/:id/dashboard/agents 응답에서 사용합니다.
type DashboardAgent struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	Model       string `json:"model,omitempty"`
	Provider    string `json:"provider,omitempty"`
	Description string `json:"description,omitempty"`
}

// AgentDetail은 에이전트 상세 정보를 나타냅니다.
// GET /api/v1/agents/:agentId 응답에서 사용합니다.
type AgentDetail struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Status      string `json:"status,omitempty"`
	Model       string `json:"model,omitempty"`
	Provider    string `json:"provider,omitempty"`
	Description string `json:"description,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

var (
	agentJSONOutput bool
)

// agentCmd는 agent 서브커맨드의 루트입니다.
var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "에이전트 관련 명령어",
	Long:  `에이전트 목록 조회, 상세 조회, 활동 내역, 퍼포먼스 조회 기능을 제공합니다.`,
}

// agentListCmd는 에이전트 목록을 조회합니다.
var agentListCmd = &cobra.Command{
	Use:   "list",
	Short: "에이전트 목록 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runAgentList(client, os.Stdout, json)
	},
}

// agentShowCmd는 에이전트 상세 정보를 조회합니다.
var agentShowCmd = &cobra.Command{
	Use:   "show <agent-id|name>",
	Short: "에이전트 상세 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runAgentShow(client, os.Stdout, args[0], json)
	},
}

// agentActivityCmd는 에이전트 활동 내역을 조회합니다.
var agentActivityCmd = &cobra.Command{
	Use:   "activity <agent-id|name>",
	Short: "에이전트 활동 내역 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runAgentActivity(client, os.Stdout, args[0], json)
	},
}

// agentPerformanceCmd는 에이전트 퍼포먼스 지표를 조회합니다.
var agentPerformanceCmd = &cobra.Command{
	Use:   "performance <agent-id|name>",
	Short: "에이전트 퍼포먼스 지표 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runAgentPerformance(client, os.Stdout, args[0], json)
	},
}

func init() {
	rootCmd.AddCommand(agentCmd)
	agentCmd.AddCommand(agentListCmd)
	agentCmd.AddCommand(agentShowCmd)
	agentCmd.AddCommand(agentActivityCmd)
	agentCmd.AddCommand(agentPerformanceCmd)

	// --json 플래그를 모든 서브커맨드에 추가
	for _, sub := range []*cobra.Command{agentListCmd, agentShowCmd, agentActivityCmd, agentPerformanceCmd} {
		sub.Flags().BoolVar(&agentJSONOutput, "json", false, "JSON 형식으로 출력")
	}
}

// runAgentList는 대시보드 에이전트 목록을 출력합니다.
func runAgentList(client *apiclient.Client, out io.Writer, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()
	path := "/api/v1/workspaces/" + workspaceID + "/dashboard/agents"

	agents, err := apiclient.DoList[DashboardAgent](client, context.Background(), "GET", path, nil)
	if err != nil {
		return fmt.Errorf("에이전트 목록 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, agents)
	}

	headers := []string{"ID", "NAME", "STATUS", "MODEL", "PROVIDER"}
	rows := make([][]string, len(agents))
	for i, ag := range agents {
		rows[i] = []string{ag.ID, ag.Name, ag.Status, ag.Model, ag.Provider}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runAgentShow는 에이전트 상세 정보를 출력합니다.
// agentRef는 에이전트 ID 또는 이름(부분 일치)을 허용합니다.
func runAgentShow(client *apiclient.Client, out io.Writer, agentRef string, jsonOutput bool) error {
	agentID, err := resolveAgentRef(client, agentRef)
	if err != nil {
		return err
	}

	agent, err := apiclient.Do[AgentDetail](client, context.Background(), "GET", "/api/v1/agents/"+agentID, nil)
	if err != nil {
		return fmt.Errorf("에이전트 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, agent)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: agent.ID},
		{Key: "Name", Value: agent.Name},
		{Key: "Status", Value: agent.Status},
		{Key: "Model", Value: agent.Model},
		{Key: "Provider", Value: agent.Provider},
		{Key: "Description", Value: agent.Description},
		{Key: "WorkspaceID", Value: agent.WorkspaceID},
		{Key: "CreatedAt", Value: agent.CreatedAt},
	})
	return nil
}

// runAgentActivity는 에이전트 활동 내역을 출력합니다.
func runAgentActivity(client *apiclient.Client, out io.Writer, agentRef string, jsonOutput bool) error {
	agentID, err := resolveAgentRef(client, agentRef)
	if err != nil {
		return err
	}

	workspaceID := client.WorkspaceID()
	path := "/api/v1/workspaces/" + workspaceID + "/dashboard/agents/" + agentID + "/activity"

	// 활동 내역은 임의의 JSON 구조이므로 interface{} 슬라이스로 수신
	activities, err := apiclient.DoList[map[string]interface{}](client, context.Background(), "GET", path, nil)
	if err != nil {
		return fmt.Errorf("에이전트 활동 내역 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, activities)
	}

	fmt.Fprintf(out, "에이전트 %s 활동 내역 (%d건)\n", agentID, len(activities))
	for _, act := range activities {
		if id, ok := act["id"].(string); ok {
			fmt.Fprintf(out, "  - %s\n", id)
		}
	}
	return nil
}

// runAgentPerformance는 에이전트 퍼포먼스 지표를 출력합니다.
func runAgentPerformance(client *apiclient.Client, out io.Writer, agentRef string, jsonOutput bool) error {
	agentID, err := resolveAgentRef(client, agentRef)
	if err != nil {
		return err
	}

	workspaceID := client.WorkspaceID()
	path := "/api/v1/workspaces/" + workspaceID + "/dashboard/agents/" + agentID + "/performance"

	perf, err := apiclient.Do[map[string]interface{}](client, context.Background(), "GET", path, nil)
	if err != nil {
		return fmt.Errorf("에이전트 퍼포먼스 조회 실패: %w", err)
	}

	return apiclient.PrintJSON(out, perf)
}

// resolveAgentRef는 에이전트 ID 또는 이름을 에이전트 ID로 변환합니다.
// UUID 형식이면 그대로 반환하고, 그 외에는 대시보드에서 이름으로 검색합니다.
func resolveAgentRef(client *apiclient.Client, agentRef string) (string, error) {
	// UUID 형식 (36자, '-' 포함)이면 ID로 간주
	if len(agentRef) == 36 && strings.Count(agentRef, "-") == 4 {
		return agentRef, nil
	}

	// 이름으로 검색 (대시보드 에이전트 목록 사용)
	workspaceID := client.WorkspaceID()
	agents, err := apiclient.DoList[DashboardAgent](client, context.Background(), "GET",
		"/api/v1/workspaces/"+workspaceID+"/dashboard/agents", nil)
	if err != nil {
		return "", fmt.Errorf("에이전트 목록 조회 실패: %w", err)
	}

	found, err := findDashboardAgentByName(agents, agentRef)
	if err != nil {
		return "", err
	}
	return found.ID, nil
}

// findDashboardAgentByName은 에이전트 이름으로 DashboardAgent를 찾습니다.
// execute.go의 findAgentByName 패턴을 재사용합니다.
func findDashboardAgentByName(agents []DashboardAgent, name string) (*DashboardAgent, error) {
	var exactMatches []DashboardAgent
	var partialMatches []DashboardAgent
	lowerName := strings.ToLower(strings.TrimSpace(name))

	for _, ag := range agents {
		agName := strings.ToLower(ag.Name)
		switch {
		case agName == lowerName:
			exactMatches = append(exactMatches, ag)
		case strings.Contains(agName, lowerName):
			partialMatches = append(partialMatches, ag)
		}
	}

	switch {
	case len(exactMatches) == 1:
		return &exactMatches[0], nil
	case len(exactMatches) > 1:
		return nil, fmt.Errorf("이름이 정확히 일치하는 에이전트가 여러 개입니다: %s", name)
	case len(partialMatches) == 1:
		return &partialMatches[0], nil
	case len(partialMatches) > 1:
		return nil, fmt.Errorf("이름이 부분 일치하는 에이전트가 여러 개입니다: %s", name)
	default:
		return nil, fmt.Errorf("에이전트를 찾을 수 없습니다: %s", name)
	}
}
