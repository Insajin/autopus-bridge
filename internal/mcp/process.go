package mcp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
)

// StopGracePeriod는 SIGTERM 후 SIGKILL까지의 유예 시간입니다.
const StopGracePeriod = 5 * time.Second

// ProcessInfo는 실행 중인 MCP 서버 프로세스 정보를 담고 있습니다.
type ProcessInfo struct {
	Name      string
	PID       int
	Port      int
	Command   string
	StartedAt time.Time
	cmd       *exec.Cmd
	cancel    context.CancelFunc
	mu        sync.Mutex
}

// IsRunning은 프로세스가 실행 중인지 확인합니다.
func (p *ProcessInfo) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.isRunning()
}

// isRunning은 뮤텍스 없이 프로세스 실행 상태를 확인합니다 (내부 호출용).
func (p *ProcessInfo) isRunning() bool {
	if p.cmd == nil || p.cmd.Process == nil {
		return false
	}
	err := p.cmd.Process.Signal(syscall.Signal(0))
	return err == nil
}

// Stop은 MCP 서버 프로세스를 안전하게 종료합니다.
func (p *ProcessInfo) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cmd == nil || p.cmd.Process == nil {
		return nil
	}

	log.Info().
		Str("name", p.Name).
		Int("pid", p.PID).
		Msg("[mcp] 프로세스 종료 시작 (SIGTERM)")

	// SIGTERM 전송
	if err := p.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		if err.Error() != "os: process already finished" {
			log.Warn().Err(err).Str("name", p.Name).Msg("[mcp] SIGTERM 전송 실패")
		}
		p.cleanup()
		return nil
	}

	// 유예 시간 후에도 실행 중이면 SIGKILL (데드락 방지: isRunning 사용)
	done := make(chan struct{})
	go func() {
		for {
			if !p.isRunning() {
				close(done)
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	select {
	case <-done:
		log.Info().Str("name", p.Name).Msg("[mcp] 프로세스 정상 종료")
	case <-time.After(StopGracePeriod):
		log.Warn().Str("name", p.Name).Msg("[mcp] SIGKILL 전송")
		_ = p.cmd.Process.Kill()
	}

	p.cleanup()
	return nil
}

// cleanup은 프로세스 관련 리소스를 정리합니다.
func (p *ProcessInfo) cleanup() {
	if p.cancel != nil {
		p.cancel()
	}
}

// startProcess는 MCP 서버 프로세스를 시작합니다.
func startProcess(ctx context.Context, cfg ServerConfig) (*ProcessInfo, error) {
	// 바이너리 존재 확인
	if cfg.RequiredBinary != "" {
		if _, err := exec.LookPath(cfg.RequiredBinary); err != nil {
			return nil, fmt.Errorf("필수 바이너리 %q를 찾을 수 없음: %w", cfg.RequiredBinary, err)
		}
	}

	cmdPath, err := exec.LookPath(cfg.Command)
	if err != nil {
		return nil, fmt.Errorf("명령어 %q를 찾을 수 없음: %w", cfg.Command, err)
	}

	procCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(procCtx, cmdPath, cfg.Args...)

	// 환경 변수 설정
	cmd.Env = os.Environ()
	for k, v := range cfg.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	if cfg.WorkingDir != "" {
		cmd.Dir = cfg.WorkingDir
	}

	// 프로세스 그룹 설정 (자식 프로세스도 함께 종료)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// stdout/stderr를 로그로 전달
	cmd.Stdout = &logWriter{name: cfg.Name, level: "info"}
	cmd.Stderr = &logWriter{name: cfg.Name, level: "error"}

	log.Info().
		Str("name", cfg.Name).
		Str("command", cfg.Command).
		Strs("args", cfg.Args).
		Msg("[mcp] 프로세스 시작")

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("MCP 서버 %q 시작 실패: %w", cfg.Name, err)
	}

	info := &ProcessInfo{
		Name:      cfg.Name,
		PID:       cmd.Process.Pid,
		Command:   cfg.Command,
		StartedAt: time.Now(),
		cmd:       cmd,
		cancel:    cancel,
	}

	// 프로세스 종료 감시 (비동기)
	go func() {
		if waitErr := cmd.Wait(); waitErr != nil {
			log.Warn().
				Str("name", cfg.Name).
				Int("pid", info.PID).
				Err(waitErr).
				Msg("[mcp] 프로세스 종료됨")
		}
	}()

	return info, nil
}

// logWriter는 MCP 서버의 stdout/stderr를 zerolog로 전달합니다.
type logWriter struct {
	name  string
	level string
}

func (w *logWriter) Write(p []byte) (int, error) {
	msg := string(p)
	switch w.level {
	case "error":
		log.Error().Str("mcp", w.name).Msg(msg)
	default:
		log.Debug().Str("mcp", w.name).Msg(msg)
	}
	return len(p), nil
}
