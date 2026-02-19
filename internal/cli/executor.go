// Package cli는 Local Agent Bridge의 CLI 명령어 실행 엔진을 제공합니다.
// SPEC-SKILL-V2-001 Block C: CLI Skill Integration
// 서버에서 수신한 CLI 명령어를 보안 검증 후 실행하고, 결과를 파싱하여 반환합니다.
package cli

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/insajin/autopus-agent-protocol"
	"github.com/rs/zerolog/log"
)

const (
	// DefaultTimeout은 CLI 명령어 실행 기본 타임아웃입니다 (120초).
	DefaultTimeout = 120 * time.Second
	// MaxOutputBytes는 stdout/stderr 최대 캡처 크기입니다 (1MB).
	MaxOutputBytes = 1 * 1024 * 1024
)

// Executor는 CLI 명령어를 보안 검증 후 실행하고 결과를 파싱하는 실행기입니다.
type Executor struct {
	security *SecurityChecker
	parsers  map[string]Parser
}

// NewExecutor는 기본 보안 검증기와 파서를 갖춘 CLI Executor를 생성합니다.
func NewExecutor() *Executor {
	return &Executor{
		security: NewSecurityChecker(),
		parsers:  defaultParsers(),
	}
}

// Execute는 CLIRequestPayload로부터 CLI 명령어를 실행하고 결과를 반환합니다.
// 보안 검증 -> 명령어 실행 -> 출력 파싱 순서로 처리됩니다.
func (e *Executor) Execute(ctx context.Context, req *ws.CLIRequestPayload) *ws.CLIResultPayload {
	start := time.Now()

	// 1단계: 보안 검증
	if err := e.security.Validate(req.Command, req.WorkingDir); err != nil {
		return &ws.CLIResultPayload{
			ExitCode:   -1,
			Stderr:     fmt.Sprintf("보안 검증 실패: %s", err.Error()),
			DurationMs: time.Since(start).Milliseconds(),
		}
	}

	// 2단계: 타임아웃 결정
	timeout := DefaultTimeout
	if req.TimeoutSeconds > 0 {
		timeout = time.Duration(req.TimeoutSeconds) * time.Second
	}

	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 3단계: 명령어 준비 - sh -c를 사용하여 셸 구문 지원
	cmd := exec.CommandContext(cmdCtx, "sh", "-c", req.Command)
	if req.WorkingDir != "" {
		cmd.Dir = req.WorkingDir
	}

	// 환경 변수 설정
	if len(req.Env) > 0 {
		cmd.Env = cmd.Environ()
		for k, v := range req.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	// stdout/stderr 캡처 (메모리 보호를 위해 크기 제한 적용)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &limitedWriter{w: &stdout, limit: MaxOutputBytes}
	cmd.Stderr = &limitedWriter{w: &stderr, limit: MaxOutputBytes}

	log.Info().
		Str("command", req.Command).
		Str("dir", req.WorkingDir).
		Dur("timeout", timeout).
		Msg("[CLI] 명령어 실행 시작")

	// 4단계: 명령어 실행
	err := cmd.Run()
	duration := time.Since(start).Milliseconds()

	result := &ws.CLIResultPayload{
		Stdout:          stdout.String(),
		Stderr:          stderr.String(),
		DurationMs:      duration,
		StdoutTruncated: stdout.Len() >= MaxOutputBytes,
		StderrTruncated: stderr.Len() >= MaxOutputBytes,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else if cmdCtx.Err() == context.DeadlineExceeded {
			result.ExitCode = -1
			result.Stderr = fmt.Sprintf("명령어 타임아웃 (%s 초과): %s", timeout, result.Stderr)
		} else {
			result.ExitCode = -1
			result.Stderr = fmt.Sprintf("실행 오류: %s: %s", err.Error(), result.Stderr)
		}
	}

	// 5단계: 출력 파싱 (파싱 포맷이 지정된 경우)
	if req.ParseFormat != "" && req.ParseFormat != "plain_text" {
		if parser, ok := e.parsers[req.ParseFormat]; ok {
			parsed, parseErr := parser.Parse(result.Stdout)
			if parseErr != nil {
				log.Warn().
					Str("format", req.ParseFormat).
					Err(parseErr).
					Msg("[CLI] 출력 파싱 경고")
			} else {
				result.ParsedResult = parsed
			}
		}
	}

	log.Info().
		Int("exit_code", result.ExitCode).
		Int64("duration_ms", duration).
		Int("stdout_len", len(result.Stdout)).
		Int("stderr_len", len(result.Stderr)).
		Msg("[CLI] 명령어 실행 완료")

	return result
}

// limitedWriter는 지정된 크기 제한까지만 쓰기를 허용하는 io.Writer 래퍼입니다.
// 제한을 초과하는 데이터는 조용히 폐기됩니다.
type limitedWriter struct {
	w     *bytes.Buffer
	limit int
}

// Write는 제한 내에서 데이터를 쓰고, 초과분은 폐기합니다.
func (lw *limitedWriter) Write(p []byte) (int, error) {
	remaining := lw.limit - lw.w.Len()
	if remaining <= 0 {
		// 제한 초과 - 데이터를 폐기하되 성공으로 보고
		return len(p), nil
	}
	if len(p) > remaining {
		p = p[:remaining]
	}
	return lw.w.Write(p)
}
