package reranker

import (
	"strings"
	"unicode"
)

// TokenizerOutput은 토큰화 결과입니다.
type TokenizerOutput struct {
	// InputIDs는 토큰 ID 목록입니다.
	InputIDs []int32
	// AttentionMask는 어텐션 마스크입니다 (1=유효 토큰, 0=패딩).
	AttentionMask []int32
	// TokenTypeIDs는 세그먼트 ID입니다 (0=쿼리/첫 번째, 1=문서/두 번째).
	TokenTypeIDs []int32
}

// 특수 토큰 ID (BERT 기반 WordPiece 표준)
const (
	tokenCLS = 101 // [CLS]
	tokenSEP = 102 // [SEP]
	tokenUNK = 100 // [UNK]
)

// WordPieceTokenizer는 Jina Reranker v2와 호환되는 간소화된 WordPiece 토크나이저입니다.
// 실제 어휘 사전 없이 문자 단위로 동작하여 다국어를 지원합니다.
// @MX:NOTE: 실제 배포에서는 jina-reranker-v2 어휘 사전을 로드해야 합니다.
// @MX:SPEC: SPEC-RAGEVO-001 REQ-D
type WordPieceTokenizer struct {
	maxLength int
}

// NewWordPieceTokenizer는 WordPieceTokenizer를 생성합니다.
func NewWordPieceTokenizer(maxLength int) *WordPieceTokenizer {
	if maxLength <= 0 {
		maxLength = 512
	}
	return &WordPieceTokenizer{maxLength: maxLength}
}

// Tokenize는 단일 텍스트를 토큰화합니다.
// 결과 형식: [CLS] tokens [SEP]
func (t *WordPieceTokenizer) Tokenize(text string) (TokenizerOutput, error) {
	tokens := t.splitTokens(text)

	// 최대 길이 계산: [CLS] + tokens + [SEP]
	maxTokens := t.maxLength - 2
	if maxTokens < 0 {
		maxTokens = 0
	}
	if len(tokens) > maxTokens {
		tokens = tokens[:maxTokens]
	}

	// InputIDs 구성: [CLS] tokens [SEP]
	ids := make([]int32, 0, len(tokens)+2)
	ids = append(ids, tokenCLS)
	ids = append(ids, tokens...)
	ids = append(ids, tokenSEP)

	seqLen := len(ids)
	mask := make([]int32, seqLen)
	typeIDs := make([]int32, seqLen)
	for i := range mask {
		mask[i] = 1
		typeIDs[i] = 0
	}

	return TokenizerOutput{
		InputIDs:      ids,
		AttentionMask: mask,
		TokenTypeIDs:  typeIDs,
	}, nil
}

// TokenizePair는 쿼리-문서 쌍을 토큰화합니다.
// 결과 형식: [CLS] query_tokens [SEP] doc_tokens [SEP]
// 세그먼트: 쿼리=0, 문서=1
func (t *WordPieceTokenizer) TokenizePair(query, document string) (TokenizerOutput, error) {
	queryTokens := t.splitTokens(query)
	docTokens := t.splitTokens(document)

	// 최대 길이 배분: [CLS] + query + [SEP] + doc + [SEP]
	// 쿼리와 문서에 균등 배분 (단, 쿼리 우선)
	available := t.maxLength - 3 // CLS + SEP + SEP
	if available < 0 {
		available = 0
	}

	if len(queryTokens)+len(docTokens) > available {
		// 쿼리 최대 절반, 나머지 문서
		queryMax := available / 2
		docMax := available - queryMax
		if len(queryTokens) > queryMax {
			queryTokens = queryTokens[:queryMax]
		}
		if len(docTokens) > docMax {
			docTokens = docTokens[:docMax]
		}
	}

	totalLen := 1 + len(queryTokens) + 1 + len(docTokens) + 1
	ids := make([]int32, 0, totalLen)
	mask := make([]int32, 0, totalLen)
	typeIDs := make([]int32, 0, totalLen)

	// [CLS] 쿼리 [SEP] — 세그먼트 0
	ids = append(ids, tokenCLS)
	ids = append(ids, queryTokens...)
	ids = append(ids, tokenSEP)
	for range 1 + len(queryTokens) + 1 {
		mask = append(mask, 1)
		typeIDs = append(typeIDs, 0)
	}

	// 문서 [SEP] — 세그먼트 1
	ids = append(ids, docTokens...)
	ids = append(ids, tokenSEP)
	for range len(docTokens) + 1 {
		mask = append(mask, 1)
		typeIDs = append(typeIDs, 1)
	}

	return TokenizerOutput{
		InputIDs:      ids,
		AttentionMask: mask,
		TokenTypeIDs:  typeIDs,
	}, nil
}

// splitTokens는 텍스트를 문자 단위 토큰 ID로 분리합니다.
// 실제 WordPiece 어휘 사전 없이 유니코드 코드포인트 기반으로 동작합니다.
func (t *WordPieceTokenizer) splitTokens(text string) []int32 {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	var tokens []int32
	words := strings.Fields(text)
	for _, word := range words {
		for _, r := range word {
			id := runeToTokenID(r)
			tokens = append(tokens, id)
		}
	}
	return tokens
}

// runeToTokenID는 단일 룬을 토큰 ID로 변환합니다.
// 실제 WordPiece 어휘 없이 간소화된 매핑을 사용합니다.
func runeToTokenID(r rune) int32 {
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		// 103 이후를 문자/숫자 토큰으로 매핑 (충돌 방지)
		id := int32(r%30000) + 103
		return id
	}
	// 특수문자, 공백 등은 UNK
	return tokenUNK
}
