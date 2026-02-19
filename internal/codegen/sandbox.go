package codegen

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// Sandbox는 코드 생성을 위한 격리된 디렉토리를 제공합니다.
// 각 생성 요청은 독립적인 샌드박스 디렉토리에서 수행됩니다.
type Sandbox struct {
	baseDir string // 샌드박스 기본 디렉토리 (~/.acos/codegen-sandbox/)
	logger  *slog.Logger
}

// 유효한 출력 진입점 파일 목록
var validEntryPoints = []string{
	"package.json",
	"tsconfig.json",
	"index.ts",
	"index.js",
	"main.ts",
	"main.js",
	"src/index.ts",
	"src/index.js",
}

// NewSandbox는 새로운 Sandbox를 생성합니다.
// baseDir가 존재하지 않으면 생성을 시도합니다.
func NewSandbox(baseDir string, logger *slog.Logger) *Sandbox {
	if logger == nil {
		logger = slog.Default()
	}
	return &Sandbox{
		baseDir: baseDir,
		logger:  logger,
	}
}

// Create는 서비스 이름 기반의 격리된 디렉토리를 생성합니다.
//
// 반환값:
//   - string: 생성된 디렉토리 경로
//   - func(): 정리(cleanup) 함수 - 디렉토리를 재귀적으로 삭제
//   - error: 생성 실패 시 에러
func (s *Sandbox) Create(serviceName string) (string, func(), error) {
	// 기본 디렉토리 존재 확인 및 생성
	if err := os.MkdirAll(s.baseDir, 0750); err != nil {
		return "", nil, fmt.Errorf("샌드박스 기본 디렉토리 생성 실패: %w", err)
	}

	// 타임스탬프 기반 고유 디렉토리명 생성
	timestamp := time.Now().Format("20060102-150405")
	dirName := fmt.Sprintf("%s-%s", serviceName, timestamp)
	dirPath := filepath.Join(s.baseDir, dirName)

	if err := os.MkdirAll(dirPath, 0750); err != nil {
		return "", nil, fmt.Errorf("샌드박스 디렉토리 생성 실패: %w", err)
	}

	s.logger.Info("샌드박스 생성",
		slog.String("service", serviceName),
		slog.String("path", dirPath),
	)

	// cleanup 함수: 디렉토리를 재귀적으로 삭제
	cleanup := func() {
		if removeErr := os.RemoveAll(dirPath); removeErr != nil {
			s.logger.Warn("샌드박스 정리 실패",
				slog.String("path", dirPath),
				slog.String("error", removeErr.Error()),
			)
		} else {
			s.logger.Info("샌드박스 정리 완료",
				slog.String("path", dirPath),
			)
		}
	}

	return dirPath, cleanup, nil
}

// ValidateOutput는 출력 디렉토리의 유효성을 검증합니다.
//
// 검증 조건:
//   - 디렉토리가 존재하는지
//   - 비어있지 않은지
//   - package.json 또는 유사한 진입점 파일이 존재하는지
func (s *Sandbox) ValidateOutput(dir string) error {
	// 디렉토리 존재 확인
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("출력 디렉토리가 존재하지 않습니다: %s", dir)
		}
		return fmt.Errorf("출력 디렉토리 접근 실패: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("경로가 디렉토리가 아닙니다: %s", dir)
	}

	// 비어있지 않은지 확인
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("디렉토리 읽기 실패: %w", err)
	}
	if len(entries) == 0 {
		return fmt.Errorf("출력 디렉토리가 비어있습니다: %s", dir)
	}

	// 진입점 파일 존재 확인
	for _, ep := range validEntryPoints {
		epPath := filepath.Join(dir, ep)
		if _, statErr := os.Stat(epPath); statErr == nil {
			return nil
		}
	}

	return fmt.Errorf("유효한 진입점 파일이 없습니다 (package.json, index.ts 등): %s", dir)
}
