//go:build !windows

package mcp

import (
	"os"
	"syscall"
)

// setSysProcAttr는 프로세스 그룹을 설정합니다 (Unix).
// 자식 프로세스도 함께 종료되도록 Setpgid를 활성화합니다.
func setSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

// sendTermSignal은 프로세스에 SIGTERM을 전송합니다 (Unix).
func sendTermSignal(process *os.Process) error {
	return process.Signal(syscall.SIGTERM)
}

// checkProcessAlive는 프로세스가 실행 중인지 확인합니다 (Unix).
// Signal(0)은 실제 시그널을 보내지 않고 프로세스 존재만 확인합니다.
func checkProcessAlive(process *os.Process) bool {
	err := process.Signal(syscall.Signal(0))
	return err == nil
}
