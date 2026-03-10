// decision.go는 decision 관련 CLI 명령어를 구현합니다.
// decision list/show/create/resolve/human-resolve/escalate/audit-log/consensus/consensus-start/vote/confidence 서브커맨드
package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/spf13/cobra"
)

// Decision은 의사결정 기본 정보를 나타냅니다.
type Decision struct {
	ID           string   `json:"id"`
	Topic        string   `json:"topic"`
	Context      string   `json:"context,omitempty"`
	Status       string   `json:"status,omitempty"`
	Level        string   `json:"level,omitempty"`
	InitiatedBy  string   `json:"initiated_by,omitempty"`
	Participants []string `json:"participants,omitempty"`
	Outcome      string   `json:"outcome,omitempty"`
	Rationale    string   `json:"rationale,omitempty"`
	CreatedAt    string   `json:"created_at,omitempty"`
}

// DecisionAuditLog는 의사결정 감사 로그를 나타냅니다.
type DecisionAuditLog struct {
	ID        string `json:"id"`
	Action    string `json:"action"`
	Actor     string `json:"actor,omitempty"`
	Details   string `json:"details,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

// ConsensusStatus는 합의 상태를 나타냅니다.
type ConsensusStatus struct {
	DecisionID string `json:"decision_id"`
	Status     string `json:"status"`
	Votes      int    `json:"votes,omitempty"`
	Required   int    `json:"required,omitempty"`
}

// ConfidenceScore는 신뢰도 점수를 나타냅니다.
type ConfidenceScore struct {
	DecisionID string  `json:"decision_id"`
	Score      float64 `json:"score"`
	Factors    string  `json:"factors,omitempty"`
}

var (
	decisionJSONOutput    bool
	decisionListStatus    string
	decisionListLevel     string
	decisionListLimit     int
	decisionListOffset    int
	decisionTopic         string
	decisionContext       string
	decisionInitiatedBy   string
	decisionParticipants  string
	decisionOutcome       string
	decisionRationale     string
	decisionResolvedBy    string
	decisionApproved      bool
	decisionFeedback      string
	decisionHumanResolver string
	decisionReason        string
	decisionEscalatedBy   string
)

// decisionCmd는 decision 서브커맨드의 루트입니다.
var decisionCmd = &cobra.Command{
	Use:   "decision",
	Short: "의사결정 관련 명령어",
	Long:  `의사결정 목록 조회, 상세 조회, 생성, 해결, 에스컬레이션, 합의, 신뢰도 기능을 제공합니다.`,
}

// decisionListCmd는 의사결정 목록을 조회합니다.
var decisionListCmd = &cobra.Command{
	Use:   "list",
	Short: "의사결정 목록 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		status, _ := cmd.Flags().GetString("status")
		level, _ := cmd.Flags().GetString("level")
		limit, _ := cmd.Flags().GetInt("limit")
		offset, _ := cmd.Flags().GetInt("offset")
		client.SetJSONOutput(jsonOut)
		return runDecisionList(client, os.Stdout, status, level, limit, offset, jsonOut)
	},
}

// decisionShowCmd는 의사결정 상세 정보를 조회합니다.
var decisionShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "의사결정 상세 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(jsonOut)
		return runDecisionShow(client, os.Stdout, args[0], jsonOut)
	},
}

// decisionCreateCmd는 새 의사결정을 생성합니다.
var decisionCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "의사결정 생성",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		topic, _ := cmd.Flags().GetString("topic")
		ctx, _ := cmd.Flags().GetString("context")
		initiatedBy, _ := cmd.Flags().GetString("initiated-by")
		participantsStr, _ := cmd.Flags().GetString("participants")
		var participants []string
		if participantsStr != "" {
			participants = strings.Split(participantsStr, ",")
		}
		client.SetJSONOutput(jsonOut)
		return runDecisionCreate(client, os.Stdout, topic, ctx, initiatedBy, participants, jsonOut)
	},
}

// decisionResolveCmd는 의사결정을 해결합니다.
var decisionResolveCmd = &cobra.Command{
	Use:   "resolve <id>",
	Short: "의사결정 해결",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		outcome, _ := cmd.Flags().GetString("outcome")
		rationale, _ := cmd.Flags().GetString("rationale")
		resolvedBy, _ := cmd.Flags().GetString("resolved-by")
		return runDecisionResolve(client, os.Stdout, args[0], outcome, rationale, resolvedBy)
	},
}

// decisionHumanResolveCmd는 사람이 의사결정을 승인/거부합니다.
var decisionHumanResolveCmd = &cobra.Command{
	Use:   "human-resolve <id>",
	Short: "사람이 의사결정 승인/거부",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		approved, _ := cmd.Flags().GetBool("approved")
		feedback, _ := cmd.Flags().GetString("feedback")
		resolvedBy, _ := cmd.Flags().GetString("resolved-by")
		return runDecisionHumanResolve(client, os.Stdout, args[0], approved, feedback, resolvedBy)
	},
}

// decisionEscalateCmd는 의사결정을 에스컬레이션합니다.
var decisionEscalateCmd = &cobra.Command{
	Use:   "escalate <id>",
	Short: "의사결정 에스컬레이션",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		reason, _ := cmd.Flags().GetString("reason")
		escalatedBy, _ := cmd.Flags().GetString("escalated-by")
		return runDecisionEscalate(client, os.Stdout, args[0], reason, escalatedBy)
	},
}

// decisionAuditLogCmd는 의사결정 감사 로그를 조회합니다.
var decisionAuditLogCmd = &cobra.Command{
	Use:   "audit-log <id>",
	Short: "의사결정 감사 로그 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(jsonOut)
		return runDecisionAuditLog(client, os.Stdout, args[0], jsonOut)
	},
}

// decisionConsensusCmd는 합의 상태를 조회합니다.
var decisionConsensusCmd = &cobra.Command{
	Use:   "consensus <id>",
	Short: "합의 상태 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(jsonOut)
		return runDecisionConsensus(client, os.Stdout, args[0], jsonOut)
	},
}

// decisionConsensusStartCmd는 합의 프로세스를 시작합니다.
var decisionConsensusStartCmd = &cobra.Command{
	Use:   "consensus-start <id>",
	Short: "합의 프로세스 시작",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(jsonOut)
		return runDecisionConsensusStart(client, os.Stdout, args[0], jsonOut)
	},
}

// decisionVoteCmd는 의사결정에 투표합니다.
var decisionVoteCmd = &cobra.Command{
	Use:   "vote <id>",
	Short: "의사결정 투표",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(jsonOut)
		return runDecisionVote(client, os.Stdout, args[0], jsonOut)
	},
}

// decisionConfidenceCmd는 신뢰도 점수를 조회합니다.
var decisionConfidenceCmd = &cobra.Command{
	Use:   "confidence <id>",
	Short: "신뢰도 점수 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(jsonOut)
		return runDecisionConfidence(client, os.Stdout, args[0], jsonOut)
	},
}

func init() {
	rootCmd.AddCommand(decisionCmd)
	decisionCmd.AddCommand(decisionListCmd)
	decisionCmd.AddCommand(decisionShowCmd)
	decisionCmd.AddCommand(decisionCreateCmd)
	decisionCmd.AddCommand(decisionResolveCmd)
	decisionCmd.AddCommand(decisionHumanResolveCmd)
	decisionCmd.AddCommand(decisionEscalateCmd)
	decisionCmd.AddCommand(decisionAuditLogCmd)
	decisionCmd.AddCommand(decisionConsensusCmd)
	decisionCmd.AddCommand(decisionConsensusStartCmd)
	decisionCmd.AddCommand(decisionVoteCmd)
	decisionCmd.AddCommand(decisionConfidenceCmd)

	// --json 플래그
	for _, sub := range []*cobra.Command{
		decisionListCmd, decisionShowCmd, decisionCreateCmd,
		decisionAuditLogCmd, decisionConsensusCmd, decisionConsensusStartCmd,
		decisionVoteCmd, decisionConfidenceCmd,
	} {
		sub.Flags().BoolVar(&decisionJSONOutput, "json", false, "JSON 형식으로 출력")
	}

	// list 전용 플래그
	decisionListCmd.Flags().StringVar(&decisionListStatus, "status", "", "상태 필터")
	decisionListCmd.Flags().StringVar(&decisionListLevel, "level", "", "레벨 필터")
	decisionListCmd.Flags().IntVar(&decisionListLimit, "limit", 0, "조회 개수 제한")
	decisionListCmd.Flags().IntVar(&decisionListOffset, "offset", 0, "조회 시작 오프셋")

	// create 전용 플래그
	decisionCreateCmd.Flags().StringVar(&decisionTopic, "topic", "", "의사결정 주제 (필수)")
	decisionCreateCmd.Flags().StringVar(&decisionContext, "context", "", "의사결정 컨텍스트")
	decisionCreateCmd.Flags().StringVar(&decisionInitiatedBy, "initiated-by", "", "시작한 에이전트/사용자")
	decisionCreateCmd.Flags().StringVar(&decisionParticipants, "participants", "", "참여자 목록 (쉼표 구분)")

	// resolve 전용 플래그
	decisionResolveCmd.Flags().StringVar(&decisionOutcome, "outcome", "", "결정 결과")
	decisionResolveCmd.Flags().StringVar(&decisionRationale, "rationale", "", "결정 근거")
	decisionResolveCmd.Flags().StringVar(&decisionResolvedBy, "resolved-by", "", "해결한 에이전트/사용자")

	// human-resolve 전용 플래그
	decisionHumanResolveCmd.Flags().BoolVar(&decisionApproved, "approved", false, "승인 여부")
	decisionHumanResolveCmd.Flags().StringVar(&decisionFeedback, "feedback", "", "피드백")
	decisionHumanResolveCmd.Flags().StringVar(&decisionHumanResolver, "resolved-by", "", "해결한 사람")

	// escalate 전용 플래그
	decisionEscalateCmd.Flags().StringVar(&decisionReason, "reason", "", "에스컬레이션 사유")
	decisionEscalateCmd.Flags().StringVar(&decisionEscalatedBy, "escalated-by", "", "에스컬레이션한 에이전트/사용자")
}

// runDecisionList는 의사결정 목록을 조회하고 출력합니다.
func runDecisionList(client *apiclient.Client, out io.Writer, status, level string, limit, offset int, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	// 쿼리 파라미터 구성
	path := "/api/v1/workspaces/" + workspaceID + "/decisions"
	params := []string{}
	if status != "" {
		params = append(params, "status="+status)
	}
	if level != "" {
		params = append(params, "level="+level)
	}
	if limit > 0 {
		params = append(params, fmt.Sprintf("limit=%d", limit))
	}
	if offset > 0 {
		params = append(params, fmt.Sprintf("offset=%d", offset))
	}
	if len(params) > 0 {
		path = path + "?" + strings.Join(params, "&")
	}

	decisions, err := apiclient.DoList[Decision](client, ctx, "GET", path, nil)
	if err != nil {
		return fmt.Errorf("의사결정 목록 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, decisions)
	}

	headers := []string{"ID", "TOPIC", "STATUS", "LEVEL", "INITIATED_BY"}
	rows := make([][]string, len(decisions))
	for i, d := range decisions {
		shortID := d.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		rows[i] = []string{shortID, d.Topic, d.Status, d.Level, d.InitiatedBy}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runDecisionShow는 의사결정 상세 정보를 출력합니다.
func runDecisionShow(client *apiclient.Client, out io.Writer, decisionID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(decisionID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	d, err := apiclient.Do[Decision](client, ctx, "GET", "/api/v1/decisions/"+decisionID, nil)
	if err != nil {
		return fmt.Errorf("의사결정 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, d)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: d.ID},
		{Key: "Topic", Value: d.Topic},
		{Key: "Context", Value: d.Context},
		{Key: "Status", Value: d.Status},
		{Key: "Level", Value: d.Level},
		{Key: "InitiatedBy", Value: d.InitiatedBy},
		{Key: "Outcome", Value: d.Outcome},
		{Key: "Rationale", Value: d.Rationale},
		{Key: "CreatedAt", Value: d.CreatedAt},
	})
	return nil
}

// runDecisionCreate는 새 의사결정을 생성합니다.
func runDecisionCreate(client *apiclient.Client, out io.Writer, topic, decCtx, initiatedBy string, participants []string, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()
	body := map[string]interface{}{
		"workspace_id":  workspaceID,
		"topic":         topic,
		"context":       decCtx,
		"initiated_by":  initiatedBy,
		"participants":  participants,
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	d, err := apiclient.Do[Decision](client, ctx, "POST", "/api/v1/decisions", body)
	if err != nil {
		return fmt.Errorf("의사결정 생성 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, d)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: d.ID},
		{Key: "Topic", Value: d.Topic},
		{Key: "Status", Value: d.Status},
	})
	return nil
}

// runDecisionResolve는 의사결정을 해결합니다.
func runDecisionResolve(client *apiclient.Client, out io.Writer, decisionID, outcome, rationale, resolvedBy string) error {
	if err := apiclient.ValidateID(decisionID); err != nil {
		return err
	}

	body := map[string]string{
		"outcome":     outcome,
		"rationale":   rationale,
		"resolved_by": resolvedBy,
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	d, err := apiclient.Do[Decision](client, ctx, "PATCH", "/api/v1/decisions/"+decisionID+"/resolve", body)
	if err != nil {
		return fmt.Errorf("의사결정 해결 실패: %w", err)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: d.ID},
		{Key: "Topic", Value: d.Topic},
		{Key: "Status", Value: d.Status},
		{Key: "Outcome", Value: d.Outcome},
	})
	return nil
}

// runDecisionHumanResolve는 사람이 의사결정을 승인/거부합니다.
func runDecisionHumanResolve(client *apiclient.Client, out io.Writer, decisionID string, approved bool, feedback, resolvedBy string) error {
	if err := apiclient.ValidateID(decisionID); err != nil {
		return err
	}

	body := map[string]interface{}{
		"approved":    approved,
		"feedback":    feedback,
		"resolved_by": resolvedBy,
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	d, err := apiclient.Do[Decision](client, ctx, "PATCH", "/api/v1/decisions/"+decisionID+"/human-resolve", body)
	if err != nil {
		return fmt.Errorf("의사결정 사람 해결 실패: %w", err)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: d.ID},
		{Key: "Topic", Value: d.Topic},
		{Key: "Status", Value: d.Status},
	})
	return nil
}

// runDecisionEscalate는 의사결정을 에스컬레이션합니다.
func runDecisionEscalate(client *apiclient.Client, out io.Writer, decisionID, reason, escalatedBy string) error {
	if err := apiclient.ValidateID(decisionID); err != nil {
		return err
	}

	body := map[string]string{
		"reason":       reason,
		"escalated_by": escalatedBy,
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	d, err := apiclient.Do[Decision](client, ctx, "POST", "/api/v1/decisions/"+decisionID+"/escalate", body)
	if err != nil {
		return fmt.Errorf("의사결정 에스컬레이션 실패: %w", err)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: d.ID},
		{Key: "Topic", Value: d.Topic},
		{Key: "Status", Value: d.Status},
	})
	return nil
}

// runDecisionAuditLog는 의사결정 감사 로그를 출력합니다.
func runDecisionAuditLog(client *apiclient.Client, out io.Writer, decisionID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(decisionID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	logs, err := apiclient.DoList[DecisionAuditLog](client, ctx, "GET", "/api/v1/decisions/"+decisionID+"/audit-log", nil)
	if err != nil {
		return fmt.Errorf("감사 로그 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, logs)
	}

	headers := []string{"ID", "ACTION", "ACTOR", "CREATED_AT"}
	rows := make([][]string, len(logs))
	for i, l := range logs {
		shortID := l.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		rows[i] = []string{shortID, l.Action, l.Actor, l.CreatedAt}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runDecisionConsensus는 합의 상태를 조회합니다.
func runDecisionConsensus(client *apiclient.Client, out io.Writer, decisionID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(decisionID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	status, err := apiclient.Do[ConsensusStatus](client, ctx, "GET", "/api/v1/decisions/"+decisionID+"/consensus", nil)
	if err != nil {
		return fmt.Errorf("합의 상태 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, status)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "DecisionID", Value: status.DecisionID},
		{Key: "Status", Value: status.Status},
		{Key: "Votes", Value: fmt.Sprintf("%d", status.Votes)},
		{Key: "Required", Value: fmt.Sprintf("%d", status.Required)},
	})
	return nil
}

// runDecisionConsensusStart는 합의 프로세스를 시작합니다.
func runDecisionConsensusStart(client *apiclient.Client, out io.Writer, decisionID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(decisionID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	status, err := apiclient.Do[ConsensusStatus](client, ctx, "POST", "/api/v1/decisions/"+decisionID+"/consensus", nil)
	if err != nil {
		return fmt.Errorf("합의 시작 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, status)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "DecisionID", Value: status.DecisionID},
		{Key: "Status", Value: status.Status},
		{Key: "Votes", Value: fmt.Sprintf("%d", status.Votes)},
		{Key: "Required", Value: fmt.Sprintf("%d", status.Required)},
	})
	return nil
}

// runDecisionVote는 의사결정에 투표합니다.
func runDecisionVote(client *apiclient.Client, out io.Writer, decisionID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(decisionID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	status, err := apiclient.Do[ConsensusStatus](client, ctx, "POST", "/api/v1/decisions/"+decisionID+"/consensus/vote", nil)
	if err != nil {
		return fmt.Errorf("투표 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, status)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "DecisionID", Value: status.DecisionID},
		{Key: "Status", Value: status.Status},
		{Key: "Votes", Value: fmt.Sprintf("%d", status.Votes)},
		{Key: "Required", Value: fmt.Sprintf("%d", status.Required)},
	})
	return nil
}

// runDecisionConfidence는 신뢰도 점수를 조회합니다.
func runDecisionConfidence(client *apiclient.Client, out io.Writer, decisionID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(decisionID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	score, err := apiclient.Do[ConfidenceScore](client, ctx, "GET", "/api/v1/decisions/"+decisionID+"/confidence", nil)
	if err != nil {
		return fmt.Errorf("신뢰도 점수 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, score)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "DecisionID", Value: score.DecisionID},
		{Key: "Score", Value: fmt.Sprintf("%g", score.Score)},
		{Key: "Factors", Value: score.Factors},
	})
	return nil
}
