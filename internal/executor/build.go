// Package executor provides build execution for Local Agent Bridge.
// FR-P3-01: Build command execution with output capture and duration tracking.
package executor

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/insajin/autopus-agent-protocol"
)

// BuildExecutor handles build command execution.
type BuildExecutor struct{}

// NewBuildExecutor creates a new BuildExecutor.
func NewBuildExecutor() *BuildExecutor {
	return &BuildExecutor{}
}

// Execute runs a build command and returns the result.
func (e *BuildExecutor) Execute(ctx context.Context, req ws.BuildRequestPayload) *ws.BuildResultPayload {
	start := time.Now()

	result := &ws.BuildResultPayload{
		ExecutionID: req.ExecutionID,
	}

	// Validate work directory.
	workDir, err := validateWorkDir(req.WorkDir)
	if err != nil {
		result.Success = false
		result.Output = fmt.Sprintf("invalid work directory: %v", err)
		result.ExitCode = 1
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}

	// Set up timeout from request or use default (10 minutes).
	timeout := time.Duration(req.Timeout) * time.Second
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Build the command using shell execution for pipeline support.
	cmd := exec.CommandContext(execCtx, "sh", "-c", req.Command)
	cmd.Dir = workDir

	// Merge environment variables.
	cmd.Env = os.Environ()
	if len(req.Env) > 0 {
		cmd.Env = append(cmd.Env, req.Env...)
	}

	// Capture stdout and stderr combined.
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	// Run the command.
	err = cmd.Run()

	result.Output = output.String()
	result.DurationMs = time.Since(start).Milliseconds()

	if err != nil {
		result.Success = false
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1
			// Append the error message if it is not just an exit code issue.
			result.Output = result.Output + "\n" + err.Error()
		}
		return result
	}

	result.Success = true
	result.ExitCode = 0
	return result
}

// validateWorkDir validates and resolves the work directory path.
func validateWorkDir(dir string) (string, error) {
	if dir == "" {
		return "", fmt.Errorf("work directory is empty")
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	info, err := os.Stat(absDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("directory does not exist: %s", absDir)
		}
		return "", fmt.Errorf("failed to stat directory: %w", err)
	}

	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory: %s", absDir)
	}

	return absDir, nil
}
