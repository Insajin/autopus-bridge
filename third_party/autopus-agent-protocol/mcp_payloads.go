package ws

// MCPStartPayload is sent by the server to request MCP server startup on the bridge.
type MCPStartPayload struct {
	ServerName     string            `json:"server_name"`
	Command        string            `json:"command"`
	Args           []string          `json:"args"`
	Env            map[string]string `json:"env,omitempty"`
	WorkingDir     string            `json:"working_dir,omitempty"`
	TimeoutSeconds int               `json:"timeout_seconds"`
}

// MCPReadyPayload is sent by the bridge when an MCP server is ready.
type MCPReadyPayload struct {
	ServerName     string `json:"server_name"`
	Transport      string `json:"transport"`
	ConnectionInfo string `json:"connection_info"`
	PID            int    `json:"pid"`
	Port           int    `json:"port,omitempty"`
}

// MCPStopPayload is sent by the server to request MCP server shutdown.
type MCPStopPayload struct {
	ServerName string `json:"server_name"`
	Force      bool   `json:"force"`
}

// MCPErrorPayload is sent by the bridge when an MCP server encounters an error.
type MCPErrorPayload struct {
	ServerName string `json:"server_name"`
	Error      string `json:"error"`
	IsFatal    bool   `json:"is_fatal"`
}

// MCPServeStartPayload is sent by the server to request the bridge to start
// its own MCP server (stdio transport) backed by the Autopus backend API.
// SPEC-AI-003 M3 T-24
type MCPServeStartPayload struct {
	BackendURL string `json:"backend_url"` // Autopus 백엔드 API URL
}

// MCPServeStopPayload is sent by the server to request the bridge to stop
// its MCP server.
// SPEC-AI-003 M3 T-25
type MCPServeStopPayload struct {
	Reason string `json:"reason,omitempty"` // 중지 사유
}

// MCPServeResultPayload is sent by the bridge as a response to
// mcp_serve_stop requests or error responses. For successful mcp_serve_start,
// use AgentMsgMCPServeReady ("mcp_serve_ready") message type instead.
// SPEC-AI-003 M3, REQ-INTEGRATION-001
type MCPServeResultPayload struct {
	Status  string `json:"status"`            // "started", "stopped", "error"
	Message string `json:"message,omitempty"` // 추가 메시지
	Error   string `json:"error,omitempty"`   // 에러 메시지
}
