// Package codegen는 Claude Code CLI를 사용한 MCP 서버 코드 생성을 처리합니다.
// SPEC-SELF-EXPAND-001: 자율 확장 에이전트 시스템의 코드 생성 컴포넌트입니다.
package codegen

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

// Generator는 Claude Code CLI를 래핑하여 MCP 서버 코드를 생성합니다.
type Generator struct {
	claudePath string        // claude CLI 바이너리 경로
	workDir    string        // 생성 작업의 기본 디렉토리
	timeout    time.Duration // 최대 생성 시간
	logger     *slog.Logger
}

// GenerateRequest는 코드 생성 요청 파라미터입니다.
type GenerateRequest struct {
	ServiceName  string   // MCP 서버 서비스 이름
	TemplateID   string   // 템플릿 식별자 (선택사항)
	Description  string   // 서비스 설명
	RequiredAPIs []string // 구현할 API 목록
	AuthType     string   // 인증 유형 (none, api_key, oauth2)
	OutputDir    string   // 샌드박스 출력 디렉토리
}

// GenerateResult는 코드 생성 결과입니다.
type GenerateResult struct {
	Files      []GeneratedFile // 생성된 파일 목록
	DurationMs int64           // 생성 소요 시간 (밀리초)
	TokensUsed int             // 사용된 Claude 토큰 수
	Error      string          // 에러 메시지 (실패 시)
}

// GeneratedFile은 생성된 단일 파일을 나타냅니다.
type GeneratedFile struct {
	Path      string // 상대 파일 경로
	Content   string // 파일 내용
	SizeBytes int64  // 파일 크기 (바이트)
}

// claudeCLIResponse는 claude CLI의 JSON 출력 구조입니다.
type claudeCLIResponse struct {
	Type              string  `json:"type"`
	Result            string  `json:"result"`
	CostUSD           float64 `json:"cost_usd,omitempty"`
	DurationMS        int64   `json:"duration_ms,omitempty"`
	TotalInputTokens  int     `json:"total_input_tokens,omitempty"`
	TotalOutputTokens int     `json:"total_output_tokens,omitempty"`
}

// ProgressFn은 코드 생성 진행 상황을 보고하는 콜백 함수입니다.
type ProgressFn func(phase string, progress int, message string)

// NewGenerator는 새로운 Generator를 생성합니다.
func NewGenerator(claudePath, workDir string, timeout time.Duration, logger *slog.Logger) *Generator {
	if logger == nil {
		logger = slog.Default()
	}
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	return &Generator{
		claudePath: claudePath,
		workDir:    workDir,
		timeout:    timeout,
		logger:     logger,
	}
}

// Generate는 Claude CLI를 사용하여 MCP 서버 코드를 생성합니다.
//
// 생성 과정:
//  1. 출력 디렉토리 확인
//  2. 생성 프롬프트 빌드
//  3. claude CLI 실행
//  4. 생성된 파일 수집
//  5. 결과 반환
func (g *Generator) Generate(ctx context.Context, req GenerateRequest, progressFn ProgressFn) (*GenerateResult, error) {
	startTime := time.Now()

	if req.ServiceName == "" {
		return nil, fmt.Errorf("서비스 이름이 비어있습니다")
	}
	if req.Description == "" {
		return nil, fmt.Errorf("서비스 설명이 비어있습니다")
	}
	if req.OutputDir == "" {
		return nil, fmt.Errorf("출력 디렉토리가 지정되지 않았습니다")
	}

	// Phase 1: 템플릿 로딩
	if progressFn != nil {
		progressFn("template_loading", 10, fmt.Sprintf("MCP 서버 '%s' 생성 준비 중", req.ServiceName))
	}

	prompt := g.buildPrompt(req)

	g.logger.Info("코드 생성 시작",
		slog.String("service", req.ServiceName),
		slog.String("output_dir", req.OutputDir),
		slog.Int("required_apis", len(req.RequiredAPIs)),
	)

	// Phase 2: 코드 생성
	if progressFn != nil {
		progressFn("generating", 30, "Claude CLI로 코드 생성 중")
	}

	// 타임아웃 컨텍스트 생성
	execCtx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	// claude CLI 명령 구성
	args := []string{
		"--print",
		"--output-format", "json",
		"--output-dir", req.OutputDir,
		prompt,
	}

	cmd := exec.CommandContext(execCtx, g.claudePath, args...)
	cmd.Dir = g.workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	if progressFn != nil {
		progressFn("generating", 70, "Claude CLI 실행 완료")
	}

	// CLI 실행 결과 처리
	var tokensUsed int
	if err != nil {
		// 컨텍스트 타임아웃 또는 취소 확인
		if ctx.Err() != nil {
			return nil, fmt.Errorf("코드 생성이 취소되었습니다: %w", ctx.Err())
		}
		if execCtx.Err() != nil {
			return nil, fmt.Errorf("코드 생성 타임아웃 (%v): %w", g.timeout, execCtx.Err())
		}

		g.logger.Error("claude CLI 실행 실패",
			slog.String("service", req.ServiceName),
			slog.String("stderr", stderr.String()),
			slog.String("error", err.Error()),
		)

		return &GenerateResult{
			DurationMs: time.Since(startTime).Milliseconds(),
			Error:      fmt.Sprintf("claude CLI 실행 실패: %s", err.Error()),
		}, nil
	}

	// CLI JSON 응답 파싱 (토큰 사용량 추출)
	tokensUsed = g.parseTokenUsage(stdout.Bytes())

	// Phase 3: 파일 수집
	if progressFn != nil {
		progressFn("collecting", 90, "생성된 파일 수집 중")
	}

	collector := NewCollector()
	files, collectErr := collector.Collect(req.OutputDir)
	if collectErr != nil {
		g.logger.Error("파일 수집 실패",
			slog.String("service", req.ServiceName),
			slog.String("output_dir", req.OutputDir),
			slog.String("error", collectErr.Error()),
		)
		return &GenerateResult{
			DurationMs: time.Since(startTime).Milliseconds(),
			TokensUsed: tokensUsed,
			Error:      fmt.Sprintf("생성된 파일 수집 실패: %s", collectErr.Error()),
		}, nil
	}

	if progressFn != nil {
		progressFn("collecting", 100, fmt.Sprintf("완료: %d개 파일 생성", len(files)))
	}

	g.logger.Info("코드 생성 완료",
		slog.String("service", req.ServiceName),
		slog.Int("files", len(files)),
		slog.Int64("duration_ms", time.Since(startTime).Milliseconds()),
		slog.Int("tokens_used", tokensUsed),
	)

	return &GenerateResult{
		Files:      files,
		DurationMs: time.Since(startTime).Milliseconds(),
		TokensUsed: tokensUsed,
	}, nil
}

// buildPrompt는 MCP 서버 생성을 위한 프롬프트를 구성합니다.
func (g *Generator) buildPrompt(req GenerateRequest) string {
	var b strings.Builder

	b.WriteString("Create a complete MCP (Model Context Protocol) server project with the following specifications.\n\n")

	// 서비스 정보
	b.WriteString("## Service Information\n\n")
	b.WriteString(fmt.Sprintf("- Service Name: %s\n", req.ServiceName))
	b.WriteString(fmt.Sprintf("- Description: %s\n", req.Description))
	if req.TemplateID != "" {
		b.WriteString(fmt.Sprintf("- Template: %s\n", req.TemplateID))
	}
	b.WriteString("\n")

	// 필요 API/도구
	if len(req.RequiredAPIs) > 0 {
		b.WriteString("## Required Tools (MCP Tools to Implement)\n\n")
		for i, api := range req.RequiredAPIs {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, api))
		}
		b.WriteString("\n")
	}

	// 인증 유형
	b.WriteString("## Authentication\n\n")
	if req.AuthType != "" && req.AuthType != "none" {
		b.WriteString(fmt.Sprintf("- Auth Type: %s\n", req.AuthType))
	} else {
		b.WriteString("- Auth Type: none (no authentication required)\n")
	}
	b.WriteString("\n")

	// 기술 요구사항
	b.WriteString("## Technical Requirements\n\n")
	b.WriteString("- Runtime: Node.js (TypeScript)\n")
	b.WriteString("- MCP SDK: @modelcontextprotocol/sdk (latest version)\n")
	b.WriteString("- Use stdio transport for MCP communication\n")
	b.WriteString("- Follow MCP specification for tool definitions\n\n")

	// 프로젝트 구조
	b.WriteString("## Required Project Structure\n\n")
	b.WriteString("Generate the following files:\n\n")
	b.WriteString("1. `package.json` - Node.js project manifest with dependencies\n")
	b.WriteString("2. `tsconfig.json` - TypeScript configuration\n")
	b.WriteString("3. `src/index.ts` - Main entry point with MCP server setup and tool handlers\n")
	b.WriteString("4. `security.json` - Security manifest declaring allowed resources\n")
	b.WriteString("5. `README.md` - Usage instructions\n\n")

	// 보안 가이드라인
	b.WriteString("## Security Guidelines\n\n")
	b.WriteString("- The security.json must declare all allowed domains, required env vars, and access scopes\n")
	b.WriteString("- Never hardcode credentials or secrets\n")
	b.WriteString("- Use environment variables for sensitive configuration\n")
	b.WriteString("- Validate and sanitize all tool inputs\n")
	b.WriteString("- Implement proper error handling with descriptive messages\n\n")

	// 코드 품질
	b.WriteString("## Code Quality\n\n")
	b.WriteString("- Use TypeScript strict mode\n")
	b.WriteString("- Add JSDoc comments for all exported functions\n")
	b.WriteString("- Handle errors gracefully with try-catch\n")
	b.WriteString("- Use async/await for asynchronous operations\n")

	return b.String()
}

// parseTokenUsage는 claude CLI의 JSON 응답에서 토큰 사용량을 추출합니다.
func (g *Generator) parseTokenUsage(output []byte) int {
	// claude CLI는 여러 줄의 JSON 객체를 출력할 수 있습니다.
	// 마지막 줄(result 타입)에서 토큰 정보를 추출합니다.
	lines := bytes.Split(output, []byte("\n"))
	for i := len(lines) - 1; i >= 0; i-- {
		line := bytes.TrimSpace(lines[i])
		if len(line) == 0 {
			continue
		}

		var resp claudeCLIResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			continue
		}

		if resp.Type == "result" {
			return resp.TotalInputTokens + resp.TotalOutputTokens
		}
	}
	return 0
}
