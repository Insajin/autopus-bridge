package protocol_test

import (
	"testing"

	"github.com/insajin/autopus-codex-rpc/protocol"
)

// TestNewErrorStringConstants는 새로 추가된 Codex 에러 문자열 상수를 검증한다.
func TestNewErrorStringConstants(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want string
	}{
		{"ErrResponseStreamConnectionFailed", protocol.CodexErrResponseStreamConnectionFailed, "ResponseStreamConnectionFailed"},
		{"ErrResponseStreamDisconnected", protocol.CodexErrResponseStreamDisconnected, "ResponseStreamDisconnected"},
		{"ErrResponseTooManyFailedAttempts", protocol.CodexErrResponseTooManyFailedAttempts, "ResponseTooManyFailedAttempts"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.val != tt.want {
				t.Errorf("에러 상수 불일치: got %q, want %q", tt.val, tt.want)
			}
			if tt.val == "" {
				t.Errorf("에러 상수 %s가 비어 있으면 안 됨", tt.name)
			}
		})
	}
}

// TestNewErrorCodeConstants는 새로 추가된 서버 과부하 에러 코드 상수를 검증한다.
func TestNewErrorCodeConstants(t *testing.T) {
	t.Run("ErrCodeServerOverloaded 값 검증", func(t *testing.T) {
		// 서버 과부하 에러 코드는 -32001이 아닌 별도 값이어야 함
		// 기존 ErrCodeContextWindowExceeded = -32001과 충돌하지 않아야 함
		if protocol.ErrCodeServerOverloaded == 0 {
			t.Error("ErrCodeServerOverloaded가 0이면 안 됨")
		}
		// JSON-RPC 서버 에러 범위는 -32099 ~ -32000
		if protocol.ErrCodeServerOverloaded > -32000 || protocol.ErrCodeServerOverloaded < -32099 {
			t.Errorf("ErrCodeServerOverloaded가 서버 에러 범위(-32099 ~ -32000)를 벗어남: %d", protocol.ErrCodeServerOverloaded)
		}
	})
}

// TestExistingErrorConstants는 기존 에러 상수가 변경되지 않았음을 검증한다 (회귀 테스트).
func TestExistingErrorConstants_Regression(t *testing.T) {
	t.Run("기존 Codex 에러 문자열 상수 유지", func(t *testing.T) {
		tests := []struct {
			name string
			val  string
			want string
		}{
			{"CodexErrContextWindowExceeded", protocol.CodexErrContextWindowExceeded, "ContextWindowExceeded"},
			{"CodexErrUsageLimitExceeded", protocol.CodexErrUsageLimitExceeded, "UsageLimitExceeded"},
			{"CodexErrUnauthorized", protocol.CodexErrUnauthorized, "Unauthorized"},
			{"CodexErrHttpConnectionFailed", protocol.CodexErrHttpConnectionFailed, "HttpConnectionFailed"},
			{"CodexErrBadRequest", protocol.CodexErrBadRequest, "BadRequest"},
			{"CodexErrSandboxError", protocol.CodexErrSandboxError, "SandboxError"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if tt.val != tt.want {
					t.Errorf("기존 에러 상수 변경됨: got %q, want %q", tt.val, tt.want)
				}
			})
		}
	})

	t.Run("기존 에러 코드 정수 상수 유지", func(t *testing.T) {
		tests := []struct {
			name string
			code int
			want int
		}{
			{"ErrCodeParseError", protocol.ErrCodeParseError, -32700},
			{"ErrCodeInvalidRequest", protocol.ErrCodeInvalidRequest, -32600},
			{"ErrCodeMethodNotFound", protocol.ErrCodeMethodNotFound, -32601},
			{"ErrCodeInvalidParams", protocol.ErrCodeInvalidParams, -32602},
			{"ErrCodeInternalError", protocol.ErrCodeInternalError, -32603},
			{"ErrCodeContextWindowExceeded", protocol.ErrCodeContextWindowExceeded, -32001},
			{"ErrCodeUsageLimitExceeded", protocol.ErrCodeUsageLimitExceeded, -32002},
			{"ErrCodeUnauthorized", protocol.ErrCodeUnauthorized, -32003},
			{"ErrCodeConnectionFailed", protocol.ErrCodeConnectionFailed, -32004},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if tt.code != tt.want {
					t.Errorf("기존 에러 코드 변경됨: got %d, want %d", tt.code, tt.want)
				}
			})
		}
	})
}
