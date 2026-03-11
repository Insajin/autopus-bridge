package knowledgesync

import (
	"crypto/sha256"
	"fmt"
	"time"
)

// FileState 는 동기화 비교를 위한 파일 메타데이터를 담습니다.
// SPEC-KHSOURCE-001 TASK-012
type FileState struct {
	RelativePath string    // 소스 루트 기준 상대 경로
	Hash         string    // SHA256 해시
	Size         int64     // 바이트 단위 크기
	ModTime      time.Time // 마지막 수정 시각
}

// SyncAction 은 동기화 실행 단위를 나타냅니다.
// SPEC-KHSOURCE-001 TASK-012
type SyncAction struct {
	// Type 은 "upload", "download", "delete", "conflict" 중 하나입니다.
	Type string
	// FilePath 는 대상 파일의 상대 경로입니다.
	FilePath string
	// Local 은 로컬 파일 상태입니다 (없으면 nil).
	Local *FileState
	// Remote 는 리모트 파일 상태입니다 (없으면 nil).
	Remote *FileState
}

// ComputeDiff 는 로컬과 리모트 파일 상태를 비교하여 필요한 동기화 액션 목록을 반환합니다.
// SHA256 해시로 변경을 감지하며 증분 동기화를 수행합니다.
// SPEC-KHSOURCE-001 TASK-012
//
// @MX:ANCHOR: ComputeDiff — 로컬/리모트 파일 상태 비교 및 동기화 액션 생성
// @MX:REASON: syncer 의 핵심 함수, watcher 와 ExecuteSync 에서 모두 사용
// @MX:SPEC: SPEC-KHSOURCE-001
func ComputeDiff(localFiles, remoteFiles []FileState) []SyncAction {
	// 빠른 조회를 위한 맵 구성
	localMap := make(map[string]*FileState, len(localFiles))
	for i := range localFiles {
		localMap[localFiles[i].RelativePath] = &localFiles[i]
	}

	remoteMap := make(map[string]*FileState, len(remoteFiles))
	for i := range remoteFiles {
		remoteMap[remoteFiles[i].RelativePath] = &remoteFiles[i]
	}

	var actions []SyncAction

	// 로컬에만 있는 파일 → upload
	for path, local := range localMap {
		remote, exists := remoteMap[path]
		if !exists {
			// 리모트에 없음 → 업로드
			localCopy := *local
			actions = append(actions, SyncAction{
				Type:     "upload",
				FilePath: path,
				Local:    &localCopy,
			})
			continue
		}

		// 양쪽에 있지만 해시가 다름 → 충돌 또는 업로드
		if local.Hash != remote.Hash {
			localCopy := *local
			remoteCopy := *remote
			if local.ModTime.After(remote.ModTime) {
				// 로컬이 더 최신 → 업로드
				actions = append(actions, SyncAction{
					Type:     "upload",
					FilePath: path,
					Local:    &localCopy,
					Remote:   &remoteCopy,
				})
			} else {
				// 리모트가 최신이거나 동시 수정 → 충돌
				actions = append(actions, SyncAction{
					Type:     "conflict",
					FilePath: path,
					Local:    &localCopy,
					Remote:   &remoteCopy,
				})
			}
		}
		// 해시 동일 → 변경 없음, 액션 불필요
	}

	// 리모트에만 있는 파일 → download
	for path, remote := range remoteMap {
		if _, exists := localMap[path]; !exists {
			remoteCopy := *remote
			actions = append(actions, SyncAction{
				Type:     "download",
				FilePath: path,
				Remote:   &remoteCopy,
			})
		}
	}

	return actions
}

// ComputeSHA256FromBytes 는 바이트 슬라이스의 SHA256 해시를 16진수 문자열로 반환합니다.
// SPEC-KHSOURCE-001 TASK-012
func ComputeSHA256FromBytes(data []byte) (string, error) {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum), nil
}
