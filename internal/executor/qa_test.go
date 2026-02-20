package executor

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/insajin/autopus-agent-protocol"
)

// ====================
// QAPipelineExecutor tests
// ====================

func TestNewQAPipelineExecutor(t *testing.T) {
	e := NewQAPipelineExecutor()
	if e == nil {
		t.Fatal("NewQAPipelineExecutor returned nil")
	}
}

func TestQAExecutor_FullPipeline_Success(t *testing.T) {
	workDir := t.TempDir()

	// Create a fake test script for browser QA.
	scriptPath := filepath.Join(workDir, "test.spec.ts")
	if err := os.WriteFile(scriptPath, []byte("// test"), 0644); err != nil {
		t.Fatalf("failed to create test script: %v", err)
	}

	// Start a health check server.
	healthServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer healthServer.Close()

	e := NewQAPipelineExecutor()
	ctx := context.Background()

	req := ws.QARequestPayload{
		ExecutionID:  "qa-full-001",
		WorkDir:      workDir,
		BuildCommand: "echo build-ok",
		ServiceConfig: &ws.ServiceConfig{
			Command:      "sleep 30",
			HealthCheck:  healthServer.URL,
			ReadyTimeout: 5,
		},
		TestCommand: "echo test-ok",
		Timeout:     30,
	}

	result := e.Execute(ctx, req)

	if result.ExecutionID != req.ExecutionID {
		t.Errorf("ExecutionID mismatch: got %s, want %s", result.ExecutionID, req.ExecutionID)
	}

	if !result.Success {
		t.Errorf("expected pipeline to succeed, got failure")
		for _, stage := range result.Stages {
			if !stage.Success {
				t.Logf("  failed stage %q: error=%s, output=%s", stage.Name, stage.Error, stage.Output)
			}
		}
	}

	if result.DurationMs <= 0 {
		t.Error("expected positive duration")
	}

	// Verify stages: build, service_start, test, cleanup.
	expectedStages := []string{stageBuild, stageService, stageTest, stageCleanup}
	if len(result.Stages) != len(expectedStages) {
		t.Fatalf("expected %d stages, got %d", len(expectedStages), len(result.Stages))
	}
	for i, name := range expectedStages {
		if result.Stages[i].Name != name {
			t.Errorf("stage[%d] name mismatch: got %s, want %s", i, result.Stages[i].Name, name)
		}
	}
}

func TestQAExecutor_BuildOnly_Success(t *testing.T) {
	workDir := t.TempDir()

	e := NewQAPipelineExecutor()
	ctx := context.Background()

	req := ws.QARequestPayload{
		ExecutionID:  "qa-build-001",
		WorkDir:      workDir,
		BuildCommand: "echo build-success",
		Timeout:      10,
	}

	result := e.Execute(ctx, req)

	if !result.Success {
		t.Error("expected build-only pipeline to succeed")
	}

	// Stages: build + cleanup.
	if len(result.Stages) != 2 {
		t.Fatalf("expected 2 stages, got %d", len(result.Stages))
	}
	if result.Stages[0].Name != stageBuild {
		t.Errorf("expected first stage to be %q, got %q", stageBuild, result.Stages[0].Name)
	}
	if result.Stages[1].Name != stageCleanup {
		t.Errorf("expected second stage to be %q, got %q", stageCleanup, result.Stages[1].Name)
	}
	if result.Stages[0].Output == "" {
		t.Error("expected build stage to have output")
	}
}

func TestQAExecutor_ServiceAndTest_Success(t *testing.T) {
	workDir := t.TempDir()

	healthServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer healthServer.Close()

	e := NewQAPipelineExecutor()
	ctx := context.Background()

	req := ws.QARequestPayload{
		ExecutionID: "qa-svc-test-001",
		WorkDir:     workDir,
		ServiceConfig: &ws.ServiceConfig{
			Command:      "sleep 30",
			HealthCheck:  healthServer.URL,
			ReadyTimeout: 5,
		},
		TestCommand: "echo tests-passed",
		Timeout:     30,
	}

	result := e.Execute(ctx, req)

	if !result.Success {
		t.Error("expected service+test pipeline to succeed")
		for _, stage := range result.Stages {
			if !stage.Success {
				t.Logf("  failed stage %q: error=%s", stage.Name, stage.Error)
			}
		}
	}

	// Stages: service_start + test + cleanup.
	expectedStages := []string{stageService, stageTest, stageCleanup}
	if len(result.Stages) != len(expectedStages) {
		t.Fatalf("expected %d stages, got %d", len(expectedStages), len(result.Stages))
	}
	for i, name := range expectedStages {
		if result.Stages[i].Name != name {
			t.Errorf("stage[%d] name mismatch: got %s, want %s", i, result.Stages[i].Name, name)
		}
	}
}

func TestQAExecutor_StageFailure_SkipsRemaining(t *testing.T) {
	workDir := t.TempDir()

	e := NewQAPipelineExecutor()
	ctx := context.Background()

	// Build will fail, so test should be skipped.
	req := ws.QARequestPayload{
		ExecutionID:  "qa-fail-001",
		WorkDir:      workDir,
		BuildCommand: "exit 1",
		TestCommand:  "echo should-not-run",
		Timeout:      10,
	}

	result := e.Execute(ctx, req)

	if result.Success {
		t.Error("expected pipeline to fail when build fails")
	}

	// Stages: build (failed) + cleanup only. Test should be skipped.
	if len(result.Stages) != 2 {
		t.Fatalf("expected 2 stages (build + cleanup), got %d", len(result.Stages))
	}

	if result.Stages[0].Name != stageBuild {
		t.Errorf("expected first stage to be %q, got %q", stageBuild, result.Stages[0].Name)
	}
	if result.Stages[0].Success {
		t.Error("expected build stage to fail")
	}
	if result.Stages[0].Error == "" {
		t.Error("expected build stage to have an error message")
	}

	// Cleanup should still run.
	if result.Stages[1].Name != stageCleanup {
		t.Errorf("expected second stage to be %q, got %q", stageCleanup, result.Stages[1].Name)
	}
	if !result.Stages[1].Success {
		t.Error("expected cleanup stage to succeed")
	}
}

func TestQAExecutor_ServiceHealthCheck_Polling(t *testing.T) {
	workDir := t.TempDir()

	// Health check server that fails the first few requests then succeeds.
	var requestCount atomic.Int32
	healthServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		if requestCount.Load() < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer healthServer.Close()

	e := NewQAPipelineExecutor()
	ctx := context.Background()

	req := ws.QARequestPayload{
		ExecutionID: "qa-health-001",
		WorkDir:     workDir,
		ServiceConfig: &ws.ServiceConfig{
			Command:      "sleep 30",
			HealthCheck:  healthServer.URL,
			ReadyTimeout: 10,
		},
		Timeout: 30,
	}

	result := e.Execute(ctx, req)

	if !result.Success {
		t.Error("expected pipeline to succeed after health check polling")
		for _, stage := range result.Stages {
			if !stage.Success {
				t.Logf("  failed stage %q: error=%s", stage.Name, stage.Error)
			}
		}
	}

	// Verify health check was polled multiple times.
	if requestCount.Load() < 3 {
		t.Errorf("expected at least 3 health check requests, got %d", requestCount.Load())
	}
}

func TestQAExecutor_ServiceHealthCheck_Timeout(t *testing.T) {
	workDir := t.TempDir()

	// Health check server that always fails.
	healthServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer healthServer.Close()

	e := NewQAPipelineExecutor()
	ctx := context.Background()

	req := ws.QARequestPayload{
		ExecutionID: "qa-health-timeout-001",
		WorkDir:     workDir,
		ServiceConfig: &ws.ServiceConfig{
			Command:      "sleep 30",
			HealthCheck:  healthServer.URL,
			ReadyTimeout: 2, // 2 second timeout to keep test fast.
		},
		TestCommand: "echo should-not-run",
		Timeout:     10,
	}

	result := e.Execute(ctx, req)

	if result.Success {
		t.Error("expected pipeline to fail when health check times out")
	}

	// Service stage should fail, test should be skipped.
	foundService := false
	for _, stage := range result.Stages {
		if stage.Name == stageService {
			foundService = true
			if stage.Success {
				t.Error("expected service stage to fail")
			}
			if stage.Error == "" {
				t.Error("expected service stage to have an error message")
			}
		}
		if stage.Name == stageTest {
			t.Error("test stage should have been skipped due to service failure")
		}
	}
	if !foundService {
		t.Error("expected to find service stage in results")
	}
}

func TestQAExecutor_OverallTimeout(t *testing.T) {
	workDir := t.TempDir()

	e := NewQAPipelineExecutor()
	ctx := context.Background()

	// Build command sleeps longer than the overall timeout.
	req := ws.QARequestPayload{
		ExecutionID:  "qa-timeout-001",
		WorkDir:      workDir,
		BuildCommand: "sleep 30",
		Timeout:      1, // 1 second overall timeout.
	}

	result := e.Execute(ctx, req)

	if result.Success {
		t.Error("expected pipeline to fail on timeout")
	}

	// Build stage should fail due to context deadline exceeded.
	if len(result.Stages) < 1 {
		t.Fatal("expected at least one stage")
	}
	if result.Stages[0].Name != stageBuild {
		t.Errorf("expected first stage to be %q, got %q", stageBuild, result.Stages[0].Name)
	}
	if result.Stages[0].Success {
		t.Error("expected build stage to fail on timeout")
	}
}

func TestQAExecutor_CleanupOnFailure(t *testing.T) {
	workDir := t.TempDir()

	healthServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer healthServer.Close()

	e := NewQAPipelineExecutor()
	ctx := context.Background()

	// Service starts but test fails.
	req := ws.QARequestPayload{
		ExecutionID: "qa-cleanup-001",
		WorkDir:     workDir,
		ServiceConfig: &ws.ServiceConfig{
			Command:      "sleep 30",
			HealthCheck:  healthServer.URL,
			ReadyTimeout: 5,
		},
		TestCommand: "exit 1",
		Timeout:     30,
	}

	result := e.Execute(ctx, req)

	if result.Success {
		t.Error("expected pipeline to fail when test fails")
	}

	// Cleanup stage should still run and succeed.
	lastStage := result.Stages[len(result.Stages)-1]
	if lastStage.Name != stageCleanup {
		t.Errorf("expected last stage to be %q, got %q", stageCleanup, lastStage.Name)
	}
	if !lastStage.Success {
		t.Error("expected cleanup stage to succeed")
	}
	if lastStage.Output == "" {
		t.Error("expected cleanup stage to have output")
	}
}

func TestQAExecutor_InvalidWorkDir(t *testing.T) {
	e := NewQAPipelineExecutor()
	ctx := context.Background()

	req := ws.QARequestPayload{
		ExecutionID:  "qa-invalid-dir-001",
		WorkDir:      "/nonexistent/path/should/not/exist",
		BuildCommand: "echo hello",
		Timeout:      10,
	}

	result := e.Execute(ctx, req)

	if result.Success {
		t.Error("expected pipeline to fail with invalid work directory")
	}

	if len(result.Stages) < 1 {
		t.Fatal("expected at least one stage (validation)")
	}
	if result.Stages[0].Success {
		t.Error("expected validation stage to fail")
	}
}

func TestQAExecutor_EmptyWorkDir(t *testing.T) {
	e := NewQAPipelineExecutor()
	ctx := context.Background()

	req := ws.QARequestPayload{
		ExecutionID:  "qa-empty-dir-001",
		WorkDir:      "",
		BuildCommand: "echo hello",
		Timeout:      10,
	}

	result := e.Execute(ctx, req)

	if result.Success {
		t.Error("expected pipeline to fail with empty work directory")
	}
}

func TestQAExecutor_NoStages(t *testing.T) {
	workDir := t.TempDir()

	e := NewQAPipelineExecutor()
	ctx := context.Background()

	// No build, no service, no test, no browser QA - only cleanup runs.
	req := ws.QARequestPayload{
		ExecutionID: "qa-empty-001",
		WorkDir:     workDir,
		Timeout:     10,
	}

	result := e.Execute(ctx, req)

	if !result.Success {
		t.Error("expected empty pipeline to succeed")
	}

	// Only cleanup stage.
	if len(result.Stages) != 1 {
		t.Fatalf("expected 1 stage (cleanup), got %d", len(result.Stages))
	}
	if result.Stages[0].Name != stageCleanup {
		t.Errorf("expected stage to be %q, got %q", stageCleanup, result.Stages[0].Name)
	}
}

func TestQAExecutor_DefaultTimeout(t *testing.T) {
	workDir := t.TempDir()

	e := NewQAPipelineExecutor()
	ctx := context.Background()

	// Timeout of 0 should use DefaultTimeout.
	req := ws.QARequestPayload{
		ExecutionID:  "qa-default-timeout-001",
		WorkDir:      workDir,
		BuildCommand: "echo ok",
		Timeout:      0,
	}

	result := e.Execute(ctx, req)

	if !result.Success {
		t.Error("expected pipeline with default timeout to succeed")
	}
}

func TestQAExecutor_ContextCancellation(t *testing.T) {
	workDir := t.TempDir()

	e := NewQAPipelineExecutor()
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context immediately.
	cancel()

	req := ws.QARequestPayload{
		ExecutionID:  "qa-cancel-001",
		WorkDir:      workDir,
		BuildCommand: "sleep 30",
		Timeout:      60,
	}

	result := e.Execute(ctx, req)

	if result.Success {
		t.Error("expected pipeline to fail on context cancellation")
	}
}

func TestQAExecutor_BuildStageOutput(t *testing.T) {
	workDir := t.TempDir()

	e := NewQAPipelineExecutor()
	ctx := context.Background()

	req := ws.QARequestPayload{
		ExecutionID:  "qa-output-001",
		WorkDir:      workDir,
		BuildCommand: "echo hello-from-build",
		Timeout:      10,
	}

	result := e.Execute(ctx, req)

	if !result.Success {
		t.Error("expected pipeline to succeed")
	}

	// Verify build output is captured.
	buildStage := result.Stages[0]
	if buildStage.Name != stageBuild {
		t.Fatalf("expected first stage to be %q, got %q", stageBuild, buildStage.Name)
	}
	if buildStage.Output == "" {
		t.Error("expected build stage to capture output")
	}
}

func TestQAExecutor_StageDuration(t *testing.T) {
	workDir := t.TempDir()

	e := NewQAPipelineExecutor()
	ctx := context.Background()

	req := ws.QARequestPayload{
		ExecutionID:  "qa-duration-001",
		WorkDir:      workDir,
		BuildCommand: "echo ok",
		Timeout:      10,
	}

	result := e.Execute(ctx, req)

	for _, stage := range result.Stages {
		if stage.DurationMs < 0 {
			t.Errorf("stage %q has negative duration: %d", stage.Name, stage.DurationMs)
		}
	}
}

func TestQAExecutor_CollectScreenshots(t *testing.T) {
	workDir := t.TempDir()

	// Create a mock test-results directory with screenshot files.
	screenshotDir := filepath.Join(workDir, "test-results", "test-run")
	if err := os.MkdirAll(screenshotDir, 0755); err != nil {
		t.Fatalf("failed to create screenshot directory: %v", err)
	}

	// Create a fake PNG file.
	pngContent := []byte("fake-png-data")
	pngPath := filepath.Join(screenshotDir, "screenshot1.png")
	if err := os.WriteFile(pngPath, pngContent, 0644); err != nil {
		t.Fatalf("failed to create PNG file: %v", err)
	}

	// Create a fake JPG file.
	jpgContent := []byte("fake-jpg-data")
	jpgPath := filepath.Join(screenshotDir, "screenshot2.jpg")
	if err := os.WriteFile(jpgPath, jpgContent, 0644); err != nil {
		t.Fatalf("failed to create JPG file: %v", err)
	}

	// Create a non-image file that should be ignored.
	txtPath := filepath.Join(screenshotDir, "log.txt")
	if err := os.WriteFile(txtPath, []byte("log data"), 0644); err != nil {
		t.Fatalf("failed to create TXT file: %v", err)
	}

	e := NewQAPipelineExecutor()
	screenshots := e.collectScreenshots(workDir)

	if len(screenshots) != 2 {
		t.Fatalf("expected 2 screenshots, got %d", len(screenshots))
	}

	// Verify screenshots are valid base64.
	for i, s := range screenshots {
		decoded, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			t.Errorf("screenshot[%d] is not valid base64: %v", i, err)
			continue
		}
		if len(decoded) == 0 {
			t.Errorf("screenshot[%d] decoded to empty bytes", i)
		}
	}
}

func TestQAExecutor_CollectScreenshots_NoDir(t *testing.T) {
	workDir := t.TempDir()
	// No test-results directory exists.

	e := NewQAPipelineExecutor()
	screenshots := e.collectScreenshots(workDir)

	if len(screenshots) != 0 {
		t.Errorf("expected 0 screenshots when no directory exists, got %d", len(screenshots))
	}
}

func TestQAExecutor_ServiceFailedToStart(t *testing.T) {
	workDir := t.TempDir()

	e := NewQAPipelineExecutor()
	ctx := context.Background()

	// Use a command that will fail immediately.
	req := ws.QARequestPayload{
		ExecutionID: "qa-svc-fail-001",
		WorkDir:     workDir,
		ServiceConfig: &ws.ServiceConfig{
			Command:      "nonexistent-command-xyz-12345",
			HealthCheck:  "http://localhost:99999/health",
			ReadyTimeout: 2,
		},
		TestCommand: "echo should-not-run",
		Timeout:     10,
	}

	result := e.Execute(ctx, req)

	if result.Success {
		t.Error("expected pipeline to fail when service fails to start")
	}

	// Test stage should be skipped.
	for _, stage := range result.Stages {
		if stage.Name == stageTest {
			t.Error("test stage should have been skipped when service fails")
		}
	}
}

func TestQAExecutor_CleanupWithNoService(t *testing.T) {
	e := NewQAPipelineExecutor()

	// Cleanup with nil service command should succeed.
	stageResult := e.runCleanupStage(nil)

	if !stageResult.Success {
		t.Error("expected cleanup to succeed with nil service command")
	}
	if stageResult.Name != stageCleanup {
		t.Errorf("expected stage name %q, got %q", stageCleanup, stageResult.Name)
	}
}

func TestQAExecutor_WaitForHealthCheck_ImmediateSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	e := NewQAPipelineExecutor()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := e.waitForHealthCheck(ctx, server.URL)
	if err != nil {
		t.Errorf("expected immediate health check to succeed: %v", err)
	}
}

func TestQAExecutor_WaitForHealthCheck_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	e := NewQAPipelineExecutor()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := e.waitForHealthCheck(ctx, server.URL)
	if err == nil {
		t.Error("expected health check to fail on context cancellation")
	}
}

// TestQAExecutor_InterfaceCompliance verifies that QAPipelineExecutor
// satisfies the websocket.QAExecutor interface.
func TestQAExecutor_InterfaceCompliance(t *testing.T) {
	e := NewQAPipelineExecutor()

	// Verify the executor has the correct Execute method signature.
	ctx := context.Background()
	workDir := t.TempDir()

	req := ws.QARequestPayload{
		ExecutionID:  "qa-interface-001",
		WorkDir:      workDir,
		BuildCommand: "echo ok",
		Timeout:      5,
	}

	result := e.Execute(ctx, req)
	if result == nil {
		t.Fatal("Execute returned nil")
	}
	if result.ExecutionID != req.ExecutionID {
		t.Errorf("ExecutionID mismatch: got %s, want %s", result.ExecutionID, req.ExecutionID)
	}
}

func TestQAExecutor_TestStageFailure_SkipsBrowserQA(t *testing.T) {
	workDir := t.TempDir()

	healthServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer healthServer.Close()

	e := NewQAPipelineExecutor()
	ctx := context.Background()

	req := ws.QARequestPayload{
		ExecutionID:  "qa-skip-browser-001",
		WorkDir:      workDir,
		BuildCommand: "echo build-ok",
		ServiceConfig: &ws.ServiceConfig{
			Command:      "sleep 30",
			HealthCheck:  healthServer.URL,
			ReadyTimeout: 5,
		},
		TestCommand: "exit 1",
		BrowserQA: &ws.BrowserQAConfig{
			Script:  "test.spec.ts",
			Browser: "chromium",
		},
		Timeout: 30,
	}

	result := e.Execute(ctx, req)

	if result.Success {
		t.Error("expected pipeline to fail when test stage fails")
	}

	// Browser QA should be skipped.
	for _, stage := range result.Stages {
		if stage.Name == stageBrowserQA {
			t.Error("browser QA stage should have been skipped when test fails")
		}
	}

	// Verify build succeeded, service succeeded, test failed, cleanup ran.
	stageNames := make([]string, len(result.Stages))
	for i, s := range result.Stages {
		stageNames[i] = s.Name
	}
	expected := []string{stageBuild, stageService, stageTest, stageCleanup}
	if len(stageNames) != len(expected) {
		t.Fatalf("expected stages %v, got %v", expected, stageNames)
	}
	for i, name := range expected {
		if stageNames[i] != name {
			t.Errorf("stage[%d] name mismatch: got %s, want %s", i, stageNames[i], name)
		}
	}
}

func TestQAExecutor_MultipleHealthCheckStatuses(t *testing.T) {
	// Test that health check accepts any 2xx status code.
	statusCodes := []int{200, 201, 204}

	for _, code := range statusCodes {
		t.Run(fmt.Sprintf("status_%d", code), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
			}))
			defer server.Close()

			e := NewQAPipelineExecutor()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err := e.waitForHealthCheck(ctx, server.URL)
			if err != nil {
				t.Errorf("expected health check to succeed with status %d: %v", code, err)
			}
		})
	}
}
