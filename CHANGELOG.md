# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `update` 명령이 실행 중인 `connect` 프로세스를 감지하면 정상 종료 후 새 바이너리로 `connect`를 자동 재시작

## [1.13.0] - 2026-03-06

### Changed

- Agent Browser 프로토콜 타입과 메시지 상수를 `autopus-agent-protocol` 공용 SDK에서 직접 사용하도록 정리
- `autopus-agent-protocol` 의존 버전을 `v0.8.0`으로 갱신

## [1.9.0] - 2026-03-04

### Added

- SPEC-BRIDGE-GATEWAY-001: Agent Response Protocol 지원
  - `agent_response_request` 메시지 수신 및 `agent_response_stream/complete/error` 응답
  - WebSocket 클라이언트에 `SendAgentResponseStream`, `SendAgentResponseComplete`, `SendAgentResponseError` 메서드 추가
  - 라우터에 `handleAgentResponseRequest`, `executeAgentResponse` 핸들러 등록
- SPEC-BRIDGE-GATEWAY-001: Provider Capabilities 전송
  - `WithProviderCapabilities` 옵션으로 `agent_connect` 메시지에 프로바이더 capabilities 포함
  - ValidateConfig() 통과한 프로바이더만 capabilities에 포함하여 잘못된 라우팅 방지
- 프로바이더 `enabled` 설정 필드 추가 (`config.yaml`에서 프로바이더별 활성화/비활성화)
- 정규 이름 매핑 유틸리티: `ToCanonicalName()`, `ToInternalName()` (claude↔anthropic, codex↔openai, gemini↔google)
- `GetAvailableProviders()`가 백엔드 정규 이름(anthropic, google, openai)으로 반환
- `GetForTask()`에서 OpenRouter 정규 이름→내부 이름 변환 폴백
- `ExecuteRequest`에 `SystemPrompt` 필드 추가
- `TaskError.ErrorCode()` 메서드 추가 (websocket 에러 코드 전파)
- Codex `~/.codex/auth.json` 자동 감지 (`readCodexAuthFile`)
- `Registry.GetRegisteredProviderNames()` 메서드 추가

### Fixed

- `isProcessRunning`에서 `os.Signal(nil)` → `syscall.Signal(0)`으로 수정
- `ErrorCodeProviderNotFound` 값을 백엔드 호환 `"provider_not_found"`로 변경
- API 키 미설정 에러를 `PROVIDER_ERROR` 대신 `provider_not_found`로 분류
- 미등록 메시지 타입 수신 시 로그 추가
- `agent_response_request` 플랫 JSON 수신 시 raw payload 보존

### Changed

- Codex App Server 기본 approval policy를 `"auto-approve"` → `"never"`로 변경
- Codex App Server approval policy 매핑: auto-approve→never, deny-all→reject
- `InitializeParams` 구조체를 `ClientInfo` 중첩 구조로 변경
- `AccountLoginParams.Method` → `AccountLoginParams.Type`으로 필드명 변경
- `thread/start` 결과 파싱: 중첩 구조 `{"thread":{"id":"..."}}` 우선 시도 후 플랫 폴백
- Enabled=false인 프로바이더는 Registry 등록 및 Validate()에서 건너뜀
- `InitializeRegistry` → `InitializeRegistryWithLogger`로 전환 (로거 주입)

## [1.8.0] - 2026-03-03

### Removed

- Plugin Mode (`mcp-serve`) 완전 제거 (SPEC-LEGACY-CLI-001)
  - `cmd/mcp-serve.go` 서브커맨드 삭제
  - WebSocket 핸들러에서 MCPServe 관련 함수 5개 및 Router 필드 5개 제거
  - MCP 서버 커맨드를 `autopus-mcp-server` standalone 바이너리로 전환
  - MCPServe 핸들러 단위 테스트 및 통합 테스트 삭제

### Fixed

- 멀티프로바이더 실행 안정화 및 플랫폼 연동 버그 수정
  - `claude_cli`: CLAUDECODE/CLAUDE_CODE_ENTRYPOINT 환경변수 필터링 (세션 충돌 방지)
  - `codex_cli`: `exec --json` JSONL 파싱 전환 및 `--skip-git-repo-check` 추가
  - `docker_client_cli`: inspect nil-safe 템플릿 + 포트 매핑 재시도 로직
  - `connect`: 프로바이더별 CLI 경로 기본값 분리 (gemini/codex 바이너리명 구분)
  - `executor/task`: OpenRouter 접두사 제거 및 `TaskError.IsRetryable` 구현
  - `websocket/handler`: 핸들러 테스트 추가
- Codex 인증 커맨드 `codex auth` -> `codex login` 수정
- OpenRouter 형식 모델 라우팅 지원 (`openai/o3-mini`, `anthropic/claude-sonnet-4-6`)

### Changed

- Device Auth 백엔드 래핑 응답 파싱 개선

## [1.7.0] - 2026-02-26

### Added

- Agent Browser 웹 자동화 핸들러 통합 (SPEC-BROWSER-AGENT-001)
  - Playwright 기반 브라우저 세션 관리 (`internal/agentbrowser/`)
  - 브라우저 액션 실행, 헬스체크, CI/CD 파이프라인 지원
  - WebSocket 라우터에 Agent Browser 핸들러 등록
  - 세션 재연결 시 Agent Browser 상태 복원 지원
- Device Auth Deprecation 메트릭 및 Bridge 버전 추적 (SPEC-BRIDGE-DEVAUTH-001 M4)
- Device Auth 핸들러 통합 테스트 (SPEC-BRIDGE-DEVAUTH-001 M5)

### Fixed

- Computer Use 워밍 풀 보충 무한 재시도 방지 및 Dockerfile 탐색 개선
- JWT exp 불일치 수정 (refresh.go, login.go, token_refresher.go)

## [1.5.0] - 2026-02-26

### Changed

- CLI 인증 시스템 PKCE + 워크스페이스 통합으로 전면 개선 (SPEC-BRIDGE-DEVAUTH-001)
  - Device Auth Flow에 PKCE S256 challenge 추가 (RFC 7636)
  - 토큰 응답에 사용자 워크스페이스 목록 포함
  - `login.go` 627→335줄 (47% 감소) 코드 대폭 단순화
  - 기존 `/auth/cli-token` 방식 deprecation 준비 완료

### Added

- CLI 정적 분석 테스트 + credentials 워크스페이스 테스트
  - login 함수 정적 분석 기반 테스트 추가
  - credentials.json 워크스페이스 데이터 검증 테스트 추가

## [1.4.0] - 2026-02-25

### Removed

- Claude Code Autopus 플러그인 완전 제거 (경로 A Dispatch Mode + MCP Tool Injection만 사용)
  - `IsPluginInstalled()`, `InstallPlugin()`, `InstallPluginTo()`, `PluginVersion()` 함수 삭제
  - 임베디드 플러그인 파일 삭제 (`.claude-plugin/`, `agents/`, `commands/`, `hooks/`)
  - `up` 명령의 플러그인 설치 블록 제거, `setup` 명령의 플러그인 상태 표시를 MCP 상태로 변경
  - 플러그인 관련 테스트 12개 삭제

### Changed

- 임베디드 디렉토리 `plugin-dist/` -> `skill-dist/`로 리네임 (Agent Skill 전용)
- 임베디드 변수 `pluginFiles` -> `skillFiles`로 리네임

### Added

- `tmux` 터미널 멀티플렉서를 비즈니스 도구 매니페스트에 추가 (developer 카테고리)

## [1.3.1] - 2026-02-25

### Fixed

- Self-update (`autopus update`) 실행 후 바이너리가 실행되지 않는 치명적 버그 수정
  - `downloadAsset()`이 tar.gz/zip 아카이브를 추출하지 않고 그대로 바이너리로 교체하던 문제
  - `extractBinary()` 함수 추가: tar.gz/zip 아카이브에서 바이너리 자동 추출
  - tar.gz (`archive/tar` + `compress/gzip`) 및 zip (`archive/zip`) 포맷 모두 지원
  - GoReleaser 중첩 경로 (`autopus-bridge_1.3.0_darwin_arm64/autopus-bridge`) 정상 처리
- Updater 패키지 테스트 추가 (tar.gz, zip, 중첩 경로, 에러 케이스)

## [1.3.0] - 2026-02-25

### Added

- Provider-Agnostic Approval Relay system (SPEC-INTERACTIVE-CLI-001)
  - `ApprovalRelay` interface + `ApprovalRouter` for unified policy routing
  - `HookHandler`: Claude Code PreToolUse/PostToolUse HTTP hook handler
  - `HookServer`: Bridge-local HTTP server for hook callbacks
  - `ScriptGenerator`: Per-session hook script generation
  - `ApprovalManager`: In-memory pending approval tracking with timeout
  - `RPCRelay`: Codex JSON-RPC approval adapter (refactored from hardcoded logic)
  - `StdinRelay`: Generic stdin/stdout approval adapter for future CLI tools
  - `MCPInjector`: MCP Tool Injection for Dispatch Mode sessions
  - `ClaudeInteractiveProvider`: PTY-based interactive Claude Code execution
  - HMAC signing for `tool_approval_request`/`tool_approval_response` messages

### Deprecated

- Plugin Mode (`mcp-serve`): stderr warning added, use Dispatch Mode with MCP Tool Injection instead

## [1.1.0] - 2026-02-20

### Added

- Codex App Server mode (`app-server`): Real-time streaming via JSON-RPC 2.0 over stdio
  - `CodexAppServerProvider` with `Execute()` and `ExecuteStreaming()` methods
  - Thread/Turn-based session management with automatic process lifecycle
  - `StreamAccumulator` integration for WebSocket streaming
  - Dual authentication support (API Key + ChatGPT token)
  - Automatic approval handling (`auto-approve` / `deny-all` policies)
  - Process auto-restart on abnormal termination (max 3 retries)
- JSON-RPC 2.0 client with concurrent-safe request/response matching
- Codex protocol types for App Server domain (Thread, Turn, Item events)

### Fixed

- Race condition in QA executor health check polling (atomic counter)
- Mutex copy in MCP health monitor (index-based iteration)
- TUI quit message updated to "Autopus Local Bridge Dashboard closed"

### Changed

- MCP process management: Platform-specific signal handling (Unix/Windows split)

## [1.0.0] - 2026-02-19

Initial standalone release, migrated from the Autopus monorepo.

Previously located at `github.com/anthropics/acos/cmd/local-agent-bridge`.

### Added

- WebSocket client with automatic reconnection and exponential backoff
- Task execution engine with streaming progress reports
- Build and test execution with result parsing (JSON, TAP, JUnit XML)
- QA pipeline orchestration with browser-based testing support
- Computer use integration for headless browser automation
- MCP server provisioning, lifecycle management, and health monitoring
- MCP code generation and sandboxed deployment
- Project context detection and technology stack analysis
- Device Authorization Flow (RFC 8628) for authentication
- Interactive TUI dashboard for real-time monitoring
- Multi-provider support (Claude, Gemini, Codex) with CLI and API modes
- Filesystem sandboxing with configurable allow/deny paths
- HMAC-SHA256 message signing for secure communication
- Self-update via GitHub Releases with SHA256 checksum verification
- Unified `up` command combining login, setup, and connect
- Configuration management with environment variable overrides (LAB_ prefix)

### Migration

- Moved from `github.com/anthropics/acos/cmd/local-agent-bridge` to `github.com/insajin/autopus-bridge`
- Protocol types extracted to separate SDK: `github.com/insajin/autopus-agent-protocol`
- See [docs/MIGRATION.md](docs/MIGRATION.md) for upgrade instructions

[1.9.0]: https://github.com/insajin/autopus-bridge/releases/tag/v1.9.0
[1.8.0]: https://github.com/insajin/autopus-bridge/releases/tag/v1.8.0
[1.7.0]: https://github.com/insajin/autopus-bridge/releases/tag/v1.7.0
[1.5.0]: https://github.com/insajin/autopus-bridge/releases/tag/v1.5.0
[1.4.0]: https://github.com/insajin/autopus-bridge/releases/tag/v1.4.0
[1.3.1]: https://github.com/insajin/autopus-bridge/releases/tag/v1.3.1
[1.3.0]: https://github.com/insajin/autopus-bridge/releases/tag/v1.3.0
[1.1.0]: https://github.com/insajin/autopus-bridge/releases/tag/v1.1.0
[1.0.0]: https://github.com/insajin/autopus-bridge/releases/tag/v1.0.0
