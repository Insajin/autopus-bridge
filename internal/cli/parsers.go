package cli

// CLI 실행 결과 출력 파서.
// SPEC-SKILL-V2-001 Block C: go_test_json, tap, json 파서
//
// 지원 포맷:
// - go_test_json: `go test -json` 출력 파싱 (테스트 결과 구조화)
// - tap: TAP (Test Anything Protocol) 출력 파싱
// - json: 일반 JSON 출력 파싱

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/insajin/autopus-agent-protocol"
)

// Parser는 CLI stdout을 구조화된 결과로 파싱하는 인터페이스입니다.
type Parser interface {
	Parse(stdout string) (*ws.CLIParsedResult, error)
}

// defaultParsers는 기본 내장 파서 맵을 반환합니다.
func defaultParsers() map[string]Parser {
	return map[string]Parser{
		"go_test_json": &GoTestJSONParser{},
		"tap":          &TAPParser{},
		"json":         &JSONOutputParser{},
	}
}

// --- GoTestJSONParser ---
// `go test -json` 출력을 파싱하여 테스트 결과를 구조화합니다.
// 각 줄은 JSON 이벤트로, Action 필드에 따라 pass/fail/skip/output을 추적합니다.

// GoTestJSONParser는 go test -json 출력을 파싱합니다.
type GoTestJSONParser struct{}

// goTestEvent는 go test -json이 출력하는 개별 이벤트 구조체입니다.
type goTestEvent struct {
	Action  string  `json:"Action"`
	Package string  `json:"Package"`
	Test    string  `json:"Test"`
	Output  string  `json:"Output"`
	Elapsed float64 `json:"Elapsed"`
}

// Parse는 go test -json 출력을 파싱하여 CLIParsedResult를 반환합니다.
func (p *GoTestJSONParser) Parse(stdout string) (*ws.CLIParsedResult, error) {
	result := &ws.CLIParsedResult{}

	// 테스트별 출력 추적 (실패 시 출력 포함을 위해)
	testOutputs := make(map[string][]string)

	scanner := bufio.NewScanner(strings.NewReader(stdout))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var event goTestEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			// JSON이 아닌 줄은 건너뜀 (go test 출력에 비-JSON 줄이 섞일 수 있음)
			continue
		}

		key := event.Package + "/" + event.Test

		switch event.Action {
		case "pass":
			if event.Test != "" {
				result.Passed++
				result.Total++
			}
		case "fail":
			if event.Test != "" {
				result.Failed++
				result.Total++
				result.Failures = append(result.Failures, ws.CLITestFailure{
					Test:    event.Test,
					Package: event.Package,
					Output:  strings.Join(testOutputs[key], ""),
				})
			}
		case "skip":
			if event.Test != "" {
				result.Skipped++
				result.Total++
			}
		case "output":
			if event.Test != "" {
				testOutputs[key] = append(testOutputs[key], event.Output)
			}
		}
	}

	result.Summary = fmt.Sprintf("total=%d passed=%d failed=%d skipped=%d",
		result.Total, result.Passed, result.Failed, result.Skipped)

	return result, nil
}

// --- TAPParser ---
// TAP (Test Anything Protocol) 출력을 파싱합니다.
// "ok" 접두사는 통과, "not ok" 접두사는 실패로 분류합니다.
// "# SKIP" 주석이 포함된 "ok"는 건너뛴 테스트로 분류합니다.

// TAPParser는 TAP 형식 출력을 파싱합니다.
type TAPParser struct{}

// Parse는 TAP 출력을 파싱하여 CLIParsedResult를 반환합니다.
func (p *TAPParser) Parse(stdout string) (*ws.CLIParsedResult, error) {
	result := &ws.CLIParsedResult{}

	scanner := bufio.NewScanner(strings.NewReader(stdout))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "ok ") {
			result.Passed++
			result.Total++
			// "# SKIP" 또는 "# skip" 주석이 있으면 건너뛴 테스트로 재분류
			if strings.Contains(line, "# SKIP") || strings.Contains(line, "# skip") {
				result.Passed--
				result.Skipped++
			}
		} else if strings.HasPrefix(line, "not ok ") {
			result.Failed++
			result.Total++
			// 테스트 이름 추출
			testName := strings.TrimPrefix(line, "not ok ")
			// 테스트 번호 접두사 제거 (예: "1 test_name" -> "test_name")
			parts := strings.SplitN(testName, " ", 2)
			if len(parts) > 1 {
				testName = parts[1]
			}
			result.Failures = append(result.Failures, ws.CLITestFailure{
				Test:   testName,
				Output: line,
			})
		}
	}

	result.Summary = fmt.Sprintf("total=%d passed=%d failed=%d skipped=%d",
		result.Total, result.Passed, result.Failed, result.Skipped)

	return result, nil
}

// --- JSONOutputParser ---
// 일반 JSON 출력을 파싱합니다.
// 전체 stdout을 JSON으로 파싱하고, 요약을 pretty-print하여 제공합니다.

// JSONOutputParser는 범용 JSON 출력을 파싱합니다.
type JSONOutputParser struct{}

// Parse는 JSON 출력을 파싱하여 CLIParsedResult를 반환합니다.
// stdout 전체가 유효한 JSON이어야 합니다.
func (p *JSONOutputParser) Parse(stdout string) (*ws.CLIParsedResult, error) {
	result := &ws.CLIParsedResult{}

	// 전체 출력을 JSON으로 파싱
	var parsed interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &parsed); err != nil {
		return nil, fmt.Errorf("유효한 JSON이 아님: %w", err)
	}

	// 요약: pretty-print된 JSON의 처음 500자
	pretty, _ := json.MarshalIndent(parsed, "", "  ")
	summary := string(pretty)
	if len(summary) > 500 {
		summary = summary[:500] + "..."
	}
	result.Summary = summary

	return result, nil
}
