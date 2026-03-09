// message.go는 메시지 관련 CLI 명령어를 구현합니다.
// message list/send/thread/agent-messages 서브커맨드
package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/spf13/cobra"
)

// Message는 메시지 기본 정보를 나타냅니다.
type Message struct {
	ID          string `json:"id"`
	Content     string `json:"content"`
	UserID      string `json:"user_id,omitempty"`
	Type        string `json:"type,omitempty"`
	IsEdited    bool   `json:"is_edited"`
	CreatedAt   string `json:"created_at,omitempty"`
	ThreadCount int    `json:"thread_count,omitempty"`
}

// MessageWithUser는 작성자 표시 이름을 포함한 메시지 정보입니다.
type MessageWithUser struct {
	Message
	UserDisplayName string `json:"user_display_name,omitempty"`
}

// messageListResponse는 메시지 목록 API의 중첩 응답 구조입니다.
// 백엔드가 {data: {messages: [], has_more, first_unread_id}} 형태로 응답합니다.
type messageListResponse struct {
	Messages      []MessageWithUser `json:"messages"`
	HasMore       bool              `json:"has_more"`
	FirstUnreadID *string           `json:"first_unread_id"`
}

var (
	messageJSONOutput bool
	messageLimit      int
	messageBefore     string
)

// messageCmd는 message 서브커맨드의 루트입니다.
var messageCmd = &cobra.Command{
	Use:   "message",
	Short: "메시지 관련 명령어",
	Long:  `채널 메시지 조회, 전송, 스레드 조회, 에이전트 메시지 조회 기능을 제공합니다.`,
}

// messageListCmd는 채널 메시지 목록을 조회합니다.
var messageListCmd = &cobra.Command{
	Use:   "list <channel-id>",
	Short: "채널 메시지 목록 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		limit, _ := cmd.Flags().GetInt("limit")
		before, _ := cmd.Flags().GetString("before")
		client.SetJSONOutput(json)
		return runMessageList(client, os.Stdout, args[0], limit, before, json)
	},
}

// messageSendCmd는 채널에 메시지를 전송합니다.
var messageSendCmd = &cobra.Command{
	Use:   "send <channel-id> <content>",
	Short: "채널에 메시지 전송",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runMessageSend(client, os.Stdout, args[0], args[1])
	},
}

// messageThreadCmd는 메시지 스레드를 조회합니다.
var messageThreadCmd = &cobra.Command{
	Use:   "thread <message-id>",
	Short: "메시지 스레드 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runMessageThread(client, os.Stdout, args[0], json)
	},
}

// messageAgentMessagesCmd는 채널의 에이전트 메시지를 조회합니다.
var messageAgentMessagesCmd = &cobra.Command{
	Use:   "agent-messages <channel-id>",
	Short: "채널 에이전트 메시지 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runMessageAgentMessages(client, os.Stdout, args[0], json)
	},
}

func init() {
	rootCmd.AddCommand(messageCmd)
	messageCmd.AddCommand(messageListCmd)
	messageCmd.AddCommand(messageSendCmd)
	messageCmd.AddCommand(messageThreadCmd)
	messageCmd.AddCommand(messageAgentMessagesCmd)

	// --json 플래그를 서브커맨드에 추가
	for _, sub := range []*cobra.Command{messageListCmd, messageThreadCmd, messageAgentMessagesCmd} {
		sub.Flags().BoolVar(&messageJSONOutput, "json", false, "JSON 형식으로 출력")
	}

	// list 전용 커서 페이지네이션 플래그
	messageListCmd.Flags().IntVar(&messageLimit, "limit", 0, "조회할 메시지 수")
	messageListCmd.Flags().StringVar(&messageBefore, "before", "", "이 메시지 ID 이전 메시지 조회 (커서 페이지네이션)")
}

// truncateContent는 콘텐츠를 최대 80자(rune 기준)로 잘라 반환합니다.
// 80자 초과 시 뒤에 "..."을 붙입니다.
func truncateContent(content string) string {
	runes := []rune(content)
	if len(runes) > 80 {
		return string(runes[:80]) + "..."
	}
	return content
}

// buildMessagePath는 쿼리 파라미터가 포함된 메시지 목록 경로를 생성합니다.
func buildMessagePath(channelID string, limit int, before string) string {
	path := "/api/v1/channels/" + channelID + "/messages"
	params := ""

	if limit > 0 {
		params += fmt.Sprintf("limit=%d", limit)
	}
	if before != "" {
		if params != "" {
			params += "&"
		}
		params += "before=" + before
	}

	if params != "" {
		path += "?" + params
	}
	return path
}

// runMessageList는 채널 메시지 목록을 조회하고 출력합니다.
func runMessageList(client *apiclient.Client, out io.Writer, channelID string, limit int, before string, jsonOutput bool) error {
	path := buildMessagePath(channelID, limit, before)

	resp, err := apiclient.Do[messageListResponse](client, context.Background(), "GET", path, nil)
	if err != nil {
		return fmt.Errorf("메시지 목록 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, resp.Messages)
	}

	printMessageTable(out, resp.Messages)
	return nil
}

// runMessageSend는 채널에 메시지를 전송합니다.
func runMessageSend(client *apiclient.Client, out io.Writer, channelID, content string) error {
	body := map[string]string{"content": content}

	msg, err := apiclient.Do[Message](client, context.Background(), "POST",
		"/api/v1/channels/"+channelID+"/messages", body)
	if err != nil {
		return fmt.Errorf("메시지 전송 실패: %w", err)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: msg.ID},
		{Key: "Content", Value: truncateContent(msg.Content)},
		{Key: "CreatedAt", Value: msg.CreatedAt},
	})
	return nil
}

// runMessageThread는 메시지 스레드를 조회하고 출력합니다.
func runMessageThread(client *apiclient.Client, out io.Writer, messageID string, jsonOutput bool) error {
	resp, err := apiclient.Do[messageListResponse](client, context.Background(), "GET",
		"/api/v1/messages/"+messageID+"/thread", nil)
	if err != nil {
		return fmt.Errorf("메시지 스레드 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, resp.Messages)
	}

	printMessageTable(out, resp.Messages)
	return nil
}

// runMessageAgentMessages는 채널의 에이전트 메시지를 조회하고 출력합니다.
func runMessageAgentMessages(client *apiclient.Client, out io.Writer, channelID string, jsonOutput bool) error {
	resp, err := apiclient.Do[messageListResponse](client, context.Background(), "GET",
		"/api/v1/channels/"+channelID+"/agent-messages", nil)
	if err != nil {
		return fmt.Errorf("에이전트 메시지 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, resp.Messages)
	}

	printMessageTable(out, resp.Messages)
	return nil
}

// printMessageTable은 메시지 목록을 테이블 형식으로 출력합니다.
// 컬럼: TIME, AUTHOR, CONTENT (80자 초과 시 잘림)
func printMessageTable(out io.Writer, messages []MessageWithUser) {
	headers := []string{"TIME", "AUTHOR", "CONTENT"}
	rows := make([][]string, len(messages))
	for i, msg := range messages {
		// 작성자: UserDisplayName 우선, 없으면 UserID 사용
		author := msg.UserDisplayName
		if author == "" {
			author = msg.UserID
		}
		rows[i] = []string{
			msg.CreatedAt,
			author,
			truncateContent(msg.Content),
		}
	}
	apiclient.PrintTable(out, headers, rows)
}
