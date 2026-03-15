// Package reranker는 ONNX Runtime 기반 Jina Reranker v2 서비스를 테스트합니다.
// SPEC-RAGEVO-001 REQ-D: Bridge 로컬 리랭커
package reranker_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-bridge/internal/reranker"
)

// TestRerankResult_Sorting은 결과가 관련성 점수 기준 내림차순으로 정렬되는지 검증합니다.
func TestRerankResult_Sorting(t *testing.T) {
	t.Parallel()

	results := []reranker.RerankResult{
		{Index: 0, RelevanceScore: 0.3, Document: "low score"},
		{Index: 1, RelevanceScore: 0.9, Document: "high score"},
		{Index: 2, RelevanceScore: 0.6, Document: "mid score"},
	}

	reranker.SortByScore(results)

	assert.Equal(t, 0.9, results[0].RelevanceScore)
	assert.Equal(t, 0.6, results[1].RelevanceScore)
	assert.Equal(t, 0.3, results[2].RelevanceScore)
}

// TestMockReranker_Rerank는 Mock 리랭커가 올바르게 동작하는지 검증합니다.
func TestMockReranker_Rerank(t *testing.T) {
	t.Parallel()

	svc := reranker.NewMockService()

	req := reranker.RerankRequest{
		Query:     "Go 언어의 특징",
		Documents: []string{"Go는 정적 타입 언어입니다", "Python은 동적 타입 언어입니다", "Go는 컴파일 언어입니다"},
		TopN:      2,
	}

	resp, err := svc.Rerank(context.Background(), req)
	require.NoError(t, err)

	// topN=2이므로 최대 2개 결과
	assert.LessOrEqual(t, len(resp.Results), 2)
	// 결과는 내림차순 정렬
	if len(resp.Results) >= 2 {
		assert.GreaterOrEqual(t, resp.Results[0].RelevanceScore, resp.Results[1].RelevanceScore)
	}
	// 각 결과에 문서 내용 포함
	for _, r := range resp.Results {
		assert.NotEmpty(t, r.Document)
		assert.GreaterOrEqual(t, r.Index, 0)
	}
}

// TestMockReranker_EmptyDocuments는 빈 문서 목록 처리를 검증합니다.
func TestMockReranker_EmptyDocuments(t *testing.T) {
	t.Parallel()

	svc := reranker.NewMockService()

	req := reranker.RerankRequest{
		Query:     "test",
		Documents: []string{},
		TopN:      5,
	}

	resp, err := svc.Rerank(context.Background(), req)
	require.NoError(t, err)
	assert.Empty(t, resp.Results)
}

// TestMockReranker_TopNLargerThanDocuments는 TopN이 문서 수보다 클 때 처리를 검증합니다.
func TestMockReranker_TopNLargerThanDocuments(t *testing.T) {
	t.Parallel()

	svc := reranker.NewMockService()

	req := reranker.RerankRequest{
		Query:     "test",
		Documents: []string{"doc1", "doc2"},
		TopN:      10,
	}

	resp, err := svc.Rerank(context.Background(), req)
	require.NoError(t, err)
	// TopN이 문서 수보다 커도 문서 수만큼만 반환
	assert.LessOrEqual(t, len(resp.Results), 2)
}

// TestMockReranker_TopNZero는 TopN=0일 때 기본값으로 처리되는지 검증합니다.
func TestMockReranker_TopNZero(t *testing.T) {
	t.Parallel()

	svc := reranker.NewMockService()

	req := reranker.RerankRequest{
		Query:     "test",
		Documents: []string{"doc1", "doc2", "doc3"},
		TopN:      0,
	}

	resp, err := svc.Rerank(context.Background(), req)
	require.NoError(t, err)
	// TopN=0이면 전체 문서 반환
	assert.Equal(t, 3, len(resp.Results))
}

// TestMockReranker_ContextCancellation은 컨텍스트 취소 처리를 검증합니다.
func TestMockReranker_ContextCancellation(t *testing.T) {
	t.Parallel()

	svc := reranker.NewMockService()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 즉시 취소

	req := reranker.RerankRequest{
		Query:     "test",
		Documents: []string{"doc1"},
		TopN:      1,
	}

	_, err := svc.Rerank(ctx, req)
	assert.ErrorIs(t, err, context.Canceled)
}

// TestONNXService_NotAvailable는 ONNX 라이브러리 없을 때 우아한 실패를 검증합니다.
func TestONNXService_NotAvailable(t *testing.T) {
	t.Parallel()

	cfg := reranker.Config{
		ModelPath: "/nonexistent/model.onnx",
		UseGPU:    false,
	}

	// ONNX 라이브러리가 없어도 생성은 성공해야 함
	svc, err := reranker.NewONNXService(cfg)
	require.NoError(t, err)

	// 사용 가능 여부는 false여야 함 (라이브러리/모델 없음)
	assert.False(t, svc.IsAvailable())
}

// TestONNXService_Rerank_Fallback은 ONNX 불가 시 에러를 반환하는지 검증합니다.
func TestONNXService_Rerank_Fallback(t *testing.T) {
	t.Parallel()

	cfg := reranker.Config{
		ModelPath: "/nonexistent/model.onnx",
		UseGPU:    false,
	}

	svc, err := reranker.NewONNXService(cfg)
	require.NoError(t, err)

	req := reranker.RerankRequest{
		Query:     "test",
		Documents: []string{"doc1"},
		TopN:      1,
	}

	_, err = svc.Rerank(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unavailable")
}
