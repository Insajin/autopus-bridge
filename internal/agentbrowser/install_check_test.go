package agentbrowser

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestNewInstallChecker(t *testing.T) {
	logger := zerolog.Nop()
	ic := NewInstallChecker(logger)
	if ic == nil {
		t.Fatal("NewInstallChecker() returned nil")
	}
	if ic.lookPathFn == nil {
		t.Error("lookPathFn is nil; want non-nil")
	}
	if ic.commandFn == nil {
		t.Error("commandFn is nil; want non-nil")
	}
}

func TestInstallChecker_Check_AllInstalled(t *testing.T) {
	logger := zerolog.Nop()
	ic := NewInstallChecker(logger)

	// LookPath를 모킹하여 모두 설치된 것으로 반환
	ic.lookPathFn = func(file string) (string, error) {
		switch file {
		case "node":
			return "/usr/local/bin/node", nil
		case "agent-browser":
			return "/usr/local/bin/agent-browser", nil
		}
		return "", fmt.Errorf("not found: %s", file)
	}

	// 버전 명령을 모킹
	ic.commandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "node" && len(args) > 0 && args[0] == "--version" {
			return exec.CommandContext(ctx, "echo", "v20.11.0")
		}
		if name == "agent-browser" && len(args) > 0 && args[0] == "--version" {
			return exec.CommandContext(ctx, "echo", "1.2.3")
		}
		return exec.CommandContext(ctx, "false")
	}

	result, err := ic.Check(context.Background())
	if err != nil {
		t.Fatalf("Check() returned error: %v", err)
	}

	if !result.Installed {
		t.Error("Installed = false; want true")
	}
	if !result.NodeInstalled {
		t.Error("NodeInstalled = false; want true")
	}
	if result.Version != "1.2.3" {
		t.Errorf("Version = %q; want %q", result.Version, "1.2.3")
	}
	if result.NodeVersion != "v20.11.0" {
		t.Errorf("NodeVersion = %q; want %q", result.NodeVersion, "v20.11.0")
	}
	if result.InstallGuide != "" {
		t.Errorf("InstallGuide = %q; want empty", result.InstallGuide)
	}
}

func TestInstallChecker_Check_NothingInstalled(t *testing.T) {
	logger := zerolog.Nop()
	ic := NewInstallChecker(logger)

	// 모두 미설치로 모킹
	ic.lookPathFn = func(file string) (string, error) {
		return "", fmt.Errorf("not found: %s", file)
	}

	result, err := ic.Check(context.Background())
	if err != nil {
		t.Fatalf("Check() returned error: %v", err)
	}

	if result.Installed {
		t.Error("Installed = true; want false")
	}
	if result.NodeInstalled {
		t.Error("NodeInstalled = true; want false")
	}
	if result.InstallGuide == "" {
		t.Error("InstallGuide is empty; want non-empty for uninstalled state")
	}
	if !strings.Contains(result.InstallGuide, "Node.js") {
		t.Errorf("InstallGuide = %q; want containing 'Node.js'", result.InstallGuide)
	}
}

func TestInstallChecker_Check_NodeInstalledButNotAgentBrowser(t *testing.T) {
	logger := zerolog.Nop()
	ic := NewInstallChecker(logger)

	ic.lookPathFn = func(file string) (string, error) {
		if file == "node" {
			return "/usr/local/bin/node", nil
		}
		return "", fmt.Errorf("not found: %s", file)
	}

	ic.commandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "node" && len(args) > 0 && args[0] == "--version" {
			return exec.CommandContext(ctx, "echo", "v22.0.0")
		}
		return exec.CommandContext(ctx, "false")
	}

	result, err := ic.Check(context.Background())
	if err != nil {
		t.Fatalf("Check() returned error: %v", err)
	}

	if result.Installed {
		t.Error("Installed = true; want false (agent-browser not installed)")
	}
	if !result.NodeInstalled {
		t.Error("NodeInstalled = false; want true")
	}
	if result.InstallGuide == "" {
		t.Error("InstallGuide is empty; want non-empty")
	}
	// Node가 설치되어 있으면 Node.js 관련 안내는 없어야 한다
	if strings.Contains(result.InstallGuide, "Node.js") {
		t.Errorf("InstallGuide = %q; want not containing 'Node.js' when Node is installed", result.InstallGuide)
	}
}

func TestInstallChecker_Check_VersionCommandFails(t *testing.T) {
	logger := zerolog.Nop()
	ic := NewInstallChecker(logger)

	ic.lookPathFn = func(file string) (string, error) {
		return "/usr/local/bin/" + file, nil
	}

	// 버전 명령이 실패하는 경우
	ic.commandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "false")
	}

	result, err := ic.Check(context.Background())
	if err != nil {
		t.Fatalf("Check() returned error: %v", err)
	}

	// LookPath에서 찾았으므로 설치는 되어 있다고 판단
	if !result.Installed {
		t.Error("Installed = false; want true (found via LookPath)")
	}
	if !result.NodeInstalled {
		t.Error("NodeInstalled = false; want true (found via LookPath)")
	}
	// 버전은 비어 있을 수 있다
	if result.Version != "" {
		t.Errorf("Version = %q; want empty (version command failed)", result.Version)
	}
}

func TestInstallGuideMessage(t *testing.T) {
	msg := InstallGuideMessage()
	if msg == "" {
		t.Error("InstallGuideMessage() returned empty string")
	}
	if !strings.Contains(msg, "npm install") {
		t.Errorf("InstallGuideMessage() = %q; want containing 'npm install'", msg)
	}
}
