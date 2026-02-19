package websocket

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestNewNetworkMonitor는 NetworkMonitor 생성을 검증합니다.
func TestNewNetworkMonitor(t *testing.T) {
	client := &Client{}
	monitor := NewNetworkMonitor(client, 5*time.Second)

	if monitor.client != client {
		t.Error("client가 올바르게 설정되지 않았습니다")
	}
	if monitor.checkInterval != 5*time.Second {
		t.Errorf("checkInterval = %v, want %v", monitor.checkInterval, 5*time.Second)
	}
	if monitor.getAddrs == nil {
		t.Error("getAddrs 함수가 nil입니다")
	}
}

// TestNewNetworkMonitor_DefaultInterval은 0 이하 간격에 기본값이 적용되는지 검증합니다.
func TestNewNetworkMonitor_DefaultInterval(t *testing.T) {
	client := &Client{}

	// 0 간격
	monitor := NewNetworkMonitor(client, 0)
	if monitor.checkInterval != DefaultNetworkCheckInterval {
		t.Errorf("0 간격 시 checkInterval = %v, want %v", monitor.checkInterval, DefaultNetworkCheckInterval)
	}

	// 음수 간격
	monitor = NewNetworkMonitor(client, -1*time.Second)
	if monitor.checkInterval != DefaultNetworkCheckInterval {
		t.Errorf("음수 간격 시 checkInterval = %v, want %v", monitor.checkInterval, DefaultNetworkCheckInterval)
	}
}

// TestNetworkMonitor_DetectsChange는 네트워크 주소 변경 시 감지되는지 검증합니다.
func TestNetworkMonitor_DetectsChange(t *testing.T) {
	client := &Client{}
	monitor := NewNetworkMonitor(client, 100*time.Millisecond)

	// 변경된 주소를 반환하도록 설정 (WiFi -> LAN 전환 시뮬레이션)
	monitor.getAddrs = func() ([]string, error) {
		return []string{"192.168.2.200/24", "10.0.0.1/8"}, nil
	}

	// 초기 주소를 수동으로 설정 (이전 네트워크 상태)
	monitor.mu.Lock()
	monitor.lastAddrs = []string{"192.168.1.100/24", "10.0.0.1/8"}
	monitor.mu.Unlock()

	// hasChanged 호출 시 변경 감지
	changed := monitor.hasChanged()
	if !changed {
		t.Error("네트워크 주소 변경이 감지되지 않았습니다")
	}
}

// TestNetworkMonitor_NoChangeNoAction은 주소가 동일할 때 변경이 감지되지 않는지 검증합니다.
func TestNetworkMonitor_NoChangeNoAction(t *testing.T) {
	client := &Client{}
	monitor := NewNetworkMonitor(client, 100*time.Millisecond)

	// 항상 동일한 주소 반환
	monitor.getAddrs = func() ([]string, error) {
		return []string{"192.168.1.100/24", "10.0.0.1/8"}, nil
	}

	// 초기 주소 설정
	monitor.mu.Lock()
	monitor.lastAddrs = []string{"192.168.1.100/24", "10.0.0.1/8"}
	monitor.mu.Unlock()

	// hasChanged 호출 시 변경 없음
	changed := monitor.hasChanged()
	if changed {
		t.Error("주소가 동일한데 변경이 감지되었습니다")
	}
}

// TestNetworkMonitor_StopOnContextCancel은 컨텍스트 취소 시 모니터가 정상 종료되는지 검증합니다.
func TestNetworkMonitor_StopOnContextCancel(t *testing.T) {
	client := &Client{}
	monitor := NewNetworkMonitor(client, 50*time.Millisecond)

	// 항상 동일한 주소 반환 (변경 없음)
	monitor.getAddrs = func() ([]string, error) {
		return []string{"192.168.1.100/24"}, nil
	}

	ctx, cancel := context.WithCancel(context.Background())

	// 모니터 시작
	monitor.Start(ctx)

	// 잠시 대기 후 취소
	time.Sleep(150 * time.Millisecond)
	cancel()

	// 고루틴 종료 대기
	time.Sleep(100 * time.Millisecond)

	// 컨텍스트 취소 후 패닉 없이 정상 종료되었는지 확인
	// (테스트가 여기까지 도달하면 성공)
}

// TestNetworkMonitor_DetectsChangeInLoop은 모니터 루프에서 변경이 감지되는지 검증합니다.
func TestNetworkMonitor_DetectsChangeInLoop(t *testing.T) {
	client := &Client{}
	// done 채널 초기화 (TriggerReconnect에서 State 체크 시 필요)
	client.done = make(chan struct{})
	client.state.Store(int32(StateConnected))
	client.reconnectStrategy = DefaultReconnectStrategy()

	monitor := NewNetworkMonitor(client, 50*time.Millisecond)

	var callCount atomic.Int32
	monitor.getAddrs = func() ([]string, error) {
		count := callCount.Add(1)
		if count <= 2 {
			// Start()에서 초기 호출 + 첫 번째 폴링
			return []string{"192.168.1.100/24"}, nil
		}
		// 두 번째 폴링부터 변경된 주소
		return []string{"192.168.2.200/24"}, nil
	}

	// 변경 감지 콜백 설정
	changeCh := make(chan struct{}, 1)
	monitor.onChangeCallback = func() {
		select {
		case changeCh <- struct{}{}:
		default:
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	monitor.Start(ctx)

	// 변경 감지 대기
	select {
	case <-changeCh:
		// 성공: 변경이 감지되었음
	case <-ctx.Done():
		t.Error("타임아웃: 네트워크 변경이 감지되지 않았습니다")
	}
}

// TestNetworkMonitor_AddressErrorIgnored는 주소 조회 실패 시 변경 없음으로 처리되는지 검증합니다.
func TestNetworkMonitor_AddressErrorIgnored(t *testing.T) {
	client := &Client{}
	monitor := NewNetworkMonitor(client, 100*time.Millisecond)

	// 에러 반환
	monitor.getAddrs = func() ([]string, error) {
		return nil, net.InvalidAddrError("테스트 에러")
	}

	// 초기 주소 설정
	monitor.mu.Lock()
	monitor.lastAddrs = []string{"192.168.1.100/24"}
	monitor.mu.Unlock()

	// 에러 시 변경 없음으로 처리
	changed := monitor.hasChanged()
	if changed {
		t.Error("주소 조회 에러 시 변경으로 감지되었습니다")
	}
}

// TestEqualStringSlices는 문자열 슬라이스 비교 함수를 검증합니다.
func TestEqualStringSlices(t *testing.T) {
	tests := []struct {
		name string
		a    []string
		b    []string
		want bool
	}{
		{
			name: "동일한 슬라이스",
			a:    []string{"a", "b", "c"},
			b:    []string{"a", "b", "c"},
			want: true,
		},
		{
			name: "다른 슬라이스",
			a:    []string{"a", "b", "c"},
			b:    []string{"a", "b", "d"},
			want: false,
		},
		{
			name: "길이가 다른 슬라이스",
			a:    []string{"a", "b"},
			b:    []string{"a", "b", "c"},
			want: false,
		},
		{
			name: "빈 슬라이스 비교",
			a:    []string{},
			b:    []string{},
			want: true,
		},
		{
			name: "nil과 빈 슬라이스",
			a:    nil,
			b:    []string{},
			want: true,
		},
		{
			name: "nil과 nil",
			a:    nil,
			b:    nil,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := equalStringSlices(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("equalStringSlices(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// TestDefaultGetInterfaceAddrs는 기본 주소 조회 함수가 정상 동작하는지 검증합니다.
func TestDefaultGetInterfaceAddrs(t *testing.T) {
	addrs, err := defaultGetInterfaceAddrs()
	if err != nil {
		t.Fatalf("defaultGetInterfaceAddrs() 에러: %v", err)
	}

	// 루프백 주소가 제외되었는지 확인
	for _, addr := range addrs {
		if addr == "127.0.0.1/8" || addr == "::1/128" {
			t.Errorf("루프백 주소가 포함되어 있습니다: %s", addr)
		}
	}

	// 정렬되었는지 확인
	for i := 1; i < len(addrs); i++ {
		if addrs[i] < addrs[i-1] {
			t.Errorf("주소가 정렬되지 않았습니다: %v", addrs)
			break
		}
	}
}

// TestNetworkMonitor_ConcurrentAccess는 동시 접근 안전성을 검증합니다.
func TestNetworkMonitor_ConcurrentAccess(t *testing.T) {
	client := &Client{}
	monitor := NewNetworkMonitor(client, 100*time.Millisecond)

	var toggle atomic.Bool
	monitor.getAddrs = func() ([]string, error) {
		if toggle.Load() {
			return []string{"192.168.1.100/24"}, nil
		}
		return []string{"10.0.0.1/8"}, nil
	}

	monitor.mu.Lock()
	monitor.lastAddrs = []string{"192.168.1.100/24"}
	monitor.mu.Unlock()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				toggle.Store(!toggle.Load())
				_ = monitor.hasChanged()
			}
		}()
	}

	wg.Wait()
	// 패닉 없이 완료되면 성공
}
