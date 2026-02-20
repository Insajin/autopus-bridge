package aitools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestDefaultAutopusMCPServer는 기본 MCP 서버 설정 값을 검증합니다.
func TestDefaultAutopusMCPServer(t *testing.T) {
	server := DefaultAutopusMCPServer()

	if server.Command != "autopus-bridge" {
		t.Errorf("Command = %q, want %q", server.Command, "autopus-bridge")
	}

	if len(server.Args) != 1 {
		t.Fatalf("Args 길이 = %d, want 1", len(server.Args))
	}

	if server.Args[0] != "mcp-serve" {
		t.Errorf("Args[0] = %q, want %q", server.Args[0], "mcp-serve")
	}
}

// TestReadJSONConfig는 JSON 설정 파일 읽기 기능을 테스트합니다.
func TestReadJSONConfig(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		setup       bool // true이면 파일을 생성
		wantErr     bool
		wantKeys    []string
		description string
	}{
		{
			name:        "유효한 JSON 파일",
			content:     `{"key": "value", "number": 42}`,
			setup:       true,
			wantErr:     false,
			wantKeys:    []string{"key", "number"},
			description: "유효한 JSON을 정상적으로 파싱해야 합니다",
		},
		{
			name:        "빈 JSON 객체",
			content:     `{}`,
			setup:       true,
			wantErr:     false,
			wantKeys:    nil,
			description: "빈 JSON 객체도 정상 처리해야 합니다",
		},
		{
			name:        "중첩된 JSON 객체",
			content:     `{"mcpServers": {"autopus": {"command": "test"}}}`,
			setup:       true,
			wantErr:     false,
			wantKeys:    []string{"mcpServers"},
			description: "중첩된 JSON 구조를 파싱해야 합니다",
		},
		{
			name:        "존재하지 않는 파일",
			content:     "",
			setup:       false,
			wantErr:     true,
			description: "파일이 없으면 에러를 반환해야 합니다",
		},
		{
			name:        "유효하지 않은 JSON",
			content:     `{invalid json`,
			setup:       true,
			wantErr:     true,
			description: "잘못된 JSON은 파싱 에러를 반환해야 합니다",
		},
		{
			name:        "JSON 배열 (map이 아닌 경우)",
			content:     `[1, 2, 3]`,
			setup:       true,
			wantErr:     true,
			description: "JSON 배열은 map으로 파싱할 수 없어 에러를 반환해야 합니다",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			filePath := filepath.Join(tmpDir, "config.json")

			if tt.setup {
				if err := os.WriteFile(filePath, []byte(tt.content), 0644); err != nil {
					t.Fatalf("테스트 파일 생성 실패: %v", err)
				}
			}

			result, err := ReadJSONConfig(filePath)

			if (err != nil) != tt.wantErr {
				t.Errorf("ReadJSONConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if result == nil {
					t.Fatal("ReadJSONConfig() 결과가 nil이면 안 됩니다")
				}
				for _, key := range tt.wantKeys {
					if _, ok := result[key]; !ok {
						t.Errorf("결과에 키 %q가 없습니다", key)
					}
				}
			}
		})
	}
}

// TestWriteJSONConfig는 JSON 설정 파일 쓰기 기능을 테스트합니다.
func TestWriteJSONConfig(t *testing.T) {
	tests := []struct {
		name    string
		data    map[string]interface{}
		wantErr bool
	}{
		{
			name: "단순 데이터 쓰기",
			data: map[string]interface{}{
				"key": "value",
			},
			wantErr: false,
		},
		{
			name: "중첩된 데이터 쓰기",
			data: map[string]interface{}{
				"mcpServers": map[string]interface{}{
					"autopus": map[string]interface{}{
						"command": "autopus-bridge",
						"args":    []string{"mcp-serve"},
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "빈 데이터 쓰기",
			data:    map[string]interface{}{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			filePath := filepath.Join(tmpDir, "output.json")

			err := WriteJSONConfig(filePath, tt.data)

			if (err != nil) != tt.wantErr {
				t.Errorf("WriteJSONConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// 쓴 파일을 다시 읽어서 내용 확인
				readBack, readErr := ReadJSONConfig(filePath)
				if readErr != nil {
					t.Fatalf("쓴 파일 다시 읽기 실패: %v", readErr)
				}

				// 원본 데이터의 키가 모두 있는지 확인
				for key := range tt.data {
					if _, ok := readBack[key]; !ok {
						t.Errorf("읽어온 데이터에 키 %q가 없습니다", key)
					}
				}
			}
		})
	}
}

// TestWriteJSONConfig_중첩디렉토리생성은 상위 디렉토리가 없는 경우 자동 생성을 검증합니다.
func TestWriteJSONConfig_중첩디렉토리생성(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "nested", "deep", "config.json")

	data := map[string]interface{}{"test": "value"}
	err := WriteJSONConfig(filePath, data)
	if err != nil {
		t.Fatalf("WriteJSONConfig() 중첩 디렉토리 생성 실패: %v", err)
	}

	// 파일이 실제로 생성되었는지 확인
	if _, statErr := os.Stat(filePath); os.IsNotExist(statErr) {
		t.Error("파일이 생성되지 않았습니다")
	}
}

// TestWriteJSONConfig_후행개행은 JSON 파일 끝에 개행 문자가 있는지 검증합니다.
func TestWriteJSONConfig_후행개행(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "config.json")

	data := map[string]interface{}{"key": "value"}
	if err := WriteJSONConfig(filePath, data); err != nil {
		t.Fatalf("WriteJSONConfig() 실패: %v", err)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("파일 읽기 실패: %v", err)
	}

	if len(content) == 0 {
		t.Fatal("파일 내용이 비어있습니다")
	}

	if content[len(content)-1] != '\n' {
		t.Error("JSON 파일 끝에 개행 문자가 없습니다")
	}
}

// TestBackupFile는 파일 백업 기능을 테스트합니다.
func TestBackupFile(t *testing.T) {
	tests := []struct {
		name          string
		setupContent  string
		createFile    bool
		wantErr       bool
		wantBackup    bool
		description   string
	}{
		{
			name:         "기존 파일 백업 생성",
			setupContent: `{"key": "original"}`,
			createFile:   true,
			wantErr:      false,
			wantBackup:   true,
			description:  "기존 파일이 있으면 .bak 파일을 생성해야 합니다",
		},
		{
			name:         "파일이 없는 경우",
			setupContent: "",
			createFile:   false,
			wantErr:      false,
			wantBackup:   false,
			description:  "원본 파일이 없으면 에러 없이 반환해야 합니다",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			filePath := filepath.Join(tmpDir, "test.json")

			if tt.createFile {
				if err := os.WriteFile(filePath, []byte(tt.setupContent), 0644); err != nil {
					t.Fatalf("테스트 파일 생성 실패: %v", err)
				}
			}

			err := BackupFile(filePath)

			if (err != nil) != tt.wantErr {
				t.Errorf("BackupFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			backupPath := filePath + ".bak"
			_, backupErr := os.Stat(backupPath)

			if tt.wantBackup {
				if os.IsNotExist(backupErr) {
					t.Error(".bak 파일이 생성되지 않았습니다")
					return
				}

				// 백업 파일 내용이 원본과 동일한지 확인
				backupContent, readErr := os.ReadFile(backupPath)
				if readErr != nil {
					t.Fatalf("백업 파일 읽기 실패: %v", readErr)
				}

				if string(backupContent) != tt.setupContent {
					t.Errorf("백업 파일 내용이 다릅니다: got %q, want %q", string(backupContent), tt.setupContent)
				}
			} else {
				if !os.IsNotExist(backupErr) {
					t.Error("백업 파일이 생성되면 안 되는데 생성되었습니다")
				}
			}
		})
	}
}

// TestEnsureDir는 디렉토리 생성 기능을 테스트합니다.
func TestEnsureDir(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantErr  bool
	}{
		{
			name:    "중첩된 디렉토리 생성",
			path:    "a/b/c/file.json",
			wantErr: false,
		},
		{
			name:    "단일 레벨 디렉토리",
			path:    "dir/file.json",
			wantErr: false,
		},
		{
			name:    "이미 존재하는 디렉토리",
			path:    "file.json", // TempDir 자체가 이미 존재
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			fullPath := filepath.Join(tmpDir, tt.path)

			err := EnsureDir(fullPath)

			if (err != nil) != tt.wantErr {
				t.Errorf("EnsureDir() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				dir := filepath.Dir(fullPath)
				info, statErr := os.Stat(dir)
				if os.IsNotExist(statErr) {
					t.Errorf("디렉토리가 생성되지 않았습니다: %s", dir)
					return
				}
				if !info.IsDir() {
					t.Errorf("%s가 디렉토리가 아닙니다", dir)
				}
			}
		})
	}
}

// TestEnsureDir_이미존재하는디렉토리는 기존 디렉토리가 있어도 에러가 없는지 검증합니다.
func TestEnsureDir_이미존재하는디렉토리(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "existing")

	// 디렉토리를 먼저 생성
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("디렉토리 사전 생성 실패: %v", err)
	}

	filePath := filepath.Join(subDir, "file.json")

	// 이미 존재하는 디렉토리에 대해 EnsureDir 호출
	err := EnsureDir(filePath)
	if err != nil {
		t.Errorf("이미 존재하는 디렉토리에서 EnsureDir() error = %v", err)
	}
}

// TestAddMCPServerToJSON는 JSON 설정에 MCP 서버를 추가하는 기능을 테스트합니다.
func TestAddMCPServerToJSON(t *testing.T) {
	tests := []struct {
		name       string
		initial    map[string]interface{}
		serverName string
		server     MCPServerConfig
	}{
		{
			name:       "빈 설정에 서버 추가",
			initial:    map[string]interface{}{},
			serverName: "autopus",
			server: MCPServerConfig{
				Command: "autopus-bridge",
				Args:    []string{"mcp-serve"},
			},
		},
		{
			name: "기존 mcpServers에 서버 추가",
			initial: map[string]interface{}{
				"mcpServers": map[string]interface{}{
					"existing": map[string]interface{}{
						"command": "existing-cmd",
						"args":    []string{"arg1"},
					},
				},
			},
			serverName: "autopus",
			server: MCPServerConfig{
				Command: "autopus-bridge",
				Args:    []string{"mcp-serve"},
			},
		},
		{
			name: "기존 서버 덮어쓰기",
			initial: map[string]interface{}{
				"mcpServers": map[string]interface{}{
					"autopus": map[string]interface{}{
						"command": "old-command",
						"args":    []string{"old-arg"},
					},
				},
			},
			serverName: "autopus",
			server: MCPServerConfig{
				Command: "autopus-bridge",
				Args:    []string{"mcp-serve"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addMCPServerToJSON(tt.initial, tt.serverName, tt.server)

			// mcpServers 필드가 존재하는지 확인
			mcpServers, ok := tt.initial["mcpServers"].(map[string]interface{})
			if !ok {
				t.Fatal("mcpServers 필드가 없거나 올바른 타입이 아닙니다")
			}

			// 추가한 서버가 존재하는지 확인
			serverData, ok := mcpServers[tt.serverName].(map[string]interface{})
			if !ok {
				t.Fatalf("서버 %q가 없거나 올바른 타입이 아닙니다", tt.serverName)
			}

			if serverData["command"] != tt.server.Command {
				t.Errorf("command = %v, want %v", serverData["command"], tt.server.Command)
			}

			// args 확인 (interface{} 슬라이스로 저장됨)
			args, ok := serverData["args"].([]string)
			if !ok {
				t.Fatal("args가 []string 타입이 아닙니다")
			}

			if len(args) != len(tt.server.Args) {
				t.Errorf("args 길이 = %d, want %d", len(args), len(tt.server.Args))
			}
		})
	}
}

// TestAddMCPServerToJSON_기존서버보존은 새 서버 추가 시 기존 서버가 유지되는지 검증합니다.
func TestAddMCPServerToJSON_기존서버보존(t *testing.T) {
	config := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"existing-server": map[string]interface{}{
				"command": "existing-cmd",
				"args":    []string{"arg1"},
			},
		},
	}

	addMCPServerToJSON(config, "autopus", DefaultAutopusMCPServer())

	mcpServers := config["mcpServers"].(map[string]interface{})

	// 기존 서버가 여전히 존재하는지 확인
	if _, ok := mcpServers["existing-server"]; !ok {
		t.Error("기존 서버 'existing-server'가 삭제되었습니다")
	}

	// 새 서버도 존재하는지 확인
	if _, ok := mcpServers["autopus"]; !ok {
		t.Error("새 서버 'autopus'가 추가되지 않았습니다")
	}
}

// TestMCPServerConfig_JSON직렬화는 MCPServerConfig의 JSON 직렬화를 검증합니다.
func TestMCPServerConfig_JSON직렬화(t *testing.T) {
	server := DefaultAutopusMCPServer()

	data, err := json.Marshal(server)
	if err != nil {
		t.Fatalf("JSON 직렬화 실패: %v", err)
	}

	var decoded MCPServerConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("JSON 역직렬화 실패: %v", err)
	}

	if decoded.Command != server.Command {
		t.Errorf("Command = %q, want %q", decoded.Command, server.Command)
	}

	if len(decoded.Args) != len(server.Args) {
		t.Fatalf("Args 길이 = %d, want %d", len(decoded.Args), len(server.Args))
	}

	for i, arg := range decoded.Args {
		if arg != server.Args[i] {
			t.Errorf("Args[%d] = %q, want %q", i, arg, server.Args[i])
		}
	}
}

// TestWriteJSONConfig_쓰기불가경로는 디렉토리 생성이 불가능한 경로에서 에러를 반환하는지 검증합니다.
func TestWriteJSONConfig_쓰기불가경로(t *testing.T) {
	// /dev/null 아래에는 디렉토리를 생성할 수 없습니다
	err := WriteJSONConfig("/dev/null/impossible/config.json", map[string]interface{}{"key": "value"})
	if err == nil {
		t.Error("쓰기 불가능한 경로에서 WriteJSONConfig()가 에러를 반환하지 않았습니다")
	}
}

// TestWriteJSONConfig_파일쓰기실패는 디렉토리가 있지만 파일 쓰기가 실패하는 경우를 검증합니다.
func TestWriteJSONConfig_파일쓰기실패(t *testing.T) {
	tmpDir := t.TempDir()

	// 파일 경로에 디렉토리를 만들어 WriteFile이 실패하도록 함
	filePath := filepath.Join(tmpDir, "config.json")
	if err := os.MkdirAll(filePath, 0755); err != nil {
		t.Fatalf("디렉토리 생성 실패: %v", err)
	}

	err := WriteJSONConfig(filePath, map[string]interface{}{"key": "value"})
	if err == nil {
		t.Error("파일 쓰기 실패 시 WriteJSONConfig()가 에러를 반환하지 않았습니다")
	}
}

// TestBackupFile_읽기불가파일은 파일이 존재하지만 읽기 권한이 없는 경우 에러를 반환하는지 검증합니다.
func TestBackupFile_읽기불가파일(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.json")

	// 파일 생성 후 읽기 권한 제거
	if err := os.WriteFile(filePath, []byte(`{"key": "value"}`), 0644); err != nil {
		t.Fatalf("파일 생성 실패: %v", err)
	}
	if err := os.Chmod(filePath, 0000); err != nil {
		t.Fatalf("권한 변경 실패: %v", err)
	}
	// 테스트 후 정리를 위해 권한 복원
	t.Cleanup(func() {
		os.Chmod(filePath, 0644)
	})

	err := BackupFile(filePath)
	if err == nil {
		t.Error("읽기 불가 파일에서 BackupFile()이 에러를 반환하지 않았습니다")
	}
}

// TestBackupFile_백업쓰기실패는 백업 파일 쓰기가 실패하는 경우를 검증합니다.
func TestBackupFile_백업쓰기실패(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.json")

	// 원본 파일 생성
	if err := os.WriteFile(filePath, []byte(`{"key": "value"}`), 0644); err != nil {
		t.Fatalf("파일 생성 실패: %v", err)
	}

	// .bak 경로에 디렉토리를 생성하여 WriteFile이 실패하도록 함
	backupPath := filePath + ".bak"
	if err := os.MkdirAll(backupPath, 0755); err != nil {
		t.Fatalf("백업 경로에 디렉토리 생성 실패: %v", err)
	}

	err := BackupFile(filePath)
	if err == nil {
		t.Error("백업 쓰기 실패 시 BackupFile()이 에러를 반환하지 않았습니다")
	}
}

// TestEnsureDir_디렉토리생성불가는 디렉토리를 생성할 수 없는 경로에서 에러를 반환하는지 검증합니다.
func TestEnsureDir_디렉토리생성불가(t *testing.T) {
	// /dev/null은 파일이므로 하위에 디렉토리를 만들 수 없습니다
	err := EnsureDir("/dev/null/subdir/file.json")
	if err == nil {
		t.Error("디렉토리 생성 불가능한 경로에서 EnsureDir()이 에러를 반환하지 않았습니다")
	}
}

// TestAIToolInfo_JSON직렬화는 AIToolInfo의 JSON 직렬화를 검증합니다.
func TestAIToolInfo_JSON직렬화(t *testing.T) {
	info := AIToolInfo{
		Name:       "Test Tool",
		Installed:  true,
		Version:    "1.0.0",
		CLIPath:    "/usr/bin/test",
		ConfigPath: "/home/user/.test",
		HasAPIKey:  true,
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("JSON 직렬화 실패: %v", err)
	}

	var decoded AIToolInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("JSON 역직렬화 실패: %v", err)
	}

	if decoded.Name != info.Name {
		t.Errorf("Name = %q, want %q", decoded.Name, info.Name)
	}
	if decoded.Installed != info.Installed {
		t.Errorf("Installed = %v, want %v", decoded.Installed, info.Installed)
	}
	if decoded.Version != info.Version {
		t.Errorf("Version = %q, want %q", decoded.Version, info.Version)
	}
	if decoded.HasAPIKey != info.HasAPIKey {
		t.Errorf("HasAPIKey = %v, want %v", decoded.HasAPIKey, info.HasAPIKey)
	}
}
