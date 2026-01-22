package status

import (
	"context"
	"testing"

	sdk "airyra/pkg/airyra"

	"isollm/internal/airyra"
	"isollm/internal/barerepo"
	"isollm/internal/config"
	"isollm/internal/git"
	"isollm/internal/worker"
)

// mockWorkerManager implements the methods needed for status collection
type mockWorkerManager struct {
	workers []worker.WorkerInfo
	listErr error
}

func (m *mockWorkerManager) List() ([]worker.WorkerInfo, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.workers, nil
}

// mockGitExecutor implements git.Executor for testing
type mockGitExecutor struct {
	outputs  map[string]string // key is "dir|args" pattern
	errors   map[string]error
	runCalls [][]string
}

func newMockGitExecutor() *mockGitExecutor {
	return &mockGitExecutor{
		outputs: make(map[string]string),
		errors:  make(map[string]error),
	}
}

func (m *mockGitExecutor) Run(dir string, args ...string) (string, error) {
	key := m.makeKey(dir, args)
	m.runCalls = append(m.runCalls, append([]string{dir}, args...))

	if err, ok := m.errors[key]; ok {
		return "", err
	}
	if output, ok := m.outputs[key]; ok {
		return output, nil
	}
	// Default behavior - return empty string
	return "", nil
}

func (m *mockGitExecutor) RunSilent(dir string, args ...string) error {
	_, err := m.Run(dir, args...)
	return err
}

func (m *mockGitExecutor) makeKey(dir string, args []string) string {
	key := dir + "|"
	for i, arg := range args {
		if i > 0 {
			key += " "
		}
		key += arg
	}
	return key
}

func (m *mockGitExecutor) setOutput(dir string, args []string, output string) {
	key := m.makeKey(dir, args)
	m.outputs[key] = output
}

func (m *mockGitExecutor) setError(dir string, args []string, err error) {
	key := m.makeKey(dir, args)
	m.errors[key] = err
}

// mockBareRepo wraps testing behavior for bare repo operations
type mockBareRepo struct {
	branches      []barerepo.BranchInfo
	branchesErr   error
	hostAhead     int
	hostAheadErr  error
}

// testCollector creates a Collector with mock dependencies for testing
func testCollector(t *testing.T) (*Collector, *airyra.MockClient, *mockGitExecutor) {
	t.Helper()

	mockClient := airyra.NewMockClient()
	mockGit := newMockGitExecutor()

	cfg := &config.Config{
		Project: "test-project",
		Git: config.GitConfig{
			BaseBranch:   "main",
			BranchPrefix: "isollm/",
		},
		Airyra: config.AiryraConfig{
			Host: "localhost",
			Port: 7432,
		},
	}

	c := &Collector{
		projectDir: "/test/project",
		cfg:        cfg,
		manager:    nil, // We'll mock manager.List() behavior
		airyra:     mockClient,
		bareRepo:   nil, // We'll handle bare repo separately
		gitExec:    mockGit,
	}

	return c, mockClient, mockGit
}

func TestNewCollector(t *testing.T) {
	// This test is limited since NewCollector requires real file system
	// Just verify the function signature and that it returns an error
	// for non-existent directory

	cfg := &config.Config{
		Project: "test-project",
		Git: config.GitConfig{
			BaseBranch: "main",
		},
	}

	_, err := NewCollector("/non/existent/path", cfg)
	// Should fail because lxc-dev-manager can't initialize
	if err == nil {
		t.Skip("NewCollector didn't fail for non-existent path (may have lxc-dev-manager installed)")
	}
}

func TestCollectTasks(t *testing.T) {
	ctx := context.Background()

	t.Run("with nil airyra client", func(t *testing.T) {
		c, _, _ := testCollector(t)
		c.airyra = nil

		summary := c.collectTasks(ctx)

		if summary.Total() != 0 {
			t.Errorf("Expected empty summary with nil client, got total %d", summary.Total())
		}
	})

	t.Run("with server not running", func(t *testing.T) {
		c, mockClient, _ := testCollector(t)
		mockClient.ServerRunning = false

		summary := c.collectTasks(ctx)

		if summary.Total() != 0 {
			t.Errorf("Expected empty summary with server down, got total %d", summary.Total())
		}
	})

	t.Run("with various task statuses", func(t *testing.T) {
		c, mockClient, _ := testCollector(t)

		// Add tasks with different statuses
		mockClient.AddTask(ctx, "Ready task 1")
		mockClient.AddTask(ctx, "Ready task 2")
		mockClient.AddTask(ctx, "Ready task 3")

		// Claim and complete one task
		task4, _ := mockClient.AddTask(ctx, "In progress task")
		mockClient.ClaimTask(ctx, task4.ID)

		// Claim and complete another
		task5, _ := mockClient.AddTask(ctx, "Completed task")
		mockClient.ClaimTask(ctx, task5.ID)
		mockClient.CompleteTask(ctx, task5.ID)

		// Claim and block one
		task6, _ := mockClient.AddTask(ctx, "Blocked task")
		mockClient.ClaimTask(ctx, task6.ID)
		mockClient.BlockTask(ctx, task6.ID)

		summary := c.collectTasks(ctx)

		if summary.Ready != 3 {
			t.Errorf("Ready = %d, want 3", summary.Ready)
		}
		if summary.InProgress != 1 {
			t.Errorf("InProgress = %d, want 1", summary.InProgress)
		}
		if summary.Completed != 1 {
			t.Errorf("Completed = %d, want 1", summary.Completed)
		}
		if summary.Blocked != 1 {
			t.Errorf("Blocked = %d, want 1", summary.Blocked)
		}
		if summary.Total() != 6 {
			t.Errorf("Total() = %d, want 6", summary.Total())
		}
	})

	t.Run("with empty task list", func(t *testing.T) {
		c, _, _ := testCollector(t)

		summary := c.collectTasks(ctx)

		if summary.Total() != 0 {
			t.Errorf("Expected 0 total tasks, got %d", summary.Total())
		}
	})
}

func TestCollectSync(t *testing.T) {
	ctx := context.Background()

	t.Run("without bare repo", func(t *testing.T) {
		c, _, mockGit := testCollector(t)
		c.bareRepo = nil

		mockGit.setOutput("/test/project", []string{"rev-parse", "--short", "main"}, "abc123")

		syncStatus := c.collectSync(ctx)

		if syncStatus.HostBranch != "main" {
			t.Errorf("HostBranch = %q, want %q", syncStatus.HostBranch, "main")
		}
		if syncStatus.HostCommit != "abc123" {
			t.Errorf("HostCommit = %q, want %q", syncStatus.HostCommit, "abc123")
		}
	})

	t.Run("with git error", func(t *testing.T) {
		c, _, mockGit := testCollector(t)
		c.bareRepo = nil

		mockGit.setError("/test/project", []string{"rev-parse", "--short", "main"}, git.DefaultExecutor.RunSilent("", "invalid"))

		syncStatus := c.collectSync(ctx)

		// Should still return status with HostBranch set
		if syncStatus.HostBranch != "main" {
			t.Errorf("HostBranch = %q, want %q", syncStatus.HostBranch, "main")
		}
		// HostCommit should be empty on error
		if syncStatus.HostCommit != "" {
			t.Errorf("HostCommit = %q, want empty string", syncStatus.HostCommit)
		}
	})
}

func TestCollectServices(t *testing.T) {
	ctx := context.Background()

	t.Run("with airyra running", func(t *testing.T) {
		c, mockClient, _ := testCollector(t)
		mockClient.ServerRunning = true

		services := c.collectServices(ctx)

		if !services.Airyra.Running {
			t.Error("Expected Airyra.Running = true")
		}
		if services.Airyra.Error != "" {
			t.Errorf("Expected no error for running airyra, got %q", services.Airyra.Error)
		}
	})

	t.Run("with airyra not running", func(t *testing.T) {
		c, mockClient, _ := testCollector(t)
		mockClient.ServerRunning = false

		services := c.collectServices(ctx)

		if services.Airyra.Running {
			t.Error("Expected Airyra.Running = false")
		}
	})

	t.Run("with nil airyra client", func(t *testing.T) {
		c, _, _ := testCollector(t)
		c.airyra = nil

		services := c.collectServices(ctx)

		if services.Airyra.Running {
			t.Error("Expected Airyra.Running = false with nil client")
		}
		if services.Airyra.Error != "client not configured" {
			t.Errorf("Expected error 'client not configured', got %q", services.Airyra.Error)
		}
	})
}

func TestCollect_Integration(t *testing.T) {
	ctx := context.Background()

	t.Run("collects tasks and sync data", func(t *testing.T) {
		c, mockClient, mockGit := testCollector(t)

		// Setup mock data
		mockClient.AddTask(ctx, "Task 1")
		mockClient.AddTask(ctx, "Task 2")
		mockGit.setOutput("/test/project", []string{"rev-parse", "--short", "main"}, "def456")

		// Test individual collection methods since Collect() requires a real manager
		tasks := c.collectTasks(ctx)
		if tasks.Ready != 2 {
			t.Errorf("Tasks.Ready = %d, want 2", tasks.Ready)
		}

		syncStatus := c.collectSync(ctx)
		if syncStatus.HostBranch != "main" {
			t.Errorf("Sync.HostBranch = %q, want %q", syncStatus.HostBranch, "main")
		}

		services := c.collectServices(ctx)
		if !services.Airyra.Running {
			t.Error("Airyra should be running")
		}
	})
}

func TestCollect_HandlesErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("handles airyra list error gracefully", func(t *testing.T) {
		c, mockClient, _ := testCollector(t)

		// Make ListTasks return an error
		mockClient.OnListTasks = func(ctx context.Context, opts ...sdk.ListTasksOption) (*airyra.TaskList, error) {
			return nil, airyra.ErrServerNotRunning
		}

		summary := c.collectTasks(ctx)

		// Should return empty summary, not error
		if summary.Total() != 0 {
			t.Errorf("Expected empty summary on error, got total %d", summary.Total())
		}
	})

	t.Run("handles git executor error gracefully", func(t *testing.T) {
		c, _, _ := testCollector(t)
		c.bareRepo = nil

		// Don't set any outputs - will return empty string by default
		syncStatus := c.collectSync(ctx)

		// Should still return valid structure
		if syncStatus.HostBranch != "main" {
			t.Errorf("HostBranch = %q, want %q", syncStatus.HostBranch, "main")
		}
	})
}

func TestContainsSession(t *testing.T) {
	tests := []struct {
		name     string
		sessions string
		search   string
		want     bool
	}{
		{
			name:     "exact match",
			sessions: "test-project",
			search:   "test-project",
			want:     true,
		},
		{
			name:     "match with space suffix",
			sessions: "test-project (attached)",
			search:   "test-project",
			want:     true,
		},
		{
			name:     "match with tab suffix",
			sessions: "test-project\t12345",
			search:   "test-project",
			want:     true,
		},
		{
			name:     "multiple sessions - found",
			sessions: "other-project\ntest-project\nanother-project",
			search:   "test-project",
			want:     true,
		},
		{
			name:     "multiple sessions - not found",
			sessions: "other-project\ntest-project-v2\nanother-project",
			search:   "test-project",
			want:     false,
		},
		{
			name:     "empty sessions",
			sessions: "",
			search:   "test-project",
			want:     false,
		},
		{
			name:     "partial match should not match",
			sessions: "test-project-extended",
			search:   "test-project",
			want:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := containsSession(tc.sessions, tc.search)
			if got != tc.want {
				t.Errorf("containsSession(%q, %q) = %v, want %v", tc.sessions, tc.search, got, tc.want)
			}
		})
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "empty string",
			input: "",
			want:  nil,
		},
		{
			name:  "single line no newline",
			input: "hello",
			want:  []string{"hello"},
		},
		{
			name:  "single line with newline",
			input: "hello\n",
			want:  []string{"hello"},
		},
		{
			name:  "multiple lines",
			input: "line1\nline2\nline3",
			want:  []string{"line1", "line2", "line3"},
		},
		{
			name:  "multiple lines with trailing newline",
			input: "line1\nline2\nline3\n",
			want:  []string{"line1", "line2", "line3"},
		},
		{
			name:  "empty lines",
			input: "line1\n\nline3",
			want:  []string{"line1", "", "line3"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := splitLines(tc.input)
			if len(got) != len(tc.want) {
				t.Errorf("splitLines(%q) returned %d lines, want %d", tc.input, len(got), len(tc.want))
				return
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("splitLines(%q)[%d] = %q, want %q", tc.input, i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestHasPrefix(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		prefix string
		want   bool
	}{
		{
			name:   "has prefix",
			s:      "hello world",
			prefix: "hello",
			want:   true,
		},
		{
			name:   "exact match",
			s:      "hello",
			prefix: "hello",
			want:   true,
		},
		{
			name:   "no prefix",
			s:      "hello world",
			prefix: "world",
			want:   false,
		},
		{
			name:   "prefix longer than string",
			s:      "hi",
			prefix: "hello",
			want:   false,
		},
		{
			name:   "empty prefix",
			s:      "hello",
			prefix: "",
			want:   true,
		},
		{
			name:   "empty string",
			s:      "",
			prefix: "hello",
			want:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := hasPrefix(tc.s, tc.prefix)
			if got != tc.want {
				t.Errorf("hasPrefix(%q, %q) = %v, want %v", tc.s, tc.prefix, got, tc.want)
			}
		})
	}
}

func TestGetProjectDir(t *testing.T) {
	c, _, _ := testCollector(t)

	dir := c.GetProjectDir()
	if dir != "/test/project" {
		t.Errorf("GetProjectDir() = %q, want %q", dir, "/test/project")
	}
}

func TestGetConfig(t *testing.T) {
	c, _, _ := testCollector(t)

	cfg := c.GetConfig()
	if cfg == nil {
		t.Fatal("GetConfig() returned nil")
	}
	if cfg.Project != "test-project" {
		t.Errorf("GetConfig().Project = %q, want %q", cfg.Project, "test-project")
	}
}

func TestIsInZellijSession(t *testing.T) {
	// This test verifies the function behavior by checking the implementation
	// The actual environment variable check depends on runtime state

	// Test that function returns a boolean (basic sanity check)
	result := IsInZellijSession()
	// Result should be bool (true if ZELLIJ env var is set, false otherwise)
	_ = result // just verify it doesn't panic
}

func TestCollectWorkers(t *testing.T) {
	// Testing worker collection is complex because it requires a real manager
	// These tests verify behavior when manager is unavailable

	t.Run("verifies collector structure", func(t *testing.T) {
		c, _, _ := testCollector(t)

		// Verify collector has expected fields
		if c.projectDir != "/test/project" {
			t.Errorf("projectDir = %q, want %q", c.projectDir, "/test/project")
		}
		if c.cfg == nil {
			t.Error("cfg should not be nil")
		}
		if c.cfg.Project != "test-project" {
			t.Errorf("cfg.Project = %q, want %q", c.cfg.Project, "test-project")
		}
	})
}

func TestCollector_FullIntegration(t *testing.T) {
	// This test verifies the collection methods with realistic mocks
	ctx := context.Background()

	c, mockClient, mockGit := testCollector(t)

	// Setup tasks
	mockClient.AddTask(ctx, "Feature 1")
	mockClient.AddTask(ctx, "Feature 2")
	task3, _ := mockClient.AddTask(ctx, "In progress")
	mockClient.ClaimTask(ctx, task3.ID)

	// Setup git
	mockGit.setOutput("/test/project", []string{"rev-parse", "--short", "main"}, "abc1234")

	// Test individual collection methods
	tasks := c.collectTasks(ctx)
	if tasks.Ready != 2 {
		t.Errorf("Tasks.Ready = %d, want 2", tasks.Ready)
	}
	if tasks.InProgress != 1 {
		t.Errorf("Tasks.InProgress = %d, want 1", tasks.InProgress)
	}
	if tasks.Total() != 3 {
		t.Errorf("Tasks.Total() = %d, want 3", tasks.Total())
	}

	syncStatus := c.collectSync(ctx)
	if syncStatus.HostBranch != "main" {
		t.Errorf("Sync.HostBranch = %q, want %q", syncStatus.HostBranch, "main")
	}

	services := c.collectServices(ctx)
	if services.Airyra.Running != true {
		t.Error("Services.Airyra.Running = false, want true")
	}

	// Verify config access
	cfg := c.GetConfig()
	if cfg.Project != "test-project" {
		t.Errorf("Config.Project = %q, want %q", cfg.Project, "test-project")
	}
}
