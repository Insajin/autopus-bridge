// upload.go는 멀티파트 파일 업로드 기능을 제공합니다.
package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// SecondDuration은 테스트에서 시간 단위로 사용하는 상수입니다.
// apiclient.NewContextWithTimeout(10 * apiclient.SecondDuration) 형태로 사용합니다.
const SecondDuration = time.Second

// 허용되는 업로드 파일 확장자 목록입니다.
var allowedUploadExtensions = map[string]bool{
	".pdf":  true,
	".doc":  true,
	".docx": true,
	".txt":  true,
	".md":   true,
}

// maxUploadSize는 업로드 허용 최대 파일 크기 (10MB)입니다.
const maxUploadSize = 10 * 1024 * 1024

// DoUpload는 multipart/form-data 방식으로 파일을 업로드합니다.
// 업로드 전에 파일 크기(최대 10MB)와 확장자(.pdf, .doc, .docx, .txt, .md)를 검사합니다.
// extraFields에 추가 form 필드를 지정할 수 있습니다.
func DoUpload(c *Client, ctx context.Context, path string, filePath string, extraFields map[string]string) (*json.RawMessage, error) {
	// 파일 존재 및 크기 검증
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("파일을 열 수 없습니다: %w", err)
	}
	if info.Size() > maxUploadSize {
		return nil, fmt.Errorf("파일 크기가 10MB를 초과합니다: %d 바이트", info.Size())
	}

	// 확장자 검증
	ext := filepath.Ext(filePath)
	if !allowedUploadExtensions[ext] {
		return nil, fmt.Errorf("허용되지 않는 파일 형식입니다: %s (허용: .pdf, .doc, .docx, .txt, .md)", ext)
	}

	// multipart 본문 구성
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	// 파일 파트 추가
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("파일 열기 실패: %w", err)
	}
	defer file.Close()

	fw, err := mw.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, fmt.Errorf("multipart 파일 파트 생성 실패: %w", err)
	}
	if _, err := io.Copy(fw, file); err != nil {
		return nil, fmt.Errorf("파일 복사 실패: %w", err)
	}

	// 추가 필드 파트 추가
	for key, val := range extraFields {
		if err := mw.WriteField(key, val); err != nil {
			return nil, fmt.Errorf("추가 필드 %q 쓰기 실패: %w", key, err)
		}
	}

	if err := mw.Close(); err != nil {
		return nil, fmt.Errorf("multipart writer 닫기 실패: %w", err)
	}

	// HTTP 요청 생성
	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return nil, fmt.Errorf("HTTP 요청 생성 실패: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	// 인증 토큰 설정
	token, err := c.tokenRefresher.GetToken()
	if err != nil {
		return nil, fmt.Errorf("인증 토큰 획득 실패: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	// 요청 실행
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("업로드 요청 실패: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("응답 읽기 실패: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("업로드 API 오류 (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	// 응답 파싱
	var apiResp APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("응답 파싱 실패: %w", err)
	}

	return &apiResp.Data, nil
}
