// Package project handles multi-project configuration management.
// Analyzer는 프로젝트 루트 디렉토리를 스캔하여 기술 스택을 감지합니다.
// SPEC-SKILL-V2-001 Block A: Smart Skill Discovery
package project

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/insajin/autopus-agent-protocol"
)

// Analyzer는 프로젝트의 기술 스택을 감지하는 분석기입니다.
type Analyzer struct {
	detectors []Detector
}

// NewAnalyzer는 기본 디텍터 세트를 가진 Analyzer를 생성합니다.
func NewAnalyzer() *Analyzer {
	return &Analyzer{
		detectors: defaultDetectors(),
	}
}

// Analyze는 주어진 rootDir을 스캔하고 ProjectContextPayload를 반환합니다.
// 성능을 위해 최상위 파일만 확인하고, 일부 서브디렉토리(src, cmd 등)는 1단계까지 탐색합니다.
func (a *Analyzer) Analyze(rootDir string) (*ws.ProjectContextPayload, error) {
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return nil, err
	}

	// 최상위 파일/디렉토리 이름 수집
	var fileNames []string
	for _, e := range entries {
		fileNames = append(fileNames, e.Name())
	}

	// 일반적인 소스 디렉토리는 1단계까지 탐색하여 패턴 검출 향상
	for _, subdir := range []string{"src", "cmd", "internal", "pkg", "lib", "app"} {
		subPath := filepath.Join(rootDir, subdir)
		if info, err := os.Stat(subPath); err == nil && info.IsDir() {
			subEntries, _ := os.ReadDir(subPath)
			for _, e := range subEntries {
				fileNames = append(fileNames, filepath.Join(subdir, e.Name()))
			}
		}
	}

	// 모든 디텍터 실행 후 결과 병합
	techStack := ws.TechStack{
		DetectedFiles: []string{},
	}

	for _, d := range a.detectors {
		result := d.Detect(rootDir, fileNames)
		if result != nil {
			mergeTechStack(&techStack, result)
		}
	}

	// 중복 제거
	techStack.Languages = dedupStrings(techStack.Languages)
	techStack.Frameworks = dedupStrings(techStack.Frameworks)
	techStack.Databases = dedupStrings(techStack.Databases)
	techStack.BuildTools = dedupStrings(techStack.BuildTools)
	techStack.TestFrameworks = dedupStrings(techStack.TestFrameworks)
	techStack.DetectedFiles = dedupStrings(techStack.DetectedFiles)

	return &ws.ProjectContextPayload{
		ProjectRoot: rootDir,
		TechStack:   techStack,
		DetectedAt:  time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// mergeTechStack은 DetectionResult를 TechStack에 병합합니다.
func mergeTechStack(dst *ws.TechStack, src *DetectionResult) {
	dst.Languages = append(dst.Languages, src.Languages...)
	dst.Frameworks = append(dst.Frameworks, src.Frameworks...)
	dst.Databases = append(dst.Databases, src.Databases...)
	dst.BuildTools = append(dst.BuildTools, src.BuildTools...)
	dst.TestFrameworks = append(dst.TestFrameworks, src.TestFrameworks...)
	dst.DetectedFiles = append(dst.DetectedFiles, src.DetectedFiles...)
}

// dedupStrings는 대소문자를 무시하고 중복 문자열을 제거합니다.
// 첫 번째 등장한 형태를 유지합니다.
func dedupStrings(input []string) []string {
	if len(input) == 0 {
		return input
	}
	seen := make(map[string]bool, len(input))
	result := make([]string, 0, len(input))
	for _, s := range input {
		lower := strings.ToLower(s)
		if !seen[lower] {
			seen[lower] = true
			result = append(result, s)
		}
	}
	return result
}
