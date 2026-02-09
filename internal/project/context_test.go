package project

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func setupTestProjectDir(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "quorum-context-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Create .quorum structure
	quorumDir := filepath.Join(tmpDir, ".quorum")
	dirs := []string{
		quorumDir,
		filepath.Join(quorumDir, "state"),
		filepath.Join(quorumDir, "logs"),
		filepath.Join(quorumDir, "attachments"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			os.RemoveAll(tmpDir)
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	// Create minimal config
	configPath := filepath.Join(quorumDir, "config.yaml")
	configContent := []byte("version: 1\n")
	if err := os.WriteFile(configPath, configContent, 0o640); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create config: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return tmpDir, cleanup
}

func TestNewProjectContext(t *testing.T) {
	t.Parallel()
	projectDir, cleanup := setupTestProjectDir(t)
	defer cleanup()

	ctx, err := NewProjectContext("test-id", projectDir)
	if err != nil {
		t.Fatalf("NewProjectContext failed: %v", err)
	}
	defer ctx.Close()

	if ctx.ID != "test-id" {
		t.Errorf("expected ID 'test-id', got '%s'", ctx.ID)
	}

	if ctx.Root != projectDir {
		t.Errorf("expected root '%s', got '%s'", projectDir, ctx.Root)
	}

	if ctx.StateManager == nil {
		t.Error("expected non-nil StateManager")
	}

	if ctx.EventBus == nil {
		t.Error("expected non-nil EventBus")
	}

	if ctx.ConfigLoader == nil {
		t.Error("expected non-nil ConfigLoader")
	}

	if ctx.Attachments == nil {
		t.Error("expected non-nil Attachments")
	}

	if ctx.IsClosed() {
		t.Error("expected context to not be closed")
	}
}

func TestNewProjectContextWithOptions(t *testing.T) {
	t.Parallel()
	projectDir, cleanup := setupTestProjectDir(t)
	defer cleanup()

	ctx, err := NewProjectContext("test-id", projectDir,
		WithEventBufferSize(50),
	)
	if err != nil {
		t.Fatalf("NewProjectContext failed: %v", err)
	}
	defer ctx.Close()

	if ctx.StateManager == nil {
		t.Error("expected non-nil StateManager")
	}
}

func TestNewProjectContextInvalidPath(t *testing.T) {
	t.Parallel()
	_, err := NewProjectContext("test-id", "/nonexistent/path")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestNewProjectContextNotQuorumProject(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "quorum-not-project-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create directory without .quorum
	_, err = NewProjectContext("test-id", tmpDir)
	if err == nil {
		t.Error("expected error for non-quorum directory")
	}
}

func TestProjectContextClose(t *testing.T) {
	t.Parallel()
	projectDir, cleanup := setupTestProjectDir(t)
	defer cleanup()

	ctx, err := NewProjectContext("test-id", projectDir)
	if err != nil {
		t.Fatalf("NewProjectContext failed: %v", err)
	}

	// Close should succeed
	err = ctx.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	if !ctx.IsClosed() {
		t.Error("expected context to be closed")
	}

	// Second close should be no-op
	err = ctx.Close()
	if err != nil {
		t.Errorf("second Close failed: %v", err)
	}
}

func TestProjectContextValidate(t *testing.T) {
	t.Parallel()
	projectDir, cleanup := setupTestProjectDir(t)
	defer cleanup()

	ctx, err := NewProjectContext("test-id", projectDir)
	if err != nil {
		t.Fatalf("NewProjectContext failed: %v", err)
	}
	defer ctx.Close()

	// Should be valid
	err = ctx.Validate(context.Background())
	if err != nil {
		t.Errorf("Validate failed: %v", err)
	}
}

func TestProjectContextValidateAfterClose(t *testing.T) {
	t.Parallel()
	projectDir, cleanup := setupTestProjectDir(t)
	defer cleanup()

	ctx, err := NewProjectContext("test-id", projectDir)
	if err != nil {
		t.Fatalf("NewProjectContext failed: %v", err)
	}

	ctx.Close()

	err = ctx.Validate(context.Background())
	if err == nil {
		t.Error("expected error validating closed context")
	}
}

func TestProjectContextValidateDeletedDirectory(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: open SQLite handle prevents RemoveAll")
	}

	projectDir, cleanup := setupTestProjectDir(t)
	defer cleanup()

	ctx, err := NewProjectContext("test-id", projectDir)
	if err != nil {
		t.Fatalf("NewProjectContext failed: %v", err)
	}
	defer ctx.Close()

	// Delete the .quorum directory
	os.RemoveAll(filepath.Join(projectDir, ".quorum"))

	err = ctx.Validate(context.Background())
	if err == nil {
		t.Error("expected error after deleting .quorum")
	}
}

func TestProjectContextTouch(t *testing.T) {
	t.Parallel()
	projectDir, cleanup := setupTestProjectDir(t)
	defer cleanup()

	ctx, err := NewProjectContext("test-id", projectDir)
	if err != nil {
		t.Fatalf("NewProjectContext failed: %v", err)
	}
	defer ctx.Close()

	originalTime := ctx.GetLastAccessed()

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	ctx.Touch()

	newTime := ctx.GetLastAccessed()
	if !newTime.After(originalTime) {
		t.Error("expected LastAccessed to be updated")
	}
}

func TestProjectContextGetConfig(t *testing.T) {
	t.Parallel()
	projectDir, cleanup := setupTestProjectDir(t)
	defer cleanup()

	ctx, err := NewProjectContext("test-id", projectDir)
	if err != nil {
		t.Fatalf("NewProjectContext failed: %v", err)
	}
	defer ctx.Close()

	cfg, err := ctx.GetConfig()
	if err != nil {
		t.Errorf("GetConfig failed: %v", err)
	}

	if cfg == nil {
		t.Error("expected non-nil config")
	}
}

func TestProjectContextGetConfigAfterClose(t *testing.T) {
	t.Parallel()
	projectDir, cleanup := setupTestProjectDir(t)
	defer cleanup()

	ctx, err := NewProjectContext("test-id", projectDir)
	if err != nil {
		t.Fatalf("NewProjectContext failed: %v", err)
	}

	ctx.Close()

	_, err = ctx.GetConfig()
	if err == nil {
		t.Error("expected error getting config from closed context")
	}
}

func TestProjectContextHasRunningWorkflows(t *testing.T) {
	t.Parallel()
	projectDir, cleanup := setupTestProjectDir(t)
	defer cleanup()

	ctx, err := NewProjectContext("test-id", projectDir)
	if err != nil {
		t.Fatalf("NewProjectContext failed: %v", err)
	}
	defer ctx.Close()

	// Should have no running workflows initially
	hasRunning, err := ctx.HasRunningWorkflows(context.Background())
	if err != nil {
		t.Errorf("HasRunningWorkflows failed: %v", err)
	}
	if hasRunning {
		t.Error("expected no running workflows")
	}
}

func TestProjectContextHasRunningWorkflowsAfterClose(t *testing.T) {
	t.Parallel()
	projectDir, cleanup := setupTestProjectDir(t)
	defer cleanup()

	ctx, err := NewProjectContext("test-id", projectDir)
	if err != nil {
		t.Fatalf("NewProjectContext failed: %v", err)
	}

	ctx.Close()

	_, err = ctx.HasRunningWorkflows(context.Background())
	if err == nil {
		t.Error("expected error checking running workflows on closed context")
	}
}

func TestProjectContextConcurrentAccess(t *testing.T) {
	t.Parallel()
	projectDir, cleanup := setupTestProjectDir(t)
	defer cleanup()

	ctx, err := NewProjectContext("test-id", projectDir)
	if err != nil {
		t.Fatalf("NewProjectContext failed: %v", err)
	}
	defer ctx.Close()

	// Run concurrent operations
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			ctx.Touch()
			_ = ctx.IsClosed()
			_ = ctx.GetLastAccessed()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestProjectContextString(t *testing.T) {
	t.Parallel()
	projectDir, cleanup := setupTestProjectDir(t)
	defer cleanup()

	ctx, err := NewProjectContext("test-id", projectDir)
	if err != nil {
		t.Fatalf("NewProjectContext failed: %v", err)
	}
	defer ctx.Close()

	str := ctx.String()
	if str == "" {
		t.Error("expected non-empty string representation")
	}

	if !containsString(str, "test-id") {
		t.Error("expected string to contain ID")
	}
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestProjectContextRelativePath(t *testing.T) {
	t.Parallel()
	projectDir, cleanup := setupTestProjectDir(t)
	defer cleanup()

	// Get current directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}

	// Change to parent of project dir so we can use relative path
	parentDir := filepath.Dir(projectDir)
	if err := os.Chdir(parentDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Use relative path - should be converted to absolute
	relativePath := filepath.Base(projectDir)
	ctx, err := NewProjectContext("test-id", relativePath)
	if err != nil {
		t.Fatalf("NewProjectContext with relative path failed: %v", err)
	}
	defer ctx.Close()

	// Root should be absolute
	if !filepath.IsAbs(ctx.Root) {
		t.Errorf("expected absolute root, got %s", ctx.Root)
	}
}

func TestProjectContextEventBusPublish(t *testing.T) {
	t.Parallel()
	projectDir, cleanup := setupTestProjectDir(t)
	defer cleanup()

	ctx, err := NewProjectContext("test-id", projectDir)
	if err != nil {
		t.Fatalf("NewProjectContext failed: %v", err)
	}
	defer ctx.Close()

	// Subscribe to events
	ch := ctx.EventBus.Subscribe()

	// Publish a test event
	testEvent := &testEventImpl{
		eventType:  "test_event",
		timestamp:  time.Now(),
		workflowID: "wf-test",
	}
	ctx.EventBus.Publish(testEvent)

	// Should receive the event
	select {
	case event := <-ch:
		if event.EventType() != "test_event" {
			t.Errorf("expected event type 'test_event', got '%s'", event.EventType())
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for event")
	}
}

// testEventImpl is a test implementation of the Event interface
type testEventImpl struct {
	eventType  string
	timestamp  time.Time
	workflowID string
	projectID  string
}

func (e *testEventImpl) EventType() string    { return e.eventType }
func (e *testEventImpl) Timestamp() time.Time { return e.timestamp }
func (e *testEventImpl) WorkflowID() string   { return e.workflowID }
func (e *testEventImpl) ProjectID() string    { return e.projectID }
