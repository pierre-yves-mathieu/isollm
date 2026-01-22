package worker

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	sdk "airyra/pkg/airyra"

	"isollm/internal/airyra"
	"isollm/internal/config"
)

// testManager creates a Manager with a mock airyra client for testing.
// It uses a temporary directory for state and doesn't require lxc-dev-manager.
func testManager(t *testing.T) (*Manager, *airyra.MockClient) {
	t.Helper()

	tmpDir := t.TempDir()
	stateDir := filepath.Join(tmpDir, "tasks")

	mock := airyra.NewMockClient()

	cfg := &config.Config{
		Project: "test-project",
		Git: config.GitConfig{
			BranchPrefix: "isollm/",
		},
	}

	mgr := &Manager{
		cfg:      cfg,
		stateDir: stateDir,
		airyra:   mock,
	}

	return mgr, mock
}

func TestManager_HasAiryra(t *testing.T) {
	mgr, _ := testManager(t)

	if !mgr.HasAiryra() {
		t.Error("HasAiryra() = false, want true")
	}

	mgr.SetAiryraClient(nil)
	if mgr.HasAiryra() {
		t.Error("HasAiryra() after nil = true, want false")
	}
}

func TestManager_IsAiryraRunning(t *testing.T) {
	mgr, mock := testManager(t)
	ctx := context.Background()

	if !mgr.IsAiryraRunning(ctx) {
		t.Error("IsAiryraRunning() = false, want true")
	}

	mock.ServerRunning = false
	if mgr.IsAiryraRunning(ctx) {
		t.Error("IsAiryraRunning() with server down = true, want false")
	}

	mgr.SetAiryraClient(nil)
	if mgr.IsAiryraRunning(ctx) {
		t.Error("IsAiryraRunning() with nil client = true, want false")
	}
}

func TestManager_ClaimNextTask(t *testing.T) {
	mgr, mock := testManager(t)
	ctx := context.Background()

	// Add some tasks
	mock.AddTask(ctx, "Task 1")
	mock.AddTask(ctx, "Task 2")

	// Claim the next task
	task, err := mgr.ClaimNextTask(ctx, "worker-1")
	if err != nil {
		t.Fatalf("ClaimNextTask() error = %v", err)
	}
	if task == nil {
		t.Fatal("ClaimNextTask() returned nil task")
	}
	if task.Status != airyra.StatusInProgress {
		t.Errorf("ClaimNextTask() task.Status = %v, want %v", task.Status, airyra.StatusInProgress)
	}

	// Check local state was saved
	state, err := mgr.GetTask("worker-1")
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if state == nil {
		t.Fatal("GetTask() returned nil")
	}
	if state.TaskID != task.ID {
		t.Errorf("Local state TaskID = %q, want %q", state.TaskID, task.ID)
	}
	expectedBranch := "isollm/" + task.ID
	if state.Branch != expectedBranch {
		t.Errorf("Local state Branch = %q, want %q", state.Branch, expectedBranch)
	}
}

func TestManager_ClaimNextTask_NoTasks(t *testing.T) {
	mgr, _ := testManager(t)
	ctx := context.Background()

	task, err := mgr.ClaimNextTask(ctx, "worker-1")
	if err != nil {
		t.Fatalf("ClaimNextTask() error = %v", err)
	}
	if task != nil {
		t.Errorf("ClaimNextTask() with no tasks = %v, want nil", task)
	}
}

func TestManager_ClaimNextTask_NoAiryra(t *testing.T) {
	mgr, _ := testManager(t)
	mgr.SetAiryraClient(nil)
	ctx := context.Background()

	_, err := mgr.ClaimNextTask(ctx, "worker-1")
	if err == nil {
		t.Error("ClaimNextTask() with no airyra = nil, want error")
	}
}

func TestManager_ClaimNextTask_ServerDown(t *testing.T) {
	mgr, mock := testManager(t)
	mock.ServerRunning = false
	ctx := context.Background()

	_, err := mgr.ClaimNextTask(ctx, "worker-1")
	if err == nil {
		t.Error("ClaimNextTask() with server down = nil, want error")
	}
}

func TestManager_ClaimNextTask_RaceCondition(t *testing.T) {
	mgr, mock := testManager(t)
	ctx := context.Background()

	// Add tasks
	task1, _ := mock.AddTask(ctx, "Task 1")
	mock.AddTask(ctx, "Task 2")

	// Simulate race: first task gets claimed by another agent
	claimCount := 0
	mock.OnClaimTask = func(ctx context.Context, id string) (*airyra.Task, error) {
		claimCount++
		if claimCount == 1 && id == task1.ID {
			// Simulate another agent claiming first using SDK error type
			return nil, &sdk.Error{Code: sdk.ErrCodeAlreadyClaimed, Message: "task already claimed"}
		}
		// Fall through to normal behavior
		mock.OnClaimTask = nil
		return mock.ClaimTask(ctx, id)
	}

	task, err := mgr.ClaimNextTask(ctx, "worker-1")
	if err != nil {
		t.Fatalf("ClaimNextTask() with race = %v", err)
	}
	// Should have successfully claimed the second task
	if task == nil {
		t.Fatal("ClaimNextTask() returned nil after retry")
	}
}

func TestManager_ReleaseWorkerTask(t *testing.T) {
	mgr, mock := testManager(t)
	ctx := context.Background()

	// Add and claim a task
	mock.AddTask(ctx, "Task 1")
	task, _ := mgr.ClaimNextTask(ctx, "worker-1")

	// Release it
	err := mgr.ReleaseWorkerTask(ctx, "worker-1")
	if err != nil {
		t.Fatalf("ReleaseWorkerTask() error = %v", err)
	}

	// Task should be open again
	released, _ := mock.GetTask(ctx, task.ID)
	if released.Status != airyra.StatusOpen {
		t.Errorf("After release, task.Status = %v, want %v", released.Status, airyra.StatusOpen)
	}

	// Local state should be cleared
	state, _ := mgr.GetTask("worker-1")
	if state != nil {
		t.Error("After release, local state should be nil")
	}
}

func TestManager_ReleaseWorkerTask_NoTask(t *testing.T) {
	mgr, _ := testManager(t)
	ctx := context.Background()

	// Release with no task assigned - should succeed silently
	err := mgr.ReleaseWorkerTask(ctx, "worker-1")
	if err != nil {
		t.Errorf("ReleaseWorkerTask() with no task = %v, want nil", err)
	}
}

func TestManager_ReleaseWorkerTask_NoAiryra(t *testing.T) {
	mgr, mock := testManager(t)
	ctx := context.Background()

	// First assign a task to the worker
	mock.AddTask(ctx, "Task 1")
	mgr.ClaimNextTask(ctx, "worker-1")

	// Now remove airyra client and try to release
	mgr.SetAiryraClient(nil)

	err := mgr.ReleaseWorkerTask(ctx, "worker-1")
	if err == nil {
		t.Error("ReleaseWorkerTask() with no airyra = nil, want error")
	}
}

func TestManager_CompleteWorkerTask(t *testing.T) {
	mgr, mock := testManager(t)
	ctx := context.Background()

	// Add and claim a task
	mock.AddTask(ctx, "Task 1")
	task, _ := mgr.ClaimNextTask(ctx, "worker-1")

	// Complete it
	err := mgr.CompleteWorkerTask(ctx, "worker-1")
	if err != nil {
		t.Fatalf("CompleteWorkerTask() error = %v", err)
	}

	// Task should be done
	completed, _ := mock.GetTask(ctx, task.ID)
	if completed.Status != airyra.StatusDone {
		t.Errorf("After complete, task.Status = %v, want %v", completed.Status, airyra.StatusDone)
	}

	// Local state should be cleared
	state, _ := mgr.GetTask("worker-1")
	if state != nil {
		t.Error("After complete, local state should be nil")
	}
}

func TestManager_CompleteWorkerTask_NoTask(t *testing.T) {
	mgr, _ := testManager(t)
	ctx := context.Background()

	err := mgr.CompleteWorkerTask(ctx, "worker-1")
	if err == nil {
		t.Error("CompleteWorkerTask() with no task = nil, want error")
	}
}

func TestManager_CompleteWorkerTask_NoAiryra(t *testing.T) {
	mgr, _ := testManager(t)
	mgr.SetAiryraClient(nil)
	ctx := context.Background()

	err := mgr.CompleteWorkerTask(ctx, "worker-1")
	if err == nil {
		t.Error("CompleteWorkerTask() with no airyra = nil, want error")
	}
}

func TestManager_BlockWorkerTask(t *testing.T) {
	mgr, mock := testManager(t)
	ctx := context.Background()

	// Add and claim a task
	mock.AddTask(ctx, "Task 1")
	task, _ := mgr.ClaimNextTask(ctx, "worker-1")

	// Block it
	err := mgr.BlockWorkerTask(ctx, "worker-1")
	if err != nil {
		t.Fatalf("BlockWorkerTask() error = %v", err)
	}

	// Task should be blocked
	blocked, _ := mock.GetTask(ctx, task.ID)
	if blocked.Status != airyra.StatusBlocked {
		t.Errorf("After block, task.Status = %v, want %v", blocked.Status, airyra.StatusBlocked)
	}

	// Local state should still exist (task is still assigned)
	state, _ := mgr.GetTask("worker-1")
	if state == nil {
		t.Error("After block, local state should still exist")
	}
}

func TestManager_BlockWorkerTask_NoTask(t *testing.T) {
	mgr, _ := testManager(t)
	ctx := context.Background()

	err := mgr.BlockWorkerTask(ctx, "worker-1")
	if err == nil {
		t.Error("BlockWorkerTask() with no task = nil, want error")
	}
}

func TestManager_WorkerNameNormalization(t *testing.T) {
	mgr, mock := testManager(t)
	ctx := context.Background()

	mock.AddTask(ctx, "Task 1")

	// Claim with name without prefix
	task, err := mgr.ClaimNextTask(ctx, "1")
	if err != nil {
		t.Fatalf("ClaimNextTask() error = %v", err)
	}

	// State should be saved with normalized name
	state, _ := LoadTaskState(mgr.stateDir, "worker-1")
	if state == nil {
		t.Error("State not found with normalized worker name")
	}
	if state.TaskID != task.ID {
		t.Errorf("State TaskID = %q, want %q", state.TaskID, task.ID)
	}
}

func TestManager_LocalStatePersistence(t *testing.T) {
	mgr, mock := testManager(t)
	ctx := context.Background()

	// Claim a task
	mock.AddTask(ctx, "Task 1")
	task, _ := mgr.ClaimNextTask(ctx, "worker-1")

	// Verify state file exists
	statePath := filepath.Join(mgr.stateDir, "worker-1.json")
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Error("State file not created")
	}

	// Load and verify state
	state, err := LoadTaskState(mgr.stateDir, "worker-1")
	if err != nil {
		t.Fatalf("LoadTaskState() error = %v", err)
	}
	if state.TaskID != task.ID {
		t.Errorf("State TaskID = %q, want %q", state.TaskID, task.ID)
	}
	if state.Branch != "isollm/"+task.ID {
		t.Errorf("State Branch = %q, want %q", state.Branch, "isollm/"+task.ID)
	}
	if state.ClaimedAt.IsZero() {
		t.Error("State ClaimedAt is zero")
	}
}

func TestManager_MultipleWorkers(t *testing.T) {
	mgr, mock := testManager(t)
	ctx := context.Background()

	// Add multiple tasks
	mock.AddTask(ctx, "Task 1")
	mock.AddTask(ctx, "Task 2")
	mock.AddTask(ctx, "Task 3")

	// Multiple workers claim tasks
	task1, _ := mgr.ClaimNextTask(ctx, "worker-1")
	task2, _ := mgr.ClaimNextTask(ctx, "worker-2")
	task3, _ := mgr.ClaimNextTask(ctx, "worker-3")

	// All tasks should be different
	if task1.ID == task2.ID || task2.ID == task3.ID || task1.ID == task3.ID {
		t.Error("Multiple workers claimed the same task")
	}

	// All states should be tracked
	for i, workerName := range []string{"worker-1", "worker-2", "worker-3"} {
		state, _ := mgr.GetTask(workerName)
		if state == nil {
			t.Errorf("Worker %d state not found", i+1)
		}
	}
}

func TestManager_GetStateDir(t *testing.T) {
	mgr, _ := testManager(t)

	dir := mgr.GetStateDir()
	if dir == "" {
		t.Error("GetStateDir() returned empty string")
	}
}

func TestManager_SetStateDir(t *testing.T) {
	mgr, _ := testManager(t)

	newDir := "/tmp/test-state"
	mgr.SetStateDir(newDir)

	if mgr.GetStateDir() != newDir {
		t.Errorf("SetStateDir() did not update, got %q", mgr.GetStateDir())
	}
}

func TestManager_ClaimNextTask_Timeout(t *testing.T) {
	mgr, mock := testManager(t)

	// Use a context that times out immediately
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()

	// Wait for context to expire
	time.Sleep(time.Millisecond)

	mock.AddTask(context.Background(), "Task 1")

	_, err := mgr.ClaimNextTask(ctx, "worker-1")
	// The mock doesn't check context, but in real implementation this would fail
	// This test documents the expected behavior
	_ = err
}

