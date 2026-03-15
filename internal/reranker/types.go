// Package reranker는 ONNX Runtime 기반 Jina Reranker v2 서비스를 제공합니다.
// SPEC-RAGEVO-001 REQ-D: Bridge 로컬 리랭커 (ONNX Runtime + Jina Reranker v2)
package reranker

import (
	"context"
	"sort"
)

// Config는 리랭커 서비스 설정입니다.
type Config struct {
	// Enabled는 리랭커 활성화 여부입니다.
	Enabled bool `yaml:"enabled" mapstructure:"enabled"`
	// ModelPath는 ONNX 모델 파일 경로입니다. 비어있으면 기본 경로를 사용합니다.
	ModelPath string `yaml:"model_path" mapstructure:"model_path"`
	// UseGPU는 GPU(CUDA/CoreML) 사용 여부입니다.
	UseGPU bool `yaml:"use_gpu" mapstructure:"use_gpu"`
}

// RerankRequest는 리랭킹 요청입니다.
type RerankRequest struct {
	// Query는 검색 쿼리입니다.
	Query string `json:"query"`
	// Documents는 리랭킹할 문서 목록입니다.
	Documents []string `json:"documents"`
	// TopN은 반환할 상위 결과 수입니다. 0이면 전체 반환.
	TopN int `json:"top_n"`
}

// RerankResult는 단일 리랭킹 결과입니다.
type RerankResult struct {
	// Index는 원본 문서 목록에서의 인덱스입니다.
	Index int `json:"index"`
	// RelevanceScore는 쿼리-문서 관련성 점수입니다 (0.0 ~ 1.0).
	RelevanceScore float64 `json:"relevance_score"`
	// Document는 문서 내용입니다.
	Document string `json:"document"`
}

// RerankResponse는 리랭킹 응답입니다.
type RerankResponse struct {
	// Results는 관련성 점수 내림차순으로 정렬된 결과 목록입니다.
	Results []RerankResult `json:"results"`
}

// Service는 리랭킹 서비스 인터페이스입니다.
// @MX:ANCHOR: 리랭커 서비스 계약 — 서버 RerankProvider와 Bridge 구현 모두 이 인터페이스 참조
// @MX:REASON: fan_in >= 3 (ONNXService, MockService, HTTP handler)
// @MX:SPEC: SPEC-RAGEVO-001 REQ-D
type Service interface {
	// Rerank는 쿼리와 문서 목록을 받아 관련성 점수로 정렬된 결과를 반환합니다.
	Rerank(ctx context.Context, req RerankRequest) (RerankResponse, error)
	// IsAvailable은 서비스 사용 가능 여부를 반환합니다.
	IsAvailable() bool
}

// SortByScore는 결과를 관련성 점수 내림차순으로 정렬합니다.
func SortByScore(results []RerankResult) {
	sort.Slice(results, func(i, j int) bool {
		return results[i].RelevanceScore > results[j].RelevanceScore
	})
}

// applyTopN은 TopN 제한을 적용합니다. TopN=0이면 전체 반환.
func applyTopN(results []RerankResult, topN int) []RerankResult {
	if topN <= 0 || topN >= len(results) {
		return results
	}
	return results[:topN]
}
