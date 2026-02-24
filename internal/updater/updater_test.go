package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// createTestTarGz는 테스트용 tar.gz 아카이브를 생성합니다.
func createTestTarGz(t *testing.T, binaryName string, content []byte) string {
	t.Helper()

	tmpFile, err := os.CreateTemp(t.TempDir(), "test-archive-*.tar.gz")
	if err != nil {
		t.Fatalf("임시 파일 생성 실패: %v", err)
	}

	gw := gzip.NewWriter(tmpFile)
	tw := tar.NewWriter(gw)

	// 바이너리 파일 추가
	header := &tar.Header{
		Name: binaryName,
		Mode: 0755,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(header); err != nil {
		t.Fatalf("tar 헤더 쓰기 실패: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("tar 데이터 쓰기 실패: %v", err)
	}

	_ = tw.Close()
	_ = gw.Close()
	_ = tmpFile.Close()

	return tmpFile.Name()
}

// createTestZip는 테스트용 zip 아카이브를 생성합니다.
func createTestZip(t *testing.T, binaryName string, content []byte) string {
	t.Helper()

	tmpFile, err := os.CreateTemp(t.TempDir(), "test-archive-*.zip")
	if err != nil {
		t.Fatalf("임시 파일 생성 실패: %v", err)
	}

	zw := zip.NewWriter(tmpFile)

	w, err := zw.Create(binaryName)
	if err != nil {
		t.Fatalf("zip 엔트리 생성 실패: %v", err)
	}
	if _, err := w.Write(content); err != nil {
		t.Fatalf("zip 데이터 쓰기 실패: %v", err)
	}

	_ = zw.Close()
	_ = tmpFile.Close()

	return tmpFile.Name()
}

func TestExtractBinary_TarGz(t *testing.T) {
	t.Parallel()

	content := []byte("#!/bin/sh\necho hello\n")
	archivePath := createTestTarGz(t, "autopus-bridge", content)

	extracted, err := extractBinary(archivePath, "test_1.0.0_linux_amd64.tar.gz")
	if err != nil {
		t.Fatalf("extractBinary 실패: %v", err)
	}
	defer os.Remove(extracted)

	data, err := os.ReadFile(extracted)
	if err != nil {
		t.Fatalf("추출된 파일 읽기 실패: %v", err)
	}

	if string(data) != string(content) {
		t.Errorf("내용 불일치: 예상 %q, 실제 %q", string(content), string(data))
	}
}

func TestExtractBinary_TarGz_NestedPath(t *testing.T) {
	t.Parallel()

	// GoReleaser가 서브디렉토리에 바이너리를 넣을 수 있음
	content := []byte("binary-data-here")
	tmpFile, err := os.CreateTemp(t.TempDir(), "test-nested-*.tar.gz")
	if err != nil {
		t.Fatal(err)
	}

	gw := gzip.NewWriter(tmpFile)
	tw := tar.NewWriter(gw)

	// 디렉토리 엔트리
	_ = tw.WriteHeader(&tar.Header{
		Name:     "autopus-bridge_1.0.0_linux_amd64/",
		Typeflag: tar.TypeDir,
		Mode:     0755,
	})

	// 바이너리 엔트리 (서브디렉토리 내)
	_ = tw.WriteHeader(&tar.Header{
		Name: "autopus-bridge_1.0.0_linux_amd64/autopus-bridge",
		Mode: 0755,
		Size: int64(len(content)),
	})
	_, _ = tw.Write(content)

	_ = tw.Close()
	_ = gw.Close()
	_ = tmpFile.Close()

	extracted, err := extractBinary(tmpFile.Name(), "test.tar.gz")
	if err != nil {
		t.Fatalf("중첩 경로 추출 실패: %v", err)
	}
	defer os.Remove(extracted)

	data, err := os.ReadFile(extracted)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(content) {
		t.Errorf("내용 불일치: 예상 %q, 실제 %q", string(content), string(data))
	}
}

func TestExtractBinary_Zip(t *testing.T) {
	t.Parallel()

	// 현재 플랫폼에 맞는 바이너리 이름 사용
	binaryName := "autopus-bridge"
	if runtime.GOOS == "windows" {
		binaryName = "autopus-bridge.exe"
	}

	content := []byte("zip-binary-data")
	archivePath := createTestZip(t, binaryName, content)

	// assetName이 .zip이면 zip 경로로 분기
	extracted, err := extractBinary(archivePath, "test_1.0.0_linux_amd64.zip")
	if err != nil {
		t.Fatalf("extractBinary(zip) 실패: %v", err)
	}
	defer os.Remove(extracted)

	data, err := os.ReadFile(extracted)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(content) {
		t.Errorf("내용 불일치: 예상 %q, 실제 %q", string(content), string(data))
	}
}

func TestExtractBinary_NotFound(t *testing.T) {
	t.Parallel()

	content := []byte("some-other-binary")
	archivePath := createTestTarGz(t, "not-the-binary-we-want", content)

	_, err := extractBinary(archivePath, "test.tar.gz")
	if err == nil {
		t.Fatal("바이너리를 찾을 수 없을 때 에러를 반환해야 합니다")
	}
}

func TestCompareVersions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		a, b     string
		expected int
	}{
		{"1.3.1", "1.3.0", 1},
		{"1.3.0", "1.3.0", 0},
		{"1.3.0", "1.3.1", -1},
		{"2.0.0", "1.9.9", 1},
		{"1.0.0", "0.9.0", 1},
	}

	for _, tc := range tests {
		result := compareVersions(tc.a, tc.b)
		if result != tc.expected {
			t.Errorf("compareVersions(%q, %q) = %d, 예상 %d", tc.a, tc.b, result, tc.expected)
		}
	}
}

func TestNormalizeVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"v1.3.0", "1.3.0"},
		{"1.3.0", "1.3.0"},
		{"  v1.0.0  ", "1.0.0"},
	}

	for _, tc := range tests {
		result := normalizeVersion(tc.input)
		if result != tc.expected {
			t.Errorf("normalizeVersion(%q) = %q, 예상 %q", tc.input, result, tc.expected)
		}
	}
}

func TestCopyToTempFile(t *testing.T) {
	t.Parallel()

	content := "test-content-12345"
	r := filepath.Join(t.TempDir(), "source.txt")
	if err := os.WriteFile(r, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(r)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	tmpPath, err := copyToTempFile(f)
	if err != nil {
		t.Fatalf("copyToTempFile 실패: %v", err)
	}
	defer os.Remove(tmpPath)

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != content {
		t.Errorf("내용 불일치: 예상 %q, 실제 %q", content, string(data))
	}
}
