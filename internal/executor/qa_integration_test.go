//go:build integration

// Package executor provides integration tests for the QA pipeline.
// NFR-05: Integration tests with real commands to verify end-to-end behavior.
package executor

import (
	"context"
	"testing"
	"time"

	"github.com/insajin/autopus-agent-protocol"
)

// TestQAPipeline_RealBuildCommand executes a real build command (echo) and
// verifies the pipeline processes it correctly.
func TestQAPipeline_RealBuildCommand(t *testing.T) {
	executor := NewQAPipelineExecutor()

	req := ws.QARequestPayload{
		ExecutionID:  "integration-build-001",
		WorkDir:      t.TempDir(),
		BuildCommand: "echo 'build successful'",
		Timeout:      30,
	}

	ctx := context.Background()
	result := executor.Execute(ctx, req)

	if result.ExecutionID != req.ExecutionID {
		t.Errorf("ExecutionID = %q, want %q", result.ExecutionID, req.ExecutionID)
	}
	if !result.Success {
		t.Errorf("pipeline failed: stages=%+v", result.Stages)
	}

	// Verify build stage.
	foundBuild := false
	for _, stage := range result.Stages {
		if stage.Name == stageBuild {
			foundBuild = true
			if !stage.Success {
				t.Errorf("build stage failed: %s", stage.Error)
			}
			if stage.DurationMs < 0 {
				t.Errorf("build stage duration = %d, want >= 0", stage.DurationMs)
			}
		}
	}
	if !foundBuild {
		t.Error("build stage not found in results")
	}

	if result.DurationMs <= 0 {
		t.Errorf("total duration = %d, want > 0", result.DurationMs)
	}
}

// TestQAPipeline_RealTestCommand executes a real test command and verifies output.
func TestQAPipeline_RealTestCommand(t *testing.T) {
	executor := NewQAPipelineExecutor()

	req := ws.QARequestPayload{
		ExecutionID: "integration-test-001",
		WorkDir:     t.TempDir(),
		TestCommand: "echo 'all tests passed'",
		Timeout:     30,
	}

	ctx := context.Background()
	result := executor.Execute(ctx, req)

	if !result.Success {
		t.Errorf("pipeline failed: stages=%+v", result.Stages)
	}

	foundTest := false
	for _, stage := range result.Stages {
		if stage.Name == stageTest {
			foundTest = true
			if !stage.Success {
				t.Errorf("test stage failed: %s", stage.Error)
			}
		}
	}
	if !foundTest {
		t.Error("test stage not found in results")
	}
}

// TestQAPipeline_FailingBuildCommand verifies that a failing build command
// causes the pipeline to fail and skips subsequent stages.
func TestQAPipeline_FailingBuildCommand(t *testing.T) {
	executor := NewQAPipelineExecutor()

	req := ws.QARequestPayload{
		ExecutionID:  "integration-fail-001",
		WorkDir:      t.TempDir(),
		BuildCommand: "false", // Always fails with exit code 1.
		TestCommand:  "echo 'should not run'",
		Timeout:      30,
	}

	ctx := context.Background()
	result := executor.Execute(ctx, req)

	if result.Success {
		t.Error("pipeline should have failed")
	}

	// Build stage should be present and failed.
	for _, stage := range result.Stages {
		if stage.Name == stageBuild {
			if stage.Success {
				t.Error("build stage should have failed")
			}
		}
		// Test stage should NOT be present since build failed.
		if stage.Name == stageTest {
			t.Error("test stage should not have run after build failure")
		}
	}
}

// TestQAPipeline_SuccessfulTrueCommand verifies that `true` command succeeds.
func TestQAPipeline_SuccessfulTrueCommand(t *testing.T) {
	executor := NewQAPipelineExecutor()

	req := ws.QARequestPayload{
		ExecutionID:  "integration-true-001",
		WorkDir:      t.TempDir(),
		BuildCommand: "true",
		TestCommand:  "true",
		Timeout:      30,
	}

	ctx := context.Background()
	result := executor.Execute(ctx, req)

	if !result.Success {
		for _, stage := range result.Stages {
			if !stage.Success {
				t.Errorf("stage %s failed: %s", stage.Name, stage.Error)
			}
		}
	}
}

// TestQAPipeline_TimeoutHandling verifies that the pipeline respects timeout settings.
func TestQAPipeline_TimeoutHandling(t *testing.T) {
	executor := NewQAPipelineExecutor()

	req := ws.QARequestPayload{
		ExecutionID:  "integration-timeout-001",
		WorkDir:      t.TempDir(),
		BuildCommand: "sleep 60", // Long-running command.
		Timeout:      1,          // 1 second timeout.
	}

	ctx := context.Background()
	start := time.Now()
	result := executor.Execute(ctx, req)
	elapsed := time.Since(start)

	if result.Success {
		t.Error("pipeline should have failed due to timeout")
	}

	// Should complete within a reasonable time (timeout + cleanup overhead).
	if elapsed > 10*time.Second {
		t.Errorf("pipeline took %v, expected to timeout within ~2s", elapsed)
	}
}

// TestQAPipeline_CleanupAlwaysRuns verifies that the cleanup stage always
// runs regardless of other stage failures.
func TestQAPipeline_CleanupAlwaysRuns(t *testing.T) {
	executor := NewQAPipelineExecutor()

	req := ws.QARequestPayload{
		ExecutionID:  "integration-cleanup-001",
		WorkDir:      t.TempDir(),
		BuildCommand: "false", // Fails.
		Timeout:      30,
	}

	ctx := context.Background()
	result := executor.Execute(ctx, req)

	foundCleanup := false
	for _, stage := range result.Stages {
		if stage.Name == stageCleanup {
			foundCleanup = true
			if !stage.Success {
				t.Errorf("cleanup stage should always succeed, got error: %s", stage.Error)
			}
		}
	}
	if !foundCleanup {
		t.Error("cleanup stage not found in results")
	}
}

// TestQAPipeline_MultiStageSequence verifies the full sequential stage execution:
// build -> test -> cleanup.
func TestQAPipeline_MultiStageSequence(t *testing.T) {
	executor := NewQAPipelineExecutor()

	req := ws.QARequestPayload{
		ExecutionID:  "integration-sequence-001",
		WorkDir:      t.TempDir(),
		BuildCommand: "echo 'building...'",
		TestCommand:  "echo 'testing...'",
		Timeout:      30,
	}

	ctx := context.Background()
	result := executor.Execute(ctx, req)

	if !result.Success {
		t.Errorf("pipeline failed: stages=%+v", result.Stages)
	}

	// Verify stage order: build, test, cleanup.
	expectedOrder := []string{stageBuild, stageTest, stageCleanup}
	if len(result.Stages) != len(expectedOrder) {
		t.Fatalf("got %d stages, want %d", len(result.Stages), len(expectedOrder))
	}

	for i, expected := range expectedOrder {
		if result.Stages[i].Name != expected {
			t.Errorf("stage[%d].Name = %q, want %q", i, result.Stages[i].Name, expected)
		}
	}
}

// TestQAPipeline_InvalidWorkDir verifies that an invalid work directory
// is properly reported.
func TestQAPipeline_InvalidWorkDir(t *testing.T) {
	executor := NewQAPipelineExecutor()

	req := ws.QARequestPayload{
		ExecutionID:  "integration-baddir-001",
		WorkDir:      "/nonexistent/path/that/does/not/exist",
		BuildCommand: "echo 'should not run'",
		Timeout:      30,
	}

	ctx := context.Background()
	result := executor.Execute(ctx, req)

	if result.Success {
		t.Error("pipeline should have failed with invalid work directory")
	}
}
