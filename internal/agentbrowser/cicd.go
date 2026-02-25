package agentbrowser

import "os"

// cicdEnvVars는 CI/CD 환경을 나타내는 환경 변수 목록이다.
var cicdEnvVars = []string{
	"CI",
	"GITHUB_ACTIONS",
	"GITLAB_CI",
	"JENKINS_URL",
	"CIRCLECI",
	"TRAVIS",
	"BUILDKITE",
	"CODEBUILD_BUILD_ID",
}

// lookupEnvFn은 환경 변수를 조회하는 함수이다 (테스트용 주입 가능).
var lookupEnvFn = os.LookupEnv

// DetectCICDEnvironment는 현재 실행 환경이 CI/CD인지 감지하고 적절한 설정을 반환한다.
// CI/CD 환경이 감지되면 Headless, JSONOutput, NoColor를 모두 활성화하고
// 기본 타임아웃(5분)을 설정한다.
// CI/CD 환경이 아닌 경우 제로값 CICDConfig를 반환한다.
func DetectCICDEnvironment() CICDConfig {
	if !IsCICD() {
		return CICDConfig{}
	}

	return CICDConfig{
		Headless:   true,
		JSONOutput: true,
		NoColor:    true,
		Timeout:    DefaultCICDTimeout,
	}
}

// IsCICD는 현재 실행 환경이 CI/CD인지 간단히 확인한다.
// CI, GITHUB_ACTIONS, GITLAB_CI, JENKINS_URL, CIRCLECI, TRAVIS,
// BUILDKITE, CODEBUILD_BUILD_ID 환경 변수 중 하나라도 설정되어 있으면 true를 반환한다.
func IsCICD() bool {
	for _, envVar := range cicdEnvVars {
		if val, ok := lookupEnvFn(envVar); ok && val != "" {
			return true
		}
	}
	return false
}

// DetectedCICDProvider는 감지된 CI/CD 제공자 이름을 반환한다.
// 감지되지 않으면 빈 문자열을 반환한다.
func DetectedCICDProvider() string {
	providerMap := map[string]string{
		"GITHUB_ACTIONS":    "github-actions",
		"GITLAB_CI":         "gitlab-ci",
		"JENKINS_URL":       "jenkins",
		"CIRCLECI":          "circleci",
		"TRAVIS":            "travis-ci",
		"BUILDKITE":         "buildkite",
		"CODEBUILD_BUILD_ID": "aws-codebuild",
	}

	for envVar, provider := range providerMap {
		if val, ok := lookupEnvFn(envVar); ok && val != "" {
			return provider
		}
	}

	// 일반적인 CI 환경 변수 확인
	if val, ok := lookupEnvFn("CI"); ok && val != "" {
		return "generic-ci"
	}

	return ""
}
