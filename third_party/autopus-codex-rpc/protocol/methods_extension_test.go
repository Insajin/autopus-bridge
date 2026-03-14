package protocol_test

import (
	"testing"

	"github.com/insajin/autopus-codex-rpc/protocol"
)

// TestNewMethodConstants는 새로 추가된 메서드 상수 값을 검증한다.
func TestNewMethodConstants(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want string
	}{
		// turn 관련 메서드
		{"MethodTurnSteer", protocol.MethodTurnSteer, "turn/steer"},

		// thread 관련 메서드
		{"MethodThreadFork", protocol.MethodThreadFork, "thread/fork"},
		{"MethodThreadRead", protocol.MethodThreadRead, "thread/read"},
		{"MethodThreadList", protocol.MethodThreadList, "thread/list"},
		{"MethodThreadRollback", protocol.MethodThreadRollback, "thread/rollback"},

		// review 관련 메서드
		{"MethodReviewStart", protocol.MethodReviewStart, "review/start"},

		// model 관련 메서드
		{"MethodModelList", protocol.MethodModelList, "model/list"},

		// config 관련 메서드
		{"MethodConfigRead", protocol.MethodConfigRead, "config/read"},
		{"MethodConfigValueWrite", protocol.MethodConfigValueWrite, "config/value/write"},
		{"MethodConfigBatchWrite", protocol.MethodConfigBatchWrite, "config/batchWrite"},

		// skills 관련 메서드
		{"MethodSkillsList", protocol.MethodSkillsList, "skills/list"},

		// mcp 서버 상태 메서드
		{"MethodMcpServerStatusList", protocol.MethodMcpServerStatusList, "mcpServerStatus/list"},

		// 실험적 기능 메서드
		{"MethodExperimentalFeatureList", protocol.MethodExperimentalFeatureList, "experimentalFeature/list"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.val != tt.want {
				t.Errorf("메서드 상수 불일치: got %q, want %q", tt.val, tt.want)
			}
			// 빈 값은 허용하지 않음
			if tt.val == "" {
				t.Errorf("메서드 상수 %s가 비어 있으면 안 됨", tt.name)
			}
		})
	}
}

// TestNotificationMethodConstants는 알림 메서드 상수 값을 검증한다.
func TestNotificationMethodConstants(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want string
	}{
		{"MethodTurnPlanUpdated", protocol.MethodTurnPlanUpdated, "turn/plan/updated"},
		{"MethodTurnDiffUpdated", protocol.MethodTurnDiffUpdated, "turn/diff/updated"},
		{"MethodItemReasoningSummaryTextDelta", protocol.MethodItemReasoningSummaryTextDelta, "item/reasoning/summaryTextDelta"},
		{"MethodItemReasoningTextDelta", protocol.MethodItemReasoningTextDelta, "item/reasoning/textDelta"},
		{"MethodThreadStatusChanged", protocol.MethodThreadStatusChanged, "thread/status/changed"},
		{"MethodThreadTokenUsageUpdated", protocol.MethodThreadTokenUsageUpdated, "thread/tokenUsage/updated"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.val != tt.want {
				t.Errorf("알림 메서드 상수 불일치: got %q, want %q", tt.val, tt.want)
			}
		})
	}
}

// TestItemTypeConstants는 아이템 타입 상수 값을 검증한다.
func TestItemTypeConstants(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want string
	}{
		{"ItemTypeReasoning", protocol.ItemTypeReasoning, "reasoning"},
		{"ItemTypePlan", protocol.ItemTypePlan, "plan"},
		{"ItemTypeContextCompaction", protocol.ItemTypeContextCompaction, "contextCompaction"},
		{"ItemTypeWebSearch", protocol.ItemTypeWebSearch, "webSearch"},
		{"ItemTypeImageView", protocol.ItemTypeImageView, "imageView"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.val != tt.want {
				t.Errorf("아이템 타입 상수 불일치: got %q, want %q", tt.val, tt.want)
			}
		})
	}
}

// TestExistingMethodConstants는 기존 메서드 상수가 변경되지 않았음을 검증한다 (회귀 테스트).
func TestExistingMethodConstants(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want string
	}{
		{"MethodInitialize", protocol.MethodInitialize, "initialize"},
		{"MethodInitialized", protocol.MethodInitialized, "initialized"},
		{"MethodAccountLoginStart", protocol.MethodAccountLoginStart, "account/login/start"},
		{"MethodThreadStart", protocol.MethodThreadStart, "thread/start"},
		{"MethodThreadResume", protocol.MethodThreadResume, "thread/resume"},
		{"MethodTurnStart", protocol.MethodTurnStart, "turn/start"},
		{"MethodTurnInterrupt", protocol.MethodTurnInterrupt, "turn/interrupt"},
		{"MethodTurnCompleted", protocol.MethodTurnCompleted, "turn/completed"},
		{"MethodItemStarted", protocol.MethodItemStarted, "item/started"},
		{"MethodItemCompleted", protocol.MethodItemCompleted, "item/completed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.val != tt.want {
				t.Errorf("기존 메서드 상수 변경됨: got %q, want %q", tt.val, tt.want)
			}
		})
	}
}
