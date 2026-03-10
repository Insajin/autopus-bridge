// pipeline.go는 배포 파이프라인 관련 CLI 명령어를 구현합니다.
// pipeline list/show/events/retry/cancel/history 서브커맨드
package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/spf13/cobra"
)

// Pipeline은 배포 파이프라인 정보를 나타냅니다.
type Pipeline struct {
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	Status    string `json:"status,omitempty"`
	Stage     string `json:"stage,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

// PipelineEvent는 파이프라인 이벤트 정보를 나타냅니다.
type PipelineEvent struct {
	ID         string `json:"id"`
	PipelineID string `json:"pipeline_id"`
	EventType  string `json:"event_type,omitempty"`
	Message    string `json:"message,omitempty"`
	CreatedAt  string `json:"created_at,omitempty"`
}

// DeploymentHistory는 배포 이력 정보를 나타냅니다.
type DeploymentHistory struct {
	ID          string `json:"id"`
	PipelineID  string `json:"pipeline_id,omitempty"`
	Environment string `json:"environment,omitempty"`
	Status      string `json:"status,omitempty"`
	DeployedAt  string `json:"deployed_at,omitempty"`
}

var pipelineJSONOutput bool

// pipelineCmd는 pipeline 서브커맨드의 루트입니다.
var pipelineCmd = &cobra.Command{
	Use:   "pipeline",
	Short: "배포 파이프라인 관련 명령어",
	Long:  `배포 파이프라인 목록 조회, 상세 조회, 이벤트, 재시도, 취소, 이력 조회 기능을 제공합니다.`,
}

// pipelineListCmd는 파이프라인 목록을 조회합니다.
var pipelineListCmd = &cobra.Command{
	Use:   "list",
	Short: "파이프라인 목록 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runPipelineList(client, os.Stdout, json)
	},
}

// pipelineShowCmd는 파이프라인 상세 정보를 조회합니다.
var pipelineShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "파이프라인 상세 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runPipelineShow(client, os.Stdout, args[0], json)
	},
}

// pipelineEventsCmd는 파이프라인 이벤트 목록을 조회합니다.
var pipelineEventsCmd = &cobra.Command{
	Use:   "events <id>",
	Short: "파이프라인 이벤트 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runPipelineEvents(client, os.Stdout, args[0], json)
	},
}

// pipelineRetryCmd는 파이프라인을 재시도합니다.
var pipelineRetryCmd = &cobra.Command{
	Use:   "retry <id>",
	Short: "파이프라인 재시도",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runPipelineRetry(client, os.Stdout, args[0], json)
	},
}

// pipelineCancelCmd는 파이프라인을 취소합니다.
var pipelineCancelCmd = &cobra.Command{
	Use:   "cancel <id>",
	Short: "파이프라인 취소",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runPipelineCancel(client, os.Stdout, args[0], json)
	},
}

// pipelineHistoryCmd는 배포 이력을 조회합니다.
var pipelineHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: "배포 이력 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runPipelineHistory(client, os.Stdout, json)
	},
}

func init() {
	rootCmd.AddCommand(pipelineCmd)
	pipelineCmd.AddCommand(pipelineListCmd)
	pipelineCmd.AddCommand(pipelineShowCmd)
	pipelineCmd.AddCommand(pipelineEventsCmd)
	pipelineCmd.AddCommand(pipelineRetryCmd)
	pipelineCmd.AddCommand(pipelineCancelCmd)
	pipelineCmd.AddCommand(pipelineHistoryCmd)

	// --json 플래그를 서브커맨드에 추가
	for _, sub := range []*cobra.Command{
		pipelineListCmd,
		pipelineShowCmd,
		pipelineEventsCmd,
		pipelineRetryCmd,
		pipelineCancelCmd,
		pipelineHistoryCmd,
	} {
		sub.Flags().BoolVar(&pipelineJSONOutput, "json", false, "JSON 형식으로 출력")
	}
}

// runPipelineList는 파이프라인 목록을 조회하고 출력합니다.
func runPipelineList(client *apiclient.Client, out io.Writer, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	pipelines, err := apiclient.DoList[Pipeline](client, ctx, "GET",
		"/api/v1/workspaces/"+workspaceID+"/deployment-pipelines", nil)
	if err != nil {
		return fmt.Errorf("파이프라인 목록 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, pipelines)
	}

	headers := []string{"ID", "NAME", "STATUS", "STAGE"}
	rows := make([][]string, len(pipelines))
	for i, p := range pipelines {
		// ID는 첫 8자만 표시
		shortID := p.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		rows[i] = []string{shortID, p.Name, p.Status, p.Stage}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runPipelineShow는 파이프라인 상세 정보를 조회하고 출력합니다.
func runPipelineShow(client *apiclient.Client, out io.Writer, pipelineID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(pipelineID); err != nil {
		return err
	}

	workspaceID := client.WorkspaceID()

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	pipeline, err := apiclient.Do[Pipeline](client, ctx, "GET",
		"/api/v1/workspaces/"+workspaceID+"/deployment-pipelines/"+pipelineID, nil)
	if err != nil {
		return fmt.Errorf("파이프라인 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, pipeline)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: pipeline.ID},
		{Key: "Name", Value: pipeline.Name},
		{Key: "Status", Value: pipeline.Status},
		{Key: "Stage", Value: pipeline.Stage},
		{Key: "CreatedAt", Value: pipeline.CreatedAt},
	})
	return nil
}

// runPipelineEvents는 파이프라인 이벤트 목록을 조회하고 출력합니다.
func runPipelineEvents(client *apiclient.Client, out io.Writer, pipelineID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(pipelineID); err != nil {
		return err
	}

	workspaceID := client.WorkspaceID()

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	events, err := apiclient.DoList[PipelineEvent](client, ctx, "GET",
		"/api/v1/workspaces/"+workspaceID+"/deployment-pipelines/"+pipelineID+"/events", nil)
	if err != nil {
		return fmt.Errorf("파이프라인 이벤트 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, events)
	}

	headers := []string{"ID", "EVENT_TYPE", "MESSAGE", "CREATED_AT"}
	rows := make([][]string, len(events))
	for i, e := range events {
		// ID는 첫 8자만 표시
		shortID := e.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		rows[i] = []string{shortID, e.EventType, e.Message, e.CreatedAt}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runPipelineRetry는 파이프라인을 재시도하고 결과를 출력합니다.
func runPipelineRetry(client *apiclient.Client, out io.Writer, pipelineID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(pipelineID); err != nil {
		return err
	}

	workspaceID := client.WorkspaceID()

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	pipeline, err := apiclient.Do[Pipeline](client, ctx, "POST",
		"/api/v1/workspaces/"+workspaceID+"/deployment-pipelines/"+pipelineID+"/retry", nil)
	if err != nil {
		return fmt.Errorf("파이프라인 재시도 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, pipeline)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: pipeline.ID},
		{Key: "Name", Value: pipeline.Name},
		{Key: "Status", Value: pipeline.Status},
		{Key: "Stage", Value: pipeline.Stage},
	})
	return nil
}

// runPipelineCancel는 파이프라인을 취소하고 결과를 출력합니다.
func runPipelineCancel(client *apiclient.Client, out io.Writer, pipelineID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(pipelineID); err != nil {
		return err
	}

	workspaceID := client.WorkspaceID()

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	pipeline, err := apiclient.Do[Pipeline](client, ctx, "POST",
		"/api/v1/workspaces/"+workspaceID+"/deployment-pipelines/"+pipelineID+"/cancel", nil)
	if err != nil {
		return fmt.Errorf("파이프라인 취소 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, pipeline)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: pipeline.ID},
		{Key: "Name", Value: pipeline.Name},
		{Key: "Status", Value: pipeline.Status},
		{Key: "Stage", Value: pipeline.Stage},
	})
	return nil
}

// runPipelineHistory는 배포 이력을 조회하고 출력합니다.
func runPipelineHistory(client *apiclient.Client, out io.Writer, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	history, err := apiclient.DoList[DeploymentHistory](client, ctx, "GET",
		"/api/v1/workspaces/"+workspaceID+"/deployment-history", nil)
	if err != nil {
		return fmt.Errorf("배포 이력 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, history)
	}

	headers := []string{"ID", "PIPELINE_ID", "ENVIRONMENT", "STATUS", "DEPLOYED_AT"}
	rows := make([][]string, len(history))
	for i, h := range history {
		// ID는 첫 8자만 표시
		shortID := h.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		// PipelineID도 첫 8자만 표시
		shortPipeID := h.PipelineID
		if len(shortPipeID) > 8 {
			shortPipeID = shortPipeID[:8]
		}
		rows[i] = []string{shortID, shortPipeID, h.Environment, h.Status, h.DeployedAt}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}
