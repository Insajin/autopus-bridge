// anomaly.go는 이상 탐지 관련 CLI 명령어를 구현합니다.
// anomaly list/detect/acknowledge/resolve 서브커맨드
package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/spf13/cobra"
)

// Anomaly는 이상 탐지 정보를 나타냅니다.
type Anomaly struct {
	ID          string `json:"id"`
	Type        string `json:"type,omitempty"`
	Severity    string `json:"severity,omitempty"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status,omitempty"`
	DetectedAt  string `json:"detected_at,omitempty"`
}

var anomalyJSONOutput bool

// anomalyCmd는 anomaly 서브커맨드의 루트입니다.
var anomalyCmd = &cobra.Command{
	Use:   "anomaly",
	Short: "이상 탐지 관련 명령어",
	Long:  `이상 탐지 목록 조회, 탐지 실행, 확인, 해결 기능을 제공합니다.`,
}

// anomalyListCmd는 이상 탐지 목록을 조회합니다.
var anomalyListCmd = &cobra.Command{
	Use:   "list",
	Short: "이상 탐지 목록 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runAnomalyList(client, os.Stdout, json)
	},
}

// anomalyDetectCmd는 이상 탐지를 실행합니다.
var anomalyDetectCmd = &cobra.Command{
	Use:   "detect",
	Short: "이상 탐지 실행",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runAnomalyDetect(client, os.Stdout, json)
	},
}

// anomalyAcknowledgeCmd는 이상 탐지를 확인 처리합니다.
var anomalyAcknowledgeCmd = &cobra.Command{
	Use:   "acknowledge <id>",
	Short: "이상 탐지 확인",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runAnomalyAcknowledge(client, os.Stdout, args[0], json)
	},
}

// anomalyResolveCmd는 이상 탐지를 해결 처리합니다.
var anomalyResolveCmd = &cobra.Command{
	Use:   "resolve <id>",
	Short: "이상 탐지 해결",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runAnomalyResolve(client, os.Stdout, args[0], json)
	},
}

func init() {
	rootCmd.AddCommand(anomalyCmd)
	anomalyCmd.AddCommand(anomalyListCmd)
	anomalyCmd.AddCommand(anomalyDetectCmd)
	anomalyCmd.AddCommand(anomalyAcknowledgeCmd)
	anomalyCmd.AddCommand(anomalyResolveCmd)

	// --json 플래그를 서브커맨드에 추가
	for _, sub := range []*cobra.Command{anomalyListCmd, anomalyDetectCmd, anomalyAcknowledgeCmd, anomalyResolveCmd} {
		sub.Flags().BoolVar(&anomalyJSONOutput, "json", false, "JSON 형식으로 출력")
	}
}

// runAnomalyList는 이상 탐지 목록을 조회하고 출력합니다.
func runAnomalyList(client *apiclient.Client, out io.Writer, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	anomalies, err := apiclient.DoList[Anomaly](client, ctx, "GET",
		"/api/v1/workspaces/"+workspaceID+"/anomalies", nil)
	if err != nil {
		return fmt.Errorf("이상 탐지 목록 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, anomalies)
	}

	headers := []string{"ID", "TYPE", "SEVERITY", "STATUS", "DETECTED_AT"}
	rows := make([][]string, len(anomalies))
	for i, a := range anomalies {
		// ID는 첫 8자만 표시
		shortID := a.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		rows[i] = []string{shortID, a.Type, a.Severity, a.Status, a.DetectedAt}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runAnomalyDetect는 이상 탐지를 실행하고 결과를 출력합니다.
func runAnomalyDetect(client *apiclient.Client, out io.Writer, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	anomalies, err := apiclient.DoList[Anomaly](client, ctx, "POST",
		"/api/v1/workspaces/"+workspaceID+"/anomalies", nil)
	if err != nil {
		return fmt.Errorf("이상 탐지 실행 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, anomalies)
	}

	fmt.Fprintf(out, "이상 탐지 완료: %d건 발견\n", len(anomalies))
	if len(anomalies) > 0 {
		headers := []string{"ID", "TYPE", "SEVERITY", "STATUS", "DETECTED_AT"}
		rows := make([][]string, len(anomalies))
		for i, a := range anomalies {
			shortID := a.ID
			if len(shortID) > 8 {
				shortID = shortID[:8]
			}
			rows[i] = []string{shortID, a.Type, a.Severity, a.Status, a.DetectedAt}
		}
		apiclient.PrintTable(out, headers, rows)
	}
	return nil
}

// runAnomalyAcknowledge는 이상 탐지를 확인 처리하고 결과를 출력합니다.
func runAnomalyAcknowledge(client *apiclient.Client, out io.Writer, anomalyID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(anomalyID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	anomaly, err := apiclient.Do[Anomaly](client, ctx, "POST",
		"/api/v1/anomalies/"+anomalyID+"/acknowledge", nil)
	if err != nil {
		return fmt.Errorf("이상 탐지 확인 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, anomaly)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: anomaly.ID},
		{Key: "Type", Value: anomaly.Type},
		{Key: "Severity", Value: anomaly.Severity},
		{Key: "Status", Value: anomaly.Status},
		{Key: "DetectedAt", Value: anomaly.DetectedAt},
	})
	return nil
}

// runAnomalyResolve는 이상 탐지를 해결 처리하고 결과를 출력합니다.
func runAnomalyResolve(client *apiclient.Client, out io.Writer, anomalyID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(anomalyID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	anomaly, err := apiclient.Do[Anomaly](client, ctx, "POST",
		"/api/v1/anomalies/"+anomalyID+"/resolve", nil)
	if err != nil {
		return fmt.Errorf("이상 탐지 해결 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, anomaly)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: anomaly.ID},
		{Key: "Type", Value: anomaly.Type},
		{Key: "Severity", Value: anomaly.Severity},
		{Key: "Status", Value: anomaly.Status},
		{Key: "DetectedAt", Value: anomaly.DetectedAt},
	})
	return nil
}
