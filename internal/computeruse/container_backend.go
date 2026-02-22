package computeruse

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

// 컴파일 타임 인터페이스 구현 확인
var _ BrowserBackend = (*ContainerBrowserBackend)(nil)

// ContainerBrowserBackend은 컨테이너화된 Chromium에 원격 CDP로 연결하는 BrowserBackend 구현체이다.
type ContainerBrowserBackend struct {
	container   *ContainerInfo
	allocCtx    context.Context
	allocCancel context.CancelFunc
	taskCtx     context.Context
	taskCancel  context.CancelFunc
	viewportW   int
	viewportH   int
	active      bool
	mu          sync.Mutex
}

// NewContainerBrowserBackend은 컨테이너 정보와 뷰포트 설정으로 새 ContainerBrowserBackend을 생성한다.
func NewContainerBrowserBackend(container *ContainerInfo, viewportW, viewportH int) *ContainerBrowserBackend {
	return &ContainerBrowserBackend{
		container: container,
		viewportW: viewportW,
		viewportH: viewportH,
	}
}

// Launch는 컨테이너의 CDP 엔드포인트에 원격 연결을 설정한다.
// 연결 실패 시 지수 백오프로 최대 3회 재시도한다.
func (cb *ContainerBrowserBackend) Launch(ctx context.Context) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.active {
		return fmt.Errorf("browser is already launched")
	}

	cdpURL := fmt.Sprintf("ws://127.0.0.1:%s", cb.container.HostPort)

	// 지수 백오프로 CDP 연결 재시도 (1초, 2초, 4초)
	var lastErr error
	backoffs := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}

	for attempt, backoff := range backoffs {
		log.Printf("[computer-use] CDP 연결 시도 %d/3: %s", attempt+1, cdpURL)

		// 원격 할당자 생성
		allocCtx, allocCancel := chromedp.NewRemoteAllocator(ctx, cdpURL)

		// 브라우저 탭 컨텍스트 생성
		taskCtx, taskCancel := chromedp.NewContext(allocCtx)

		// 연결 확인 (about:blank 이동)
		if err := chromedp.Run(taskCtx, chromedp.Navigate("about:blank")); err != nil {
			taskCancel()
			allocCancel()
			lastErr = err

			if attempt < len(backoffs)-1 {
				log.Printf("[computer-use] CDP 연결 실패, %v 후 재시도: %v", backoff, err)
				select {
				case <-ctx.Done():
					return fmt.Errorf("CDP 연결 중 컨텍스트 취소: %w", ctx.Err())
				case <-time.After(backoff):
					continue
				}
			}
			continue
		}

		// 뷰포트 설정
		if err := chromedp.Run(taskCtx, chromedp.EmulateViewport(int64(cb.viewportW), int64(cb.viewportH))); err != nil {
			log.Printf("[computer-use] warning: 뷰포트 설정 실패: %v", err)
		}

		cb.allocCtx = allocCtx
		cb.allocCancel = allocCancel
		cb.taskCtx = taskCtx
		cb.taskCancel = taskCancel
		cb.active = true

		log.Printf("[computer-use] 컨테이너 브라우저 연결 완료: container=%s, port=%s, viewport=%dx%d",
			cb.container.ID[:min(12, len(cb.container.ID))], cb.container.HostPort, cb.viewportW, cb.viewportH)
		return nil
	}

	return fmt.Errorf("CDP 연결 실패 (3회 시도 후): %w", lastErr)
}

// Close는 원격 CDP 연결을 종료한다.
func (cb *ContainerBrowserBackend) Close() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if !cb.active {
		return nil
	}

	cb.active = false

	// 컨텍스트 역순 정리
	if cb.taskCancel != nil {
		cb.taskCancel()
	}
	if cb.allocCancel != nil {
		cb.allocCancel()
	}

	log.Printf("[computer-use] 컨테이너 브라우저 연결 종료")
	return nil
}

// IsActive는 원격 브라우저 연결이 활성 상태인지 반환한다.
func (cb *ContainerBrowserBackend) IsActive() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.active
}

// Screenshot은 전체 페이지 스크린샷을 PNG 바이트로 캡처한다.
func (cb *ContainerBrowserBackend) Screenshot(ctx context.Context) ([]byte, error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if !cb.active {
		return nil, fmt.Errorf("browser is not active")
	}

	var buf []byte
	if err := chromedp.Run(cb.taskCtx, chromedp.CaptureScreenshot(&buf)); err != nil {
		return nil, fmt.Errorf("failed to capture screenshot: %w", err)
	}

	return buf, nil
}

// Navigate는 지정된 URL로 이동한다.
func (cb *ContainerBrowserBackend) Navigate(ctx context.Context, url string) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if !cb.active {
		return fmt.Errorf("browser is not active")
	}

	if err := chromedp.Run(cb.taskCtx, chromedp.Navigate(url)); err != nil {
		return fmt.Errorf("failed to navigate to %s: %w", url, err)
	}

	return nil
}

// Click은 지정된 x, y 좌표에서 마우스 클릭을 수행한다.
func (cb *ContainerBrowserBackend) Click(ctx context.Context, x, y float64) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if !cb.active {
		return fmt.Errorf("browser is not active")
	}

	if err := chromedp.Run(cb.taskCtx, chromedp.MouseClickXY(x, y)); err != nil {
		return fmt.Errorf("failed to click at (%.0f, %.0f): %w", x, y, err)
	}

	return nil
}

// Type은 현재 포커스된 요소에 키보드 입력을 전송한다.
func (cb *ContainerBrowserBackend) Type(ctx context.Context, text string) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if !cb.active {
		return fmt.Errorf("browser is not active")
	}

	if err := chromedp.Run(cb.taskCtx, chromedp.KeyEvent(text)); err != nil {
		return fmt.Errorf("failed to type text: %w", err)
	}

	return nil
}

// Scroll은 지정된 방향과 양만큼 페이지를 스크롤한다.
func (cb *ContainerBrowserBackend) Scroll(ctx context.Context, direction string, amount int) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if !cb.active {
		return fmt.Errorf("browser is not active")
	}

	deltaY := amount
	if direction == "up" {
		deltaY = -amount
	}

	scrollJS := fmt.Sprintf("window.scrollBy(0, %d)", deltaY)
	if err := chromedp.Run(cb.taskCtx, chromedp.Evaluate(scrollJS, nil)); err != nil {
		return fmt.Errorf("failed to scroll %s by %d: %w", direction, amount, err)
	}

	return nil
}
