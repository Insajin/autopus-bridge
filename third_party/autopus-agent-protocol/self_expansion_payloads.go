package ws

// MCPCodegenRequestPayload is sent from server to bridge to request MCP code generation.
// Message type: mcp_codegen_request (Server -> Bridge)
type MCPCodegenRequestPayload struct {
	ServiceName      string            `json:"service_name"`
	TemplateID       string            `json:"template_id,omitempty"`
	Description      string            `json:"description"`
	RequiredAPIs     []string          `json:"required_apis"`
	AuthType         string            `json:"auth_type"`
	SecurityManifest *SecurityManifest `json:"security_manifest,omitempty"`
}

// SecurityManifest declares the allowed resources for a generated MCP server.
type SecurityManifest struct {
	AllowedDomains    []string `json:"allowed_domains"`
	RequiredEnvVars   []string `json:"required_env_vars"`
	FilesystemAccess  string   `json:"filesystem_access"`
	NetworkAccess     string   `json:"network_access"`
	MaxRequestRate    int      `json:"max_request_rate,omitempty"`
	MaxResponseSizeKB int      `json:"max_response_size_kb,omitempty"`
}

// MCPCodegenProgressPayload reports code generation progress.
// Message type: mcp_codegen_progress (Bridge -> Server)
type MCPCodegenProgressPayload struct {
	Phase    string `json:"phase"`    // "template_loading", "generating", "collecting"
	Progress int    `json:"progress"` // 0-100
	Message  string `json:"message"`
}

// MCPCodegenResultPayload is sent from bridge to server with generated code.
// Message type: mcp_codegen_result (Bridge -> Server)
type MCPCodegenResultPayload struct {
	Status           string             `json:"status"` // "success", "error"
	Files            []MCPGeneratedFile `json:"files,omitempty"`
	TotalFiles       int                `json:"total_files"`
	TotalSizeBytes   int64              `json:"total_size_bytes"`
	GenerationDurMs  int64              `json:"generation_duration_ms"`
	ClaudeTokensUsed int                `json:"claude_tokens_used,omitempty"`
	Error            string             `json:"error,omitempty"`
}

// MCPGeneratedFile represents a single generated file.
type MCPGeneratedFile struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	SizeBytes int64  `json:"size_bytes"`
}

// MCPDeployPayload is sent from server to bridge to deploy approved MCP code.
// Message type: mcp_deploy (Server -> Bridge)
type MCPDeployPayload struct {
	ServiceName      string             `json:"service_name"`
	Files            []MCPGeneratedFile `json:"files"`
	SecurityManifest *SecurityManifest  `json:"security_manifest"`
	EnvVars          map[string]string  `json:"env_vars,omitempty"`
}

// MCPDeployResultPayload is sent from bridge to server after deployment.
// Message type: mcp_deploy_result (Bridge -> Server)
type MCPDeployResultPayload struct {
	ServiceName string `json:"service_name"`
	Success     bool   `json:"success"`
	DeployPath  string `json:"deploy_path,omitempty"`
	Error       string `json:"error,omitempty"`
}

// MCPHealthReportPayload is sent periodically from bridge to server.
// Message type: mcp_health_report (Bridge -> Server)
type MCPHealthReportPayload struct {
	Servers    []MCPServerHealth `json:"servers"`
	ReportedAt string            `json:"reported_at"`
}

// MCPServerHealth represents health status of a single MCP server.
type MCPServerHealth struct {
	Name          string  `json:"name"`
	Status        string  `json:"status"` // "running", "stopped", "error"
	UptimeSeconds int64   `json:"uptime_seconds"`
	TotalCalls    int     `json:"total_calls"`
	ErrorCount    int     `json:"error_count"`
	ErrorRate     float64 `json:"error_rate"`
	AvgResponseMs int64   `json:"avg_response_ms"`
	MemoryMB      int     `json:"memory_mb"`
	LastError     *string `json:"last_error,omitempty"`
}
