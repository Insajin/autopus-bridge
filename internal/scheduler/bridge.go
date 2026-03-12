package scheduler

import (
	"context"
	"fmt"

	"github.com/insajin/autopus-bridge/internal/apiclient"
)

// APIScheduleFetcher는 apiclient.Client를 사용하여 스케줄을 가져옵니다.
type APIScheduleFetcher struct {
	client *apiclient.Client
}

// NewAPIScheduleFetcher는 새 APIScheduleFetcher를 생성합니다.
func NewAPIScheduleFetcher(client *apiclient.Client) *APIScheduleFetcher {
	return &APIScheduleFetcher{client: client}
}

// FetchSchedules는 서버에서 활성 스케줄 목록을 가져옵니다.
func (f *APIScheduleFetcher) FetchSchedules(ctx context.Context) ([]ScheduleInfo, error) {
	workspaceID := f.client.WorkspaceID()
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID가 설정되지 않았습니다")
	}

	schedules, err := apiclient.DoList[ScheduleInfo](f.client, ctx,
		"GET", "/api/v1/workspaces/"+workspaceID+"/schedules", nil)
	if err != nil {
		return nil, fmt.Errorf("스케줄 목록 조회 실패: %w", err)
	}

	return schedules, nil
}

// APITaskTrigger는 apiclient.Client를 사용하여 태스크 실행을 트리거합니다.
type APITaskTrigger struct {
	client *apiclient.Client
}

// NewAPITaskTrigger는 새 APITaskTrigger를 생성합니다.
func NewAPITaskTrigger(client *apiclient.Client) *APITaskTrigger {
	return &APITaskTrigger{client: client}
}

// TriggerExecution은 지정된 에이전트에 태스크 실행을 요청합니다.
func (t *APITaskTrigger) TriggerExecution(ctx context.Context, agentID, prompt string) error {
	workspaceID := t.client.WorkspaceID()
	if workspaceID == "" {
		return fmt.Errorf("workspace ID가 설정되지 않았습니다")
	}

	body := map[string]interface{}{
		"agent_id": agentID,
		"prompt":   prompt,
	}

	_, err := apiclient.Do[map[string]interface{}](t.client, ctx,
		"POST", "/api/v1/workspaces/"+workspaceID+"/execute", body)
	if err != nil {
		return fmt.Errorf("태스크 실행 트리거 실패: %w", err)
	}

	return nil
}
