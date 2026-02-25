package agentbrowser

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// InstallChecker는 agent-browser CLI의 설치 상태를 확인한다.
type InstallChecker struct {
	logger zerolog.Logger
	// lookPathFn은 실행 파일 경로를 탐색하는 함수이다 (테스트용 주입 가능).
	lookPathFn func(file string) (string, error)
	// commandFn은 외부 명령을 실행하는 함수이다 (테스트용 주입 가능).
	commandFn func(ctx context.Context, name string, args ...string) *exec.Cmd
}

// InstallCheckResult는 설치 상태 확인 결과이다.
type InstallCheckResult struct {
	// Installed는 agent-browser가 설치되어 있는지 여부이다.
	Installed bool
	// Version은 agent-browser의 버전 문자열이다.
	Version string
	// NodeInstalled는 Node.js가 설치되어 있는지 여부이다.
	NodeInstalled bool
	// NodeVersion은 Node.js의 버전 문자열이다.
	NodeVersion string
	// InstallGuide는 미설치 시 설치 안내 메시지이다.
	InstallGuide string
}

// NewInstallChecker는 새로운 InstallChecker를 생성한다.
func NewInstallChecker(logger zerolog.Logger) *InstallChecker {
	return &InstallChecker{
		logger:     logger,
		lookPathFn: exec.LookPath,
		commandFn:  exec.CommandContext,
	}
}

// Check는 agent-browser CLI 및 Node.js의 설치 상태를 확인한다.
func (ic *InstallChecker) Check(ctx context.Context) (*InstallCheckResult, error) {
	result := &InstallCheckResult{}

	// Node.js 설치 확인
	nodeInstalled, nodeVersion := ic.checkNode(ctx)
	result.NodeInstalled = nodeInstalled
	result.NodeVersion = nodeVersion

	// agent-browser 설치 확인
	agentInstalled, agentVersion := ic.checkAgentBrowser(ctx)
	result.Installed = agentInstalled
	result.Version = agentVersion

	// 미설치 시 안내 메시지 설정
	if !result.Installed {
		result.InstallGuide = "npm install -g @anthropic-ai/agent-browser && agent-browser install"
		if !result.NodeInstalled {
			result.InstallGuide = "Node.js를 먼저 설치한 후 " + result.InstallGuide
		}
	}

	ic.logger.Info().
		Bool("agent_browser_installed", result.Installed).
		Str("agent_browser_version", result.Version).
		Bool("node_installed", result.NodeInstalled).
		Str("node_version", result.NodeVersion).
		Msg("agent-browser 설치 상태 확인 완료")

	return result, nil
}

// checkNode는 Node.js 설치 상태와 버전을 확인한다.
func (ic *InstallChecker) checkNode(ctx context.Context) (installed bool, version string) {
	_, err := ic.lookPathFn("node")
	if err != nil {
		return false, ""
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var stdout bytes.Buffer
	cmd := ic.commandFn(timeoutCtx, "node", "--version")
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		ic.logger.Warn().Err(err).Msg("node --version 실행 실패")
		return true, ""
	}

	return true, strings.TrimSpace(stdout.String())
}

// checkAgentBrowser는 agent-browser CLI 설치 상태와 버전을 확인한다.
func (ic *InstallChecker) checkAgentBrowser(ctx context.Context) (installed bool, version string) {
	_, err := ic.lookPathFn("agent-browser")
	if err != nil {
		return false, ""
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var stdout bytes.Buffer
	cmd := ic.commandFn(timeoutCtx, "agent-browser", "--version")
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		ic.logger.Warn().Err(err).Msg("agent-browser --version 실행 실패")
		// LookPath에서 찾았으므로 설치는 되어 있다
		return true, ""
	}

	return true, strings.TrimSpace(stdout.String())
}

// InstallGuideMessage는 설치 안내 메시지를 반환한다.
func InstallGuideMessage() string {
	return fmt.Sprintf(
		"agent-browser가 설치되어 있지 않습니다. 다음 명령으로 설치해 주세요:\n" +
			"  npm install -g @anthropic-ai/agent-browser && agent-browser install",
	)
}
