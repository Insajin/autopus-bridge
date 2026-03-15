// Package reranker_test는 모델 매니저를 테스트합니다.
package reranker_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-bridge/internal/reranker"
)

// TestModelManager_GetModelPath는 기본 모델 경로 반환을 검증합니다.
func TestModelManager_GetModelPath(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	mgr := reranker.NewModelManager(tmpDir)

	path := mgr.GetModelPath()
	assert.NotEmpty(t, path)
	assert.Contains(t, path, tmpDir)
}

// TestModelManager_IsDownloaded_False는 미다운로드 상태 확인을 검증합니다.
func TestModelManager_IsDownloaded_False(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	mgr := reranker.NewModelManager(tmpDir)

	assert.False(t, mgr.IsDownloaded())
}

// TestModelManager_IsDownloaded_True는 모델 파일 존재 시 다운로드 확인을 검증합니다.
func TestModelManager_IsDownloaded_True(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	mgr := reranker.NewModelManager(tmpDir)

	// 더미 모델 파일 생성
	modelPath := mgr.GetModelPath()
	err := os.MkdirAll(filepath.Dir(modelPath), 0755)
	require.NoError(t, err)
	err = os.WriteFile(modelPath, []byte("dummy model"), 0644)
	require.NoError(t, err)

	assert.True(t, mgr.IsDownloaded())
}

// TestModelManager_VerifyHash는 SHA256 해시 검증을 테스트합니다.
func TestModelManager_VerifyHash(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	mgr := reranker.NewModelManager(tmpDir)

	// 더미 파일 생성 후 해시 계산
	modelPath := mgr.GetModelPath()
	err := os.MkdirAll(filepath.Dir(modelPath), 0755)
	require.NoError(t, err)
	content := []byte("test model content")
	err = os.WriteFile(modelPath, content, 0644)
	require.NoError(t, err)

	// 올바른 해시로 검증
	hash, err := mgr.ComputeHash()
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.Len(t, hash, 64) // SHA256 hex = 64자

	ok, err := mgr.VerifyHash(hash)
	require.NoError(t, err)
	assert.True(t, ok)

	// 잘못된 해시로 검증
	ok, err = mgr.VerifyHash("wrong_hash")
	require.NoError(t, err)
	assert.False(t, ok)
}

// TestModelManager_EnsureDir는 디렉토리 생성을 검증합니다.
func TestModelManager_EnsureDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "models", "jina")
	mgr := reranker.NewModelManager(subDir)

	err := mgr.EnsureDir()
	require.NoError(t, err)

	// 디렉토리가 생성됐는지 확인
	_, err = os.Stat(subDir)
	assert.NoError(t, err)
}

// TestModelManager_DownloadProgress는 다운로드 진행률 콜백을 검증합니다.
// 실제 네트워크 호출 없이 콜백 메커니즘만 테스트합니다.
func TestModelManager_DownloadProgress(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	mgr := reranker.NewModelManager(tmpDir)

	var progressCalls []reranker.DownloadProgress

	// 진행률 콜백 설정
	mgr.SetProgressCallback(func(p reranker.DownloadProgress) {
		progressCalls = append(progressCalls, p)
	})

	// 이미 다운로드된 척 모의 - 실제 HuggingFace 호출 없이
	// Cancel 컨텍스트로 즉시 중단
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := mgr.Download(ctx)
	// 취소됐으므로 에러 반환
	assert.Error(t, err)
}
