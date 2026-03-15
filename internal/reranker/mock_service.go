package reranker

import (
	"context"
)

// MockService는 테스트 및 ONNX 라이브러리 미사용 환경을 위한 모의 리랭커입니다.
// 키워드 매칭 기반으로 간단한 관련성 점수를 계산합니다.
type MockService struct{}

// NewMockService는 MockService를 생성합니다.
func NewMockService() Service {
	return &MockService{}
}

// IsAvailable은 항상 true를 반환합니다.
func (m *MockService) IsAvailable() bool {
	return true
}

// Rerank는 키워드 매칭 기반으로 문서를 리랭킹합니다.
func (m *MockService) Rerank(ctx context.Context, req RerankRequest) (RerankResponse, error) {
	if err := ctx.Err(); err != nil {
		return RerankResponse{}, err
	}

	results := make([]RerankResult, 0, len(req.Documents))
	for i, doc := range req.Documents {
		score := computeMockScore(req.Query, doc)
		results = append(results, RerankResult{
			Index:          i,
			RelevanceScore: score,
			Document:       doc,
		})
	}

	SortByScore(results)
	results = applyTopN(results, req.TopN)

	return RerankResponse{Results: results}, nil
}

// computeMockScore는 쿼리와 문서 간 간단한 키워드 매칭 점수를 계산합니다.
// 실제 ONNX 추론 대신 테스트 목적으로 사용됩니다.
func computeMockScore(query, doc string) float64 {
	if len(query) == 0 || len(doc) == 0 {
		return 0.0
	}

	// 쿼리 룬 집합 구성
	queryRunes := []rune(query)
	docRunes := []rune(doc)

	if len(queryRunes) == 0 {
		return 0.0
	}

	// 쿼리 문자가 문서에 포함된 비율로 점수 계산
	matched := 0
	for _, qr := range queryRunes {
		for _, dr := range docRunes {
			if qr == dr {
				matched++
				break
			}
		}
	}

	return float64(matched) / float64(len(queryRunes))
}
