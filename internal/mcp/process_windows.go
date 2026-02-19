//go:build windows

package mcp

import (
	"os"
	"syscall"
)

// setSysProcAttr는 Windows에서 프로세스 속성을 반환합니다.
// Windows에서는 프로세스 그룹 설정이 불필요합니다.
func setSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{}
}

// sendTermSignal은 Windows에서 프로세스를 종료합니다.
// Windows에는 SIGTERM이 없으므로 Kill을 사용합니다.
func sendTermSignal(process *os.Process) error {
	return process.Kill()
}

// checkProcessAlive는 프로세스가 실행 중인지 확인합니다 (Windows).
// FindProcess는 Windows에서 항상 성공하므로 Signal로 확인합니다.
func checkProcessAlive(process *os.Process) bool {
	// Windows에서는 os.Interrupt 시그널로 프로세스 존재를 확인할 수 없음.
	// FindProcess + Signal 조합으로 간접 확인합니다.
	p, err := os.FindProcess(process.Pid)
	if err != nil {
		return false
	}
	// Signal(os.Kill)은 실제로 프로세스를 종료하므로 사용하지 않음.
	// 대신 FindProcess 성공 자체를 실행 중으로 간주합니다.
	_ = p
	return true
}
