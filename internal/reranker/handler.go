package reranker

import (
	"encoding/json"
	"net/http"
)

// errorResponse는 에러 응답 구조체입니다.
type errorResponse struct {
	Error string `json:"error"`
}

// Handler는 리랭커 HTTP 핸들러입니다.
// POST /api/v1/rerank 엔드포인트를 처리합니다.
// @MX:ANCHOR: 리랭커 HTTP 진입점 — Bridge HTTP 서버에서 라우트 등록
// @MX:REASON: server/routes.go에서 참조
// @MX:SPEC: SPEC-RAGEVO-001 REQ-D AC-D8
type Handler struct {
	svc Service
}

// NewHandler는 Handler를 생성합니다.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// ServeHTTP는 http.Handler 인터페이스를 구현합니다.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "POST 메서드만 허용됩니다"})
		return
	}

	if !h.svc.IsAvailable() {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "리랭커 서비스를 사용할 수 없습니다"})
		return
	}

	var req RerankRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "요청 파싱 실패: " + err.Error()})
		return
	}

	if req.Query == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "query 필드가 필요합니다"})
		return
	}

	resp, err := h.svc.Rerank(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "리랭킹 실패: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// writeJSON은 JSON 응답을 작성합니다.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}
