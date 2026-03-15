package reranker

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
)

// errUnavailable은 ONNX Runtime 또는 모델을 사용할 수 없을 때 반환되는 에러입니다.
var errUnavailable = errors.New("reranker unavailable: ONNX Runtime or model not loaded")

// ONNXService는 ONNX Runtime을 사용하는 Jina Reranker v2 서비스입니다.
// ONNX Runtime 공유 라이브러리(.dylib/.so/.dll)가 필요하며,
// 없는 경우 IsAvailable()=false를 반환하고 Rerank() 호출 시 에러를 반환합니다.
//
// @MX:NOTE: github.com/yalue/onnxruntime_go 바인딩을 사용합니다.
// 런타임에 onnxruntime 공유 라이브러리가 필요합니다 (~50MB).
// @MX:SPEC: SPEC-RAGEVO-001 REQ-D
type ONNXService struct {
	cfg       Config
	modelMgr  *ModelManager
	tokenizer *WordPieceTokenizer

	mu        sync.RWMutex
	available bool
	// session은 실제 ONNX Runtime 세션 (인터페이스로 추상화)
	session onnxSession
}

// onnxSession은 ONNX Runtime 세션 추상화 인터페이스입니다.
// 테스트 시 mock으로 교체 가능합니다.
//
// @MX:NOTE: 실제 구현은 github.com/yalue/onnxruntime_go를 사용합니다.
// 현재는 빌드 태그 없이 동작하도록 인터페이스만 정의합니다.
type onnxSession interface {
	// RunInference는 입력 텐서로 추론을 실행하고 로짓을 반환합니다.
	RunInference(inputIDs, attentionMask, tokenTypeIDs []int32) ([]float32, error)
	// Close는 세션을 닫습니다.
	Close() error
}

// NewONNXService는 ONNXService를 생성합니다.
// ONNX Runtime 라이브러리나 모델이 없어도 생성은 성공합니다.
// IsAvailable()로 사용 가능 여부를 확인하세요.
func NewONNXService(cfg Config) (*ONNXService, error) {
	baseDir := cfg.ModelPath
	if baseDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("홈 디렉토리 확인 실패: %w", err)
		}
		baseDir = filepath.Join(home, ".autopus-bridge", "models")
	} else {
		// ModelPath가 파일 경로인 경우 디렉토리 추출
		baseDir = filepath.Dir(cfg.ModelPath)
	}

	svc := &ONNXService{
		cfg:       cfg,
		modelMgr:  NewModelManager(baseDir),
		tokenizer: NewWordPieceTokenizer(512),
	}

	// 모델 존재 시 초기화 시도
	if svc.modelMgr.IsDownloaded() {
		if err := svc.initSession(); err != nil {
			// 초기화 실패는 에러로 처리하지 않음 — IsAvailable()=false로 반환
			svc.available = false
		}
	}

	return svc, nil
}

// initSession은 ONNX Runtime 세션을 초기화합니다.
// ONNX Runtime 공유 라이브러리가 없으면 실패합니다.
func (s *ONNXService) initSession() error {
	// @MX:NOTE: 실제 구현에서는 onnxruntime_go를 통해 세션 생성
	// 현재는 라이브러리 없이 빌드 가능하도록 unavailable 상태 유지
	// 빌드 태그 'onnx'로 실제 구현 활성화 예정
	s.mu.Lock()
	defer s.mu.Unlock()
	s.available = false
	return fmt.Errorf("ONNX Runtime 공유 라이브러리가 없습니다 (build tag: onnx)")
}

// IsAvailable은 ONNX Runtime과 모델이 모두 사용 가능한지 반환합니다.
func (s *ONNXService) IsAvailable() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.available
}

// Rerank는 Jina Reranker v2 ONNX 모델로 문서를 리랭킹합니다.
// IsAvailable()이 false면 errUnavailable을 반환합니다.
func (s *ONNXService) Rerank(ctx context.Context, req RerankRequest) (RerankResponse, error) {
	if !s.IsAvailable() {
		return RerankResponse{}, errUnavailable
	}

	if err := ctx.Err(); err != nil {
		return RerankResponse{}, err
	}

	results := make([]RerankResult, 0, len(req.Documents))
	for i, doc := range req.Documents {
		if err := ctx.Err(); err != nil {
			return RerankResponse{}, err
		}

		score, err := s.score(req.Query, doc)
		if err != nil {
			return RerankResponse{}, fmt.Errorf("문서 %d 점수 계산 실패: %w", i, err)
		}

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

// score는 쿼리-문서 쌍의 관련성 점수를 계산합니다.
func (s *ONNXService) score(query, doc string) (float64, error) {
	s.mu.RLock()
	sess := s.session
	s.mu.RUnlock()

	if sess == nil {
		return 0, errUnavailable
	}

	tokens, err := s.tokenizer.TokenizePair(query, doc)
	if err != nil {
		return 0, fmt.Errorf("토큰화 실패: %w", err)
	}

	logits, err := sess.RunInference(tokens.InputIDs, tokens.AttentionMask, tokens.TokenTypeIDs)
	if err != nil {
		return 0, fmt.Errorf("ONNX 추론 실패: %w", err)
	}

	if len(logits) == 0 {
		return 0, fmt.Errorf("추론 결과가 없습니다")
	}

	// 시그모이드로 0~1 범위 점수 변환
	return sigmoid(float64(logits[0])), nil
}

// sigmoid는 시그모이드 함수를 적용합니다.
func sigmoid(x float64) float64 {
	return 1.0 / (1.0 + math.Exp(-x))
}

// ModelManager는 서비스의 모델 매니저를 반환합니다.
// 외부에서 모델 다운로드 상태 확인 및 다운로드에 사용합니다.
func (s *ONNXService) ModelManager() *ModelManager {
	return s.modelMgr
}
