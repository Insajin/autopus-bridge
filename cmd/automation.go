// automation.go는 자동화 관련 CLI 명령어를 구현합니다.
// automation list/show/create/update/delete/toggle/add-action 서브커맨드
package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/spf13/cobra"
)

// Automation은 자동화 기본 정보를 나타냅니다.
type Automation struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description,omitempty"`
	TriggerType   string `json:"trigger_type,omitempty"`
	TriggerConfig string `json:"trigger_config,omitempty"`
	AgentID       string `json:"agent_id,omitempty"`
	IsActive      bool   `json:"is_active"`
	CreatedAt     string `json:"created_at,omitempty"`
}

// AutomationAction은 자동화 액션 정보를 나타냅니다.
type AutomationAction struct {
	ID           string `json:"id"`
	ActionType   string `json:"action_type"`
	ActionConfig string `json:"action_config,omitempty"`
	OrderIndex   int    `json:"order_index"`
}

var (
	automationJSONOutput    bool
	automationName          string
	automationDesc          string
	automationTriggerType   string
	automationTriggerConfig string
	automationAgentID       string
	automationActionType    string
	automationActionConfig  string
	automationActionOrder   int
)

// automationCmd는 automation 서브커맨드의 루트입니다.
var automationCmd = &cobra.Command{
	Use:   "automation",
	Short: "자동화 관련 명령어",
	Long:  `자동화 목록 조회, 상세 조회, 생성, 수정, 삭제, 토글, 액션 추가 기능을 제공합니다.`,
}

// automationListCmd는 프로젝트의 자동화 목록을 조회합니다.
var automationListCmd = &cobra.Command{
	Use:   "list <project-id>",
	Short: "자동화 목록 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runAutomationList(client, os.Stdout, args[0], json)
	},
}

// automationShowCmd는 자동화 상세 정보를 조회합니다.
var automationShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "자동화 상세 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runAutomationShow(client, os.Stdout, args[0], json)
	},
}

// automationCreateCmd는 새 자동화를 생성합니다.
var automationCreateCmd = &cobra.Command{
	Use:   "create <project-id>",
	Short: "자동화 생성",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		name, _ := cmd.Flags().GetString("name")
		triggerType, _ := cmd.Flags().GetString("trigger-type")
		triggerConfig, _ := cmd.Flags().GetString("trigger-config")
		agentID, _ := cmd.Flags().GetString("agent")
		client.SetJSONOutput(json)
		return runAutomationCreate(client, os.Stdout, args[0], name, triggerType, triggerConfig, agentID, json)
	},
}

// automationUpdateCmd는 자동화를 수정합니다.
var automationUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "자동화 수정",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		name, _ := cmd.Flags().GetString("name")
		desc, _ := cmd.Flags().GetString("description")
		triggerType, _ := cmd.Flags().GetString("trigger-type")
		triggerConfig, _ := cmd.Flags().GetString("trigger-config")
		agentID, _ := cmd.Flags().GetString("agent")
		client.SetJSONOutput(json)
		return runAutomationUpdate(client, os.Stdout, args[0], name, desc, triggerType, triggerConfig, agentID, json)
	},
}

// automationDeleteCmd는 자동화를 삭제합니다.
var automationDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "자동화 삭제",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runAutomationDelete(client, os.Stdout, args[0])
	},
}

// automationToggleCmd는 자동화 활성/비활성 상태를 토글합니다.
var automationToggleCmd = &cobra.Command{
	Use:   "toggle <id>",
	Short: "자동화 활성/비활성 토글",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runAutomationToggle(client, os.Stdout, args[0], json)
	},
}

// automationAddActionCmd는 자동화에 액션을 추가합니다.
var automationAddActionCmd = &cobra.Command{
	Use:   "add-action <id>",
	Short: "자동화 액션 추가",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		actionType, _ := cmd.Flags().GetString("type")
		actionConfig, _ := cmd.Flags().GetString("config")
		order, _ := cmd.Flags().GetInt("order")
		client.SetJSONOutput(json)
		return runAutomationAddAction(client, os.Stdout, args[0], actionType, actionConfig, order, json)
	},
}

func init() {
	rootCmd.AddCommand(automationCmd)
	automationCmd.AddCommand(automationListCmd)
	automationCmd.AddCommand(automationShowCmd)
	automationCmd.AddCommand(automationCreateCmd)
	automationCmd.AddCommand(automationUpdateCmd)
	automationCmd.AddCommand(automationDeleteCmd)
	automationCmd.AddCommand(automationToggleCmd)
	automationCmd.AddCommand(automationAddActionCmd)

	// --json 플래그를 출력이 있는 서브커맨드에 추가
	for _, sub := range []*cobra.Command{
		automationListCmd, automationShowCmd, automationCreateCmd,
		automationUpdateCmd, automationToggleCmd, automationAddActionCmd,
	} {
		sub.Flags().BoolVar(&automationJSONOutput, "json", false, "JSON 형식으로 출력")
	}

	// create 전용 플래그
	automationCreateCmd.Flags().StringVar(&automationName, "name", "", "자동화 이름 (필수)")
	automationCreateCmd.Flags().StringVar(&automationTriggerType, "trigger-type", "", "트리거 유형")
	automationCreateCmd.Flags().StringVar(&automationTriggerConfig, "trigger-config", "", "트리거 설정")
	automationCreateCmd.Flags().StringVar(&automationAgentID, "agent", "", "연결할 에이전트 ID")

	// update 전용 플래그
	automationUpdateCmd.Flags().StringVar(&automationName, "name", "", "자동화 이름")
	automationUpdateCmd.Flags().StringVar(&automationDesc, "description", "", "자동화 설명")
	automationUpdateCmd.Flags().StringVar(&automationTriggerType, "trigger-type", "", "트리거 유형")
	automationUpdateCmd.Flags().StringVar(&automationTriggerConfig, "trigger-config", "", "트리거 설정")
	automationUpdateCmd.Flags().StringVar(&automationAgentID, "agent", "", "연결할 에이전트 ID")

	// add-action 전용 플래그
	automationAddActionCmd.Flags().StringVar(&automationActionType, "type", "", "액션 유형 (필수)")
	automationAddActionCmd.Flags().StringVar(&automationActionConfig, "config", "", "액션 설정")
	automationAddActionCmd.Flags().IntVar(&automationActionOrder, "order", 0, "액션 순서")
}

// runAutomationList는 프로젝트의 자동화 목록을 조회하고 출력합니다.
func runAutomationList(client *apiclient.Client, out io.Writer, projectID string, jsonOutput bool) error {
	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	automations, err := apiclient.DoList[Automation](client, ctx, "GET",
		"/api/v1/projects/"+projectID+"/automations", nil)
	if err != nil {
		return fmt.Errorf("자동화 목록 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, automations)
	}

	headers := []string{"ID", "NAME", "TRIGGER", "ACTIVE", "AGENT"}
	rows := make([][]string, len(automations))
	for i, a := range automations {
		// ID는 첫 8자만 표시
		shortID := a.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		rows[i] = []string{
			shortID,
			a.Name,
			a.TriggerType,
			fmt.Sprintf("%v", a.IsActive),
			a.AgentID,
		}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runAutomationShow는 자동화 상세 정보를 출력합니다.
func runAutomationShow(client *apiclient.Client, out io.Writer, automationID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(automationID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	a, err := apiclient.Do[Automation](client, ctx, "GET", "/api/v1/automations/"+automationID, nil)
	if err != nil {
		return fmt.Errorf("자동화 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, a)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: a.ID},
		{Key: "Name", Value: a.Name},
		{Key: "Description", Value: a.Description},
		{Key: "TriggerType", Value: a.TriggerType},
		{Key: "TriggerConfig", Value: a.TriggerConfig},
		{Key: "AgentID", Value: a.AgentID},
		{Key: "IsActive", Value: fmt.Sprintf("%v", a.IsActive)},
		{Key: "CreatedAt", Value: a.CreatedAt},
	})
	return nil
}

// runAutomationCreate는 새 자동화를 생성합니다.
func runAutomationCreate(client *apiclient.Client, out io.Writer, projectID, name, triggerType, triggerConfig, agentID string, jsonOutput bool) error {
	// 빈 값을 제외한 body 구성
	body := map[string]interface{}{}
	if name != "" {
		body["name"] = name
	}
	if triggerType != "" {
		body["trigger_type"] = triggerType
	}
	if triggerConfig != "" {
		body["trigger_config"] = triggerConfig
	}
	if agentID != "" {
		body["agent_id"] = agentID
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	a, err := apiclient.Do[Automation](client, ctx, "POST",
		"/api/v1/projects/"+projectID+"/automations", body)
	if err != nil {
		return fmt.Errorf("자동화 생성 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, a)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: a.ID},
		{Key: "Name", Value: a.Name},
		{Key: "TriggerType", Value: a.TriggerType},
		{Key: "IsActive", Value: fmt.Sprintf("%v", a.IsActive)},
	})
	return nil
}

// runAutomationUpdate는 자동화를 수정합니다.
func runAutomationUpdate(client *apiclient.Client, out io.Writer, automationID, name, desc, triggerType, triggerConfig, agentID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(automationID); err != nil {
		return err
	}

	// 제공된 필드만 포함하는 body 구성
	body := map[string]interface{}{}
	if name != "" {
		body["name"] = name
	}
	if desc != "" {
		body["description"] = desc
	}
	if triggerType != "" {
		body["trigger_type"] = triggerType
	}
	if triggerConfig != "" {
		body["trigger_config"] = triggerConfig
	}
	if agentID != "" {
		body["agent_id"] = agentID
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	a, err := apiclient.Do[Automation](client, ctx, "PATCH", "/api/v1/automations/"+automationID, body)
	if err != nil {
		return fmt.Errorf("자동화 수정 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, a)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: a.ID},
		{Key: "Name", Value: a.Name},
		{Key: "TriggerType", Value: a.TriggerType},
		{Key: "IsActive", Value: fmt.Sprintf("%v", a.IsActive)},
	})
	return nil
}

// runAutomationDelete는 자동화를 삭제합니다.
func runAutomationDelete(client *apiclient.Client, out io.Writer, automationID string) error {
	if err := apiclient.ValidateID(automationID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	_, err := apiclient.Do[map[string]interface{}](client, ctx, "DELETE",
		"/api/v1/automations/"+automationID, nil)
	if err != nil {
		return fmt.Errorf("자동화 삭제 실패: %w", err)
	}

	fmt.Fprintf(out, "자동화 삭제 완료: %s\n", automationID)
	return nil
}

// runAutomationToggle은 자동화 활성/비활성 상태를 토글합니다.
func runAutomationToggle(client *apiclient.Client, out io.Writer, automationID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(automationID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	a, err := apiclient.Do[Automation](client, ctx, "POST",
		"/api/v1/automations/"+automationID+"/toggle", nil)
	if err != nil {
		return fmt.Errorf("자동화 토글 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, a)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: a.ID},
		{Key: "Name", Value: a.Name},
		{Key: "IsActive", Value: fmt.Sprintf("%v", a.IsActive)},
	})
	return nil
}

// runAutomationAddAction은 자동화에 액션을 추가합니다.
func runAutomationAddAction(client *apiclient.Client, out io.Writer, automationID, actionType, actionConfig string, orderIndex int, jsonOutput bool) error {
	if err := apiclient.ValidateID(automationID); err != nil {
		return err
	}

	// 빈 값을 제외한 body 구성
	body := map[string]interface{}{
		"action_type": actionType,
		"order_index": orderIndex,
	}
	if actionConfig != "" {
		body["action_config"] = actionConfig
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	action, err := apiclient.Do[AutomationAction](client, ctx, "POST",
		"/api/v1/automations/"+automationID+"/actions", body)
	if err != nil {
		return fmt.Errorf("자동화 액션 추가 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, action)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: action.ID},
		{Key: "ActionType", Value: action.ActionType},
		{Key: "OrderIndex", Value: fmt.Sprintf("%d", action.OrderIndex)},
	})
	return nil
}
