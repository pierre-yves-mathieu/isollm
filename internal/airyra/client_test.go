package airyra

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestDefaultAgentID(t *testing.T) {
	agentID := defaultAgentID()

	if agentID == "" {
		t.Error("defaultAgentID() returned empty string")
	}

	// Should contain @ and :
	hasAt := false
	hasColon := false
	for _, c := range agentID {
		if c == '@' {
			hasAt = true
		}
		if c == ':' {
			hasColon = true
		}
	}

	if !hasAt {
		t.Errorf("defaultAgentID() = %q, expected to contain '@'", agentID)
	}
	if !hasColon {
		t.Errorf("defaultAgentID() = %q, expected to contain ':'", agentID)
	}
}

func TestDefaultAgentIDContainsUser(t *testing.T) {
	user := os.Getenv("USER")
	if user == "" {
		t.Skip("USER environment variable not set")
	}

	agentID := defaultAgentID()
	// Check that it starts with the username
	if len(agentID) < len(user) || agentID[:len(user)] != user {
		t.Errorf("defaultAgentID() = %q, expected to start with %q", agentID, user)
	}
}

// MockClient tests

func TestMockClient_Health(t *testing.T) {
	mock := NewMockClient()
	ctx := context.Background()

	// Server running
	if err := mock.Health(ctx); err != nil {
		t.Errorf("Health() with server running = %v, want nil", err)
	}

	// Server not running
	mock.ServerRunning = false
	if err := mock.Health(ctx); err == nil {
		t.Error("Health() with server not running = nil, want error")
	}
}

func TestMockClient_IsServerRunning(t *testing.T) {
	mock := NewMockClient()
	ctx := context.Background()

	if !mock.IsServerRunning(ctx) {
		t.Error("IsServerRunning() = false, want true")
	}

	mock.ServerRunning = false
	if mock.IsServerRunning(ctx) {
		t.Error("IsServerRunning() = true, want false")
	}
}

func TestMockClient_AddTask(t *testing.T) {
	mock := NewMockClient()
	ctx := context.Background()

	task, err := mock.AddTask(ctx, "Test task")
	if err != nil {
		t.Fatalf("AddTask() error = %v", err)
	}

	if task.ID == "" {
		t.Error("AddTask() returned task with empty ID")
	}
	if task.Title != "Test task" {
		t.Errorf("AddTask() title = %q, want %q", task.Title, "Test task")
	}
	if task.Status != StatusOpen {
		t.Errorf("AddTask() status = %v, want %v", task.Status, StatusOpen)
	}
	if task.Priority != PriorityNormal {
		t.Errorf("AddTask() priority = %d, want %d", task.Priority, PriorityNormal)
	}
}

func TestMockClient_AddTaskServerDown(t *testing.T) {
	mock := NewMockClient()
	mock.ServerRunning = false
	ctx := context.Background()

	_, err := mock.AddTask(ctx, "Test task")
	if err == nil {
		t.Error("AddTask() with server down = nil, want error")
	}
}

func TestMockClient_GetTask(t *testing.T) {
	mock := NewMockClient()
	ctx := context.Background()

	// Create a task
	created, _ := mock.AddTask(ctx, "Test task")

	// Get it back
	task, err := mock.GetTask(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if task.ID != created.ID {
		t.Errorf("GetTask() ID = %q, want %q", task.ID, created.ID)
	}
}

func TestMockClient_GetTaskNotFound(t *testing.T) {
	mock := NewMockClient()
	ctx := context.Background()

	_, err := mock.GetTask(ctx, "nonexistent")
	if err == nil {
		t.Error("GetTask() for nonexistent task = nil, want error")
	}
	if !IsTaskNotFound(err) {
		t.Errorf("GetTask() error = %v, want TaskNotFound error", err)
	}
}

func TestMockClient_ListTasks(t *testing.T) {
	mock := NewMockClient()
	ctx := context.Background()

	// Create some tasks
	mock.AddTask(ctx, "Task 1")
	mock.AddTask(ctx, "Task 2")
	mock.AddTask(ctx, "Task 3")

	list, err := mock.ListTasks(ctx)
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}
	if list.Total != 3 {
		t.Errorf("ListTasks() total = %d, want 3", list.Total)
	}
	if len(list.Tasks) != 3 {
		t.Errorf("ListTasks() len(tasks) = %d, want 3", len(list.Tasks))
	}
}

func TestMockClient_ListReadyTasks(t *testing.T) {
	mock := NewMockClient()
	ctx := context.Background()

	// Create tasks
	task1, _ := mock.AddTask(ctx, "Task 1")
	mock.AddTask(ctx, "Task 2")

	// Claim one
	mock.ClaimTask(ctx, task1.ID)

	// Only unclaimed should be ready
	list, err := mock.ListReadyTasks(ctx)
	if err != nil {
		t.Fatalf("ListReadyTasks() error = %v", err)
	}
	if list.Total != 1 {
		t.Errorf("ListReadyTasks() total = %d, want 1", list.Total)
	}
}

func TestMockClient_DeleteTask(t *testing.T) {
	mock := NewMockClient()
	ctx := context.Background()

	task, _ := mock.AddTask(ctx, "Test task")

	if err := mock.DeleteTask(ctx, task.ID); err != nil {
		t.Fatalf("DeleteTask() error = %v", err)
	}

	// Should not be found anymore
	_, err := mock.GetTask(ctx, task.ID)
	if !IsTaskNotFound(err) {
		t.Errorf("GetTask() after delete = %v, want TaskNotFound", err)
	}
}

func TestMockClient_DeleteTaskNotFound(t *testing.T) {
	mock := NewMockClient()
	ctx := context.Background()

	err := mock.DeleteTask(ctx, "nonexistent")
	if !IsTaskNotFound(err) {
		t.Errorf("DeleteTask() for nonexistent = %v, want TaskNotFound", err)
	}
}

func TestMockClient_ClaimTask(t *testing.T) {
	mock := NewMockClient()
	ctx := context.Background()

	task, _ := mock.AddTask(ctx, "Test task")

	claimed, err := mock.ClaimTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("ClaimTask() error = %v", err)
	}
	if claimed.Status != StatusInProgress {
		t.Errorf("ClaimTask() status = %v, want %v", claimed.Status, StatusInProgress)
	}
	if claimed.ClaimedBy == nil {
		t.Error("ClaimTask() ClaimedBy = nil, want non-nil")
	}
	if claimed.ClaimedAt == nil {
		t.Error("ClaimTask() ClaimedAt = nil, want non-nil")
	}
}

func TestMockClient_ClaimTaskAlreadyClaimed(t *testing.T) {
	mock := NewMockClient()
	ctx := context.Background()

	task, _ := mock.AddTask(ctx, "Test task")
	mock.ClaimTask(ctx, task.ID)

	// Try to claim again with different agent
	mock.SetAgentID("other-agent")
	_, err := mock.ClaimTask(ctx, task.ID)
	if err == nil {
		t.Error("ClaimTask() already claimed = nil, want error")
	}
	if !IsAlreadyClaimed(err) {
		t.Errorf("ClaimTask() error = %v, want AlreadyClaimed", err)
	}
}

func TestMockClient_CompleteTask(t *testing.T) {
	mock := NewMockClient()
	ctx := context.Background()

	task, _ := mock.AddTask(ctx, "Test task")
	mock.ClaimTask(ctx, task.ID)

	completed, err := mock.CompleteTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("CompleteTask() error = %v", err)
	}
	if completed.Status != StatusDone {
		t.Errorf("CompleteTask() status = %v, want %v", completed.Status, StatusDone)
	}
}

func TestMockClient_CompleteTaskNotOwner(t *testing.T) {
	mock := NewMockClient()
	ctx := context.Background()

	task, _ := mock.AddTask(ctx, "Test task")
	mock.ClaimTask(ctx, task.ID)

	// Change agent and try to complete
	mock.SetAgentID("other-agent")
	_, err := mock.CompleteTask(ctx, task.ID)
	if err == nil {
		t.Error("CompleteTask() as non-owner = nil, want error")
	}
	if !IsNotOwner(err) {
		t.Errorf("CompleteTask() error = %v, want NotOwner", err)
	}
}

func TestMockClient_ReleaseTask(t *testing.T) {
	mock := NewMockClient()
	ctx := context.Background()

	task, _ := mock.AddTask(ctx, "Test task")
	mock.ClaimTask(ctx, task.ID)

	released, err := mock.ReleaseTask(ctx, task.ID, false)
	if err != nil {
		t.Fatalf("ReleaseTask() error = %v", err)
	}
	if released.Status != StatusOpen {
		t.Errorf("ReleaseTask() status = %v, want %v", released.Status, StatusOpen)
	}
	if released.ClaimedBy != nil {
		t.Error("ReleaseTask() ClaimedBy = non-nil, want nil")
	}
}

func TestMockClient_ReleaseTaskForce(t *testing.T) {
	mock := NewMockClient()
	ctx := context.Background()

	task, _ := mock.AddTask(ctx, "Test task")
	mock.ClaimTask(ctx, task.ID)

	// Change agent and force release
	mock.SetAgentID("other-agent")
	released, err := mock.ReleaseTask(ctx, task.ID, true)
	if err != nil {
		t.Fatalf("ReleaseTask(force=true) error = %v", err)
	}
	if released.Status != StatusOpen {
		t.Errorf("ReleaseTask(force=true) status = %v, want %v", released.Status, StatusOpen)
	}
}

func TestMockClient_BlockTask(t *testing.T) {
	mock := NewMockClient()
	ctx := context.Background()

	task, _ := mock.AddTask(ctx, "Test task")
	mock.ClaimTask(ctx, task.ID)

	blocked, err := mock.BlockTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("BlockTask() error = %v", err)
	}
	if blocked.Status != StatusBlocked {
		t.Errorf("BlockTask() status = %v, want %v", blocked.Status, StatusBlocked)
	}
}

func TestMockClient_UnblockTask(t *testing.T) {
	mock := NewMockClient()
	ctx := context.Background()

	task, _ := mock.AddTask(ctx, "Test task")
	mock.ClaimTask(ctx, task.ID)
	mock.BlockTask(ctx, task.ID)

	unblocked, err := mock.UnblockTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("UnblockTask() error = %v", err)
	}
	if unblocked.Status != StatusInProgress {
		t.Errorf("UnblockTask() status = %v, want %v", unblocked.Status, StatusInProgress)
	}
}

func TestMockClient_ClearDoneTasks(t *testing.T) {
	mock := NewMockClient()
	ctx := context.Background()

	// Create and complete some tasks
	task1, _ := mock.AddTask(ctx, "Task 1")
	task2, _ := mock.AddTask(ctx, "Task 2")
	mock.AddTask(ctx, "Task 3") // Leave this one open

	mock.ClaimTask(ctx, task1.ID)
	mock.CompleteTask(ctx, task1.ID)

	mock.ClaimTask(ctx, task2.ID)
	mock.CompleteTask(ctx, task2.ID)

	count, err := mock.ClearDoneTasks(ctx)
	if err != nil {
		t.Fatalf("ClearDoneTasks() error = %v", err)
	}
	if count != 2 {
		t.Errorf("ClearDoneTasks() = %d, want 2", count)
	}
	if mock.GetTaskCount() != 1 {
		t.Errorf("After ClearDoneTasks() task count = %d, want 1", mock.GetTaskCount())
	}
}

func TestMockClient_ClearAllTasks(t *testing.T) {
	mock := NewMockClient()
	ctx := context.Background()

	mock.AddTask(ctx, "Task 1")
	mock.AddTask(ctx, "Task 2")
	mock.AddTask(ctx, "Task 3")

	count, err := mock.ClearAllTasks(ctx)
	if err != nil {
		t.Fatalf("ClearAllTasks() error = %v", err)
	}
	if count != 3 {
		t.Errorf("ClearAllTasks() = %d, want 3", count)
	}
	if mock.GetTaskCount() != 0 {
		t.Errorf("After ClearAllTasks() task count = %d, want 0", mock.GetTaskCount())
	}
}

func TestMockClient_Dependencies(t *testing.T) {
	mock := NewMockClient()
	ctx := context.Background()

	parent, _ := mock.AddTask(ctx, "Parent task")
	child, _ := mock.AddTask(ctx, "Child task")

	// Add dependency
	if err := mock.AddDependency(ctx, child.ID, parent.ID); err != nil {
		t.Fatalf("AddDependency() error = %v", err)
	}

	// List dependencies
	deps, err := mock.ListDependencies(ctx, child.ID)
	if err != nil {
		t.Fatalf("ListDependencies() error = %v", err)
	}
	if len(deps) != 1 {
		t.Errorf("ListDependencies() len = %d, want 1", len(deps))
	}
	if deps[0].ParentID != parent.ID {
		t.Errorf("Dependency ParentID = %q, want %q", deps[0].ParentID, parent.ID)
	}

	// Remove dependency
	if err := mock.RemoveDependency(ctx, child.ID, parent.ID); err != nil {
		t.Fatalf("RemoveDependency() error = %v", err)
	}

	deps, _ = mock.ListDependencies(ctx, child.ID)
	if len(deps) != 0 {
		t.Errorf("After RemoveDependency() len = %d, want 0", len(deps))
	}
}

func TestMockClient_DependencyCycle(t *testing.T) {
	mock := NewMockClient()
	ctx := context.Background()

	task, _ := mock.AddTask(ctx, "Task")

	// Self-dependency should fail
	err := mock.AddDependency(ctx, task.ID, task.ID)
	if err == nil {
		t.Error("AddDependency(self) = nil, want error")
	}
	if !IsCycleDetected(err) {
		t.Errorf("AddDependency(self) error = %v, want CycleDetected", err)
	}
}

func TestMockClient_ListReadyTasksWithDependencies(t *testing.T) {
	mock := NewMockClient()
	ctx := context.Background()

	parent, _ := mock.AddTask(ctx, "Parent task")
	child, _ := mock.AddTask(ctx, "Child task")
	mock.AddDependency(ctx, child.ID, parent.ID)

	// Child should not be ready (parent not done)
	ready, _ := mock.ListReadyTasks(ctx)
	for _, task := range ready.Tasks {
		if task.ID == child.ID {
			t.Error("Child task should not be ready when parent is not done")
		}
	}

	// Complete parent
	mock.ClaimTask(ctx, parent.ID)
	mock.CompleteTask(ctx, parent.ID)

	// Now child should be ready
	ready, _ = mock.ListReadyTasks(ctx)
	found := false
	for _, task := range ready.Tasks {
		if task.ID == child.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("Child task should be ready after parent is done")
	}
}

func TestMockClient_Reset(t *testing.T) {
	mock := NewMockClient()
	ctx := context.Background()

	mock.AddTask(ctx, "Task 1")
	mock.AddTask(ctx, "Task 2")
	mock.ServerRunning = false

	mock.Reset()

	if mock.GetTaskCount() != 0 {
		t.Errorf("After Reset() task count = %d, want 0", mock.GetTaskCount())
	}
	if !mock.ServerRunning {
		t.Error("After Reset() ServerRunning = false, want true")
	}
}

func TestMockClient_Hooks(t *testing.T) {
	mock := NewMockClient()
	ctx := context.Background()

	// Test OnHealth hook
	healthCalled := false
	mock.OnHealth = func(ctx context.Context) error {
		healthCalled = true
		return nil
	}
	mock.Health(ctx)
	if !healthCalled {
		t.Error("OnHealth hook not called")
	}

	// OnAddTask hook tested implicitly through other tests using hooks
}

func TestMockClient_Concurrent(t *testing.T) {
	mock := NewMockClient()
	ctx := context.Background()

	// Create tasks concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(i int) {
			mock.AddTask(ctx, "Task")
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("Timeout waiting for concurrent AddTask")
		}
	}

	if mock.GetTaskCount() != 10 {
		t.Errorf("After concurrent AddTask, count = %d, want 10", mock.GetTaskCount())
	}
}
