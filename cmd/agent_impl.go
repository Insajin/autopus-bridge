// agent_impl.go는 agent.go에서 선언한 커맨드들의 핸들러 구현을 제공합니다.
// create/update/delete/toggle/provider/set-provider 서브커맨드 핸들러
package cmd

import (
	"fmt"
	"io"

	"github.com/insajin/autopus-bridge/internal/apiclient"
)

// runAgentCreate는 새 에이전트를 생성하고 결과를 출력합니다.
func runAgentCreate(client *apiclient.Client, out io.Writer, name, agentType, tier, persona, model, provider string, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()
	body := map[string]string{"name": name}
	if agentType != "" {
		body["type"] = agentType
	}
	if tier != "" {
		body["tier"] = tier
	}
	if persona != "" {
		body["persona"] = persona
	}
	if model != "" {
		body["model"] = model
	}
	if provider != "" {
		body["provider"] = provider
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	agent, err := apiclient.Do[AgentDetail](client, ctx, "POST",
		"/api/v1/workspaces/"+workspaceID+"/agents", body)
	if err != nil {
		return fmt.Errorf("에이전트 생성 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, agent)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: agent.ID},
		{Key: "Name", Value: agent.Name},
		{Key: "Status", Value: agent.Status},
		{Key: "Model", Value: agent.Model},
		{Key: "Provider", Value: agent.Provider},
	})
	return nil
}

// runAgentUpdate는 에이전트 정보를 수정하고 결과를 출력합니다.
func runAgentUpdate(client *apiclient.Client, out io.Writer, agentID, name, persona, model, provider, status string, jsonOutput bool) error {
	if err := apiclient.ValidateID(agentID); err != nil {
		return err
	}

	body := map[string]string{}
	if name != "" {
		body["name"] = name
	}
	if persona != "" {
		body["persona"] = persona
	}
	if model != "" {
		body["model"] = model
	}
	if provider != "" {
		body["provider"] = provider
	}
	if status != "" {
		body["status"] = status
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	agent, err := apiclient.Do[AgentDetail](client, ctx, "PATCH",
		"/api/v1/agents/"+agentID, body)
	if err != nil {
		return fmt.Errorf("에이전트 수정 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, agent)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: agent.ID},
		{Key: "Name", Value: agent.Name},
		{Key: "Status", Value: agent.Status},
		{Key: "Model", Value: agent.Model},
		{Key: "Provider", Value: agent.Provider},
	})
	return nil
}

// runAgentDelete는 에이전트를 삭제합니다.
func runAgentDelete(client *apiclient.Client, out io.Writer, agentID string) error {
	if err := apiclient.ValidateID(agentID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	_, err := apiclient.Do[map[string]interface{}](client, ctx, "DELETE",
		"/api/v1/agents/"+agentID, nil)
	if err != nil {
		return fmt.Errorf("에이전트 삭제 실패: %w", err)
	}

	fmt.Fprintf(out, "에이전트 삭제 완료: %s\n", agentID)
	return nil
}

// runAgentToggle는 에이전트의 활성/비활성 상태를 전환하고 결과를 출력합니다.
func runAgentToggle(client *apiclient.Client, out io.Writer, agentID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(agentID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	agent, err := apiclient.Do[AgentDetail](client, ctx, "PATCH",
		"/api/v1/agents/"+agentID+"/toggle", nil)
	if err != nil {
		return fmt.Errorf("에이전트 상태 전환 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, agent)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: agent.ID},
		{Key: "Name", Value: agent.Name},
		{Key: "Status", Value: agent.Status},
	})
	return nil
}

// runAgentProvider는 에이전트 프로바이더 설정을 조회하고 출력합니다.
func runAgentProvider(client *apiclient.Client, out io.Writer, agentID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(agentID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	cfg, err := apiclient.Do[AgentProviderConfig](client, ctx, "GET",
		"/api/v1/agents/"+agentID+"/provider", nil)
	if err != nil {
		return fmt.Errorf("에이전트 프로바이더 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, cfg)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "AgentID", Value: cfg.AgentID},
		{Key: "Provider", Value: cfg.Provider},
		{Key: "Model", Value: cfg.Model},
	})
	return nil
}

// runAgentSetProvider는 에이전트 프로바이더를 설정하고 결과를 출력합니다.
func runAgentSetProvider(client *apiclient.Client, out io.Writer, agentID, provider, model string, jsonOutput bool) error {
	if err := apiclient.ValidateID(agentID); err != nil {
		return err
	}

	body := map[string]string{
		"provider": provider,
		"model":    model,
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	cfg, err := apiclient.Do[AgentProviderConfig](client, ctx, "PUT",
		"/api/v1/agents/"+agentID+"/provider", body)
	if err != nil {
		return fmt.Errorf("에이전트 프로바이더 설정 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, cfg)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "AgentID", Value: cfg.AgentID},
		{Key: "Provider", Value: cfg.Provider},
		{Key: "Model", Value: cfg.Model},
	})
	return nil
}
