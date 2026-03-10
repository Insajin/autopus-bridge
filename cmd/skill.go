// skill.go는 스킬 레지스트리 관련 CLI 명령어를 구현합니다.
// skill list/show/sync/quality/versions/rollback/executions/agent-skills/assign/unassign/recommend/auto-assign 서브커맨드
package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/spf13/cobra"
)

// SkillEntry는 스킬 레지스트리의 기본 스킬 정보를 나타냅니다.
type SkillEntry struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Version     string `json:"version,omitempty"`
	Status      string `json:"status,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

// SkillQuality는 스킬 품질 지표를 나타냅니다.
type SkillQuality struct {
	SkillID     string  `json:"skill_id"`
	Score       float64 `json:"score"`
	Executions  int     `json:"executions"`
	SuccessRate float64 `json:"success_rate"`
}

// SkillVersion은 스킬 버전 정보를 나타냅니다.
type SkillVersion struct {
	ID        string `json:"id"`
	SkillID   string `json:"skill_id"`
	Version   string `json:"version"`
	CreatedAt string `json:"created_at,omitempty"`
}

// SkillExecution은 스킬 실행 이력을 나타냅니다.
type SkillExecution struct {
	ID        string `json:"id"`
	SkillID   string `json:"skill_id"`
	Status    string `json:"status"`
	Duration  string `json:"duration,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

// AgentSkill은 에이전트에 할당된 스킬을 나타냅니다.
type AgentSkill struct {
	ID      string `json:"id"`
	SkillID string `json:"skill_id"`
	Name    string `json:"name"`
	Level   string `json:"level,omitempty"`
}

// SkillRecommendation은 에이전트에 대한 스킬 추천 정보를 나타냅니다.
type SkillRecommendation struct {
	SkillID string  `json:"skill_id"`
	Name    string  `json:"name"`
	Score   float64 `json:"score"`
	Reason  string  `json:"reason,omitempty"`
}

var skillJSONOutput bool

// skillCmd는 skill 서브커맨드의 루트입니다.
var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "스킬 레지스트리 관련 명령어",
	Long:  `스킬 목록 조회, 상세 조회, 동기화, 품질 지표, 버전 관리, 실행 이력, 에이전트 스킬 관련 기능을 제공합니다.`,
}

var skillListCmd = &cobra.Command{
	Use:   "list",
	Short: "스킬 목록 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runSkillList(client, os.Stdout, json)
	},
}

var skillShowCmd = &cobra.Command{
	Use:   "show <skill-id>",
	Short: "스킬 상세 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runSkillShow(client, os.Stdout, args[0], json)
	},
}

var skillSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "스킬 동기화",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runSkillSync(client, os.Stdout)
	},
}

var skillQualityCmd = &cobra.Command{
	Use:   "quality <skill-id>",
	Short: "스킬 품질 지표 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runSkillQuality(client, os.Stdout, args[0])
	},
}

var skillVersionsCmd = &cobra.Command{
	Use:   "versions <skill-id>",
	Short: "스킬 버전 목록 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runSkillVersions(client, os.Stdout, args[0])
	},
}

var skillRollbackCmd = &cobra.Command{
	Use:   "rollback <skill-id> <version-id>",
	Short: "스킬 버전 롤백",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runSkillRollback(client, os.Stdout, args[0], args[1])
	},
}

var skillExecutionsCmd = &cobra.Command{
	Use:   "executions <skill-id>",
	Short: "스킬 실행 이력 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runSkillExecutions(client, os.Stdout, args[0])
	},
}

var agentSkillsCmd = &cobra.Command{
	Use:   "agent-skills <agent-id>",
	Short: "에이전트 스킬 목록 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runAgentSkills(client, os.Stdout, args[0], json)
	},
}

var skillAssignCmd = &cobra.Command{
	Use:   "assign <agent-id> <skill-id>",
	Short: "에이전트에 스킬 할당",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runSkillAssign(client, os.Stdout, args[0], args[1])
	},
}

var skillUnassignCmd = &cobra.Command{
	Use:   "unassign <agent-id> <skill-id>",
	Short: "에이전트에서 스킬 해제",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runSkillUnassign(client, os.Stdout, args[0], args[1])
	},
}

var skillRecommendCmd = &cobra.Command{
	Use:   "recommend <agent-id>",
	Short: "에이전트 스킬 추천",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runSkillRecommend(client, os.Stdout, args[0])
	},
}

var skillAutoAssignCmd = &cobra.Command{
	Use:   "auto-assign <agent-id>",
	Short: "에이전트 스킬 자동 할당",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runSkillAutoAssign(client, os.Stdout, args[0])
	},
}

func init() {
	rootCmd.AddCommand(skillCmd)
	skillCmd.AddCommand(skillListCmd)
	skillCmd.AddCommand(skillShowCmd)
	skillCmd.AddCommand(skillSyncCmd)
	skillCmd.AddCommand(skillQualityCmd)
	skillCmd.AddCommand(skillVersionsCmd)
	skillCmd.AddCommand(skillRollbackCmd)
	skillCmd.AddCommand(skillExecutionsCmd)
	skillCmd.AddCommand(agentSkillsCmd)
	skillCmd.AddCommand(skillAssignCmd)
	skillCmd.AddCommand(skillUnassignCmd)
	skillCmd.AddCommand(skillRecommendCmd)
	skillCmd.AddCommand(skillAutoAssignCmd)

	// --json 플래그를 서브커맨드에 추가
	for _, sub := range []*cobra.Command{skillListCmd, skillShowCmd, agentSkillsCmd, skillRollbackCmd} {
		sub.Flags().BoolVar(&skillJSONOutput, "json", false, "JSON 형식으로 출력")
	}
}

// runSkillList는 워크스페이스의 스킬 목록을 조회합니다.
func runSkillList(client *apiclient.Client, out io.Writer, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	skills, err := apiclient.DoList[SkillEntry](client, ctx, "GET",
		"/api/v1/workspaces/"+workspaceID+"/skills/registry", nil)
	if err != nil {
		return fmt.Errorf("스킬 목록 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, skills)
	}

	headers := []string{"ID", "NAME", "VERSION", "STATUS"}
	rows := make([][]string, len(skills))
	for i, s := range skills {
		shortID := s.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		rows[i] = []string{shortID, s.Name, s.Version, s.Status}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runSkillShow는 스킬 상세 정보를 출력합니다.
func runSkillShow(client *apiclient.Client, out io.Writer, skillID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(skillID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	skill, err := apiclient.Do[SkillEntry](client, ctx, "GET",
		"/api/v1/skills/registry/"+skillID, nil)
	if err != nil {
		return fmt.Errorf("스킬 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, skill)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: skill.ID},
		{Key: "Name", Value: skill.Name},
		{Key: "Description", Value: skill.Description},
		{Key: "Version", Value: skill.Version},
		{Key: "Status", Value: skill.Status},
		{Key: "CreatedAt", Value: skill.CreatedAt},
	})
	return nil
}

// runSkillSync는 워크스페이스의 스킬을 동기화합니다.
func runSkillSync(client *apiclient.Client, out io.Writer) error {
	workspaceID := client.WorkspaceID()

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	_, err := apiclient.Do[map[string]interface{}](client, ctx, "POST",
		"/api/v1/workspaces/"+workspaceID+"/skills/sync", nil)
	if err != nil {
		return fmt.Errorf("스킬 동기화 실패: %w", err)
	}

	fmt.Fprintln(out, "스킬 동기화 완료")
	return nil
}

// runSkillQuality는 스킬 품질 지표를 출력합니다.
func runSkillQuality(client *apiclient.Client, out io.Writer, skillID string) error {
	if err := apiclient.ValidateID(skillID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	quality, err := apiclient.Do[SkillQuality](client, ctx, "GET",
		"/api/v1/skills/registry/"+skillID+"/quality", nil)
	if err != nil {
		return fmt.Errorf("스킬 품질 지표 조회 실패: %w", err)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "SkillID", Value: quality.SkillID},
		{Key: "Score", Value: fmt.Sprintf("%.2f", quality.Score)},
		{Key: "Executions", Value: fmt.Sprintf("%d", quality.Executions)},
		{Key: "SuccessRate", Value: fmt.Sprintf("%.2f", quality.SuccessRate)},
	})
	return nil
}

// runSkillVersions는 스킬 버전 목록을 출력합니다.
func runSkillVersions(client *apiclient.Client, out io.Writer, skillID string) error {
	if err := apiclient.ValidateID(skillID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	versions, err := apiclient.DoList[SkillVersion](client, ctx, "GET",
		"/api/v1/skills/registry/"+skillID+"/versions", nil)
	if err != nil {
		return fmt.Errorf("스킬 버전 목록 조회 실패: %w", err)
	}

	headers := []string{"ID", "VERSION", "CREATED_AT"}
	rows := make([][]string, len(versions))
	for i, v := range versions {
		shortID := v.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		rows[i] = []string{shortID, v.Version, v.CreatedAt}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runSkillRollback는 스킬을 특정 버전으로 롤백합니다.
func runSkillRollback(client *apiclient.Client, out io.Writer, skillID, versionID string) error {
	if err := apiclient.ValidateID(skillID); err != nil {
		return err
	}
	if err := apiclient.ValidateID(versionID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	skill, err := apiclient.Do[SkillEntry](client, ctx, "POST",
		"/api/v1/skills/registry/"+skillID+"/versions/"+versionID+"/rollback", nil)
	if err != nil {
		return fmt.Errorf("스킬 롤백 실패: %w", err)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: skill.ID},
		{Key: "Name", Value: skill.Name},
		{Key: "Version", Value: skill.Version},
		{Key: "Status", Value: skill.Status},
	})
	return nil
}

// runSkillExecutions는 스킬 실행 이력을 출력합니다.
func runSkillExecutions(client *apiclient.Client, out io.Writer, skillID string) error {
	if err := apiclient.ValidateID(skillID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	executions, err := apiclient.DoList[SkillExecution](client, ctx, "GET",
		"/api/v1/skills/registry/"+skillID+"/executions", nil)
	if err != nil {
		return fmt.Errorf("스킬 실행 이력 조회 실패: %w", err)
	}

	headers := []string{"ID", "STATUS", "DURATION", "CREATED_AT"}
	rows := make([][]string, len(executions))
	for i, e := range executions {
		shortID := e.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		rows[i] = []string{shortID, e.Status, e.Duration, e.CreatedAt}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runAgentSkills는 에이전트에 할당된 스킬 목록을 출력합니다.
func runAgentSkills(client *apiclient.Client, out io.Writer, agentID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(agentID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	skills, err := apiclient.DoList[AgentSkill](client, ctx, "GET",
		"/api/v1/agents/"+agentID+"/skills", nil)
	if err != nil {
		return fmt.Errorf("에이전트 스킬 목록 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, skills)
	}

	headers := []string{"ID", "SKILL_ID", "NAME", "LEVEL"}
	rows := make([][]string, len(skills))
	for i, s := range skills {
		shortID := s.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		rows[i] = []string{shortID, s.SkillID, s.Name, s.Level}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runSkillAssign는 에이전트에 스킬을 할당합니다.
func runSkillAssign(client *apiclient.Client, out io.Writer, agentID, skillID string) error {
	if err := apiclient.ValidateID(agentID); err != nil {
		return err
	}
	if err := apiclient.ValidateID(skillID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	body := map[string]string{"skill_id": skillID}
	skill, err := apiclient.Do[AgentSkill](client, ctx, "POST",
		"/api/v1/agents/"+agentID+"/skills", body)
	if err != nil {
		return fmt.Errorf("스킬 할당 실패: %w", err)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: skill.ID},
		{Key: "SkillID", Value: skill.SkillID},
		{Key: "Name", Value: skill.Name},
		{Key: "Level", Value: skill.Level},
	})
	return nil
}

// runSkillUnassign는 에이전트에서 스킬을 해제합니다.
func runSkillUnassign(client *apiclient.Client, out io.Writer, agentID, skillID string) error {
	if err := apiclient.ValidateID(agentID); err != nil {
		return err
	}
	if err := apiclient.ValidateID(skillID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	_, err := apiclient.Do[map[string]interface{}](client, ctx, "DELETE",
		"/api/v1/agents/"+agentID+"/skills/"+skillID, nil)
	if err != nil {
		return fmt.Errorf("스킬 해제 실패: %w", err)
	}

	fmt.Fprintf(out, "스킬 해제 완료: %s\n", skillID)
	return nil
}

// runSkillRecommend는 에이전트에 추천 스킬 목록을 출력합니다.
func runSkillRecommend(client *apiclient.Client, out io.Writer, agentID string) error {
	if err := apiclient.ValidateID(agentID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	recommendations, err := apiclient.DoList[SkillRecommendation](client, ctx, "GET",
		"/api/v1/agents/"+agentID+"/skills/recommendations", nil)
	if err != nil {
		return fmt.Errorf("스킬 추천 조회 실패: %w", err)
	}

	headers := []string{"SKILL_ID", "NAME", "SCORE", "REASON"}
	rows := make([][]string, len(recommendations))
	for i, r := range recommendations {
		rows[i] = []string{r.SkillID, r.Name, fmt.Sprintf("%.2f", r.Score), r.Reason}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runSkillAutoAssign는 에이전트에 스킬을 자동 할당합니다.
func runSkillAutoAssign(client *apiclient.Client, out io.Writer, agentID string) error {
	if err := apiclient.ValidateID(agentID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	skills, err := apiclient.DoList[AgentSkill](client, ctx, "POST",
		"/api/v1/agents/"+agentID+"/skills/auto-assign", nil)
	if err != nil {
		return fmt.Errorf("스킬 자동 할당 실패: %w", err)
	}

	headers := []string{"ID", "SKILL_ID", "NAME", "LEVEL"}
	rows := make([][]string, len(skills))
	for i, s := range skills {
		shortID := s.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		rows[i] = []string{shortID, s.SkillID, s.Name, s.Level}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}
