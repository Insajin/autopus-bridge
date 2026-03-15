// Package reranker_test는 커버리지 보완 테스트를 포함합니다.
package reranker_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-bridge/internal/reranker"
)

// TestONNXService_ModelManager는 ModelManager 접근자를 검증합니다.
func TestONNXService_ModelManager(t *testing.T) {
	t.Parallel()

	cfg := reranker.Config{
		ModelPath: "/tmp/test_model.onnx",
	}
	svc, err := reranker.NewONNXService(cfg)
	require.NoError(t, err)

	mgr := svc.ModelManager()
	assert.NotNil(t, mgr)
}

// TestONNXService_WithModelPath는 커스텀 ModelPath 설정을 검증합니다.
func TestONNXService_WithModelPath(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfg := reranker.Config{
		ModelPath: filepath.Join(tmpDir, "model.onnx"),
		UseGPU:    false,
	}

	svc, err := reranker.NewONNXService(cfg)
	require.NoError(t, err)
	assert.False(t, svc.IsAvailable())
}

// TestONNXService_WithEmptyModelPath는 빈 ModelPath 기본값 처리를 검증합니다.
func TestONNXService_WithEmptyModelPath(t *testing.T) {
	t.Parallel()

	cfg := reranker.Config{
		Enabled: true,
	}

	svc, err := reranker.NewONNXService(cfg)
	require.NoError(t, err)
	assert.NotNil(t, svc)
	assert.False(t, svc.IsAvailable())
}

// TestONNXService_Rerank_NotAvailable는 unavailable 상태에서 에러 메시지를 검증합니다.
func TestONNXService_Rerank_NotAvailable(t *testing.T) {
	t.Parallel()

	cfg := reranker.Config{ModelPath: "/nonexistent/model.onnx"}
	svc, err := reranker.NewONNXService(cfg)
	require.NoError(t, err)

	_, rerankErr := svc.Rerank(context.Background(), reranker.RerankRequest{
		Query:     "test",
		Documents: []string{"doc"},
		TopN:      1,
	})
	require.Error(t, rerankErr)
	assert.Contains(t, rerankErr.Error(), "unavailable")
}

// TestWordPieceTokenizer_DefaultMaxLength는 maxLength<=0 기본값 처리를 검증합니다.
func TestWordPieceTokenizer_DefaultMaxLength(t *testing.T) {
	t.Parallel()

	// maxLength=0이면 기본값 512 사용
	tok := reranker.NewWordPieceTokenizer(0)
	tokens, err := tok.Tokenize("hello")
	require.NoError(t, err)
	assert.LessOrEqual(t, len(tokens.InputIDs), 512)
}

// TestWordPieceTokenizer_NegativeMaxLength는 음수 maxLength 처리를 검증합니다.
func TestWordPieceTokenizer_NegativeMaxLength(t *testing.T) {
	t.Parallel()

	tok := reranker.NewWordPieceTokenizer(-1)
	tokens, err := tok.Tokenize("hello")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(tokens.InputIDs), 2)
}

// TestTokenizer_SpecialChars는 특수문자 처리를 검증합니다 (UNK 토큰 경로).
func TestTokenizer_SpecialChars(t *testing.T) {
	t.Parallel()

	tok := reranker.NewWordPieceTokenizer(512)

	// 특수문자는 UNK 토큰으로 처리됨
	tokens, err := tok.Tokenize("!@#$%")
	require.NoError(t, err)
	// [CLS] + UNK tokens + [SEP] 구조
	assert.GreaterOrEqual(t, len(tokens.InputIDs), 2)
}

// TestTokenizer_PairTruncation은 쌍 토큰화 최대 길이 잘라내기를 검증합니다.
func TestTokenizer_PairTruncation(t *testing.T) {
	t.Parallel()

	maxLen := 10
	tok := reranker.NewWordPieceTokenizer(maxLen)

	longQuery := "this is a very long query that needs truncation"
	longDoc := "this is also a very long document that needs truncation too"

	tokens, err := tok.TokenizePair(longQuery, longDoc)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(tokens.InputIDs), maxLen)
}

// TestModelManager_ComputeHash_NoFile는 파일 없을 때 해시 계산 에러를 검증합니다.
func TestModelManager_ComputeHash_NoFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	mgr := reranker.NewModelManager(tmpDir)

	_, err := mgr.ComputeHash()
	assert.Error(t, err)
}

// TestModelManager_VerifyHash_NoFile는 파일 없을 때 해시 검증 에러를 검증합니다.
func TestModelManager_VerifyHash_NoFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	mgr := reranker.NewModelManager(tmpDir)

	_, err := mgr.VerifyHash("anything")
	assert.Error(t, err)
}

// TestModelManager_EnsureDir_ReadOnly는 읽기 전용 부모 디렉토리에서 에러를 검증합니다.
func TestModelManager_EnsureDir_ReadOnly(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root 권한에서는 읽기 전용 테스트 스킵")
	}
	t.Parallel()

	// 읽기 전용 디렉토리 생성
	tmpDir := t.TempDir()
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	require.NoError(t, os.Mkdir(readOnlyDir, 0555))

	// 읽기 전용 디렉토리 하위에 쓰기 시도
	mgr := reranker.NewModelManager(filepath.Join(readOnlyDir, "models"))
	err := mgr.EnsureDir()
	assert.Error(t, err)
}

// TestHandler_Rerank_InternalError는 서비스 에러 시 500 반환을 검증합니다.
func TestHandler_Rerank_InternalError(t *testing.T) {
	t.Parallel()

	svc := &errorServiceStub{}
	h := reranker.NewHandler(svc)

	body := []byte(`{"query":"test","documents":["doc1"],"top_n":1}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rerank", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestSortByScore_EmptySlice는 빈 슬라이스 정렬을 검증합니다.
func TestSortByScore_EmptySlice(t *testing.T) {
	t.Parallel()

	var results []reranker.RerankResult
	// 패닉 없이 동작해야 함
	reranker.SortByScore(results)
	assert.Empty(t, results)
}

// TestSortByScore_SingleElement는 단일 요소 정렬을 검증합니다.
func TestSortByScore_SingleElement(t *testing.T) {
	t.Parallel()

	results := []reranker.RerankResult{
		{Index: 0, RelevanceScore: 0.7, Document: "only"},
	}
	reranker.SortByScore(results)
	assert.Equal(t, 0.7, results[0].RelevanceScore)
}

// errorServiceStub은 항상 에러를 반환하는 테스트 스텁입니다.
type errorServiceStub struct{}

func (e *errorServiceStub) IsAvailable() bool { return true }

func (e *errorServiceStub) Rerank(_ context.Context, _ reranker.RerankRequest) (reranker.RerankResponse, error) {
	return reranker.RerankResponse{}, fmt.Errorf("internal error")
}
