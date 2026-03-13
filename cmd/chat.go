// chat.go는 채팅 관련 CLI 명령어를 구현합니다.
// AC18-AC25: chat one-shot / REPL / watch / history 서브커맨드
package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/spf13/cobra"
)

// DMChannel은 DM 채널 정보를 나타냅니다.
type DMChannel struct {
	ID          string `json:"id"`
	AgentID     string `json:"agent_id,omitempty"`
	ChannelType string `json:"channel_type,omitempty"`
	Name        string `json:"name,omitempty"`
	Type        string `json:"type,omitempty"`
}

// ChatMessage는 채팅 메시지를 나타냅니다.
type ChatMessage struct {
	ID        string `json:"id"`
	Content   string `json:"content"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at,omitempty"`
}

var (
	chatAgentName string
	chatAgentID   string
	chatChannelID string
	chatWatch     bool
	chatRaw       bool
	chatLimit     int
)

// chatCmd는 chat 서브커맨드의 루트입니다.
var chatCmd = &cobra.Command{
	Use:   "chat [message]",
	Short: "에이전트와 채팅합니다",
	Long: `에이전트와 채팅합니다.

메시지를 인자로 전달하면 one-shot 모드로 실행합니다.
메시지 없이 실행하면 REPL (인터랙티브) 모드로 진입합니다.

예시:
  autopus chat "안녕하세요" --agent CTO
  autopus chat --agent CTO           # REPL 모드
  autopus chat --channel <id> --watch  # 채널 이벤트 감시`,
	RunE: runChat,
}

// chatHistoryCmd는 채팅 기록을 조회합니다.
var chatHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: "채팅 기록 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)

		channelID := chatChannelID
		if channelID == "" {
			// 에이전트 이름으로 DM 채널 조회
			if chatAgentName == "" {
				return fmt.Errorf("--agent 또는 --channel을 지정하세요")
			}
			agent, resolveErr := resolveAgentForChat(client, chatAgentName)
			if resolveErr != nil {
				return resolveErr
			}
			ch, dmErr := resolveDMChannel(client, agent)
			if dmErr != nil {
				return dmErr
			}
			channelID = ch.ID
		}

		limit, _ := cmd.Flags().GetInt("limit")
		return runChatHistory(client, os.Stdout, channelID, limit, json)
	},
}

func init() {
	rootCmd.AddCommand(chatCmd)
	chatCmd.AddCommand(chatHistoryCmd)

	// chat 명령 플래그
	chatCmd.Flags().StringVar(&chatAgentName, "agent", "", "대상 에이전트 이름")
	chatCmd.Flags().StringVar(&chatAgentID, "agent-id", "", "대상 에이전트 ID")
	chatCmd.Flags().StringVar(&chatChannelID, "channel", "", "대상 채널 ID")
	chatCmd.Flags().BoolVar(&chatWatch, "watch", false, "채널 이벤트 감시 모드")
	chatCmd.Flags().BoolVar(&chatRaw, "raw", false, "마크다운 없이 원본 텍스트 출력")

	// history 명령 플래그
	chatHistoryCmd.Flags().StringVar(&chatAgentName, "agent", "", "에이전트 이름으로 DM 채널 조회")
	chatHistoryCmd.Flags().StringVar(&chatChannelID, "channel", "", "채널 ID 직접 지정")
	chatHistoryCmd.Flags().IntVar(&chatLimit, "limit", 20, "최대 조회 건수")
	chatHistoryCmd.Flags().Bool("json", false, "JSON 형식으로 출력")
}

// runChat은 chat 명령의 실행 로직입니다.
func runChat(cmd *cobra.Command, args []string) error {
	client, err := newAPIClient()
	if err != nil {
		return err
	}

	// --channel --watch: 채널 이벤트 감시 모드
	if chatWatch && chatChannelID != "" {
		return runChatWatch(cmd.Context(), client, os.Stdout, chatChannelID)
	}

	// 에이전트 결정
	agent, err := resolveAgentForChat(client, resolveAgentNameForChat())
	if err != nil {
		return err
	}

	// DM 채널 조회/생성
	channel, err := resolveDMChannel(client, agent)
	if err != nil {
		return err
	}

	// 메시지가 인자로 주어진 경우: one-shot 모드
	if len(args) > 0 {
		message := strings.Join(args, " ")
		return runChatOneShot(cmd.Context(), client, os.Stdout, channel.ID, message)
	}

	// 메시지 없음: REPL 모드
	return runChatREPL(cmd.Context(), client, os.Stdin, os.Stdout, channel)
}

// resolveAgentNameForChat은 --agent 또는 --agent-id 플래그에서 에이전트 참조를 반환합니다.
func resolveAgentNameForChat() string {
	if chatAgentID != "" {
		return chatAgentID
	}
	return chatAgentName
}

// resolveAgentForChat은 에이전트 이름 또는 ID로 DashboardAgent를 찾습니다.
func resolveAgentForChat(client *apiclient.Client, agentRef string) (*DashboardAgent, error) {
	if agentRef == "" {
		return nil, fmt.Errorf("--agent 또는 --agent-id를 지정하세요")
	}

	workspaceID := client.WorkspaceID()
	agents, err := apiclient.DoList[DashboardAgent](client, context.Background(), "GET",
		"/api/v1/workspaces/"+workspaceID+"/dashboard/agents", nil)
	if err != nil {
		return nil, fmt.Errorf("에이전트 목록 조회 실패: %w", err)
	}

	// ID로 직접 매칭 시도
	for i, ag := range agents {
		if ag.ID == agentRef {
			return &agents[i], nil
		}
	}

	// 이름으로 검색
	return findDashboardAgentByName(agents, agentRef)
}

// resolveDMChannel은 에이전트의 DM 채널을 찾거나 생성합니다.
// CEO 에이전트: POST /workspaces/:id/dm-channels/ensure-ceo
// 그 외: GET /workspaces/:id/dm-channels 후 agentID로 필터링
func resolveDMChannel(client *apiclient.Client, agent *DashboardAgent) (*DMChannel, error) {
	workspaceID := client.WorkspaceID()

	// CEO 에이전트 확인
	if strings.ToUpper(agent.Name) == "CEO" {
		ch, err := apiclient.Do[DMChannel](client, context.Background(), "POST",
			"/api/v1/workspaces/"+workspaceID+"/dm-channels/ensure-ceo", map[string]string{})
		if err != nil {
			return nil, fmt.Errorf("CEO DM 채널 생성 실패: %w", err)
		}
		return ch, nil
	}

	// 일반 에이전트: DM 채널 목록 조회 후 에이전트 ID로 필터링
	channels, err := apiclient.DoList[DMChannel](client, context.Background(), "GET",
		"/api/v1/workspaces/"+workspaceID+"/dm-channels", nil)
	if err != nil {
		return nil, fmt.Errorf("DM 채널 목록 조회 실패: %w", err)
	}

	for i, ch := range channels {
		if ch.AgentID == agent.ID {
			return &channels[i], nil
		}
	}

	return nil, fmt.Errorf("에이전트 %s의 DM 채널을 찾을 수 없습니다", agent.Name)
}

// sendChatMessage는 채널에 메시지를 전송합니다.
func sendChatMessage(client *apiclient.Client, channelID, content string) error {
	body := map[string]string{"content": content}
	_, err := apiclient.Do[map[string]interface{}](client, context.Background(), "POST",
		"/api/v1/channels/"+channelID+"/messages", body)
	if err != nil {
		return fmt.Errorf("메시지 전송 실패: %w", err)
	}
	return nil
}

// runChatOneShot은 one-shot 채팅을 수행합니다.
// 메시지 전송 → SSE 구독 → agent_typing 이벤트 수신 → IsComplete=true 시 종료
func runChatOneShot(ctx context.Context, client *apiclient.Client, out io.Writer, channelID, message string) error {
	// 메시지 전송
	if err := sendChatMessage(client, channelID, message); err != nil {
		return err
	}

	// SSE 구독하여 응답 수신
	return receiveAgentResponse(ctx, client, out, channelID)
}

// receiveAgentResponse는 SSE를 통해 에이전트 응답을 수신합니다.
func receiveAgentResponse(ctx context.Context, client *apiclient.Client, out io.Writer, channelID string) error {
	token, err := client.Token()
	if err != nil {
		return fmt.Errorf("인증 토큰 획득 실패: %w", err)
	}

	subscriber := apiclient.NewSSESubscriber(client.BaseURL(), client.WorkspaceID(), token)
	eventCh, errCh := subscriber.Subscribe(ctx)
	typingCh := apiclient.FilterAgentTyping(eventCh, channelID)

	for {
		select {
		case payload, ok := <-typingCh:
			if !ok {
				return nil
			}
			// TextDelta를 실시간으로 출력
			fmt.Fprint(out, payload.TextDelta)
			if payload.IsComplete {
				fmt.Fprintln(out)
				return nil
			}

		case err, ok := <-errCh:
			if !ok {
				return nil
			}
			if err != nil {
				return fmt.Errorf("SSE 스트림 오류: %w", err)
			}
			return nil

		case <-ctx.Done():
			return nil
		}
	}
}

// runChatREPL은 인터랙티브 REPL 모드를 실행합니다.
func runChatREPL(ctx context.Context, client *apiclient.Client, in io.Reader, out io.Writer, channel *DMChannel) error {
	fmt.Fprintln(out, "채팅 REPL 모드 (Ctrl+C로 종료)")
	fmt.Fprintf(out, "채널: %s\n\n", channel.ID)

	scanner := bufio.NewScanner(in)
	for {
		fmt.Fprint(out, "> ")
		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// 메시지 전송 및 응답 대기
		if err := runChatOneShot(ctx, client, out, channel.ID, line); err != nil {
			fmt.Fprintf(out, "오류: %v\n", err)
		}

		if ctx.Err() != nil {
			break
		}
	}

	return scanner.Err()
}

// runChatWatch는 채널 이벤트를 감시합니다.
func runChatWatch(ctx context.Context, client *apiclient.Client, out io.Writer, channelID string) error {
	token, err := client.Token()
	if err != nil {
		return fmt.Errorf("인증 토큰 획득 실패: %w", err)
	}

	fmt.Fprintf(out, "채널 %s 감시 중...\n", channelID)

	subscriber := apiclient.NewSSESubscriber(client.BaseURL(), client.WorkspaceID(), token)
	eventCh, errCh := subscriber.Subscribe(ctx)

	for {
		select {
		case event, ok := <-eventCh:
			if !ok {
				return nil
			}
			fmt.Fprintf(out, "[%s] %s\n", event.Type, string(event.Data))

		case err, ok := <-errCh:
			if !ok {
				return nil
			}
			if err != nil {
				return fmt.Errorf("SSE 스트림 오류: %w", err)
			}
			return nil

		case <-ctx.Done():
			return nil
		}
	}
}

// chatHistoryResponse는 채널 메시지 API의 중첩 응답 구조입니다.
// 백엔드가 {data: {messages: [], has_more, first_unread_id}} 형태로 응답하므로
// DoList[ChatMessage]가 아닌 Do[chatHistoryResponse]로 파싱해야 합니다.
// REQ-CC-002: message.go의 messageListResponse와 동일한 패턴을 적용합니다.
type chatHistoryResponse struct {
	Messages      []ChatMessage `json:"messages"`
	HasMore       bool          `json:"has_more"`
	FirstUnreadID *string       `json:"first_unread_id"`
}

// runChatHistory는 채널의 채팅 기록을 출력합니다.
func runChatHistory(client *apiclient.Client, out io.Writer, channelID string, limit int, jsonOutput bool) error {
	path := "/api/v1/channels/" + channelID + "/messages"
	if limit > 0 {
		path += fmt.Sprintf("?limit=%d", limit)
	}

	// REQ-CC-002: 백엔드가 {data:{messages:[], has_more, first_unread_id}} 형태로 응답하므로
	// DoList[ChatMessage] 대신 Do[chatHistoryResponse]로 파싱한다.
	// 이전: DoList[ChatMessage] — data를 직접 배열로 파싱하려다 실패
	// 이후: Do[chatHistoryResponse] — 중첩 구조를 올바르게 파싱
	resp, err := apiclient.Do[chatHistoryResponse](client, context.Background(), "GET", path, nil)
	if err != nil {
		return fmt.Errorf("채팅 기록 조회 실패: %w", err)
	}

	messages := resp.Messages
	if jsonOutput {
		return apiclient.PrintJSON(out, messages)
	}

	for _, msg := range messages {
		prefix := "User"
		if msg.Role == "assistant" || msg.Role == "agent" {
			prefix = "Agent"
		}
		fmt.Fprintf(out, "[%s] %s\n", prefix, msg.Content)
	}
	return nil
}
