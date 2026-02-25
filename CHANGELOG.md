# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

[1.5.0]: https://github.com/insajin/autopus-bridge/releases/tag/v1.5.0
[1.4.0]: https://github.com/insajin/autopus-bridge/releases/tag/v1.4.0
[1.3.1]: https://github.com/insajin/autopus-bridge/releases/tag/v1.3.1
[1.3.0]: https://github.com/insajin/autopus-bridge/releases/tag/v1.3.0
[1.1.0]: https://github.com/insajin/autopus-bridge/releases/tag/v1.1.0
[1.0.0]: https://github.com/insajin/autopus-bridge/releases/tag/v1.0.0
