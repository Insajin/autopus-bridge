// template.go는 에이전트 템플릿 관련 CLI 명령어를 구현합니다.
// template list/show/domains/categories/deploy 서브커맨드
package cmd

import (
	"fmt"
	"io"
	"net/url"
	"os"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/spf13/cobra"
)

// AgentTemplate는 에이전트 템플릿 정보를 나타냅니다.
type AgentTemplate struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Domain      string `json:"domain,omitempty"`
	Category    string `json:"category,omitempty"`
	Description string `json:"description,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

// TemplateDomain은 템플릿 도메인 정보를 나타냅니다.
type TemplateDomain struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// TemplateCategory는 템플릿 카테고리 정보를 나타냅니다.
type TemplateCategory struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

var (
	templateJSONOutput bool
	templateDomain     string
	templateCategory   string
)

// templateCmd는 template 서브커맨드의 루트입니다.
var templateCmd = &cobra.Command{
	Use:   "template",
	Short: "에이전트 템플릿 관련 명령어",
	Long:  `에이전트 템플릿 목록 조회, 상세 조회, 도메인/카테고리 조회, 배포 기능을 제공합니다.`,
}

var templateListCmd = &cobra.Command{
	Use:   "list",
	Short: "에이전트 템플릿 목록 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		domain, _ := cmd.Flags().GetString("domain")
		category, _ := cmd.Flags().GetString("category")
		client.SetJSONOutput(json)
		return runTemplateList(client, os.Stdout, domain, category, json)
	},
}

var templateShowCmd = &cobra.Command{
	Use:   "show <template-id>",
	Short: "에이전트 템플릿 상세 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runTemplateShow(client, os.Stdout, args[0], json)
	},
}

var templateDomainsCmd = &cobra.Command{
	Use:   "domains",
	Short: "템플릿 도메인 목록 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runTemplateDomains(client, os.Stdout)
	},
}

var templateCategoriesCmd = &cobra.Command{
	Use:   "categories",
	Short: "템플릿 카테고리 목록 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runTemplateCategories(client, os.Stdout)
	},
}

var templateDeployCmd = &cobra.Command{
	Use:   "deploy <template-id>",
	Short: "에이전트 템플릿 배포",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runTemplateDeploy(client, os.Stdout, args[0])
	},
}

func init() {
	rootCmd.AddCommand(templateCmd)
	templateCmd.AddCommand(templateListCmd)
	templateCmd.AddCommand(templateShowCmd)
	templateCmd.AddCommand(templateDomainsCmd)
	templateCmd.AddCommand(templateCategoriesCmd)
	templateCmd.AddCommand(templateDeployCmd)

	// --json 플래그를 서브커맨드에 추가
	for _, sub := range []*cobra.Command{templateListCmd, templateShowCmd} {
		sub.Flags().BoolVar(&templateJSONOutput, "json", false, "JSON 형식으로 출력")
	}

	// list 전용 필터 플래그
	templateListCmd.Flags().StringVar(&templateDomain, "domain", "", "도메인 필터")
	templateListCmd.Flags().StringVar(&templateCategory, "category", "", "카테고리 필터")
}

// runTemplateList는 에이전트 템플릿 목록을 조회합니다.
func runTemplateList(client *apiclient.Client, out io.Writer, domain, category string, jsonOutput bool) error {
	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	// 쿼리 파라미터 구성
	path := "/api/v1/agent-templates/"
	params := url.Values{}
	if domain != "" {
		params.Set("domain", domain)
	}
	if category != "" {
		params.Set("category", category)
	}
	if len(params) > 0 {
		path = path + "?" + params.Encode()
	}

	templates, err := apiclient.DoList[AgentTemplate](client, ctx, "GET", path, nil)
	if err != nil {
		return fmt.Errorf("템플릿 목록 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, templates)
	}

	headers := []string{"ID", "NAME", "DOMAIN", "CATEGORY"}
	rows := make([][]string, len(templates))
	for i, t := range templates {
		shortID := t.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		rows[i] = []string{shortID, t.Name, t.Domain, t.Category}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runTemplateShow는 에이전트 템플릿 상세 정보를 출력합니다.
func runTemplateShow(client *apiclient.Client, out io.Writer, templateID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(templateID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	tmpl, err := apiclient.Do[AgentTemplate](client, ctx, "GET",
		"/api/v1/agent-templates/"+templateID, nil)
	if err != nil {
		return fmt.Errorf("템플릿 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, tmpl)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: tmpl.ID},
		{Key: "Name", Value: tmpl.Name},
		{Key: "Domain", Value: tmpl.Domain},
		{Key: "Category", Value: tmpl.Category},
		{Key: "Description", Value: tmpl.Description},
		{Key: "CreatedAt", Value: tmpl.CreatedAt},
	})
	return nil
}

// runTemplateDomains는 템플릿 도메인 목록을 출력합니다.
func runTemplateDomains(client *apiclient.Client, out io.Writer) error {
	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	domains, err := apiclient.DoList[TemplateDomain](client, ctx, "GET",
		"/api/v1/agent-templates/domains", nil)
	if err != nil {
		return fmt.Errorf("템플릿 도메인 목록 조회 실패: %w", err)
	}

	headers := []string{"NAME", "COUNT"}
	rows := make([][]string, len(domains))
	for i, d := range domains {
		rows[i] = []string{d.Name, fmt.Sprintf("%d", d.Count)}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runTemplateCategories는 템플릿 카테고리 목록을 출력합니다.
func runTemplateCategories(client *apiclient.Client, out io.Writer) error {
	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	categories, err := apiclient.DoList[TemplateCategory](client, ctx, "GET",
		"/api/v1/agent-templates/categories", nil)
	if err != nil {
		return fmt.Errorf("템플릿 카테고리 목록 조회 실패: %w", err)
	}

	headers := []string{"NAME", "COUNT"}
	rows := make([][]string, len(categories))
	for i, c := range categories {
		rows[i] = []string{c.Name, fmt.Sprintf("%d", c.Count)}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runTemplateDeploy는 에이전트 템플릿을 배포합니다.
func runTemplateDeploy(client *apiclient.Client, out io.Writer, templateID string) error {
	if err := apiclient.ValidateID(templateID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	result, err := apiclient.Do[map[string]interface{}](client, ctx, "POST",
		"/api/v1/agent-templates/"+templateID+"/deploy", nil)
	if err != nil {
		return fmt.Errorf("템플릿 배포 실패: %w", err)
	}

	return apiclient.PrintJSON(out, result)
}
