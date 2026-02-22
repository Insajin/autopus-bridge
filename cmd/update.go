// Package cmd는 Local Agent Bridge CLI의 명령어를 정의합니다.
// update.go는 CLI 자동 업데이트 명령을 구현합니다.
// FR-P1-08: GitHub Releases를 통한 자동 업데이트 시스템
package cmd

import (
	"fmt"

	"github.com/insajin/autopus-bridge/internal/updater"
	"github.com/spf13/cobra"
)

const (
	// githubRepo는 GitHub 저장소 경로입니다.
	githubRepo = "Insajin/autopus-bridge"
)

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

	if err := u.DownloadAndReplace(release); err != nil {
		return fmt.Errorf("업데이트 실패: %w", err)
	}

	fmt.Printf("\n업데이트 완료: v%s -> %s\n", version, release.Version)
	fmt.Println("새 버전이 적용되었습니다.")

	return nil
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
