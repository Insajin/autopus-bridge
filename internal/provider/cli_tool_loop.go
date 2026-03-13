// Package provider는 AI 프로바이더 통합 레이어를 제공합니다.
package provider

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

// cliToolCallMarkerStart는 모델이 도구 호출 JSON 블록의 시작을 표시하는 마커입니다.
const cliToolCallMarkerStart = "<<<TOOL_CALL>>>"

// cliToolCallMarkerEnd는 모델이 도구 호출 JSON 블록의 끝을 표시하는 마커입니다.
const cliToolCallMarkerEnd = "<<<END_TOOL_CALL>>>"

// cliToolCallPayload는 모델이 출력하는 도구 호출 JSON 페이로드입니다.
type cliToolCallPayload struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// buildCLIToolLoopPrompt는 도구 정의와 도구 호출 출력 형식 지침을 포함하는
// 프롬프트를 생성합니다. CLI 프로바이더는 네이티브 tool-calling을 지원하지 않기 때문에
// 프롬프트 기반 시뮬레이션을 사용합니다.
//
// 프롬프트 구조:
// 1. 시스템 프롬프트 (있는 경우)
// 2. 사용 가능한 도구 목록과 스키마
// 3. 도구 호출 출력 형식 지침
// 4. 대화 이력 (ToolLoopMessages)
func buildCLIToolLoopPrompt(req ExecuteRequest) string {
	var sb strings.Builder

	// 1. 시스템 프롬프트
	if req.SystemPrompt != "" {
		sb.WriteString("=== SYSTEM ===\n")
		sb.WriteString(req.SystemPrompt)
		sb.WriteString("\n\n")
	}

	// 2. 도구 정의
	if len(req.ToolDefinitions) > 0 {
		sb.WriteString("=== AVAILABLE TOOLS / 사용 가능한 도구 ===\n")
		sb.WriteString("You have access to the following tools. Use them when appropriate.\n")
		sb.WriteString("다음 도구들을 사용할 수 있습니다. 필요한 경우 사용하세요.\n\n")

		for _, tool := range req.ToolDefinitions {
			sb.WriteString(fmt.Sprintf("Tool name / 도구 이름: %s\n", tool.Name))
			if tool.Description != "" {
				sb.WriteString(fmt.Sprintf("Description / 설명: %s\n", tool.Description))
			}
			if len(tool.InputSchema) > 0 && string(tool.InputSchema) != "null" {
				sb.WriteString("Parameters / 파라미터:\n")
				// 스키마를 들여쓰기하여 읽기 쉽게 출력
				var schemaObj interface{}
				if err := json.Unmarshal(tool.InputSchema, &schemaObj); err == nil {
					if pretty, err := json.MarshalIndent(schemaObj, "  ", "  "); err == nil {
						sb.WriteString("  ")
						sb.Write(pretty)
						sb.WriteString("\n")
					} else {
						sb.WriteString("  ")
						sb.Write(tool.InputSchema)
						sb.WriteString("\n")
					}
				} else {
					sb.WriteString("  ")
					sb.Write(tool.InputSchema)
					sb.WriteString("\n")
				}
			}
			sb.WriteString("\n")
		}
	}

	// 3. 도구 호출 출력 형식 지침
	sb.WriteString("=== TOOL CALL FORMAT / 도구 호출 형식 ===\n")
	sb.WriteString("CRITICAL INSTRUCTIONS / 중요 지침:\n\n")
	sb.WriteString("When you need to call a tool, output EXACTLY this format and nothing else on those lines:\n")
	sb.WriteString("도구를 호출해야 할 때, 다음 형식을 정확하게 출력하세요:\n\n")
	sb.WriteString("<<<TOOL_CALL>>>\n")
	sb.WriteString(`{"name": "tool_name", "arguments": {"param1": "value1", "param2": "value2"}}`)
	sb.WriteString("\n<<<END_TOOL_CALL>>>\n\n")
	sb.WriteString("Rules / 규칙:\n")
	sb.WriteString("- The JSON must be on a single line between the markers\n")
	sb.WriteString("- JSON은 마커 사이에 한 줄로 작성해야 합니다\n")
	sb.WriteString("- 'arguments' must match the tool's parameter schema exactly\n")
	sb.WriteString("- 'arguments'는 도구의 파라미터 스키마와 정확히 일치해야 합니다\n")
	sb.WriteString("- You can call multiple tools by outputting multiple blocks\n")
	sb.WriteString("- 여러 블록을 출력하여 여러 도구를 호출할 수 있습니다\n")
	sb.WriteString("- If you do NOT need to call a tool, just respond normally with text\n")
	sb.WriteString("- 도구를 호출할 필요가 없는 경우, 일반 텍스트로 응답하세요\n\n")

	// 4. 대화 이력
	if len(req.ToolLoopMessages) > 0 {
		sb.WriteString("=== CONVERSATION HISTORY / 대화 이력 ===\n")
		for _, msg := range req.ToolLoopMessages {
			switch msg.Role {
			case "user":
				sb.WriteString("USER: ")
				sb.WriteString(msg.Content)
				sb.WriteString("\n\n")
			case "assistant":
				sb.WriteString("ASSISTANT: ")
				if msg.Content != "" {
					sb.WriteString(msg.Content)
				}
				// 이전 도구 호출 기록 복원
				for _, tc := range msg.ToolCalls {
					sb.WriteString("\n<<<TOOL_CALL>>>\n")
					payload := cliToolCallPayload{Name: tc.Name, Arguments: tc.Input}
					if data, err := json.Marshal(payload); err == nil {
						sb.Write(data)
					}
					sb.WriteString("\n<<<END_TOOL_CALL>>>")
				}
				sb.WriteString("\n\n")
			case "tool":
				// 도구 실행 결과
				sb.WriteString("TOOL RESULTS / 도구 실행 결과:\n")
				for _, tr := range msg.ToolResults {
					sb.WriteString(fmt.Sprintf("[%s]: %s\n", tr.ToolName, tr.Content))
					if tr.IsError {
						sb.WriteString("  (ERROR)\n")
					}
				}
				sb.WriteString("\n")
			}
		}
	}

	// 마지막 사용자 메시지가 없고 req.Prompt가 있으면 추가
	if req.Prompt != "" {
		// ToolLoopMessages에 이미 포함된 경우 중복 방지
		hasUserPrompt := false
		for _, msg := range req.ToolLoopMessages {
			if msg.Role == "user" && msg.Content == req.Prompt {
				hasUserPrompt = true
				break
			}
		}
		if !hasUserPrompt {
			sb.WriteString("USER: ")
			sb.WriteString(req.Prompt)
			sb.WriteString("\n\n")
		}
	}

	sb.WriteString("ASSISTANT: ")

	return sb.String()
}

// parseCLIToolCalls는 CLI 텍스트 출력에서 도구 호출을 추출합니다.
// <<<TOOL_CALL>>> ... <<<END_TOOL_CALL>>> 마커를 찾아 JSON을 파싱합니다.
// 파싱에 실패한 블록은 경고 로그를 남기고 건너뜁니다.
//
// 반환값:
// - []ToolCall: 파싱된 도구 호출 목록 (없으면 빈 슬라이스)
// - string: 도구 호출 마커를 제거한 나머지 텍스트
func parseCLIToolCalls(output string) ([]ToolCall, string) {
	var toolCalls []ToolCall
	var remainingParts []string

	// 마커로 텍스트를 분리하여 순서대로 처리
	remaining := output
	callIndex := 0

	for {
		startIdx := strings.Index(remaining, cliToolCallMarkerStart)
		if startIdx == -1 {
			// 더 이상 마커가 없으면 나머지 텍스트를 보존
			if strings.TrimSpace(remaining) != "" {
				remainingParts = append(remainingParts, remaining)
			}
			break
		}

		// 마커 이전 텍스트 보존
		before := remaining[:startIdx]
		if strings.TrimSpace(before) != "" {
			remainingParts = append(remainingParts, before)
		}

		// 마커 이후에서 END 마커 탐색
		afterStart := remaining[startIdx+len(cliToolCallMarkerStart):]
		endIdx := strings.Index(afterStart, cliToolCallMarkerEnd)
		if endIdx == -1 {
			// END 마커가 없으면 나머지를 텍스트로 처리
			log.Printf("[CLIToolLoop] TOOL_CALL 블록이 닫히지 않음, 텍스트로 처리")
			remainingParts = append(remainingParts, remaining[startIdx:])
			break
		}

		// JSON 블록 추출 및 파싱
		jsonBlock := strings.TrimSpace(afterStart[:endIdx])
		var payload cliToolCallPayload
		if err := json.Unmarshal([]byte(jsonBlock), &payload); err != nil {
			log.Printf("[CLIToolLoop] 도구 호출 JSON 파싱 실패: %v, 블록: %s", err, jsonBlock)
		} else if payload.Name != "" {
			// 유효한 도구 호출인 경우 ToolCall로 변환
			toolCall := ToolCall{
				ID:    fmt.Sprintf("call_%d", callIndex),
				Name:  payload.Name,
				Input: payload.Arguments,
			}
			// Arguments가 nil이면 빈 객체로 초기화
			if toolCall.Input == nil {
				toolCall.Input = json.RawMessage("{}")
			}
			toolCalls = append(toolCalls, toolCall)
			callIndex++
		} else {
			log.Printf("[CLIToolLoop] 도구 이름이 없는 호출 블록 무시: %s", jsonBlock)
		}

		// END 마커 이후로 이동
		remaining = afterStart[endIdx+len(cliToolCallMarkerEnd):]
	}

	// 나머지 텍스트 결합
	remainingText := strings.TrimSpace(strings.Join(remainingParts, ""))

	return toolCalls, remainingText
}

// buildToolLoopExecuteRequest는 tool_loop 요청을 CLI 실행용 단순 요청으로 변환합니다.
// 도구 정의와 대화 이력을 프롬프트에 포함시킵니다.
func buildToolLoopExecuteRequest(req ExecuteRequest) ExecuteRequest {
	return ExecuteRequest{
		Prompt:       buildCLIToolLoopPrompt(req),
		Model:        req.Model,
		MaxTokens:    req.MaxTokens,
		WorkDir:      req.WorkDir,
		SystemPrompt: "", // buildCLIToolLoopPrompt에서 이미 포함됨
		ResponseMode: "", // 재귀 방지: tool_loop 모드 제거
	}
}

// wrapToolLoopResponse는 CLI 텍스트 출력을 tool_loop 응답으로 래핑합니다.
// 도구 호출이 감지되면 StopReason을 "tool_use"로 설정합니다.
func wrapToolLoopResponse(resp *ExecuteResponse, rawOutput string) *ExecuteResponse {
	toolCalls, remainingText := parseCLIToolCalls(rawOutput)

	result := *resp // 복사
	result.ToolCalls = toolCalls

	if len(toolCalls) > 0 {
		result.StopReason = "tool_use"
		result.Output = remainingText
	} else {
		result.StopReason = "end_turn"
		result.Output = rawOutput
	}

	return &result
}

// buildToolLoopPromptForCodex는 Codex CLI 전용 tool_loop 프롬프트를 생성합니다.
// Codex CLI는 단순 프롬프트 문자열을 받으므로 시스템 프롬프트와 대화 이력을
// 하나의 프롬프트로 합칩니다.
func buildToolLoopPromptForCodex(req ExecuteRequest) string {
	// 일반 CLI tool loop 프롬프트와 동일하게 처리
	return buildCLIToolLoopPrompt(req)
}

// buildToolLoopPromptForClaude는 Claude CLI 전용 tool_loop 프롬프트를 생성합니다.
// Claude CLI는 --system-prompt 플래그를 지원하므로 시스템 프롬프트를 분리합니다.
// 반환값: (mainPrompt, systemPrompt)
func buildToolLoopPromptForClaude(req ExecuteRequest) (string, string) {
	// 시스템 프롬프트에 도구 정의와 형식 지침 포함
	var sysSb strings.Builder
	if req.SystemPrompt != "" {
		sysSb.WriteString(req.SystemPrompt)
		sysSb.WriteString("\n\n")
	}

	// 도구 정의를 시스템 프롬프트에 추가
	if len(req.ToolDefinitions) > 0 {
		sysSb.WriteString("=== AVAILABLE TOOLS / 사용 가능한 도구 ===\n")
		sysSb.WriteString("You have access to the following tools. Use them when appropriate.\n")
		sysSb.WriteString("다음 도구들을 사용할 수 있습니다. 필요한 경우 사용하세요.\n\n")

		for _, tool := range req.ToolDefinitions {
			sysSb.WriteString(fmt.Sprintf("Tool: %s\n", tool.Name))
			if tool.Description != "" {
				sysSb.WriteString(fmt.Sprintf("Description: %s\n", tool.Description))
			}
			if len(tool.InputSchema) > 0 && string(tool.InputSchema) != "null" {
				var schemaObj interface{}
				if err := json.Unmarshal(tool.InputSchema, &schemaObj); err == nil {
					if pretty, err := json.MarshalIndent(schemaObj, "  ", "  "); err == nil {
						sysSb.WriteString("Parameters:\n  ")
						sysSb.Write(pretty)
						sysSb.WriteString("\n")
					}
				}
			}
			sysSb.WriteString("\n")
		}
	}

	// 도구 호출 형식 지침
	sysSb.WriteString("=== TOOL CALL FORMAT / 도구 호출 형식 ===\n")
	sysSb.WriteString("When calling a tool, output EXACTLY:\n")
	sysSb.WriteString("도구 호출 시 정확히 다음 형식으로 출력:\n\n")
	sysSb.WriteString("<<<TOOL_CALL>>>\n")
	sysSb.WriteString(`{"name": "tool_name", "arguments": {"param": "value"}}`)
	sysSb.WriteString("\n<<<END_TOOL_CALL>>>\n\n")
	sysSb.WriteString("- JSON must be on a single line between the markers\n")
	sysSb.WriteString("- Multiple tools: output multiple blocks\n")
	sysSb.WriteString("- No tool needed: respond with normal text only\n")

	systemPrompt := sysSb.String()

	// 메인 프롬프트: 대화 이력만 포함
	var mainSb strings.Builder
	if len(req.ToolLoopMessages) > 0 {
		for _, msg := range req.ToolLoopMessages {
			switch msg.Role {
			case "user":
				mainSb.WriteString("USER: ")
				mainSb.WriteString(msg.Content)
				mainSb.WriteString("\n\n")
			case "assistant":
				mainSb.WriteString("ASSISTANT: ")
				if msg.Content != "" {
					mainSb.WriteString(msg.Content)
				}
				for _, tc := range msg.ToolCalls {
					mainSb.WriteString("\n<<<TOOL_CALL>>>\n")
					payload := cliToolCallPayload{Name: tc.Name, Arguments: tc.Input}
					if data, err := json.Marshal(payload); err == nil {
						mainSb.Write(data)
					}
					mainSb.WriteString("\n<<<END_TOOL_CALL>>>")
				}
				mainSb.WriteString("\n\n")
			case "tool":
				mainSb.WriteString("TOOL RESULTS:\n")
				for _, tr := range msg.ToolResults {
					mainSb.WriteString(fmt.Sprintf("[%s]: %s\n", tr.ToolName, tr.Content))
				}
				mainSb.WriteString("\n")
			}
		}
	}

	if req.Prompt != "" {
		mainSb.WriteString(req.Prompt)
	}

	return mainSb.String(), systemPrompt
}

