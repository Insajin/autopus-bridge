// api.go는 raw API 요청 CLI 명령어를 구현합니다.
// AC31-AC36: autopus api <METHOD> <PATH> [--data <JSON>] [-H <header>]
package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/spf13/cobra"
)

var (
	apiData    string
	apiHeaders []string
)

// apiCmd는 raw API 요청을 실행하는 명령어입니다.
var apiCmd = &cobra.Command{
	Use:   "api <METHOD> <PATH>",
	Short: "Autopus API에 직접 요청을 실행합니다",
	Long: `Autopus 백엔드 API에 HTTP 요청을 직접 전송합니다.

예시:
  autopus api GET /api/v1/workspaces
  autopus api POST /api/v1/channels/:channelId/messages --data '{"content":"test"}'
  autopus api DELETE /api/v1/channels/:channelId

:workspaceId는 현재 워크스페이스 ID로 자동 치환됩니다.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}

		method := strings.ToUpper(args[0])
		path := args[1]

		// --data 미지정 시 stdin에서 읽기 (터미널이 아닌 경우)
		data := apiData
		if data == "" && !isTerminal(os.Stdin) {
			stdinData, readErr := io.ReadAll(os.Stdin)
			if readErr == nil && len(stdinData) > 0 {
				data = string(stdinData)
			}
		}

		return runRawAPI(cmd.Context(), client, os.Stdout, method, path, data, apiHeaders)
	},
}

func init() {
	rootCmd.AddCommand(apiCmd)

	apiCmd.Flags().StringVarP(&apiData, "data", "d", "", "요청 본문 (JSON 문자열)")
	apiCmd.Flags().StringArrayVarP(&apiHeaders, "header", "H", nil, "추가 헤더 (반복 가능, 예: -H 'X-Foo: bar')")
}

// runRawAPI는 API 요청을 실행하고 응답을 출력합니다.
// 테스트에서 직접 호출할 수 있도록 client와 out을 주입받습니다.
func runRawAPI(ctx context.Context, client *apiclient.Client, out io.Writer, method, path, data string, headers []string) error {
	// :workspaceId 치환
	resolvedPath := client.ResolvePath(path)

	// 요청 본문 파싱
	var body interface{}
	if data != "" {
		if err := json.Unmarshal([]byte(data), &body); err != nil {
			return fmt.Errorf("요청 본문 JSON 파싱 실패: %w", err)
		}
	}

	// 추가 헤더 파싱
	extraHeaders := parseHeaders(headers)

	// Client.DoRaw를 사용하여 HTTP 상태 코드와 응답 본문을 캡처
	statusCode, respBody, err := client.DoRaw(ctx, method, resolvedPath, body, extraHeaders)
	if err != nil {
		return err
	}

	// 상태 코드 출력
	fmt.Fprintf(out, "HTTP %d\n\n", statusCode)

	// JSON pretty-print 출력
	var prettyBuf bytes.Buffer
	if jsonErr := json.Indent(&prettyBuf, respBody, "", "  "); jsonErr == nil {
		fmt.Fprintln(out, prettyBuf.String())
	} else {
		// JSON이 아닌 경우 그대로 출력
		fmt.Fprintln(out, string(respBody))
	}

	return nil
}

// parseHeaders는 "Key: Value" 형식의 헤더 문자열 슬라이스를 map으로 변환합니다.
func parseHeaders(headers []string) map[string]string {
	result := make(map[string]string)
	for _, h := range headers {
		parts := strings.SplitN(h, ":", 2)
		if len(parts) == 2 {
			result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return result
}

// isTerminal은 주어진 파일이 터미널(tty)인지 확인합니다.
func isTerminal(f *os.File) bool {
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}
