// autonomy_workflow_test.go는 자율 워크플로우의 라이프사이클 상태 전이를 테스트합니다.
// buildStatefulServer를 사용하여 순차적 API 호출의 상태 전이를 검증합니다.
package cmd

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
)

// TestMeetingLifecycle_HappyPath는 미팅의 정상 라이프사이클을 테스트합니다.
// 생성(scheduled) → 시작(in_progress) → 메시지 조회 → 종료(ended)
func TestMeetingLifecycle_HappyPath(t *testing.T) {
	t.Parallel()

	steps := []ServerStep{
		// 1단계: 미팅 생성 → scheduled
		{
			Method:     http.MethodPost,
			PathPrefix: "/api/v1/workspaces/ws-1/meetings",
			ResponseData: Meeting{
				ID:          "meet-lc-1",
				Title:       "스프린트 플래닝",
				Agenda:      "이번 스프린트 목표 설정",
				Status:      "scheduled",
				ChannelID:   "ch-1",
				ScheduledAt: "2026-03-11T09:00:00Z",
				CreatedAt:   "2026-03-10T18:00:00Z",
			},
		},
		// 2단계: 미팅 시작 → in_progress
		{
			Method:     http.MethodPatch,
			PathPrefix: "/api/v1/meetings/meet-lc-1/start",
			ResponseData: Meeting{
				ID:        "meet-lc-1",
				Title:     "스프린트 플래닝",
				Status:    "in_progress",
				StartedAt: "2026-03-11T09:00:00Z",
			},
		},
		// 3단계: 메시지 조회
		{
			Method:     http.MethodGet,
			PathPrefix: "/api/v1/meetings/meet-lc-1/messages",
			ResponseData: []MeetingMessage{
				{ID: "msg-1", MeetingID: "meet-lc-1", Sender: "agent-1", Content: "목표를 정합시다", CreatedAt: "2026-03-11T09:01:00Z"},
				{ID: "msg-2", MeetingID: "meet-lc-1", Sender: "agent-2", Content: "동의합니다", CreatedAt: "2026-03-11T09:02:00Z"},
			},
		},
		// 4단계: 미팅 종료 → ended
		{
			Method:     http.MethodPatch,
			PathPrefix: "/api/v1/meetings/meet-lc-1/end",
			ResponseData: Meeting{
				ID:      "meet-lc-1",
				Title:   "스프린트 플래닝",
				Status:  "ended",
				EndedAt: "2026-03-11T10:00:00Z",
			},
		},
	}

	srv := buildStatefulServer(t, steps)
	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	// 생성
	err := runMeetingCreate(client, &buf, "스프린트 플래닝", "이번 스프린트 목표 설정", "ch-1", "agent-1,agent-2", "2026-03-11T09:00:00Z", false)
	if err != nil {
		t.Fatalf("미팅 생성 실패: %v", err)
	}
	if !strings.Contains(buf.String(), "스프린트 플래닝") {
		t.Errorf("생성 출력에 '스프린트 플래닝'이 없습니다: %s", buf.String())
	}

	// 시작
	buf.Reset()
	err = runMeetingStart(client, &buf, "meet-lc-1")
	if err != nil {
		t.Fatalf("미팅 시작 실패: %v", err)
	}
	if !strings.Contains(buf.String(), "in_progress") {
		t.Errorf("시작 출력에 'in_progress'가 없습니다: %s", buf.String())
	}

	// 메시지 조회
	buf.Reset()
	err = runMeetingMessages(client, &buf, "meet-lc-1", false)
	if err != nil {
		t.Fatalf("미팅 메시지 조회 실패: %v", err)
	}
	if !strings.Contains(buf.String(), "목표를 정합시다") {
		t.Errorf("메시지 출력에 '목표를 정합시다'가 없습니다: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "동의합니다") {
		t.Errorf("메시지 출력에 '동의합니다'가 없습니다: %s", buf.String())
	}

	// 종료
	buf.Reset()
	err = runMeetingEnd(client, &buf, "meet-lc-1")
	if err != nil {
		t.Fatalf("미팅 종료 실패: %v", err)
	}
	if !strings.Contains(buf.String(), "ended") {
		t.Errorf("종료 출력에 'ended'가 없습니다: %s", buf.String())
	}
}

// TestMeetingLifecycle_Cancel는 미팅 취소 라이프사이클을 테스트합니다.
// 생성(scheduled) → 취소(cancelled)
func TestMeetingLifecycle_Cancel(t *testing.T) {
	t.Parallel()

	steps := []ServerStep{
		{
			Method:     http.MethodPost,
			PathPrefix: "/api/v1/workspaces/ws-1/meetings",
			ResponseData: Meeting{
				ID:     "meet-cancel-1",
				Title:  "취소될 미팅",
				Status: "scheduled",
			},
		},
		{
			Method:     http.MethodPatch,
			PathPrefix: "/api/v1/meetings/meet-cancel-1/cancel",
			ResponseData: Meeting{
				ID:     "meet-cancel-1",
				Title:  "취소될 미팅",
				Status: "cancelled",
			},
		},
	}

	srv := buildStatefulServer(t, steps)
	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	// 생성
	err := runMeetingCreate(client, &buf, "취소될 미팅", "", "ch-1", "", "", false)
	if err != nil {
		t.Fatalf("미팅 생성 실패: %v", err)
	}
	if !strings.Contains(buf.String(), "취소될 미팅") {
		t.Errorf("생성 출력에 '취소될 미팅'이 없습니다: %s", buf.String())
	}

	// 취소
	buf.Reset()
	err = runMeetingCancel(client, &buf, "meet-cancel-1")
	if err != nil {
		t.Fatalf("미팅 취소 실패: %v", err)
	}
	if !strings.Contains(buf.String(), "cancelled") {
		t.Errorf("취소 출력에 'cancelled'가 없습니다: %s", buf.String())
	}
}

// TestMeetingLifecycle_RegenerateMinutes는 회의록 재생성 라이프사이클을 테스트합니다.
// 생성(scheduled) → 시작(in_progress) → 종료(ended) → 회의록 재생성(ended)
func TestMeetingLifecycle_RegenerateMinutes(t *testing.T) {
	t.Parallel()

	steps := []ServerStep{
		{
			Method:     http.MethodPost,
			PathPrefix: "/api/v1/workspaces/ws-1/meetings",
			ResponseData: Meeting{
				ID:     "meet-regen-1",
				Title:  "회의록 테스트",
				Status: "scheduled",
			},
		},
		{
			Method:     http.MethodPatch,
			PathPrefix: "/api/v1/meetings/meet-regen-1/start",
			ResponseData: Meeting{
				ID:     "meet-regen-1",
				Title:  "회의록 테스트",
				Status: "in_progress",
			},
		},
		{
			Method:     http.MethodPatch,
			PathPrefix: "/api/v1/meetings/meet-regen-1/end",
			ResponseData: Meeting{
				ID:     "meet-regen-1",
				Title:  "회의록 테스트",
				Status: "ended",
			},
		},
		{
			Method:     http.MethodPost,
			PathPrefix: "/api/v1/meetings/meet-regen-1/regenerate-minutes",
			ResponseData: Meeting{
				ID:     "meet-regen-1",
				Title:  "회의록 테스트",
				Status: "ended",
			},
		},
	}

	srv := buildStatefulServer(t, steps)
	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	// 생성
	err := runMeetingCreate(client, &buf, "회의록 테스트", "", "ch-1", "", "", false)
	if err != nil {
		t.Fatalf("미팅 생성 실패: %v", err)
	}

	// 시작
	buf.Reset()
	err = runMeetingStart(client, &buf, "meet-regen-1")
	if err != nil {
		t.Fatalf("미팅 시작 실패: %v", err)
	}
	if !strings.Contains(buf.String(), "in_progress") {
		t.Errorf("시작 출력에 'in_progress'가 없습니다: %s", buf.String())
	}

	// 종료
	buf.Reset()
	err = runMeetingEnd(client, &buf, "meet-regen-1")
	if err != nil {
		t.Fatalf("미팅 종료 실패: %v", err)
	}
	if !strings.Contains(buf.String(), "ended") {
		t.Errorf("종료 출력에 'ended'가 없습니다: %s", buf.String())
	}

	// 회의록 재생성
	buf.Reset()
	err = runMeetingRegenerateMinutes(client, &buf, "meet-regen-1")
	if err != nil {
		t.Fatalf("회의록 재생성 실패: %v", err)
	}
	if !strings.Contains(buf.String(), "회의록 테스트") {
		t.Errorf("재생성 출력에 '회의록 테스트'가 없습니다: %s", buf.String())
	}
}

// TestTaskLifecycle_HappyPath는 태스크의 정상 라이프사이클을 테스트합니다.
// 생성(pending) → 배정(assigned) → 시작(in_progress) → 완료(completed)
func TestTaskLifecycle_HappyPath(t *testing.T) {
	t.Parallel()

	steps := []ServerStep{
		// 태스크 생성 → pending
		{
			Method:     http.MethodPost,
			PathPrefix: "/api/v1/workspaces/ws-1/agent-tasks",
			ResponseData: AgentTask{
				ID:       "task-lc-1",
				Title:    "버그 수정",
				TaskType: "code",
				Priority: 1,
				Status:   "pending",
			},
		},
		// 에이전트 배정 → assigned
		{
			Method:     http.MethodPatch,
			PathPrefix: "/api/v1/agent-tasks/task-lc-1/assign",
			ResponseData: AgentTask{
				ID:        "task-lc-1",
				Title:     "버그 수정",
				Status:    "assigned",
				AgentID:   "agent-1",
				AgentName: "Alice",
			},
		},
		// 태스크 시작 → in_progress
		{
			Method:     http.MethodPost,
			PathPrefix: "/api/v1/agent-tasks/task-lc-1/start",
			ResponseData: AgentTask{
				ID:     "task-lc-1",
				Title:  "버그 수정",
				Status: "in_progress",
			},
		},
		// 태스크 완료 → completed
		{
			Method:     http.MethodPost,
			PathPrefix: "/api/v1/agent-tasks/task-lc-1/complete",
			ResponseData: AgentTask{
				ID:     "task-lc-1",
				Title:  "버그 수정",
				Status: "completed",
			},
		},
	}

	srv := buildStatefulServer(t, steps)
	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	// 생성
	err := runTaskCreate(client, &buf, "버그 수정", "code", 1, "agent-1", "로그인 실패 버그", false)
	if err != nil {
		t.Fatalf("태스크 생성 실패: %v", err)
	}
	if !strings.Contains(buf.String(), "버그 수정") {
		t.Errorf("생성 출력에 '버그 수정'이 없습니다: %s", buf.String())
	}

	// 배정
	buf.Reset()
	err = runTaskAssign(client, &buf, "task-lc-1", "agent-1", false)
	if err != nil {
		t.Fatalf("태스크 배정 실패: %v", err)
	}
	if !strings.Contains(buf.String(), "agent-1") {
		t.Errorf("배정 출력에 'agent-1'이 없습니다: %s", buf.String())
	}

	// 시작
	buf.Reset()
	err = runTaskStart(client, &buf, "task-lc-1")
	if err != nil {
		t.Fatalf("태스크 시작 실패: %v", err)
	}
	if !strings.Contains(buf.String(), "in_progress") {
		t.Errorf("시작 출력에 'in_progress'가 없습니다: %s", buf.String())
	}

	// 완료
	buf.Reset()
	err = runTaskComplete(client, &buf, "task-lc-1", `{"result":"fixed"}`)
	if err != nil {
		t.Fatalf("태스크 완료 실패: %v", err)
	}
	if !strings.Contains(buf.String(), "completed") {
		t.Errorf("완료 출력에 'completed'가 없습니다: %s", buf.String())
	}
}

// TestTaskLifecycle_Failure는 태스크 실패 라이프사이클을 테스트합니다.
// 생성(pending) → 배정(assigned) → 시작(in_progress) → 실패(failed)
func TestTaskLifecycle_Failure(t *testing.T) {
	t.Parallel()

	steps := []ServerStep{
		{
			Method:     http.MethodPost,
			PathPrefix: "/api/v1/workspaces/ws-1/agent-tasks",
			ResponseData: AgentTask{
				ID:       "task-fail-1",
				Title:    "불안정한 작업",
				TaskType: "analysis",
				Priority: 2,
				Status:   "pending",
			},
		},
		{
			Method:     http.MethodPatch,
			PathPrefix: "/api/v1/agent-tasks/task-fail-1/assign",
			ResponseData: AgentTask{
				ID:        "task-fail-1",
				Title:     "불안정한 작업",
				Status:    "assigned",
				AgentID:   "agent-2",
				AgentName: "Bob",
			},
		},
		{
			Method:     http.MethodPost,
			PathPrefix: "/api/v1/agent-tasks/task-fail-1/start",
			ResponseData: AgentTask{
				ID:     "task-fail-1",
				Title:  "불안정한 작업",
				Status: "in_progress",
			},
		},
		{
			Method:     http.MethodPost,
			PathPrefix: "/api/v1/agent-tasks/task-fail-1/fail",
			ResponseData: AgentTask{
				ID:     "task-fail-1",
				Title:  "불안정한 작업",
				Status: "failed",
			},
		},
	}

	srv := buildStatefulServer(t, steps)
	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	// 생성
	err := runTaskCreate(client, &buf, "불안정한 작업", "analysis", 2, "agent-2", "분석 작업", false)
	if err != nil {
		t.Fatalf("태스크 생성 실패: %v", err)
	}

	// 배정
	buf.Reset()
	err = runTaskAssign(client, &buf, "task-fail-1", "agent-2", false)
	if err != nil {
		t.Fatalf("태스크 배정 실패: %v", err)
	}

	// 시작
	buf.Reset()
	err = runTaskStart(client, &buf, "task-fail-1")
	if err != nil {
		t.Fatalf("태스크 시작 실패: %v", err)
	}

	// 실패
	buf.Reset()
	err = runTaskFail(client, &buf, "task-fail-1", "외부 API 타임아웃")
	if err != nil {
		t.Fatalf("태스크 실패 처리 실패: %v", err)
	}
	if !strings.Contains(buf.String(), "failed") {
		t.Errorf("실패 출력에 'failed'가 없습니다: %s", buf.String())
	}
}

// TestTaskLifecycle_Cancel는 태스크 취소 라이프사이클을 테스트합니다.
// 생성(pending) → 취소(cancelled)
func TestTaskLifecycle_Cancel(t *testing.T) {
	t.Parallel()

	steps := []ServerStep{
		{
			Method:     http.MethodPost,
			PathPrefix: "/api/v1/workspaces/ws-1/agent-tasks",
			ResponseData: AgentTask{
				ID:       "task-cancel-1",
				Title:    "취소될 태스크",
				TaskType: "code",
				Priority: 3,
				Status:   "pending",
			},
		},
		{
			Method:     http.MethodPost,
			PathPrefix: "/api/v1/agent-tasks/task-cancel-1/cancel",
			ResponseData: AgentTask{
				ID:     "task-cancel-1",
				Title:  "취소될 태스크",
				Status: "cancelled",
			},
		},
	}

	srv := buildStatefulServer(t, steps)
	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	// 생성
	err := runTaskCreate(client, &buf, "취소될 태스크", "code", 3, "", "불필요한 작업", false)
	if err != nil {
		t.Fatalf("태스크 생성 실패: %v", err)
	}

	// 취소
	buf.Reset()
	err = runTaskCancel(client, &buf, "task-cancel-1")
	if err != nil {
		t.Fatalf("태스크 취소 실패: %v", err)
	}
	if !strings.Contains(buf.String(), "cancelled") {
		t.Errorf("취소 출력에 'cancelled'가 없습니다: %s", buf.String())
	}
}

// TestDecisionLifecycle_Consensus는 의사결정 합의 라이프사이클을 테스트합니다.
// 생성(open) → 투표 x3 (투표 수 증가) → 해결(resolved)
func TestDecisionLifecycle_Consensus(t *testing.T) {
	t.Parallel()

	steps := []ServerStep{
		// 의사결정 생성 → open
		{
			Method:     http.MethodPost,
			PathPrefix: "/api/v1/decisions",
			ResponseData: Decision{
				ID:           "dec-cons-1",
				Topic:        "배포 전략 결정",
				Context:      "블루-그린 vs 카나리 배포",
				Status:       "open",
				Level:        "team",
				InitiatedBy:  "agent-1",
				Participants: []string{"agent-1", "agent-2", "agent-3"},
				CreatedAt:    "2026-03-11T10:00:00Z",
			},
		},
		// 1차 투표
		{
			Method:     http.MethodPost,
			PathPrefix: "/api/v1/decisions/dec-cons-1/consensus/vote",
			ResponseData: ConsensusStatus{
				DecisionID: "dec-cons-1",
				Status:     "voting",
				Votes:      1,
				Required:   3,
			},
		},
		// 2차 투표
		{
			Method:     http.MethodPost,
			PathPrefix: "/api/v1/decisions/dec-cons-1/consensus/vote",
			ResponseData: ConsensusStatus{
				DecisionID: "dec-cons-1",
				Status:     "voting",
				Votes:      2,
				Required:   3,
			},
		},
		// 3차 투표 (합의 달성)
		{
			Method:     http.MethodPost,
			PathPrefix: "/api/v1/decisions/dec-cons-1/consensus/vote",
			ResponseData: ConsensusStatus{
				DecisionID: "dec-cons-1",
				Status:     "consensus_reached",
				Votes:      3,
				Required:   3,
			},
		},
		// 해결
		{
			Method:     http.MethodPatch,
			PathPrefix: "/api/v1/decisions/dec-cons-1/resolve",
			ResponseData: Decision{
				ID:        "dec-cons-1",
				Topic:     "배포 전략 결정",
				Status:    "resolved",
				Outcome:   "카나리 배포 채택",
				Rationale: "점진적 롤아웃이 리스크 최소화에 유리",
			},
		},
	}

	srv := buildStatefulServer(t, steps)
	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	// 생성
	err := runDecisionCreate(client, &buf, "배포 전략 결정", "블루-그린 vs 카나리 배포", "agent-1", []string{"agent-1", "agent-2", "agent-3"}, false)
	if err != nil {
		t.Fatalf("의사결정 생성 실패: %v", err)
	}
	if !strings.Contains(buf.String(), "배포 전략 결정") {
		t.Errorf("생성 출력에 '배포 전략 결정'이 없습니다: %s", buf.String())
	}

	// 1차 투표
	buf.Reset()
	err = runDecisionVote(client, &buf, "dec-cons-1", false)
	if err != nil {
		t.Fatalf("1차 투표 실패: %v", err)
	}

	// 2차 투표
	buf.Reset()
	err = runDecisionVote(client, &buf, "dec-cons-1", false)
	if err != nil {
		t.Fatalf("2차 투표 실패: %v", err)
	}

	// 3차 투표
	buf.Reset()
	err = runDecisionVote(client, &buf, "dec-cons-1", false)
	if err != nil {
		t.Fatalf("3차 투표 실패: %v", err)
	}
	if !strings.Contains(buf.String(), "consensus_reached") {
		t.Errorf("3차 투표 출력에 'consensus_reached'가 없습니다: %s", buf.String())
	}

	// 해결
	buf.Reset()
	err = runDecisionResolve(client, &buf, "dec-cons-1", "카나리 배포 채택", "점진적 롤아웃이 리스크 최소화에 유리", "agent-1")
	if err != nil {
		t.Fatalf("의사결정 해결 실패: %v", err)
	}
	if !strings.Contains(buf.String(), "resolved") {
		t.Errorf("해결 출력에 'resolved'가 없습니다: %s", buf.String())
	}
}

// TestDecisionLifecycle_Escalate는 의사결정 에스컬레이션 라이프사이클을 테스트합니다.
// 생성(open) → 에스컬레이션(escalated)
func TestDecisionLifecycle_Escalate(t *testing.T) {
	t.Parallel()

	steps := []ServerStep{
		{
			Method:     http.MethodPost,
			PathPrefix: "/api/v1/decisions",
			ResponseData: Decision{
				ID:          "dec-esc-1",
				Topic:       "예산 초과 승인",
				Context:     "프로젝트 예산 200% 초과",
				Status:      "open",
				Level:       "team",
				InitiatedBy: "agent-1",
				CreatedAt:   "2026-03-11T11:00:00Z",
			},
		},
		{
			Method:     http.MethodPost,
			PathPrefix: "/api/v1/decisions/dec-esc-1/escalate",
			ResponseData: Decision{
				ID:     "dec-esc-1",
				Topic:  "예산 초과 승인",
				Status: "escalated",
				Level:  "org",
			},
		},
	}

	srv := buildStatefulServer(t, steps)
	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	// 생성
	err := runDecisionCreate(client, &buf, "예산 초과 승인", "프로젝트 예산 200% 초과", "agent-1", []string{"agent-1"}, false)
	if err != nil {
		t.Fatalf("의사결정 생성 실패: %v", err)
	}
	if !strings.Contains(buf.String(), "예산 초과 승인") {
		t.Errorf("생성 출력에 '예산 초과 승인'이 없습니다: %s", buf.String())
	}

	// 에스컬레이션
	buf.Reset()
	err = runDecisionEscalate(client, &buf, "dec-esc-1", "팀 레벨 권한 초과", "agent-1")
	if err != nil {
		t.Fatalf("에스컬레이션 실패: %v", err)
	}
	if !strings.Contains(buf.String(), "escalated") {
		t.Errorf("에스컬레이션 출력에 'escalated'가 없습니다: %s", buf.String())
	}
}

// TestApprovalChain_Approved는 승인 체인의 정상 승인 흐름을 테스트합니다.
// 1단계 승인(pending→approved) → 2단계 승인(approved)
func TestApprovalChain_Approved(t *testing.T) {
	t.Parallel()

	steps := []ServerStep{
		// 1단계 승인
		{
			Method:     http.MethodPost,
			PathPrefix: "/api/v1/approvals/apr-step-1/approve",
			ResponseData: Approval{
				ID:          "apr-step-1",
				Title:       "프로덕션 배포 승인",
				Status:      "approved",
				RequestedBy: "agent-1",
				ApprovedBy:  "manager-1",
				CreatedAt:   "2026-03-11T12:00:00Z",
			},
		},
		// 2단계 승인
		{
			Method:     http.MethodPost,
			PathPrefix: "/api/v1/approvals/apr-step-2/approve",
			ResponseData: Approval{
				ID:          "apr-step-2",
				Title:       "프로덕션 배포 최종 승인",
				Status:      "approved",
				RequestedBy: "agent-1",
				ApprovedBy:  "director-1",
				CreatedAt:   "2026-03-11T12:30:00Z",
			},
		},
	}

	srv := buildStatefulServer(t, steps)
	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	// 1단계 승인
	err := runApprovalApprove(client, &buf, "apr-step-1")
	if err != nil {
		t.Fatalf("1단계 승인 실패: %v", err)
	}
	if !strings.Contains(buf.String(), "approved") {
		t.Errorf("1단계 출력에 'approved'가 없습니다: %s", buf.String())
	}

	// 2단계 승인
	buf.Reset()
	err = runApprovalApprove(client, &buf, "apr-step-2")
	if err != nil {
		t.Fatalf("2단계 승인 실패: %v", err)
	}
	if !strings.Contains(buf.String(), "approved") {
		t.Errorf("2단계 출력에 'approved'가 없습니다: %s", buf.String())
	}
}

// TestApprovalChain_Rejected는 승인 체인의 거부 흐름을 테스트합니다.
// 1단계 거부(rejected)
func TestApprovalChain_Rejected(t *testing.T) {
	t.Parallel()

	steps := []ServerStep{
		{
			Method:     http.MethodPost,
			PathPrefix: "/api/v1/approvals/apr-reject-1/reject",
			ResponseData: Approval{
				ID:          "apr-reject-1",
				Title:       "위험한 변경 승인 요청",
				Status:      "rejected",
				RequestedBy: "agent-1",
				CreatedAt:   "2026-03-11T13:00:00Z",
			},
		},
	}

	srv := buildStatefulServer(t, steps)
	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	// 거부
	err := runApprovalReject(client, &buf, "apr-reject-1")
	if err != nil {
		t.Fatalf("승인 거부 실패: %v", err)
	}
	if !strings.Contains(buf.String(), "rejected") {
		t.Errorf("거부 출력에 'rejected'가 없습니다: %s", buf.String())
	}
}
