// Package server는 Bridge 로컬 HTTP 서버 라우트를 관리합니다.
// SPEC-RAGEVO-001: Bridge 리랭커 HTTP API 등록
package server

import (
	"net/http"

	"github.com/insajin/autopus-bridge/internal/reranker"
)

// RegisterRoutes는 mux에 모든 Bridge HTTP 라우트를 등록합니다.
// rerankerSvc가 nil이면 리랭커 라우트를 등록하지 않습니다.
//
// @MX:NOTE: Bridge HTTP 서버의 라우트 진입점
// @MX:SPEC: SPEC-RAGEVO-001 REQ-D AC-D8
func RegisterRoutes(mux *http.ServeMux, rerankerSvc reranker.Service) {
	if rerankerSvc != nil {
		h := reranker.NewHandler(rerankerSvc)
		mux.Handle("POST /api/v1/rerank", h)
	}
}
