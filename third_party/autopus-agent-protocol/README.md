# Autopus Agent Protocol

Go protocol SDK containing WebSocket message types for communication between the Autopus platform and local development agents.

This package provides pure type definitions with zero external dependencies, making it safe to import in any Go project.

## Installation

```bash
go get github.com/insajin/autopus-agent-protocol@latest
```

## Usage

```go
package main

import (
	"encoding/json"
	"time"

	ws "github.com/insajin/autopus-agent-protocol"
)

func main() {
	// Create a message envelope
	msg := ws.AgentMessage{
		Type:      ws.AgentMsgConnect,
		ID:        "msg-001",
		Timestamp: time.Now(),
	}

	// Create a connect payload
	payload := ws.AgentConnectPayload{
		Version:      "1.0.0",
		Capabilities: []string{"build", "test", "qa"},
		Token:        "your-jwt-token",
	}

	// Marshal payload into the message
	raw, _ := json.Marshal(payload)
	msg.Payload = raw

	// Send msg over your WebSocket connection...
}
```

## Type Categories

### Agent Messages

The core message envelope and connection lifecycle types.

| Type | Description |
|------|-------------|
| `AgentMessage` | Envelope for all WebSocket messages |
| `AgentConnectPayload` | Sent when an agent connects to the server |
| `ConnectAckPayload` | Server acknowledgement after authentication |
| `AgentDisconnectPayload` | Sent when an agent disconnects |
| `AgentHeartbeatPayload` | Exchanged for keepalive |
| `TaskRequestPayload` | Server-to-agent task execution request |
| `TaskProgressPayload` | Agent-to-server streaming progress updates |
| `TaskResultPayload` | Agent-to-server execution result |
| `TokenUsage` | Token consumption tracking |
| `TaskErrorPayload` | Sent when execution fails |
| `BuildRequestPayload` | Server-to-agent build request |
| `BuildResultPayload` | Agent-to-server build result |
| `TestRequestPayload` | Server-to-agent test execution request |
| `TestResultPayload` | Agent-to-server test result |
| `TestSummary` | Aggregated test result counts |
| `QARequestPayload` | Server-to-agent QA pipeline request |
| `ServiceConfig` | Service configuration for QA tests |
| `BrowserQAConfig` | Browser-based QA test parameters |
| `QAResultPayload` | Agent-to-server QA pipeline result |
| `QAStageResult` | Result of a single QA pipeline stage |
| `ComputerActionPayload` | Computer use action request |
| `ComputerResultPayload` | Computer use action result |
| `ComputerSessionPayload` | Computer use session start/end |
| `BrowserActionPayload` | Agent Browser action request |
| `BrowserResultPayload` | Agent Browser action result |
| `BrowserSessionPayload` | Agent Browser session lifecycle |
| `WorkerPhaseChangedPayload` | Worker phase transition event |

### MCP Payloads

Types for MCP (Model Context Protocol) server provisioning.

| Type | Description |
|------|-------------|
| `MCPStartPayload` | Server requests MCP server startup on bridge |
| `MCPReadyPayload` | Bridge reports MCP server is ready |
| `MCPStopPayload` | Server requests MCP server shutdown |
| `MCPErrorPayload` | Bridge reports MCP server error |

### Self-Expansion Payloads

Types for autonomous MCP code generation and deployment.

| Type | Description |
|------|-------------|
| `MCPCodegenRequestPayload` | Server-to-bridge code generation request |
| `SecurityManifest` | Allowed resources for generated MCP server |
| `MCPCodegenProgressPayload` | Code generation progress report |
| `MCPCodegenResultPayload` | Generated code result |
| `MCPGeneratedFile` | Single generated file |
| `MCPDeployPayload` | Server-to-bridge deployment request |
| `MCPDeployResultPayload` | Deployment result |
| `MCPHealthReportPayload` | Periodic MCP health report |
| `MCPServerHealth` | Health status of a single MCP server |

### Skill Payloads

Types for project context detection and CLI execution.

| Type | Description |
|------|-------------|
| `ProjectContextPayload` | Bridge-to-server project tech stack context |
| `TechStack` | Detected technology stack |
| `CLIRequestPayload` | Server-to-bridge CLI execution request |
| `CLIResultPayload` | Bridge-to-server CLI execution result |
| `CLIParsedResult` | Structured output from CLI parsing |
| `CLITestFailure` | Single test failure from CLI output |

## Message Type Constants

All message type string constants used in `AgentMessage.Type`:

| Constant | Value | Direction |
|----------|-------|-----------|
| `AgentMsgConnect` | `agent_connect` | Agent -> Server |
| `AgentMsgConnectAck` | `agent_connect_ack` | Server -> Agent |
| `AgentMsgDisconnect` | `agent_disconnect` | Agent -> Server |
| `AgentMsgHeartbeat` | `agent_heartbeat` | Bidirectional |
| `AgentMsgTaskReq` | `task_request` | Server -> Agent |
| `AgentMsgTaskProg` | `task_progress` | Agent -> Server |
| `AgentMsgTaskResult` | `task_result` | Agent -> Server |
| `AgentMsgTaskError` | `task_error` | Agent -> Server |
| `AgentMsgBuildReq` | `build_request` | Server -> Agent |
| `AgentMsgBuildResult` | `build_result` | Agent -> Server |
| `AgentMsgTestReq` | `test_request` | Server -> Agent |
| `AgentMsgTestResult` | `test_result` | Agent -> Server |
| `AgentMsgQAReq` | `qa_request` | Server -> Agent |
| `AgentMsgQAResult` | `qa_result` | Agent -> Server |
| `AgentMsgComputerAction` | `computer_action` | Server -> Agent |
| `AgentMsgComputerResult` | `computer_result` | Agent -> Server |
| `AgentMsgComputerSessionStart` | `computer_session_start` | Server -> Agent |
| `AgentMsgComputerSessionEnd` | `computer_session_end` | Server -> Agent |
| `AgentMsgBrowserSessionStart` | `browser_session_start` | Server -> Agent |
| `AgentMsgBrowserAction` | `browser_action` | Server -> Agent |
| `AgentMsgBrowserSessionEnd` | `browser_session_end` | Server -> Agent |
| `AgentMsgBrowserSessionReady` | `browser_session_ready` | Agent -> Server |
| `AgentMsgBrowserResult` | `browser_result` | Agent -> Server |
| `AgentMsgBrowserNotAvailable` | `browser_not_available` | Agent -> Server |
| `AgentMsgBrowserError` | `browser_error` | Agent -> Server |
| `AgentMsgWorkerPhaseChanged` | `worker_phase_changed` | Agent -> Server |
| `AgentMsgProjectContext` | `project_context` | Agent -> Server |
| `AgentMsgCLIRequest` | `cli_request` | Server -> Agent |
| `AgentMsgCLIResult` | `cli_result` | Agent -> Server |
| `AgentMsgMCPStart` | `mcp_start` | Server -> Agent |
| `AgentMsgMCPReady` | `mcp_ready` | Agent -> Server |
| `AgentMsgMCPStop` | `mcp_stop` | Server -> Agent |
| `AgentMsgMCPError` | `mcp_error` | Agent -> Server |
| `AgentMsgMCPCodegenRequest` | `mcp_codegen_request` | Server -> Agent |
| `AgentMsgMCPCodegenProgress` | `mcp_codegen_progress` | Agent -> Server |
| `AgentMsgMCPCodegenResult` | `mcp_codegen_result` | Agent -> Server |
| `AgentMsgMCPDeploy` | `mcp_deploy` | Server -> Agent |
| `AgentMsgMCPDeployResult` | `mcp_deploy_result` | Agent -> Server |
| `AgentMsgMCPHealthReport` | `mcp_health_report` | Agent -> Server |

## Contributing

Contributions are welcome. Please open an issue or submit a pull request on GitHub.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/my-change`)
3. Commit your changes (`git commit -m 'Add new message type'`)
4. Push to the branch (`git push origin feature/my-change`)
5. Open a Pull Request

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
