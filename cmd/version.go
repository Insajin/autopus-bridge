package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// versionCmd는 버전 정보를 출력하는 명령어입니다.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "버전 정보를 출력합니다",
	Long:  `Autopus Local Bridge의 버전, 커밋 해시, 빌드 날짜를 출력합니다.`,
	Run: func(cmd *cobra.Command, args []string) {
		version, commit, buildDate := GetVersionInfo()

		fmt.Printf("Autopus Local Bridge\n")
		fmt.Printf("  Version:    %s\n", version)
		fmt.Printf("  Commit:     %s\n", commit)
		fmt.Printf("  Built:      %s\n", buildDate)
		fmt.Printf("  Go version: %s\n", runtime.Version())
		fmt.Printf("  OS/Arch:    %s/%s\n", runtime.GOOS, runtime.GOARCH)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
