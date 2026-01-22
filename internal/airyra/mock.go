package airyra

import (
	"context"
	"fmt"
	"sync"
	"time"

	sdk "airyra/pkg/airyra"
)

// MockClient implements a mock airyra client for testing.
// It maintains an in-memory task store and supports hooks for customizing behavior.
type MockClient struct {
	mu      sync.RWMutex
	tasks   map[string]*Task
	deps    map[string][]Dependency // childID -> dependencies
	nextID  int
	agentID string

	// Behavior flags
	ServerRunning bool

	// Hooks for testing specific behaviors (called if non-nil)
	OnHealth       func(ctx context.Context) error
	OnAddTask      func(ctx context.Context, title string, opts ...sdk.CreateTaskOption) (*Task, error)
	OnGetTask      func(ctx context.Context, id string) (*Task, error)
	OnListTasks    func(ctx context.Context, opts ...sdk.ListTasksOption) (*TaskList, error)
	OnClaimTask    func(ctx context.Context, id string) (*Task, error)
	OnCompleteTask func(ctx context.Context, id string) (*Task, error)
	OnReleaseTask  func(ctx context.Context, id string, force bool) (*Task, error)
	OnBlockTask    func(ctx context.Context, id string) (*Task, error)
	OnDeleteTask   func(ctx context.Context, id string) error
}

// NewMockClient creates a new mock client with default behavior.
func NewMockClient() *MockClient {
	return &MockClient{
		tasks:         make(map[string]*Task),
		deps:          make(map[string][]Dependency),
		nextID:        1,
		agentID:       "test-agent",
		ServerRunning: true,
	}
}

// SetAgentID sets the agent ID for task ownership.
func (m *MockClient) SetAgentID(agentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agentID = agentID
}

// Health checks if the mock server is "running".
func (m *MockClient) Health(ctx context.Context) error {
	if m.OnHealth != nil {
		return m.OnHealth(ctx)
	}
	if !m.ServerRunning {
		return ErrServerNotRunning
	}
	return nil
}

// IsServerRunning returns true if the mock server is healthy.
func (m *MockClient) IsServerRunning(ctx context.Context) bool {
	return m.Health(ctx) == nil
}

// AddTask creates a new task.
func (m *MockClient) AddTask(ctx context.Context, title string, opts ...sdk.CreateTaskOption) (*Task, error) {
	if m.OnAddTask != nil {
		return m.OnAddTask(ctx, title, opts...)
	}
	if !m.ServerRunning {
		return nil, ErrServerNotRunning
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	id := fmt.Sprintf("ar-%04d", m.nextID)
	m.nextID++

	now := time.Now()
	task := &Task{
		ID:        id,
		Title:     title,
		Status:    StatusOpen,
		Priority:  PriorityNormal,
		CreatedAt: now,
		UpdatedAt: now,
	}

	m.tasks[id] = task
	return task, nil
}

// GetTask retrieves a task by ID.
func (m *MockClient) GetTask(ctx context.Context, id string) (*Task, error) {
	if m.OnGetTask != nil {
		return m.OnGetTask(ctx, id)
	}
	if !m.ServerRunning {
		return nil, ErrServerNotRunning
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.tasks[id]
	if !ok {
		return nil, &sdk.Error{Code: sdk.ErrCodeTaskNotFound, Message: "task not found"}
	}
	return task, nil
}

// ListTasks lists tasks with optional filtering.
func (m *MockClient) ListTasks(ctx context.Context, opts ...sdk.ListTasksOption) (*TaskList, error) {
	if m.OnListTasks != nil {
		return m.OnListTasks(ctx, opts...)
	}
	if !m.ServerRunning {
		return nil, ErrServerNotRunning
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var tasks []*Task
	for _, t := range m.tasks {
		tasks = append(tasks, t)
	}

	return &TaskList{
		Tasks:      tasks,
		Page:       1,
		PerPage:    len(tasks),
		Total:      len(tasks),
		TotalPages: 1,
	}, nil
}

// ListReadyTasks lists tasks ready to be claimed.
func (m *MockClient) ListReadyTasks(ctx context.Context) (*TaskList, error) {
	if !m.ServerRunning {
		return nil, ErrServerNotRunning
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var tasks []*Task
	for _, t := range m.tasks {
		if t.Status == StatusOpen && t.ClaimedBy == nil {
			// Check if all dependencies are satisfied
			if m.areDependenciesSatisfied(t.ID) {
				tasks = append(tasks, t)
			}
		}
	}

	return &TaskList{
		Tasks:      tasks,
		Page:       1,
		PerPage:    len(tasks),
		Total:      len(tasks),
		TotalPages: 1,
	}, nil
}

// DeleteTask deletes a task.
func (m *MockClient) DeleteTask(ctx context.Context, id string) error {
	if m.OnDeleteTask != nil {
		return m.OnDeleteTask(ctx, id)
	}
	if !m.ServerRunning {
		return ErrServerNotRunning
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.tasks[id]; !ok {
		return &sdk.Error{Code: sdk.ErrCodeTaskNotFound, Message: "task not found"}
	}

	delete(m.tasks, id)
	delete(m.deps, id)
	return nil
}

// ClearDoneTasks deletes all completed tasks.
func (m *MockClient) ClearDoneTasks(ctx context.Context) (int, error) {
	if !m.ServerRunning {
		return 0, ErrServerNotRunning
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for id, t := range m.tasks {
		if t.Status == StatusDone {
			delete(m.tasks, id)
			delete(m.deps, id)
			count++
		}
	}
	return count, nil
}

// ClearAllTasks deletes all tasks.
func (m *MockClient) ClearAllTasks(ctx context.Context) (int, error) {
	if !m.ServerRunning {
		return 0, ErrServerNotRunning
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	count := len(m.tasks)
	m.tasks = make(map[string]*Task)
	m.deps = make(map[string][]Dependency)
	return count, nil
}

// ClaimTask claims a task for this agent.
func (m *MockClient) ClaimTask(ctx context.Context, id string) (*Task, error) {
	if m.OnClaimTask != nil {
		return m.OnClaimTask(ctx, id)
	}
	if !m.ServerRunning {
		return nil, ErrServerNotRunning
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[id]
	if !ok {
		return nil, &sdk.Error{Code: sdk.ErrCodeTaskNotFound, Message: "task not found"}
	}

	if task.ClaimedBy != nil {
		return nil, &sdk.Error{Code: sdk.ErrCodeAlreadyClaimed, Message: "task already claimed"}
	}

	if task.Status != StatusOpen {
		return nil, &sdk.Error{Code: sdk.ErrCodeInvalidTransition, Message: "can only claim open tasks"}
	}

	now := time.Now()
	agentCopy := m.agentID // Make a copy so SetAgentID doesn't affect this
	task.ClaimedBy = &agentCopy
	task.ClaimedAt = &now
	task.Status = StatusInProgress
	task.UpdatedAt = now

	return task, nil
}

// CompleteTask marks a task as done.
func (m *MockClient) CompleteTask(ctx context.Context, id string) (*Task, error) {
	if m.OnCompleteTask != nil {
		return m.OnCompleteTask(ctx, id)
	}
	if !m.ServerRunning {
		return nil, ErrServerNotRunning
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[id]
	if !ok {
		return nil, &sdk.Error{Code: sdk.ErrCodeTaskNotFound, Message: "task not found"}
	}

	if task.ClaimedBy == nil || *task.ClaimedBy != m.agentID {
		return nil, &sdk.Error{Code: sdk.ErrCodeNotOwner, Message: "not the owner"}
	}

	if task.Status != StatusInProgress {
		return nil, &sdk.Error{Code: sdk.ErrCodeInvalidTransition, Message: "can only complete in_progress tasks"}
	}

	task.Status = StatusDone
	task.UpdatedAt = time.Now()

	return task, nil
}

// ReleaseTask releases a claimed task.
func (m *MockClient) ReleaseTask(ctx context.Context, id string, force bool) (*Task, error) {
	if m.OnReleaseTask != nil {
		return m.OnReleaseTask(ctx, id, force)
	}
	if !m.ServerRunning {
		return nil, ErrServerNotRunning
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[id]
	if !ok {
		return nil, &sdk.Error{Code: sdk.ErrCodeTaskNotFound, Message: "task not found"}
	}

	if !force && (task.ClaimedBy == nil || *task.ClaimedBy != m.agentID) {
		return nil, &sdk.Error{Code: sdk.ErrCodeNotOwner, Message: "not the owner"}
	}

	task.ClaimedBy = nil
	task.ClaimedAt = nil
	task.Status = StatusOpen
	task.UpdatedAt = time.Now()

	return task, nil
}

// BlockTask marks a task as blocked.
func (m *MockClient) BlockTask(ctx context.Context, id string) (*Task, error) {
	if m.OnBlockTask != nil {
		return m.OnBlockTask(ctx, id)
	}
	if !m.ServerRunning {
		return nil, ErrServerNotRunning
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[id]
	if !ok {
		return nil, &sdk.Error{Code: sdk.ErrCodeTaskNotFound, Message: "task not found"}
	}

	if task.ClaimedBy == nil || *task.ClaimedBy != m.agentID {
		return nil, &sdk.Error{Code: sdk.ErrCodeNotOwner, Message: "not the owner"}
	}

	task.Status = StatusBlocked
	task.UpdatedAt = time.Now()

	return task, nil
}

// UnblockTask unblocks a blocked task.
func (m *MockClient) UnblockTask(ctx context.Context, id string) (*Task, error) {
	if !m.ServerRunning {
		return nil, ErrServerNotRunning
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[id]
	if !ok {
		return nil, &sdk.Error{Code: sdk.ErrCodeTaskNotFound, Message: "task not found"}
	}

	if task.Status != StatusBlocked {
		return nil, &sdk.Error{Code: sdk.ErrCodeInvalidTransition, Message: "can only unblock blocked tasks"}
	}

	task.Status = StatusInProgress
	task.UpdatedAt = time.Now()

	return task, nil
}

// AddDependency adds a dependency (child depends on parent).
func (m *MockClient) AddDependency(ctx context.Context, childID, parentID string) error {
	if !m.ServerRunning {
		return ErrServerNotRunning
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check both tasks exist
	if _, ok := m.tasks[childID]; !ok {
		return &sdk.Error{Code: sdk.ErrCodeTaskNotFound, Message: "child task not found"}
	}
	if _, ok := m.tasks[parentID]; !ok {
		return &sdk.Error{Code: sdk.ErrCodeTaskNotFound, Message: "parent task not found"}
	}

	// Check for cycles (simple check)
	if childID == parentID {
		return &sdk.Error{Code: sdk.ErrCodeCycleDetected, Message: "cannot depend on self"}
	}

	dep := Dependency{ChildID: childID, ParentID: parentID}
	m.deps[childID] = append(m.deps[childID], dep)
	return nil
}

// RemoveDependency removes a dependency.
func (m *MockClient) RemoveDependency(ctx context.Context, childID, parentID string) error {
	if !m.ServerRunning {
		return ErrServerNotRunning
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	deps := m.deps[childID]
	for i, d := range deps {
		if d.ParentID == parentID {
			m.deps[childID] = append(deps[:i], deps[i+1:]...)
			return nil
		}
	}

	return &sdk.Error{Code: sdk.ErrCodeDependencyNotFound, Message: "dependency not found"}
}

// ListDependencies lists dependencies for a task.
func (m *MockClient) ListDependencies(ctx context.Context, taskID string) ([]Dependency, error) {
	if !m.ServerRunning {
		return nil, ErrServerNotRunning
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.deps[taskID], nil
}

// Helper methods for testing

// AddTaskDirect adds a task directly to the mock store (for test setup).
func (m *MockClient) AddTaskDirect(task *Task) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tasks[task.ID] = task
}

// GetTaskCount returns the number of tasks in the store.
func (m *MockClient) GetTaskCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.tasks)
}

// Reset clears all tasks and resets the mock state.
func (m *MockClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tasks = make(map[string]*Task)
	m.deps = make(map[string][]Dependency)
	m.nextID = 1
	m.ServerRunning = true
}

// areDependenciesSatisfied checks if all dependencies of a task are done.
// Must be called with lock held.
func (m *MockClient) areDependenciesSatisfied(taskID string) bool {
	deps := m.deps[taskID]
	for _, d := range deps {
		parent, ok := m.tasks[d.ParentID]
		if !ok || parent.Status != StatusDone {
			return false
		}
	}
	return true
}
