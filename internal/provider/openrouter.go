// Package provider는 AI 프로바이더 통합 레이어를 제공합니다.
package provider

import "strings"

// OpenRouter 형식 프로바이더 접두사 매핑 (내부 이름 -> OpenRouter 접두사)
var providerToOpenRouter = map[string]string{
	"claude": "anthropic",
	"codex":  "openai",
	"gemini": "google",
}

// OpenRouter 접두사 -> 내부 프로바이더 이름 매핑
var openRouterToProvider = map[string]string{
	"anthropic": "claude",
	"openai":    "codex",
	"google":    "gemini",
}

// IsOpenRouterFormat은 모델 ID가 OpenRouter 형식(provider/model)인지 확인합니다.
func IsOpenRouterFormat(modelID string) bool {
	return strings.Contains(modelID, "/")
}

// ParseOpenRouterID는 OpenRouter 형식 모델 ID를 프로바이더 접두사와 모델명으로 분리합니다.
// 예: "openai/o3-mini" -> ("openai", "o3-mini")
// 레거시 형식이면 ("", modelID)를 반환합니다.
func ParseOpenRouterID(modelID string) (prefix, model string) {
	parts := strings.SplitN(modelID, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", modelID
}

// ResolveProviderName은 OpenRouter 접두사에서 내부 프로바이더 이름을 반환합니다.
// 예: "openai" -> "codex", "anthropic" -> "claude"
// 알 수 없는 접두사면 빈 문자열을 반환합니다.
func ResolveProviderName(openRouterPrefix string) string {
	return openRouterToProvider[openRouterPrefix]
}

// StripProviderPrefix는 OpenRouter 형식 모델 ID에서 프로바이더 접두사를 제거합니다.
// 레거시 형식이면 그대로 반환합니다.
// 예: "openai/o3-mini" -> "o3-mini", "o3-mini" -> "o3-mini"
func StripProviderPrefix(modelID string) string {
	_, model := ParseOpenRouterID(modelID)
	return model
}
