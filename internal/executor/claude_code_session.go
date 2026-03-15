// Package executor - Claude Code CLI 세션 구현
package executor

import (
	"context"
	"fmt"
	"strings"
	"sync"

	claudecode "github.com/bpowers/go-claudecode"
)

// ClaudeCodeSession은 go-claudecode SDK를 사용하는 CodingSession 구현체입니다.
// @MX:NOTE: [AUTO] Claude Code CLI와 go-claudecode SDK를 통해 상호작용
// @MX:SPEC: SPEC-CODING-RELAY-001 TASK-003
type ClaudeCodeSession struct {
	client    *claudecode.Client
	sessionID string
	opened    bool
	mu        sync.Mutex
}

// NewClaudeCodeSession은 새로운 ClaudeCodeSession을 생성합니다.
func NewClaudeCodeSession() *ClaudeCodeSession {
	return &ClaudeCodeSession{}
}

// Open은 claude CLI를 시작하고 세션을 초기화합니다.
func (s *ClaudeCodeSession) Open(ctx context.Context, req CodingSessionOpenRequest) error {
	opts := s.buildOptions(req)

	s.client = claudecode.NewClient(opts...)

	if err := s.client.Connect(ctx); err != nil {
		return fmt.Errorf("Claude Code 세션 연결 실패: %w", err)
	}

	s.opened = true
	return nil
}

// Send는 메시지를 전송하고 완전한 응답을 수집합니다.
func (s *ClaudeCodeSession) Send(ctx context.Context, message string) (*CodingSessionResponse, error) {
	if !s.opened || s.client == nil {
		return nil, fmt.Errorf("세션이 열려있지 않습니다: Open()을 먼저 호출하세요")
	}

	// 메시지 전송
	if err := s.client.Query(ctx, message, "default"); err != nil {
		return nil, fmt.Errorf("메시지 전송 실패: %w", err)
	}

	// 응답 수집
	return s.collectResponse(ctx)
}

// SessionID는 현재 세션 ID를 반환합니다.
func (s *ClaudeCodeSession) SessionID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client == nil {
		return s.sessionID
	}
	id := s.client.SessionID()
	if id != "" {
		s.sessionID = id
	}
	return s.sessionID
}

// Close는 세션을 종료하고 리소스를 해제합니다.
func (s *ClaudeCodeSession) Close(ctx context.Context) error {
	if s.client == nil {
		return nil
	}
	defer func() {
		s.opened = false
		s.client = nil
	}()
	return s.client.Close(ctx)
}

// buildOptions는 CodingSessionOpenRequest로부터 claudecode.Option 슬라이스를 생성합니다.
func (s *ClaudeCodeSession) buildOptions(req CodingSessionOpenRequest) []claudecode.Option {
	var opts []claudecode.Option

	// 작업 디렉토리 설정
	if req.WorkDir != "" {
		opts = append(opts, claudecode.WithCWD(req.WorkDir))
	}

	// 모델 설정
	if req.Model != "" {
		opts = append(opts, claudecode.WithModel(req.Model))
	}

	// 예산 설정
	if req.MaxBudgetUSD > 0 {
		opts = append(opts, claudecode.WithMaxBudgetUSD(req.MaxBudgetUSD))
	}

	// 도구 설정
	if len(req.Tools) > 0 {
		opts = append(opts, claudecode.WithTools(req.Tools...))
	}

	// 허용된 도구 설정
	if len(req.AllowedTools) > 0 {
		opts = append(opts, claudecode.WithAllowedTools(req.AllowedTools...))
	}

	// 세션 재개 설정
	if req.ResumeSession != "" {
		opts = append(opts, claudecode.WithResume(req.ResumeSession))
	}

	// 시스템 프롬프트 설정
	if req.SystemPrompt != "" {
		opts = append(opts, claudecode.WithSystemPrompt(req.SystemPrompt))
	}

	// 권한 모드: bypassPermissions (에이전트 자동화용)
	opts = append(opts, claudecode.WithPermissionMode(claudecode.PermissionBypassPermissions))

	return opts
}

// collectResponse는 ResultMessage가 올 때까지 응답을 수집하여 CodingSessionResponse를 구성합니다.
func (s *ClaudeCodeSession) collectResponse(ctx context.Context) (*CodingSessionResponse, error) {
	resp := &CodingSessionResponse{}
	var contentParts []string

	msgCh := s.client.ReceiveResponse(ctx)
	for moe := range msgCh {
		if moe.Err != nil {
			return nil, fmt.Errorf("응답 수신 에러: %w", moe.Err)
		}

		switch msg := moe.Message.(type) {
		case *claudecode.AssistantMessage:
			// 텍스트 블록 수집
			for _, block := range msg.Content {
				if tb, ok := block.(claudecode.TextBlock); ok {
					contentParts = append(contentParts, tb.Text)
				}
			}

		case *claudecode.ResultMessage:
			// 최종 결과 — 메타데이터 수집
			resp.SessionID = msg.SessionID
			resp.TurnCount = msg.NumTurns
			if msg.TotalCostUSD != nil {
				resp.CostUSD = *msg.TotalCostUSD
			}
			// 세션 ID 업데이트
			s.mu.Lock()
			s.sessionID = msg.SessionID
			s.mu.Unlock()
		}
	}

	resp.Content = strings.Join(contentParts, "\n")
	return resp, nil
}
