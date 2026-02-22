package computeruse

import (
	"reflect"
	"testing"
)

// --- 컴파일 타임 인터페이스 준수 확인 ---

// CLIDockerClient가 DockerClient 인터페이스를 구현하는지 컴파일 타임에 검증한다.
var _ DockerClient = (*CLIDockerClient)(nil)

// --- NewCLIDockerClient 생성자 테스트 ---

func TestNewCLIDockerClient_Default(t *testing.T) {
	client := NewCLIDockerClient("")
	if client == nil {
		t.Fatal("NewCLIDockerClient('') returned nil")
	}
	if client.dockerPath != "docker" {
		t.Errorf("dockerPath = %q; want %q", client.dockerPath, "docker")
	}
}

func TestNewCLIDockerClient_CustomPath(t *testing.T) {
	client := NewCLIDockerClient("/usr/local/bin/docker")
	if client == nil {
		t.Fatal("NewCLIDockerClient('/usr/local/bin/docker') returned nil")
	}
	if client.dockerPath != "/usr/local/bin/docker" {
		t.Errorf("dockerPath = %q; want %q", client.dockerPath, "/usr/local/bin/docker")
	}
}

// --- buildCreateArgs 인자 생성 테스트 ---

func TestBuildCreateArgs_FullConfig(t *testing.T) {
	config := &ContainerCreateConfig{
		Image:       "autopus/chromium-sandbox:latest",
		Network:     "autopus-sandbox-net",
		MemoryLimit: 512 * 1024 * 1024,
		CPUQuota:    100000,
		PIDLimit:    100,
		TmpfsSize:   "64m",
		ReadOnly:    true,
		User:        "sandbox",
	}

	args := buildCreateArgs(config)

	// 전체 인자 목록 검증
	expected := []string{
		"create",
		"--memory", "536870912",
		"--cpus", "1.0",
		"--pids-limit", "100",
		"--read-only",
		"--user", "sandbox",
		"--tmpfs", "/tmp:size=64m",
		"--tmpfs", "/dev/shm:size=64m",
		"--network", "autopus-sandbox-net",
		"-p", "0:9222",
		"autopus/chromium-sandbox:latest",
	}

	if !reflect.DeepEqual(args, expected) {
		t.Errorf("buildCreateArgs() =\n  %v\nwant:\n  %v", args, expected)
	}
}

func TestBuildCreateArgs_MinimalConfig(t *testing.T) {
	// 최소 설정: 이미지만 지정
	config := &ContainerCreateConfig{
		Image: "alpine:latest",
	}

	args := buildCreateArgs(config)

	expected := []string{
		"create",
		"-p", "0:9222",
		"alpine:latest",
	}

	if !reflect.DeepEqual(args, expected) {
		t.Errorf("buildCreateArgs() =\n  %v\nwant:\n  %v", args, expected)
	}
}

func TestBuildCreateArgs_ReadOnlyFalse(t *testing.T) {
	config := &ContainerCreateConfig{
		Image:    "alpine:latest",
		ReadOnly: false,
		User:     "root",
	}

	args := buildCreateArgs(config)

	// --read-only가 포함되지 않아야 한다
	for _, arg := range args {
		if arg == "--read-only" {
			t.Error("buildCreateArgs() 에 --read-only가 포함됨; ReadOnly=false일 때 포함되면 안됨")
		}
	}

	// --user root는 포함되어야 한다
	found := false
	for i, arg := range args {
		if arg == "--user" && i+1 < len(args) && args[i+1] == "root" {
			found = true
			break
		}
	}
	if !found {
		t.Error("buildCreateArgs() 에 --user root가 포함되지 않음")
	}
}

func TestBuildCreateArgs_HalfCPU(t *testing.T) {
	config := &ContainerCreateConfig{
		Image:    "alpine:latest",
		CPUQuota: 50000, // 0.5 CPU
	}

	args := buildCreateArgs(config)

	// --cpus 0.5가 포함되어야 한다
	found := false
	for i, arg := range args {
		if arg == "--cpus" && i+1 < len(args) && args[i+1] == "0.5" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("buildCreateArgs() 에 --cpus 0.5가 포함되지 않음; args=%v", args)
	}
}

func TestBuildCreateArgs_ImageAlwaysLast(t *testing.T) {
	config := &ContainerCreateConfig{
		Image:       "my-image:v2",
		Network:     "net1",
		MemoryLimit: 1024,
		CPUQuota:    200000,
		PIDLimit:    50,
		TmpfsSize:   "128m",
		ReadOnly:    true,
		User:        "app",
	}

	args := buildCreateArgs(config)

	// 이미지는 항상 마지막 인자여야 한다
	lastArg := args[len(args)-1]
	if lastArg != "my-image:v2" {
		t.Errorf("마지막 인자 = %q; want %q (이미지는 항상 마지막이어야 함)", lastArg, "my-image:v2")
	}
}

// --- parseInspectOutput 파싱 테스트 ---

func TestParseInspectOutput_Valid(t *testing.T) {
	output := "abc123def456789|running|49152"

	result, err := parseInspectOutput(output)
	if err != nil {
		t.Fatalf("parseInspectOutput() error = %v; want nil", err)
	}
	if result.ID != "abc123def456789" {
		t.Errorf("ID = %q; want %q", result.ID, "abc123def456789")
	}
	if result.Status != "running" {
		t.Errorf("Status = %q; want %q", result.Status, "running")
	}
	if result.HostPort != "49152" {
		t.Errorf("HostPort = %q; want %q", result.HostPort, "49152")
	}
}

func TestParseInspectOutput_ExitedStatus(t *testing.T) {
	output := "deadbeef12345678|exited|8080"

	result, err := parseInspectOutput(output)
	if err != nil {
		t.Fatalf("parseInspectOutput() error = %v; want nil", err)
	}
	if result.Status != "exited" {
		t.Errorf("Status = %q; want %q", result.Status, "exited")
	}
}

func TestParseInspectOutput_InvalidFormat_NoPipe(t *testing.T) {
	output := "abc123running49152"

	_, err := parseInspectOutput(output)
	if err == nil {
		t.Error("parseInspectOutput() with invalid format = nil error; want error")
	}
}

func TestParseInspectOutput_InvalidFormat_OnePipe(t *testing.T) {
	output := "abc123|running"

	_, err := parseInspectOutput(output)
	if err == nil {
		t.Error("parseInspectOutput() with only one pipe = nil error; want error")
	}
}

func TestParseInspectOutput_EmptyFields(t *testing.T) {
	// 빈 필드도 3개 파이프 구분이면 파싱 성공
	output := "||"

	result, err := parseInspectOutput(output)
	if err != nil {
		t.Fatalf("parseInspectOutput() error = %v; want nil", err)
	}
	if result.ID != "" {
		t.Errorf("ID = %q; want empty", result.ID)
	}
	if result.Status != "" {
		t.Errorf("Status = %q; want empty", result.Status)
	}
	if result.HostPort != "" {
		t.Errorf("HostPort = %q; want empty", result.HostPort)
	}
}

func TestParseInspectOutput_PortWithPipeInValue(t *testing.T) {
	// HostPort 필드에 파이프가 포함된 경우 (SplitN이므로 3번째 필드에 포함)
	output := "abc123|running|port|extra"

	result, err := parseInspectOutput(output)
	if err != nil {
		t.Fatalf("parseInspectOutput() error = %v; want nil", err)
	}
	// SplitN(3)이므로 세 번째 필드가 "port|extra"가 된다
	if result.HostPort != "port|extra" {
		t.Errorf("HostPort = %q; want %q", result.HostPort, "port|extra")
	}
}

// --- Close no-op 테스트 ---

func TestCLIDockerClient_Close(t *testing.T) {
	client := NewCLIDockerClient("")
	err := client.Close()
	if err != nil {
		t.Errorf("Close() = error %v; want nil", err)
	}
}
