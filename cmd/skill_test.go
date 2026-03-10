// skill_test.go는 skill 서브커맨드 핸들러 함수를 테스트합니다.
package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunSkillList(t *testing.T) {
	skills := []SkillEntry{
		{ID: "skill-1", Name: "코드리뷰", Version: "1.0.0", Status: "active"},
		{ID: "skill-2", Name: "테스트생성", Version: "2.0.0", Status: "active"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/skills/registry" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(skills))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runSkillList(client, &buf, false)
	if err != nil {
		t.Fatalf("runSkillList 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "코드리뷰") {
		t.Errorf("출력에 '코드리뷰'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "테스트생성") {
		t.Errorf("출력에 '테스트생성'이 없습니다: %s", out)
	}
}

func TestRunSkillListJSON(t *testing.T) {
	skills := []SkillEntry{
		{ID: "skill-1", Name: "코드리뷰", Version: "1.0.0", Status: "active"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(skills))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runSkillList(client, &buf, true)
	if err != nil {
		t.Fatalf("runSkillList JSON 오류: %v", err)
	}

	var parsed []SkillEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].ID != "skill-1" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

func TestRunSkillShow(t *testing.T) {
	skill := SkillEntry{ID: "skill-1", Name: "코드리뷰", Version: "1.0.0", Status: "active", Description: "코드 리뷰 스킬"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/skills/registry/skill-1" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(skill))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runSkillShow(client, &buf, "skill-1", false)
	if err != nil {
		t.Fatalf("runSkillShow 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "코드리뷰") {
		t.Errorf("출력에 '코드리뷰'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "코드 리뷰 스킬") {
		t.Errorf("출력에 '코드 리뷰 스킬'이 없습니다: %s", out)
	}
}

func TestRunSkillShowJSON(t *testing.T) {
	skill := SkillEntry{ID: "skill-1", Name: "코드리뷰", Version: "1.0.0", Status: "active"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(skill))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runSkillShow(client, &buf, "skill-1", true)
	if err != nil {
		t.Fatalf("runSkillShow JSON 오류: %v", err)
	}

	var parsed SkillEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.ID != "skill-1" {
		t.Errorf("예상치 않은 ID: %s", parsed.ID)
	}
}

func TestRunSkillShow_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runSkillShow(client, &buf, "bad/id", false)
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

func TestRunSkillSync(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/skills/sync" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(map[string]interface{}{"message": "synced"}))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runSkillSync(client, &buf)
	if err != nil {
		t.Fatalf("runSkillSync 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "스킬 동기화 완료") {
		t.Errorf("출력에 '스킬 동기화 완료'가 없습니다: %s", out)
	}
}

func TestRunSkillSync_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"error":"서버 오류"}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runSkillSync(client, &buf)
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}

func TestRunSkillQuality(t *testing.T) {
	quality := SkillQuality{
		SkillID:     "skill-1",
		Score:       0.95,
		Executions:  100,
		SuccessRate: 0.98,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/skills/registry/skill-1/quality" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(quality))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runSkillQuality(client, &buf, "skill-1")
	if err != nil {
		t.Fatalf("runSkillQuality 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "skill-1") {
		t.Errorf("출력에 'skill-1'이 없습니다: %s", out)
	}
}

func TestRunSkillQuality_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runSkillQuality(client, &buf, "bad id")
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

func TestRunSkillVersions(t *testing.T) {
	versions := []SkillVersion{
		{ID: "ver-1", SkillID: "skill-1", Version: "1.0.0"},
		{ID: "ver-2", SkillID: "skill-1", Version: "2.0.0"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/skills/registry/skill-1/versions" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(versions))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runSkillVersions(client, &buf, "skill-1")
	if err != nil {
		t.Fatalf("runSkillVersions 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "1.0.0") {
		t.Errorf("출력에 '1.0.0'이 없습니다: %s", out)
	}
	if !strings.Contains(out, "2.0.0") {
		t.Errorf("출력에 '2.0.0'이 없습니다: %s", out)
	}
}

func TestRunSkillVersions_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runSkillVersions(client, &buf, "bad/id")
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	}
}

func TestRunSkillRollback(t *testing.T) {
	skill := SkillEntry{ID: "skill-1", Name: "코드리뷰", Version: "1.0.0", Status: "active"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/skills/registry/skill-1/versions/ver-1/rollback" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(skill))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runSkillRollback(client, &buf, "skill-1", "ver-1")
	if err != nil {
		t.Fatalf("runSkillRollback 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "코드리뷰") {
		t.Errorf("출력에 '코드리뷰'가 없습니다: %s", out)
	}
}

func TestRunSkillRollback_InvalidSkillID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runSkillRollback(client, &buf, "bad/id", "ver-1")
	if err == nil {
		t.Error("유효하지 않은 skillID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

func TestRunSkillRollback_InvalidVersionID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runSkillRollback(client, &buf, "skill-1", "bad id")
	if err == nil {
		t.Error("유효하지 않은 versionID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

func TestRunSkillExecutions(t *testing.T) {
	executions := []SkillExecution{
		{ID: "exec-1", SkillID: "skill-1", Status: "success", Duration: "1.2s"},
		{ID: "exec-2", SkillID: "skill-1", Status: "failed", Duration: "0.5s"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/skills/registry/skill-1/executions" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(executions))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runSkillExecutions(client, &buf, "skill-1")
	if err != nil {
		t.Fatalf("runSkillExecutions 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "success") {
		t.Errorf("출력에 'success'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "failed") {
		t.Errorf("출력에 'failed'가 없습니다: %s", out)
	}
}

func TestRunSkillExecutions_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runSkillExecutions(client, &buf, "bad/id")
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	}
}

func TestRunAgentSkills(t *testing.T) {
	agentSkills := []AgentSkill{
		{ID: "as-1", SkillID: "skill-1", Name: "코드리뷰", Level: "expert"},
		{ID: "as-2", SkillID: "skill-2", Name: "테스트생성", Level: "intermediate"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agents/agent-1/skills" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(agentSkills))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runAgentSkills(client, &buf, "agent-1", false)
	if err != nil {
		t.Fatalf("runAgentSkills 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "코드리뷰") {
		t.Errorf("출력에 '코드리뷰'가 없습니다: %s", out)
	}
}

func TestRunAgentSkillsJSON(t *testing.T) {
	agentSkills := []AgentSkill{
		{ID: "as-1", SkillID: "skill-1", Name: "코드리뷰", Level: "expert"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(agentSkills))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runAgentSkills(client, &buf, "agent-1", true)
	if err != nil {
		t.Fatalf("runAgentSkills JSON 오류: %v", err)
	}

	var parsed []AgentSkill
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].ID != "as-1" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

func TestRunAgentSkills_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runAgentSkills(client, &buf, "bad/id", false)
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	}
}

func TestRunSkillAssign(t *testing.T) {
	agentSkill := AgentSkill{ID: "as-new", SkillID: "skill-1", Name: "코드리뷰"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agents/agent-1/skills" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(agentSkill))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runSkillAssign(client, &buf, "agent-1", "skill-1")
	if err != nil {
		t.Fatalf("runSkillAssign 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "코드리뷰") {
		t.Errorf("출력에 '코드리뷰'가 없습니다: %s", out)
	}
}

func TestRunSkillAssign_InvalidAgentID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runSkillAssign(client, &buf, "bad/id", "skill-1")
	if err == nil {
		t.Error("유효하지 않은 agentID에서 에러가 발생해야 합니다")
	}
}

func TestRunSkillAssign_InvalidSkillID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runSkillAssign(client, &buf, "agent-1", "bad id")
	if err == nil {
		t.Error("유효하지 않은 skillID에서 에러가 발생해야 합니다")
	}
}

func TestRunSkillUnassign(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agents/agent-1/skills/skill-1" || r.Method != http.MethodDelete {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(map[string]string{"message": "unassigned"}))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runSkillUnassign(client, &buf, "agent-1", "skill-1")
	if err != nil {
		t.Fatalf("runSkillUnassign 오류: %v", err)
	}
}

func TestRunSkillUnassign_InvalidAgentID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runSkillUnassign(client, &buf, "bad/id", "skill-1")
	if err == nil {
		t.Error("유효하지 않은 agentID에서 에러가 발생해야 합니다")
	}
}

func TestRunSkillUnassign_InvalidSkillID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runSkillUnassign(client, &buf, "agent-1", "bad id")
	if err == nil {
		t.Error("유효하지 않은 skillID에서 에러가 발생해야 합니다")
	}
}

func TestRunSkillRecommend(t *testing.T) {
	recommendations := []SkillRecommendation{
		{SkillID: "skill-3", Name: "문서화", Score: 0.9, Reason: "문서 작성 빈도 높음"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agents/agent-1/skills/recommendations" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(recommendations))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runSkillRecommend(client, &buf, "agent-1")
	if err != nil {
		t.Fatalf("runSkillRecommend 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "문서화") {
		t.Errorf("출력에 '문서화'가 없습니다: %s", out)
	}
}

func TestRunSkillRecommend_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runSkillRecommend(client, &buf, "bad/id")
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	}
}

func TestRunSkillAutoAssign(t *testing.T) {
	agentSkills := []AgentSkill{
		{ID: "as-auto-1", SkillID: "skill-1", Name: "코드리뷰"},
		{ID: "as-auto-2", SkillID: "skill-2", Name: "테스트생성"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agents/agent-1/skills/auto-assign" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(agentSkills))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runSkillAutoAssign(client, &buf, "agent-1")
	if err != nil {
		t.Fatalf("runSkillAutoAssign 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "코드리뷰") {
		t.Errorf("출력에 '코드리뷰'가 없습니다: %s", out)
	}
}

func TestRunSkillAutoAssign_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runSkillAutoAssign(client, &buf, "bad/id")
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	}
}
