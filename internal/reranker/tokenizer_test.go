// Package reranker_test는 WordPiece 토크나이저를 테스트합니다.
package reranker_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-bridge/internal/reranker"
)

// TestTokenizer_BasicTokenization은 기본 토큰화를 검증합니다.
func TestTokenizer_BasicTokenization(t *testing.T) {
	t.Parallel()

	tok := reranker.NewWordPieceTokenizer(512)

	tokens, err := tok.Tokenize("hello world")
	require.NoError(t, err)

	// [CLS] hello world [SEP] 구조 확인
	assert.GreaterOrEqual(t, len(tokens.InputIDs), 3)
}

// TestTokenizer_QueryDocumentPair는 쿼리-문서 쌍 토큰화를 검증합니다.
// Jina v2 형식: [CLS] query [SEP] document [SEP]
func TestTokenizer_QueryDocumentPair(t *testing.T) {
	t.Parallel()

	tok := reranker.NewWordPieceTokenizer(512)

	tokens, err := tok.TokenizePair("Go 언어", "Go는 컴파일 언어입니다")
	require.NoError(t, err)

	// 세 특수 토큰 포함: [CLS], [SEP], [SEP]
	assert.GreaterOrEqual(t, len(tokens.InputIDs), 5)
	// 어텐션 마스크 길이 일치
	assert.Equal(t, len(tokens.InputIDs), len(tokens.AttentionMask))
	// 토큰 타입 ID 길이 일치
	assert.Equal(t, len(tokens.InputIDs), len(tokens.TokenTypeIDs))
}

// TestTokenizer_MaxLengthTruncation은 최대 길이 잘라내기를 검증합니다.
func TestTokenizer_MaxLengthTruncation(t *testing.T) {
	t.Parallel()

	maxLen := 16
	tok := reranker.NewWordPieceTokenizer(maxLen)

	// 긴 텍스트
	longText := "this is a very long document that should be truncated because it exceeds the maximum token length limit set for the tokenizer"
	tokens, err := tok.Tokenize(longText)
	require.NoError(t, err)

	assert.LessOrEqual(t, len(tokens.InputIDs), maxLen)
}

// TestTokenizer_EmptyInput은 빈 입력 처리를 검증합니다.
func TestTokenizer_EmptyInput(t *testing.T) {
	t.Parallel()

	tok := reranker.NewWordPieceTokenizer(512)

	tokens, err := tok.Tokenize("")
	require.NoError(t, err)

	// [CLS] [SEP] 최소 구조
	assert.GreaterOrEqual(t, len(tokens.InputIDs), 2)
}

// TestTokenizer_KoreanText는 한국어 텍스트 처리를 검증합니다.
func TestTokenizer_KoreanText(t *testing.T) {
	t.Parallel()

	tok := reranker.NewWordPieceTokenizer(512)

	tokens, err := tok.Tokenize("안녕하세요 세계")
	require.NoError(t, err)

	// 한국어도 토큰화 가능해야 함
	assert.GreaterOrEqual(t, len(tokens.InputIDs), 2)
}

// TestTokenizer_TokenTypeIDs는 세그먼트 구분을 검증합니다.
// 쿼리 부분: 0, 문서 부분: 1
func TestTokenizer_TokenTypeIDs(t *testing.T) {
	t.Parallel()

	tok := reranker.NewWordPieceTokenizer(512)

	tokens, err := tok.TokenizePair("query", "document")
	require.NoError(t, err)

	// 토큰 타입 ID는 0 또는 1만 포함
	for _, id := range tokens.TokenTypeIDs {
		assert.True(t, id == 0 || id == 1, "token type ID must be 0 or 1, got %d", id)
	}
}
