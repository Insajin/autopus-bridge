package codegen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Collector는 출력 디렉토리에서 생성된 파일을 수집합니다.
// 파일 크기 및 개수 제한을 통해 안전하게 파일을 읽습니다.
type Collector struct {
	maxFiles     int   // 최대 수집 파일 수 (기본값: 20)
	maxFileSize  int64 // 단일 파일 최대 크기 (바이트, 기본값: 50KB = 51200)
	maxTotalSize int64 // 총 최대 크기 (바이트, 기본값: 500KB = 512000)
}

// 기본 제한값
const (
	defaultMaxFiles     = 20
	defaultMaxFileSize  = 51200  // 50KB
	defaultMaxTotalSize = 512000 // 500KB

	// binaryCheckSize는 바이너리 파일 감지를 위해 확인하는 바이트 수입니다.
	binaryCheckSize = 512
)

// 건너뛸 디렉토리 목록
var skipDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	".next":        true,
	"dist":         true,
	"__pycache__":  true,
	".cache":       true,
}

// NewCollector는 기본 설정으로 새로운 Collector를 생성합니다.
func NewCollector() *Collector {
	return &Collector{
		maxFiles:     defaultMaxFiles,
		maxFileSize:  defaultMaxFileSize,
		maxTotalSize: defaultMaxTotalSize,
	}
}

// WithMaxFiles는 최대 수집 파일 수를 설정합니다. (체이너블)
func (c *Collector) WithMaxFiles(n int) *Collector {
	c.maxFiles = n
	return c
}

// WithMaxFileSize는 단일 파일 최대 크기를 설정합니다. (체이너블)
func (c *Collector) WithMaxFileSize(n int64) *Collector {
	c.maxFileSize = n
	return c
}

// WithMaxTotalSize는 총 최대 크기를 설정합니다. (체이너블)
func (c *Collector) WithMaxTotalSize(n int64) *Collector {
	c.maxTotalSize = n
	return c
}

// Collect는 지정된 디렉토리에서 생성된 파일을 재귀적으로 수집합니다.
//
// 수집 규칙:
//   - node_modules/, .git/ 등 불필요한 디렉토리 건너뛰기
//   - 바이너리 파일 감지 및 건너뛰기 (첫 512바이트에 null 바이트 존재 여부)
//   - 파일 수/크기 제한 초과 시 에러 반환
func (c *Collector) Collect(dir string) ([]GeneratedFile, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("디렉토리 접근 실패: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("경로가 디렉토리가 아닙니다: %s", dir)
	}

	var files []GeneratedFile
	var totalSize int64

	walkErr := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 디렉토리 처리
		if info.IsDir() {
			// 루트 디렉토리는 건너뛰지 않음
			if path == dir {
				return nil
			}
			// 건너뛸 디렉토리 확인
			if skipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		// 파일 수 제한 확인
		if len(files) >= c.maxFiles {
			return fmt.Errorf("최대 파일 수(%d) 초과", c.maxFiles)
		}

		// 단일 파일 크기 제한 확인
		if info.Size() > c.maxFileSize {
			// 너무 큰 파일은 건너뛰기 (에러가 아닌 무시)
			return nil
		}

		// 파일 읽기
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			// 읽기 실패한 파일은 건너뛰기
			return nil
		}

		// 바이너리 파일 감지
		if isBinary(content) {
			return nil
		}

		// 총 크기 제한 확인
		fileSize := int64(len(content))
		if totalSize+fileSize > c.maxTotalSize {
			return fmt.Errorf("총 파일 크기(%d바이트) 초과: 제한 %d바이트", totalSize+fileSize, c.maxTotalSize)
		}

		// 상대 경로 계산
		relPath, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			relPath = info.Name()
		}

		files = append(files, GeneratedFile{
			Path:      relPath,
			Content:   string(content),
			SizeBytes: fileSize,
		})
		totalSize += fileSize

		return nil
	})

	if walkErr != nil {
		return files, walkErr
	}

	return files, nil
}

// isBinary는 파일 내용이 바이너리인지 판별합니다.
// 첫 512바이트 내에 null 바이트(0x00)가 있으면 바이너리로 판단합니다.
func isBinary(content []byte) bool {
	checkLen := len(content)
	if checkLen > binaryCheckSize {
		checkLen = binaryCheckSize
	}

	return strings.ContainsRune(string(content[:checkLen]), 0)
}
