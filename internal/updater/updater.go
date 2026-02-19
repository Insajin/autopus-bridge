// Package updater는 CLI 자동 업데이트 기능을 제공합니다.
// FR-P1-08: GitHub Releases를 통한 자동 업데이트 시스템
package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	// githubAPIBase는 GitHub API 기본 URL입니다.
	githubAPIBase = "https://api.github.com"
	// checkIntervalHours는 업데이트 확인 간격(시간)입니다.
	checkIntervalHours = 24
	// httpTimeout는 HTTP 요청 타임아웃입니다.
	httpTimeout = 30 * time.Second
	// lastCheckFile는 마지막 업데이트 확인 시간을 저장하는 파일명입니다.
	lastCheckFile = ".last_update_check"
)

// Updater는 자동 업데이트를 관리하는 구조체입니다.
type Updater struct {
	currentVersion string
	githubRepo     string
	checkInterval  time.Duration
	httpClient     *http.Client
}

// Release는 GitHub Release 정보를 나타냅니다.
type Release struct {
	Version     string    `json:"tag_name"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []Asset   `json:"assets"`
	Body        string    `json:"body"`
	HTMLURL     string    `json:"html_url"`
}

// Asset는 GitHub Release의 첨부 파일 정보를 나타냅니다.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
	ContentType        string `json:"content_type"`
}

// New는 새로운 Updater를 생성합니다.
func New(currentVersion, githubRepo string) *Updater {
	return &Updater{
		currentVersion: currentVersion,
		githubRepo:     githubRepo,
		checkInterval:  checkIntervalHours * time.Hour,
		httpClient: &http.Client{
			Timeout: httpTimeout,
		},
	}
}

// CheckForUpdate는 최신 릴리스를 확인하고 업데이트 여부를 반환합니다.
// 반환값: 최신 릴리스, 업데이트 존재 여부, 에러
func (u *Updater) CheckForUpdate() (*Release, bool, error) {
	release, err := u.fetchLatestRelease()
	if err != nil {
		return nil, false, fmt.Errorf("최신 릴리스 확인 실패: %w", err)
	}

	// 마지막 확인 시간 기록
	_ = u.saveLastCheckTime()

	// 버전 비교
	latestVersion := normalizeVersion(release.Version)
	currentVersion := normalizeVersion(u.currentVersion)

	if currentVersion == "dev" || currentVersion == "" {
		// dev 빌드는 업데이트 대상이 아님
		return release, false, nil
	}

	hasUpdate := compareVersions(latestVersion, currentVersion) > 0
	return release, hasUpdate, nil
}

// DownloadAndReplace는 최신 바이너리를 다운로드하고 현재 바이너리를 교체합니다.
func (u *Updater) DownloadAndReplace(release *Release) error {
	// 현재 플랫폼에 맞는 에셋 찾기
	asset, err := u.findPlatformAsset(release)
	if err != nil {
		return fmt.Errorf("플랫폼 에셋 검색 실패: %w", err)
	}

	// 체크섬 파일 찾기 및 다운로드
	expectedChecksum, err := u.fetchChecksum(release, asset.Name)
	if err != nil {
		// 체크섬 파일이 없는 경우에도 경고만 출력하고 계속 진행
		fmt.Printf("  경고: 체크섬 파일을 찾을 수 없습니다. 무결성 검증을 건너뜁니다.\n")
		expectedChecksum = ""
	}

	// 바이너리 다운로드
	fmt.Printf("  다운로드 중: %s (%.1f MB)\n", asset.Name, float64(asset.Size)/(1024*1024))

	tmpFile, err := u.downloadAsset(asset)
	if err != nil {
		return fmt.Errorf("바이너리 다운로드 실패: %w", err)
	}
	defer func() {
		// 임시 파일 정리 (교체 성공 시에는 이미 삭제됨)
		_ = os.Remove(tmpFile)
	}()

	// SHA256 체크섬 검증
	if expectedChecksum != "" {
		fmt.Printf("  체크섬 검증 중...\n")
		if err := verifyChecksum(tmpFile, expectedChecksum); err != nil {
			return fmt.Errorf("체크섬 검증 실패: %w", err)
		}
		fmt.Printf("  체크섬 검증 완료\n")
	}

	// 현재 바이너리 교체 (atomic rename)
	currentBinary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("현재 바이너리 경로 확인 실패: %w", err)
	}

	// 심볼릭 링크 해석
	currentBinary, err = filepath.EvalSymlinks(currentBinary)
	if err != nil {
		return fmt.Errorf("심볼릭 링크 해석 실패: %w", err)
	}

	// 실행 권한 설정
	if err := os.Chmod(tmpFile, 0755); err != nil {
		return fmt.Errorf("실행 권한 설정 실패: %w", err)
	}

	// 기존 바이너리 백업
	backupPath := currentBinary + ".bak"
	if err := os.Rename(currentBinary, backupPath); err != nil {
		return fmt.Errorf("기존 바이너리 백업 실패: %w", err)
	}

	// 새 바이너리 이동
	if err := os.Rename(tmpFile, currentBinary); err != nil {
		// 실패 시 백업 복원
		_ = os.Rename(backupPath, currentBinary)
		return fmt.Errorf("새 바이너리 설치 실패: %w", err)
	}

	// 백업 파일 삭제
	_ = os.Remove(backupPath)

	return nil
}

// ShouldCheck는 업데이트 확인이 필요한지 판단합니다.
// 마지막 확인 시간으로부터 checkInterval 이상 경과한 경우 true를 반환합니다.
func (u *Updater) ShouldCheck() bool {
	lastCheck, err := u.loadLastCheckTime()
	if err != nil {
		// 파일이 없거나 읽기 실패한 경우 확인 필요
		return true
	}

	return time.Since(lastCheck) >= u.checkInterval
}

// GetCurrentVersion은 현재 버전을 반환합니다.
func (u *Updater) GetCurrentVersion() string {
	return u.currentVersion
}

// fetchLatestRelease는 GitHub API에서 최신 릴리스 정보를 가져옵니다.
func (u *Updater) fetchLatestRelease() (*Release, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", githubAPIBase, u.githubRepo)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", fmt.Sprintf("autopus-bridge/%s", u.currentVersion))

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GitHub API 요청 실패: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Rate limit 처리
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("GitHub API 요청 한도 초과. 잠시 후 다시 시도하세요")
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("릴리스를 찾을 수 없습니다: %s", u.githubRepo)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API 응답 오류 (HTTP %d)", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("릴리스 정보 파싱 실패: %w", err)
	}

	return &release, nil
}

// findPlatformAsset는 현재 OS/아키텍처에 맞는 에셋을 찾습니다.
func (u *Updater) findPlatformAsset(release *Release) (*Asset, error) {
	osName := runtime.GOOS
	archName := runtime.GOARCH

	// 아키텍처 매핑 (일반적인 네이밍 패턴)
	archAliases := map[string][]string{
		"amd64": {"amd64", "x86_64", "x64"},
		"arm64": {"arm64", "aarch64"},
		"386":   {"386", "i386", "x86"},
	}

	aliases, ok := archAliases[archName]
	if !ok {
		aliases = []string{archName}
	}

	for _, asset := range release.Assets {
		name := strings.ToLower(asset.Name)

		// OS 이름 확인
		if !strings.Contains(name, osName) {
			continue
		}

		// 아키텍처 확인
		for _, alias := range aliases {
			if strings.Contains(name, strings.ToLower(alias)) {
				// 체크섬 파일 제외
				if strings.HasSuffix(name, ".sha256") || strings.Contains(name, "checksums") {
					continue
				}
				return &asset, nil
			}
		}
	}

	return nil, fmt.Errorf("현재 플랫폼(%s/%s)에 맞는 바이너리를 찾을 수 없습니다", osName, archName)
}

// fetchChecksum는 체크섬 파일에서 특정 에셋의 SHA256 해시를 가져옵니다.
func (u *Updater) fetchChecksum(release *Release, assetName string) (string, error) {
	// checksums.txt 또는 SHA256SUMS 파일 찾기
	var checksumAsset *Asset
	for _, asset := range release.Assets {
		name := strings.ToLower(asset.Name)
		if strings.Contains(name, "checksums") || strings.Contains(name, "sha256sums") || strings.HasSuffix(name, "checksums.txt") {
			checksumAsset = &asset
			break
		}
	}

	if checksumAsset == nil {
		return "", fmt.Errorf("체크섬 파일을 찾을 수 없습니다")
	}

	// 체크섬 파일 다운로드
	resp, err := u.httpClient.Get(checksumAsset.BrowserDownloadURL)
	if err != nil {
		return "", fmt.Errorf("체크섬 파일 다운로드 실패: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("체크섬 파일 다운로드 실패 (HTTP %d)", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("체크섬 파일 읽기 실패: %w", err)
	}

	// 체크섬 파일 파싱 (형식: <hash>  <filename> 또는 <hash> <filename>)
	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 공백으로 분리
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			fileName := parts[len(parts)-1]
			// 파일명 앞의 * 제거 (바이너리 모드 표시)
			fileName = strings.TrimPrefix(fileName, "*")
			if fileName == assetName {
				return parts[0], nil
			}
		}
	}

	return "", fmt.Errorf("에셋 '%s'의 체크섬을 찾을 수 없습니다", assetName)
}

// downloadAsset는 에셋을 다운로드하고 임시 파일 경로를 반환합니다.
func (u *Updater) downloadAsset(asset *Asset) (string, error) {
	resp, err := u.httpClient.Get(asset.BrowserDownloadURL)
	if err != nil {
		return "", fmt.Errorf("다운로드 요청 실패: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("다운로드 실패 (HTTP %d)", resp.StatusCode)
	}

	// 임시 파일 생성
	tmpFile, err := os.CreateTemp("", "autopus-bridge-update-*")
	if err != nil {
		return "", fmt.Errorf("임시 파일 생성 실패: %w", err)
	}
	defer func() { _ = tmpFile.Close() }()

	// 다운로드 (진행률 표시)
	written, err := io.Copy(tmpFile, resp.Body)
	if err != nil {
		_ = os.Remove(tmpFile.Name())
		return "", fmt.Errorf("파일 쓰기 실패: %w", err)
	}

	if written == 0 {
		_ = os.Remove(tmpFile.Name())
		return "", fmt.Errorf("다운로드된 파일이 비어있습니다")
	}

	return tmpFile.Name(), nil
}

// verifyChecksum는 파일의 SHA256 체크섬을 검증합니다.
func verifyChecksum(filePath, expectedChecksum string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("파일 열기 실패: %w", err)
	}
	defer func() { _ = f.Close() }()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return fmt.Errorf("해시 계산 실패: %w", err)
	}

	actualChecksum := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(actualChecksum, expectedChecksum) {
		return fmt.Errorf("체크섬 불일치: 예상 %s, 실제 %s", expectedChecksum, actualChecksum)
	}

	return nil
}

// getConfigDir는 설정 디렉토리 경로를 반환합니다.
func getConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("홈 디렉토리를 찾을 수 없습니다: %w", err)
	}
	return filepath.Join(home, ".config", "local-agent-bridge"), nil
}

// saveLastCheckTime는 현재 시간을 마지막 확인 시간으로 저장합니다.
func (u *Updater) saveLastCheckTime() error {
	configDir, err := getConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("설정 디렉토리 생성 실패: %w", err)
	}

	filePath := filepath.Join(configDir, lastCheckFile)
	data := []byte(time.Now().Format(time.RFC3339))
	return os.WriteFile(filePath, data, 0600)
}

// loadLastCheckTime는 마지막 확인 시간을 로드합니다.
func (u *Updater) loadLastCheckTime() (time.Time, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return time.Time{}, err
	}

	filePath := filepath.Join(configDir, lastCheckFile)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return time.Time{}, err
	}

	return time.Parse(time.RFC3339, strings.TrimSpace(string(data)))
}

// normalizeVersion는 버전 문자열에서 'v' 접두사를 제거합니다.
func normalizeVersion(version string) string {
	return strings.TrimPrefix(strings.TrimSpace(version), "v")
}

// compareVersions는 두 semver 버전을 비교합니다.
// a > b이면 1, a == b이면 0, a < b이면 -1을 반환합니다.
func compareVersions(a, b string) int {
	aParts := parseVersion(a)
	bParts := parseVersion(b)

	for i := 0; i < 3; i++ {
		if aParts[i] > bParts[i] {
			return 1
		}
		if aParts[i] < bParts[i] {
			return -1
		}
	}

	return 0
}

// parseVersion는 semver 문자열을 [major, minor, patch] 배열로 파싱합니다.
func parseVersion(version string) [3]int {
	var result [3]int

	// 프리릴리스 접미사 제거 (예: 1.2.3-beta.1)
	if idx := strings.IndexByte(version, '-'); idx >= 0 {
		version = version[:idx]
	}

	parts := strings.Split(version, ".")
	for i := 0; i < 3 && i < len(parts); i++ {
		val := 0
		for _, c := range parts[i] {
			if c >= '0' && c <= '9' {
				val = val*10 + int(c-'0')
			} else {
				break
			}
		}
		result[i] = val
	}

	return result
}
