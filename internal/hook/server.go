package hook

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

// HookServer hosts a localhost-only HTTP server that Claude Code hook
// scripts (curl) communicate with. It exposes PreToolUse and PostToolUse
// endpoints behind per-session token authentication.
type HookServer struct {
	handler  *HookHandler
	server   *http.Server
	listener net.Listener
	port     int
	token    string // per-session token validated via X-Session-Token header
	logger   zerolog.Logger
}

// HookServerOption configures a HookServer.
type HookServerOption func(*HookServer)

// WithPort sets the port the hook server listens on. When port is 0
// (the default) the OS assigns an available ephemeral port.
func WithPort(port int) HookServerOption {
	return func(s *HookServer) {
		s.port = port
	}
}

// WithLogger sets the logger for the hook server.
func WithLogger(logger zerolog.Logger) HookServerOption {
	return func(s *HookServer) {
		s.logger = logger
	}
}

// NewHookServer creates a HookServer that delegates request handling
// to the given HookHandler and authenticates requests with token.
func NewHookServer(handler *HookHandler, token string, opts ...HookServerOption) *HookServer {
	s := &HookServer{
		handler: handler,
		token:   token,
		logger:  zerolog.Nop(),
	}
	for _, opt := range opts {
		opt(s)
	}
	s.logger = s.logger.With().Str("component", "hook-server").Logger()
	return s
}

// Start begins listening on localhost with the configured (or OS-assigned)
// port and starts serving HTTP requests in a background goroutine.
// The returned port is the actual port the server is bound to.
func (s *HookServer) Start(ctx context.Context) (int, error) {
	mux := http.NewServeMux()

	// Health check endpoint -- no auth required.
	mux.HandleFunc("GET /hooks/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok"}`)
	})

	// Protected routes (PreToolUse / PostToolUse).
	mux.HandleFunc("POST /hooks/pre-tool-use", s.authMiddleware(s.handler.HandlePreToolUse))
	mux.HandleFunc("POST /hooks/post-tool-use", s.authMiddleware(s.handler.HandlePostToolUse))

	addr := fmt.Sprintf("127.0.0.1:%d", s.port)
	var err error
	s.listener, err = net.Listen("tcp", addr)
	if err != nil {
		return 0, fmt.Errorf("hook server: failed to listen on %s: %w", addr, err)
	}

	// Capture the actual assigned port (important when s.port was 0).
	actualPort := s.listener.Addr().(*net.TCPAddr).Port
	s.port = actualPort

	s.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 5 * time.Minute, // PreToolUse may block for approval
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		s.logger.Info().
			Int("port", actualPort).
			Msg("hook HTTP server started on localhost")

		if err := s.server.Serve(s.listener); err != nil && err != http.ErrServerClosed {
			s.logger.Error().Err(err).Msg("hook HTTP server error")
		}
	}()

	return actualPort, nil
}

// Stop performs a graceful shutdown of the HTTP server with a 5-second
// deadline.
func (s *HookServer) Stop() error {
	if s.server == nil {
		return nil
	}

	s.logger.Info().Msg("shutting down hook HTTP server")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.server.Shutdown(ctx)
}

// Port returns the port the server is listening on. Returns 0 if the
// server has not been started yet.
func (s *HookServer) Port() int {
	return s.port
}

// authMiddleware wraps an http.HandlerFunc with X-Session-Token validation.
func (s *HookServer) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		provided := r.Header.Get("X-Session-Token")
		if provided == "" || provided != s.token {
			s.logger.Warn().
				Str("remote_addr", r.RemoteAddr).
				Msg("hook request rejected: invalid or missing session token")
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}
