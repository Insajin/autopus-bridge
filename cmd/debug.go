// debug.go는 디버그 유틸리티 CLI 명령어를 구현합니다.
// debug ping/ws/token 서브커맨드 제공
package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/insajin/autopus-bridge/internal/auth"
	"github.com/spf13/cobra"
)

var (
	debugTokenJSON bool
)

// debugCmd는 debug 서브커맨드의 루트입니다.
var debugCmd = &cobra.Command{
	Use:   "debug",
	Short: "디버그 유틸리티",
	Long:  `연결 테스트, WebSocket 확인, 토큰 정보 조회 기능을 제공합니다.`,
}

// debugPingCmd는 서버 응답 시간을 측정합니다.
var debugPingCmd = &cobra.Command{
	Use:   "ping",
	Short: "서버 응답 시간 측정",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runDebugPing(client, os.Stdout)
	},
}

// debugWSCmd는 WebSocket 연결 가능성을 테스트합니다.
var debugWSCmd = &cobra.Command{
	Use:   "ws",
	Short: "WebSocket 연결 테스트",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runDebugWS(client, os.Stdout)
	},
}

// debugTokenCmd는 현재 JWT 토큰 정보를 조회합니다.
var debugTokenCmd = &cobra.Command{
	Use:   "token",
	Short: "JWT 토큰 정보 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		return runDebugToken(client, os.Stdout, jsonOut)
	},
}

func init() {
	rootCmd.AddCommand(debugCmd)
	debugCmd.AddCommand(debugPingCmd)
	debugCmd.AddCommand(debugWSCmd)
	debugCmd.AddCommand(debugTokenCmd)

	debugTokenCmd.Flags().BoolVar(&debugTokenJSON, "json", false, "JSON 형식으로 출력")
}

// runDebugPing은 서버에 GET 요청을 전송하고 응답 시간을 출력합니다.
// /api/v1/workspaces/:workspaceId/org-health 엔드포인트 사용
func runDebugPing(client *apiclient.Client, out io.Writer) error {
	ctx, cancel := apiclient.NewContextWithTimeout(5 * time.Second)
	defer cancel()

	start := time.Now()
	_, err := client.Get(ctx, "/api/v1/workspaces/"+client.WorkspaceID()+"/org-health")
	elapsed := time.Since(start)

	if err != nil {
		fmt.Fprintf(out, "PING FAILED: %v\n", err)
		return err
	}
	fmt.Fprintf(out, "PONG: %dms\n", elapsed.Milliseconds())
	return nil
}

// runDebugWS는 WebSocket URL을 추론하고 HTTP 기반 연결을 테스트합니다.
// @MX:NOTE: 실제 WebSocket 업그레이드 없이 HTTP 경로로 서버 연결 가능 여부 확인
func runDebugWS(client *apiclient.Client, out io.Writer) error {
	baseURL := client.BaseURL()

	// HTTP URL에서 WebSocket URL 추론
	wsURL := strings.Replace(baseURL, "https://", "wss://", 1)
	wsURL = strings.Replace(wsURL, "http://", "ws://", 1)
	wsURL += "/ws/agent"

	fmt.Fprintf(out, "WebSocket URL: %s\n", wsURL)

	// HTTP 경로로 서버 연결 가능 여부 확인
	ctx, cancel := apiclient.NewContextWithTimeout(5 * time.Second)
	defer cancel()

	_, err := client.Get(ctx, "/api/v1/workspaces/"+client.WorkspaceID()+"/org-health")
	if err != nil {
		fmt.Fprintf(out, "서버 연결 실패: %v\n", err)
		return err
	}
	fmt.Fprintf(out, "서버 연결 성공 (HTTP 경로 확인)\n")
	return nil
}

// runDebugToken은 현재 JWT 토큰 정보를 조회하여 출력합니다.
// 토큰은 마스킹하여 출력하며, 만료 여부를 확인합니다.
// @MX:NOTE: auth.MaskToken으로 토큰 보안 마스킹, auth.ParseJWTExpiry로 만료 시간 파싱
func runDebugToken(client *apiclient.Client, out io.Writer, jsonOutput bool) error {
	token, err := client.Token()
	if err != nil {
		return fmt.Errorf("인증 토큰 획득 실패: %w", err)
	}

	masked := auth.MaskToken(token)
	workspaceID := client.WorkspaceID()

	// JWT에서 만료 시간 파싱
	expiry, parseErr := auth.ParseJWTExpiry(token)
	remaining := time.Duration(0)
	expiryStr := "알 수 없음"

	if parseErr == nil {
		expiryStr = expiry.Format(time.RFC3339)
		remaining = time.Until(expiry)
	}

	if jsonOutput {
		result := map[string]interface{}{
			"token":        masked,
			"workspace_id": workspaceID,
			"expires_at":   expiryStr,
			"remaining":    remaining.Round(time.Second).String(),
		}
		return apiclient.PrintJSON(out, result)
	}

	fields := []apiclient.KeyValue{
		{Key: "Token", Value: masked},
		{Key: "WorkspaceID", Value: workspaceID},
		{Key: "ExpiresAt", Value: expiryStr},
		{Key: "Remaining", Value: remaining.Round(time.Second).String()},
	}

	// 만료 경고 출력
	if parseErr == nil && remaining <= 0 {
		fmt.Fprintf(out, "경고: 토큰이 만료되었습니다. 'autopus login'으로 재로그인하세요.\n")
	}

	apiclient.PrintDetail(out, fields)
	return nil
}
