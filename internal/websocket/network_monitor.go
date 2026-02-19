// Package websocket는 Local Agent Bridge의 WebSocket 통신을 담당합니다.
// network_monitor.go는 네트워크 인터페이스 변경을 감지하여 WebSocket 재연결을 트리거합니다.
// FR-P2-03: 네트워크 변경 감지 및 자동 재연결
package websocket

import (
	"context"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/insajin/autopus-bridge/internal/logger"
)

// DefaultNetworkCheckInterval은 네트워크 변경 감지 기본 폴링 간격입니다.
const DefaultNetworkCheckInterval = 5 * time.Second

// PingTimeout은 WebSocket ping 응답 대기 시간입니다.
const PingTimeout = 5 * time.Second

// NetworkMonitor는 네트워크 인터페이스 변경을 감지하여 WebSocket 재연결을 트리거합니다 (FR-P2-03).
// 주기적으로 net.InterfaceAddrs()를 폴링하여 네트워크 주소 변경을 감지하고,
// 변경이 감지되면 WebSocket 연결의 유효성을 검증한 후 재연결을 수행합니다.
type NetworkMonitor struct {
	// client는 WebSocket 클라이언트입니다.
	client *Client
	// checkInterval은 네트워크 변경 감지 폴링 간격입니다.
	checkInterval time.Duration

	// lastAddrs는 마지막으로 확인된 네트워크 주소 목록입니다.
	lastAddrs []string
	// mu는 lastAddrs 접근을 보호하는 뮤텍스입니다.
	mu sync.Mutex

	// getAddrs는 네트워크 주소를 조회하는 함수입니다.
	// 테스트에서 주입 가능하도록 함수 필드로 정의합니다.
	getAddrs func() ([]string, error)

	// onChangeCallback은 네트워크 변경 감지 시 호출되는 콜백입니다 (테스트용).
	onChangeCallback func()
}

// NewNetworkMonitor는 새로운 NetworkMonitor를 생성합니다.
// client는 WebSocket 클라이언트, interval은 폴링 간격입니다.
func NewNetworkMonitor(client *Client, interval time.Duration) *NetworkMonitor {
	if interval <= 0 {
		interval = DefaultNetworkCheckInterval
	}

	m := &NetworkMonitor{
		client:        client,
		checkInterval: interval,
		getAddrs:      defaultGetInterfaceAddrs,
	}

	return m
}

// Start는 네트워크 모니터링 고루틴을 시작합니다.
// 전달된 컨텍스트가 취소되면 모니터링을 중지합니다.
func (m *NetworkMonitor) Start(ctx context.Context) {
	// 현재 주소를 초기값으로 설정
	addrs, err := m.getAddrs()
	if err != nil {
		logger.Warn().
			Err(err).
			Msg("네트워크 주소 초기 조회 실패, 빈 상태로 시작합니다")
	}

	m.mu.Lock()
	m.lastAddrs = addrs
	m.mu.Unlock()

	logger.Info().
		Int("addr_count", len(addrs)).
		Dur("interval", m.checkInterval).
		Msg("네트워크 변경 감지 모니터 시작 (FR-P2-03)")

	go m.monitorLoop(ctx)
}

// monitorLoop는 주기적으로 네트워크 변경을 감지하는 루프입니다.
func (m *NetworkMonitor) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Debug().Msg("네트워크 모니터 종료 (컨텍스트 취소)")
			return
		case <-ticker.C:
			if m.hasChanged() {
				logger.Info().Msg("네트워크 인터페이스 변경 감지됨")

				// 콜백 호출 (테스트용)
				if m.onChangeCallback != nil {
					m.onChangeCallback()
				}

				// WebSocket 연결 유효성 검증
				if !m.validateConnection() {
					logger.Warn().Msg("WebSocket 연결이 유효하지 않음, 재연결 트리거")
					m.client.TriggerReconnect(ctx, "네트워크 변경 감지 (FR-P2-03)")
					return
				}

				logger.Info().Msg("네트워크 변경 감지되었으나 WebSocket 연결은 유효함")
			}
		}
	}
}

// hasChanged는 네트워크 인터페이스 주소가 변경되었는지 확인합니다.
// 현재 주소 목록을 이전 목록과 비교하여 변경 여부를 반환합니다.
func (m *NetworkMonitor) hasChanged() bool {
	currentAddrs, err := m.getAddrs()
	if err != nil {
		logger.Debug().
			Err(err).
			Msg("네트워크 주소 조회 실패, 변경 없음으로 처리")
		return false
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	changed := !equalStringSlices(m.lastAddrs, currentAddrs)
	if changed {
		logger.Debug().
			Strs("prev_addrs", m.lastAddrs).
			Strs("curr_addrs", currentAddrs).
			Msg("네트워크 주소 변경 상세")
	}

	// 항상 최신 주소로 갱신
	m.lastAddrs = currentAddrs

	return changed
}

// validateConnection은 WebSocket 연결이 유효한지 ping으로 검증합니다.
// ping 전송에 성공하면 true, 실패하면 false를 반환합니다.
func (m *NetworkMonitor) validateConnection() bool {
	if m.client.State() != StateConnected {
		return false
	}

	err := m.client.Ping()
	if err != nil {
		logger.Debug().
			Err(err).
			Msg("WebSocket ping 실패")
		return false
	}

	return true
}

// defaultGetInterfaceAddrs는 시스템의 네트워크 인터페이스 주소를 조회합니다.
// 결과는 정렬된 문자열 슬라이스로 반환됩니다.
func defaultGetInterfaceAddrs() ([]string, error) {
	ifaces, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}

	addrs := make([]string, 0, len(ifaces))
	for _, addr := range ifaces {
		// 루프백 주소(127.0.0.1, ::1) 제외
		addrStr := addr.String()
		if strings.HasPrefix(addrStr, "127.") || strings.HasPrefix(addrStr, "::1") {
			continue
		}
		addrs = append(addrs, addrStr)
	}

	// 결정적 비교를 위해 정렬
	sort.Strings(addrs)

	return addrs, nil
}

// equalStringSlices는 두 문자열 슬라이스가 동일한지 비교합니다.
// 두 슬라이스 모두 정렬되어 있다고 가정합니다.
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Ping은 WebSocket 서버에 ping 메시지를 전송하여 연결 유효성을 검증합니다.
// FR-P2-03: 네트워크 변경 시 연결 검증에 사용됩니다.
func (c *Client) Ping() error {
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()

	if conn == nil {
		return net.ErrClosed
	}

	return conn.WriteControl(
		websocket.PingMessage,
		[]byte("ping"),
		time.Now().Add(PingTimeout),
	)
}

// TriggerReconnect는 외부 컴포넌트에서 재연결을 트리거합니다.
// FR-P2-03: 네트워크 변경 감지 시 호출됩니다.
// 재연결 전략을 초기화하고 handleDisconnect를 호출합니다.
func (c *Client) TriggerReconnect(ctx context.Context, reason string) {
	if c.State() == StateClosed || c.State() == StateReconnecting {
		return
	}

	logger.Info().
		Str("reason", reason).
		Msg("외부 재연결 트리거 수신")

	// 재연결 전략 초기화 (네트워크 변경은 일시적 장애가 아니므로 카운터 리셋)
	c.reconnectStrategy.Reset()

	go c.handleDisconnect(ctx, reason)
}
