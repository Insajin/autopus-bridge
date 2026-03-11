// autonomy.go는 자율성 단계 관련 CLI 명령어를 구현합니다.
// autonomy phase/phase-update/history/readiness/transition-history/trends/recommendation 서브커맨드
package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/spf13/cobra"
)

// AutonomyPhase는 자율성 단계 정보를 나타냅니다.
type AutonomyPhase struct {
	Phase       string `json:"phase"`
	Description string `json:"description,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

// AutonomyHistory는 자율성 단계 변경 이력을 나타냅니다.
type AutonomyHistory struct {
	ID        string `json:"id"`
	Phase     string `json:"phase"`
	ChangedAt string `json:"changed_at,omitempty"`
	ChangedBy string `json:"changed_by,omitempty"`
}

// AutonomyReadiness는 자율성 전환 준비 상태를 나타냅니다.
type AutonomyReadiness struct {
	CurrentPhase    string              `json:"current_phase"`
	TargetPhase     string              `json:"target_phase,omitempty"`
	NextPhase       string              `json:"next_phase,omitempty"`
	Score           float64             `json:"score,omitempty"`
	ReadyScore      float64             `json:"ready_score,omitempty"`
	Conditions      []AutonomyCondition `json:"conditions,omitempty"`
	ConsecutiveDays int                 `json:"consecutive_days,omitempty"`
	IsEligible      bool                `json:"is_eligible,omitempty"`
	Blockers        string              `json:"blockers,omitempty"`
}

// AutonomyTrend는 자율성 점수 추이를 나타냅니다.
type AutonomyTrend struct {
	Period         string  `json:"period,omitempty"`
	CurrentPhase   string  `json:"current_phase,omitempty"`
	TargetPhase    string  `json:"target_phase,omitempty"`
	SnapshotAt     string  `json:"snapshot_at,omitempty"`
	Score          float64 `json:"score,omitempty"`
	ReadinessScore float64 `json:"readiness_score,omitempty"`
}

// AutonomyRecommendation은 자율성 전환 추천 정보를 나타냅니다.
type AutonomyRecommendation struct {
	RecommendedPhase string  `json:"recommended_phase"`
	Type             string  `json:"type,omitempty"`
	CurrentPhase     string  `json:"current_phase,omitempty"`
	TargetPhase      string  `json:"target_phase,omitempty"`
	ReadinessScore   float64 `json:"readiness_score,omitempty"`
	Confidence       float64 `json:"confidence"`
	Rationale        string  `json:"rationale,omitempty"`
	Reason           string  `json:"reason,omitempty"`
	ConsecutiveDays  int     `json:"consecutive_days,omitempty"`
}

// AutonomyCondition은 준비도/권고 응답의 개별 조건을 나타냅니다.
type AutonomyCondition struct {
	Name      string  `json:"name"`
	Threshold float64 `json:"threshold"`
	Current   float64 `json:"current"`
	Weight    float64 `json:"weight"`
	Met       bool    `json:"met"`
}

type autonomyPhaseHistoryResponse struct {
	Logs []autonomyPhaseHistoryItem `json:"logs"`
}

type autonomyPhaseHistoryItem struct {
	ID            string  `json:"id"`
	PreviousPhase string  `json:"previous_phase,omitempty"`
	NewPhase      string  `json:"new_phase,omitempty"`
	ChangedBy     *string `json:"changed_by,omitempty"`
	CreatedAt     string  `json:"created_at,omitempty"`
}

type autonomyTransitionHistoryResponse struct {
	Days      int             `json:"days"`
	Snapshots []AutonomyTrend `json:"snapshots"`
}

type autonomyTransitionTrendsResponse struct {
	Days   int             `json:"days"`
	Trends []AutonomyTrend `json:"trends"`
}

type autonomyRecommendationEnvelope struct {
	Recommendation *AutonomyRecommendation `json:"recommendation"`
	Message        string                  `json:"message,omitempty"`
}

var (
	autonomyJSONOutput  bool
	autonomyPhaseTarget string
)

// autonomyCmd는 autonomy 서브커맨드의 루트입니다.
var autonomyCmd = &cobra.Command{
	Use:   "autonomy",
	Short: "자율성 단계 관련 명령어",
	Long:  `자율성 단계 조회, 단계 업데이트, 이력 조회, 준비 상태 조회, 전환 이력, 추이, 추천 기능을 제공합니다.`,
}

var autonomyPhaseCmd = &cobra.Command{
	Use:   "phase",
	Short: "현재 자율성 단계 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(jsonOut)
		return runAutonomyPhase(client, os.Stdout, jsonOut)
	},
}

var autonomyPhaseUpdateCmd = &cobra.Command{
	Use:   "phase-update",
	Short: "자율성 단계 업데이트",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		phase, _ := cmd.Flags().GetString("phase")
		client.SetJSONOutput(jsonOut)
		return runAutonomyPhaseUpdate(client, os.Stdout, phase, jsonOut)
	},
}

var autonomyHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: "자율성 단계 변경 이력 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(jsonOut)
		return runAutonomyHistory(client, os.Stdout, jsonOut)
	},
}

var autonomyReadinessCmd = &cobra.Command{
	Use:   "readiness",
	Short: "자율성 전환 준비 상태 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(jsonOut)
		return runAutonomyReadiness(client, os.Stdout, jsonOut)
	},
}

var autonomyTransitionHistoryCmd = &cobra.Command{
	Use:   "transition-history",
	Short: "자율성 전환 이력 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(jsonOut)
		return runAutonomyTransitionHistory(client, os.Stdout, jsonOut)
	},
}

var autonomyTrendsCmd = &cobra.Command{
	Use:   "trends",
	Short: "자율성 점수 추이 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(jsonOut)
		return runAutonomyTrends(client, os.Stdout, jsonOut)
	},
}

var autonomyRecommendationCmd = &cobra.Command{
	Use:   "recommendation",
	Short: "자율성 전환 추천 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(jsonOut)
		return runAutonomyRecommendation(client, os.Stdout, jsonOut)
	},
}

func init() {
	rootCmd.AddCommand(autonomyCmd)
	autonomyCmd.AddCommand(autonomyPhaseCmd)
	autonomyCmd.AddCommand(autonomyPhaseUpdateCmd)
	autonomyCmd.AddCommand(autonomyHistoryCmd)
	autonomyCmd.AddCommand(autonomyReadinessCmd)
	autonomyCmd.AddCommand(autonomyTransitionHistoryCmd)
	autonomyCmd.AddCommand(autonomyTrendsCmd)
	autonomyCmd.AddCommand(autonomyRecommendationCmd)

	// --json 플래그를 서브커맨드에 추가
	for _, sub := range []*cobra.Command{
		autonomyPhaseCmd, autonomyPhaseUpdateCmd, autonomyHistoryCmd,
		autonomyReadinessCmd, autonomyTransitionHistoryCmd, autonomyTrendsCmd, autonomyRecommendationCmd,
	} {
		sub.Flags().BoolVar(&autonomyJSONOutput, "json", false, "JSON 형식으로 출력")
	}

	// phase-update 전용 플래그
	autonomyPhaseUpdateCmd.Flags().StringVar(&autonomyPhaseTarget, "phase", "", "변경할 자율성 단계")
}

// runAutonomyPhase는 현재 자율성 단계를 조회하고 출력합니다.
func runAutonomyPhase(client *apiclient.Client, out io.Writer, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	phase, err := apiclient.Do[AutonomyPhase](client, ctx, "GET",
		"/api/v1/workspaces/"+workspaceID+"/autonomy/phase", nil)
	if err != nil {
		return fmt.Errorf("자율성 단계 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, phase)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "Phase", Value: phase.Phase},
		{Key: "Description", Value: phase.Description},
		{Key: "UpdatedAt", Value: phase.UpdatedAt},
	})
	return nil
}

// runAutonomyPhaseUpdate는 자율성 단계를 업데이트합니다.
func runAutonomyPhaseUpdate(client *apiclient.Client, out io.Writer, phase string, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	reqBody := map[string]string{"phase": phase}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	updated, err := apiclient.Do[AutonomyPhase](client, ctx, "PATCH",
		"/api/v1/workspaces/"+workspaceID+"/autonomy/phase", reqBody)
	if err != nil {
		return fmt.Errorf("자율성 단계 업데이트 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, updated)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "Phase", Value: updated.Phase},
		{Key: "Description", Value: updated.Description},
		{Key: "UpdatedAt", Value: updated.UpdatedAt},
	})
	return nil
}

// runAutonomyHistory는 자율성 단계 변경 이력을 조회하고 출력합니다.
func runAutonomyHistory(client *apiclient.Client, out io.Writer, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	resp, err := apiclient.Do[autonomyPhaseHistoryResponse](client, ctx, "GET",
		"/api/v1/workspaces/"+workspaceID+"/autonomy/phase/history", nil)
	history := make([]AutonomyHistory, 0)
	if err == nil {
		for _, log := range resp.Logs {
			changedBy := ""
			if log.ChangedBy != nil {
				changedBy = *log.ChangedBy
			}
			history = append(history, AutonomyHistory{
				ID:        log.ID,
				Phase:     log.NewPhase,
				ChangedAt: log.CreatedAt,
				ChangedBy: changedBy,
			})
		}
	} else {
		history, err = apiclient.DoList[AutonomyHistory](client, ctx, "GET",
			"/api/v1/workspaces/"+workspaceID+"/autonomy/phase/history", nil)
		if err != nil {
			return fmt.Errorf("자율성 단계 이력 조회 실패: %w", err)
		}
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, history)
	}

	headers := []string{"ID", "PHASE", "CHANGED_AT", "CHANGED_BY"}
	rows := make([][]string, len(history))
	for i, h := range history {
		rows[i] = []string{h.ID, h.Phase, h.ChangedAt, h.ChangedBy}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runAutonomyReadiness는 자율성 전환 준비 상태를 조회하고 출력합니다.
func runAutonomyReadiness(client *apiclient.Client, out io.Writer, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	readiness, err := apiclient.Do[AutonomyReadiness](client, ctx, "GET",
		"/api/v1/workspaces/"+workspaceID+"/autonomy/transition/readiness", nil)
	if err != nil {
		return fmt.Errorf("자율성 전환 준비 상태 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, readiness)
	}

	score := readiness.Score
	if score == 0 && readiness.ReadyScore != 0 {
		score = readiness.ReadyScore
	}
	nextPhase := readiness.TargetPhase
	if readiness.NextPhase != "" {
		nextPhase = readiness.NextPhase
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "CurrentPhase", Value: readiness.CurrentPhase},
		{Key: "NextPhase", Value: nextPhase},
		{Key: "ReadyScore", Value: fmt.Sprintf("%.2f", score)},
		{Key: "ConsecutiveDays", Value: fmt.Sprintf("%d", readiness.ConsecutiveDays)},
		{Key: "IsEligible", Value: fmt.Sprintf("%v", readiness.IsEligible)},
		{Key: "Blockers", Value: readiness.Blockers},
	})
	return nil
}

// runAutonomyTransitionHistory는 자율성 전환 이력을 조회하고 출력합니다.
func runAutonomyTransitionHistory(client *apiclient.Client, out io.Writer, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	resp, err := apiclient.Do[autonomyTransitionHistoryResponse](client, ctx, "GET",
		"/api/v1/workspaces/"+workspaceID+"/autonomy/transition/history", nil)
	snapshots := []AutonomyTrend{}
	if err == nil {
		snapshots = resp.Snapshots
	} else {
		legacy, legacyErr := apiclient.DoList[AutonomyHistory](client, ctx, "GET",
			"/api/v1/workspaces/"+workspaceID+"/autonomy/transition/history", nil)
		if legacyErr != nil {
			return fmt.Errorf("자율성 전환 이력 조회 실패: %w", err)
		}
		for _, item := range legacy {
			snapshots = append(snapshots, AutonomyTrend{
				CurrentPhase: item.Phase,
				SnapshotAt:   item.ChangedAt,
			})
		}
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, snapshots)
	}

	headers := []string{"SNAPSHOT_AT", "CURRENT_PHASE", "TARGET_PHASE", "READINESS"}
	rows := make([][]string, len(snapshots))
	for i, h := range snapshots {
		rows[i] = []string{h.SnapshotAt, h.CurrentPhase, h.TargetPhase, fmt.Sprintf("%.2f", h.ReadinessScore)}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runAutonomyTrends는 자율성 점수 추이를 조회하고 출력합니다.
func runAutonomyTrends(client *apiclient.Client, out io.Writer, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	resp, err := apiclient.Do[autonomyTransitionTrendsResponse](client, ctx, "GET",
		"/api/v1/workspaces/"+workspaceID+"/autonomy/transition/trends", nil)
	trends := []AutonomyTrend{}
	if err == nil {
		trends = resp.Trends
	} else {
		trends, err = apiclient.DoList[AutonomyTrend](client, ctx, "GET",
			"/api/v1/workspaces/"+workspaceID+"/autonomy/transition/trends", nil)
		if err != nil {
			return fmt.Errorf("자율성 추이 조회 실패: %w", err)
		}
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, trends)
	}

	headers := []string{"SNAPSHOT_AT", "CURRENT_PHASE", "TARGET_PHASE", "READINESS"}
	rows := make([][]string, len(trends))
	for i, t := range trends {
		snapshotAt := t.SnapshotAt
		if snapshotAt == "" {
			snapshotAt = t.Period
		}
		score := t.ReadinessScore
		if score == 0 {
			score = t.Score
		}
		rows[i] = []string{snapshotAt, t.CurrentPhase, t.TargetPhase, fmt.Sprintf("%.2f", score)}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runAutonomyRecommendation는 자율성 전환 추천을 조회하고 출력합니다.
func runAutonomyRecommendation(client *apiclient.Client, out io.Writer, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	resp, err := apiclient.Do[autonomyRecommendationEnvelope](client, ctx, "GET",
		"/api/v1/workspaces/"+workspaceID+"/autonomy/transition/recommendation", nil)
	var rec *AutonomyRecommendation
	message := ""
	if err == nil {
		rec = resp.Recommendation
		message = resp.Message
	}
	if err != nil || (rec == nil && message == "") {
		legacy, legacyErr := apiclient.Do[AutonomyRecommendation](client, ctx, "GET",
			"/api/v1/workspaces/"+workspaceID+"/autonomy/transition/recommendation", nil)
		if legacyErr != nil {
			if err != nil {
				return fmt.Errorf("자율성 전환 추천 조회 실패: %w", err)
			}
			return fmt.Errorf("자율성 전환 추천 조회 실패: %w", legacyErr)
		}
		rec = legacy
	}

	if rec == nil {
		if jsonOutput {
			return apiclient.PrintJSON(out, map[string]string{"message": message})
		}
		apiclient.PrintDetail(out, []apiclient.KeyValue{
			{Key: "Message", Value: message},
		})
		return nil
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, rec)
	}

	recommendedPhase := rec.RecommendedPhase
	if recommendedPhase == "" {
		recommendedPhase = rec.TargetPhase
	}
	rationale := rec.Rationale
	if rationale == "" {
		rationale = rec.Reason
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "RecommendedPhase", Value: recommendedPhase},
		{Key: "CurrentPhase", Value: rec.CurrentPhase},
		{Key: "Type", Value: rec.Type},
		{Key: "Confidence", Value: fmt.Sprintf("%.2f", rec.Confidence)},
		{Key: "ReadinessScore", Value: fmt.Sprintf("%.2f", rec.ReadinessScore)},
		{Key: "Rationale", Value: rationale},
	})
	return nil
}
