// attachment.go는 첨부파일 관련 CLI 명령어를 구현합니다.
// attachment list/upload/show/download/delete 서브커맨드
package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/spf13/cobra"
)

// Attachment는 첨부파일 기본 정보를 나타냅니다.
type Attachment struct {
	ID        string `json:"id"`
	Filename  string `json:"filename"`
	Size      int64  `json:"size,omitempty"`
	MimeType  string `json:"mime_type,omitempty"`
	IssueID   string `json:"issue_id,omitempty"`
	URL       string `json:"url,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

var (
	attachmentJSONOutput bool
	attachmentOutput     string
)

// attachmentCmd는 attachment 서브커맨드의 루트입니다.
var attachmentCmd = &cobra.Command{
	Use:   "attachment",
	Short: "첨부파일 관련 명령어",
	Long:  `첨부파일 목록 조회, 업로드, 상세 조회, 다운로드, 삭제 기능을 제공합니다.`,
}

// attachmentListCmd는 이슈의 첨부파일 목록을 조회합니다.
var attachmentListCmd = &cobra.Command{
	Use:   "list <issue-id>",
	Short: "첨부파일 목록 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runAttachmentList(client, os.Stdout, args[0], json)
	},
}

// attachmentUploadCmd는 이슈에 파일을 업로드합니다.
var attachmentUploadCmd = &cobra.Command{
	Use:   "upload <issue-id> <file-path>",
	Short: "파일 업로드",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runAttachmentUpload(client, os.Stdout, args[0], args[1], json)
	},
}

// attachmentShowCmd는 첨부파일 상세 정보를 조회합니다.
var attachmentShowCmd = &cobra.Command{
	Use:   "show <attachment-id>",
	Short: "첨부파일 상세 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runAttachmentShow(client, os.Stdout, args[0], json)
	},
}

// attachmentDownloadCmd는 첨부파일을 다운로드합니다.
var attachmentDownloadCmd = &cobra.Command{
	Use:   "download <attachment-id>",
	Short: "첨부파일 다운로드",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		output, _ := cmd.Flags().GetString("output")
		return runAttachmentDownload(client, os.Stdout, args[0], output)
	},
}

// attachmentDeleteCmd는 첨부파일을 삭제합니다.
var attachmentDeleteCmd = &cobra.Command{
	Use:   "delete <attachment-id>",
	Short: "첨부파일 삭제",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runAttachmentDelete(client, os.Stdout, args[0])
	},
}

func init() {
	rootCmd.AddCommand(attachmentCmd)
	attachmentCmd.AddCommand(attachmentListCmd)
	attachmentCmd.AddCommand(attachmentUploadCmd)
	attachmentCmd.AddCommand(attachmentShowCmd)
	attachmentCmd.AddCommand(attachmentDownloadCmd)
	attachmentCmd.AddCommand(attachmentDeleteCmd)

	// --json 플래그
	for _, sub := range []*cobra.Command{attachmentListCmd, attachmentUploadCmd, attachmentShowCmd} {
		sub.Flags().BoolVar(&attachmentJSONOutput, "json", false, "JSON 형식으로 출력")
	}

	// download 전용 --output 플래그
	attachmentDownloadCmd.Flags().StringVar(&attachmentOutput, "output", "", "저장할 파일 경로")
}

// runAttachmentList는 이슈의 첨부파일 목록을 조회하고 출력합니다.
func runAttachmentList(client *apiclient.Client, out io.Writer, issueID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(issueID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	attachments, err := apiclient.DoList[Attachment](client, ctx, "GET",
		"/api/v1/issues/"+issueID+"/attachments", nil)
	if err != nil {
		return fmt.Errorf("첨부파일 목록 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, attachments)
	}

	headers := []string{"ID", "FILENAME", "SIZE", "MIME_TYPE"}
	rows := make([][]string, len(attachments))
	for i, att := range attachments {
		shortID := att.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		rows[i] = []string{shortID, att.Filename, fmt.Sprintf("%d", att.Size), att.MimeType}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runAttachmentUpload는 이슈에 파일을 업로드합니다.
// apiclient.DoUpload를 사용하여 multipart/form-data 방식으로 업로드합니다.
func runAttachmentUpload(client *apiclient.Client, out io.Writer, issueID, filePath string, jsonOutput bool) error {
	if err := apiclient.ValidateID(issueID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(30 * apiclient.SecondDuration)
	defer cancel()

	rawData, err := apiclient.DoUpload(client, ctx,
		"/api/v1/issues/"+issueID+"/attachments", filePath, nil)
	if err != nil {
		return fmt.Errorf("파일 업로드 실패: %w", err)
	}

	if jsonOutput {
		fmt.Fprintf(out, "%s\n", string(*rawData))
		return nil
	}

	// 업로드 결과를 Attachment 타입으로 파싱 시도
	var att Attachment
	if parseErr := json.Unmarshal(*rawData, &att); parseErr == nil && att.ID != "" {
		apiclient.PrintDetail(out, []apiclient.KeyValue{
			{Key: "ID", Value: att.ID},
			{Key: "Filename", Value: att.Filename},
			{Key: "Size", Value: fmt.Sprintf("%d", att.Size)},
		})
	} else {
		fmt.Fprintf(out, "업로드 완료\n")
	}
	return nil
}

// runAttachmentShow는 첨부파일 상세 정보를 출력합니다.
func runAttachmentShow(client *apiclient.Client, out io.Writer, attachmentID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(attachmentID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	att, err := apiclient.Do[Attachment](client, ctx, "GET",
		"/api/v1/attachments/"+attachmentID, nil)
	if err != nil {
		return fmt.Errorf("첨부파일 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, att)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: att.ID},
		{Key: "Filename", Value: att.Filename},
		{Key: "Size", Value: fmt.Sprintf("%d", att.Size)},
		{Key: "MimeType", Value: att.MimeType},
		{Key: "IssueID", Value: att.IssueID},
		{Key: "URL", Value: att.URL},
		{Key: "CreatedAt", Value: att.CreatedAt},
	})
	return nil
}

// runAttachmentDownload는 첨부파일을 다운로드합니다.
// outputPath가 지정된 경우 파일로 저장하고, 아니면 stdout에 출력합니다.
func runAttachmentDownload(client *apiclient.Client, out io.Writer, attachmentID, outputPath string) error {
	if err := apiclient.ValidateID(attachmentID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(30 * apiclient.SecondDuration)
	defer cancel()

	statusCode, body, err := client.DoRaw(ctx, "GET",
		"/api/v1/attachments/"+attachmentID+"/download", nil, nil)
	if err != nil {
		return fmt.Errorf("첨부파일 다운로드 실패: %w", err)
	}
	if statusCode >= 400 {
		return fmt.Errorf("다운로드 API 오류 (HTTP %d): %s", statusCode, string(body))
	}

	if outputPath != "" {
		if err := os.WriteFile(outputPath, body, 0644); err != nil {
			return fmt.Errorf("파일 저장 실패: %w", err)
		}
		fmt.Fprintf(out, "다운로드 완료: %s (%d 바이트)\n", outputPath, len(body))
		return nil
	}

	// outputPath 미지정 시 stdout으로 출력
	_, err = out.Write(body)
	return err
}

// runAttachmentDelete는 첨부파일을 삭제합니다.
func runAttachmentDelete(client *apiclient.Client, out io.Writer, attachmentID string) error {
	if err := apiclient.ValidateID(attachmentID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	_, err := apiclient.Do[map[string]interface{}](client, ctx, "DELETE",
		"/api/v1/attachments/"+attachmentID, nil)
	if err != nil {
		return fmt.Errorf("첨부파일 삭제 실패: %w", err)
	}

	fmt.Fprintf(out, "첨부파일 삭제 완료: %s\n", attachmentID)
	return nil
}
