package computeruse

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/chromedp/chromedp"
)

// BrowserManager manages a single Chrome DevTools Protocol browser instance
// for interactive computer use sessions.
type BrowserManager struct {
	viewportW int
	viewportH int
	headless  bool

	// allocCtx and allocCancel control the browser process lifecycle.
	allocCtx    context.Context
	allocCancel context.CancelFunc

	// taskCtx and taskCancel control the browser tab/target lifecycle.
	taskCtx    context.Context
	taskCancel context.CancelFunc

	active bool
	mu     sync.Mutex
}

// NewBrowserManager creates a new BrowserManager with the given viewport and headless settings.
func NewBrowserManager(viewportW, viewportH int, headless bool) *BrowserManager {
	return &BrowserManager{
		viewportW: viewportW,
		viewportH: viewportH,
		headless:  headless,
	}
}

// Launch starts a Chrome browser instance via CDP.
func (bm *BrowserManager) Launch(ctx context.Context) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if bm.active {
		return fmt.Errorf("browser is already launched")
	}

	// Configure browser options.
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.WindowSize(bm.viewportW, bm.viewportH),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
		chromedp.Flag("disable-extensions", true),
	)

	if bm.headless {
		opts = append(opts, chromedp.Headless)
	} else {
		// Remove the default headless flag for headed mode.
		opts = append(opts, chromedp.Flag("headless", false))
	}

	// Create the browser allocator context (manages the Chrome process).
	bm.allocCtx, bm.allocCancel = chromedp.NewExecAllocator(ctx, opts...)

	// Create the task context (manages the browser tab).
	bm.taskCtx, bm.taskCancel = chromedp.NewContext(bm.allocCtx)

	// Navigate to about:blank to ensure the browser is fully started.
	if err := chromedp.Run(bm.taskCtx, chromedp.Navigate("about:blank")); err != nil {
		bm.allocCancel()
		return fmt.Errorf("failed to launch browser: %w", err)
	}

	// Set viewport emulation to match the requested dimensions.
	if err := chromedp.Run(bm.taskCtx, chromedp.EmulateViewport(int64(bm.viewportW), int64(bm.viewportH))); err != nil {
		log.Printf("[computer-use] warning: failed to set viewport: %v", err)
	}

	bm.active = true
	log.Printf("[computer-use] browser launched (viewport=%dx%d, headless=%v)", bm.viewportW, bm.viewportH, bm.headless)

	return nil
}

// Close terminates the browser process and releases resources.
func (bm *BrowserManager) Close() error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if !bm.active {
		return nil
	}

	bm.active = false

	// Cancel contexts in reverse order.
	if bm.taskCancel != nil {
		bm.taskCancel()
	}
	if bm.allocCancel != nil {
		bm.allocCancel()
	}

	log.Printf("[computer-use] browser closed")
	return nil
}

// IsActive returns whether the browser is currently running.
func (bm *BrowserManager) IsActive() bool {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	return bm.active
}

// Screenshot captures a full-page screenshot as PNG bytes.
// REQ-M2-03: Screenshot in PNG base64.
func (bm *BrowserManager) Screenshot(ctx context.Context) ([]byte, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if !bm.active {
		return nil, fmt.Errorf("browser is not active")
	}

	var buf []byte
	if err := chromedp.Run(bm.taskCtx, chromedp.CaptureScreenshot(&buf)); err != nil {
		return nil, fmt.Errorf("failed to capture screenshot: %w", err)
	}

	return buf, nil
}

// Navigate navigates the browser to the specified URL.
// REQ-M2-07: Navigate to URL.
func (bm *BrowserManager) Navigate(ctx context.Context, url string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if !bm.active {
		return fmt.Errorf("browser is not active")
	}

	if err := chromedp.Run(bm.taskCtx, chromedp.Navigate(url)); err != nil {
		return fmt.Errorf("failed to navigate to %s: %w", url, err)
	}

	return nil
}

// Click performs a mouse click at the specified x,y coordinates.
// REQ-M2-04: Click at x,y coordinates.
func (bm *BrowserManager) Click(ctx context.Context, x, y float64) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if !bm.active {
		return fmt.Errorf("browser is not active")
	}

	if err := chromedp.Run(bm.taskCtx, chromedp.MouseClickXY(x, y)); err != nil {
		return fmt.Errorf("failed to click at (%.0f, %.0f): %w", x, y, err)
	}

	return nil
}

// Type sends keyboard input text to the currently focused element.
// REQ-M2-05: Type text.
func (bm *BrowserManager) Type(ctx context.Context, text string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if !bm.active {
		return fmt.Errorf("browser is not active")
	}

	if err := chromedp.Run(bm.taskCtx, chromedp.KeyEvent(text)); err != nil {
		return fmt.Errorf("failed to type text: %w", err)
	}

	return nil
}

// Scroll scrolls the page in the given direction by the given amount in pixels.
// REQ-M2-06: Scroll up/down.
func (bm *BrowserManager) Scroll(ctx context.Context, direction string, amount int) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if !bm.active {
		return fmt.Errorf("browser is not active")
	}

	// Convert direction to pixel delta.
	deltaY := amount
	if direction == "up" {
		deltaY = -amount
	}

	// Use JavaScript to scroll since chromedp doesn't have a direct scroll action.
	scrollJS := fmt.Sprintf("window.scrollBy(0, %d)", deltaY)
	if err := chromedp.Run(bm.taskCtx, chromedp.Evaluate(scrollJS, nil)); err != nil {
		return fmt.Errorf("failed to scroll %s by %d: %w", direction, amount, err)
	}

	return nil
}
