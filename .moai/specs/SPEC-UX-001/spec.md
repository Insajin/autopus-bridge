# SPEC-UX-001: autopus-bridge 초보자 UX 개선

## 개요

autopus-bridge의 `autopus up` 실행 시 초보자가 겪는 주요 실패 지점들을 자동 복구하여,
별도의 수동 조치 없이 바로 서버에 연결할 수 있도록 개선한다.

## 문제 분석

### 사용자 로그에서 발견된 5가지 실패 지점

| # | 실패 지점 | 현재 동작 | 영향도 |
|---|----------|----------|--------|
| 1 | Step 12 서버 인증 실패 | 에러 출력 후 프로세스 종료 | **Critical** - 연결 불가 |
| 2 | connect.go 토큰 만료 | "lab login으로 다시 로그인하세요" 출력 | **Critical** - 초보자 막힘 |
| 3 | Docker 이미지 풀 실패 | "로컬 빌드가 필요할 수 있습니다" 출력 | **Medium** - 혼란 유발 |
| 4 | csvkit pip3 설치 실패 (PEP 668) | "exit status 1" 에러 | **Low** - 진행은 가능 |
| 5 | Codex CLI 인증 실패 | 진행은 되지만 연결 시 실패 가능 | **Medium** - 기능 제한 |

### 근본 원인

1. **Step 12 인증 실패 자동 복구 없음**: `up.go` Step 1-2에서는 `performBrowserAuthWithFallback()`으로 자동 재인증하지만, Step 12(`runConnect`)에서는 인증 실패 시 에러만 반환하고 종료
2. **connect.go의 "lab login" 안내**: 더 이상 존재하지 않는 `lab login` 명령 안내
3. **PEP 668 미대응**: macOS/최신 Linux에서 `pip3 install`이 시스템 Python 보호로 차단됨
4. **Docker 이미지 빌드 경로 없음**: Dockerfile 경로를 안내하지 않음
5. **에러 메시지가 개발자 중심**: 초보자가 이해하기 어려운 기술 용어

---

## 요구사항 (EARS Format)

### REQ-UX-001: Step 12 인증 실패 시 자동 재인증

**When** Step 12 서버 연결에서 인증이 실패하면,
**the system shall** 자동으로 토큰 갱신을 시도하고,
갱신도 실패하면 브라우저 인증 -> Device Code Flow 폴백을 실행하여
사용자 개입 없이 재인증 후 연결을 재시도한다.

**파일**: `cmd/up.go` (runUp 함수 Step 12 영역)

**변경 내용**:
```go
// 현재: Step 12에서 runConnect 에러 시 그냥 반환
return runConnect(cmd, nil)

// 변경: 인증 실패 감지 시 자동 재인증 후 재시도
err = runConnect(cmd, nil)
if err != nil && isAuthError(err) {
    fmt.Println("\n  서버 인증에 실패했습니다. 자동으로 재인증을 시도합니다...")
    newCreds, authErr := performBrowserAuthWithFallback()
    if authErr != nil {
        return fmt.Errorf("재인증 실패: %w", authErr)
    }
    // credentials 갱신 후 재시도
    return runConnect(cmd, nil)
}
return err
```

**isAuthError 헬퍼 함수 추가**:
```go
func isAuthError(err error) bool {
    msg := err.Error()
    patterns := []string{
        "인증 실패",
        "authentication failed",
        "서버 인증 거부",
        "토큰",
        "token",
        "unauthorized",
    }
    lower := strings.ToLower(msg)
    for _, p := range patterns {
        if strings.Contains(lower, p) {
            return true
        }
    }
    return false
}
```

### REQ-UX-002: connect.go 토큰 만료 시 자동 재인증

**When** connect.go에서 저장된 토큰이 만료되고 갱신도 실패하면,
**the system shall** `performBrowserAuthWithFallback()`을 호출하여
자동으로 브라우저 인증을 시도한다.

**파일**: `cmd/connect.go` (runConnect 함수, 라인 86-114)

**변경 내용**:
```go
// 현재 (라인 100-101):
logger.Warn().Err(err).Msg("토큰 자동 갱신 실패. 'lab login'으로 다시 로그인하세요.")

// 변경: 자동 재인증 시도
logger.Warn().Err(err).Msg("토큰 자동 갱신 실패, 브라우저 재인증 시도")
fmt.Println("  토큰 갱신에 실패했습니다. 브라우저에서 재인증을 시작합니다...")
newCreds, authErr := performBrowserAuthWithFallback()
if authErr != nil {
    return fmt.Errorf("재인증 실패: %w", authErr)
}
authToken = newCreds.AccessToken
```

또한 라인 109의 "lab login" 참조를 올바른 명령으로 수정:
```go
// 현재:
logger.Warn().Msg("저장된 인증 정보가 만료되었습니다. 'lab login'으로 다시 로그인하세요.")

// 변경:
fmt.Println("  저장된 인증 정보가 만료되었습니다. 브라우저에서 재인증을 시작합니다...")
newCreds, authErr := performBrowserAuthWithFallback()
if authErr != nil {
    return fmt.Errorf("재인증 실패: %w", authErr)
}
authToken = newCreds.AccessToken
```

### REQ-UX-003: csvkit 설치 명령 PEP 668 대응

**When** csvkit 설치 명령을 macOS 또는 최신 Linux에서 실행하면,
**the system shall** `pip3 install` 대신 `pipx install`을 사용하고,
pipx가 없으면 먼저 pipx를 설치한다.

**파일**: `cmd/tools.go` (getBusinessToolManifest, 라인 108-115)

**변경 내용**:
```go
// 현재:
{
    Name:     "csvkit",
    Category: toolCategoryRecommended,
    Purpose:  "CSV 처리 (csvstat, csvsql)",
    InstallCmd: map[string]string{
        "darwin": "pip3 install csvkit",
        "linux":  "pip3 install csvkit",
    },
},

// 변경:
{
    Name:     "csvkit",
    Category: toolCategoryRecommended,
    Purpose:  "CSV 처리 (csvstat, csvsql)",
    InstallCmd: map[string]string{
        "darwin": "pipx install csvkit",
        "linux":  "pipx install csvkit",
    },
},
```

**추가**: `runInstallCommand` 또는 `stepInstallMissingTools`에서 pipx 미설치 시 자동 설치:
```go
// pipx가 필요한 명령인데 pipx가 없으면 먼저 설치
if strings.HasPrefix(installCmd, "pipx ") {
    if _, err := exec.LookPath("pipx"); err != nil {
        fmt.Println("  pipx가 설치되어 있지 않습니다. 먼저 설치합니다...")
        if brewErr := runInstallCommand("brew install pipx"); brewErr != nil {
            printError("pipx 설치 실패. csvkit을 건너뜁니다.")
            continue
        }
        // pipx ensurepath
        _ = runInstallCommand("pipx ensurepath")
        printSuccess("pipx 설치 완료")
    }
}
```

### REQ-UX-004: Docker 이미지 풀 실패 시 안내 개선

**When** Chromium Sandbox Docker 이미지 풀이 실패하면,
**the system shall** 이미지가 선택적(non-blocking)이라는 점을 명확히 안내하고,
Computer Use 기능이 필요할 때만 이미지가 필요하다는 맥락을 제공한다.

**파일**: `cmd/up.go` (stepChromiumSandboxImage, 라인 625-628)

**변경 내용**:
```go
// 현재:
fmt.Println("  ! 이미지 풀 실패 (로컬 빌드가 필요할 수 있습니다)")
fmt.Printf("    docker build -t %s .\n", imageName)

// 변경:
fmt.Println("  ! 이미지 풀 실패 - Computer Use 기능을 사용하지 않으면 무시해도 됩니다")
fmt.Println("    Computer Use가 필요한 경우:")
fmt.Println("    docker pull autopus/chromium-sandbox:latest (Docker Hub 로그인 필요)")
fmt.Println("    또는 Dockerfile이 있는 디렉토리에서: docker build -t autopus/chromium-sandbox:latest .")
```

### REQ-UX-005: 에러 메시지 초보자 친화 개선

**When** 각 단계에서 에러가 발생하면,
**the system shall** 기술 용어 대신 행동 중심의 안내 메시지를 표시하고,
"무엇이 잘못됐는지"와 "어떻게 해결하는지"를 분리하여 보여준다.

**파일**: `cmd/up.go` (printFixSuggestion 함수)

**변경 내용**: `printFixSuggestion`에 "connection" 케이스 추가:
```go
case "connection":
    fmt.Println("    서버 연결에 실패했습니다.")
    fmt.Println()
    fmt.Println("    다음을 확인해 주세요:")
    fmt.Println("    1. 인터넷에 연결되어 있는지 확인하세요")
    fmt.Println("    2. 'autopus up --force'로 처음부터 다시 시도하세요")
    fmt.Println("    3. 문제가 지속되면 https://docs.autopus.co/troubleshooting 을 참고하세요")
```

---

## 수정 파일 목록

| 파일 | 변경 유형 | 설명 |
|------|----------|------|
| `cmd/up.go` | 수정 | Step 12 인증 실패 자동 복구, Docker 안내 개선 |
| `cmd/connect.go` | 수정 | 토큰 만료 시 자동 재인증, "lab login" 참조 수정 |
| `cmd/tools.go` | 수정 | csvkit 설치를 pipx로 변경 |

## 수락 기준

- [ ] `autopus up` 실행 시 토큰 만료 상태에서도 자동 재인증으로 서버 연결 성공
- [ ] connect 명령에서 "lab login" 메시지가 더 이상 표시되지 않음
- [ ] csvkit이 `pipx install csvkit`으로 설치됨 (PEP 668 에러 없음)
- [ ] Docker 이미지 풀 실패 시 초보자가 이해할 수 있는 안내 메시지 표시
- [ ] 기존 정상 동작(토큰 유효, 모든 도구 설치됨)에 영향 없음

## 리스크

- `performBrowserAuthWithFallback()`이 `connect.go`에서 호출되려면 up.go에 정의된 함수를 같은 패키지에서 접근 가능해야 함 (같은 `cmd` 패키지이므로 문제 없음)
- 재인증 루프 방지: 최대 1회만 재인증 시도하도록 제한

---

Version: 1.0.0
Created: 2026-02-25
