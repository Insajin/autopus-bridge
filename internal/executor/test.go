// Package executor provides test execution for Local Agent Bridge.
// FR-P3-02: Test command execution with output parsing and summary extraction.
package executor

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/insajin/autopus-agent-protocol"
)

// TestExecutor handles test command execution.
type TestExecutor struct{}

// NewTestExecutor creates a new TestExecutor.
func NewTestExecutor() *TestExecutor {
	return &TestExecutor{}
}

// Execute runs a test command and returns the result with parsed summary.
func (e *TestExecutor) Execute(ctx context.Context, req ws.TestRequestPayload) *ws.TestResultPayload {
	start := time.Now()

	result := &ws.TestResultPayload{
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

	// Build the test command, appending pattern filter if provided.
	command := req.Command
	if req.Pattern != "" {
		command = command + " " + req.Pattern
	}

	// Set up timeout from request or use default (10 minutes).
	timeout := time.Duration(req.Timeout) * time.Second
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Build the command using shell execution.
	cmd := exec.CommandContext(execCtx, "sh", "-c", command)
	cmd.Dir = workDir
	cmd.Env = os.Environ()

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
			result.Output = result.Output + "\n" + err.Error()
		}
	} else {
		result.Success = true
		result.ExitCode = 0
	}

	// Parse test output to extract summary counts.
	result.Summary = parseTestOutput(result.Output, command)

	return result
}

// parseTestOutput attempts to extract test counts from command output.
// It detects the test framework from the command and output, then applies
// the appropriate parser.
func parseTestOutput(output, command string) ws.TestSummary {
	cmdLower := strings.ToLower(command)

	// Detect Go test output.
	if strings.Contains(cmdLower, "go test") {
		return parseGoTestOutput(output)
	}

	// Detect pytest output.
	if strings.Contains(cmdLower, "pytest") || strings.Contains(cmdLower, "python -m pytest") {
		return parsePytestOutput(output)
	}

	// Detect npm/jest/vitest output.
	if strings.Contains(cmdLower, "jest") ||
		strings.Contains(cmdLower, "vitest") ||
		strings.Contains(cmdLower, "npm test") ||
		strings.Contains(cmdLower, "npx") {
		return parseJestOutput(output)
	}

	// Fallback: try all parsers in order and return the first non-empty result.
	if s := parseGoTestOutput(output); s.Total > 0 {
		return s
	}
	if s := parsePytestOutput(output); s.Total > 0 {
		return s
	}
	if s := parseJestOutput(output); s.Total > 0 {
		return s
	}

	return ws.TestSummary{}
}

// Go test output patterns:
//
//	--- PASS: TestName (0.00s)
//	--- FAIL: TestName (0.00s)
//	--- SKIP: TestName (0.00s)
//	ok   package  0.123s
//	FAIL package  0.123s
var (
	goTestPassRe = regexp.MustCompile(`(?m)^--- PASS:`)
	goTestFailRe = regexp.MustCompile(`(?m)^--- FAIL:`)
	goTestSkipRe = regexp.MustCompile(`(?m)^--- SKIP:`)
)

func parseGoTestOutput(output string) ws.TestSummary {
	passed := len(goTestPassRe.FindAllString(output, -1))
	failed := len(goTestFailRe.FindAllString(output, -1))
	skipped := len(goTestSkipRe.FindAllString(output, -1))
	total := passed + failed + skipped

	return ws.TestSummary{
		Total:   total,
		Passed:  passed,
		Failed:  failed,
		Skipped: skipped,
	}
}

// pytest output patterns:
//
//	====== 5 passed, 2 failed, 1 skipped in 1.23s ======
//	====== 10 passed in 0.45s ======
var pytestSummaryRe = regexp.MustCompile(`(?m)=+\s*(.*?)\s+in\s+[\d.]+s\s*=+`)

func parsePytestOutput(output string) ws.TestSummary {
	match := pytestSummaryRe.FindStringSubmatch(output)
	if len(match) < 2 {
		return ws.TestSummary{}
	}

	summaryLine := match[1]
	summary := ws.TestSummary{}

	// Extract "N passed", "N failed", "N skipped", "N error" patterns.
	re := regexp.MustCompile(`(\d+)\s+(passed|failed|skipped|error|errors|warnings?)`)
	matches := re.FindAllStringSubmatch(summaryLine, -1)
	for _, m := range matches {
		count, _ := strconv.Atoi(m[1])
		switch {
		case strings.HasPrefix(m[2], "passed"):
			summary.Passed = count
		case strings.HasPrefix(m[2], "failed"):
			summary.Failed = count
		case strings.HasPrefix(m[2], "skipped"):
			summary.Skipped = count
		}
	}

	summary.Total = summary.Passed + summary.Failed + summary.Skipped
	return summary
}

// Jest/Vitest output patterns:
//
//	Tests:       2 failed, 8 passed, 10 total
//	Test Suites: 1 failed, 3 passed, 4 total
var jestTestsRe = regexp.MustCompile(`(?m)Tests:\s+(.+?)(\d+)\s+total`)

func parseJestOutput(output string) ws.TestSummary {
	match := jestTestsRe.FindStringSubmatch(output)
	if len(match) < 3 {
		return ws.TestSummary{}
	}

	summary := ws.TestSummary{}
	summary.Total, _ = strconv.Atoi(match[2])

	// Parse the breakdown before "total".
	parts := match[1]
	passedRe := regexp.MustCompile(`(\d+)\s+passed`)
	failedRe := regexp.MustCompile(`(\d+)\s+failed`)
	skippedRe := regexp.MustCompile(`(\d+)\s+skipped`)

	if m := passedRe.FindStringSubmatch(parts); len(m) >= 2 {
		summary.Passed, _ = strconv.Atoi(m[1])
	}
	if m := failedRe.FindStringSubmatch(parts); len(m) >= 2 {
		summary.Failed, _ = strconv.Atoi(m[1])
	}
	if m := skippedRe.FindStringSubmatch(parts); len(m) >= 2 {
		summary.Skipped, _ = strconv.Atoi(m[1])
	}

	return summary
}
