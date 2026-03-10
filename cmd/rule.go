// rule.go는 트리거 규칙 관련 CLI 명령어를 구현합니다.
// rule list/show/create/update/delete/toggle/logs 서브커맨드
package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/spf13/cobra"
)

// TriggerRule은 트리거 규칙 정보를 나타냅니다.
type TriggerRule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	EventType   string `json:"event_type,omitempty"`
	IsActive    bool   `json:"is_active"`
	CreatedAt   string `json:"created_at,omitempty"`
}

// RuleLog는 규칙 실행 로그 정보를 나타냅니다.
type RuleLog struct {
	ID      string `json:"id"`
	RuleID  string `json:"rule_id"`
	Status  string `json:"status"`
	FiredAt string `json:"fired_at,omitempty"`
	Details string `json:"details,omitempty"`
}

// RuleCreateParams는 규칙 생성에 필요한 파라미터입니다.
type RuleCreateParams struct {
	Name           string
	EventType      string
	EventFilter    string
	Conditions     string
	ConditionLogic string
	Actions        string
	Cooldown       int
	MaxFirings     int
}

// RuleUpdateParams는 규칙 업데이트에 필요한 파라미터입니다.
type RuleUpdateParams struct {
	Name           string
	EventType      string
	EventFilter    string
	Conditions     string
	ConditionLogic string
	Actions        string
}

var (
	ruleJSONOutput    bool
	ruleName          string
	ruleEventType     string
	ruleEventFilter   string
	ruleConditions    string
	ruleCondLogic     string
	ruleActions       string
	ruleCooldown      int
	ruleMaxFirings    int
	ruleUpdateName    string
	ruleUpdateEvent   string
	ruleUpdateFilter  string
	ruleUpdateConds   string
	ruleUpdateLogic   string
	ruleUpdateActions string
)

// ruleCmd는 rule 서브커맨드의 루트입니다.
var ruleCmd = &cobra.Command{
	Use:   "rule",
	Short: "트리거 규칙 관련 명령어",
	Long:  `트리거 규칙 목록 조회, 상세 조회, 생성, 수정, 삭제, 토글, 로그 조회 기능을 제공합니다.`,
}

// ruleListCmd는 규칙 목록을 조회합니다.
var ruleListCmd = &cobra.Command{
	Use:   "list",
	Short: "규칙 목록 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runRuleList(client, os.Stdout, json)
	},
}

// ruleShowCmd는 규칙 상세 정보를 조회합니다.
var ruleShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "규칙 상세 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runRuleShow(client, os.Stdout, args[0], json)
	},
}

// ruleCreateCmd는 새 규칙을 생성합니다.
var ruleCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "규칙 생성",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		name, _ := cmd.Flags().GetString("name")
		eventType, _ := cmd.Flags().GetString("event-type")
		eventFilter, _ := cmd.Flags().GetString("event-filter")
		conditions, _ := cmd.Flags().GetString("conditions")
		condLogic, _ := cmd.Flags().GetString("condition-logic")
		actions, _ := cmd.Flags().GetString("actions")
		cooldown, _ := cmd.Flags().GetInt("cooldown")
		maxFirings, _ := cmd.Flags().GetInt("max-firings")
		client.SetJSONOutput(json)
		params := RuleCreateParams{
			Name:           name,
			EventType:      eventType,
			EventFilter:    eventFilter,
			Conditions:     conditions,
			ConditionLogic: condLogic,
			Actions:        actions,
			Cooldown:       cooldown,
			MaxFirings:     maxFirings,
		}
		return runRuleCreate(client, os.Stdout, params, json)
	},
}

// ruleUpdateCmd는 규칙을 업데이트합니다.
var ruleUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "규칙 수정",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		name, _ := cmd.Flags().GetString("name")
		eventType, _ := cmd.Flags().GetString("event-type")
		eventFilter, _ := cmd.Flags().GetString("event-filter")
		conditions, _ := cmd.Flags().GetString("conditions")
		condLogic, _ := cmd.Flags().GetString("condition-logic")
		actions, _ := cmd.Flags().GetString("actions")
		client.SetJSONOutput(json)
		params := RuleUpdateParams{
			Name:           name,
			EventType:      eventType,
			EventFilter:    eventFilter,
			Conditions:     conditions,
			ConditionLogic: condLogic,
			Actions:        actions,
		}
		return runRuleUpdate(client, os.Stdout, args[0], params, json)
	},
}

// ruleDeleteCmd는 규칙을 삭제합니다.
var ruleDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "규칙 삭제",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runRuleDelete(client, os.Stdout, args[0])
	},
}

// ruleToggleCmd는 규칙의 활성화 상태를 토글합니다.
var ruleToggleCmd = &cobra.Command{
	Use:   "toggle <id>",
	Short: "규칙 활성화 토글",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runRuleToggle(client, os.Stdout, args[0])
	},
}

// ruleLogsCmd는 규칙 실행 로그를 조회합니다.
var ruleLogsCmd = &cobra.Command{
	Use:   "logs <id>",
	Short: "규칙 실행 로그 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runRuleLogs(client, os.Stdout, args[0], json)
	},
}

func init() {
	rootCmd.AddCommand(ruleCmd)
	ruleCmd.AddCommand(ruleListCmd)
	ruleCmd.AddCommand(ruleShowCmd)
	ruleCmd.AddCommand(ruleCreateCmd)
	ruleCmd.AddCommand(ruleUpdateCmd)
	ruleCmd.AddCommand(ruleDeleteCmd)
	ruleCmd.AddCommand(ruleToggleCmd)
	ruleCmd.AddCommand(ruleLogsCmd)

	// --json 플래그를 서브커맨드에 추가
	for _, sub := range []*cobra.Command{ruleListCmd, ruleShowCmd, ruleCreateCmd, ruleUpdateCmd, ruleLogsCmd} {
		sub.Flags().BoolVar(&ruleJSONOutput, "json", false, "JSON 형식으로 출력")
	}

	// create 전용 플래그
	ruleCreateCmd.Flags().StringVar(&ruleName, "name", "", "규칙 이름 (필수)")
	ruleCreateCmd.Flags().StringVar(&ruleEventType, "event-type", "", "이벤트 유형 (필수)")
	ruleCreateCmd.Flags().StringVar(&ruleEventFilter, "event-filter", "", "이벤트 필터 (JSON 문자열)")
	ruleCreateCmd.Flags().StringVar(&ruleConditions, "conditions", "", "조건 (JSON 문자열)")
	ruleCreateCmd.Flags().StringVar(&ruleCondLogic, "condition-logic", "", "조건 논리 (and/or)")
	ruleCreateCmd.Flags().StringVar(&ruleActions, "actions", "", "액션 (JSON 문자열, 필수)")
	ruleCreateCmd.Flags().IntVar(&ruleCooldown, "cooldown", 0, "쿨다운 (초)")
	ruleCreateCmd.Flags().IntVar(&ruleMaxFirings, "max-firings", 0, "최대 실행 횟수")

	// update 전용 플래그
	ruleUpdateCmd.Flags().StringVar(&ruleUpdateName, "name", "", "규칙 이름")
	ruleUpdateCmd.Flags().StringVar(&ruleUpdateEvent, "event-type", "", "이벤트 유형")
	ruleUpdateCmd.Flags().StringVar(&ruleUpdateFilter, "event-filter", "", "이벤트 필터 (JSON 문자열)")
	ruleUpdateCmd.Flags().StringVar(&ruleUpdateConds, "conditions", "", "조건 (JSON 문자열)")
	ruleUpdateCmd.Flags().StringVar(&ruleUpdateLogic, "condition-logic", "", "조건 논리 (and/or)")
	ruleUpdateCmd.Flags().StringVar(&ruleUpdateActions, "actions", "", "액션 (JSON 문자열)")
}

// runRuleList는 규칙 목록을 조회하고 출력합니다.
func runRuleList(client *apiclient.Client, out io.Writer, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	rules, err := apiclient.DoList[TriggerRule](client, ctx, "GET",
		"/api/v1/workspaces/"+workspaceID+"/rules", nil)
	if err != nil {
		return fmt.Errorf("규칙 목록 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, rules)
	}

	headers := []string{"ID", "NAME", "EVENT_TYPE", "ACTIVE"}
	rows := make([][]string, len(rules))
	for i, r := range rules {
		shortID := r.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		rows[i] = []string{shortID, r.Name, r.EventType, fmt.Sprintf("%v", r.IsActive)}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runRuleShow는 규칙 상세 정보를 출력합니다.
func runRuleShow(client *apiclient.Client, out io.Writer, ruleID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(ruleID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	rule, err := apiclient.Do[TriggerRule](client, ctx, "GET",
		"/api/v1/rules/"+ruleID, nil)
	if err != nil {
		return fmt.Errorf("규칙 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, rule)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: rule.ID},
		{Key: "Name", Value: rule.Name},
		{Key: "Description", Value: rule.Description},
		{Key: "EventType", Value: rule.EventType},
		{Key: "IsActive", Value: fmt.Sprintf("%v", rule.IsActive)},
		{Key: "CreatedAt", Value: rule.CreatedAt},
	})
	return nil
}

// runRuleCreate는 새 규칙을 생성합니다.
func runRuleCreate(client *apiclient.Client, out io.Writer, params RuleCreateParams, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	body := map[string]interface{}{
		"name":       params.Name,
		"event_type": params.EventType,
	}
	if params.EventFilter != "" {
		body["event_filter"] = params.EventFilter
	}
	if params.Conditions != "" {
		body["conditions"] = params.Conditions
	}
	if params.ConditionLogic != "" {
		body["condition_logic"] = params.ConditionLogic
	}
	if params.Actions != "" {
		body["actions"] = params.Actions
	}
	if params.Cooldown > 0 {
		body["cooldown"] = params.Cooldown
	}
	if params.MaxFirings > 0 {
		body["max_firings"] = params.MaxFirings
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	rule, err := apiclient.Do[TriggerRule](client, ctx, "POST",
		"/api/v1/workspaces/"+workspaceID+"/rules", body)
	if err != nil {
		return fmt.Errorf("규칙 생성 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, rule)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: rule.ID},
		{Key: "Name", Value: rule.Name},
		{Key: "EventType", Value: rule.EventType},
		{Key: "IsActive", Value: fmt.Sprintf("%v", rule.IsActive)},
	})
	return nil
}

// runRuleUpdate는 규칙을 업데이트합니다.
func runRuleUpdate(client *apiclient.Client, out io.Writer, ruleID string, params RuleUpdateParams, jsonOutput bool) error {
	if err := apiclient.ValidateID(ruleID); err != nil {
		return err
	}

	body := map[string]interface{}{}
	if params.Name != "" {
		body["name"] = params.Name
	}
	if params.EventType != "" {
		body["event_type"] = params.EventType
	}
	if params.EventFilter != "" {
		body["event_filter"] = params.EventFilter
	}
	if params.Conditions != "" {
		body["conditions"] = params.Conditions
	}
	if params.ConditionLogic != "" {
		body["condition_logic"] = params.ConditionLogic
	}
	if params.Actions != "" {
		body["actions"] = params.Actions
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	rule, err := apiclient.Do[TriggerRule](client, ctx, "PATCH",
		"/api/v1/rules/"+ruleID, body)
	if err != nil {
		return fmt.Errorf("규칙 수정 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, rule)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: rule.ID},
		{Key: "Name", Value: rule.Name},
		{Key: "EventType", Value: rule.EventType},
		{Key: "IsActive", Value: fmt.Sprintf("%v", rule.IsActive)},
	})
	return nil
}

// runRuleDelete는 규칙을 삭제합니다.
func runRuleDelete(client *apiclient.Client, out io.Writer, ruleID string) error {
	if err := apiclient.ValidateID(ruleID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	_, err := apiclient.Do[map[string]interface{}](client, ctx, "DELETE",
		"/api/v1/rules/"+ruleID, nil)
	if err != nil {
		return fmt.Errorf("규칙 삭제 실패: %w", err)
	}

	fmt.Fprintf(out, "규칙 삭제 완료: %s\n", ruleID)
	return nil
}

// runRuleToggle는 규칙의 활성화 상태를 토글합니다.
func runRuleToggle(client *apiclient.Client, out io.Writer, ruleID string) error {
	if err := apiclient.ValidateID(ruleID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	rule, err := apiclient.Do[TriggerRule](client, ctx, "PATCH",
		"/api/v1/rules/"+ruleID+"/toggle", nil)
	if err != nil {
		return fmt.Errorf("규칙 토글 실패: %w", err)
	}

	fmt.Fprintf(out, "규칙 토글 완료: %s (is_active=%v)\n", ruleID, rule.IsActive)
	return nil
}

// runRuleLogs는 규칙 실행 로그를 조회하고 출력합니다.
func runRuleLogs(client *apiclient.Client, out io.Writer, ruleID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(ruleID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	logs, err := apiclient.DoList[RuleLog](client, ctx, "GET",
		"/api/v1/rules/"+ruleID+"/logs", nil)
	if err != nil {
		return fmt.Errorf("규칙 로그 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, logs)
	}

	headers := []string{"ID", "STATUS", "FIRED_AT"}
	rows := make([][]string, len(logs))
	for i, l := range logs {
		shortID := l.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		rows[i] = []string{shortID, l.Status, l.FiredAt}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}
