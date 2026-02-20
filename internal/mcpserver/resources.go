package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// 리소스 캐시 키 상수
const (
	cacheKeyStatus     = "resource:status"
	cacheKeyWorkspaces = "resource:workspaces"
	cacheKeyAgents     = "resource:agents"
)

// PlatformStatus는 플랫폼 상태 정보입니다.
type PlatformStatus struct {
	Connected  bool   `json:"connected"`
	BackendURL string `json:"backend_url"`
	ServerName string `json:"server_name"`
	Version    string `json:"version"`
	Message    string `json:"message,omitempty"`
	Cached     bool   `json:"cached,omitempty"`
	CachedAt   string `json:"cached_at,omitempty"`
}

// CachedResponse는 캐시된 응답을 래핑하는 구조체입니다.
// 원본 데이터에 캐시 메타데이터를 추가하여 반환합니다.
type CachedResponse struct {
	Data     interface{} `json:"data"`
	Cached   bool        `json:"cached"`
	CachedAt string      `json:"cached_at"`
}

// newTextResource는 텍스트 리소스 콘텐츠를 생성하는 헬퍼입니다.
func newTextResource(uri, text, mimeType string) mcp.TextResourceContents {
	return mcp.TextResourceContents{
		URI:      uri,
		MIMEType: mimeType,
		Text:     text,
	}
}

// handleStatusResource는 autopus://status 리소스 핸들러입니다.
// 플랫폼 연결 상태 및 헬스 정보를 반환합니다.
// 백엔드 미연결 시 캐시된 상태 정보를 폴백으로 반환합니다.
func (s *Server) handleStatusResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	status := PlatformStatus{
		ServerName: ServerName,
		Version:    ServerVersion,
		BackendURL: s.client.baseURL,
	}

	// 백엔드 연결 확인 (에이전트 목록 API를 헬스체크로 활용)
	_, err := s.client.ListAgents(ctx, "", "")
	if err != nil {
		s.logger.Warn().Err(err).Msg("백엔드 연결 상태 확인 실패")

		// 캐시에서 폴백 데이터 조회 (만료된 것도 허용)
		if cached, storedAt, ok := s.cache.GetStale(cacheKeyStatus); ok {
			s.logger.Info().Msg("캐시된 상태 정보를 폴백으로 반환")
			cachedStatus, _ := cached.(*PlatformStatus)
			fallback := *cachedStatus
			fallback.Connected = false
			fallback.Message = fmt.Sprintf("Backend unreachable: %s (returning cached data)", err.Error())
			fallback.Cached = true
			fallback.CachedAt = storedAt.Format(time.RFC3339)

			data, marshalErr := json.MarshalIndent(fallback, "", "  ")
			if marshalErr != nil {
				return nil, fmt.Errorf("상태 직렬화 실패: %w", marshalErr)
			}
			return []mcp.ResourceContents{
				newTextResource(request.Params.URI, string(data), "application/json"),
			}, nil
		}

		// 캐시도 없으면 기본 에러 응답
		status.Connected = false
		status.Message = fmt.Sprintf("Backend unreachable: %s", err.Error())
	} else {
		status.Connected = true
		status.Message = "Connected to Autopus backend"

		// 성공 시 캐시에 저장
		s.cache.Set(cacheKeyStatus, &status)
	}

	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("상태 직렬화 실패: %w", err)
	}

	return []mcp.ResourceContents{
		newTextResource(request.Params.URI, string(data), "application/json"),
	}, nil
}

// handleExecutionResource는 autopus://executions/{id} 리소스 핸들러입니다.
// 특정 실행의 상세 정보를 반환합니다.
// 실행 리소스는 항상 최신 데이터가 필요하므로 캐시하지 않습니다.
func (s *Server) handleExecutionResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	// URI에서 execution ID 추출: autopus://executions/{id}
	uri := request.Params.URI
	executionID := extractIDFromURI(uri, "executions")
	if executionID == "" {
		return nil, fmt.Errorf("invalid execution URI: %s", uri)
	}

	s.logger.Debug().
		Str("execution_id", executionID).
		Msg("실행 상세 리소스 조회")

	status, err := s.client.GetExecutionStatus(ctx, executionID)
	if err != nil {
		// Graceful degradation: 오류 시에도 리소스 반환
		errorResp := map[string]string{
			"error":        "Failed to fetch execution details",
			"execution_id": executionID,
			"details":      err.Error(),
		}
		data, _ := json.MarshalIndent(errorResp, "", "  ")
		return []mcp.ResourceContents{
			newTextResource(uri, string(data), "application/json"),
		}, nil
	}

	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("실행 상태 직렬화 실패: %w", err)
	}

	return []mcp.ResourceContents{
		newTextResource(uri, string(data), "application/json"),
	}, nil
}

// handleWorkspacesResource는 autopus://workspaces 리소스 핸들러입니다.
// 접근 가능한 워크스페이스 목록을 반환합니다.
// 백엔드 미연결 시 캐시된 워크스페이스 목록을 폴백으로 반환합니다.
func (s *Server) handleWorkspacesResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	s.logger.Debug().Msg("워크스페이스 목록 리소스 조회")

	resp, err := s.client.ManageWorkspace(ctx, &ManageWorkspaceRequest{
		Action: "list",
	})
	if err != nil {
		s.logger.Warn().Err(err).Msg("워크스페이스 목록 조회 실패")

		// 캐시에서 폴백 데이터 조회 (만료된 것도 허용)
		if cached, storedAt, ok := s.cache.GetStale(cacheKeyWorkspaces); ok {
			s.logger.Info().Msg("캐시된 워크스페이스 목록을 폴백으로 반환")
			cachedResp := &CachedResponse{
				Data:     cached,
				Cached:   true,
				CachedAt: storedAt.Format(time.RFC3339),
			}
			data, _ := json.MarshalIndent(cachedResp, "", "  ")
			return []mcp.ResourceContents{
				newTextResource(request.Params.URI, string(data), "application/json"),
			}, nil
		}

		// 캐시도 없으면 기본 에러 응답
		errorResp := map[string]interface{}{
			"error":      "Failed to fetch workspaces",
			"details":    err.Error(),
			"workspaces": []interface{}{},
		}
		data, _ := json.MarshalIndent(errorResp, "", "  ")
		return []mcp.ResourceContents{
			newTextResource(request.Params.URI, string(data), "application/json"),
		}, nil
	}

	// 성공 시 캐시에 저장
	s.cache.Set(cacheKeyWorkspaces, resp)

	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("워크스페이스 목록 직렬화 실패: %w", err)
	}

	return []mcp.ResourceContents{
		newTextResource(request.Params.URI, string(data), "application/json"),
	}, nil
}

// handleAgentsResource는 autopus://agents 리소스 핸들러입니다.
// 사용 가능한 에이전트 카탈로그를 반환합니다.
// 백엔드 미연결 시 캐시된 에이전트 카탈로그를 폴백으로 반환합니다.
func (s *Server) handleAgentsResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	s.logger.Debug().Msg("에이전트 카탈로그 리소스 조회")

	resp, err := s.client.ListAgents(ctx, "", "")
	if err != nil {
		s.logger.Warn().Err(err).Msg("에이전트 카탈로그 조회 실패")

		// 캐시에서 폴백 데이터 조회 (만료된 것도 허용)
		if cached, storedAt, ok := s.cache.GetStale(cacheKeyAgents); ok {
			s.logger.Info().Msg("캐시된 에이전트 카탈로그를 폴백으로 반환")
			cachedResp := &CachedResponse{
				Data:     cached,
				Cached:   true,
				CachedAt: storedAt.Format(time.RFC3339),
			}
			data, _ := json.MarshalIndent(cachedResp, "", "  ")
			return []mcp.ResourceContents{
				newTextResource(request.Params.URI, string(data), "application/json"),
			}, nil
		}

		// 캐시도 없으면 기본 에러 응답
		errorResp := map[string]interface{}{
			"error":   "Failed to fetch agent catalog",
			"details": err.Error(),
			"agents":  []interface{}{},
		}
		data, _ := json.MarshalIndent(errorResp, "", "  ")
		return []mcp.ResourceContents{
			newTextResource(request.Params.URI, string(data), "application/json"),
		}, nil
	}

	// 성공 시 캐시에 저장
	s.cache.Set(cacheKeyAgents, resp)

	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("에이전트 카탈로그 직렬화 실패: %w", err)
	}

	return []mcp.ResourceContents{
		newTextResource(request.Params.URI, string(data), "application/json"),
	}, nil
}

// extractIDFromURI는 URI에서 리소스 ID를 추출합니다.
// 예: "autopus://executions/abc-123" -> "abc-123"
func extractIDFromURI(uri, resourceType string) string {
	prefix := "autopus://" + resourceType + "/"
	if !strings.HasPrefix(uri, prefix) {
		return ""
	}
	id := strings.TrimPrefix(uri, prefix)
	// 추가 경로 세그먼트 제거
	if idx := strings.Index(id, "/"); idx != -1 {
		id = id[:idx]
	}
	return id
}
