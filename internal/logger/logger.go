// Package logger는 구조화된 로깅을 제공합니다.
// REQ-U-01: 시스템은 항상 구조화된 JSON 로그를 출력해야 한다
// REQ-U-02: 시스템은 항상 민감 정보를 마스킹하여 로그에 기록해야 한다
package logger

import (
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/insajin/autopus-bridge/internal/config"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// 민감 정보 패턴 (REQ-U-02)
var sensitivePatterns = []*regexp.Regexp{
	// API 키 패턴 (sk-ant-*, AIza*, etc.)
	regexp.MustCompile(`(sk-ant-[a-zA-Z0-9\-_]{20,})`),
	regexp.MustCompile(`(AIza[a-zA-Z0-9\-_]{30,})`),
	regexp.MustCompile(`(sk-[a-zA-Z0-9]{20,})`),
	// JWT 토큰 패턴 (eyJ로 시작하는 Base64)
	regexp.MustCompile(`(eyJ[a-zA-Z0-9\-_]+\.eyJ[a-zA-Z0-9\-_]+\.[a-zA-Z0-9\-_]+)`),
	// Bearer 토큰
	regexp.MustCompile(`(Bearer\s+[a-zA-Z0-9\-_\.]+)`),
	// 일반 API 키 패턴 (api_key=, apikey=, key= 등)
	regexp.MustCompile(`((?:api[_-]?key|apikey|key|token|secret|password)\s*[=:]\s*)([a-zA-Z0-9\-_\.]{10,})`),
}

// maskedWriter는 민감 정보를 마스킹하는 io.Writer입니다.
type maskedWriter struct {
	underlying io.Writer
}

// Write는 민감 정보를 마스킹한 후 기록합니다.
func (w *maskedWriter) Write(p []byte) (n int, err error) {
	masked := MaskSensitive(string(p))
	return w.underlying.Write([]byte(masked))
}

// Setup은 로거를 초기화합니다.
func Setup(cfg config.LoggingConfig) {
	// 로그 레벨 설정
	level := parseLevel(cfg.Level)
	zerolog.SetGlobalLevel(level)

	// 타임스탬프 포맷 설정 (RFC3339)
	zerolog.TimeFieldFormat = time.RFC3339

	// 출력 대상 설정
	var output io.Writer = os.Stdout
	if cfg.File != "" {
		file, err := os.OpenFile(cfg.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			// 파일 열기 실패 시 stdout 사용
			log.Warn().Err(err).Str("file", cfg.File).Msg("로그 파일을 열 수 없어 stdout을 사용합니다")
		} else {
			output = file
		}
	}

	// 민감 정보 마스킹 Writer 래핑
	maskedOutput := &maskedWriter{underlying: output}

	// 포맷 설정
	if cfg.Format == "text" {
		// 콘솔 포맷 (개발 시 가독성)
		consoleWriter := zerolog.ConsoleWriter{
			Out:        maskedOutput,
			TimeFormat: time.RFC3339,
		}
		log.Logger = zerolog.New(consoleWriter).With().Timestamp().Caller().Logger()
	} else {
		// JSON 포맷 (기본값, REQ-U-01)
		log.Logger = zerolog.New(maskedOutput).With().Timestamp().Caller().Logger()
	}
}

// parseLevel은 문자열 레벨을 zerolog.Level로 변환합니다.
func parseLevel(level string) zerolog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}

// MaskSensitive는 문자열에서 민감 정보를 마스킹합니다.
// REQ-U-02: 민감 정보(API 키, JWT 토큰) 마스킹
func MaskSensitive(input string) string {
	result := input
	for _, pattern := range sensitivePatterns {
		result = pattern.ReplaceAllStringFunc(result, func(match string) string {
			// 키-값 패턴 처리 (api_key=xxx 형태)
			if strings.Contains(match, "=") || strings.Contains(match, ":") {
				parts := regexp.MustCompile(`[=:]`).Split(match, 2)
				if len(parts) == 2 {
					prefix := parts[0] + string(match[len(parts[0])])
					value := strings.TrimSpace(parts[1])
					return prefix + maskValue(value)
				}
			}
			// Bearer 토큰 처리
			if strings.HasPrefix(match, "Bearer ") {
				return "Bearer " + maskValue(strings.TrimPrefix(match, "Bearer "))
			}
			// 일반 토큰/키 마스킹
			return maskValue(match)
		})
	}
	return result
}

// maskValue는 값을 마스킹합니다.
// 앞 4자와 뒤 4자만 남기고 나머지는 ***로 대체합니다.
func maskValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 8 {
		return "***"
	}
	return value[:4] + "***" + value[len(value)-4:]
}

// Debug는 디버그 레벨 로그를 기록합니다.
func Debug() *zerolog.Event {
	return log.Debug()
}

// Info는 정보 레벨 로그를 기록합니다.
func Info() *zerolog.Event {
	return log.Info()
}

// Warn은 경고 레벨 로그를 기록합니다.
func Warn() *zerolog.Event {
	return log.Warn()
}

// Error는 오류 레벨 로그를 기록합니다.
func Error() *zerolog.Event {
	return log.Error()
}

// Fatal은 치명적 오류 레벨 로그를 기록하고 프로그램을 종료합니다.
func Fatal() *zerolog.Event {
	return log.Fatal()
}

// WithContext는 컨텍스트 필드를 추가한 새 로거를 반환합니다.
func WithContext(ctx map[string]interface{}) zerolog.Logger {
	l := log.With()
	for k, v := range ctx {
		l = l.Interface(k, v)
	}
	return l.Logger()
}

// WithExecutionID는 실행 ID를 컨텍스트에 추가한 로거를 반환합니다.
func WithExecutionID(executionID string) zerolog.Logger {
	return log.With().Str("execution_id", executionID).Logger()
}

// WithTaskContext는 작업 관련 컨텍스트를 추가한 로거를 반환합니다.
func WithTaskContext(executionID, model string) zerolog.Logger {
	return log.With().
		Str("execution_id", executionID).
		Str("model", model).
		Logger()
}
