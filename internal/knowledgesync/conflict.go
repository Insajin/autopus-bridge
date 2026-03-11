package knowledgesync

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// ConflictResolution 은 LWW 충돌 해결 결과를 담습니다.
// SPEC-KHSOURCE-001 TASK-013
type ConflictResolution struct {
	// Winner 는 "local" 또는 "remote" 입니다.
	Winner string
	// BackupPath 는 패배한 파일의 백업 경로입니다.
	// 패턴: {dirname}/{basename}.conflict.{timestamp}{ext}
	BackupPath string
	// Reason 은 사람이 읽을 수 있는 충돌 해결 이유입니다.
	Reason string
}

// ResolveLWW 는 LWW (Last-Write-Wins) 전략으로 로컬과 리모트 파일 충돌을 해결합니다.
// 더 최신 수정 시각을 가진 파일이 승리하며, 패배한 파일은 백업 경로에 보존됩니다.
// SPEC-KHSOURCE-001 TASK-013
//
// @MX:WARN: LWW 충돌 해결 — 패배한 파일 버전이 덮어쓰여집니다
// @MX:REASON: 타임스탬프 비교로 승자 결정, 동시 수정 시 remote 우선 (안전한 기본값)
// @MX:SPEC: SPEC-KHSOURCE-001
func ResolveLWW(local, remote FileState) ConflictResolution {
	var winner string
	var loserPath string
	var reason string

	if local.ModTime.After(remote.ModTime) {
		// 로컬이 더 최신 → 로컬 승리, 리모트를 백업
		winner = "local"
		loserPath = remote.RelativePath
		reason = fmt.Sprintf(
			"로컬 파일이 더 최신입니다 (local: %s, remote: %s)",
			local.ModTime.Format(time.RFC3339),
			remote.ModTime.Format(time.RFC3339),
		)
	} else {
		// 동시 수정 또는 리모트가 최신 → 리모트 승리, 로컬을 백업
		winner = "remote"
		loserPath = local.RelativePath
		if local.ModTime.Equal(remote.ModTime) {
			reason = fmt.Sprintf(
				"수정 시각이 동일합니다. 리모트를 기본값으로 선택합니다 (%s)",
				local.ModTime.Format(time.RFC3339),
			)
		} else {
			reason = fmt.Sprintf(
				"리모트 파일이 더 최신입니다 (local: %s, remote: %s)",
				local.ModTime.Format(time.RFC3339),
				remote.ModTime.Format(time.RFC3339),
			)
		}
	}

	backupPath := buildConflictBackupPath(loserPath)

	return ConflictResolution{
		Winner:     winner,
		BackupPath: backupPath,
		Reason:     reason,
	}
}

// buildConflictBackupPath 는 충돌 파일의 백업 경로를 생성합니다.
// 패턴: {dirname}/{basename}.conflict.{timestamp}{ext}
// 예: docs/readme.md → docs/readme.conflict.20260311T150405.md
func buildConflictBackupPath(filePath string) string {
	dir := filepath.Dir(filePath)
	base := filepath.Base(filePath)
	ext := filepath.Ext(base)
	nameWithoutExt := strings.TrimSuffix(base, ext)

	timestamp := time.Now().UTC().Format("20060102T150405")
	conflictName := fmt.Sprintf("%s.conflict.%s%s", nameWithoutExt, timestamp, ext)

	if dir == "." {
		return conflictName
	}
	return filepath.ToSlash(filepath.Join(dir, conflictName))
}
