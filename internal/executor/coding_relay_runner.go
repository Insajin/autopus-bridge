// Package executor - 코딩 릴레이 실행기
package executor

import (
	"context"
	"encoding/json"
	"fmt"

	ws "github.com/insajin/autopus-agent-protocol"
	"github.com/rs/zerolog/log"
)

// CodingRelayRunner는 코딩 릴레이 루프를 실행합니다.
// websocket.CodingRelayRunner 인터페이스를 구현합니다.
// @MX:ANCHOR: [AUTO] 코딩 릴레이 루프 핵심 실행기 (fan_in >= 3)
// @MX:REASON: WebSocket 핸들러 콜백, 테스트, CodingSessionManager에서 사용
// @MX:SPEC: SPEC-CODING-RELAY-001
type CodingRelayRunner struct {
	mgr *CodingSessionManager
}

// NewCodingRelayRunner는 새로운 CodingRelayRunner를 생성합니다.
func NewCodingRelayRunner(mgr *CodingSessionManager) *CodingRelayRunner {
	return &CodingRelayRunner{mgr: mgr}
}

// RunRelay는 코딩 릴레이 루프를 실행합니다.
// websocket.CodingRelayRunner 인터페이스를 구현합니다.
// 흐름: Open → [Send → Evaluate → Feedback] × maxIterations → Complete/Error
// @MX:NOTE: [AUTO] sendMsg 콜백으로 WS 메시지 전송, feedbackCh로 Worker 피드백 수신
func (r *CodingRelayRunner) RunRelay(
	ctx context.Context,
	req ws.CodingRelayRequestPayload,
	sendMsg func(msgType string, payload []byte) error,
	feedbackCh <-chan ws.CodingRelayFeedbackPayload,
) {
	maxIterations := req.MaxIterations
	if maxIterations <= 0 {
		maxIterations = 10
	}

	// 프로바이더 탐지 (세마포어 획득 전에 수행하여 슬롯 점유 최소화)
	providers := r.mgr.detectProviders()
	provider := CodingProviderAPI // 기본: API 폴백
	if len(providers) > 0 {
		provider = providers[0]
	}

	// 세션 슬롯 획득
	if err := r.mgr.AcquireSlot(ctx); err != nil {
		sendRelayError(sendMsg, req.RequestID, "", fmt.Sprintf("세션 슬롯 획득 실패: %v", err), "slot_exhausted")
		return
	}
	defer r.mgr.ReleaseSlot()

	session, err := r.mgr.CreateSession(provider)
	if err != nil {
		sendRelayError(sendMsg, req.RequestID, "", fmt.Sprintf("세션 생성 실패: %v", err), "session_create_failed")
		return
	}

	// 세션 열기
	openReq := CodingSessionOpenRequest{
		ResumeSession: req.SessionID,
		MaxBudgetUSD:  req.MaxBudgetUSD,
	}
	if err := session.Open(ctx, openReq); err != nil {
		sendRelayError(sendMsg, req.RequestID, "", fmt.Sprintf("세션 열기 실패: %v", err), "session_open_failed")
		return
	}
	// 세션 등록 (CloseSession이 session.Close도 내부적으로 호출)
	r.mgr.RegisterSession(req.RequestID, session)
	defer r.mgr.CloseSession(ctx, req.RequestID) //nolint:errcheck

	message := req.TaskDescription
	var lastResp *CodingSessionResponse

	for iteration := 1; iteration <= maxIterations; iteration++ {
		// 진행 상황 전송
		sendRelayProgress(sendMsg, req.RequestID, iteration, "running",
			fmt.Sprintf("이터레이션 %d/%d 실행 중", iteration, maxIterations))

		// 코딩 에이전트에 메시지 전송
		resp, err := session.Send(ctx, message)
		if err != nil {
			sendRelayError(sendMsg, req.RequestID, session.SessionID(),
				fmt.Sprintf("세션 Send 실패 (iter=%d): %v", iteration, err), "session_send_failed")
			return
		}
		lastResp = resp

		// Worker 평가를 위해 이터레이션 결과 전송
		if err := sendRelayEvaluate(sendMsg, ws.CodingRelayEvaluatePayload{
			RequestID:    req.RequestID,
			Iteration:    iteration,
			Content:      resp.Content,
			DiffSummary:  resp.DiffSummary,
			TestOutput:   resp.TestOutput,
			FilesChanged: resp.FilesChanged,
			SessionID:    session.SessionID(),
		}); err != nil {
			log.Error().Err(err).Msg("[coding-relay] evaluate 전송 실패")
			return
		}

		// Worker 피드백 대기
		select {
		case <-ctx.Done():
			sendRelayError(sendMsg, req.RequestID, session.SessionID(), "컨텍스트 취소됨", "context_cancelled")
			return
		case feedback, ok := <-feedbackCh:
			if !ok {
				return
			}
			if feedback.Approved {
				// Worker 승인 — 릴레이 완료
				sendRelayComplete(sendMsg, ws.CodingRelayCompletePayload{
					RequestID:    req.RequestID,
					Success:      true,
					DiffSummary:  resp.DiffSummary,
					FilesChanged: resp.FilesChanged,
					TestOutput:   resp.TestOutput,
					SessionID:    session.SessionID(),
					Iterations:   iteration,
					CostUSD:      resp.CostUSD,
				})
				return
			}
			// Worker 피드백을 다음 이터레이션 메시지로 사용
			message = feedback.Feedback
		}
	}

	// 최대 반복 횟수 도달 — 마지막 결과로 완료 (미승인)
	var finalResp CodingSessionResponse
	if lastResp != nil {
		finalResp = *lastResp
	}
	sendRelayComplete(sendMsg, ws.CodingRelayCompletePayload{
		RequestID:    req.RequestID,
		Success:      false,
		DiffSummary:  finalResp.DiffSummary,
		FilesChanged: finalResp.FilesChanged,
		TestOutput:   finalResp.TestOutput,
		SessionID:    session.SessionID(),
		Iterations:   maxIterations,
		CostUSD:      finalResp.CostUSD,
	})
}

// sendRelayEvaluate는 이터레이션 평가 페이로드를 직렬화하여 전송합니다.
func sendRelayEvaluate(sendMsg func(string, []byte) error, payload ws.CodingRelayEvaluatePayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("evaluate 직렬화 실패: %w", err)
	}
	return sendMsg(ws.AgentMsgCodingRelayEvaluate, data)
}

// sendRelayComplete는 릴레이 완료 페이로드를 직렬화하여 전송합니다.
func sendRelayComplete(sendMsg func(string, []byte) error, payload ws.CodingRelayCompletePayload) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Error().Err(err).Msg("[coding-relay] complete 직렬화 실패")
		return
	}
	log.Info().Str("request_id", payload.RequestID).Bool("success", payload.Success).Int("iterations", payload.Iterations).Msg("[coding-relay] 완료")
	_ = sendMsg(ws.AgentMsgCodingRelayComplete, data)
}

// sendRelayError는 릴레이 에러 페이로드를 직렬화하여 전송합니다.
func sendRelayError(sendMsg func(string, []byte) error, requestID, sessionID, errMsg, code string) {
	payload := ws.CodingRelayErrorPayload{
		RequestID: requestID,
		Error:     errMsg,
		Code:      code,
		SessionID: sessionID,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		log.Error().Err(err).Msg("[coding-relay] error 직렬화 실패")
		return
	}
	log.Error().Str("request_id", requestID).Str("code", code).Msg("[coding-relay] 에러")
	_ = sendMsg(ws.AgentMsgCodingRelayError, data)
}

// sendRelayProgress는 릴레이 진행 상황 페이로드를 직렬화하여 전송합니다.
func sendRelayProgress(sendMsg func(string, []byte) error, requestID string, iteration int, status, msg string) {
	payload := ws.CodingRelayProgressPayload{
		RequestID: requestID,
		Iteration: iteration,
		Status:    status,
		Message:   msg,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_ = sendMsg(ws.AgentMsgCodingRelayProgress, data)
}
