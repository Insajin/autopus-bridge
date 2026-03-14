# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.10.0] - 2026-03-13

### Added

- `AgentConnectPayload.WorkspaceID` 필드를 추가하여 bridge 세션을 워크스페이스 스코프로 연결 가능
- `workspace_required` 인증 에러 코드를 추가하여 backend가 workspace-less bridge 연결을 명시적으로 거절할 수 있도록 지원

## [0.8.0] - 2026-03-06

### Added

- Agent Browser protocol types and message constants for `SPEC-BROWSER-AGENT-001`
- `BrowserActionPayload`, `BrowserResultPayload`, `BrowserSessionPayload`
- `browser_session_start`, `browser_action`, `browser_session_end`, `browser_session_ready`, `browser_result`, `browser_not_available`, `browser_error`

## [0.1.0] - 2026-02-19

Initial release of protocol types extracted from the Autopus monorepo.

### Added

- `AgentMessage` envelope type for all WebSocket communication
- Agent lifecycle types: `AgentConnectPayload`, `ConnectAckPayload`, `AgentDisconnectPayload`, `AgentHeartbeatPayload`
- Task execution types: `TaskRequestPayload`, `TaskProgressPayload`, `TaskResultPayload`, `TaskErrorPayload`, `TokenUsage`
- Build operation types: `BuildRequestPayload`, `BuildResultPayload`
- Test operation types: `TestRequestPayload`, `TestResultPayload`, `TestSummary`
- QA pipeline types: `QARequestPayload`, `QAResultPayload`, `QAStageResult`, `ServiceConfig`, `BrowserQAConfig`
- Computer use types: `ComputerActionPayload`, `ComputerResultPayload`, `ComputerSessionPayload`
- Worker orchestration types: `WorkerPhaseChangedPayload`
- MCP provisioning types: `MCPStartPayload`, `MCPReadyPayload`, `MCPStopPayload`, `MCPErrorPayload`
- Self-expansion types: `MCPCodegenRequestPayload`, `MCPCodegenProgressPayload`, `MCPCodegenResultPayload`, `MCPGeneratedFile`, `SecurityManifest`, `MCPDeployPayload`, `MCPDeployResultPayload`, `MCPHealthReportPayload`, `MCPServerHealth`
- Skill discovery types: `ProjectContextPayload`, `TechStack`, `CLIRequestPayload`, `CLIResultPayload`, `CLIParsedResult`, `CLITestFailure`
- 32 message type constants covering all protocol operations
- Computer use constraint constants

[0.1.0]: https://github.com/insajin/autopus-agent-protocol/releases/tag/v0.1.0
