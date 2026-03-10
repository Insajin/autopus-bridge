// anomaly_test.go는 anomaly 서브커맨드 핸들러 함수를 테스트합니다.
package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunAnomalyList(t *testing.T) {
	// 테스트 이상 목록
	anomalies := []Anomaly{
		{ID: "anm-1", Type: "spike", Severity: "high", Status: "open", DetectedAt: "2024-01-01T00:00:00Z"},
		{ID: "anm-2", Type: "drift", Severity: "low", Status: "resolved", DetectedAt: "2024-01-02T00:00:00Z"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/anomalies" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(anomalies))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runAnomalyList(client, &buf, false)
	if err != nil {
		t.Fatalf("runAnomalyList 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "spike") {
		t.Errorf("출력에 'spike'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "drift") {
		t.Errorf("출력에 'drift'가 없습니다: %s", out)
	}
}

func TestRunAnomalyListJSON(t *testing.T) {
	anomalies := []Anomaly{
		{ID: "anm-1", Type: "spike", Severity: "high", Status: "open"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(anomalies))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runAnomalyList(client, &buf, true)
	if err != nil {
		t.Fatalf("runAnomalyList JSON 오류: %v", err)
	}

	var parsed []Anomaly
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].ID != "anm-1" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

func TestRunAnomalyListError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"error":"server error"}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runAnomalyList(client, &buf, false)
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}

func TestRunAnomalyDetect(t *testing.T) {
	// detect는 POST 후 이상 목록을 반환
	detected := []Anomaly{
		{ID: "anm-3", Type: "outlier", Severity: "medium", Status: "open"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/anomalies" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(detected))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runAnomalyDetect(client, &buf, false)
	if err != nil {
		t.Fatalf("runAnomalyDetect 오류: %v", err)
	}

	out := buf.String()
	// "이상 탐지 완료: N건 발견" 메시지 또는 테이블 출력 확인
	if !strings.Contains(out, "이상 탐지 완료") && !strings.Contains(out, "outlier") {
		t.Errorf("출력에 탐지 결과가 없습니다: %s", out)
	}
}

func TestRunAnomalyDetectJSON(t *testing.T) {
	detected := []Anomaly{
		{ID: "anm-3", Type: "outlier", Severity: "medium", Status: "open"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(detected))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runAnomalyDetect(client, &buf, true)
	if err != nil {
		t.Fatalf("runAnomalyDetect JSON 오류: %v", err)
	}

	var parsed []Anomaly
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].ID != "anm-3" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

func TestRunAnomalyAcknowledge(t *testing.T) {
	anomaly := Anomaly{ID: "anm-1", Type: "spike", Severity: "high", Status: "acknowledged"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/anomalies/anm-1/acknowledge" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(anomaly))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runAnomalyAcknowledge(client, &buf, "anm-1", false)
	if err != nil {
		t.Fatalf("runAnomalyAcknowledge 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "acknowledged") {
		t.Errorf("출력에 'acknowledged'가 없습니다: %s", out)
	}
}

func TestRunAnomalyAcknowledge_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runAnomalyAcknowledge(client, &buf, "bad/id", false)
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

func TestRunAnomalyAcknowledgeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"error":"not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runAnomalyAcknowledge(client, &buf, "anm-nonexistent", false)
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}

func TestRunAnomalyResolve(t *testing.T) {
	anomaly := Anomaly{ID: "anm-1", Type: "spike", Severity: "high", Status: "resolved"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/anomalies/anm-1/resolve" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(anomaly))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runAnomalyResolve(client, &buf, "anm-1", false)
	if err != nil {
		t.Fatalf("runAnomalyResolve 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "resolved") {
		t.Errorf("출력에 'resolved'가 없습니다: %s", out)
	}
}

func TestRunAnomalyResolve_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runAnomalyResolve(client, &buf, "id with space", false)
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

func TestRunAnomalyResolveError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"error":"not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runAnomalyResolve(client, &buf, "anm-nonexistent", false)
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}

func TestRunAnomalyDetectEmpty(t *testing.T) {
	// 탐지 결과가 0건인 경우
	detected := []Anomaly{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(detected))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runAnomalyDetect(client, &buf, false)
	if err != nil {
		t.Fatalf("runAnomalyDetect 오류: %v", err)
	}

	out := buf.String()
	// 0건이어도 결과 표시
	if !strings.Contains(out, "이상 탐지 완료") && !strings.Contains(out, fmt.Sprintf("%d", 0)) {
		t.Errorf("0건 탐지 결과가 표시되지 않습니다: %s", out)
	}
}
