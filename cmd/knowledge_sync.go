package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/insajin/autopus-bridge/internal/knowledgesync"
	"github.com/spf13/cobra"
)

var (
	// knowledge sync 플래그
	syncFolder         string
	syncSourceID       string
	syncWatch          bool
	syncExcludeExtra   string
	syncBandwidthLimit float64
)

var errKnowledgeSyncNotImplemented = errors.New("knowledge sync transport is not implemented yet")

// knowledgeSyncCmd 는 `autopus knowledge sync` 커맨드입니다.
// 로컬 폴더와 Knowledge Hub 소스를 양방향 동기화합니다.
// SPEC-KHSOURCE-001 TASK-009
var knowledgeSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "로컬 폴더를 Knowledge Hub 소스와 동기화합니다",
	Long: `로컬 폴더를 Knowledge Hub 소스와 양방향으로 동기화합니다.

SHA256 해시 기반 증분 동기화를 수행하며, 충돌 시 Last-Write-Wins 전략을 사용합니다.
--watch 플래그를 사용하면 파일 시스템 변경을 감지하여 실시간으로 동기화합니다.

예시:
  autopus knowledge sync --folder /path/to/docs --source-id <uuid>
  autopus knowledge sync --folder /path/to/docs --source-id <uuid> --watch
  autopus knowledge sync --folder /path/to/docs --source-id <uuid> --exclude "*.log,tmp/**"`,
	RunE: runKnowledgeSync,
}

// init 는 knowledgeSyncCmd 플래그와 부모 커맨드를 등록합니다.
func init() {
	knowledgeSyncCmd.Flags().StringVar(&syncFolder, "folder", "",
		"동기화할 로컬 폴더 경로 (필수)")
	knowledgeSyncCmd.Flags().StringVar(&syncSourceID, "source-id", "",
		"Knowledge Hub 소스 UUID (필수)")
	knowledgeSyncCmd.Flags().BoolVar(&syncWatch, "watch", false,
		"파일 변경 감지 후 실시간 동기화 활성화")
	knowledgeSyncCmd.Flags().StringVar(&syncExcludeExtra, "exclude", "",
		"추가 제외 패턴 (쉼표 구분, 예: *.log,.tmp/**)")
	knowledgeSyncCmd.Flags().Float64Var(&syncBandwidthLimit, "bandwidth-limit", 0,
		"최대 대역폭 (MB/s, 0 = 무제한)")

	_ = knowledgeSyncCmd.MarkFlagRequired("folder")
	_ = knowledgeSyncCmd.MarkFlagRequired("source-id")
}

// runKnowledgeSync 는 knowledge sync 커맨드를 실행합니다.
func runKnowledgeSync(cmd *cobra.Command, args []string) error {
	// 폴더 유효성 검사
	if _, err := os.Stat(syncFolder); os.IsNotExist(err) {
		return fmt.Errorf("폴더를 찾을 수 없습니다: %s", syncFolder)
	}

	// 추가 제외 패턴 파싱
	var extraExcludes []string
	if syncExcludeExtra != "" {
		for _, p := range strings.Split(syncExcludeExtra, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				extraExcludes = append(extraExcludes, p)
			}
		}
	}

	// 제외 패턴 병합
	excludePatterns := knowledgesync.MergePatterns(knowledgesync.DefaultExcludePatterns, extraExcludes)

	fmt.Printf("Knowledge Hub 동기화 시작\n")
	fmt.Printf("  폴더: %s\n", syncFolder)
	fmt.Printf("  소스 ID: %s\n", syncSourceID)
	fmt.Printf("  감시 모드: %v\n", syncWatch)
	fmt.Printf("  제외 패턴: %d개\n", len(excludePatterns))

	return fmt.Errorf("%w (source_id=%s)", errKnowledgeSyncNotImplemented, syncSourceID)
}

// runOnceMode 는 일회성 동기화를 실행합니다.
func runOnceMode(excludePatterns []string) error {
	fmt.Println("일회성 동기화를 실행합니다...")

	// TODO: WebSocket 연결 및 실제 동기화 구현
	// 현재는 로컬 파일 스캔 및 해시 계산만 수행

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	_ = ctx

	fmt.Println("동기화 완료")
	return nil
}

// runWatchMode 는 파일 감시 모드로 실시간 동기화를 실행합니다.
func runWatchMode(excludePatterns []string) error {
	fmt.Println("파일 감시 모드를 시작합니다...")

	watcher, err := knowledgesync.NewWatcher(excludePatterns)
	if err != nil {
		return fmt.Errorf("파일 감시자 생성 실패: %w", err)
	}
	defer watcher.Close()

	if err := watcher.Add(syncFolder); err != nil {
		return fmt.Errorf("폴더 감시 등록 실패: %w", err)
	}

	fmt.Printf("폴더 감시 중: %s\n", syncFolder)
	fmt.Println("종료하려면 Ctrl+C 를 누르세요")

	// 종료 시그널 수신
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	events := watcher.Events()
	for {
		select {
		case event, ok := <-events:
			if !ok {
				return nil
			}
			fmt.Printf("파일 변경: [%s] %s\n", event.Type, event.FilePath)
			// TODO: WebSocket 으로 변경 이벤트 전송

		case sig := <-sigChan:
			fmt.Printf("\n신호 수신: %v, 종료합니다.\n", sig)
			return nil
		}
	}
}
