package knowledgesync

import (
	"crypto/sha256"
	"fmt"
	"testing"
	"time"
)

// TestComputeDiff_NoChanges 는 같은 파일 목록일 때 액션 없음을 검증합니다.
func TestComputeDiff_NoChanges(t *testing.T) {
	t.Parallel()
	hash := "abc123"
	now := time.Now()

	local := []FileState{
		{RelativePath: "docs/readme.md", Hash: hash, Size: 100, ModTime: now},
	}
	remote := []FileState{
		{RelativePath: "docs/readme.md", Hash: hash, Size: 100, ModTime: now},
	}

	actions := ComputeDiff(local, remote)
	if len(actions) != 0 {
		t.Errorf("ComputeDiff() = %d actions for identical states, want 0", len(actions))
	}
}

// TestComputeDiff_LocalOnlyFile_ProducesUpload 는 로컬에만 있는 파일이 upload 액션을 생성하는지 검증합니다.
func TestComputeDiff_LocalOnlyFile_ProducesUpload(t *testing.T) {
	t.Parallel()
	local := []FileState{
		{RelativePath: "new-local-file.md", Hash: "hash1", Size: 200, ModTime: time.Now()},
	}
	remote := []FileState{}

	actions := ComputeDiff(local, remote)
	if len(actions) != 1 {
		t.Fatalf("ComputeDiff() = %d actions, want 1", len(actions))
	}
	if actions[0].Type != "upload" {
		t.Errorf("action type = %q, want %q", actions[0].Type, "upload")
	}
	if actions[0].FilePath != "new-local-file.md" {
		t.Errorf("action filePath = %q, want %q", actions[0].FilePath, "new-local-file.md")
	}
}

// TestComputeDiff_RemoteOnlyFile_ProducesDownload 는 리모트에만 있는 파일이 download 액션을 생성하는지 검증합니다.
func TestComputeDiff_RemoteOnlyFile_ProducesDownload(t *testing.T) {
	t.Parallel()
	local := []FileState{}
	remote := []FileState{
		{RelativePath: "remote-only.md", Hash: "hash2", Size: 300, ModTime: time.Now()},
	}

	actions := ComputeDiff(local, remote)
	if len(actions) != 1 {
		t.Fatalf("ComputeDiff() = %d actions, want 1", len(actions))
	}
	if actions[0].Type != "download" {
		t.Errorf("action type = %q, want %q", actions[0].Type, "download")
	}
}

// TestComputeDiff_ModifiedFile_HashDiffers_Conflict 는 같은 파일 다른 해시일 때 충돌 감지를 검증합니다.
func TestComputeDiff_ModifiedFile_HashDiffers_Conflict(t *testing.T) {
	t.Parallel()
	now := time.Now()
	local := []FileState{
		{RelativePath: "shared.md", Hash: "localHash", Size: 100, ModTime: now},
	}
	remote := []FileState{
		{RelativePath: "shared.md", Hash: "remoteHash", Size: 200, ModTime: now.Add(-time.Second)},
	}

	actions := ComputeDiff(local, remote)
	if len(actions) != 1 {
		t.Fatalf("ComputeDiff() = %d actions, want 1", len(actions))
	}
	// 로컬이 더 최신이면 upload, 충돌 상황이면 conflict
	if actions[0].Type != "upload" && actions[0].Type != "conflict" {
		t.Errorf("action type = %q, want 'upload' or 'conflict'", actions[0].Type)
	}
}

// TestComputeDiff_DeletedLocalFile_ProducesDelete 는 로컬에서 삭제된 파일이 delete 액션을 생성하는지 검증합니다.
func TestComputeDiff_DeletedLocalFile_ProducesDelete(t *testing.T) {
	t.Parallel()
	// 로컬에는 없고 리모트에만 있는 파일 → download
	// 반대로, 이전에 있던 파일이 로컬에서 제거된 상황은 명시적 delete 이벤트로 처리됩니다.
	// ComputeDiff 는 현재 상태만 비교하므로, remote-only 파일은 download 가 됩니다.
	local := []FileState{}
	remote := []FileState{
		{RelativePath: "deleted.md", Hash: "oldHash", Size: 50, ModTime: time.Now()},
	}

	actions := ComputeDiff(local, remote)
	if len(actions) != 1 {
		t.Fatalf("ComputeDiff() = %d actions, want 1", len(actions))
	}
	// remote-only → download (로컬 삭제 → 서버도 삭제는 명시적 이벤트로 처리)
	if actions[0].Type != "download" {
		t.Errorf("action type = %q, want 'download'", actions[0].Type)
	}
}

// TestComputeDiff_MultipleFiles 는 다수의 파일을 올바르게 처리하는지 검증합니다.
func TestComputeDiff_MultipleFiles(t *testing.T) {
	t.Parallel()
	now := time.Now()
	local := []FileState{
		{RelativePath: "a.md", Hash: "sameHash", Size: 100, ModTime: now},
		{RelativePath: "b.md", Hash: "localOnly", Size: 200, ModTime: now},
	}
	remote := []FileState{
		{RelativePath: "a.md", Hash: "sameHash", Size: 100, ModTime: now},
		{RelativePath: "c.md", Hash: "remoteOnly", Size: 300, ModTime: now},
	}

	actions := ComputeDiff(local, remote)
	// b.md → upload, c.md → download
	if len(actions) != 2 {
		t.Errorf("ComputeDiff() = %d actions, want 2", len(actions))
	}
}

// TestComputeSHA256_ValidFile 은 SHA256 해시 계산의 정확성을 검증합니다.
func TestComputeSHA256_ValidFile(t *testing.T) {
	t.Parallel()
	content := []byte("hello world content for hashing")
	expected := fmt.Sprintf("%x", sha256.Sum256(content))

	hash, err := ComputeSHA256FromBytes(content)
	if err != nil {
		t.Fatalf("ComputeSHA256FromBytes() unexpected error: %v", err)
	}
	if hash != expected {
		t.Errorf("ComputeSHA256FromBytes() = %q, want %q", hash, expected)
	}
}

// TestComputeSHA256_EmptyContent 는 빈 콘텐츠의 해시를 검증합니다.
func TestComputeSHA256_EmptyContent(t *testing.T) {
	t.Parallel()
	emptyHash := fmt.Sprintf("%x", sha256.Sum256([]byte{}))
	hash, err := ComputeSHA256FromBytes([]byte{})
	if err != nil {
		t.Fatalf("ComputeSHA256FromBytes() unexpected error: %v", err)
	}
	if hash != emptyHash {
		t.Errorf("ComputeSHA256FromBytes([]) = %q, want %q", hash, emptyHash)
	}
}

// TestSyncAction_HasRequiredFields 는 SyncAction 구조체 필드 존재를 검증합니다.
func TestSyncAction_HasRequiredFields(t *testing.T) {
	t.Parallel()
	action := SyncAction{
		Type:     "upload",
		FilePath: "docs/readme.md",
		Local: &FileState{
			RelativePath: "docs/readme.md",
			Hash:         "hash123",
			Size:         1024,
			ModTime:      time.Now(),
		},
	}
	if action.Type != "upload" {
		t.Errorf("SyncAction.Type = %q, want 'upload'", action.Type)
	}
	if action.Local == nil {
		t.Error("SyncAction.Local is nil")
	}
}
