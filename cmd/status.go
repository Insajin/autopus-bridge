// Package cmd는 Local Agent Bridge CLI의 명령어를 정의합니다.
// status.go는 연결 상태 확인 명령을 구현합니다.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/insajin/autopus-bridge/internal/config"
	"github.com/spf13/cobra"
)

// StatusInfo는 상태 정보를 담는 구조체입니다.
type StatusInfo struct {
	// Connected는 연결 상태입니다.
	Connected bool `json:"connected"`
	// ServerURL은 연결된 서버 URL입니다.
	ServerURL string `json:"server_url,omitempty"`
	// StartTime은 연결 시작 시간입니다.
	StartTime *time.Time `json:"start_time,omitempty"`
	// Uptime은 연결 유지 시간입니다.
	Uptime string `json:"uptime,omitempty"`
	// CurrentTask는 현재 실행 중인 작업 ID입니다.
	CurrentTask string `json:"current_task,omitempty"`
	// TasksCompleted는 완료된 작업 수입니다.
	TasksCompleted int `json:"tasks_completed"`
	// TasksFailed는 실패한 작업 수입니다.
	TasksFailed int `json:"tasks_failed"`
	// PID는 실행 중인 프로세스 ID입니다.
	PID int `json:"pid,omitempty"`
}

// statusCmd는 현재 연결 상태를 확인하는 명령어입니다.
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "현재 연결 상태를 확인합니다",
	Long: `Autopus Local Bridge의 현재 연결 상태와 실행 통계를 표시합니다.

표시 항목:
  - 연결 상태 (연결됨/연결되지 않음)
  - 서버 URL
  - 연결 유지 시간
  - 현재 실행 중인 작업
  - 완료/실패한 작업 수

이 명령은 상태 파일을 기반으로 정보를 표시합니다.
실제 연결 확인을 위해서는 connect 명령으로 서버에 연결하세요.`,
	RunE: runStatus,
}

var (
	statusJSON   bool
	statusSimple bool
)

func init() {
	rootCmd.AddCommand(statusCmd)

	// status 명령 플래그
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "JSON 형식으로 출력")
	statusCmd.Flags().BoolVarP(&statusSimple, "simple", "s", false, "간단한 형식으로 출력")
}

// runStatus는 status 명령의 실행 로직입니다.
func runStatus(cmd *cobra.Command, args []string) error {
	// 상태 정보 수집
	status, err := collectStatus()
	if err != nil {
		return fmt.Errorf("상태 수집 실패: %w", err)
	}

	// 출력 형식에 따라 표시
	if statusJSON {
		return printStatusJSON(status)
	}
	if statusSimple {
		return printStatusSimple(status)
	}
	return printStatusFull(status)
}

// collectStatus는 현재 상태 정보를 수집합니다.
func collectStatus() (*StatusInfo, error) {
	status := &StatusInfo{
		Connected:      false,
		TasksCompleted: 0,
		TasksFailed:    0,
	}

	// 상태 파일에서 정보 읽기 시도
	statusFile := getStatusFilePath()
	if data, err := os.ReadFile(statusFile); err == nil {
		var fileStatus StatusInfo
		if err := json.Unmarshal(data, &fileStatus); err == nil {
			status = &fileStatus

			// PID가 있으면 프로세스가 실행 중인지 확인
			if status.PID > 0 {
				if !isProcessRunning(status.PID) {
					// 프로세스가 종료됨 - 상태 초기화
					status.Connected = false
					status.CurrentTask = ""
					status.PID = 0
				}
			}

			// 연결 중이고 시작 시간이 있으면 uptime 계산
			if status.Connected && status.StartTime != nil {
				status.Uptime = formatDuration(time.Since(*status.StartTime))
			}
		}
	}

	// 설정에서 서버 URL 가져오기
	cfg, err := config.Load()
	if err == nil {
		status.ServerURL = cfg.Server.URL
	}

	return status, nil
}

// printStatusJSON는 JSON 형식으로 상태를 출력합니다.
func printStatusJSON(status *StatusInfo) error {
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON 직렬화 실패: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

// printStatusSimple는 간단한 형식으로 상태를 출력합니다.
func printStatusSimple(status *StatusInfo) error {
	if status.Connected {
		fmt.Println("connected")
	} else {
		fmt.Println("disconnected")
	}
	return nil
}

// printStatusFull는 전체 형식으로 상태를 출력합니다.
func printStatusFull(status *StatusInfo) error {
	fmt.Println("Autopus Local Bridge 상태")
	fmt.Println("========================")
	fmt.Println()

	// 연결 상태
	if status.Connected {
		fmt.Println("상태:        연결됨")
		if status.PID > 0 {
			fmt.Printf("프로세스 ID: %d\n", status.PID)
		}
	} else {
		fmt.Println("상태:        연결되지 않음")
	}

	// 서버 URL
	if status.ServerURL != "" {
		fmt.Printf("서버:        %s\n", status.ServerURL)
	}

	// 연결 시간
	if status.Connected && status.Uptime != "" {
		fmt.Printf("연결 시간:   %s\n", status.Uptime)
	}

	fmt.Println()

	// 작업 상태
	fmt.Println("작업 통계")
	fmt.Println("---------")
	if status.CurrentTask != "" {
		fmt.Printf("현재 작업:   %s (실행 중)\n", status.CurrentTask)
	} else {
		fmt.Println("현재 작업:   없음")
	}
	fmt.Printf("완료됨:      %d\n", status.TasksCompleted)
	fmt.Printf("실패함:      %d\n", status.TasksFailed)

	fmt.Println()

	// 환경변수 상태
	fmt.Println("환경변수 상태")
	fmt.Println("-------------")
	printEnvStatusForStatus("CLAUDE_API_KEY")
	printEnvStatusForStatus("GEMINI_API_KEY")
	printEnvStatusForStatus("LAB_TOKEN")

	fmt.Println()

	// 안내 메시지
	if !status.Connected {
		fmt.Println("서버에 연결하려면:")
		fmt.Println("  autopus connect --token <JWT_TOKEN>")
	}

	return nil
}

// getStatusFilePath는 상태 파일 경로를 반환합니다.
func getStatusFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "autopus", "status.json")
}

// isProcessRunning은 주어진 PID의 프로세스가 실행 중인지 확인합니다.
func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Unix 계열에서는 Signal 0을 보내서 프로세스 존재 확인
	err = process.Signal(os.Signal(nil))
	return err == nil
}

// formatDuration은 기간을 읽기 쉬운 형식으로 포맷합니다.
func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%d일 %d시간 %d분", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%d시간 %d분 %d초", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%d분 %d초", minutes, seconds)
	}
	return fmt.Sprintf("%d초", seconds)
}

// printEnvStatusForStatus는 환경변수 설정 상태를 출력합니다.
func printEnvStatusForStatus(envVar string) {
	value := os.Getenv(envVar)
	if value != "" {
		fmt.Printf("  %s: 설정됨\n", envVar)
	} else {
		fmt.Printf("  %s: 설정되지 않음\n", envVar)
	}
}

// SaveStatus는 현재 상태를 파일에 저장합니다.
// connect 명령에서 사용됩니다.
func SaveStatus(status *StatusInfo) error {
	statusFile := getStatusFilePath()
	if statusFile == "" {
		return fmt.Errorf("상태 파일 경로를 찾을 수 없습니다")
	}

	// 디렉토리 확인/생성
	if err := config.EnsureConfigDir(); err != nil {
		return fmt.Errorf("설정 디렉토리 생성 실패: %w", err)
	}

	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON 직렬화 실패: %w", err)
	}

	if err := os.WriteFile(statusFile, data, 0600); err != nil {
		return fmt.Errorf("상태 파일 저장 실패: %w", err)
	}

	return nil
}

// ClearStatus는 상태 파일을 삭제합니다.
// disconnect 시 사용됩니다.
func ClearStatus() error {
	statusFile := getStatusFilePath()
	if statusFile == "" {
		return nil
	}
	return os.Remove(statusFile)
}
