//go:build integration

// workflow_integration_test.go는 커맨드 간 통합 워크플로우를 테스트합니다.
// 미팅 → 의사결정 → 태스크, 스프린트 기반 개발 등 실제 사용 시나리오를 검증합니다.
package cmd

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
)

// TestWorkflow_MeetingToDecisionToTask는 미팅 생성부터 태스크 완료까지의
// 전체 파이프라인을 테스트합니다.
// 흐름: 미팅 생성 → 시작 → 메시지 조회 → 종료 → 의사결정 생성 → 투표 → 해결 → 태스크 생성 → 할당 → 시작 → 완료
func TestWorkflow_MeetingToDecisionToTask(t *testing.T) {
	t.Parallel()

	// 11단계 stateful 서버 구성
	steps := []ServerStep{
		// 스텝 0: 미팅 생성
		{
			Method:     http.MethodPost,
			PathPrefix: "/api/v1/workspaces/ws-1/meetings",
			ResponseData: Meeting{
				ID:           "meet-001",
				Title:        "아키텍처 리뷰",
				Agenda:       "마이크로서비스 전환 논의",
				Status:       "scheduled",
				ChannelID:    "ch-1",
				Participants: []string{"agent-1", "agent-2"},
				ScheduledAt:  "2026-03-11T10:00:00Z",
				CreatedAt:    "2026-03-11T09:00:00Z",
			},
		},
		// 스텝 1: 미팅 시작
		{
			Method:     http.MethodPatch,
			PathPrefix: "/api/v1/meetings/meet-001/start",
			ResponseData: Meeting{
				ID:        "meet-001",
				Title:     "아키텍처 리뷰",
				Status:    "in_progress",
				StartedAt: "2026-03-11T10:00:00Z",
			},
		},
		// 스텝 2: 미팅 메시지 조회
		{
			Method:     http.MethodGet,
			PathPrefix: "/api/v1/meetings/meet-001/messages",
			ResponseData: []MeetingMessage{
				{ID: "msg-1", MeetingID: "meet-001", Sender: "agent-1", Content: "마이크로서비스로 전환 제안", CreatedAt: "2026-03-11T10:05:00Z"},
				{ID: "msg-2", MeetingID: "meet-001", Sender: "agent-2", Content: "동의합니다", CreatedAt: "2026-03-11T10:06:00Z"},
			},
		},
		// 스텝 3: 미팅 종료
		{
			Method:     http.MethodPatch,
			PathPrefix: "/api/v1/meetings/meet-001/end",
			ResponseData: Meeting{
				ID:      "meet-001",
				Title:   "아키텍처 리뷰",
				Status:  "ended",
				EndedAt: "2026-03-11T11:00:00Z",
			},
		},
		// 스텝 4: 의사결정 생성
		{
			Method:     http.MethodPost,
			PathPrefix: "/api/v1/decisions",
			ResponseData: Decision{
				ID:           "dec-001",
				Topic:        "마이크로서비스 전환",
				Context:      "아키텍처 리뷰 미팅에서 논의",
				Status:       "pending",
				Level:        "team",
				InitiatedBy:  "agent-1",
				Participants: []string{"agent-1", "agent-2"},
				CreatedAt:    "2026-03-11T11:05:00Z",
			},
		},
		// 스텝 5: 의사결정 투표
		{
			Method:     http.MethodPost,
			PathPrefix: "/api/v1/decisions/dec-001/consensus/vote",
			ResponseData: ConsensusStatus{
				DecisionID: "dec-001",
				Status:     "voting",
				Votes:      2,
				Required:   2,
			},
		},
		// 스텝 6: 의사결정 해결
		{
			Method:     http.MethodPatch,
			PathPrefix: "/api/v1/decisions/dec-001/resolve",
			ResponseData: Decision{
				ID:        "dec-001",
				Topic:     "마이크로서비스 전환",
				Status:    "resolved",
				Outcome:   "approved",
				Rationale: "팀 전원 동의",
			},
		},
		// 스텝 7: 태스크 생성
		{
			Method:     http.MethodPost,
			PathPrefix: "/api/v1/workspaces/ws-1/agent-tasks",
			ResponseData: AgentTask{
				ID:          "task-001",
				Title:       "서비스 분리 구현",
				Description: "모놀리스에서 주문 서비스 분리",
				TaskType:    "code",
				Priority:    1,
				Status:      "pending",
				AgentID:     "agent-1",
				AgentName:   "Backend Agent",
				CreatedAt:   "2026-03-11T11:10:00Z",
			},
		},
		// 스텝 8: 태스크 할당
		{
			Method:     http.MethodPatch,
			PathPrefix: "/api/v1/agent-tasks/task-001/assign",
			ResponseData: AgentTask{
				ID:        "task-001",
				Title:     "서비스 분리 구현",
				Status:    "assigned",
				AgentID:   "agent-1",
				AgentName: "Backend Agent",
			},
		},
		// 스텝 9: 태스크 시작
		{
			Method:     http.MethodPost,
			PathPrefix: "/api/v1/agent-tasks/task-001/start",
			ResponseData: AgentTask{
				ID:     "task-001",
				Title:  "서비스 분리 구현",
				Status: "in_progress",
			},
		},
		// 스텝 10: 태스크 완료
		{
			Method:     http.MethodPost,
			PathPrefix: "/api/v1/agent-tasks/task-001/complete",
			ResponseData: AgentTask{
				ID:     "task-001",
				Title:  "서비스 분리 구현",
				Status: "completed",
			},
		},
	}

	srv := buildStatefulServer(t, steps)
	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	// 스텝 0: 미팅 생성
	if err := runMeetingCreate(client, &buf, "아키텍처 리뷰", "마이크로서비스 전환 논의", "ch-1", "agent-1,agent-2", "2026-03-11T10:00:00Z", false); err != nil {
		t.Fatalf("미팅 생성 실패: %v", err)
	}
	if !strings.Contains(buf.String(), "meet-001") {
		t.Errorf("미팅 생성 출력에 ID가 없습니다: %s", buf.String())
	}
	buf.Reset()

	// 스텝 1: 미팅 시작
	if err := runMeetingStart(client, &buf, "meet-001"); err != nil {
		t.Fatalf("미팅 시작 실패: %v", err)
	}
	buf.Reset()

	// 스텝 2: 미팅 메시지 조회
	if err := runMeetingMessages(client, &buf, "meet-001", false); err != nil {
		t.Fatalf("미팅 메시지 조회 실패: %v", err)
	}
	if !strings.Contains(buf.String(), "마이크로서비스로 전환 제안") {
		t.Errorf("메시지 출력에 내용이 없습니다: %s", buf.String())
	}
	buf.Reset()

	// 스텝 3: 미팅 종료
	if err := runMeetingEnd(client, &buf, "meet-001"); err != nil {
		t.Fatalf("미팅 종료 실패: %v", err)
	}
	if !strings.Contains(buf.String(), "ended") {
		t.Errorf("미팅 종료 출력에 상태가 없습니다: %s", buf.String())
	}
	buf.Reset()

	// 스텝 4: 의사결정 생성
	if err := runDecisionCreate(client, &buf, "마이크로서비스 전환", "아키텍처 리뷰 미팅에서 논의", "agent-1", []string{"agent-1", "agent-2"}, false); err != nil {
		t.Fatalf("의사결정 생성 실패: %v", err)
	}
	if !strings.Contains(buf.String(), "dec-001") {
		t.Errorf("의사결정 생성 출력에 ID가 없습니다: %s", buf.String())
	}
	buf.Reset()

	// 스텝 5: 의사결정 투표
	if err := runDecisionVote(client, &buf, "dec-001", false); err != nil {
		t.Fatalf("의사결정 투표 실패: %v", err)
	}
	buf.Reset()

	// 스텝 6: 의사결정 해결
	if err := runDecisionResolve(client, &buf, "dec-001", "approved", "팀 전원 동의", "agent-1"); err != nil {
		t.Fatalf("의사결정 해결 실패: %v", err)
	}
	if !strings.Contains(buf.String(), "resolved") {
		t.Errorf("의사결정 해결 출력에 상태가 없습니다: %s", buf.String())
	}
	buf.Reset()

	// 스텝 7: 태스크 생성
	if err := runTaskCreate(client, &buf, "서비스 분리 구현", "code", 1, "agent-1", "모놀리스에서 주문 서비스 분리", false); err != nil {
		t.Fatalf("태스크 생성 실패: %v", err)
	}
	if !strings.Contains(buf.String(), "task-001") {
		t.Errorf("태스크 생성 출력에 ID가 없습니다: %s", buf.String())
	}
	buf.Reset()

	// 스텝 8: 태스크 할당
	if err := runTaskAssign(client, &buf, "task-001", "agent-1", false); err != nil {
		t.Fatalf("태스크 할당 실패: %v", err)
	}
	if !strings.Contains(buf.String(), "할당 완료") {
		t.Errorf("태스크 할당 출력에 상태가 없습니다: %s", buf.String())
	}
	buf.Reset()

	// 스텝 9: 태스크 시작
	if err := runTaskStart(client, &buf, "task-001"); err != nil {
		t.Fatalf("태스크 시작 실패: %v", err)
	}
	buf.Reset()

	// 스텝 10: 태스크 완료
	if err := runTaskComplete(client, &buf, "task-001", `{"result":"success"}`); err != nil {
		t.Fatalf("태스크 완료 실패: %v", err)
	}
	if !strings.Contains(buf.String(), "completed") {
		t.Errorf("태스크 완료 출력에 상태가 없습니다: %s", buf.String())
	}
}

// TestWorkflow_SprintDrivenDevelopment는 스프린트 기반 개발 워크플로우를 테스트합니다.
// 흐름: 스프린트 생성 → 이슈 추가 → 태스크 생성 → 할당 → 시작 → 완료 → 스프린트 완료
func TestWorkflow_SprintDrivenDevelopment(t *testing.T) {
	t.Parallel()

	// 7단계 stateful 서버 구성
	steps := []ServerStep{
		// 스텝 0: 스프린트 생성
		{
			Method:     http.MethodPost,
			PathPrefix: "/api/v1/projects/proj-1/sprints",
			ResponseData: Sprint{
				ID:         "sprint-001",
				Name:       "Sprint 2026-Q1",
				Goal:       "핵심 기능 구현",
				Status:     "planned",
				StartDate:  "2026-03-11",
				EndDate:    "2026-03-25",
				IssueCount: 0,
			},
		},
		// 스텝 1: 스프린트에 이슈 추가
		{
			Method:     http.MethodPost,
			PathPrefix: "/api/v1/sprints/sprint-001/issues/issue-101",
			ResponseData: map[string]interface{}{
				"ok": true,
			},
		},
		// 스텝 2: 태스크 생성
		{
			Method:     http.MethodPost,
			PathPrefix: "/api/v1/workspaces/ws-1/agent-tasks",
			ResponseData: AgentTask{
				ID:          "task-001",
				Title:       "이슈 issue-101 구현",
				Description: "핵심 기능 구현 태스크",
				TaskType:    "code",
				Priority:    1,
				Status:      "pending",
				AgentID:     "agent-1",
				AgentName:   "Backend Agent",
				CreatedAt:   "2026-03-11T12:00:00Z",
			},
		},
		// 스텝 3: 태스크 할당
		{
			Method:     http.MethodPatch,
			PathPrefix: "/api/v1/agent-tasks/task-001/assign",
			ResponseData: AgentTask{
				ID:        "task-001",
				Title:     "이슈 issue-101 구현",
				Status:    "assigned",
				AgentID:   "agent-1",
				AgentName: "Backend Agent",
			},
		},
		// 스텝 4: 태스크 시작
		{
			Method:     http.MethodPost,
			PathPrefix: "/api/v1/agent-tasks/task-001/start",
			ResponseData: AgentTask{
				ID:     "task-001",
				Title:  "이슈 issue-101 구현",
				Status: "in_progress",
			},
		},
		// 스텝 5: 태스크 완료
		{
			Method:     http.MethodPost,
			PathPrefix: "/api/v1/agent-tasks/task-001/complete",
			ResponseData: AgentTask{
				ID:     "task-001",
				Title:  "이슈 issue-101 구현",
				Status: "completed",
			},
		},
		// 스텝 6: 스프린트 완료
		{
			Method:     http.MethodPost,
			PathPrefix: "/api/v1/sprints/sprint-001/complete",
			ResponseData: Sprint{
				ID:         "sprint-001",
				Name:       "Sprint 2026-Q1",
				Goal:       "핵심 기능 구현",
				Status:     "completed",
				StartDate:  "2026-03-11",
				EndDate:    "2026-03-25",
				IssueCount: 1,
			},
		},
	}

	srv := buildStatefulServer(t, steps)
	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	// 스텝 0: 스프린트 생성
	if err := runSprintCreate(client, &buf, "proj-1", "Sprint 2026-Q1", "핵심 기능 구현", "2026-03-11", "2026-03-25", false); err != nil {
		t.Fatalf("스프린트 생성 실패: %v", err)
	}
	if !strings.Contains(buf.String(), "sprint-001") {
		t.Errorf("스프린트 생성 출력에 ID가 없습니다: %s", buf.String())
	}
	buf.Reset()

	// 스텝 1: 이슈 추가
	if err := runSprintAddIssue(client, &buf, "sprint-001", "issue-101"); err != nil {
		t.Fatalf("이슈 추가 실패: %v", err)
	}
	buf.Reset()

	// 스텝 2: 태스크 생성
	if err := runTaskCreate(client, &buf, "이슈 issue-101 구현", "code", 1, "agent-1", "핵심 기능 구현 태스크", false); err != nil {
		t.Fatalf("태스크 생성 실패: %v", err)
	}
	if !strings.Contains(buf.String(), "task-001") {
		t.Errorf("태스크 생성 출력에 ID가 없습니다: %s", buf.String())
	}
	buf.Reset()

	// 스텝 3: 태스크 할당
	if err := runTaskAssign(client, &buf, "task-001", "agent-1", false); err != nil {
		t.Fatalf("태스크 할당 실패: %v", err)
	}
	buf.Reset()

	// 스텝 4: 태스크 시작
	if err := runTaskStart(client, &buf, "task-001"); err != nil {
		t.Fatalf("태스크 시작 실패: %v", err)
	}
	buf.Reset()

	// 스텝 5: 태스크 완료
	if err := runTaskComplete(client, &buf, "task-001", `{"result":"done"}`); err != nil {
		t.Fatalf("태스크 완료 실패: %v", err)
	}
	if !strings.Contains(buf.String(), "completed") {
		t.Errorf("태스크 완료 출력에 상태가 없습니다: %s", buf.String())
	}
	buf.Reset()

	// 스텝 6: 스프린트 완료
	if err := runSprintComplete(client, &buf, "sprint-001"); err != nil {
		t.Fatalf("스프린트 완료 실패: %v", err)
	}
	if !strings.Contains(buf.String(), "completed") {
		t.Errorf("스프린트 완료 출력에 상태가 없습니다: %s", buf.String())
	}
}
