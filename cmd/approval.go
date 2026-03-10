// approval.go는 approval 관련 CLI 명령어를 구현합니다.
// approval list/show/approve/reject 서브커맨드
package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/spf13/cobra"
)

// Approval은 승인 요청 기본 정보를 나타냅니다.
type Approval struct {
	ID          string `json:"id"`
	Title       string `json:"title,omitempty"`
	Status      string `json:"status,omitempty"`
	RequestedBy string `json:"requested_by,omitempty"`
	ApprovedBy  string `json:"approved_by,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

var approvalJSONOutput bool

// approvalCmd는 approval 서브커맨드의 루트입니다.
var approvalCmd = &cobra.Command{
	Use:   "approval",
	Short: "승인 요청 관련 명령어",
	Long:  `승인 요청 목록 조회, 상세 조회, 승인, 거부 기능을 제공합니다.`,
}

// approvalListCmd는 프로젝트의 승인 요청 목록을 조회합니다.
var approvalListCmd = &cobra.Command{
	Use:   "list <project-id>",
	Short: "승인 요청 목록 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(jsonOut)
		return runApprovalList(client, os.Stdout, args[0], jsonOut)
	},
}

// approvalShowCmd는 승인 요청 상세 정보를 조회합니다.
var approvalShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "승인 요청 상세 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(jsonOut)
		return runApprovalShow(client, os.Stdout, args[0], jsonOut)
	},
}

// approvalApproveCmd는 승인 요청을 승인합니다.
var approvalApproveCmd = &cobra.Command{
	Use:   "approve <id>",
	Short: "승인 요청 승인",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runApprovalApprove(client, os.Stdout, args[0])
	},
}

// approvalRejectCmd는 승인 요청을 거부합니다.
var approvalRejectCmd = &cobra.Command{
	Use:   "reject <id>",
	Short: "승인 요청 거부",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runApprovalReject(client, os.Stdout, args[0])
	},
}

func init() {
	rootCmd.AddCommand(approvalCmd)
	approvalCmd.AddCommand(approvalListCmd)
	approvalCmd.AddCommand(approvalShowCmd)
	approvalCmd.AddCommand(approvalApproveCmd)
	approvalCmd.AddCommand(approvalRejectCmd)

	// --json 플래그
	for _, sub := range []*cobra.Command{approvalListCmd, approvalShowCmd} {
		sub.Flags().BoolVar(&approvalJSONOutput, "json", false, "JSON 형식으로 출력")
	}
}

// runApprovalList는 프로젝트의 승인 요청 목록을 조회하고 출력합니다.
func runApprovalList(client *apiclient.Client, out io.Writer, projectID string, jsonOutput bool) error {
	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	approvals, err := apiclient.DoList[Approval](client, ctx, "GET",
		"/api/v1/projects/"+projectID+"/approvals", nil)
	if err != nil {
		return fmt.Errorf("승인 요청 목록 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, approvals)
	}

	headers := []string{"ID", "TITLE", "STATUS", "REQUESTED_BY"}
	rows := make([][]string, len(approvals))
	for i, a := range approvals {
		shortID := a.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		rows[i] = []string{shortID, a.Title, a.Status, a.RequestedBy}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runApprovalShow는 승인 요청 상세 정보를 출력합니다.
func runApprovalShow(client *apiclient.Client, out io.Writer, approvalID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(approvalID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	a, err := apiclient.Do[Approval](client, ctx, "GET", "/api/v1/approvals/"+approvalID, nil)
	if err != nil {
		return fmt.Errorf("승인 요청 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, a)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: a.ID},
		{Key: "Title", Value: a.Title},
		{Key: "Status", Value: a.Status},
		{Key: "RequestedBy", Value: a.RequestedBy},
		{Key: "ApprovedBy", Value: a.ApprovedBy},
		{Key: "CreatedAt", Value: a.CreatedAt},
	})
	return nil
}

// runApprovalApprove는 승인 요청을 승인합니다.
func runApprovalApprove(client *apiclient.Client, out io.Writer, approvalID string) error {
	if err := apiclient.ValidateID(approvalID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	a, err := apiclient.Do[Approval](client, ctx, "POST", "/api/v1/approvals/"+approvalID+"/approve", nil)
	if err != nil {
		return fmt.Errorf("승인 요청 승인 실패: %w", err)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: a.ID},
		{Key: "Title", Value: a.Title},
		{Key: "Status", Value: a.Status},
	})
	return nil
}

// runApprovalReject는 승인 요청을 거부합니다.
func runApprovalReject(client *apiclient.Client, out io.Writer, approvalID string) error {
	if err := apiclient.ValidateID(approvalID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	a, err := apiclient.Do[Approval](client, ctx, "POST", "/api/v1/approvals/"+approvalID+"/reject", nil)
	if err != nil {
		return fmt.Errorf("승인 요청 거부 실패: %w", err)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: a.ID},
		{Key: "Title", Value: a.Title},
		{Key: "Status", Value: a.Status},
	})
	return nil
}
