package computeruse

import (
	"context"
	"strings"
	"testing"
)

// --- Constructor Tests ---

func TestNewBrowserManager(t *testing.T) {
	bm := NewBrowserManager(1920, 1080, true)
	if bm == nil {
		t.Fatal("NewBrowserManager() returned nil")
	}
	if bm.viewportW != 1920 {
		t.Errorf("viewportW = %d; want 1920", bm.viewportW)
	}
	if bm.viewportH != 1080 {
		t.Errorf("viewportH = %d; want 1080", bm.viewportH)
	}
	if !bm.headless {
		t.Error("headless = false; want true")
	}
}

func TestNewBrowserManager_HeadedMode(t *testing.T) {
	bm := NewBrowserManager(800, 600, false)
	if bm.headless {
		t.Error("headless = true; want false")
	}
	if bm.viewportW != 800 {
		t.Errorf("viewportW = %d; want 800", bm.viewportW)
	}
	if bm.viewportH != 600 {
		t.Errorf("viewportH = %d; want 600", bm.viewportH)
	}
}

func TestNewBrowserManager_ZeroViewport(t *testing.T) {
	// Constructor does not validate viewport; that is the caller's responsibility.
	bm := NewBrowserManager(0, 0, true)
	if bm == nil {
		t.Fatal("NewBrowserManager(0, 0, true) returned nil")
	}
	if bm.viewportW != 0 || bm.viewportH != 0 {
		t.Errorf("viewport = %dx%d; want 0x0", bm.viewportW, bm.viewportH)
	}
}

// --- IsActive Tests ---

func TestBrowserManager_IsActive_BeforeLaunch(t *testing.T) {
	bm := NewBrowserManager(1280, 720, true)
	if bm.IsActive() {
		t.Error("IsActive() = true before Launch; want false")
	}
}

// --- Pre-Launch Error Tests ---
// All browser operations should return an error when the browser has not been launched.

func TestBrowserManager_Screenshot_BeforeLaunch(t *testing.T) {
	bm := NewBrowserManager(1280, 720, true)
	ctx := context.Background()

	data, err := bm.Screenshot(ctx)
	if err == nil {
		t.Error("Screenshot() before Launch returned nil error; want error")
	}
	if data != nil {
		t.Errorf("Screenshot() before Launch returned data (len=%d); want nil", len(data))
	}
	if !strings.Contains(err.Error(), "not active") {
		t.Errorf("Screenshot() error = %q; want containing 'not active'", err.Error())
	}
}

func TestBrowserManager_Click_BeforeLaunch(t *testing.T) {
	bm := NewBrowserManager(1280, 720, true)
	ctx := context.Background()

	err := bm.Click(ctx, 100, 200)
	if err == nil {
		t.Error("Click() before Launch returned nil error; want error")
	}
	if !strings.Contains(err.Error(), "not active") {
		t.Errorf("Click() error = %q; want containing 'not active'", err.Error())
	}
}

func TestBrowserManager_Type_BeforeLaunch(t *testing.T) {
	bm := NewBrowserManager(1280, 720, true)
	ctx := context.Background()

	err := bm.Type(ctx, "hello")
	if err == nil {
		t.Error("Type() before Launch returned nil error; want error")
	}
	if !strings.Contains(err.Error(), "not active") {
		t.Errorf("Type() error = %q; want containing 'not active'", err.Error())
	}
}

func TestBrowserManager_Scroll_BeforeLaunch(t *testing.T) {
	bm := NewBrowserManager(1280, 720, true)
	ctx := context.Background()

	err := bm.Scroll(ctx, "down", 300)
	if err == nil {
		t.Error("Scroll() before Launch returned nil error; want error")
	}
	if !strings.Contains(err.Error(), "not active") {
		t.Errorf("Scroll() error = %q; want containing 'not active'", err.Error())
	}
}

func TestBrowserManager_Navigate_BeforeLaunch(t *testing.T) {
	bm := NewBrowserManager(1280, 720, true)
	ctx := context.Background()

	err := bm.Navigate(ctx, "http://example.com")
	if err == nil {
		t.Error("Navigate() before Launch returned nil error; want error")
	}
	if !strings.Contains(err.Error(), "not active") {
		t.Errorf("Navigate() error = %q; want containing 'not active'", err.Error())
	}
}

// --- Close Tests ---

func TestBrowserManager_Close_BeforeLaunch(t *testing.T) {
	bm := NewBrowserManager(1280, 720, true)

	// Closing a browser that was never launched should be a no-op.
	err := bm.Close()
	if err != nil {
		t.Errorf("Close() before Launch = error %v; want nil (no-op)", err)
	}
}

func TestBrowserManager_Close_Idempotent(t *testing.T) {
	bm := NewBrowserManager(1280, 720, true)

	// Multiple closes should all succeed without error.
	for i := 0; i < 3; i++ {
		if err := bm.Close(); err != nil {
			t.Errorf("Close() call %d = error %v; want nil", i+1, err)
		}
	}
}

// --- Launch Error Tests ---

func TestBrowserManager_Launch_CancelledContext(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser launch test in short mode")
	}

	bm := NewBrowserManager(1280, 720, true)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately to force launch failure.

	err := bm.Launch(ctx)
	if err == nil {
		// If Launch somehow succeeded, clean up.
		_ = bm.Close()
		t.Skip("Launch succeeded despite cancelled context; Chrome may handle this differently")
	}

	// Browser should not be marked as active after failed launch.
	if bm.IsActive() {
		t.Error("IsActive() = true after failed Launch; want false")
	}
}

func TestBrowserManager_Launch_DoubleLaunch(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser launch test in short mode")
	}

	bm := NewBrowserManager(1280, 720, true)

	// Manually set active to simulate a launched browser without requiring Chrome.
	bm.mu.Lock()
	bm.active = true
	bm.mu.Unlock()

	ctx := context.Background()
	err := bm.Launch(ctx)
	if err == nil {
		t.Error("Launch() on already-active browser = nil; want error")
	}
	if !strings.Contains(err.Error(), "already launched") {
		t.Errorf("Launch() error = %q; want containing 'already launched'", err.Error())
	}
}

// --- Integration Tests (require Chrome) ---

func TestBrowserManager_LaunchAndClose_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser integration test in short mode")
	}

	bm := NewBrowserManager(1280, 720, true)
	ctx := context.Background()

	err := bm.Launch(ctx)
	if err != nil {
		t.Skipf("Skipping integration test: browser launch failed (Chrome may not be installed): %v", err)
	}
	defer func() { _ = bm.Close() }()

	if !bm.IsActive() {
		t.Error("IsActive() = false after successful Launch; want true")
	}

	// Close the browser.
	err = bm.Close()
	if err != nil {
		t.Fatalf("Close() after Launch = error %v; want nil", err)
	}

	if bm.IsActive() {
		t.Error("IsActive() = true after Close; want false")
	}
}

func TestBrowserManager_Navigate_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser integration test in short mode")
	}

	bm := NewBrowserManager(1280, 720, true)
	ctx := context.Background()

	err := bm.Launch(ctx)
	if err != nil {
		t.Skipf("Skipping integration test: browser launch failed: %v", err)
	}
	defer func() { _ = bm.Close() }()

	err = bm.Navigate(ctx, "about:blank")
	if err != nil {
		t.Errorf("Navigate(about:blank) = error %v; want nil", err)
	}
}

func TestBrowserManager_Screenshot_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser integration test in short mode")
	}

	bm := NewBrowserManager(1280, 720, true)
	ctx := context.Background()

	err := bm.Launch(ctx)
	if err != nil {
		t.Skipf("Skipping integration test: browser launch failed: %v", err)
	}
	defer func() { _ = bm.Close() }()

	data, err := bm.Screenshot(ctx)
	if err != nil {
		t.Fatalf("Screenshot() = error %v; want nil", err)
	}
	if len(data) == 0 {
		t.Error("Screenshot() returned empty data; want non-empty PNG bytes")
	}
}
