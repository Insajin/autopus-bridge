// skills.go는 Agent Skills 설치 기능을 제공합니다.
// Gemini CLI와 Codex CLI는 Agent Skills 표준(~/.agents/skills/)을 지원합니다.
package aitools

import (
	"fmt"
	"os"
	"path/filepath"
)

// agentSkillSourcePath는 임베디드 파일시스템 내 스킬 파일의 경로입니다.
const agentSkillSourcePath = "plugin-dist/skills/autopus-platform/SKILL.md"

// agentSkillRelDir는 대상 디렉토리 내 스킬이 설치될 하위 경로입니다.
const agentSkillRelDir = "autopus-platform"

// agentSkillFileName는 스킬 파일 이름입니다.
const agentSkillFileName = "SKILL.md"

// IsAgentSkillInstalled는 Agent Skill이 ~/.agents/skills/autopus-platform/에
// 이미 설치되어 있는지 확인합니다.
func IsAgentSkillInstalled() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	skillPath := filepath.Join(home, ".agents", "skills", agentSkillRelDir, agentSkillFileName)
	_, err = os.Stat(skillPath)
	return err == nil
}

// InstallAgentSkill은 autopus-platform 스킬을 ~/.agents/skills/autopus-platform/에 설치합니다.
// Gemini CLI와 Codex CLI가 ~/.agents/skills/를 스캔하여 스킬을 자동 로드합니다.
func InstallAgentSkill() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("홈 디렉토리 확인 실패: %w", err)
	}

	targetDir := filepath.Join(home, ".agents", "skills")
	return InstallAgentSkillTo(targetDir)
}

// InstallAgentSkillTo는 지정된 경로에 Agent Skill을 설치합니다 (테스트용).
// targetDir는 ~/.agents/skills/ 에 해당하는 상위 디렉토리입니다.
func InstallAgentSkillTo(targetDir string) error {
	// 임베디드 파일에서 SKILL.md 읽기
	data, err := pluginFiles.ReadFile(agentSkillSourcePath)
	if err != nil {
		return fmt.Errorf("임베디드 스킬 파일 읽기 실패 (%s): %w", agentSkillSourcePath, err)
	}

	// 대상 디렉토리 생성: targetDir/autopus-platform/
	skillDir := filepath.Join(targetDir, agentSkillRelDir)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return fmt.Errorf("스킬 디렉토리 생성 실패: %w", err)
	}

	// SKILL.md 복사
	targetPath := filepath.Join(skillDir, agentSkillFileName)
	if err := os.WriteFile(targetPath, data, 0644); err != nil {
		return fmt.Errorf("스킬 파일 저장 실패: %w", err)
	}

	return nil
}
