// embed.go는 Claude Code 플러그인 템플릿 파일을 바이너리에 포함합니다.
package aitools

import "embed"

//go:embed all:plugin-dist
var pluginFiles embed.FS
