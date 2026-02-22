// Package cmd provides CLI commands for Local Agent Bridge.
// tools.go implements business tool detection, installation, and status reporting.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

// toolCategory 도구 분류
type toolCategory string

const (
	toolCategoryEssential   toolCategory = "essential"
	toolCategoryRecommended toolCategory = "recommended"
	toolCategoryDeveloper   toolCategory = "developer"
	toolCategoryOptional    toolCategory = "optional"
)

// businessTool 비즈니스 도구 정보
type businessTool struct {
	Name       string            `json:"name"`
	Category   toolCategory      `json:"category"`
	Purpose    string            `json:"purpose"`
	Installed  bool              `json:"installed"`
	Version    string            `json:"version,omitempty"`
	Path       string            `json:"path,omitempty"`
	InstallCmd map[string]string `json:"-"` // darwin, linux
}

// toolsCmd 도구 관리 명령어
var toolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "비즈니스 도구를 관리합니다",
	Long: `Autopus 에이전트가 사용하는 비즈니스 도구(pandoc, csvkit 등)를 감지하고 설치합니다.

서브커맨드:
  list      설치 현황 표시
  install   미설치 도구 대화형 설치
  check     상태 확인 (JSON 출력, CI용)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var toolsListCmd = &cobra.Command{
	Use:   "list",
	Short: "비즈니스 도구 설치 현황을 표시합니다",
	RunE:  runToolsList,
}

var toolsInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "미설치 도구를 대화형으로 설치합니다",
	RunE:  runToolsInstall,
}

var toolsCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "도구 상태를 JSON으로 출력합니다 (CI용)",
	RunE:  runToolsCheck,
}

func init() {
	rootCmd.AddCommand(toolsCmd)
	toolsCmd.AddCommand(toolsListCmd)
	toolsCmd.AddCommand(toolsInstallCmd)
	toolsCmd.AddCommand(toolsCheckCmd)
}

// getBusinessToolManifest 비즈니스 도구 매니페스트를 반환합니다.
func getBusinessToolManifest() []businessTool {
	return []businessTool{
		{
			Name:     "pandoc",
			Category: toolCategoryEssential,
			Purpose:  "문서 변환 (Markdown->PDF/DOCX/PPTX)",
			InstallCmd: map[string]string{
				"darwin": "brew install pandoc",
				"linux":  "sudo apt-get install -y pandoc",
			},
		},
		{
			Name:     "python3",
			Category: toolCategoryEssential,
			Purpose:  "데이터 분석/스크립트",
			InstallCmd: map[string]string{
				"darwin": "brew install python3",
				"linux":  "sudo apt-get install -y python3",
			},
		},
		{
			Name:     "node",
			Category: toolCategoryEssential,
			Purpose:  "JavaScript 런타임 (AI CLI 필수)",
			InstallCmd: map[string]string{
				"darwin": "brew install node",
				"linux":  "curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash - && sudo apt-get install -y nodejs",
			},
		},
		{
			Name:     "csvkit",
			Category: toolCategoryRecommended,
			Purpose:  "CSV 처리 (csvstat, csvsql)",
			InstallCmd: map[string]string{
				"darwin": "pip3 install csvkit",
				"linux":  "pip3 install csvkit",
			},
		},
		{
			Name:     "mlr",
			Category: toolCategoryRecommended,
			Purpose:  "CSV/JSON 고급 가공",
			InstallCmd: map[string]string{
				"darwin": "brew install miller",
				"linux":  "sudo apt-get install -y miller",
			},
		},
		{
			Name:     "gnuplot",
			Category: toolCategoryRecommended,
			Purpose:  "CLI 차트 생성",
			InstallCmd: map[string]string{
				"darwin": "brew install gnuplot",
				"linux":  "sudo apt-get install -y gnuplot",
			},
		},
		{
			Name:     "marp",
			Category: toolCategoryRecommended,
			Purpose:  "Markdown 슬라이드 생성",
			InstallCmd: map[string]string{
				"darwin": "npm install -g @marp-team/marp-cli",
				"linux":  "npm install -g @marp-team/marp-cli",
			},
		},
		{
			Name:     "d2",
			Category: toolCategoryRecommended,
			Purpose:  "다이어그램 생성",
			InstallCmd: map[string]string{
				"darwin": "brew install d2",
				"linux":  "curl -fsSL https://d2lang.com/install.sh | sh -s --",
			},
		},
		{
			Name:     "jq",
			Category: toolCategoryDeveloper,
			Purpose:  "JSON 처리",
			InstallCmd: map[string]string{
				"darwin": "brew install jq",
				"linux":  "sudo apt-get install -y jq",
			},
		},
		{
			Name:     "yq",
			Category: toolCategoryDeveloper,
			Purpose:  "YAML 처리",
			InstallCmd: map[string]string{
				"darwin": "brew install yq",
				"linux":  "sudo apt-get install -y yq",
			},
		},
		{
			Name:     "gh",
			Category: toolCategoryDeveloper,
			Purpose:  "GitHub CLI",
			InstallCmd: map[string]string{
				"darwin": "brew install gh",
				"linux":  "sudo apt-get install -y gh",
			},
		},
		{
			Name:     "rg",
			Category: toolCategoryDeveloper,
			Purpose:  "ripgrep 고속 검색",
			InstallCmd: map[string]string{
				"darwin": "brew install ripgrep",
				"linux":  "sudo apt-get install -y ripgrep",
			},
		},
		{
			Name:     "fzf",
			Category: toolCategoryDeveloper,
			Purpose:  "퍼지 파인더",
			InstallCmd: map[string]string{
				"darwin": "brew install fzf",
				"linux":  "sudo apt-get install -y fzf",
			},
		},
		{
			Name:     "hledger",
			Category: toolCategoryOptional,
			Purpose:  "복식부기 회계",
			InstallCmd: map[string]string{
				"darwin": "brew install hledger",
				"linux":  "sudo apt-get install -y hledger",
			},
		},
		{
			Name:     "wkhtmltopdf",
			Category: toolCategoryOptional,
			Purpose:  "HTML->PDF 변환",
			InstallCmd: map[string]string{
				"darwin": "brew install wkhtmltopdf",
				"linux":  "sudo apt-get install -y wkhtmltopdf",
			},
		},
	}
}

// detectBusinessTools 비즈니스 도구 설치 상태를 감지합니다.
func detectBusinessTools() []businessTool {
	tools := getBusinessToolManifest()

	for i := range tools {
		// csvkit는 csvstat 바이너리로 감지
		checkName := tools[i].Name
		if checkName == "csvkit" {
			checkName = "csvstat"
		}

		path, err := exec.LookPath(checkName)
		if err == nil {
			tools[i].Installed = true
			tools[i].Path = path
			tools[i].Version = getToolVersion(checkName)
		}
	}

	return tools
}

// getToolVersion 도구 버전을 가져옵니다.
func getToolVersion(name string) string {
	var cmd *exec.Cmd

	switch name {
	case "pandoc":
		cmd = exec.Command("pandoc", "--version")
	case "python3":
		cmd = exec.Command("python3", "--version")
	case "csvstat":
		cmd = exec.Command("csvstat", "--version")
	case "mlr":
		cmd = exec.Command("mlr", "--version")
	case "gnuplot":
		cmd = exec.Command("gnuplot", "--version")
	case "marp":
		cmd = exec.Command("marp", "--version")
	case "d2":
		cmd = exec.Command("d2", "--version")
	case "hledger":
		cmd = exec.Command("hledger", "--version")
	case "wkhtmltopdf":
		cmd = exec.Command("wkhtmltopdf", "--version")
	case "node":
		cmd = exec.Command("node", "--version")
	case "jq":
		cmd = exec.Command("jq", "--version")
	case "yq":
		cmd = exec.Command("yq", "--version")
	case "gh":
		cmd = exec.Command("gh", "--version")
	case "rg":
		cmd = exec.Command("rg", "--version")
	case "fzf":
		cmd = exec.Command("fzf", "--version")
	default:
		return ""
	}

	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	// 첫 줄에서 버전 정보 추출
	line := strings.TrimSpace(strings.Split(string(out), "\n")[0])
	return line
}

// runToolsList 도구 현황을 표시합니다.
func runToolsList(cmd *cobra.Command, args []string) error {
	tools := detectBusinessTools()

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println(" Autopus 비즈니스 도구 현황")
	fmt.Println("========================================")

	categories := []struct {
		cat   toolCategory
		label string
	}{
		{toolCategoryEssential, "필수 (Essential)"},
		{toolCategoryRecommended, "권장 (Recommended)"},
		{toolCategoryDeveloper, "개발자 (Developer)"},
		{toolCategoryOptional, "선택 (Optional)"},
	}

	for _, c := range categories {
		fmt.Printf("\n  %s:\n", c.label)
		for _, t := range tools {
			if t.Category != c.cat {
				continue
			}
			if t.Installed {
				ver := ""
				if t.Version != "" {
					ver = fmt.Sprintf(" (%s)", truncateVersion(t.Version, 30))
				}
				fmt.Printf("    [v] %-14s %s%s\n", t.Name, t.Purpose, ver)
			} else {
				fmt.Printf("    [ ] %-14s %s\n", t.Name, t.Purpose)
			}
		}
	}

	// 요약
	installed, total := countTools(tools)
	fmt.Printf("\n  합계: %d/%d 설치됨\n\n", installed, total)

	return nil
}

// runToolsInstall 미설치 도구를 대화형으로 설치합니다.
func runToolsInstall(cmd *cobra.Command, args []string) error {
	tools := detectBusinessTools()

	missing := filterMissing(tools)
	if len(missing) == 0 {
		fmt.Println()
		fmt.Println("  모든 비즈니스 도구가 설치되어 있습니다.")
		fmt.Println()
		return nil
	}

	fmt.Println()
	fmt.Println("미설치 도구:")
	for _, t := range missing {
		fmt.Printf("  [ ] %-14s %s [%s]\n", t.Name, t.Purpose, t.Category)
	}

	fmt.Printf("\n%d개 도구를 설치하시겠습니까? (Y/n): ", len(missing))
	var input string
	if _, err := fmt.Scanln(&input); err != nil {
		input = "y" // 기본값: 설치 진행
	}
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "n" || input == "no" {
		fmt.Println("설치를 취소합니다.")
		return nil
	}

	osName := runtime.GOOS
	for _, t := range missing {
		installCmd, ok := t.InstallCmd[osName]
		if !ok {
			fmt.Printf("  ! %-14s %s에서 자동 설치를 지원하지 않습니다\n", t.Name, osName)
			continue
		}

		fmt.Printf("  설치 중: %s ...\n", installCmd)
		if err := runInstallCommand(installCmd); err != nil {
			fmt.Printf("  ✗ %s 설치 실패: %v\n", t.Name, err)
		} else {
			fmt.Printf("  ✓ %s 설치 완료\n", t.Name)
		}
	}

	fmt.Println()
	return nil
}

// runToolsCheck 도구 상태를 JSON으로 출력합니다.
func runToolsCheck(cmd *cobra.Command, args []string) error {
	tools := detectBusinessTools()

	type checkResult struct {
		Tools     []businessTool `json:"tools"`
		Installed int            `json:"installed"`
		Total     int            `json:"total"`
		AllReady  bool           `json:"all_ready"`
	}

	installed, total := countTools(tools)
	result := checkResult{
		Tools:     tools,
		Installed: installed,
		Total:     total,
		AllReady:  installed == total,
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON 직렬화 실패: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

// runInstallCommand 설치 명령을 실행합니다.
func runInstallCommand(installCmd string) error {
	parts := strings.Fields(installCmd)
	if len(parts) == 0 {
		return fmt.Errorf("빈 설치 명령")
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// filterMissing 미설치 도구만 필터링합니다.
func filterMissing(tools []businessTool) []businessTool {
	var missing []businessTool
	for _, t := range tools {
		if !t.Installed {
			missing = append(missing, t)
		}
	}
	return missing
}

// countTools 설치된 도구 수를 셉니다.
func countTools(tools []businessTool) (installed, total int) {
	total = len(tools)
	for _, t := range tools {
		if t.Installed {
			installed++
		}
	}
	return
}

// truncateVersion 버전 문자열을 지정 길이로 자릅니다.
func truncateVersion(v string, maxLen int) string {
	if len(v) <= maxLen {
		return v
	}
	return v[:maxLen-3] + "..."
}

// BuildToolsCapabilities capabilities 메시지에 포함할 도구 상태를 생성합니다.
func BuildToolsCapabilities() map[string]interface{} {
	tools := detectBusinessTools()
	result := make(map[string]interface{})

	for _, t := range tools {
		entry := map[string]interface{}{
			"installed": t.Installed,
			"category":  string(t.Category),
			"purpose":   t.Purpose,
		}
		if t.Installed && t.Version != "" {
			entry["version"] = t.Version
		}
		result[t.Name] = entry
	}

	return result
}
