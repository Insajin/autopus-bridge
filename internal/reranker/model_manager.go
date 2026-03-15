package reranker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const (
	// jinaModelFilename은 다운로드할 ONNX 모델 파일명입니다.
	jinaModelFilename = "jina-reranker-v2-base-multilingual.onnx"
	// jinaModelURL은 HuggingFace에서 모델을 다운로드하는 URL 템플릿입니다.
	// @MX:NOTE: 실제 URL은 HuggingFace API를 통해 확인 필요
	jinaModelURL = "https://huggingface.co/jinaai/jina-reranker-v2-base-multilingual/resolve/main/onnx/model.onnx"
)

// DownloadProgress는 모델 다운로드 진행률 정보입니다.
type DownloadProgress struct {
	// BytesDownloaded는 다운로드된 바이트 수입니다.
	BytesDownloaded int64
	// TotalBytes는 전체 파일 크기입니다. -1이면 알 수 없음.
	TotalBytes int64
	// Percentage는 진행률 (0.0 ~ 100.0)입니다.
	Percentage float64
}

// ProgressCallback은 다운로드 진행률 콜백 타입입니다.
type ProgressCallback func(DownloadProgress)

// ModelManager는 ONNX 모델 다운로드 및 버전 관리를 담당합니다.
// @MX:NOTE: 최초 실행 시 HuggingFace에서 ~280MB 모델을 다운로드합니다
// @MX:SPEC: SPEC-RAGEVO-001 REQ-D
type ModelManager struct {
	baseDir          string
	progressCallback ProgressCallback
}

// NewModelManager는 ModelManager를 생성합니다.
// baseDir은 모델을 저장할 기본 디렉토리입니다 (예: ~/.autopus-bridge/models).
func NewModelManager(baseDir string) *ModelManager {
	return &ModelManager{
		baseDir: baseDir,
	}
}

// GetModelPath는 모델 파일의 전체 경로를 반환합니다.
func (m *ModelManager) GetModelPath() string {
	return filepath.Join(m.baseDir, jinaModelFilename)
}

// IsDownloaded는 모델 파일이 존재하는지 확인합니다.
func (m *ModelManager) IsDownloaded() bool {
	_, err := os.Stat(m.GetModelPath())
	return err == nil
}

// EnsureDir는 모델 저장 디렉토리를 생성합니다.
func (m *ModelManager) EnsureDir() error {
	if err := os.MkdirAll(m.baseDir, 0755); err != nil {
		return fmt.Errorf("모델 디렉토리 생성 실패: %w", err)
	}
	return nil
}

// SetProgressCallback은 다운로드 진행률 콜백을 설정합니다.
func (m *ModelManager) SetProgressCallback(cb ProgressCallback) {
	m.progressCallback = cb
}

// ComputeHash는 모델 파일의 SHA256 해시를 계산합니다.
func (m *ModelManager) ComputeHash() (string, error) {
	f, err := os.Open(m.GetModelPath())
	if err != nil {
		return "", fmt.Errorf("모델 파일 열기 실패: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("해시 계산 실패: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerifyHash는 모델 파일의 SHA256 해시를 검증합니다.
func (m *ModelManager) VerifyHash(expectedHash string) (bool, error) {
	actual, err := m.ComputeHash()
	if err != nil {
		return false, err
	}
	return actual == expectedHash, nil
}

// Download는 HuggingFace에서 Jina Reranker v2 ONNX 모델을 다운로드합니다.
// 컨텍스트 취소 시 즉시 중단합니다.
// @MX:WARN: 네트워크 요청 — 타임아웃 없이 호출하면 무기한 블로킹 가능
// @MX:REASON: HTTP 클라이언트에 context 타임아웃 필수
func (m *ModelManager) Download(ctx context.Context) error {
	if err := m.EnsureDir(); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jinaModelURL, nil)
	if err != nil {
		return fmt.Errorf("HTTP 요청 생성 실패: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("모델 다운로드 실패: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("모델 다운로드 실패: HTTP %d", resp.StatusCode)
	}

	tmpPath := m.GetModelPath() + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("임시 파일 생성 실패: %w", err)
	}
	defer func() {
		f.Close()
		// 실패 시 임시 파일 정리
		if _, err := os.Stat(tmpPath); err == nil {
			os.Remove(tmpPath)
		}
	}()

	totalBytes := resp.ContentLength
	var downloaded int64

	buf := make([]byte, 32*1024)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := f.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("파일 쓰기 실패: %w", writeErr)
			}
			downloaded += int64(n)

			if m.progressCallback != nil {
				pct := 0.0
				if totalBytes > 0 {
					pct = float64(downloaded) / float64(totalBytes) * 100
				}
				m.progressCallback(DownloadProgress{
					BytesDownloaded: downloaded,
					TotalBytes:      totalBytes,
					Percentage:      pct,
				})
			}
		}

		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("다운로드 읽기 오류: %w", readErr)
		}
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("파일 닫기 실패: %w", err)
	}

	// 원자적 이동
	if err := os.Rename(tmpPath, m.GetModelPath()); err != nil {
		return fmt.Errorf("모델 파일 이동 실패: %w", err)
	}

	return nil
}
