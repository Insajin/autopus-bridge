package agentbrowser

import (
	"testing"
	"time"
)

// mockLookupEnv는 테스트용 환경 변수 조회 함수를 생성한다.
func mockLookupEnv(envs map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		val, ok := envs[key]
		return val, ok
	}
}

// withMockEnv는 lookupEnvFn을 모킹하고 테스트 후 원래 함수를 복원한다.
func withMockEnv(t *testing.T, envs map[string]string) {
	t.Helper()
	original := lookupEnvFn
	lookupEnvFn = mockLookupEnv(envs)
	t.Cleanup(func() {
		lookupEnvFn = original
	})
}

// --- IsCICD 테스트 ---

func TestIsCICD_NoEnvVars(t *testing.T) {
	withMockEnv(t, map[string]string{})

	if IsCICD() {
		t.Error("IsCICD() = true; want false when no CI env vars are set")
	}
}

func TestIsCICD_CI(t *testing.T) {
	withMockEnv(t, map[string]string{"CI": "true"})

	if !IsCICD() {
		t.Error("IsCICD() = false; want true when CI=true")
	}
}

func TestIsCICD_GitHubActions(t *testing.T) {
	withMockEnv(t, map[string]string{"GITHUB_ACTIONS": "true"})

	if !IsCICD() {
		t.Error("IsCICD() = false; want true when GITHUB_ACTIONS=true")
	}
}

func TestIsCICD_GitLabCI(t *testing.T) {
	withMockEnv(t, map[string]string{"GITLAB_CI": "true"})

	if !IsCICD() {
		t.Error("IsCICD() = false; want true when GITLAB_CI=true")
	}
}

func TestIsCICD_JenkinsURL(t *testing.T) {
	withMockEnv(t, map[string]string{"JENKINS_URL": "http://jenkins.example.com"})

	if !IsCICD() {
		t.Error("IsCICD() = false; want true when JENKINS_URL is set")
	}
}

func TestIsCICD_CircleCI(t *testing.T) {
	withMockEnv(t, map[string]string{"CIRCLECI": "true"})

	if !IsCICD() {
		t.Error("IsCICD() = false; want true when CIRCLECI=true")
	}
}

func TestIsCICD_Travis(t *testing.T) {
	withMockEnv(t, map[string]string{"TRAVIS": "true"})

	if !IsCICD() {
		t.Error("IsCICD() = false; want true when TRAVIS=true")
	}
}

func TestIsCICD_Buildkite(t *testing.T) {
	withMockEnv(t, map[string]string{"BUILDKITE": "true"})

	if !IsCICD() {
		t.Error("IsCICD() = false; want true when BUILDKITE=true")
	}
}

func TestIsCICD_CodeBuild(t *testing.T) {
	withMockEnv(t, map[string]string{"CODEBUILD_BUILD_ID": "build-123"})

	if !IsCICD() {
		t.Error("IsCICD() = false; want true when CODEBUILD_BUILD_ID is set")
	}
}

func TestIsCICD_EmptyValue(t *testing.T) {
	withMockEnv(t, map[string]string{"CI": ""})

	if IsCICD() {
		t.Error("IsCICD() = true; want false when CI is empty string")
	}
}

func TestIsCICD_MultipleVars(t *testing.T) {
	withMockEnv(t, map[string]string{
		"CI":             "true",
		"GITHUB_ACTIONS": "true",
	})

	if !IsCICD() {
		t.Error("IsCICD() = false; want true when multiple CI vars set")
	}
}

// --- DetectCICDEnvironment 테스트 ---

func TestDetectCICDEnvironment_CICD(t *testing.T) {
	withMockEnv(t, map[string]string{"CI": "true"})

	config := DetectCICDEnvironment()

	if !config.Headless {
		t.Error("config.Headless = false; want true in CI/CD")
	}
	if !config.JSONOutput {
		t.Error("config.JSONOutput = false; want true in CI/CD")
	}
	if !config.NoColor {
		t.Error("config.NoColor = false; want true in CI/CD")
	}
	if config.Timeout != DefaultCICDTimeout {
		t.Errorf("config.Timeout = %v; want %v", config.Timeout, DefaultCICDTimeout)
	}
}

func TestDetectCICDEnvironment_NotCICD(t *testing.T) {
	withMockEnv(t, map[string]string{})

	config := DetectCICDEnvironment()

	if config.Headless {
		t.Error("config.Headless = true; want false when not in CI/CD")
	}
	if config.JSONOutput {
		t.Error("config.JSONOutput = true; want false when not in CI/CD")
	}
	if config.NoColor {
		t.Error("config.NoColor = true; want false when not in CI/CD")
	}
	if config.Timeout != 0 {
		t.Errorf("config.Timeout = %v; want 0", config.Timeout)
	}
}

func TestDetectCICDEnvironment_GitHubActions(t *testing.T) {
	withMockEnv(t, map[string]string{"GITHUB_ACTIONS": "true"})

	config := DetectCICDEnvironment()

	if !config.IsEnabled() {
		t.Error("config.IsEnabled() = false; want true for GitHub Actions")
	}
	if config.Timeout != 5*time.Minute {
		t.Errorf("config.Timeout = %v; want 5m", config.Timeout)
	}
}

// --- DetectedCICDProvider 테스트 ---

func TestDetectedCICDProvider_GitHubActions(t *testing.T) {
	withMockEnv(t, map[string]string{"GITHUB_ACTIONS": "true"})

	provider := DetectedCICDProvider()
	if provider != "github-actions" {
		t.Errorf("DetectedCICDProvider() = %q; want %q", provider, "github-actions")
	}
}

func TestDetectedCICDProvider_GitLabCI(t *testing.T) {
	withMockEnv(t, map[string]string{"GITLAB_CI": "true"})

	provider := DetectedCICDProvider()
	if provider != "gitlab-ci" {
		t.Errorf("DetectedCICDProvider() = %q; want %q", provider, "gitlab-ci")
	}
}

func TestDetectedCICDProvider_Jenkins(t *testing.T) {
	withMockEnv(t, map[string]string{"JENKINS_URL": "http://jenkins.local"})

	provider := DetectedCICDProvider()
	if provider != "jenkins" {
		t.Errorf("DetectedCICDProvider() = %q; want %q", provider, "jenkins")
	}
}

func TestDetectedCICDProvider_CircleCI(t *testing.T) {
	withMockEnv(t, map[string]string{"CIRCLECI": "true"})

	provider := DetectedCICDProvider()
	if provider != "circleci" {
		t.Errorf("DetectedCICDProvider() = %q; want %q", provider, "circleci")
	}
}

func TestDetectedCICDProvider_TravisCI(t *testing.T) {
	withMockEnv(t, map[string]string{"TRAVIS": "true"})

	provider := DetectedCICDProvider()
	if provider != "travis-ci" {
		t.Errorf("DetectedCICDProvider() = %q; want %q", provider, "travis-ci")
	}
}

func TestDetectedCICDProvider_Buildkite(t *testing.T) {
	withMockEnv(t, map[string]string{"BUILDKITE": "true"})

	provider := DetectedCICDProvider()
	if provider != "buildkite" {
		t.Errorf("DetectedCICDProvider() = %q; want %q", provider, "buildkite")
	}
}

func TestDetectedCICDProvider_AWSCodeBuild(t *testing.T) {
	withMockEnv(t, map[string]string{"CODEBUILD_BUILD_ID": "abc-123"})

	provider := DetectedCICDProvider()
	if provider != "aws-codebuild" {
		t.Errorf("DetectedCICDProvider() = %q; want %q", provider, "aws-codebuild")
	}
}

func TestDetectedCICDProvider_GenericCI(t *testing.T) {
	withMockEnv(t, map[string]string{"CI": "true"})

	provider := DetectedCICDProvider()
	if provider != "generic-ci" {
		t.Errorf("DetectedCICDProvider() = %q; want %q", provider, "generic-ci")
	}
}

func TestDetectedCICDProvider_NoneDetected(t *testing.T) {
	withMockEnv(t, map[string]string{})

	provider := DetectedCICDProvider()
	if provider != "" {
		t.Errorf("DetectedCICDProvider() = %q; want empty string", provider)
	}
}

// --- CICDConfig.IsEnabled 테스트 ---

func TestCICDConfig_IsEnabled_AllFalse(t *testing.T) {
	config := CICDConfig{}
	if config.IsEnabled() {
		t.Error("IsEnabled() = true; want false for zero-value config")
	}
}

func TestCICDConfig_IsEnabled_HeadlessOnly(t *testing.T) {
	config := CICDConfig{Headless: true}
	if !config.IsEnabled() {
		t.Error("IsEnabled() = false; want true when Headless is true")
	}
}

func TestCICDConfig_IsEnabled_JSONOutputOnly(t *testing.T) {
	config := CICDConfig{JSONOutput: true}
	if !config.IsEnabled() {
		t.Error("IsEnabled() = false; want true when JSONOutput is true")
	}
}

func TestCICDConfig_IsEnabled_NoColorOnly(t *testing.T) {
	config := CICDConfig{NoColor: true}
	if !config.IsEnabled() {
		t.Error("IsEnabled() = false; want true when NoColor is true")
	}
}

func TestCICDConfig_IsEnabled_TimeoutOnly(t *testing.T) {
	// Timeout만 설정된 경우는 IsEnabled가 false이다.
	// Headless/JSONOutput/NoColor 중 하나가 필요하다.
	config := CICDConfig{Timeout: 5 * time.Minute}
	if config.IsEnabled() {
		t.Error("IsEnabled() = true; want false when only Timeout is set")
	}
}

func TestCICDConfig_IsEnabled_AllTrue(t *testing.T) {
	config := CICDConfig{
		Headless:   true,
		JSONOutput: true,
		NoColor:    true,
		Timeout:    5 * time.Minute,
	}
	if !config.IsEnabled() {
		t.Error("IsEnabled() = false; want true when all flags are set")
	}
}
