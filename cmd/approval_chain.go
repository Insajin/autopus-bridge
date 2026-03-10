// approval_chain.go는 승인 체인 관련 CLI 명령어를 구현합니다.
// approval-chain templates/create-template/list/start/show/approve/reject 서브커맨드
package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/spf13/cobra"
)

// ApprovalChainTemplate는 승인 체인 템플릿 정보를 나타냅니다.
type ApprovalChainTemplate struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Steps       int    `json:"steps,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

// ApprovalChain은 승인 체인 인스턴스 정보를 나타냅니다.
type ApprovalChain struct {
	ID          string `json:"id"`
	TemplateID  string `json:"template_id,omitempty"`
	Status      string `json:"status,omitempty"`
	CurrentStep int    `json:"current_step,omitempty"`
	TotalSteps  int    `json:"total_steps,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

var (
	approvalChainJSONOutput        bool
	approvalChainTemplateName      string
	approvalChainTemplateDesc      string
	approvalChainTemplateSteps     string
	approvalChainStartTemplateID   string
)

// approvalChainCmd는 approval-chain 서브커맨드의 루트입니다.
var approvalChainCmd = &cobra.Command{
	Use:   "approval-chain",
	Short: "승인 체인 관련 명령어",
	Long:  `승인 체인 템플릿 조회, 템플릿 생성, 체인 목록 조회, 시작, 상세 조회, 승인, 거절 기능을 제공합니다.`,
}

// approvalChainTemplatesCmd는 승인 체인 템플릿 목록을 조회합니다.
var approvalChainTemplatesCmd = &cobra.Command{
	Use:   "templates",
	Short: "승인 체인 템플릿 목록 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runApprovalChainTemplates(client, os.Stdout, json)
	},
}

// approvalChainCreateTemplateCmd는 새 승인 체인 템플릿을 생성합니다.
var approvalChainCreateTemplateCmd = &cobra.Command{
	Use:   "create-template",
	Short: "승인 체인 템플릿 생성",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		name, _ := cmd.Flags().GetString("name")
		desc, _ := cmd.Flags().GetString("description")
		steps, _ := cmd.Flags().GetString("steps")
		client.SetJSONOutput(json)
		return runApprovalChainCreateTemplate(client, os.Stdout, name, desc, steps, json)
	},
}

// approvalChainListCmd는 승인 체인 목록을 조회합니다.
var approvalChainListCmd = &cobra.Command{
	Use:   "list",
	Short: "승인 체인 목록 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runApprovalChainList(client, os.Stdout, json)
	},
}

// approvalChainStartCmd는 새 승인 체인을 시작합니다.
var approvalChainStartCmd = &cobra.Command{
	Use:   "start",
	Short: "승인 체인 시작",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		template, _ := cmd.Flags().GetString("template")
		client.SetJSONOutput(json)
		return runApprovalChainStart(client, os.Stdout, template, json)
	},
}

// approvalChainShowCmd는 승인 체인 상세 정보를 조회합니다.
var approvalChainShowCmd = &cobra.Command{
	Use:   "show <chain-id>",
	Short: "승인 체인 상세 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runApprovalChainShow(client, os.Stdout, args[0], json)
	},
}

// approvalChainApproveCmd는 승인 체인의 단계를 승인합니다.
var approvalChainApproveCmd = &cobra.Command{
	Use:   "approve <chain-id> <step-id>",
	Short: "승인 체인 단계 승인",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runApprovalChainApprove(client, os.Stdout, args[0], args[1])
	},
}

// approvalChainRejectCmd는 승인 체인의 단계를 거절합니다.
var approvalChainRejectCmd = &cobra.Command{
	Use:   "reject <chain-id> <step-id>",
	Short: "승인 체인 단계 거절",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runApprovalChainReject(client, os.Stdout, args[0], args[1])
	},
}

func init() {
	rootCmd.AddCommand(approvalChainCmd)
	approvalChainCmd.AddCommand(approvalChainTemplatesCmd)
	approvalChainCmd.AddCommand(approvalChainCreateTemplateCmd)
	approvalChainCmd.AddCommand(approvalChainListCmd)
	approvalChainCmd.AddCommand(approvalChainStartCmd)
	approvalChainCmd.AddCommand(approvalChainShowCmd)
	approvalChainCmd.AddCommand(approvalChainApproveCmd)
	approvalChainCmd.AddCommand(approvalChainRejectCmd)

	// --json 플래그를 서브커맨드에 추가
	for _, sub := range []*cobra.Command{
		approvalChainTemplatesCmd,
		approvalChainCreateTemplateCmd,
		approvalChainListCmd,
		approvalChainStartCmd,
		approvalChainShowCmd,
	} {
		sub.Flags().BoolVar(&approvalChainJSONOutput, "json", false, "JSON 형식으로 출력")
	}

	// create-template 전용 플래그
	approvalChainCreateTemplateCmd.Flags().StringVar(&approvalChainTemplateName, "name", "", "템플릿 이름 (필수)")
	approvalChainCreateTemplateCmd.Flags().StringVar(&approvalChainTemplateDesc, "description", "", "템플릿 설명")
	approvalChainCreateTemplateCmd.Flags().StringVar(&approvalChainTemplateSteps, "steps", "", "단계 정의 (JSON 문자열)")

	// start 전용 플래그
	approvalChainStartCmd.Flags().StringVar(&approvalChainStartTemplateID, "template", "", "템플릿 ID")
}

// runApprovalChainTemplates는 승인 체인 템플릿 목록을 조회하고 출력합니다.
func runApprovalChainTemplates(client *apiclient.Client, out io.Writer, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	templates, err := apiclient.DoList[ApprovalChainTemplate](client, ctx, "GET",
		"/api/v1/workspaces/"+workspaceID+"/approval-chains/templates", nil)
	if err != nil {
		return fmt.Errorf("승인 체인 템플릿 목록 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, templates)
	}

	headers := []string{"ID", "NAME", "STEPS"}
	rows := make([][]string, len(templates))
	for i, tmpl := range templates {
		shortID := tmpl.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		rows[i] = []string{shortID, tmpl.Name, fmt.Sprintf("%d", tmpl.Steps)}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runApprovalChainCreateTemplate는 새 승인 체인 템플릿을 생성합니다.
func runApprovalChainCreateTemplate(client *apiclient.Client, out io.Writer, name, description, steps string, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	body := map[string]interface{}{
		"name": name,
	}
	if description != "" {
		body["description"] = description
	}
	if steps != "" {
		body["steps"] = steps
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	tmpl, err := apiclient.Do[ApprovalChainTemplate](client, ctx, "POST",
		"/api/v1/workspaces/"+workspaceID+"/approval-chains/templates", body)
	if err != nil {
		return fmt.Errorf("승인 체인 템플릿 생성 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, tmpl)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: tmpl.ID},
		{Key: "Name", Value: tmpl.Name},
		{Key: "Description", Value: tmpl.Description},
		{Key: "Steps", Value: fmt.Sprintf("%d", tmpl.Steps)},
	})
	return nil
}

// runApprovalChainList는 승인 체인 목록을 조회하고 출력합니다.
func runApprovalChainList(client *apiclient.Client, out io.Writer, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	chains, err := apiclient.DoList[ApprovalChain](client, ctx, "GET",
		"/api/v1/workspaces/"+workspaceID+"/approval-chains", nil)
	if err != nil {
		return fmt.Errorf("승인 체인 목록 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, chains)
	}

	headers := []string{"ID", "TEMPLATE", "STATUS", "CURRENT", "TOTAL"}
	rows := make([][]string, len(chains))
	for i, ch := range chains {
		shortID := ch.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		shortTmplID := ch.TemplateID
		if len(shortTmplID) > 8 {
			shortTmplID = shortTmplID[:8]
		}
		rows[i] = []string{
			shortID,
			shortTmplID,
			ch.Status,
			fmt.Sprintf("%d", ch.CurrentStep),
			fmt.Sprintf("%d", ch.TotalSteps),
		}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runApprovalChainStart는 새 승인 체인을 시작합니다.
func runApprovalChainStart(client *apiclient.Client, out io.Writer, templateID string, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()
	body := map[string]string{"template_id": templateID}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	chain, err := apiclient.Do[ApprovalChain](client, ctx, "POST",
		"/api/v1/workspaces/"+workspaceID+"/approval-chains", body)
	if err != nil {
		return fmt.Errorf("승인 체인 시작 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, chain)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: chain.ID},
		{Key: "TemplateID", Value: chain.TemplateID},
		{Key: "Status", Value: chain.Status},
		{Key: "CurrentStep", Value: fmt.Sprintf("%d", chain.CurrentStep)},
		{Key: "TotalSteps", Value: fmt.Sprintf("%d", chain.TotalSteps)},
	})
	return nil
}

// runApprovalChainShow는 승인 체인 상세 정보를 출력합니다.
func runApprovalChainShow(client *apiclient.Client, out io.Writer, chainID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(chainID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	chain, err := apiclient.Do[ApprovalChain](client, ctx, "GET",
		"/api/v1/approval-chains/"+chainID, nil)
	if err != nil {
		return fmt.Errorf("승인 체인 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, chain)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: chain.ID},
		{Key: "TemplateID", Value: chain.TemplateID},
		{Key: "Status", Value: chain.Status},
		{Key: "CurrentStep", Value: fmt.Sprintf("%d", chain.CurrentStep)},
		{Key: "TotalSteps", Value: fmt.Sprintf("%d", chain.TotalSteps)},
		{Key: "CreatedAt", Value: chain.CreatedAt},
	})
	return nil
}

// runApprovalChainApprove는 승인 체인의 단계를 승인합니다.
func runApprovalChainApprove(client *apiclient.Client, out io.Writer, chainID, stepID string) error {
	if err := apiclient.ValidateID(chainID); err != nil {
		return err
	}
	if err := apiclient.ValidateID(stepID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	_, err := apiclient.Do[map[string]interface{}](client, ctx, "POST",
		"/api/v1/approval-chains/"+chainID+"/steps/"+stepID+"/approve", nil)
	if err != nil {
		return fmt.Errorf("승인 체인 단계 승인 실패: %w", err)
	}

	fmt.Fprintf(out, "승인 완료: chain=%s, step=%s\n", chainID, stepID)
	return nil
}

// runApprovalChainReject는 승인 체인의 단계를 거절합니다.
func runApprovalChainReject(client *apiclient.Client, out io.Writer, chainID, stepID string) error {
	if err := apiclient.ValidateID(chainID); err != nil {
		return err
	}
	if err := apiclient.ValidateID(stepID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	_, err := apiclient.Do[map[string]interface{}](client, ctx, "POST",
		"/api/v1/approval-chains/"+chainID+"/steps/"+stepID+"/reject", nil)
	if err != nil {
		return fmt.Errorf("승인 체인 단계 거절 실패: %w", err)
	}

	fmt.Fprintf(out, "거절 완료: chain=%s, step=%s\n", chainID, stepID)
	return nil
}
