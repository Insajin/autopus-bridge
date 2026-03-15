// Package reranker_test는 HTTP 핸들러를 테스트합니다.
package reranker_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-bridge/internal/reranker"
)

// TestHandler_Rerank_Success는 정상 리랭킹 요청을 검증합니다.
func TestHandler_Rerank_Success(t *testing.T) {
	t.Parallel()

	svc := reranker.NewMockService()
	h := reranker.NewHandler(svc)

	body := reranker.RerankRequest{
		Query:     "Go 언어",
		Documents: []string{"Go는 컴파일 언어", "Python은 인터프리터 언어", "Go는 구글이 만든 언어"},
		TopN:      2,
	}
	reqBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/rerank", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp reranker.RerankResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.LessOrEqual(t, len(resp.Results), 2)
	for _, r := range resp.Results {
		assert.NotEmpty(t, r.Document)
	}
}

// TestHandler_Rerank_InvalidJSON은 잘못된 JSON 요청 처리를 검증합니다.
func TestHandler_Rerank_InvalidJSON(t *testing.T) {
	t.Parallel()

	svc := reranker.NewMockService()
	h := reranker.NewHandler(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/rerank", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestHandler_Rerank_EmptyQuery는 빈 쿼리 처리를 검증합니다.
func TestHandler_Rerank_EmptyQuery(t *testing.T) {
	t.Parallel()

	svc := reranker.NewMockService()
	h := reranker.NewHandler(svc)

	body := reranker.RerankRequest{
		Query:     "",
		Documents: []string{"doc1"},
		TopN:      1,
	}
	reqBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/rerank", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestHandler_Rerank_MethodNotAllowed는 GET 요청 거부를 검증합니다.
func TestHandler_Rerank_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	svc := reranker.NewMockService()
	h := reranker.NewHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rerank", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

// TestHandler_Rerank_ServiceUnavailable는 서비스 불가 시 503 반환을 검증합니다.
func TestHandler_Rerank_ServiceUnavailable(t *testing.T) {
	t.Parallel()

	svc := &unavailableServiceStub{}
	h := reranker.NewHandler(svc)

	body := reranker.RerankRequest{
		Query:     "test",
		Documents: []string{"doc1"},
		TopN:      1,
	}
	reqBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/rerank", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// unavailableServiceStub은 항상 unavailable을 반환하는 테스트 스텁입니다.
type unavailableServiceStub struct{}

func (u *unavailableServiceStub) IsAvailable() bool { return false }

func (u *unavailableServiceStub) Rerank(_ context.Context, _ reranker.RerankRequest) (reranker.RerankResponse, error) {
	return reranker.RerankResponse{}, nil
}
