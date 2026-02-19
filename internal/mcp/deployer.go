package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
)

// DeployFile은 서비스 디렉토리에 기록할 파일 정보입니다.
type DeployFile struct {
	Path    string // 서비스 디렉토리 내 상대 경로
	Content string
}

// Deployer는 생성된 MCP 서버 코드를 디스크에 배포하고 Manager에 등록합니다.
// SPEC-SELF-EXPAND-001: MCP 서버 자율 배포
type Deployer struct {
	baseDir string   // MCP 서버 기본 디렉토리 (~/.acos/mcp-servers/)
	manager *Manager // MCP 서버 등록 및 시작용
}

// NewDeployer는 새로운 Deployer를 생성합니다.
func NewDeployer(baseDir string, manager *Manager) *Deployer {
	return &Deployer{
		baseDir: baseDir,
		manager: manager,
	}
}

// Deploy는 MCP 서버 코드를 디스크에 기록하고 Manager에 등록합니다.
// 1. 서비스 디렉토리 생성
// 2. 파일 기록 (하위 디렉토리 자동 생성)
// 3. 환경 변수 .env 파일 기록
// 4. Manager에 ServerConfig 등록
func (d *Deployer) Deploy(ctx context.Context, serviceName string, files []DeployFile, envVars map[string]string) (string, error) {
	if serviceName == "" {
		return "", fmt.Errorf("서비스 이름이 비어있음")
	}

	serviceDir := filepath.Join(d.baseDir, serviceName)

	// 1. 서비스 디렉토리 생성
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		return "", fmt.Errorf("서비스 디렉토리 생성 실패 %q: %w", serviceDir, err)
	}

	log.Info().
		Str("service", serviceName).
		Str("dir", serviceDir).
		Int("files", len(files)).
		Msg("[mcp-deployer] 배포 시작")

	// 2. 파일 기록
	for _, f := range files {
		filePath := filepath.Join(serviceDir, f.Path)

		// 하위 디렉토리 자동 생성
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("디렉토리 생성 실패 %q: %w", dir, err)
		}

		if err := os.WriteFile(filePath, []byte(f.Content), 0644); err != nil {
			return "", fmt.Errorf("파일 기록 실패 %q: %w", filePath, err)
		}

		log.Debug().
			Str("service", serviceName).
			Str("file", f.Path).
			Msg("[mcp-deployer] 파일 기록 완료")
	}

	// 3. 환경 변수 .env 파일 기록
	if len(envVars) > 0 {
		envPath := filepath.Join(serviceDir, ".env")
		if err := writeEnvFile(envPath, envVars); err != nil {
			return "", fmt.Errorf("환경 변수 파일 기록 실패: %w", err)
		}

		log.Debug().
			Str("service", serviceName).
			Int("vars", len(envVars)).
			Msg("[mcp-deployer] .env 파일 기록 완료")
	}

	// 4. Manager에 ServerConfig 등록 및 시작
	cfg := d.buildServerConfig(serviceName, serviceDir, envVars)

	_, err := d.manager.Start(ctx, serviceName, &cfg)
	if err != nil {
		log.Warn().
			Err(err).
			Str("service", serviceName).
			Msg("[mcp-deployer] 서버 시작 실패 (파일은 배포 완료)")
		// 파일 배포는 성공했으므로 deploy path는 반환
		return serviceDir, fmt.Errorf("서버 시작 실패: %w", err)
	}

	log.Info().
		Str("service", serviceName).
		Str("dir", serviceDir).
		Msg("[mcp-deployer] 배포 및 시작 완료")

	return serviceDir, nil
}

// Undeploy는 MCP 서버를 중지하고 서비스 디렉토리를 삭제합니다.
func (d *Deployer) Undeploy(serviceName string) error {
	if serviceName == "" {
		return fmt.Errorf("서비스 이름이 비어있음")
	}

	// 1. MCP 서버 중지
	if err := d.manager.Stop(serviceName); err != nil {
		log.Warn().
			Err(err).
			Str("service", serviceName).
			Msg("[mcp-deployer] 서버 중지 실패 (디렉토리 삭제 계속)")
	}

	// 2. 서비스 디렉토리 삭제
	serviceDir := filepath.Join(d.baseDir, serviceName)
	if err := os.RemoveAll(serviceDir); err != nil {
		return fmt.Errorf("서비스 디렉토리 삭제 실패 %q: %w", serviceDir, err)
	}

	log.Info().
		Str("service", serviceName).
		Msg("[mcp-deployer] 언디플로이 완료")

	return nil
}

// ListDeployed는 baseDir 하위에 배포된 서비스 이름 목록을 반환합니다.
func (d *Deployer) ListDeployed() ([]string, error) {
	entries, err := os.ReadDir(d.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("배포 디렉토리 읽기 실패: %w", err)
	}

	var services []string
	for _, entry := range entries {
		if entry.IsDir() {
			services = append(services, entry.Name())
		}
	}
	return services, nil
}

// buildServerConfig는 서비스 디렉토리 내용을 기반으로 ServerConfig를 생성합니다.
// package.json이 있으면 "npm start", 아니면 "npx tsx src/index.ts"를 사용합니다.
func (d *Deployer) buildServerConfig(serviceName, serviceDir string, envVars map[string]string) ServerConfig {
	command := "npx"
	args := []string{"tsx", "src/index.ts"}

	// package.json이 존재하면 npm start 사용
	packageJSON := filepath.Join(serviceDir, "package.json")
	if _, err := os.Stat(packageJSON); err == nil {
		command = "npm"
		args = []string{"start"}
	}

	return ServerConfig{
		Name:       serviceName,
		Command:    command,
		Args:       args,
		Env:        envVars,
		WorkingDir: serviceDir,
	}
}

// writeEnvFile은 환경 변수를 KEY=VALUE 형식으로 .env 파일에 기록합니다.
func writeEnvFile(path string, envVars map[string]string) error {
	var lines []string
	for k, v := range envVars {
		lines = append(lines, fmt.Sprintf("%s=%s", k, v))
	}
	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(path, []byte(content), 0644)
}
