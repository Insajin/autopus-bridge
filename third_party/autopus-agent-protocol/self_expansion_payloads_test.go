package ws

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// TestMCPCodegenRequestPayload_JSON verifies JSON marshal/unmarshal roundtrip for MCPCodegenRequestPayload.
func TestMCPCodegenRequestPayload_JSON(t *testing.T) {
	t.Run("full payload roundtrip", func(t *testing.T) {
		manifest := &SecurityManifest{
			AllowedDomains:    []string{"api.example.com", "cdn.example.com"},
			RequiredEnvVars:   []string{"API_KEY", "SECRET"},
			FilesystemAccess:  "read-only",
			NetworkAccess:     "restricted",
			MaxRequestRate:    100,
			MaxResponseSizeKB: 1024,
		}
		original := MCPCodegenRequestPayload{
			ServiceName:      "weather-service",
			TemplateID:       "rest-api-v2",
			Description:      "A weather data aggregation MCP server",
			RequiredAPIs:     []string{"openweathermap", "weatherapi"},
			AuthType:         "bearer",
			SecurityManifest: manifest,
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var decoded MCPCodegenRequestPayload
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if original.ServiceName != decoded.ServiceName {
			t.Errorf("ServiceName = %q, want %q", decoded.ServiceName, original.ServiceName)
		}
		if original.TemplateID != decoded.TemplateID {
			t.Errorf("TemplateID = %q, want %q", decoded.TemplateID, original.TemplateID)
		}
		if original.Description != decoded.Description {
			t.Errorf("Description = %q, want %q", decoded.Description, original.Description)
		}
		if !reflect.DeepEqual(original.RequiredAPIs, decoded.RequiredAPIs) {
			t.Errorf("RequiredAPIs = %v, want %v", decoded.RequiredAPIs, original.RequiredAPIs)
		}
		if original.AuthType != decoded.AuthType {
			t.Errorf("AuthType = %q, want %q", decoded.AuthType, original.AuthType)
		}
		if decoded.SecurityManifest == nil {
			t.Fatal("SecurityManifest should not be nil")
		}
		if !reflect.DeepEqual(original.SecurityManifest.AllowedDomains, decoded.SecurityManifest.AllowedDomains) {
			t.Errorf("AllowedDomains = %v, want %v", decoded.SecurityManifest.AllowedDomains, original.SecurityManifest.AllowedDomains)
		}
		if original.SecurityManifest.MaxRequestRate != decoded.SecurityManifest.MaxRequestRate {
			t.Errorf("MaxRequestRate = %d, want %d", decoded.SecurityManifest.MaxRequestRate, original.SecurityManifest.MaxRequestRate)
		}
	})

	t.Run("verify JSON field names", func(t *testing.T) {
		original := MCPCodegenRequestPayload{
			ServiceName:  "test-svc",
			TemplateID:   "tmpl-1",
			Description:  "desc",
			RequiredAPIs: []string{"api1"},
			AuthType:     "oauth2",
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var raw map[string]interface{}
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		expectedFields := []string{"service_name", "template_id", "description", "required_apis", "auth_type"}
		for _, field := range expectedFields {
			if _, ok := raw[field]; !ok {
				t.Errorf("JSON should contain field %q", field)
			}
		}
	})

	t.Run("omitempty fields omitted when empty", func(t *testing.T) {
		original := MCPCodegenRequestPayload{
			ServiceName:  "svc",
			Description:  "desc",
			RequiredAPIs: []string{},
			AuthType:     "none",
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		s := string(data)
		if strings.Contains(s, "template_id") {
			t.Error("empty template_id should be omitted")
		}
		if strings.Contains(s, "security_manifest") {
			t.Error("nil security_manifest should be omitted")
		}
	})
}

// TestMCPCodegenProgressPayload_JSON verifies JSON marshal/unmarshal roundtrip for MCPCodegenProgressPayload.
func TestMCPCodegenProgressPayload_JSON(t *testing.T) {
	tests := []struct {
		name     string
		payload  MCPCodegenProgressPayload
		checkMsg string
	}{
		{
			name: "progress 0%",
			payload: MCPCodegenProgressPayload{
				Phase:    "template_loading",
				Progress: 0,
				Message:  "Loading template...",
			},
			checkMsg: "Loading template...",
		},
		{
			name: "progress 50%",
			payload: MCPCodegenProgressPayload{
				Phase:    "generating",
				Progress: 50,
				Message:  "Generating code files...",
			},
			checkMsg: "Generating code files...",
		},
		{
			name: "progress 100%",
			payload: MCPCodegenProgressPayload{
				Phase:    "collecting",
				Progress: 100,
				Message:  "Code generation complete",
			},
			checkMsg: "Code generation complete",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.payload)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var decoded MCPCodegenProgressPayload
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if !reflect.DeepEqual(tt.payload, decoded) {
				t.Errorf("roundtrip mismatch: got %+v, want %+v", decoded, tt.payload)
			}
			if decoded.Message != tt.checkMsg {
				t.Errorf("Message = %q, want %q", decoded.Message, tt.checkMsg)
			}
		})
	}

	t.Run("verify stage field serialization", func(t *testing.T) {
		payload := MCPCodegenProgressPayload{
			Phase:    "generating",
			Progress: 75,
			Message:  "Almost done",
		}

		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var raw map[string]interface{}
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		expectedFields := []string{"phase", "progress", "message"}
		for _, field := range expectedFields {
			if _, ok := raw[field]; !ok {
				t.Errorf("JSON should contain field %q", field)
			}
		}
		if raw["phase"] != "generating" {
			t.Errorf("phase = %v, want generating", raw["phase"])
		}
		if raw["progress"] != float64(75) {
			t.Errorf("progress = %v, want 75", raw["progress"])
		}
	})
}

// TestMCPCodegenResultPayload_JSON verifies JSON marshal/unmarshal roundtrip for MCPCodegenResultPayload.
func TestMCPCodegenResultPayload_JSON(t *testing.T) {
	t.Run("full success payload with files", func(t *testing.T) {
		original := MCPCodegenResultPayload{
			Status: "success",
			Files: []MCPGeneratedFile{
				{Path: "main.go", Content: "package main", SizeBytes: 12},
				{Path: "handler.go", Content: "package main\nfunc handler() {}", SizeBytes: 32},
			},
			TotalFiles:       2,
			TotalSizeBytes:   44,
			GenerationDurMs:  1500,
			ClaudeTokensUsed: 2048,
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var decoded MCPCodegenResultPayload
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if decoded.Status != "success" {
			t.Errorf("Status = %q, want success", decoded.Status)
		}
		if len(decoded.Files) != 2 {
			t.Fatalf("Files length = %d, want 2", len(decoded.Files))
		}
		if decoded.Files[0].Path != "main.go" {
			t.Errorf("Files[0].Path = %q, want main.go", decoded.Files[0].Path)
		}
		if decoded.Files[1].Path != "handler.go" {
			t.Errorf("Files[1].Path = %q, want handler.go", decoded.Files[1].Path)
		}
		if decoded.Files[0].SizeBytes != 12 {
			t.Errorf("Files[0].SizeBytes = %d, want 12", decoded.Files[0].SizeBytes)
		}
		if decoded.TotalFiles != 2 {
			t.Errorf("TotalFiles = %d, want 2", decoded.TotalFiles)
		}
		if decoded.TotalSizeBytes != 44 {
			t.Errorf("TotalSizeBytes = %d, want 44", decoded.TotalSizeBytes)
		}
		if decoded.GenerationDurMs != 1500 {
			t.Errorf("GenerationDurMs = %d, want 1500", decoded.GenerationDurMs)
		}
		if decoded.ClaudeTokensUsed != 2048 {
			t.Errorf("ClaudeTokensUsed = %d, want 2048", decoded.ClaudeTokensUsed)
		}
		if decoded.Error != "" {
			t.Errorf("Error should be empty, got %q", decoded.Error)
		}
	})

	t.Run("error payload with error_message", func(t *testing.T) {
		original := MCPCodegenResultPayload{
			Status:          "error",
			TotalFiles:      0,
			TotalSizeBytes:  0,
			GenerationDurMs: 300,
			Error:           "template not found: rest-api-v3",
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var decoded MCPCodegenResultPayload
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if decoded.Status != "error" {
			t.Errorf("Status = %q, want error", decoded.Status)
		}
		if decoded.Files != nil {
			t.Errorf("Files should be nil, got %v", decoded.Files)
		}
		if decoded.Error != "template not found: rest-api-v3" {
			t.Errorf("Error = %q, want template not found: rest-api-v3", decoded.Error)
		}
	})

	t.Run("verify JSON field names", func(t *testing.T) {
		original := MCPCodegenResultPayload{
			Status:          "success",
			Files:           []MCPGeneratedFile{{Path: "a.go", Content: "x", SizeBytes: 1}},
			TotalFiles:      1,
			TotalSizeBytes:  1,
			GenerationDurMs: 100,
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var raw map[string]interface{}
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		expectedFields := []string{"status", "files", "total_files", "total_size_bytes", "generation_duration_ms"}
		for _, field := range expectedFields {
			if _, ok := raw[field]; !ok {
				t.Errorf("JSON should contain field %q", field)
			}
		}
	})

	t.Run("omitempty fields omitted when empty", func(t *testing.T) {
		original := MCPCodegenResultPayload{
			Status:          "success",
			TotalFiles:      0,
			TotalSizeBytes:  0,
			GenerationDurMs: 50,
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var raw map[string]interface{}
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		omittedFields := []string{"files", "claude_tokens_used", "error"}
		for _, field := range omittedFields {
			if _, ok := raw[field]; ok {
				t.Errorf("field %q should be omitted when empty/zero", field)
			}
		}
	})
}

// TestMCPDeployPayload_JSON verifies JSON marshal/unmarshal roundtrip for MCPDeployPayload.
func TestMCPDeployPayload_JSON(t *testing.T) {
	t.Run("full deploy payload with env_vars and security_manifest", func(t *testing.T) {
		original := MCPDeployPayload{
			ServiceName: "weather-service",
			Files: []MCPGeneratedFile{
				{Path: "main.go", Content: "package main\nfunc main() {}", SizeBytes: 28},
				{Path: "config.yaml", Content: "port: 8080", SizeBytes: 10},
			},
			SecurityManifest: &SecurityManifest{
				AllowedDomains:    []string{"api.weather.com"},
				RequiredEnvVars:   []string{"WEATHER_API_KEY"},
				FilesystemAccess:  "none",
				NetworkAccess:     "restricted",
				MaxRequestRate:    50,
				MaxResponseSizeKB: 512,
			},
			EnvVars: map[string]string{
				"WEATHER_API_KEY": "sk-test-123",
				"LOG_LEVEL":       "info",
			},
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var decoded MCPDeployPayload
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if decoded.ServiceName != "weather-service" {
			t.Errorf("ServiceName = %q, want weather-service", decoded.ServiceName)
		}
		if len(decoded.Files) != 2 {
			t.Fatalf("Files length = %d, want 2", len(decoded.Files))
		}
		if decoded.Files[0].Path != "main.go" {
			t.Errorf("Files[0].Path = %q, want main.go", decoded.Files[0].Path)
		}
		if decoded.SecurityManifest == nil {
			t.Fatal("SecurityManifest should not be nil")
		}
		if !reflect.DeepEqual(decoded.SecurityManifest.AllowedDomains, []string{"api.weather.com"}) {
			t.Errorf("AllowedDomains = %v, want [api.weather.com]", decoded.SecurityManifest.AllowedDomains)
		}
		if decoded.SecurityManifest.FilesystemAccess != "none" {
			t.Errorf("FilesystemAccess = %q, want none", decoded.SecurityManifest.FilesystemAccess)
		}
		if len(decoded.EnvVars) != 2 {
			t.Fatalf("EnvVars length = %d, want 2", len(decoded.EnvVars))
		}
		if decoded.EnvVars["WEATHER_API_KEY"] != "sk-test-123" {
			t.Errorf("EnvVars[WEATHER_API_KEY] = %q, want sk-test-123", decoded.EnvVars["WEATHER_API_KEY"])
		}
		if decoded.EnvVars["LOG_LEVEL"] != "info" {
			t.Errorf("EnvVars[LOG_LEVEL] = %q, want info", decoded.EnvVars["LOG_LEVEL"])
		}
	})

	t.Run("deploy payload without optional env_vars", func(t *testing.T) {
		original := MCPDeployPayload{
			ServiceName: "minimal-svc",
			Files:       []MCPGeneratedFile{{Path: "main.go", Content: "pkg", SizeBytes: 3}},
			SecurityManifest: &SecurityManifest{
				AllowedDomains:   []string{},
				RequiredEnvVars:  []string{},
				FilesystemAccess: "none",
				NetworkAccess:    "none",
			},
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var raw map[string]interface{}
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}
		if _, ok := raw["env_vars"]; ok {
			t.Error("nil env_vars should be omitted")
		}

		var decoded MCPDeployPayload
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if decoded.EnvVars != nil {
			t.Errorf("EnvVars should be nil, got %v", decoded.EnvVars)
		}
	})
}

// TestMCPDeployResultPayload_JSON verifies JSON marshal/unmarshal roundtrip for MCPDeployResultPayload.
func TestMCPDeployResultPayload_JSON(t *testing.T) {
	t.Run("success variant", func(t *testing.T) {
		original := MCPDeployResultPayload{
			ServiceName: "weather-service",
			Success:     true,
			DeployPath:  "/opt/mcp/weather-service",
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var decoded MCPDeployResultPayload
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if decoded.ServiceName != "weather-service" {
			t.Errorf("ServiceName = %q, want weather-service", decoded.ServiceName)
		}
		if !decoded.Success {
			t.Error("Success should be true")
		}
		if decoded.DeployPath != "/opt/mcp/weather-service" {
			t.Errorf("DeployPath = %q, want /opt/mcp/weather-service", decoded.DeployPath)
		}
		if decoded.Error != "" {
			t.Errorf("Error should be empty, got %q", decoded.Error)
		}
	})

	t.Run("failure variant", func(t *testing.T) {
		original := MCPDeployResultPayload{
			ServiceName: "broken-service",
			Success:     false,
			Error:       "permission denied: /opt/mcp/broken-service",
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var decoded MCPDeployResultPayload
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if decoded.ServiceName != "broken-service" {
			t.Errorf("ServiceName = %q, want broken-service", decoded.ServiceName)
		}
		if decoded.Success {
			t.Error("Success should be false")
		}
		if decoded.Error != "permission denied: /opt/mcp/broken-service" {
			t.Errorf("Error = %q, want permission denied: /opt/mcp/broken-service", decoded.Error)
		}
		if decoded.DeployPath != "" {
			t.Errorf("DeployPath should be empty, got %q", decoded.DeployPath)
		}
	})

	t.Run("omitempty fields omitted when empty", func(t *testing.T) {
		successPayload := MCPDeployResultPayload{
			ServiceName: "svc",
			Success:     true,
			DeployPath:  "/opt/svc",
		}
		data, err := json.Marshal(successPayload)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}
		if strings.Contains(string(data), `"error"`) {
			t.Error("empty error should be omitted on success")
		}

		failPayload := MCPDeployResultPayload{
			ServiceName: "svc",
			Success:     false,
			Error:       "fail",
		}
		data, err = json.Marshal(failPayload)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}
		if strings.Contains(string(data), "deploy_path") {
			t.Error("empty deploy_path should be omitted on failure")
		}
	})
}

// TestMCPHealthReportPayload_JSON verifies JSON marshal/unmarshal roundtrip for MCPHealthReportPayload.
func TestMCPHealthReportPayload_JSON(t *testing.T) {
	t.Run("multiple servers in report", func(t *testing.T) {
		lastErr := "connection timeout"
		original := MCPHealthReportPayload{
			Servers: []MCPServerHealth{
				{
					Name:          "weather-service",
					Status:        "running",
					UptimeSeconds: 86400,
					TotalCalls:    15000,
					ErrorCount:    12,
					ErrorRate:     0.0008,
					AvgResponseMs: 45,
					MemoryMB:      128,
					LastError:     nil,
				},
				{
					Name:          "translation-service",
					Status:        "error",
					UptimeSeconds: 3600,
					TotalCalls:    500,
					ErrorCount:    50,
					ErrorRate:     0.1,
					AvgResponseMs: 200,
					MemoryMB:      256,
					LastError:     &lastErr,
				},
			},
			ReportedAt: "2026-02-18T10:30:00Z",
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var decoded MCPHealthReportPayload
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if len(decoded.Servers) != 2 {
			t.Fatalf("Servers length = %d, want 2", len(decoded.Servers))
		}
		if decoded.Servers[0].Name != "weather-service" {
			t.Errorf("Servers[0].Name = %q, want weather-service", decoded.Servers[0].Name)
		}
		if decoded.Servers[0].Status != "running" {
			t.Errorf("Servers[0].Status = %q, want running", decoded.Servers[0].Status)
		}
		if decoded.Servers[0].UptimeSeconds != 86400 {
			t.Errorf("Servers[0].UptimeSeconds = %d, want 86400", decoded.Servers[0].UptimeSeconds)
		}
		if decoded.Servers[0].TotalCalls != 15000 {
			t.Errorf("Servers[0].TotalCalls = %d, want 15000", decoded.Servers[0].TotalCalls)
		}
		if decoded.Servers[0].ErrorRate != 0.0008 {
			t.Errorf("Servers[0].ErrorRate = %f, want 0.0008", decoded.Servers[0].ErrorRate)
		}
		if decoded.Servers[0].LastError != nil {
			t.Errorf("Servers[0].LastError should be nil, got %v", decoded.Servers[0].LastError)
		}

		if decoded.Servers[1].Name != "translation-service" {
			t.Errorf("Servers[1].Name = %q, want translation-service", decoded.Servers[1].Name)
		}
		if decoded.Servers[1].Status != "error" {
			t.Errorf("Servers[1].Status = %q, want error", decoded.Servers[1].Status)
		}
		if decoded.Servers[1].LastError == nil {
			t.Fatal("Servers[1].LastError should not be nil")
		}
		if *decoded.Servers[1].LastError != "connection timeout" {
			t.Errorf("Servers[1].LastError = %q, want connection timeout", *decoded.Servers[1].LastError)
		}

		if decoded.ReportedAt != "2026-02-18T10:30:00Z" {
			t.Errorf("ReportedAt = %q, want 2026-02-18T10:30:00Z", decoded.ReportedAt)
		}
	})

	t.Run("empty servers array", func(t *testing.T) {
		original := MCPHealthReportPayload{
			Servers:    []MCPServerHealth{},
			ReportedAt: "2026-02-18T00:00:00Z",
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var decoded MCPHealthReportPayload
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if len(decoded.Servers) != 0 {
			t.Errorf("Servers should be empty, got %d items", len(decoded.Servers))
		}
		if decoded.ReportedAt != "2026-02-18T00:00:00Z" {
			t.Errorf("ReportedAt = %q, want 2026-02-18T00:00:00Z", decoded.ReportedAt)
		}
	})

	t.Run("single server without last_error", func(t *testing.T) {
		original := MCPHealthReportPayload{
			Servers: []MCPServerHealth{
				{
					Name:          "healthy-svc",
					Status:        "running",
					UptimeSeconds: 7200,
					TotalCalls:    100,
					ErrorCount:    0,
					ErrorRate:     0.0,
					AvgResponseMs: 10,
					MemoryMB:      64,
				},
			},
			ReportedAt: "2026-02-18T12:00:00Z",
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		if strings.Contains(string(data), "last_error") {
			t.Error("nil last_error should be omitted")
		}

		var decoded MCPHealthReportPayload
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if decoded.Servers[0].LastError != nil {
			t.Errorf("LastError should be nil, got %v", decoded.Servers[0].LastError)
		}
	})
}

// TestSecurityManifest_JSON verifies JSON marshal/unmarshal roundtrip for SecurityManifest.
func TestSecurityManifest_JSON(t *testing.T) {
	t.Run("full manifest with all fields", func(t *testing.T) {
		original := SecurityManifest{
			AllowedDomains:    []string{"api.example.com", "cdn.example.com", "auth.example.com"},
			RequiredEnvVars:   []string{"API_KEY", "DB_URL", "REDIS_URL"},
			FilesystemAccess:  "read-only",
			NetworkAccess:     "restricted",
			MaxRequestRate:    200,
			MaxResponseSizeKB: 2048,
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var decoded SecurityManifest
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if !reflect.DeepEqual(original, decoded) {
			t.Errorf("roundtrip mismatch: got %+v, want %+v", decoded, original)
		}
	})

	t.Run("verify JSON field names", func(t *testing.T) {
		original := SecurityManifest{
			AllowedDomains:    []string{"example.com"},
			RequiredEnvVars:   []string{"KEY"},
			FilesystemAccess:  "none",
			NetworkAccess:     "full",
			MaxRequestRate:    10,
			MaxResponseSizeKB: 100,
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var raw map[string]interface{}
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		expectedFields := []string{"allowed_domains", "required_env_vars", "filesystem_access", "network_access", "max_request_rate", "max_response_size_kb"}
		for _, field := range expectedFields {
			if _, ok := raw[field]; !ok {
				t.Errorf("JSON should contain field %q", field)
			}
		}
	})

	t.Run("omitempty fields omitted when zero", func(t *testing.T) {
		original := SecurityManifest{
			AllowedDomains:   []string{"example.com"},
			RequiredEnvVars:  []string{},
			FilesystemAccess: "none",
			NetworkAccess:    "none",
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		s := string(data)
		if strings.Contains(s, "max_request_rate") {
			t.Error("zero max_request_rate should be omitted")
		}
		if strings.Contains(s, "max_response_size_kb") {
			t.Error("zero max_response_size_kb should be omitted")
		}
	})
}

// TestGeneratedFilePayload_JSON verifies JSON marshal/unmarshal roundtrip for MCPGeneratedFile.
func TestGeneratedFilePayload_JSON(t *testing.T) {
	t.Run("single file with all fields", func(t *testing.T) {
		original := MCPGeneratedFile{
			Path:      "internal/handler/weather.go",
			Content:   "package handler\n\nimport \"net/http\"\n\nfunc WeatherHandler(w http.ResponseWriter, r *http.Request) {\n\tw.WriteHeader(200)\n}\n",
			SizeBytes: 120,
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var decoded MCPGeneratedFile
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if !reflect.DeepEqual(original, decoded) {
			t.Errorf("roundtrip mismatch: got %+v, want %+v", decoded, original)
		}
	})

	t.Run("verify JSON field names", func(t *testing.T) {
		original := MCPGeneratedFile{
			Path:      "main.go",
			Content:   "package main",
			SizeBytes: 12,
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var raw map[string]interface{}
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		expectedFields := []string{"path", "content", "size_bytes"}
		for _, field := range expectedFields {
			if _, ok := raw[field]; !ok {
				t.Errorf("JSON should contain field %q", field)
			}
		}
	})

	t.Run("file with empty content", func(t *testing.T) {
		original := MCPGeneratedFile{
			Path:      ".gitkeep",
			Content:   "",
			SizeBytes: 0,
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var decoded MCPGeneratedFile
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if decoded.Path != ".gitkeep" {
			t.Errorf("Path = %q, want .gitkeep", decoded.Path)
		}
		if decoded.Content != "" {
			t.Errorf("Content should be empty, got %q", decoded.Content)
		}
		if decoded.SizeBytes != 0 {
			t.Errorf("SizeBytes = %d, want 0", decoded.SizeBytes)
		}
	})
}

// TestAllPayloads_EmptyStruct verifies that marshaling empty structs does not panic.
func TestAllPayloads_EmptyStruct(t *testing.T) {
	tests := []struct {
		name    string
		payload interface{}
	}{
		{"MCPCodegenRequestPayload", MCPCodegenRequestPayload{}},
		{"MCPCodegenProgressPayload", MCPCodegenProgressPayload{}},
		{"MCPCodegenResultPayload", MCPCodegenResultPayload{}},
		{"MCPGeneratedFile", MCPGeneratedFile{}},
		{"MCPDeployPayload", MCPDeployPayload{}},
		{"MCPDeployResultPayload", MCPDeployResultPayload{}},
		{"MCPHealthReportPayload", MCPHealthReportPayload{}},
		{"MCPServerHealth", MCPServerHealth{}},
		{"SecurityManifest", SecurityManifest{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.payload)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}
			if len(data) == 0 {
				t.Error("marshaled data should not be empty")
			}
			if !json.Valid(data) {
				t.Error("output should be valid JSON")
			}
		})
	}
}
