package protocol_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/insajin/autopus-codex-rpc/protocol"
)

// TestJSONRPCError_ErrorInterface는 JSONRPCError가 error 인터페이스를 구현하는지 검증한다.
func TestJSONRPCError_ErrorInterface(t *testing.T) {
	rpcErr := &protocol.JSONRPCError{
		Code:    -32600,
		Message: "Invalid Request",
	}

	// error 인터페이스를 구현해야 함
	var err error = rpcErr
	if err == nil {
		t.Fatal("JSONRPCError가 error 인터페이스를 구현해야 함")
	}

	errStr := err.Error()
	if errStr == "" {
		t.Error("Error() 반환값이 비어 있으면 안 됨")
	}

	// 에러 코드와 메시지가 포함되어야 함
	if !strings.Contains(errStr, "-32600") || !strings.Contains(errStr, "Invalid Request") {
		t.Errorf("Error() 반환값에 코드와 메시지가 포함되어야 함: %q", errStr)
	}
}

// TestMapJSONRPCError는 JSON-RPC 에러 매핑을 검증한다.
func TestMapJSONRPCError(t *testing.T) {
	t.Run("nil 에러는 nil 반환", func(t *testing.T) {
		result := protocol.MapJSONRPCError(nil)
		if result != nil {
			t.Errorf("nil 입력에 nil이 반환되어야 함, got: %v", result)
		}
	})

	t.Run("컨텍스트 윈도우 초과 에러 매핑", func(t *testing.T) {
		rpcErr := &protocol.JSONRPCError{
			Code:    protocol.ErrCodeContextWindowExceeded,
			Message: "ContextWindowExceeded: 토큰 한도 초과",
		}
		result := protocol.MapJSONRPCError(rpcErr)
		if result == nil {
			t.Fatal("결과가 nil이면 안 됨")
		}
		if !strings.Contains(result.Error(), "컨텍스트") || !strings.Contains(result.Error(), "초과") {
			t.Errorf("에러 메시지에 컨텍스트 초과 내용 포함 필요: %v", result)
		}
	})

	t.Run("사용량 제한 초과 에러 매핑", func(t *testing.T) {
		rpcErr := &protocol.JSONRPCError{
			Code:    protocol.ErrCodeUsageLimitExceeded,
			Message: "UsageLimitExceeded: 사용량 한도 초과",
		}
		result := protocol.MapJSONRPCError(rpcErr)
		if result == nil {
			t.Fatal("결과가 nil이면 안 됨")
		}
	})

	t.Run("인증 실패 에러 매핑", func(t *testing.T) {
		rpcErr := &protocol.JSONRPCError{
			Code:    protocol.ErrCodeUnauthorized,
			Message: "Unauthorized",
		}
		result := protocol.MapJSONRPCError(rpcErr)
		if result == nil {
			t.Fatal("결과가 nil이면 안 됨")
		}
	})

	t.Run("연결 실패 에러 매핑", func(t *testing.T) {
		rpcErr := &protocol.JSONRPCError{
			Code:    protocol.ErrCodeConnectionFailed,
			Message: "Connection failed",
		}
		result := protocol.MapJSONRPCError(rpcErr)
		if result == nil {
			t.Fatal("결과가 nil이면 안 됨")
		}
	})

	t.Run("알 수 없는 에러 코드는 JSONRPCError 반환", func(t *testing.T) {
		rpcErr := &protocol.JSONRPCError{
			Code:    -99999,
			Message: "Unknown error",
		}
		result := protocol.MapJSONRPCError(rpcErr)
		if result == nil {
			t.Fatal("결과가 nil이면 안 됨")
		}
		// 알 수 없는 에러는 JSONRPCError 자체를 반환
		var jsonRPCErr *protocol.JSONRPCError
		if !errors.As(result, &jsonRPCErr) {
			t.Errorf("알 수 없는 에러는 *JSONRPCError 타입이어야 함, got: %T", result)
		}
	})
}

// TestErrorCodes는 에러 코드 상수 값을 검증한다.
func TestErrorCodes(t *testing.T) {
	tests := []struct {
		name string
		code int
		want int
	}{
		{"ParseError", protocol.ErrCodeParseError, -32700},
		{"InvalidRequest", protocol.ErrCodeInvalidRequest, -32600},
		{"MethodNotFound", protocol.ErrCodeMethodNotFound, -32601},
		{"InvalidParams", protocol.ErrCodeInvalidParams, -32602},
		{"InternalError", protocol.ErrCodeInternalError, -32603},
		{"ContextWindowExceeded", protocol.ErrCodeContextWindowExceeded, -32001},
		{"UsageLimitExceeded", protocol.ErrCodeUsageLimitExceeded, -32002},
		{"Unauthorized", protocol.ErrCodeUnauthorized, -32003},
		{"ConnectionFailed", protocol.ErrCodeConnectionFailed, -32004},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.code != tt.want {
				t.Errorf("에러 코드 불일치: got %d, want %d", tt.code, tt.want)
			}
		})
	}
}

// TestCodexErrorStringConstants는 Codex 에러 문자열 상수를 검증한다.
func TestCodexErrorStringConstants(t *testing.T) {
	tests := []struct {
		name string
		val  string
	}{
		{"ContextWindowExceeded", protocol.CodexErrContextWindowExceeded},
		{"UsageLimitExceeded", protocol.CodexErrUsageLimitExceeded},
		{"Unauthorized", protocol.CodexErrUnauthorized},
		{"HttpConnectionFailed", protocol.CodexErrHttpConnectionFailed},
		{"BadRequest", protocol.CodexErrBadRequest},
		{"SandboxError", protocol.CodexErrSandboxError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.val == "" {
				t.Errorf("상수 %s가 비어 있으면 안 됨", tt.name)
			}
		})
	}
}
