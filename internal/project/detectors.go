// Detector 인터페이스와 각 기술 스택 감지기 구현체입니다.
// SPEC-SKILL-V2-001 Block A: Smart Skill Discovery
package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// DetectionResult는 단일 Detector의 감지 결과를 담습니다.
type DetectionResult struct {
	Languages      []string
	Frameworks     []string
	Databases      []string
	BuildTools     []string
	TestFrameworks []string
	DetectedFiles  []string
}

// Detector는 프로젝트 파일을 분석하여 감지된 기술을 반환합니다.
type Detector interface {
	Detect(rootDir string, fileNames []string) *DetectionResult
}

// defaultDetectors는 내장 기술 디텍터 세트를 반환합니다.
func defaultDetectors() []Detector {
	return []Detector{
		&GoDetector{},
		&NodeDetector{},
		&PythonDetector{},
		&RustDetector{},
		&JavaDetector{},
		&RubyDetector{},
		&ElixirDetector{},
		&DartDetector{},
		&DockerDetector{},
	}
}

// --- GoDetector ---

// GoDetector는 Go 프로젝트의 기술 스택을 감지합니다.
type GoDetector struct{}

// Detect는 go.mod 파일을 기반으로 Go 프로젝트를 분석합니다.
func (d *GoDetector) Detect(rootDir string, fileNames []string) *DetectionResult {
	if !containsFile(fileNames, "go.mod") {
		return nil
	}
	result := &DetectionResult{
		Languages:     []string{"go"},
		DetectedFiles: []string{"go.mod"},
	}

	// go.mod 내용을 읽어 프레임워크/DB 감지
	content, err := os.ReadFile(filepath.Join(rootDir, "go.mod"))
	if err == nil {
		modContent := string(content)
		if strings.Contains(modContent, "gofiber/fiber") {
			result.Frameworks = append(result.Frameworks, "fiber")
		}
		if strings.Contains(modContent, "gin-gonic/gin") {
			result.Frameworks = append(result.Frameworks, "gin")
		}
		if strings.Contains(modContent, "go-chi/chi") {
			result.Frameworks = append(result.Frameworks, "chi")
		}
		if strings.Contains(modContent, "gorilla/mux") {
			result.Frameworks = append(result.Frameworks, "gorilla-mux")
		}
		if strings.Contains(modContent, "labstack/echo") {
			result.Frameworks = append(result.Frameworks, "echo")
		}
		if strings.Contains(modContent, "jackc/pgx") || strings.Contains(modContent, "lib/pq") {
			result.Databases = append(result.Databases, "postgresql")
		}
		if strings.Contains(modContent, "go-redis/redis") || strings.Contains(modContent, "redis/go-redis") {
			result.Databases = append(result.Databases, "redis")
		}
		if strings.Contains(modContent, "go-sql-driver/mysql") {
			result.Databases = append(result.Databases, "mysql")
		}
		if strings.Contains(modContent, "gorm.io/gorm") {
			result.Frameworks = append(result.Frameworks, "gorm")
		}
	}

	// 빌드 도구 감지
	if containsFile(fileNames, "Makefile") {
		result.BuildTools = append(result.BuildTools, "make")
		result.DetectedFiles = append(result.DetectedFiles, "Makefile")
	}
	if containsFile(fileNames, ".goreleaser.yml") || containsFile(fileNames, ".goreleaser.yaml") {
		result.BuildTools = append(result.BuildTools, "goreleaser")
	}

	// Go는 내장 테스트 프레임워크 사용
	result.TestFrameworks = append(result.TestFrameworks, "go-test")

	return result
}

// --- NodeDetector ---

// NodeDetector는 Node.js/JavaScript/TypeScript 프로젝트를 감지합니다.
type NodeDetector struct{}

// Detect는 package.json을 기반으로 Node.js 프로젝트를 분석합니다.
func (d *NodeDetector) Detect(rootDir string, fileNames []string) *DetectionResult {
	if !containsFile(fileNames, "package.json") {
		return nil
	}
	result := &DetectionResult{
		DetectedFiles: []string{"package.json"},
	}

	// package.json 읽기
	content, err := os.ReadFile(filepath.Join(rootDir, "package.json"))
	if err != nil {
		result.Languages = []string{"javascript"}
		return result
	}

	var pkg map[string]interface{}
	if err := json.Unmarshal(content, &pkg); err != nil {
		result.Languages = []string{"javascript"}
		return result
	}

	// TypeScript 감지
	if containsFile(fileNames, "tsconfig.json") {
		result.Languages = append(result.Languages, "typescript")
		result.DetectedFiles = append(result.DetectedFiles, "tsconfig.json")
	} else {
		result.Languages = append(result.Languages, "javascript")
	}

	// 모든 의존성 병합하여 프레임워크 감지
	allDeps := mergeDeps(pkg)

	// 프레임워크 감지
	if _, ok := allDeps["next"]; ok {
		result.Frameworks = append(result.Frameworks, "nextjs")
	}
	if _, ok := allDeps["react"]; ok {
		result.Frameworks = append(result.Frameworks, "react")
	}
	if _, ok := allDeps["vue"]; ok {
		result.Frameworks = append(result.Frameworks, "vue")
	}
	if _, ok := allDeps["express"]; ok {
		result.Frameworks = append(result.Frameworks, "express")
	}
	if _, ok := allDeps["@nestjs/core"]; ok {
		result.Frameworks = append(result.Frameworks, "nestjs")
	}
	if _, ok := allDeps["nuxt"]; ok {
		result.Frameworks = append(result.Frameworks, "nuxt")
	}
	if _, ok := allDeps["svelte"]; ok {
		result.Frameworks = append(result.Frameworks, "svelte")
	}
	if _, ok := allDeps["astro"]; ok {
		result.Frameworks = append(result.Frameworks, "astro")
	}

	// 데이터베이스 감지
	if _, ok := allDeps["pg"]; ok {
		result.Databases = append(result.Databases, "postgresql")
	}
	if _, ok := allDeps["prisma"]; ok {
		result.Databases = append(result.Databases, "prisma")
	}
	if _, ok := allDeps["@prisma/client"]; ok {
		result.Databases = append(result.Databases, "prisma")
	}
	if _, ok := allDeps["mongoose"]; ok {
		result.Databases = append(result.Databases, "mongodb")
	}
	if _, ok := allDeps["redis"]; ok {
		result.Databases = append(result.Databases, "redis")
	}
	if _, ok := allDeps["ioredis"]; ok {
		result.Databases = append(result.Databases, "redis")
	}
	if _, ok := allDeps["drizzle-orm"]; ok {
		result.Frameworks = append(result.Frameworks, "drizzle")
	}

	// 테스트 프레임워크 감지
	if _, ok := allDeps["vitest"]; ok {
		result.TestFrameworks = append(result.TestFrameworks, "vitest")
	}
	if _, ok := allDeps["jest"]; ok {
		result.TestFrameworks = append(result.TestFrameworks, "jest")
	}
	if _, ok := allDeps["playwright"]; ok {
		result.TestFrameworks = append(result.TestFrameworks, "playwright")
	}
	if _, ok := allDeps["@playwright/test"]; ok {
		result.TestFrameworks = append(result.TestFrameworks, "playwright")
	}
	if _, ok := allDeps["cypress"]; ok {
		result.TestFrameworks = append(result.TestFrameworks, "cypress")
	}

	// 패키지 매니저/빌드 도구 감지
	if containsFile(fileNames, "bun.lockb") || containsFile(fileNames, "bun.lock") {
		result.BuildTools = append(result.BuildTools, "bun")
	} else if containsFile(fileNames, "pnpm-lock.yaml") {
		result.BuildTools = append(result.BuildTools, "pnpm")
	} else if containsFile(fileNames, "yarn.lock") {
		result.BuildTools = append(result.BuildTools, "yarn")
	} else {
		result.BuildTools = append(result.BuildTools, "npm")
	}

	if _, ok := allDeps["vite"]; ok {
		result.BuildTools = append(result.BuildTools, "vite")
	}
	if _, ok := allDeps["webpack"]; ok {
		result.BuildTools = append(result.BuildTools, "webpack")
	}
	if _, ok := allDeps["turbo"]; ok {
		result.BuildTools = append(result.BuildTools, "turborepo")
	}

	return result
}

// --- PythonDetector ---

// PythonDetector는 Python 프로젝트의 기술 스택을 감지합니다.
type PythonDetector struct{}

// Detect는 pyproject.toml, requirements.txt 등을 기반으로 Python 프로젝트를 분석합니다.
func (d *PythonDetector) Detect(rootDir string, fileNames []string) *DetectionResult {
	hasPyproject := containsFile(fileNames, "pyproject.toml")
	hasRequirements := containsFile(fileNames, "requirements.txt")
	hasPipfile := containsFile(fileNames, "Pipfile")
	hasSetupPy := containsFile(fileNames, "setup.py")

	if !hasPyproject && !hasRequirements && !hasPipfile && !hasSetupPy {
		return nil
	}

	result := &DetectionResult{
		Languages: []string{"python"},
	}
	if hasPyproject {
		result.DetectedFiles = append(result.DetectedFiles, "pyproject.toml")
	}
	if hasRequirements {
		result.DetectedFiles = append(result.DetectedFiles, "requirements.txt")

		content, err := os.ReadFile(filepath.Join(rootDir, "requirements.txt"))
		if err == nil {
			reqContent := strings.ToLower(string(content))
			if strings.Contains(reqContent, "django") {
				result.Frameworks = append(result.Frameworks, "django")
			}
			if strings.Contains(reqContent, "fastapi") {
				result.Frameworks = append(result.Frameworks, "fastapi")
			}
			if strings.Contains(reqContent, "flask") {
				result.Frameworks = append(result.Frameworks, "flask")
			}
			if strings.Contains(reqContent, "pytest") {
				result.TestFrameworks = append(result.TestFrameworks, "pytest")
			}
			if strings.Contains(reqContent, "psycopg") {
				result.Databases = append(result.Databases, "postgresql")
			}
		}
	}

	// 빌드/패키지 도구 감지
	if hasPipfile {
		result.BuildTools = append(result.BuildTools, "pipenv")
	}
	if containsFile(fileNames, "poetry.lock") {
		result.BuildTools = append(result.BuildTools, "poetry")
	}
	if containsFile(fileNames, "uv.lock") {
		result.BuildTools = append(result.BuildTools, "uv")
	}

	return result
}

// --- RustDetector ---

// RustDetector는 Rust 프로젝트의 기술 스택을 감지합니다.
type RustDetector struct{}

// Detect는 Cargo.toml을 기반으로 Rust 프로젝트를 분석합니다.
func (d *RustDetector) Detect(rootDir string, fileNames []string) *DetectionResult {
	if !containsFile(fileNames, "Cargo.toml") {
		return nil
	}
	result := &DetectionResult{
		Languages:     []string{"rust"},
		BuildTools:    []string{"cargo"},
		DetectedFiles: []string{"Cargo.toml"},
	}

	content, err := os.ReadFile(filepath.Join(rootDir, "Cargo.toml"))
	if err == nil {
		cargoContent := string(content)
		if strings.Contains(cargoContent, "actix-web") {
			result.Frameworks = append(result.Frameworks, "actix-web")
		}
		if strings.Contains(cargoContent, "axum") {
			result.Frameworks = append(result.Frameworks, "axum")
		}
		if strings.Contains(cargoContent, "tokio") {
			result.Frameworks = append(result.Frameworks, "tokio")
		}
	}

	return result
}

// --- JavaDetector ---

// JavaDetector는 Java/Kotlin 프로젝트의 기술 스택을 감지합니다.
type JavaDetector struct{}

// Detect는 pom.xml, build.gradle 등을 기반으로 Java 프로젝트를 분석합니다.
func (d *JavaDetector) Detect(rootDir string, fileNames []string) *DetectionResult {
	hasPom := containsFile(fileNames, "pom.xml")
	hasGradle := containsFile(fileNames, "build.gradle") || containsFile(fileNames, "build.gradle.kts")

	if !hasPom && !hasGradle {
		return nil
	}

	result := &DetectionResult{
		Languages: []string{"java"},
	}
	if hasPom {
		result.BuildTools = append(result.BuildTools, "maven")
		result.DetectedFiles = append(result.DetectedFiles, "pom.xml")
	}
	if hasGradle {
		result.BuildTools = append(result.BuildTools, "gradle")
	}

	// Kotlin 감지
	if containsFile(fileNames, "build.gradle.kts") {
		result.Languages = append(result.Languages, "kotlin")
	}

	return result
}

// --- RubyDetector ---

// RubyDetector는 Ruby 프로젝트의 기술 스택을 감지합니다.
type RubyDetector struct{}

// Detect는 Gemfile을 기반으로 Ruby 프로젝트를 분석합니다.
func (d *RubyDetector) Detect(rootDir string, fileNames []string) *DetectionResult {
	if !containsFile(fileNames, "Gemfile") {
		return nil
	}
	result := &DetectionResult{
		Languages:     []string{"ruby"},
		BuildTools:    []string{"bundler"},
		DetectedFiles: []string{"Gemfile"},
	}

	content, err := os.ReadFile(filepath.Join(rootDir, "Gemfile"))
	if err == nil {
		gemContent := string(content)
		if strings.Contains(gemContent, "rails") {
			result.Frameworks = append(result.Frameworks, "rails")
		}
		if strings.Contains(gemContent, "rspec") {
			result.TestFrameworks = append(result.TestFrameworks, "rspec")
		}
	}

	return result
}

// --- ElixirDetector ---

// ElixirDetector는 Elixir 프로젝트의 기술 스택을 감지합니다.
type ElixirDetector struct{}

// Detect는 mix.exs를 기반으로 Elixir 프로젝트를 분석합니다.
func (d *ElixirDetector) Detect(rootDir string, fileNames []string) *DetectionResult {
	if !containsFile(fileNames, "mix.exs") {
		return nil
	}
	result := &DetectionResult{
		Languages:      []string{"elixir"},
		BuildTools:     []string{"mix"},
		TestFrameworks: []string{"exunit"},
		DetectedFiles:  []string{"mix.exs"},
	}

	content, err := os.ReadFile(filepath.Join(rootDir, "mix.exs"))
	if err == nil {
		mixContent := string(content)
		if strings.Contains(mixContent, ":phoenix") {
			result.Frameworks = append(result.Frameworks, "phoenix")
		}
	}

	return result
}

// --- DartDetector ---

// DartDetector는 Dart/Flutter 프로젝트의 기술 스택을 감지합니다.
type DartDetector struct{}

// Detect는 pubspec.yaml을 기반으로 Dart 프로젝트를 분석합니다.
func (d *DartDetector) Detect(rootDir string, fileNames []string) *DetectionResult {
	if !containsFile(fileNames, "pubspec.yaml") {
		return nil
	}
	result := &DetectionResult{
		Languages:     []string{"dart"},
		DetectedFiles: []string{"pubspec.yaml"},
	}

	content, err := os.ReadFile(filepath.Join(rootDir, "pubspec.yaml"))
	if err == nil {
		pubContent := string(content)
		if strings.Contains(pubContent, "flutter") {
			result.Frameworks = append(result.Frameworks, "flutter")
		}
	}

	return result
}

// --- DockerDetector ---

// DockerDetector는 Docker 관련 설정을 감지합니다.
type DockerDetector struct{}

// Detect는 Dockerfile, docker-compose.yml 등을 기반으로 Docker 사용을 분석합니다.
func (d *DockerDetector) Detect(rootDir string, fileNames []string) *DetectionResult {
	hasDockerfile := containsFile(fileNames, "Dockerfile")
	hasCompose := containsFile(fileNames, "docker-compose.yml") ||
		containsFile(fileNames, "docker-compose.yaml") ||
		containsFile(fileNames, "compose.yml") ||
		containsFile(fileNames, "compose.yaml")

	if !hasDockerfile && !hasCompose {
		return nil
	}

	result := &DetectionResult{}
	if hasDockerfile {
		result.BuildTools = append(result.BuildTools, "docker")
		result.DetectedFiles = append(result.DetectedFiles, "Dockerfile")
	}
	if hasCompose {
		result.BuildTools = append(result.BuildTools, "docker-compose")
	}

	return result
}

// --- 헬퍼 함수 ---

// containsFile은 fileNames 목록에 target 파일이 존재하는지 대소문자 무시하여 확인합니다.
func containsFile(fileNames []string, target string) bool {
	lower := strings.ToLower(target)
	for _, f := range fileNames {
		if strings.ToLower(f) == lower || strings.ToLower(filepath.Base(f)) == lower {
			return true
		}
	}
	return false
}

// mergeDeps는 package.json의 dependencies, devDependencies, peerDependencies를 병합합니다.
func mergeDeps(pkg map[string]interface{}) map[string]bool {
	deps := make(map[string]bool)
	for _, key := range []string{"dependencies", "devDependencies", "peerDependencies"} {
		if d, ok := pkg[key]; ok {
			if dMap, ok := d.(map[string]interface{}); ok {
				for name := range dMap {
					deps[name] = true
				}
			}
		}
	}
	return deps
}
