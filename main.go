// Package main은 Local Agent Bridge CLI의 진입점입니다.
// Autopus 서버와 WebSocket으로 통신하여 사용자의 로컬 AI 에이전트를 연결합니다.
package main

import (
	"os"

	"github.com/insajin/autopus-bridge/cmd"
)

// 빌드 시 ldflags로 주입되는 버전 정보
var (
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"
)

func main() {
	// 버전 정보를 root 패키지에 설정
	cmd.SetVersionInfo(version, commit, buildDate)

	// CLI 실행
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
