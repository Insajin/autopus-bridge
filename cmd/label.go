// label.go는 라벨 관련 CLI 명령어를 구현합니다.
// label list/create/update/delete/add/remove 서브커맨드
package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/spf13/cobra"
)

// Label은 라벨 기본 정보를 나타냅니다.
type Label struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Color     string `json:"color,omitempty"`
	ProjectID string `json:"project_id,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

var (
	labelJSONOutput bool
	labelName       string
	labelColor      string
)

// labelCmd는 label 서브커맨드의 루트입니다.
var labelCmd = &cobra.Command{
	Use:   "label",
	Short: "라벨 관련 명령어",
	Long:  `라벨 목록 조회, 생성, 수정, 삭제, 이슈 라벨 추가/제거 기능을 제공합니다.`,
}

// labelListCmd는 프로젝트의 라벨 목록을 조회합니다.
var labelListCmd = &cobra.Command{
	Use:   "list <project-id>",
	Short: "라벨 목록 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runLabelList(client, os.Stdout, args[0], json)
	},
}

// labelCreateCmd는 새 라벨을 생성합니다.
var labelCreateCmd = &cobra.Command{
	Use:   "create <project-id>",
	Short: "라벨 생성",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		name, _ := cmd.Flags().GetString("name")
		color, _ := cmd.Flags().GetString("color")
		client.SetJSONOutput(json)
		return runLabelCreate(client, os.Stdout, args[0], name, color, json)
	},
}

// labelUpdateCmd는 라벨을 수정합니다.
var labelUpdateCmd = &cobra.Command{
	Use:   "update <label-id>",
	Short: "라벨 수정",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		name, _ := cmd.Flags().GetString("name")
		color, _ := cmd.Flags().GetString("color")
		client.SetJSONOutput(json)
		return runLabelUpdate(client, os.Stdout, args[0], name, color, json)
	},
}

// labelDeleteCmd는 라벨을 삭제합니다.
var labelDeleteCmd = &cobra.Command{
	Use:   "delete <label-id>",
	Short: "라벨 삭제",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runLabelDelete(client, os.Stdout, args[0])
	},
}

// labelAddCmd는 이슈에 라벨을 추가합니다.
var labelAddCmd = &cobra.Command{
	Use:   "add <issue-id> <label-id>",
	Short: "이슈에 라벨 추가",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runLabelAdd(client, os.Stdout, args[0], args[1])
	},
}

// labelRemoveCmd는 이슈에서 라벨을 제거합니다.
var labelRemoveCmd = &cobra.Command{
	Use:   "remove <issue-id> <label-id>",
	Short: "이슈에서 라벨 제거",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runLabelRemove(client, os.Stdout, args[0], args[1])
	},
}

func init() {
	rootCmd.AddCommand(labelCmd)
	labelCmd.AddCommand(labelListCmd)
	labelCmd.AddCommand(labelCreateCmd)
	labelCmd.AddCommand(labelUpdateCmd)
	labelCmd.AddCommand(labelDeleteCmd)
	labelCmd.AddCommand(labelAddCmd)
	labelCmd.AddCommand(labelRemoveCmd)

	// --json 플래그
	for _, sub := range []*cobra.Command{labelListCmd, labelCreateCmd, labelUpdateCmd} {
		sub.Flags().BoolVar(&labelJSONOutput, "json", false, "JSON 형식으로 출력")
	}

	// create/update 전용 플래그
	labelCreateCmd.Flags().StringVar(&labelName, "name", "", "라벨 이름 (필수)")
	labelCreateCmd.Flags().StringVar(&labelColor, "color", "", "라벨 색상 (예: #ff0000)")
	labelUpdateCmd.Flags().StringVar(&labelName, "name", "", "라벨 이름")
	labelUpdateCmd.Flags().StringVar(&labelColor, "color", "", "라벨 색상")
}

// runLabelList는 프로젝트의 라벨 목록을 조회하고 출력합니다.
func runLabelList(client *apiclient.Client, out io.Writer, projectID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(projectID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	labels, err := apiclient.DoList[Label](client, ctx, "GET",
		"/api/v1/projects/"+projectID+"/labels", nil)
	if err != nil {
		return fmt.Errorf("라벨 목록 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, labels)
	}

	headers := []string{"ID", "NAME", "COLOR"}
	rows := make([][]string, len(labels))
	for i, lbl := range labels {
		shortID := lbl.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		rows[i] = []string{shortID, lbl.Name, lbl.Color}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runLabelCreate는 새 라벨을 생성합니다.
func runLabelCreate(client *apiclient.Client, out io.Writer, projectID, name, color string, jsonOutput bool) error {
	if err := apiclient.ValidateID(projectID); err != nil {
		return err
	}

	body := map[string]string{"name": name, "color": color}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	lbl, err := apiclient.Do[Label](client, ctx, "POST",
		"/api/v1/projects/"+projectID+"/labels", body)
	if err != nil {
		return fmt.Errorf("라벨 생성 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, lbl)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: lbl.ID},
		{Key: "Name", Value: lbl.Name},
		{Key: "Color", Value: lbl.Color},
	})
	return nil
}

// runLabelUpdate는 라벨을 수정합니다.
func runLabelUpdate(client *apiclient.Client, out io.Writer, labelID, name, color string, jsonOutput bool) error {
	if err := apiclient.ValidateID(labelID); err != nil {
		return err
	}

	body := map[string]string{}
	if name != "" {
		body["name"] = name
	}
	if color != "" {
		body["color"] = color
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	lbl, err := apiclient.Do[Label](client, ctx, "PATCH",
		"/api/v1/labels/"+labelID, body)
	if err != nil {
		return fmt.Errorf("라벨 수정 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, lbl)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: lbl.ID},
		{Key: "Name", Value: lbl.Name},
		{Key: "Color", Value: lbl.Color},
	})
	return nil
}

// runLabelDelete는 라벨을 삭제합니다.
func runLabelDelete(client *apiclient.Client, out io.Writer, labelID string) error {
	if err := apiclient.ValidateID(labelID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	_, err := apiclient.Do[map[string]interface{}](client, ctx, "DELETE",
		"/api/v1/labels/"+labelID, nil)
	if err != nil {
		return fmt.Errorf("라벨 삭제 실패: %w", err)
	}

	fmt.Fprintf(out, "라벨 삭제 완료: %s\n", labelID)
	return nil
}

// runLabelAdd는 이슈에 라벨을 추가합니다.
func runLabelAdd(client *apiclient.Client, out io.Writer, issueID, labelID string) error {
	if err := apiclient.ValidateID(issueID); err != nil {
		return err
	}
	if err := apiclient.ValidateID(labelID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	_, err := apiclient.Do[map[string]interface{}](client, ctx, "POST",
		"/api/v1/issues/"+issueID+"/labels/"+labelID, nil)
	if err != nil {
		return fmt.Errorf("라벨 추가 실패: %w", err)
	}

	fmt.Fprintf(out, "라벨 추가 완료\n")
	return nil
}

// runLabelRemove는 이슈에서 라벨을 제거합니다.
func runLabelRemove(client *apiclient.Client, out io.Writer, issueID, labelID string) error {
	if err := apiclient.ValidateID(issueID); err != nil {
		return err
	}
	if err := apiclient.ValidateID(labelID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	_, err := apiclient.Do[map[string]interface{}](client, ctx, "DELETE",
		"/api/v1/issues/"+issueID+"/labels/"+labelID, nil)
	if err != nil {
		return fmt.Errorf("라벨 제거 실패: %w", err)
	}

	fmt.Fprintf(out, "라벨 제거 완료\n")
	return nil
}
