// planning.go는 전략적 계획 관련 CLI 명령어를 구현합니다.
// planning goals/goal-create/initiatives/initiative-create/alignment 서브커맨드
package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/spf13/cobra"
)

// StrategicGoal은 전략적 목표를 나타냅니다.
type StrategicGoal struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	Description     string `json:"description,omitempty"`
	OwnerAgentID    string `json:"owner_agent_id,omitempty"`
	Status          string `json:"status,omitempty"`
	SuccessCriteria string `json:"success_criteria,omitempty"`
	CreatedAt       string `json:"created_at,omitempty"`
}

// Initiative는 이니셔티브를 나타냅니다.
type Initiative struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	GoalID      string `json:"goal_id,omitempty"`
	Status      string `json:"status,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

// AlignmentStatus는 목표 정렬 상태를 나타냅니다.
type AlignmentStatus struct {
	TotalGoals     int     `json:"total_goals"`
	AlignedGoals   int     `json:"aligned_goals"`
	AlignmentScore float64 `json:"alignment_score"`
}

var (
	planningJSONOutput     bool
	planningGoalTitle      string
	planningGoalDesc       string
	planningGoalOwner      string
	planningGoalCriteria   string
	planningInitTitle      string
	planningInitDesc       string
	planningInitGoalID     string
)

// planningCmd는 planning 서브커맨드의 루트입니다.
var planningCmd = &cobra.Command{
	Use:   "planning",
	Short: "전략적 계획 관련 명령어",
	Long:  `목표 조회, 목표 생성, 이니셔티브 조회, 이니셔티브 생성, 정렬 상태 조회 기능을 제공합니다.`,
}

var planningGoalsCmd = &cobra.Command{
	Use:   "goals",
	Short: "전략적 목표 목록 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(jsonOut)
		return runPlanningGoals(client, os.Stdout, jsonOut)
	},
}

var planningGoalCreateCmd = &cobra.Command{
	Use:   "goal-create",
	Short: "전략적 목표 생성",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		title, _ := cmd.Flags().GetString("title")
		desc, _ := cmd.Flags().GetString("description")
		owner, _ := cmd.Flags().GetString("owner-agent")
		criteria, _ := cmd.Flags().GetString("success-criteria")
		client.SetJSONOutput(jsonOut)
		return runPlanningGoalCreate(client, os.Stdout, title, desc, owner, criteria, jsonOut)
	},
}

var planningInitiativesCmd = &cobra.Command{
	Use:   "initiatives",
	Short: "이니셔티브 목록 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(jsonOut)
		return runPlanningInitiatives(client, os.Stdout, jsonOut)
	},
}

var planningInitiativeCreateCmd = &cobra.Command{
	Use:   "initiative-create",
	Short: "이니셔티브 생성",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		title, _ := cmd.Flags().GetString("title")
		desc, _ := cmd.Flags().GetString("description")
		goalID, _ := cmd.Flags().GetString("goal-id")
		client.SetJSONOutput(jsonOut)
		return runPlanningInitiativeCreate(client, os.Stdout, title, desc, goalID, jsonOut)
	},
}

var planningAlignmentCmd = &cobra.Command{
	Use:   "alignment",
	Short: "목표 정렬 상태 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(jsonOut)
		return runPlanningAlignment(client, os.Stdout, jsonOut)
	},
}

func init() {
	rootCmd.AddCommand(planningCmd)
	planningCmd.AddCommand(planningGoalsCmd)
	planningCmd.AddCommand(planningGoalCreateCmd)
	planningCmd.AddCommand(planningInitiativesCmd)
	planningCmd.AddCommand(planningInitiativeCreateCmd)
	planningCmd.AddCommand(planningAlignmentCmd)

	// --json 플래그를 서브커맨드에 추가
	for _, sub := range []*cobra.Command{
		planningGoalsCmd, planningGoalCreateCmd,
		planningInitiativesCmd, planningInitiativeCreateCmd, planningAlignmentCmd,
	} {
		sub.Flags().BoolVar(&planningJSONOutput, "json", false, "JSON 형식으로 출력")
	}

	// goal-create 플래그
	planningGoalCreateCmd.Flags().StringVar(&planningGoalTitle, "title", "", "목표 제목")
	planningGoalCreateCmd.Flags().StringVar(&planningGoalDesc, "description", "", "목표 설명")
	planningGoalCreateCmd.Flags().StringVar(&planningGoalOwner, "owner-agent", "", "소유 에이전트 ID")
	planningGoalCreateCmd.Flags().StringVar(&planningGoalCriteria, "success-criteria", "", "성공 기준")

	// initiative-create 플래그
	planningInitiativeCreateCmd.Flags().StringVar(&planningInitTitle, "title", "", "이니셔티브 제목")
	planningInitiativeCreateCmd.Flags().StringVar(&planningInitDesc, "description", "", "이니셔티브 설명")
	planningInitiativeCreateCmd.Flags().StringVar(&planningInitGoalID, "goal-id", "", "목표 ID")
}

// runPlanningGoals는 전략적 목표 목록을 조회하고 출력합니다.
func runPlanningGoals(client *apiclient.Client, out io.Writer, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	goals, err := apiclient.DoList[StrategicGoal](client, ctx, "GET",
		"/api/v1/workspaces/"+workspaceID+"/planning/goals", nil)
	if err != nil {
		return fmt.Errorf("전략적 목표 목록 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, goals)
	}

	headers := []string{"ID", "TITLE", "OWNER", "STATUS"}
	rows := make([][]string, len(goals))
	for i, g := range goals {
		shortID := g.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		rows[i] = []string{shortID, g.Title, g.OwnerAgentID, g.Status}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runPlanningGoalCreate는 새 전략적 목표를 생성합니다.
func runPlanningGoalCreate(client *apiclient.Client, out io.Writer, title, desc, owner, criteria string, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	reqBody := map[string]string{
		"workspace_id": workspaceID,
		"title":        title,
	}
	if desc != "" {
		reqBody["description"] = desc
	}
	if owner != "" {
		reqBody["owner_agent_id"] = owner
	}
	if criteria != "" {
		reqBody["success_criteria"] = criteria
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	goal, err := apiclient.Do[StrategicGoal](client, ctx, "POST", "/api/v1/goals", reqBody)
	if err != nil {
		return fmt.Errorf("전략적 목표 생성 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, goal)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: goal.ID},
		{Key: "Title", Value: goal.Title},
		{Key: "OwnerAgentID", Value: goal.OwnerAgentID},
		{Key: "Status", Value: goal.Status},
	})
	return nil
}

// runPlanningInitiatives는 이니셔티브 목록을 조회하고 출력합니다.
func runPlanningInitiatives(client *apiclient.Client, out io.Writer, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	initiatives, err := apiclient.DoList[Initiative](client, ctx, "GET",
		"/api/v1/workspaces/"+workspaceID+"/planning/initiatives", nil)
	if err != nil {
		return fmt.Errorf("이니셔티브 목록 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, initiatives)
	}

	headers := []string{"ID", "TITLE", "GOAL_ID", "STATUS"}
	rows := make([][]string, len(initiatives))
	for i, ini := range initiatives {
		shortID := ini.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		rows[i] = []string{shortID, ini.Title, ini.GoalID, ini.Status}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runPlanningInitiativeCreate는 새 이니셔티브를 생성합니다.
func runPlanningInitiativeCreate(client *apiclient.Client, out io.Writer, title, desc, goalID string, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	reqBody := map[string]string{
		"title": title,
	}
	if desc != "" {
		reqBody["description"] = desc
	}
	if goalID != "" {
		reqBody["goal_id"] = goalID
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	initiative, err := apiclient.Do[Initiative](client, ctx, "POST",
		"/api/v1/workspaces/"+workspaceID+"/planning/initiatives", reqBody)
	if err != nil {
		return fmt.Errorf("이니셔티브 생성 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, initiative)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: initiative.ID},
		{Key: "Title", Value: initiative.Title},
		{Key: "GoalID", Value: initiative.GoalID},
		{Key: "Status", Value: initiative.Status},
	})
	return nil
}

// runPlanningAlignment는 목표 정렬 상태를 조회하고 출력합니다.
func runPlanningAlignment(client *apiclient.Client, out io.Writer, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	alignment, err := apiclient.Do[AlignmentStatus](client, ctx, "GET",
		"/api/v1/workspaces/"+workspaceID+"/planning/alignment", nil)
	if err != nil {
		return fmt.Errorf("목표 정렬 상태 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, alignment)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "TotalGoals", Value: fmt.Sprintf("%d", alignment.TotalGoals)},
		{Key: "AlignedGoals", Value: fmt.Sprintf("%d", alignment.AlignedGoals)},
		{Key: "AlignmentScore", Value: fmt.Sprintf("%.2f", alignment.AlignmentScore)},
	})
	return nil
}
