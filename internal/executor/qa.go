// Package executor provides QA pipeline execution for Local Agent Bridge.
// FR-P3-04: Build + Service + Test + Browser QA pipeline execution.
// FR-P3-05: Service lifecycle management with health check polling.
// FR-P3-06: Browser QA execution via Playwright with screenshot capture.
package executor

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/insajin/autopus-agent-protocol"
)

// Health check polling constants.
const (
	// healthCheckInterval is the interval between health check attempts.
	healthCheckInterval = 500 * time.Millisecond
	// defaultReadyTimeout is the default service ready timeout (30 seconds).
	defaultReadyTimeout = 30 * time.Second
	// healthCheckHTTPTimeout is the HTTP client timeout for a single health check request.
	healthCheckHTTPTimeout = 2 * time.Second
)

// QA stage name constants.
const (
	stageBuild     = "build"
	stageService   = "service_start"
	stageTest      = "test"
	stageBrowserQA = "browser_qa"
	stageCleanup   = "cleanup"
)

// QAPipelineExecutor handles QA pipeline execution with sequential stages.
type QAPipelineExecutor struct{}

// NewQAPipelineExecutor creates a new QAPipelineExecutor.
func NewQAPipelineExecutor() *QAPipelineExecutor {
	return &QAPipelineExecutor{}
}

// Execute runs the QA pipeline and returns the result.
// Stages run sequentially: build -> service start -> test -> browser QA -> cleanup.
// If any stage fails, remaining stages are skipped (except cleanup).
func (e *QAPipelineExecutor) Execute(ctx context.Context, req ws.QARequestPayload) *ws.QAResultPayload {
	start := time.Now()

	result := &ws.QAResultPayload{
		ExecutionID: req.ExecutionID,
		Stages:      make([]ws.QAStageResult, 0),
	}

	// Validate work directory.
	workDir, err := validateWorkDir(req.WorkDir)
	if err != nil {
		result.Success = false
		result.DurationMs = time.Since(start).Milliseconds()
		result.Stages = append(result.Stages, ws.QAStageResult{
			Name:       "validation",
			Success:    false,
			Output:     fmt.Sprintf("invalid work directory: %v", err),
			DurationMs: time.Since(start).Milliseconds(),
			Error:      err.Error(),
		})
		return result
	}

	// Set up overall timeout from request or use default.
	timeout := time.Duration(req.Timeout) * time.Second
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Track the service process for cleanup.
	var serviceCmd *exec.Cmd
	allPassed := true

	// Stage 1: Build (optional).
	if req.BuildCommand != "" {
		stageResult := e.runBuildStage(execCtx, workDir, req.BuildCommand)
		result.Stages = append(result.Stages, stageResult)
		if !stageResult.Success {
			allPassed = false
		}
	}

	// Stage 2: Service Start (optional).
	if allPassed && req.ServiceConfig != nil {
		var stageResult ws.QAStageResult
		stageResult, serviceCmd = e.runServiceStage(execCtx, workDir, req.ServiceConfig)
		result.Stages = append(result.Stages, stageResult)
		if !stageResult.Success {
			allPassed = false
		}
	}

	// Stage 3: Test (optional).
	if allPassed && req.TestCommand != "" {
		stageResult := e.runTestStage(execCtx, workDir, req.TestCommand)
		result.Stages = append(result.Stages, stageResult)
		if !stageResult.Success {
			allPassed = false
		}
	}

	// Stage 4: Browser QA (optional).
	if allPassed && req.BrowserQA != nil {
		stageResult, screenshots := e.runBrowserQAStage(execCtx, workDir, req.BrowserQA)
		result.Stages = append(result.Stages, stageResult)
		if len(screenshots) > 0 {
			result.Screenshots = screenshots
		}
		if !stageResult.Success {
			allPassed = false
		}
	}

	// Stage 5: Cleanup (always runs).
	cleanupResult := e.runCleanupStage(serviceCmd)
	result.Stages = append(result.Stages, cleanupResult)

	result.Success = allPassed
	result.DurationMs = time.Since(start).Milliseconds()

	return result
}

// runBuildStage executes the build command.
func (e *QAPipelineExecutor) runBuildStage(ctx context.Context, workDir, command string) ws.QAStageResult {
	start := time.Now()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = workDir
	cmd.Env = os.Environ()

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	err := cmd.Run()

	stageResult := ws.QAStageResult{
		Name:       stageBuild,
		Output:     output.String(),
		DurationMs: time.Since(start).Milliseconds(),
	}

	if err != nil {
		stageResult.Success = false
		stageResult.Error = err.Error()
	} else {
		stageResult.Success = true
	}

	return stageResult
}

// runServiceStage starts a background service and waits for it to become ready.
// Returns the stage result and the running command (for cleanup).
func (e *QAPipelineExecutor) runServiceStage(ctx context.Context, workDir string, cfg *ws.ServiceConfig) (ws.QAStageResult, *exec.Cmd) {
	start := time.Now()

	cmd := exec.CommandContext(ctx, "sh", "-c", cfg.Command)
	cmd.Dir = workDir
	cmd.Env = os.Environ()

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	// Start the service as a background process.
	if err := cmd.Start(); err != nil {
		return ws.QAStageResult{
			Name:       stageService,
			Success:    false,
			Output:     output.String(),
			DurationMs: time.Since(start).Milliseconds(),
			Error:      fmt.Sprintf("failed to start service: %v", err),
		}, nil
	}

	// Wait for health check to pass.
	readyTimeout := time.Duration(cfg.ReadyTimeout) * time.Second
	if readyTimeout <= 0 {
		readyTimeout = defaultReadyTimeout
	}

	healthCtx, healthCancel := context.WithTimeout(ctx, readyTimeout)
	defer healthCancel()

	err := e.waitForHealthCheck(healthCtx, cfg.HealthCheck)
	if err != nil {
		return ws.QAStageResult{
			Name:       stageService,
			Success:    false,
			Output:     output.String(),
			DurationMs: time.Since(start).Milliseconds(),
			Error:      fmt.Sprintf("service health check failed: %v", err),
		}, cmd
	}

	return ws.QAStageResult{
		Name:       stageService,
		Success:    true,
		Output:     "service started and health check passed",
		DurationMs: time.Since(start).Milliseconds(),
	}, cmd
}

// waitForHealthCheck polls the health check URL until it responds with 200 OK
// or the context is cancelled.
func (e *QAPipelineExecutor) waitForHealthCheck(ctx context.Context, url string) error {
	client := &http.Client{Timeout: healthCheckHTTPTimeout}

	ticker := time.NewTicker(healthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("health check timed out: %w", ctx.Err())
		case <-ticker.C:
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				continue
			}
			resp, err := client.Do(req)
			if err != nil {
				continue
			}
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
		}
	}
}

// runTestStage executes the test command.
func (e *QAPipelineExecutor) runTestStage(ctx context.Context, workDir, command string) ws.QAStageResult {
	start := time.Now()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = workDir
	cmd.Env = os.Environ()

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	err := cmd.Run()

	stageResult := ws.QAStageResult{
		Name:       stageTest,
		Output:     output.String(),
		DurationMs: time.Since(start).Milliseconds(),
	}

	if err != nil {
		stageResult.Success = false
		stageResult.Error = err.Error()
	} else {
		stageResult.Success = true
	}

	return stageResult
}

// runBrowserQAStage executes browser-based QA tests via Playwright.
// Returns the stage result and any captured screenshots as base64 strings.
func (e *QAPipelineExecutor) runBrowserQAStage(ctx context.Context, workDir string, cfg *ws.BrowserQAConfig) (ws.QAStageResult, []string) {
	start := time.Now()

	// Build the Playwright command.
	args := []string{"playwright", "test", cfg.Script}

	if cfg.Browser != "" {
		args = append(args, "--browser", cfg.Browser)
	}

	if cfg.Screenshot {
		args = append(args, "--screenshot", "on")
	}

	cmdStr := "npx " + strings.Join(args, " ")

	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	cmd.Dir = workDir
	cmd.Env = os.Environ()

	// Set headless mode via environment if configured.
	if cfg.Headless {
		cmd.Env = append(cmd.Env, "PLAYWRIGHT_HEADLESS=1")
	}

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	err := cmd.Run()

	stageResult := ws.QAStageResult{
		Name:       stageBrowserQA,
		Output:     output.String(),
		DurationMs: time.Since(start).Milliseconds(),
	}

	if err != nil {
		stageResult.Success = false
		stageResult.Error = err.Error()
	} else {
		stageResult.Success = true
	}

	// Collect screenshots if enabled.
	var screenshots []string
	if cfg.Screenshot {
		screenshots = e.collectScreenshots(workDir)
	}

	return stageResult, screenshots
}

// collectScreenshots finds and base64-encodes screenshot files from
// Playwright's default test-results directory.
func (e *QAPipelineExecutor) collectScreenshots(workDir string) []string {
	screenshotDir := filepath.Join(workDir, "test-results")

	var screenshots []string

	// Walk the directory looking for image files.
	_ = filepath.Walk(screenshotDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			encoded := base64.StdEncoding.EncodeToString(data)
			screenshots = append(screenshots, encoded)
		}

		return nil
	})

	return screenshots
}

// runCleanupStage kills any started service process and cleans up resources.
func (e *QAPipelineExecutor) runCleanupStage(serviceCmd *exec.Cmd) ws.QAStageResult {
	start := time.Now()

	var messages []string

	if serviceCmd != nil && serviceCmd.Process != nil {
		if err := serviceCmd.Process.Kill(); err != nil {
			messages = append(messages, fmt.Sprintf("failed to kill service process: %v", err))
		} else {
			messages = append(messages, "service process terminated")
		}
		// Wait for process to fully exit to avoid zombies.
		_ = serviceCmd.Wait()
	} else {
		messages = append(messages, "no service process to clean up")
	}

	output := strings.Join(messages, "; ")

	return ws.QAStageResult{
		Name:       stageCleanup,
		Success:    true,
		Output:     output,
		DurationMs: time.Since(start).Milliseconds(),
	}
}
