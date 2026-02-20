# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

[1.1.0]: https://github.com/insajin/autopus-bridge/releases/tag/v1.1.0
[1.0.0]: https://github.com/insajin/autopus-bridge/releases/tag/v1.0.0
