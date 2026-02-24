// embed.go는 Agent Skill 파일을 바이너리에 포함합니다.
package aitools

import "embed"

//go:embed all:skill-dist
var skillFiles embed.FS
