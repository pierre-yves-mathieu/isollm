package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Helper to create a test FileState with a temp directory
func newTestState(t *testing.T) (*FileState, string) {
	t.Helper()
	dir := t.TempDir()
	return NewWithDir(dir), dir
}

// Helper to create a sample session
func sampleSession() *Session {
	return &Session{
		Status:        SessionStatusRunning,
		StartedAt:     time.Now(),
		PID:           os.Getpid(), // Use current process so it's "alive"
		ProjectRoot:   "/test/project",
		BareRepoPath:  "/test/bare",
		ZellijSession: "test-session",
	}
}

// Helper to create a sample worker
func sampleWorker(name string) *WorkerState {
	return &WorkerState{
		Name:         name,
		ContainerID:  "container-" + name,
		Status:       WorkerStatusIdle,
		IP:           "10.0.0.1",
		StartedAt:    time.Now(),
		LastActivity: time.Now(),
	}
}

// ==================== Session Tests ====================

func TestFileState_CreateSession(t *testing.T) {
	fs, _ := newTestState(t)

	session := sampleSession()
	err := fs.CreateSession(session)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Verify file was created
	fullPath := filepath.Join(fs.stateDir, "session.json")
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		t.Fatal("session.json was not created")
	}

	// Verify version was set
	loaded, err := fs.LoadSession()
	if err != nil {
		t.Fatalf("LoadSession failed: %v", err)
	}
	if loaded.Version != CurrentSessionVersion {
		t.Errorf("Version = %d, want %d", loaded.Version, CurrentSessionVersion)
	}
	if loaded.Status != session.Status {
		t.Errorf("Status = %s, want %s", loaded.Status, session.Status)
	}
}

func TestFileState_CreateSession_AlreadyExists(t *testing.T) {
	fs, _ := newTestState(t)

	session := sampleSession()
	if err := fs.CreateSession(session); err != nil {
		t.Fatalf("First CreateSession failed: %v", err)
	}

	// Try to create again
	err := fs.CreateSession(session)
	if !errors.Is(err, ErrSessionExists) {
		t.Errorf("Expected ErrSessionExists, got: %v", err)
	}
}

func TestFileState_SaveLoadSession(t *testing.T) {
	fs, _ := newTestState(t)

	// First create, then save updates
	session := sampleSession()
	if err := fs.CreateSession(session); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Update and save
	session.Status = SessionStatusShuttingDown
	session.ZellijSession = "updated-session"
	if err := fs.SaveSession(session); err != nil {
		t.Fatalf("SaveSession failed: %v", err)
	}

	// Load and verify
	loaded, err := fs.LoadSession()
	if err != nil {
		t.Fatalf("LoadSession failed: %v", err)
	}
	if loaded.Status != SessionStatusShuttingDown {
		t.Errorf("Status = %s, want %s", loaded.Status, SessionStatusShuttingDown)
	}
	if loaded.ZellijSession != "updated-session" {
		t.Errorf("ZellijSession = %s, want updated-session", loaded.ZellijSession)
	}
}

func TestFileState_LoadSession_NotFound(t *testing.T) {
	fs, _ := newTestState(t)

	_, err := fs.LoadSession()
	if !errors.Is(err, ErrNoSession) {
		t.Errorf("Expected ErrNoSession, got: %v", err)
	}
}

func TestFileState_ClearSession(t *testing.T) {
	fs, _ := newTestState(t)

	session := sampleSession()
	if err := fs.CreateSession(session); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Clear
	if err := fs.ClearSession(); err != nil {
		t.Fatalf("ClearSession failed: %v", err)
	}

	// Verify file is gone
	_, err := fs.LoadSession()
	if !errors.Is(err, ErrNoSession) {
		t.Errorf("Expected ErrNoSession after clear, got: %v", err)
	}
}

func TestFileState_ClearSession_NotExists(t *testing.T) {
	fs, _ := newTestState(t)

	// Should not error when file doesn't exist
	if err := fs.ClearSession(); err != nil {
		t.Errorf("ClearSession on non-existent file returned error: %v", err)
	}
}

func TestFileState_HasActiveSession_NoSession(t *testing.T) {
	fs, _ := newTestState(t)

	active, err := fs.HasActiveSession()
	if err != nil {
		t.Fatalf("HasActiveSession failed: %v", err)
	}
	if active {
		t.Error("Expected false for no session, got true")
	}
}

func TestFileState_HasActiveSession_StaleSession(t *testing.T) {
	fs, _ := newTestState(t)

	// Create session with a PID that doesn't exist
	// PID 1 is init/systemd, typically owned by root, so we use a high unlikely PID
	session := sampleSession()
	session.PID = 999999999 // Very unlikely to exist
	if err := fs.CreateSession(session); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	active, err := fs.HasActiveSession()
	if err != nil {
		t.Fatalf("HasActiveSession failed: %v", err)
	}
	if active {
		t.Error("Expected false for stale session, got true")
	}
}

func TestFileState_HasActiveSession_LiveSession(t *testing.T) {
	fs, _ := newTestState(t)

	// Create session with current process PID (guaranteed to be alive)
	session := sampleSession()
	session.PID = os.Getpid()
	if err := fs.CreateSession(session); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	active, err := fs.HasActiveSession()
	if err != nil {
		t.Fatalf("HasActiveSession failed: %v", err)
	}
	if !active {
		t.Error("Expected true for live session, got false")
	}
}

func TestFileState_HasActiveSession_InvalidPID(t *testing.T) {
	fs, _ := newTestState(t)

	session := sampleSession()
	session.PID = 0 // Invalid PID
	if err := fs.CreateSession(session); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	active, err := fs.HasActiveSession()
	if err != nil {
		t.Fatalf("HasActiveSession failed: %v", err)
	}
	if active {
		t.Error("Expected false for invalid PID, got true")
	}
}

func TestFileState_HasActiveSession_NegativePID(t *testing.T) {
	fs, _ := newTestState(t)

	session := sampleSession()
	session.PID = -1 // Negative PID
	if err := fs.CreateSession(session); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	active, err := fs.HasActiveSession()
	if err != nil {
		t.Fatalf("HasActiveSession failed: %v", err)
	}
	if active {
		t.Error("Expected false for negative PID, got true")
	}
}

// ==================== Worker Tests ====================

func TestFileState_SaveLoadWorker(t *testing.T) {
	fs, _ := newTestState(t)

	worker := sampleWorker("worker-1")
	worker.CurrentTask = &TaskRef{
		ID:        "task-123",
		Title:     "Test Task",
		ClaimedAt: time.Now(),
	}

	if err := fs.SaveWorker(worker); err != nil {
		t.Fatalf("SaveWorker failed: %v", err)
	}

	loaded, err := fs.LoadWorker("worker-1")
	if err != nil {
		t.Fatalf("LoadWorker failed: %v", err)
	}

	if loaded.Name != worker.Name {
		t.Errorf("Name = %s, want %s", loaded.Name, worker.Name)
	}
	if loaded.Version != CurrentWorkerVersion {
		t.Errorf("Version = %d, want %d", loaded.Version, CurrentWorkerVersion)
	}
	if loaded.Status != worker.Status {
		t.Errorf("Status = %s, want %s", loaded.Status, worker.Status)
	}
	if loaded.CurrentTask == nil {
		t.Fatal("CurrentTask is nil")
	}
	if loaded.CurrentTask.ID != "task-123" {
		t.Errorf("CurrentTask.ID = %s, want task-123", loaded.CurrentTask.ID)
	}
}

func TestFileState_LoadWorker_NotFound(t *testing.T) {
	fs, _ := newTestState(t)

	_, err := fs.LoadWorker("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent worker")
	}
	// Check error message contains worker name
	if err != nil && !contains(err.Error(), "nonexistent") {
		t.Errorf("Error should mention worker name, got: %v", err)
	}
}

func TestFileState_LoadAllWorkers(t *testing.T) {
	fs, _ := newTestState(t)

	// Save multiple workers
	workers := []*WorkerState{
		sampleWorker("worker-1"),
		sampleWorker("worker-2"),
		sampleWorker("worker-3"),
	}
	workers[0].Status = WorkerStatusBusy
	workers[1].Status = WorkerStatusIdle
	workers[2].Status = WorkerStatusCreating

	for _, w := range workers {
		if err := fs.SaveWorker(w); err != nil {
			t.Fatalf("SaveWorker failed for %s: %v", w.Name, err)
		}
	}

	result, err := fs.LoadAllWorkers()
	if err != nil {
		t.Fatalf("LoadAllWorkers failed: %v", err)
	}

	if len(result.Workers) != 3 {
		t.Errorf("Expected 3 workers, got %d", len(result.Workers))
	}
	if len(result.Errors) != 0 {
		t.Errorf("Expected 0 errors, got %d: %v", len(result.Errors), result.Errors)
	}
}

func TestFileState_LoadAllWorkers_Empty(t *testing.T) {
	fs, _ := newTestState(t)

	result, err := fs.LoadAllWorkers()
	if err != nil {
		t.Fatalf("LoadAllWorkers failed: %v", err)
	}

	if len(result.Workers) != 0 {
		t.Errorf("Expected 0 workers, got %d", len(result.Workers))
	}
	if len(result.Errors) != 0 {
		t.Errorf("Expected 0 errors, got %d", len(result.Errors))
	}
}

func TestFileState_LoadAllWorkers_PartialFailure(t *testing.T) {
	fs, dir := newTestState(t)

	// Save some valid workers
	if err := fs.SaveWorker(sampleWorker("worker-1")); err != nil {
		t.Fatalf("SaveWorker failed: %v", err)
	}
	if err := fs.SaveWorker(sampleWorker("worker-2")); err != nil {
		t.Fatalf("SaveWorker failed: %v", err)
	}

	// Create a corrupt JSON file
	workersDir := filepath.Join(dir, "workers")
	corruptPath := filepath.Join(workersDir, "worker-corrupt.json")
	if err := os.WriteFile(corruptPath, []byte("not valid json{{{"), 0644); err != nil {
		t.Fatalf("Failed to write corrupt file: %v", err)
	}

	result, err := fs.LoadAllWorkers()
	if err != nil {
		t.Fatalf("LoadAllWorkers failed: %v", err)
	}

	// Should have 2 valid workers and 1 error
	if len(result.Workers) != 2 {
		t.Errorf("Expected 2 workers, got %d", len(result.Workers))
	}
	if len(result.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(result.Errors))
	}
}

func TestFileState_LoadAllWorkers_SkipsNonJSON(t *testing.T) {
	fs, dir := newTestState(t)

	// Save a valid worker
	if err := fs.SaveWorker(sampleWorker("worker-1")); err != nil {
		t.Fatalf("SaveWorker failed: %v", err)
	}

	// Create non-JSON files in workers directory
	workersDir := filepath.Join(dir, "workers")
	if err := os.WriteFile(filepath.Join(workersDir, "readme.txt"), []byte("ignore me"), 0644); err != nil {
		t.Fatalf("Failed to write txt file: %v", err)
	}
	if err := os.Mkdir(filepath.Join(workersDir, "subdir"), 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	result, err := fs.LoadAllWorkers()
	if err != nil {
		t.Fatalf("LoadAllWorkers failed: %v", err)
	}

	if len(result.Workers) != 1 {
		t.Errorf("Expected 1 worker, got %d", len(result.Workers))
	}
	if len(result.Errors) != 0 {
		t.Errorf("Expected 0 errors, got %d", len(result.Errors))
	}
}

func TestFileState_DeleteWorker(t *testing.T) {
	fs, _ := newTestState(t)

	worker := sampleWorker("worker-1")
	if err := fs.SaveWorker(worker); err != nil {
		t.Fatalf("SaveWorker failed: %v", err)
	}

	if err := fs.DeleteWorker("worker-1"); err != nil {
		t.Fatalf("DeleteWorker failed: %v", err)
	}

	// Verify it's gone
	_, err := fs.LoadWorker("worker-1")
	if err == nil {
		t.Error("Expected error loading deleted worker")
	}
}

func TestFileState_DeleteWorker_NotExists(t *testing.T) {
	fs, _ := newTestState(t)

	// Should not error when deleting non-existent worker
	if err := fs.DeleteWorker("nonexistent"); err != nil {
		t.Errorf("DeleteWorker on non-existent worker returned error: %v", err)
	}
}

func TestFileState_ClearAllWorkers(t *testing.T) {
	fs, dir := newTestState(t)

	// Save multiple workers
	for i := 1; i <= 3; i++ {
		name := "worker-" + string(rune('0'+i))
		if err := fs.SaveWorker(sampleWorker(name)); err != nil {
			t.Fatalf("SaveWorker failed: %v", err)
		}
	}

	// Clear all
	if err := fs.ClearAllWorkers(); err != nil {
		t.Fatalf("ClearAllWorkers failed: %v", err)
	}

	// Verify workers directory is gone
	workersDir := filepath.Join(dir, "workers")
	if _, err := os.Stat(workersDir); !os.IsNotExist(err) {
		t.Error("Expected workers directory to be removed")
	}
}

func TestFileState_ClearAllWorkers_NotExists(t *testing.T) {
	fs, _ := newTestState(t)

	// Should not error when workers dir doesn't exist
	if err := fs.ClearAllWorkers(); err != nil {
		t.Errorf("ClearAllWorkers on non-existent dir returned error: %v", err)
	}
}

// ==================== Worker State Updates ====================

func TestFileState_WorkerStatusTransitions(t *testing.T) {
	fs, _ := newTestState(t)

	worker := sampleWorker("worker-1")
	worker.Status = WorkerStatusCreating

	// Save initial state
	if err := fs.SaveWorker(worker); err != nil {
		t.Fatalf("SaveWorker failed: %v", err)
	}

	// Update to idle
	worker.Status = WorkerStatusIdle
	if err := fs.SaveWorker(worker); err != nil {
		t.Fatalf("SaveWorker (idle) failed: %v", err)
	}

	loaded, _ := fs.LoadWorker("worker-1")
	if loaded.Status != WorkerStatusIdle {
		t.Errorf("Status = %s, want %s", loaded.Status, WorkerStatusIdle)
	}

	// Update to busy with task
	worker.Status = WorkerStatusBusy
	worker.CurrentTask = &TaskRef{
		ID:        "task-1",
		Title:     "Do something",
		ClaimedAt: time.Now(),
	}
	worker.CurrentBranch = "feature/task-1"
	if err := fs.SaveWorker(worker); err != nil {
		t.Fatalf("SaveWorker (busy) failed: %v", err)
	}

	loaded, _ = fs.LoadWorker("worker-1")
	if loaded.Status != WorkerStatusBusy {
		t.Errorf("Status = %s, want %s", loaded.Status, WorkerStatusBusy)
	}
	if loaded.CurrentTask == nil || loaded.CurrentTask.ID != "task-1" {
		t.Error("CurrentTask not saved correctly")
	}
	if loaded.CurrentBranch != "feature/task-1" {
		t.Errorf("CurrentBranch = %s, want feature/task-1", loaded.CurrentBranch)
	}
}

func TestFileState_WorkerError(t *testing.T) {
	fs, _ := newTestState(t)

	worker := sampleWorker("worker-1")
	worker.Status = WorkerStatusError
	worker.LastError = "container crashed"
	worker.ErrorTime = time.Now()

	if err := fs.SaveWorker(worker); err != nil {
		t.Fatalf("SaveWorker failed: %v", err)
	}

	loaded, err := fs.LoadWorker("worker-1")
	if err != nil {
		t.Fatalf("LoadWorker failed: %v", err)
	}

	if loaded.Status != WorkerStatusError {
		t.Errorf("Status = %s, want %s", loaded.Status, WorkerStatusError)
	}
	if loaded.LastError != "container crashed" {
		t.Errorf("LastError = %s, want 'container crashed'", loaded.LastError)
	}
	if loaded.ErrorTime.IsZero() {
		t.Error("ErrorTime should not be zero")
	}
}

// ==================== Atomic Write Tests ====================

func TestFileState_AtomicWrite_Session(t *testing.T) {
	fs, dir := newTestState(t)

	session := sampleSession()
	if err := fs.CreateSession(session); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Update session
	session.Status = SessionStatusShuttingDown
	if err := fs.SaveSession(session); err != nil {
		t.Fatalf("SaveSession failed: %v", err)
	}

	// Check no temp file left behind
	matches, _ := filepath.Glob(filepath.Join(dir, "*.tmp"))
	if len(matches) > 0 {
		t.Errorf("Temp files left behind: %v", matches)
	}
}

func TestFileState_AtomicWrite_Worker(t *testing.T) {
	fs, dir := newTestState(t)

	worker := sampleWorker("worker-1")
	if err := fs.SaveWorker(worker); err != nil {
		t.Fatalf("SaveWorker failed: %v", err)
	}

	// Update worker
	worker.Status = WorkerStatusBusy
	if err := fs.SaveWorker(worker); err != nil {
		t.Fatalf("SaveWorker update failed: %v", err)
	}

	// Check no temp file left behind
	workersDir := filepath.Join(dir, "workers")
	matches, _ := filepath.Glob(filepath.Join(workersDir, "*.tmp"))
	if len(matches) > 0 {
		t.Errorf("Temp files left behind: %v", matches)
	}
}

// ==================== JSON Validity Tests ====================

func TestFileState_ValidJSON_Session(t *testing.T) {
	fs, dir := newTestState(t)

	session := sampleSession()
	if err := fs.CreateSession(session); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Read raw file and validate JSON
	data, err := os.ReadFile(filepath.Join(dir, "session.json"))
	if err != nil {
		t.Fatalf("Failed to read session.json: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Errorf("session.json is not valid JSON: %v", err)
	}

	// Check it has version field
	if _, ok := parsed["version"]; !ok {
		t.Error("session.json missing version field")
	}
}

func TestFileState_ValidJSON_Worker(t *testing.T) {
	fs, dir := newTestState(t)

	worker := sampleWorker("worker-1")
	if err := fs.SaveWorker(worker); err != nil {
		t.Fatalf("SaveWorker failed: %v", err)
	}

	// Read raw file and validate JSON
	data, err := os.ReadFile(filepath.Join(dir, "workers", "worker-1.json"))
	if err != nil {
		t.Fatalf("Failed to read worker-1.json: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Errorf("worker-1.json is not valid JSON: %v", err)
	}

	// Check required fields
	if _, ok := parsed["version"]; !ok {
		t.Error("worker-1.json missing version field")
	}
	if _, ok := parsed["name"]; !ok {
		t.Error("worker-1.json missing name field")
	}
}

// ==================== Constructor Tests ====================

func TestNew(t *testing.T) {
	fs := New("/test/project")
	expected := "/test/project/.isollm"
	if fs.stateDir != expected {
		t.Errorf("stateDir = %s, want %s", fs.stateDir, expected)
	}
}

func TestNewWithDir(t *testing.T) {
	fs := NewWithDir("/custom/state/dir")
	if fs.stateDir != "/custom/state/dir" {
		t.Errorf("stateDir = %s, want /custom/state/dir", fs.stateDir)
	}
}

// ==================== Helper Tests ====================

func TestIsProcessAlive_CurrentProcess(t *testing.T) {
	// Current process should be alive
	if !isProcessAlive(os.Getpid()) {
		t.Error("Current process should be alive")
	}
}

func TestIsProcessAlive_InvalidPID(t *testing.T) {
	if isProcessAlive(0) {
		t.Error("PID 0 should not be considered alive")
	}
	if isProcessAlive(-1) {
		t.Error("Negative PID should not be considered alive")
	}
}

func TestIsProcessAlive_NonexistentPID(t *testing.T) {
	// Very high PID that is unlikely to exist
	if isProcessAlive(999999999) {
		t.Error("Nonexistent PID should not be considered alive")
	}
}

// ==================== Edge Cases ====================

func TestFileState_SessionWithAllFields(t *testing.T) {
	fs, _ := newTestState(t)

	session := &Session{
		Status:        SessionStatusInitializing,
		StartedAt:     time.Now().Add(-time.Hour),
		PID:           os.Getpid(),
		ProjectRoot:   "/path/to/project",
		BareRepoPath:  "/path/to/bare/repo",
		ZellijSession: "isollm-abc123",
	}

	if err := fs.CreateSession(session); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	loaded, err := fs.LoadSession()
	if err != nil {
		t.Fatalf("LoadSession failed: %v", err)
	}

	if loaded.ProjectRoot != session.ProjectRoot {
		t.Errorf("ProjectRoot mismatch")
	}
	if loaded.BareRepoPath != session.BareRepoPath {
		t.Errorf("BareRepoPath mismatch")
	}
	if loaded.ZellijSession != session.ZellijSession {
		t.Errorf("ZellijSession mismatch")
	}
}

func TestFileState_WorkerWithAllFields(t *testing.T) {
	fs, _ := newTestState(t)

	now := time.Now()
	worker := &WorkerState{
		Name:        "worker-full",
		ContainerID: "abc123def456",
		Status:      WorkerStatusBusy,
		IP:          "192.168.1.100",
		CurrentTask: &TaskRef{
			ID:        "task-999",
			Title:     "Important task with special chars: <>&\"'",
			ClaimedAt: now.Add(-30 * time.Minute),
		},
		CurrentBranch: "feature/important-task",
		StartedAt:     now.Add(-time.Hour),
		LastActivity:  now,
		LastError:     "",
	}

	if err := fs.SaveWorker(worker); err != nil {
		t.Fatalf("SaveWorker failed: %v", err)
	}

	loaded, err := fs.LoadWorker("worker-full")
	if err != nil {
		t.Fatalf("LoadWorker failed: %v", err)
	}

	if loaded.ContainerID != worker.ContainerID {
		t.Errorf("ContainerID mismatch")
	}
	if loaded.IP != worker.IP {
		t.Errorf("IP mismatch")
	}
	if loaded.CurrentTask.Title != worker.CurrentTask.Title {
		t.Errorf("CurrentTask.Title mismatch")
	}
}

func TestFileState_EmptyWorkerName(t *testing.T) {
	fs, _ := newTestState(t)

	worker := sampleWorker("")
	worker.Name = "" // empty name

	// This should still work (creates ".json" file)
	if err := fs.SaveWorker(worker); err != nil {
		t.Fatalf("SaveWorker with empty name failed: %v", err)
	}

	loaded, err := fs.LoadWorker("")
	if err != nil {
		t.Fatalf("LoadWorker with empty name failed: %v", err)
	}
	if loaded.Name != "" {
		t.Errorf("Expected empty name, got %s", loaded.Name)
	}
}

func TestFileState_SpecialCharactersInWorkerName(t *testing.T) {
	fs, _ := newTestState(t)

	// Worker name with dashes and underscores (common)
	worker := sampleWorker("worker-test_123")
	if err := fs.SaveWorker(worker); err != nil {
		t.Fatalf("SaveWorker failed: %v", err)
	}

	loaded, err := fs.LoadWorker("worker-test_123")
	if err != nil {
		t.Fatalf("LoadWorker failed: %v", err)
	}
	if loaded.Name != "worker-test_123" {
		t.Errorf("Name mismatch")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
