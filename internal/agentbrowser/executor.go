package agentbrowser

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// CommandExecutor는 agent-browser CLI 명령을 실행한다.
type CommandExecutor struct {
	logger zerolog.Logger
	// commandFn은 외부 명령을 실행하는 함수이다 (테스트용 주입 가능).
	commandFn func(ctx context.Context, name string, args ...string) *exec.Cmd
	// binaryName은 agent-browser 실행 파일 이름이다.
	binaryName string
	// cicdConfig는 CI/CD 환경 설정이다 (REQ-M5-04).
	cicdConfig CICDConfig
}

// NewCommandExecutor는 새로운 CommandExecutor를 생성한다.
func NewCommandExecutor(logger zerolog.Logger) *CommandExecutor {
	return &CommandExecutor{
		logger:     logger,
		commandFn:  exec.CommandContext,
		binaryName: "agent-browser",
	}
}

// SetCICDConfig는 CI/CD 환경 설정을 적용한다.
func (ce *CommandExecutor) SetCICDConfig(config CICDConfig) {
	ce.cicdConfig = config
}

// Execute는 agent-browser CLI 명령을 실행하고 결과를 반환한다.
// CI/CD 모드가 활성화된 경우 글로벌 타임아웃을 적용한다.
func (ce *CommandExecutor) Execute(ctx context.Context, args ...string) (*CommandResult, error) {
	start := time.Now()
	result := &CommandResult{}

	// CI/CD 글로벌 타임아웃 적용 (REQ-M5-04)
	if ce.cicdConfig.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, ce.cicdConfig.Timeout)
		defer cancel()
	}

	ce.logger.Debug().Strs("args", args).Bool("cicd_mode", ce.cicdConfig.IsEnabled()).Msg("agent-browser 명령 실행")

	var stdout, stderr bytes.Buffer
	cmd := ce.commandFn(ctx, ce.binaryName, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result.DurationMs = time.Since(start).Milliseconds()

	if err != nil {
		// 종료 코드가 0이 아니더라도 stderr 출력이 있을 수 있다
		errMsg := stderr.String()
		if errMsg == "" {
			errMsg = err.Error()
		}
		result.Error = strings.TrimSpace(errMsg)
		return result, fmt.Errorf("agent-browser 명령 실행 실패: %w", err)
	}

	// stdout 파싱
	output := stdout.String()
	result.Output = strings.TrimSpace(output)

	// JSON 출력 시도 파싱 (snapshot 등의 구조화된 응답)
	ce.parseJSONOutput(result, output)

	ce.logger.Debug().
		Int64("duration_ms", result.DurationMs).
		Int("output_len", len(result.Output)).
		Msg("agent-browser 명령 실행 완료")

	return result, nil
}

// ExecuteFromPayload는 BrowserActionPayload에서 CLI 인수를 빌드하고 실행한다.
func (ce *CommandExecutor) ExecuteFromPayload(ctx context.Context, payload BrowserActionPayload) (*CommandResult, error) {
	args := ce.BuildArgs(payload)
	return ce.Execute(ctx, args...)
}

// BuildArgs는 BrowserActionPayload에서 agent-browser CLI 인수 목록을 생성한다.
// CI/CD 모드가 활성화된 경우 --headless, --json, --no-color 플래그를 자동 추가한다.
func (ce *CommandExecutor) BuildArgs(payload BrowserActionPayload) []string {
	args := []string{payload.Command}

	// CI/CD 모드 플래그 주입 (REQ-M5-04)
	if ce.cicdConfig.Headless {
		args = append(args, "--headless")
	}
	if ce.cicdConfig.JSONOutput {
		args = append(args, "--json")
	}
	if ce.cicdConfig.NoColor {
		args = append(args, "--no-color")
	}

	// @ref 인수 추가 (click @e42, fill @e15 등)
	if payload.Ref != nil {
		args = append(args, fmt.Sprintf("@e%d", *payload.Ref))
	}

	// 명령별 파라미터 처리
	switch payload.Command {
	case "open", "navigate":
		if url, ok := payload.Params["url"].(string); ok {
			args = append(args, url)
		}
	case "fill", "type":
		if text, ok := payload.Params["text"].(string); ok {
			args = append(args, text)
		}
	case "press":
		if key, ok := payload.Params["key"].(string); ok {
			args = append(args, key)
		}
	case "scroll":
		if direction, ok := payload.Params["direction"].(string); ok {
			args = append(args, direction)
		}
		if amount, ok := payload.Params["amount"]; ok {
			args = append(args, fmt.Sprintf("%v", amount))
		}
	case "get":
		if subCmd, ok := payload.Params["type"].(string); ok {
			args = append(args, subCmd)
		}
		if selector, ok := payload.Params["selector"].(string); ok {
			args = append(args, selector)
		}
	case "wait":
		if selector, ok := payload.Params["selector"].(string); ok {
			args = append(args, selector)
		}
	case "set":
		if subCmd, ok := payload.Params["type"].(string); ok {
			args = append(args, subCmd)
		}
		if w, ok := payload.Params["width"]; ok {
			args = append(args, fmt.Sprintf("%v", w))
		}
		if h, ok := payload.Params["height"]; ok {
			args = append(args, fmt.Sprintf("%v", h))
		}
	}

	return args
}

// parseJSONOutput은 stdout 출력이 JSON인 경우 구조화된 데이터를 추출한다.
func (ce *CommandExecutor) parseJSONOutput(result *CommandResult, output string) {
	trimmed := strings.TrimSpace(output)
	if len(trimmed) == 0 || (trimmed[0] != '{' && trimmed[0] != '[') {
		return
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		// JSON이 아닌 경우 무시
		return
	}

	// snapshot 필드 추출
	if snapshot, ok := parsed["snapshot"].(string); ok {
		result.Snapshot = snapshot
	}

	// output 필드 추출
	if out, ok := parsed["output"].(string); ok {
		result.Output = out
	}

	// error 필드 추출
	if errMsg, ok := parsed["error"].(string); ok {
		result.Error = errMsg
	}
}
