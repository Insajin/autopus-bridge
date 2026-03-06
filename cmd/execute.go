package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/insajin/autopus-bridge/internal/auth"
	"github.com/insajin/autopus-bridge/internal/config"
	"github.com/insajin/autopus-bridge/internal/mcpserver"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

var (
	executeAgentID    string
	executeAgentName  string
	executeWorkspace  string
	executeModel      string
	executeProvider   string
	executeTools      []string
	executeTimeoutSec int
	executeMaxTokens  int
	executeWait       bool
	executeWaitPoll   time.Duration
	executeWaitLimit  time.Duration
	executeStream     bool
	executeJSON       bool
)

type executeOutput struct {
	ExecutionID string          `json:"execution_id"`
	WorkspaceID string          `json:"workspace_id"`
	AgentID     string          `json:"agent_id"`
	AgentName   string          `json:"agent_name,omitempty"`
	Status      string          `json:"status,omitempty"`
	Provider    string          `json:"provider,omitempty"`
	Result      json.RawMessage `json:"result,omitempty"`
	Error       string          `json:"error,omitempty"`
}

var executeCmd = &cobra.Command{
	Use:   "execute <task description>",
	Short: "Autopus 워크스페이스에 작업을 제출합니다",
	Long: `현재 로그인된 Autopus 워크스페이스에 작업을 제출하고 결과를 출력합니다.

에이전트 ID를 명시하지 않으면 현재 워크스페이스의 에이전트 목록을 조회하여
자동 선택하거나, 여러 개일 경우 선택 프롬프트를 표시합니다.`,
	Args: cobra.ExactArgs(1),
	RunE: runExecute,
}

func init() {
	rootCmd.AddCommand(executeCmd)

	executeCmd.Flags().StringVar(&executeAgentID, "agent-id", "", "대상 에이전트 ID")
	executeCmd.Flags().StringVar(&executeAgentName, "agent", "", "대상 에이전트 이름")
	executeCmd.Flags().StringVar(&executeWorkspace, "workspace-id", "", "대상 워크스페이스 ID (기본값: 저장된 credentials)")
	executeCmd.Flags().StringVar(&executeModel, "model", "", "실행에 사용할 모델")
	executeCmd.Flags().StringVar(&executeProvider, "provider", "", "실행에 사용할 프로바이더")
	executeCmd.Flags().StringSliceVar(&executeTools, "tools", nil, "허용 도구 목록 (쉼표 구분 또는 반복 사용)")
	executeCmd.Flags().IntVar(&executeTimeoutSec, "timeout", 0, "실행 타임아웃(초)")
	executeCmd.Flags().IntVar(&executeMaxTokens, "max-tokens", 0, "최대 생성 토큰 수")
	executeCmd.Flags().BoolVar(&executeWait, "wait", false, "실행이 끝날 때까지 상태를 polling합니다")
	executeCmd.Flags().DurationVar(&executeWaitPoll, "poll-interval", 2*time.Second, "상태 polling 간격")
	executeCmd.Flags().DurationVar(&executeWaitLimit, "wait-timeout", 10*time.Minute, "최대 대기 시간")
	executeCmd.Flags().BoolVar(&executeStream, "stream", false, "Server-Sent Events로 실행 출력을 스트리밍합니다")
	executeCmd.Flags().BoolVar(&executeJSON, "json", false, "JSON 형식으로 출력")
}

func runExecute(cmd *cobra.Command, args []string) error {
	if err := validateExecuteMode(); err != nil {
		return err
	}

	creds, err := auth.Load()
	if err != nil {
		return fmt.Errorf("저장된 인증 정보 로드 실패: %w", err)
	}
	if creds == nil {
		return errors.New("로그인이 필요합니다. 'autopus-bridge login' 또는 'autopus-bridge up'을 먼저 실행하세요")
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("설정 로드 실패: %w", err)
	}

	workspaceID := resolveWorkspaceID(creds)
	if workspaceID == "" {
		return errors.New("워크스페이스가 선택되지 않았습니다. 'autopus-bridge up'을 다시 실행하거나 --workspace-id를 지정하세요")
	}

	apiBaseURL := serverURLToHTTPBase(resolveServerURL(creds, cfg))
	tokenRefresher := auth.NewTokenRefresher(creds)
	client := mcpserver.NewBackendClient(apiBaseURL, tokenRefresher, 60*time.Second, zerolog.Nop())

	submitCtx, submitCancel := context.WithTimeout(cmd.Context(), 90*time.Second)
	defer submitCancel()

	selectedAgent, err := resolveExecuteAgent(submitCtx, client, workspaceID, os.Stdin, os.Stdout)
	if err != nil {
		return err
	}

	if executeStream {
		token, tokenErr := tokenRefresher.GetToken()
		if tokenErr != nil {
			return fmt.Errorf("인증 토큰 획득 실패: %w", tokenErr)
		}

		executionID := uuid.NewString()
		fmt.Printf("Execution ID: %s\n", executionID)
		fmt.Printf("Workspace:    %s\n", workspaceID)
		fmt.Printf("Agent:        %s", selectedAgent.ID)
		if selectedAgent.Name != "" {
			fmt.Printf(" (%s)", selectedAgent.Name)
		}
		fmt.Println()
		fmt.Println()

		return streamExecution(
			cmd.Context(),
			client,
			apiBaseURL,
			workspaceID,
			executionID,
			token,
			selectedAgent.ID,
			selectedAgent.Name,
			args[0],
			cmd.OutOrStdout(),
		)
	}

	result, err := client.ExecuteTask(submitCtx, &mcpserver.ExecuteTaskRequest{
		WorkspaceID:       workspaceID,
		AgentID:           selectedAgent.ID,
		Prompt:            args[0],
		Provider:          executeProvider,
		Tools:             executeTools,
		Model:             executeModel,
		TimeoutSeconds:    executeTimeoutSec,
		MaxTokens:         executeMaxTokens,
		FallbackProviders: nil,
	})
	if err != nil {
		return fmt.Errorf("작업 제출 실패: %w", err)
	}

	output := executeOutput{
		ExecutionID: result.ExecutionID,
		WorkspaceID: workspaceID,
		AgentID:     selectedAgent.ID,
		AgentName:   selectedAgent.Name,
		Provider:    result.Provider,
		Result:      result.Result,
	}

	if executeWait {
		waitCtx := cmd.Context()
		waitCancel := func() {}
		if executeWaitLimit > 0 {
			waitCtx, waitCancel = context.WithTimeout(cmd.Context(), executeWaitLimit)
		}
		defer waitCancel()

		status, waitErr := waitForExecution(waitCtx, client, result.ExecutionID, executeWaitPoll, cmd.ErrOrStderr(), executeJSON)
		if waitErr != nil {
			return waitErr
		}
		output.Status = status.Status
		output.Error = status.Error
		if len(status.Result) > 0 {
			output.Result = status.Result
		}
	}

	if err := printExecuteOutput(output); err != nil {
		return err
	}

	if executeWait && isFailedExecutionStatus(output.Status) {
		if output.Error != "" {
			return fmt.Errorf("execution %s finished with status %s: %s", output.ExecutionID, output.Status, output.Error)
		}
		return fmt.Errorf("execution %s finished with status %s", output.ExecutionID, output.Status)
	}

	return nil
}

func validateExecuteMode() error {
	if executeWait && executeStream {
		return errors.New("--wait and --stream cannot be used together")
	}
	if !executeStream {
		return nil
	}
	if executeJSON {
		return errors.New("--stream does not support --json")
	}
	return nil
}

func printExecuteOutput(output executeOutput) error {
	if executeJSON {
		data, marshalErr := json.MarshalIndent(output, "", "  ")
		if marshalErr != nil {
			return fmt.Errorf("JSON 출력 생성 실패: %w", marshalErr)
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("Execution ID: %s\n", output.ExecutionID)
	fmt.Printf("Workspace:    %s\n", output.WorkspaceID)
	fmt.Printf("Agent:        %s", output.AgentID)
	if output.AgentName != "" {
		fmt.Printf(" (%s)", output.AgentName)
	}
	fmt.Println()
	if output.Status != "" {
		fmt.Printf("Status:       %s\n", output.Status)
	}
	if output.Provider != "" {
		fmt.Printf("Provider:     %s\n", output.Provider)
	}
	if output.Error != "" {
		fmt.Printf("Error:        %s\n", output.Error)
	}
	if len(output.Result) > 0 {
		fmt.Println()
		fmt.Println("Result:")
		fmt.Println(string(output.Result))
	}

	return nil
}

type sseStreamEvent struct {
	Type    string          `json:"type"`
	Content json.RawMessage `json:"content,omitempty"`
}

type streamSummaryContent struct {
	ExecutionID string          `json:"execution_id"`
	Status      string          `json:"status"`
	Result      json.RawMessage `json:"result,omitempty"`
	Error       string          `json:"error,omitempty"`
}

type streamPrinter struct {
	out           io.Writer
	inlineContent bool
	summary       *streamSummaryContent
}

func (p *streamPrinter) printEvent(event sseStreamEvent) error {
	if event.Type == "summary" {
		var summary streamSummaryContent
		if err := json.Unmarshal(event.Content, &summary); err == nil {
			p.summary = &summary
			return nil
		}
	}
	switch event.Type {
	case "text", "content":
		var text string
		if err := json.Unmarshal(event.Content, &text); err == nil {
			_, err = fmt.Fprint(p.out, text)
			p.inlineContent = true
			return err
		}
	}

	if p.inlineContent {
		if _, err := fmt.Fprintln(p.out); err != nil {
			return err
		}
		p.inlineContent = false
	}

	if event.Type == "done" {
		return nil
	}
	if len(event.Content) > 0 {
		_, err := fmt.Fprintf(p.out, "[%s] %s\n", event.Type, string(event.Content))
		return err
	}
	_, err := fmt.Fprintf(p.out, "[%s]\n", event.Type)
	return err
}

func (p *streamPrinter) finish() error {
	if !p.inlineContent {
		return nil
	}
	p.inlineContent = false
	_, err := fmt.Fprintln(p.out)
	return err
}

func streamExecution(
	ctx context.Context,
	client *mcpserver.BackendClient,
	baseURL string,
	workspaceID string,
	executionID string,
	token string,
	agentID string,
	agentName string,
	prompt string,
	out io.Writer,
) error {
	streamURL, err := buildExecutionStreamURL(baseURL, workspaceID, executionID, agentID, prompt)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, streamURL, nil)
	if err != nil {
		return fmt.Errorf("스트림 요청 생성 실패: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "text/event-stream")

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("스트림 연결 실패: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("스트림 요청 실패 (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	printer := &streamPrinter{out: out}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") || !strings.HasPrefix(line, "data:") {
			continue
		}

		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		var event sseStreamEvent
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			if printer.inlineContent {
				if finishErr := printer.finish(); finishErr != nil {
					return finishErr
				}
			}
			if _, writeErr := fmt.Fprintln(out, payload); writeErr != nil {
				return writeErr
			}
			continue
		}

		if err := printer.printEvent(event); err != nil {
			return err
		}
		if event.Type == "done" {
			return printer.finish()
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("스트림 읽기 실패: %w", err)
	}

	if err := printer.finish(); err != nil {
		return err
	}

	if client == nil {
		return nil
	}

	summary := printer.summary
	if summary == nil && client != nil {
		status, err := fetchStreamExecutionSummary(ctx, client, executionID)
		if err == nil && status != nil {
			summary = &streamSummaryContent{
				ExecutionID: status.ExecutionID,
				Status:      status.Status,
				Result:      status.Result,
				Error:       status.Error,
			}
		}
	}
	if summary == nil {
		return nil
	}
	output := executeOutput{
		ExecutionID: executionID,
		WorkspaceID: workspaceID,
		AgentID:     agentID,
		AgentName:   agentName,
		Status:      summary.Status,
		Result:      summary.Result,
		Error:       summary.Error,
	}
	if summary.ExecutionID != "" {
		output.ExecutionID = summary.ExecutionID
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "Summary:")
	fmt.Fprintln(out, "--------")
	if err := printExecuteOutput(output); err != nil {
		return err
	}
	if isFailedExecutionStatus(output.Status) {
		if output.Error != "" {
			return fmt.Errorf("execution %s finished with status %s: %s", output.ExecutionID, output.Status, output.Error)
		}
		return fmt.Errorf("execution %s finished with status %s", output.ExecutionID, output.Status)
	}
	return nil
}

func buildExecutionStreamURL(baseURL, workspaceID, executionID, agentID, prompt string) (string, error) {
	u, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return "", fmt.Errorf("스트림 URL 생성 실패: %w", err)
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/api/v1/workspaces/" + workspaceID + "/executions/" + executionID + "/stream"
	query := u.Query()
	query.Set("agent_id", agentID)
	query.Set("prompt", prompt)
	if executeProvider != "" {
		query.Set("provider", executeProvider)
	}
	if executeModel != "" {
		query.Set("model", executeModel)
	}
	if len(executeTools) > 0 {
		query.Set("tools", strings.Join(executeTools, ","))
	}
	if executeTimeoutSec > 0 {
		query.Set("timeout_seconds", strconv.Itoa(executeTimeoutSec))
	}
	if executeMaxTokens > 0 {
		query.Set("max_tokens", strconv.Itoa(executeMaxTokens))
	}
	u.RawQuery = query.Encode()
	return u.String(), nil
}

func fetchStreamExecutionSummary(
	ctx context.Context,
	fetcher executionStatusFetcher,
	executionID string,
) (*mcpserver.ExecutionStatus, error) {
	summaryCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		status, err := fetcher.GetExecutionStatus(summaryCtx, executionID)
		if err == nil && status != nil {
			if status.ExecutionID == "" {
				status.ExecutionID = executionID
			}
			if isTerminalExecutionStatus(status.Status) {
				return status, nil
			}
		}

		select {
		case <-summaryCtx.Done():
			return nil, summaryCtx.Err()
		case <-ticker.C:
		}
	}
}

func resolveWorkspaceID(creds *auth.Credentials) string {
	if executeWorkspace != "" {
		return executeWorkspace
	}
	if creds == nil {
		return ""
	}
	return creds.WorkspaceID
}

func resolveServerURL(creds *auth.Credentials, cfg *config.Config) string {
	if creds != nil && creds.ServerURL != "" {
		return creds.ServerURL
	}
	if cfg != nil && cfg.Server.URL != "" {
		return cfg.Server.URL
	}
	return "wss://api.autopus.co/ws/agent"
}

func serverURLToHTTPBase(serverURL string) string {
	httpURL := serverURL
	switch {
	case strings.HasPrefix(httpURL, "wss://"):
		httpURL = "https://" + strings.TrimPrefix(httpURL, "wss://")
	case strings.HasPrefix(httpURL, "ws://"):
		httpURL = "http://" + strings.TrimPrefix(httpURL, "ws://")
	}

	if idx := strings.Index(httpURL, "/ws"); idx != -1 {
		httpURL = httpURL[:idx]
	}

	return strings.TrimRight(httpURL, "/")
}

func resolveExecuteAgent(
	ctx context.Context,
	client *mcpserver.BackendClient,
	workspaceID string,
	in io.Reader,
	out io.Writer,
) (*mcpserver.AgentInfo, error) {
	if executeAgentID != "" {
		return &mcpserver.AgentInfo{ID: executeAgentID, Name: executeAgentName}, nil
	}

	agentsResp, err := client.ListAgents(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("에이전트 목록 조회 실패: %w", err)
	}
	if agentsResp == nil || len(agentsResp.Agents) == 0 {
		return nil, errors.New("사용 가능한 에이전트가 없습니다")
	}

	if executeAgentName != "" {
		agent, matchErr := findAgentByName(agentsResp.Agents, executeAgentName)
		if matchErr != nil {
			return nil, matchErr
		}
		return agent, nil
	}

	if len(agentsResp.Agents) == 1 {
		return &agentsResp.Agents[0], nil
	}

	return chooseAgentInteractively(in, out, agentsResp.Agents)
}

func findAgentByName(agents []mcpserver.AgentInfo, name string) (*mcpserver.AgentInfo, error) {
	var exactMatches []mcpserver.AgentInfo
	var partialMatches []mcpserver.AgentInfo
	lowerName := strings.ToLower(strings.TrimSpace(name))

	for _, agent := range agents {
		agentName := strings.ToLower(agent.Name)
		switch {
		case agentName == lowerName:
			exactMatches = append(exactMatches, agent)
		case strings.Contains(agentName, lowerName):
			partialMatches = append(partialMatches, agent)
		}
	}

	switch {
	case len(exactMatches) == 1:
		return &exactMatches[0], nil
	case len(exactMatches) > 1:
		return nil, fmt.Errorf("이름이 정확히 일치하는 에이전트가 여러 개입니다: %s", name)
	case len(partialMatches) == 1:
		return &partialMatches[0], nil
	case len(partialMatches) > 1:
		return nil, fmt.Errorf("이름이 부분 일치하는 에이전트가 여러 개입니다: %s", name)
	default:
		return nil, fmt.Errorf("에이전트를 찾을 수 없습니다: %s", name)
	}
}

func chooseAgentInteractively(
	in io.Reader,
	out io.Writer,
	agents []mcpserver.AgentInfo,
) (*mcpserver.AgentInfo, error) {
	if len(agents) == 0 {
		return nil, errors.New("선택할 에이전트가 없습니다")
	}

	fmt.Fprintln(out, "사용할 에이전트를 선택하세요:")
	for i, agent := range agents {
		if agent.Model != "" {
			fmt.Fprintf(out, "  %d. %s (%s) [%s]\n", i+1, agent.Name, agent.ID, agent.Model)
			continue
		}
		fmt.Fprintf(out, "  %d. %s (%s)\n", i+1, agent.Name, agent.ID)
	}
	fmt.Fprint(out, "> ")

	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("입력 읽기 실패: %w", err)
	}

	choice, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		return nil, fmt.Errorf("유효한 번호를 입력하세요")
	}
	if choice < 1 || choice > len(agents) {
		return nil, fmt.Errorf("선택 범위를 벗어났습니다: %d", choice)
	}

	return &agents[choice-1], nil
}

type executionStatusFetcher interface {
	GetExecutionStatus(ctx context.Context, executionID string) (*mcpserver.ExecutionStatus, error)
}

func waitForExecution(
	ctx context.Context,
	fetcher executionStatusFetcher,
	executionID string,
	pollInterval time.Duration,
	out io.Writer,
	jsonOutput bool,
) (*mcpserver.ExecutionStatus, error) {
	if pollInterval <= 0 {
		pollInterval = 2 * time.Second
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	var lastStatus string

	for {
		status, err := fetcher.GetExecutionStatus(ctx, executionID)
		if err != nil {
			return nil, fmt.Errorf("실행 상태 조회 실패: %w", err)
		}
		if status.ExecutionID == "" {
			status.ExecutionID = executionID
		}

		if !jsonOutput && status.Status != "" && status.Status != lastStatus {
			fmt.Fprintf(out, "Execution %s status: %s\n", status.ExecutionID, status.Status)
			lastStatus = status.Status
		}

		if isTerminalExecutionStatus(status.Status) {
			return status, nil
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("실행 대기 시간 초과: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func isTerminalExecutionStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "failed", "rejected", "cancelled", "approved":
		return true
	default:
		return false
	}
}

func isFailedExecutionStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "failed", "rejected", "cancelled":
		return true
	default:
		return false
	}
}
