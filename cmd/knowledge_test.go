// knowledge_test.go는 knowledge 서브커맨드 핸들러 함수의 단위 테스트를 제공합니다.
package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---- 지식 항목(Entry) 관련 테스트 ----

// TestRunKnowledgeList는 지식 항목 목록 조회를 검증합니다.
func TestRunKnowledgeList(t *testing.T) {
	importance := 5
	entries := []KnowledgeEntry{
		{ID: "kn-1111-2222", Title: "AI 전략", Category: "strategy", SourceType: "decision", Importance: &importance},
		{ID: "kn-3333-4444", Title: "시장 분석", Category: "market", SourceType: "analysis"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/knowledge" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		// SuccessPage 응답 형식: meta 포함
		resp := map[string]interface{}{
			"success": true,
			"data":    entries,
			"meta":    map[string]interface{}{"page": 1, "page_size": 20, "total": 2, "total_pages": 1},
		}
		b, _ := json.Marshal(resp)
		w.Write(b)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runKnowledgeList(client, &buf, "", "", nil, 0, 0, 0, false)
	if err != nil {
		t.Fatalf("runKnowledgeList 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "AI 전략") {
		t.Errorf("출력에 'AI 전략'이 없습니다: %s", out)
	}
	if !strings.Contains(out, "시장 분석") {
		t.Errorf("출력에 '시장 분석'이 없습니다: %s", out)
	}
}

// TestRunKnowledgeListJSON은 지식 항목 목록 JSON 출력을 검증합니다.
func TestRunKnowledgeListJSON(t *testing.T) {
	entries := []KnowledgeEntry{
		{ID: "kn-1111-2222", Title: "AI 전략", Category: "strategy"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"success": true,
			"data":    entries,
			"meta":    map[string]interface{}{"page": 1, "page_size": 20, "total": 1, "total_pages": 1},
		}
		b, _ := json.Marshal(resp)
		w.Write(b)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runKnowledgeList(client, &buf, "", "", nil, 0, 0, 0, true)
	if err != nil {
		t.Fatalf("runKnowledgeList JSON 오류: %v", err)
	}

	var parsed []KnowledgeEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].ID != "kn-1111-2222" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

// TestRunKnowledgeListWithFilters는 필터 파라미터 전달을 검증합니다.
func TestRunKnowledgeListWithFilters(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"success": true,
			"data":    []KnowledgeEntry{},
			"meta":    map[string]interface{}{"page": 1, "page_size": 20, "total": 0, "total_pages": 0},
		}
		b, _ := json.Marshal(resp)
		w.Write(b)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runKnowledgeList(client, &buf, "strategy", "decision", []string{"ai", "ml"}, 3, 1, 20, false)
	if err != nil {
		t.Fatalf("runKnowledgeList with filters 오류: %v", err)
	}

	if !strings.Contains(capturedQuery, "category=strategy") {
		t.Errorf("쿼리에 category=strategy가 없습니다: %s", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "source_type=decision") {
		t.Errorf("쿼리에 source_type=decision이 없습니다: %s", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "importance=3") {
		t.Errorf("쿼리에 importance=3이 없습니다: %s", capturedQuery)
	}
}

// TestRunKnowledgeShow는 지식 항목 상세 조회를 검증합니다.
func TestRunKnowledgeShow(t *testing.T) {
	importance := 8
	entry := KnowledgeEntry{
		ID:         "kn-1111-2222",
		Title:      "AI 전략 문서",
		Content:    "GPT-4를 활용한 전략",
		Category:   "strategy",
		SourceType: "decision",
		Tags:       []string{"ai", "strategy"},
		Importance: &importance,
		CreatedAt:  "2026-01-01T00:00:00Z",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/knowledge/kn-1111-2222" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(entry))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runKnowledgeShow(client, &buf, "kn-1111-2222", false)
	if err != nil {
		t.Fatalf("runKnowledgeShow 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "AI 전략 문서") {
		t.Errorf("출력에 'AI 전략 문서'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "strategy") {
		t.Errorf("출력에 'strategy'가 없습니다: %s", out)
	}
}

// TestRunKnowledgeShow_InvalidID는 잘못된 ID 검증을 확인합니다.
func TestRunKnowledgeShow_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runKnowledgeShow(client, &buf, "bad/id", false)
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunKnowledgeSearch는 지식 검색을 검증합니다.
func TestRunKnowledgeSearch(t *testing.T) {
	results := []KnowledgeSearchResult{
		{ID: "kn-1111-2222", Title: "AI 전략", Score: 0.95},
		{ID: "kn-3333-4444", Title: "머신러닝 기초", Score: 0.82},
	}

	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/knowledge/search" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(results))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runKnowledgeSearch(client, &buf, "AI 전략", 10, "strategy", nil, false)
	if err != nil {
		t.Fatalf("runKnowledgeSearch 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "AI 전략") {
		t.Errorf("출력에 'AI 전략'이 없습니다: %s", out)
	}
	if !strings.Contains(out, "머신러닝 기초") {
		t.Errorf("출력에 '머신러닝 기초'가 없습니다: %s", out)
	}

	// 요청 본문에 query 포함 확인
	if capturedBody["query"] != "AI 전략" {
		t.Errorf("요청 본문 query = %v, want 'AI 전략'", capturedBody["query"])
	}
}

// TestRunKnowledgeSearchJSON은 검색 JSON 출력을 검증합니다.
func TestRunKnowledgeSearchJSON(t *testing.T) {
	results := []KnowledgeSearchResult{
		{ID: "kn-1111-2222", Title: "AI 전략", Score: 0.95},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(results))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runKnowledgeSearch(client, &buf, "AI", 5, "", nil, true)
	if err != nil {
		t.Fatalf("runKnowledgeSearch JSON 오류: %v", err)
	}

	var parsed []KnowledgeSearchResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].ID != "kn-1111-2222" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

// TestRunKnowledgeCreate는 지식 항목 생성을 검증합니다.
func TestRunKnowledgeCreate(t *testing.T) {
	importance := 7
	entry := KnowledgeEntry{
		ID:         "kn-new-1234",
		Title:      "새 전략 문서",
		Content:    "내용",
		Category:   "strategy",
		SourceType: "decision",
		Importance: &importance,
	}

	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/knowledge" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(entry))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runKnowledgeCreate(client, &buf, "새 전략 문서", "내용", "strategy", "decision", []string{"ai"}, 7, false)
	if err != nil {
		t.Fatalf("runKnowledgeCreate 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "새 전략 문서") {
		t.Errorf("출력에 '새 전략 문서'가 없습니다: %s", out)
	}

	if capturedBody["title"] != "새 전략 문서" {
		t.Errorf("요청 본문 title = %v, want '새 전략 문서'", capturedBody["title"])
	}
	if capturedBody["category"] != "strategy" {
		t.Errorf("요청 본문 category = %v, want 'strategy'", capturedBody["category"])
	}
}

// TestRunKnowledgeUpdate는 지식 항목 수정을 검증합니다.
func TestRunKnowledgeUpdate(t *testing.T) {
	entry := KnowledgeEntry{
		ID:    "kn-1111-2222",
		Title: "수정된 제목",
	}

	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/knowledge/kn-1111-2222" || r.Method != http.MethodPatch {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(entry))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runKnowledgeUpdate(client, &buf, "kn-1111-2222", "수정된 제목", "", "", nil, false)
	if err != nil {
		t.Fatalf("runKnowledgeUpdate 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "수정된 제목") {
		t.Errorf("출력에 '수정된 제목'이 없습니다: %s", out)
	}

	if capturedBody["title"] != "수정된 제목" {
		t.Errorf("요청 본문 title = %v, want '수정된 제목'", capturedBody["title"])
	}
}

// TestRunKnowledgeUpdate_InvalidID는 잘못된 ID 검증을 확인합니다.
func TestRunKnowledgeUpdate_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runKnowledgeUpdate(client, &buf, "bad/id", "제목", "", "", nil, false)
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	}
}

// TestRunKnowledgeDelete는 지식 항목 삭제를 검증합니다.
func TestRunKnowledgeDelete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/knowledge/kn-1111-2222" || r.Method != http.MethodDelete {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(map[string]bool{"deleted": true}))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runKnowledgeDelete(client, &buf, "kn-1111-2222")
	if err != nil {
		t.Fatalf("runKnowledgeDelete 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "kn-1111-2222") {
		t.Errorf("출력에 항목 ID가 없습니다: %s", out)
	}
}

// TestRunKnowledgeDelete_InvalidID는 잘못된 ID 검증을 확인합니다.
func TestRunKnowledgeDelete_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runKnowledgeDelete(client, &buf, "bad/id")
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	}
}

// TestRunKnowledgeUpload는 파일 업로드를 검증합니다.
func TestRunKnowledgeUpload(t *testing.T) {
	// 임시 파일 생성
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(filePath, []byte("테스트 내용"), 0600); err != nil {
		t.Fatalf("임시 파일 생성 실패: %v", err)
	}

	entry := KnowledgeEntry{
		ID:    "kn-upload-1234",
		Title: "test.txt",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/knowledge/upload" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(entry))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runKnowledgeUpload(client, &buf, filePath, "strategy", false)
	if err != nil {
		t.Fatalf("runKnowledgeUpload 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "kn-upload-1234") {
		t.Errorf("출력에 업로드된 항목 ID가 없습니다: %s", out)
	}
}

// TestRunKnowledgeUpload_InvalidExtension은 허용되지 않는 파일 확장자 거부를 검증합니다.
func TestRunKnowledgeUpload_InvalidExtension(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "bad.exe")
	if err := os.WriteFile(filePath, []byte("바이너리"), 0600); err != nil {
		t.Fatalf("임시 파일 생성 실패: %v", err)
	}

	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runKnowledgeUpload(client, &buf, filePath, "", false)
	if err == nil {
		t.Error("허용되지 않는 확장자에서 에러가 발생해야 합니다")
	}
}

// TestRunKnowledgeStats는 지식 통계 조회를 검증합니다.
func TestRunKnowledgeStats(t *testing.T) {
	stats := KnowledgeStats{
		TotalEntries:      42,
		ByCategory:        map[string]int{"strategy": 10, "market": 8},
		BySourceType:      map[string]int{"decision": 15, "analysis": 12},
		AverageImportance: 6.5,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/knowledge/stats" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(stats))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runKnowledgeStats(client, &buf, false)
	if err != nil {
		t.Fatalf("runKnowledgeStats 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "42") {
		t.Errorf("출력에 총 항목 수 '42'가 없습니다: %s", out)
	}
}

// TestRunKnowledgeStatsJSON은 통계 JSON 출력을 검증합니다.
func TestRunKnowledgeStatsJSON(t *testing.T) {
	stats := KnowledgeStats{
		TotalEntries: 42,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(stats))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runKnowledgeStats(client, &buf, true)
	if err != nil {
		t.Fatalf("runKnowledgeStats JSON 오류: %v", err)
	}

	var parsed KnowledgeStats
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.TotalEntries != 42 {
		t.Errorf("예상치 않은 TotalEntries: %d", parsed.TotalEntries)
	}
}

// ---- 폴더 관련 테스트 ----

// TestRunKnowledgeFolderList는 폴더 목록 조회를 검증합니다.
func TestRunKnowledgeFolderList(t *testing.T) {
	folders := []KnowledgeFolder{
		{ID: "folder-1111-2222", Path: "/data/docs", Name: "문서", Status: "active", FileCount: 10},
		{ID: "folder-3333-4444", Path: "/data/reports", Name: "보고서", Status: "syncing", FileCount: 5},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/knowledge/folders" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		// folders가 {folders: []} 래퍼로 반환됨
		data := map[string]interface{}{"folders": folders}
		w.Write(buildAPIResponse(data))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runKnowledgeFolderList(client, &buf, false)
	if err != nil {
		t.Fatalf("runKnowledgeFolderList 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "/data/docs") {
		t.Errorf("출력에 '/data/docs'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "/data/reports") {
		t.Errorf("출력에 '/data/reports'가 없습니다: %s", out)
	}
}

// TestRunKnowledgeFolderListJSON은 폴더 목록 JSON 출력을 검증합니다.
func TestRunKnowledgeFolderListJSON(t *testing.T) {
	folders := []KnowledgeFolder{
		{ID: "folder-1111-2222", Path: "/data/docs"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		data := map[string]interface{}{"folders": folders}
		w.Write(buildAPIResponse(data))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runKnowledgeFolderList(client, &buf, true)
	if err != nil {
		t.Fatalf("runKnowledgeFolderList JSON 오류: %v", err)
	}

	var parsed []KnowledgeFolder
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].ID != "folder-1111-2222" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

// TestRunKnowledgeFolderShow는 폴더 상세 조회를 검증합니다.
func TestRunKnowledgeFolderShow(t *testing.T) {
	folder := KnowledgeFolder{
		ID:        "folder-1111-2222",
		Path:      "/data/docs",
		Name:      "문서 폴더",
		Status:    "active",
		FileCount: 15,
		CreatedAt: "2026-01-01T00:00:00Z",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/knowledge/folders/folder-1111-2222" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(folder))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runKnowledgeFolderShow(client, &buf, "folder-1111-2222", false)
	if err != nil {
		t.Fatalf("runKnowledgeFolderShow 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "/data/docs") {
		t.Errorf("출력에 '/data/docs'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "문서 폴더") {
		t.Errorf("출력에 '문서 폴더'가 없습니다: %s", out)
	}
}

// TestRunKnowledgeFolderShow_InvalidID는 잘못된 ID 검증을 확인합니다.
func TestRunKnowledgeFolderShow_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runKnowledgeFolderShow(client, &buf, "bad/id", false)
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	}
}

// TestRunKnowledgeFolderCreate는 폴더 생성을 검증합니다.
func TestRunKnowledgeFolderCreate(t *testing.T) {
	folder := KnowledgeFolder{
		ID:     "folder-new-1234",
		Path:   "/data/new-docs",
		Status: "active",
	}

	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/knowledge/folders" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(folder))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runKnowledgeFolderCreate(client, &buf, "/data/new-docs", false)
	if err != nil {
		t.Fatalf("runKnowledgeFolderCreate 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "/data/new-docs") {
		t.Errorf("출력에 '/data/new-docs'가 없습니다: %s", out)
	}

	if capturedBody["path"] != "/data/new-docs" {
		t.Errorf("요청 본문 path = %v, want '/data/new-docs'", capturedBody["path"])
	}
}

// TestRunKnowledgeFolderSync는 폴더 동기화를 검증합니다.
func TestRunKnowledgeFolderSync(t *testing.T) {
	result := FolderSyncResult{
		FolderID:     "folder-1111-2222",
		Status:       "completed",
		FilesAdded:   5,
		FilesUpdated: 2,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/knowledge/folders/folder-1111-2222/sync" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(result))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runKnowledgeFolderSync(client, &buf, "folder-1111-2222", false)
	if err != nil {
		t.Fatalf("runKnowledgeFolderSync 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "completed") {
		t.Errorf("출력에 'completed'가 없습니다: %s", out)
	}
}

// TestRunKnowledgeFolderSync_InvalidID는 잘못된 ID 검증을 확인합니다.
func TestRunKnowledgeFolderSync_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runKnowledgeFolderSync(client, &buf, "bad/id", false)
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	}
}

// TestRunKnowledgeFolderFiles는 폴더 파일 목록 조회를 검증합니다.
func TestRunKnowledgeFolderFiles(t *testing.T) {
	files := []KnowledgeFolderFile{
		{ID: "file-1111-2222", Name: "document.pdf", Status: "indexed", Size: 102400},
		{ID: "file-3333-4444", Name: "report.docx", Status: "pending", Size: 51200},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/knowledge/folders/folder-1111-2222/files" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"success": true,
			"data":    files,
			"meta":    map[string]interface{}{"page": 1, "page_size": 20, "total": 2, "total_pages": 1},
		}
		b, _ := json.Marshal(resp)
		w.Write(b)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runKnowledgeFolderFiles(client, &buf, "folder-1111-2222", "", "", 0, 0, false)
	if err != nil {
		t.Fatalf("runKnowledgeFolderFiles 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "document.pdf") {
		t.Errorf("출력에 'document.pdf'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "report.docx") {
		t.Errorf("출력에 'report.docx'가 없습니다: %s", out)
	}
}

// TestRunKnowledgeFolderFiles_InvalidID는 잘못된 ID 검증을 확인합니다.
func TestRunKnowledgeFolderFiles_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runKnowledgeFolderFiles(client, &buf, "bad/id", "", "", 0, 0, false)
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	}
}

// TestRunKnowledgeFolderBrowse는 디렉토리 탐색을 검증합니다.
func TestRunKnowledgeFolderBrowse(t *testing.T) {
	entries := []FolderBrowseEntry{
		{Name: "docs", Type: "directory"},
		{Name: "report.pdf", Type: "file", Size: 102400},
	}

	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/knowledge/folders/browse" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		// entries가 {entries: []} 래퍼로 반환됨
		data := map[string]interface{}{"entries": entries}
		w.Write(buildAPIResponse(data))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runKnowledgeFolderBrowse(client, &buf, "/data", false)
	if err != nil {
		t.Fatalf("runKnowledgeFolderBrowse 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "docs") {
		t.Errorf("출력에 'docs'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "report.pdf") {
		t.Errorf("출력에 'report.pdf'가 없습니다: %s", out)
	}

	if capturedBody["path"] != "/data" {
		t.Errorf("요청 본문 path = %v, want '/data'", capturedBody["path"])
	}
}

// TestRunKnowledgeFolderBrowseJSON은 디렉토리 탐색 JSON 출력을 검증합니다.
func TestRunKnowledgeFolderBrowseJSON(t *testing.T) {
	entries := []FolderBrowseEntry{
		{Name: "docs", Type: "directory"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		data := map[string]interface{}{"entries": entries}
		w.Write(buildAPIResponse(data))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runKnowledgeFolderBrowse(client, &buf, "/data", true)
	if err != nil {
		t.Fatalf("runKnowledgeFolderBrowse JSON 오류: %v", err)
	}

	var parsed []FolderBrowseEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].Name != "docs" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

// TestRunKnowledgeFolderDelete는 폴더 삭제를 검증합니다.
func TestRunKnowledgeFolderDelete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/knowledge/folders/folder-1111-2222" || r.Method != http.MethodDelete {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(map[string]bool{"deleted": true}))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runKnowledgeFolderDelete(client, &buf, "folder-1111-2222")
	if err != nil {
		t.Fatalf("runKnowledgeFolderDelete 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "folder-1111-2222") {
		t.Errorf("출력에 폴더 ID가 없습니다: %s", out)
	}
}

// TestRunKnowledgeFolderDelete_InvalidID는 잘못된 ID 검증을 확인합니다.
func TestRunKnowledgeFolderDelete_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runKnowledgeFolderDelete(client, &buf, "bad/id")
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	}
}

// TestRunKnowledgeShowJSON은 지식 항목 상세 JSON 출력을 검증합니다.
func TestRunKnowledgeShowJSON(t *testing.T) {
	entry := KnowledgeEntry{
		ID:    "kn-1111-2222",
		Title: "AI 전략",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(entry))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runKnowledgeShow(client, &buf, "kn-1111-2222", true)
	if err != nil {
		t.Fatalf("runKnowledgeShow JSON 오류: %v", err)
	}

	var parsed KnowledgeEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.ID != "kn-1111-2222" {
		t.Errorf("예상치 않은 ID: %s", parsed.ID)
	}
}

// TestRunKnowledgeFolderShowJSON은 폴더 상세 JSON 출력을 검증합니다.
func TestRunKnowledgeFolderShowJSON(t *testing.T) {
	folder := KnowledgeFolder{
		ID:   "folder-1111-2222",
		Path: "/data/docs",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(folder))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runKnowledgeFolderShow(client, &buf, "folder-1111-2222", true)
	if err != nil {
		t.Fatalf("runKnowledgeFolderShow JSON 오류: %v", err)
	}

	var parsed KnowledgeFolder
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.ID != "folder-1111-2222" {
		t.Errorf("예상치 않은 ID: %s", parsed.ID)
	}
}

// TestRunKnowledgeFolderFilesJSON은 폴더 파일 목록 JSON 출력을 검증합니다.
func TestRunKnowledgeFolderFilesJSON(t *testing.T) {
	files := []KnowledgeFolderFile{
		{ID: "file-1111-2222", Name: "document.pdf"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"success": true,
			"data":    files,
			"meta":    map[string]interface{}{"page": 1, "page_size": 20, "total": 1, "total_pages": 1},
		}
		b, _ := json.Marshal(resp)
		w.Write(b)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runKnowledgeFolderFiles(client, &buf, "folder-1111-2222", "", "", 0, 0, true)
	if err != nil {
		t.Fatalf("runKnowledgeFolderFiles JSON 오류: %v", err)
	}

	var parsed []KnowledgeFolderFile
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].ID != "file-1111-2222" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

// TestRunKnowledgeFolderSyncJSON은 폴더 동기화 JSON 출력을 검증합니다.
func TestRunKnowledgeFolderSyncJSON(t *testing.T) {
	result := FolderSyncResult{
		FolderID: "folder-1111-2222",
		Status:   "completed",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(result))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runKnowledgeFolderSync(client, &buf, "folder-1111-2222", true)
	if err != nil {
		t.Fatalf("runKnowledgeFolderSync JSON 오류: %v", err)
	}

	var parsed FolderSyncResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.Status != "completed" {
		t.Errorf("예상치 않은 Status: %s", parsed.Status)
	}
}

// TestRunKnowledgeCreateJSON은 지식 항목 생성 JSON 출력을 검증합니다.
func TestRunKnowledgeCreateJSON(t *testing.T) {
	entry := KnowledgeEntry{
		ID:    "kn-new-1234",
		Title: "새 전략 문서",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(entry))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runKnowledgeCreate(client, &buf, "새 전략 문서", "내용", "", "", nil, 0, true)
	if err != nil {
		t.Fatalf("runKnowledgeCreate JSON 오류: %v", err)
	}

	var parsed KnowledgeEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.ID != "kn-new-1234" {
		t.Errorf("예상치 않은 ID: %s", parsed.ID)
	}
}

// TestRunKnowledgeFolderCreateJSON은 폴더 생성 JSON 출력을 검증합니다.
func TestRunKnowledgeFolderCreateJSON(t *testing.T) {
	folder := KnowledgeFolder{
		ID:   "folder-new-1234",
		Path: "/data/new-docs",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(folder))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runKnowledgeFolderCreate(client, &buf, "/data/new-docs", true)
	if err != nil {
		t.Fatalf("runKnowledgeFolderCreate JSON 오류: %v", err)
	}

	var parsed KnowledgeFolder
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.ID != "folder-new-1234" {
		t.Errorf("예상치 않은 ID: %s", parsed.ID)
	}
}

// TestRunKnowledgeUpdateWithMultipleFields는 여러 필드를 동시에 수정하는 경우를 검증합니다.
func TestRunKnowledgeUpdateWithMultipleFields(t *testing.T) {
	entry := KnowledgeEntry{
		ID:       "kn-1111-2222",
		Title:    "수정된 제목",
		Content:  "수정된 내용",
		Category: "market",
		Tags:     []string{"updated", "test"},
	}

	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(entry))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runKnowledgeUpdate(client, &buf, "kn-1111-2222", "수정된 제목", "수정된 내용", "market", []string{"updated", "test"}, false)
	if err != nil {
		t.Fatalf("runKnowledgeUpdate 여러 필드 오류: %v", err)
	}

	if capturedBody["title"] != "수정된 제목" {
		t.Errorf("요청 본문 title = %v, want '수정된 제목'", capturedBody["title"])
	}
	if capturedBody["content"] != "수정된 내용" {
		t.Errorf("요청 본문 content = %v, want '수정된 내용'", capturedBody["content"])
	}
	if capturedBody["category"] != "market" {
		t.Errorf("요청 본문 category = %v, want 'market'", capturedBody["category"])
	}
}

// TestRunKnowledgeUpdateJSON은 수정 JSON 출력을 검증합니다.
func TestRunKnowledgeUpdateJSON(t *testing.T) {
	entry := KnowledgeEntry{
		ID:    "kn-1111-2222",
		Title: "수정된 제목",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(entry))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runKnowledgeUpdate(client, &buf, "kn-1111-2222", "수정된 제목", "", "", nil, true)
	if err != nil {
		t.Fatalf("runKnowledgeUpdate JSON 오류: %v", err)
	}

	var parsed KnowledgeEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.ID != "kn-1111-2222" {
		t.Errorf("예상치 않은 ID: %s", parsed.ID)
	}
}

// TestRunKnowledgeFolderFilesWithFilters는 파일 목록 필터 파라미터 전달을 검증합니다.
func TestRunKnowledgeFolderFilesWithFilters(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"success": true,
			"data":    []KnowledgeFolderFile{},
			"meta":    map[string]interface{}{"page": 1, "page_size": 20, "total": 0, "total_pages": 0},
		}
		b, _ := json.Marshal(resp)
		w.Write(b)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runKnowledgeFolderFiles(client, &buf, "folder-1111-2222", "indexed", "report", 2, 10, false)
	if err != nil {
		t.Fatalf("runKnowledgeFolderFiles with filters 오류: %v", err)
	}

	if !strings.Contains(capturedQuery, "status=indexed") {
		t.Errorf("쿼리에 status=indexed가 없습니다: %s", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "search=report") {
		t.Errorf("쿼리에 search=report가 없습니다: %s", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "page=2") {
		t.Errorf("쿼리에 page=2가 없습니다: %s", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "per_page=10") {
		t.Errorf("쿼리에 per_page=10이 없습니다: %s", capturedQuery)
	}
}
