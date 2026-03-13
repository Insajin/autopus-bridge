// Package cmd는 Local Agent Bridge CLI의 명령어를 정의합니다.
// update.go는 CLI 자동 업데이트 명령을 구현합니다.
// FR-P1-08: GitHub Releases를 통한 자동 업데이트 시스템
package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/insajin/autopus-bridge/internal/updater"
	"github.com/spf13/cobra"
)

const (
	// githubRepo는 GitHub 저장소 경로입니다.
	githubRepo = "Insajin/autopus-bridge"
	// updateStopTimeout은 실행 중인 connect 프로세스의 정상 종료 대기 시간입니다.
	updateStopTimeout = 20 * time.Second
	// updateStopPollInterval은 종료 대기 중 상태 확인 간격입니다.
	updateStopPollInterval = 250 * time.Millisecond
)

var (
	updateSleep            = time.Sleep
	updateExecCommand      = exec.Command
	updateFindProcess      = os.FindProcess
	updateProcessRunningFn = isProcessRunning
)

type runningConnectProcess struct {
	PID       int
	ServerURL string
}

// updateCmd는 최신 버전으로 업데이트하는 명령어입니다.
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "최신 버전으로 업데이트합니다",
	Long: `GitHub Releases에서 최신 버전을 확인하고 자동으로 업데이트합니다.

업데이트 과정:
  1. GitHub에서 최신 릴리스 확인
  2. 현재 버전과 비교
  3. 새 버전이 있으면 다운로드 및 설치
  4. SHA256 체크섬 검증
  5. 바이너리 교체 (atomic rename)`,
	RunE: runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

// runUpdate는 update 명령의 실행 로직입니다.
func runUpdate(cmd *cobra.Command, args []string) error {
	version, _, _ := GetVersionInfo()

	// dev 빌드 확인
	if version == "" || version == "dev" {
		fmt.Println("개발 빌드에서는 자동 업데이트를 사용할 수 없습니다.")
		fmt.Println("릴리스 빌드를 사용하세요: https://github.com/" + githubRepo + "/releases")
		return nil
	}

	fmt.Printf("현재 버전: v%s\n", version)
	fmt.Println("최신 버전 확인 중...")

	u := updater.New(version, githubRepo)
	runningProc, err := detectRunningConnectProcess()
	if err != nil {
		return fmt.Errorf("실행 중인 connect 프로세스 확인 실패: %w", err)
	}

	release, hasUpdate, err := u.CheckForUpdate()
	if err != nil {
		return fmt.Errorf("업데이트 확인 실패: %w", err)
	}

	if !hasUpdate {
		fmt.Printf("\n이미 최신 버전입니다 (v%s)\n", version)
		return nil
	}

	fmt.Printf("\n새 버전 발견: %s (현재: v%s)\n", release.Version, version)
	fmt.Println("업데이트를 시작합니다...")
	fmt.Println()

	if runningProc != nil {
		fmt.Printf("실행 중인 연결 프로세스 감지: PID %d\n", runningProc.PID)
		fmt.Println("기존 연결을 정상 종료한 뒤 새 버전으로 다시 시작합니다...")
		if err := stopRunningConnectProcess(runningProc.PID); err != nil {
			return fmt.Errorf("연결 프로세스 종료 실패: %w", err)
		}
	}

	if err := u.DownloadAndReplace(release); err != nil {
		return fmt.Errorf("업데이트 실패: %w", err)
	}

	fmt.Printf("\n업데이트 완료: v%s -> %s\n", version, release.Version)
	fmt.Println("새 버전이 적용되었습니다.")
	if runningProc != nil {
		if err := restartConnectProcess(runningProc); err != nil {
			return fmt.Errorf("업데이트는 완료되었지만 연결 자동 복구 실패: %w", err)
		}
		fmt.Println("연결 프로세스를 새 버전으로 다시 시작했습니다.")
	}

	return nil
}

func detectRunningConnectProcess() (*runningConnectProcess, error) {
	status, err := loadRawStatusInfo()
	if err == nil && status != nil && status.Connected && status.PID > 0 &&
		status.PID != os.Getpid() && updateProcessRunningFn(status.PID) {
		return &runningConnectProcess{
			PID:       status.PID,
			ServerURL: status.ServerURL,
		}, nil
	}

	lockPath, err := getConnectLockPath(resolveCurrentWorkspaceScopeID())
	if err != nil {
		return nil, err
	}
	pid, err := readLockPID(lockPath)
	if err != nil || pid <= 0 || pid == os.Getpid() || !updateProcessRunningFn(pid) {
		return nil, nil
	}

	serverURL := ""
	if status != nil {
		serverURL = status.ServerURL
	}

	return &runningConnectProcess{
		PID:       pid,
		ServerURL: serverURL,
	}, nil
}

func loadRawStatusInfo() (*StatusInfo, error) {
	statusFile := getStatusFilePath()
	data, err := os.ReadFile(statusFile)
	if err != nil {
		return nil, err
	}

	var status StatusInfo
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, err
	}

	return &status, nil
}

func stopRunningConnectProcess(pid int) error {
	proc, err := updateFindProcess(pid)
	if err != nil {
		return fmt.Errorf("프로세스 조회 실패: %w", err)
	}

	if err := proc.Signal(os.Interrupt); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("정상 종료 시그널 전송 실패: %w", err)
	}

	if waitForProcessExit(pid, updateStopTimeout) {
		return nil
	}

	if err := proc.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("강제 종료 실패: %w", err)
	}

	if !waitForProcessExit(pid, 5*time.Second) {
		return fmt.Errorf("프로세스 종료 대기 시간 초과 (PID: %d)", pid)
	}

	return nil
}

func waitForProcessExit(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if !updateProcessRunningFn(pid) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		updateSleep(updateStopPollInterval)
	}
}

func restartConnectProcess(proc *runningConnectProcess) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("현재 실행 파일 경로 확인 실패: %w", err)
	}

	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("실행 파일 경로 해석 실패: %w", err)
	}

	args := buildReconnectArgs(proc)
	restartCmd := updateExecCommand(execPath, args...)
	restartCmd.Env = os.Environ()

	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("출력 리디렉션 준비 실패: %w", err)
	}
	defer devNull.Close()

	restartCmd.Stdin = devNull
	restartCmd.Stdout = devNull
	restartCmd.Stderr = devNull

	if err := restartCmd.Start(); err != nil {
		return fmt.Errorf("connect 재시작 실패: %w", err)
	}

	if restartCmd.Process != nil {
		_ = restartCmd.Process.Release()
	}

	return nil
}

func buildReconnectArgs(proc *runningConnectProcess) []string {
	args := make([]string, 0, 6)
	if cfgFile != "" {
		args = append(args, "--config", cfgFile)
	}
	if verbose {
		args = append(args, "--verbose")
	}
	args = append(args, "connect")
	if proc != nil && proc.ServerURL != "" {
		args = append(args, "--server", proc.ServerURL)
	}
	return args
}

// CheckUpdateInBackground는 백그라운드에서 업데이트를 확인합니다.
// connect 명령의 시작 시 호출되며, 24시간에 한 번만 확인합니다.
// 업데이트가 있으면 안내 메시지를 출력합니다.
// 메인 동작을 차단하지 않습니다.
func CheckUpdateInBackground(currentVersion string) {
	// dev 빌드 무시
	if currentVersion == "" || currentVersion == "dev" {
		return
	}

	go func() {
		u := updater.New(currentVersion, githubRepo)

		// 24시간 이내에 이미 확인했으면 건너뛰기
		if !u.ShouldCheck() {
			return
		}

		release, hasUpdate, err := u.CheckForUpdate()
		if err != nil {
			// 백그라운드 확인이므로 에러 무시
			return
		}

		if hasUpdate {
			fmt.Printf("\n새 버전 %s이 사용 가능합니다. 'autopus-bridge update'로 업데이트하세요.\n\n",
				release.Version)
		}
	}()
}
