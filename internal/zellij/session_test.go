package zellij

import (
	"errors"
	"sync"
	"testing"
)

// MockExecutor implements the Executor interface for testing
type MockExecutor struct {
	mu sync.Mutex

	// Sessions tracks created sessions (name -> layoutPath)
	Sessions map[string]string

	// Attached tracks attach calls
	Attached []string

	// Killed tracks kill calls
	Killed []string

	// SentKeys tracks SendKeys calls: session -> pane -> keys
	SentKeys map[string]map[string][]string

	// Error injection
	ListSessionsErr   error
	SessionExistsErr  error
	CreateSessionErr  error
	AttachSessionErr  error
	KillSessionErr    error
	SendKeysErr       error

	// Custom behavior
	SessionExistsFunc func(name string) (bool, error)
}

func NewMockExecutor() *MockExecutor {
	return &MockExecutor{
		Sessions: make(map[string]string),
		SentKeys: make(map[string]map[string][]string),
	}
}

func (m *MockExecutor) ListSessions() ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.ListSessionsErr != nil {
		return nil, m.ListSessionsErr
	}

	sessions := make([]string, 0, len(m.Sessions))
	for name := range m.Sessions {
		sessions = append(sessions, name)
	}
	return sessions, nil
}

func (m *MockExecutor) SessionExists(name string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.SessionExistsFunc != nil {
		return m.SessionExistsFunc(name)
	}

	if m.SessionExistsErr != nil {
		return false, m.SessionExistsErr
	}

	_, exists := m.Sessions[name]
	return exists, nil
}

func (m *MockExecutor) CreateSession(name, layoutPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.CreateSessionErr != nil {
		return m.CreateSessionErr
	}

	m.Sessions[name] = layoutPath
	return nil
}

func (m *MockExecutor) AttachSession(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.AttachSessionErr != nil {
		return m.AttachSessionErr
	}

	m.Attached = append(m.Attached, name)
	return nil
}

func (m *MockExecutor) KillSession(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.KillSessionErr != nil {
		return m.KillSessionErr
	}

	if _, exists := m.Sessions[name]; !exists {
		return ErrSessionNotFound
	}

	delete(m.Sessions, name)
	m.Killed = append(m.Killed, name)
	return nil
}

func (m *MockExecutor) SendKeys(session, pane string, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.SendKeysErr != nil {
		return m.SendKeysErr
	}

	if m.SentKeys[session] == nil {
		m.SentKeys[session] = make(map[string][]string)
	}
	m.SentKeys[session][pane] = append(m.SentKeys[session][pane], keys...)
	return nil
}

func TestNewManager(t *testing.T) {
	// Note: This test requires zellij to be installed
	// In CI environments without zellij, this test may be skipped
	t.Run("creates manager when zellij is available", func(t *testing.T) {
		// This test depends on system state (zellij being installed)
		// We primarily test NewManagerWithExecutor which doesn't have this dependency
		mgr := NewManagerWithExecutor(NewMockExecutor())
		if mgr == nil {
			t.Fatal("NewManagerWithExecutor returned nil")
		}
		if mgr.sessions == nil {
			t.Error("sessions map should be initialized")
		}
	})
}

func TestNewManagerWithExecutor(t *testing.T) {
	exec := NewMockExecutor()
	mgr := NewManagerWithExecutor(exec)

	if mgr == nil {
		t.Fatal("NewManagerWithExecutor returned nil")
	}
	if mgr.executor != exec {
		t.Error("executor was not set correctly")
	}
	if mgr.sessions == nil {
		t.Error("sessions map should be initialized")
	}
}

func TestStartSession(t *testing.T) {
	t.Run("creates new session successfully", func(t *testing.T) {
		exec := NewMockExecutor()
		mgr := NewManagerWithExecutor(exec)

		cfg := SessionConfig{
			Name:   "test-session",
			Layout: LayoutModeHorizontal,
			Workers: []WorkerPane{
				{Name: "worker-1", ContainerName: "proj-w1"},
			},
		}

		session, err := mgr.StartSession(cfg)
		if err != nil {
			t.Fatalf("StartSession failed: %v", err)
		}

		if session == nil {
			t.Fatal("StartSession returned nil session")
		}
		if session.Name != "test-session" {
			t.Errorf("expected session name 'test-session', got %q", session.Name)
		}
		if session.LayoutPath == "" {
			t.Error("session should have a layout path")
		}

		// Verify session was created in executor
		if _, exists := exec.Sessions["test-session"]; !exists {
			t.Error("session was not created in executor")
		}

		// Clean up layout file
		RemoveLayoutFile(session.LayoutPath)
	})

	t.Run("returns error if session already exists", func(t *testing.T) {
		exec := NewMockExecutor()
		exec.Sessions["existing-session"] = "/some/path.kdl"
		mgr := NewManagerWithExecutor(exec)

		cfg := SessionConfig{
			Name:    "existing-session",
			Layout:  LayoutModeHorizontal,
			Workers: []WorkerPane{},
		}

		_, err := mgr.StartSession(cfg)
		if !errors.Is(err, ErrSessionExists) {
			t.Errorf("expected ErrSessionExists, got: %v", err)
		}
	})

	t.Run("returns error on session exists check failure", func(t *testing.T) {
		exec := NewMockExecutor()
		exec.SessionExistsErr = errors.New("connection failed")
		mgr := NewManagerWithExecutor(exec)

		cfg := SessionConfig{
			Name:    "test-session",
			Layout:  LayoutModeHorizontal,
			Workers: []WorkerPane{},
		}

		_, err := mgr.StartSession(cfg)
		if err == nil {
			t.Fatal("expected error on session exists check failure")
		}
		if !errors.Is(err, exec.SessionExistsErr) {
			if err.Error() != "failed to check session existence: connection failed" {
				t.Errorf("unexpected error: %v", err)
			}
		}
	})

	t.Run("returns error on create session failure", func(t *testing.T) {
		exec := NewMockExecutor()
		exec.CreateSessionErr = errors.New("create failed")
		mgr := NewManagerWithExecutor(exec)

		cfg := SessionConfig{
			Name:   "test-session",
			Layout: LayoutModeHorizontal,
			Workers: []WorkerPane{
				{Name: "worker-1", ContainerName: "proj-w1"},
			},
		}

		_, err := mgr.StartSession(cfg)
		if err == nil {
			t.Fatal("expected error on create session failure")
		}
	})

	t.Run("tracks session in manager cache", func(t *testing.T) {
		exec := NewMockExecutor()
		mgr := NewManagerWithExecutor(exec)

		cfg := SessionConfig{
			Name:   "cached-session",
			Layout: LayoutModeHorizontal,
			Workers: []WorkerPane{
				{Name: "worker-1", ContainerName: "proj-w1"},
			},
		}

		session, err := mgr.StartSession(cfg)
		if err != nil {
			t.Fatalf("StartSession failed: %v", err)
		}
		defer RemoveLayoutFile(session.LayoutPath)

		// Check manager cache
		cached, exists := mgr.GetSession("cached-session")
		if !exists {
			t.Fatal("session not found in manager cache")
		}
		if cached.Name != "cached-session" {
			t.Errorf("cached session has wrong name: %q", cached.Name)
		}
	})
}

func TestStopSession(t *testing.T) {
	t.Run("stops existing session", func(t *testing.T) {
		exec := NewMockExecutor()
		mgr := NewManagerWithExecutor(exec)

		// Start a session first
		cfg := SessionConfig{
			Name:   "stop-test",
			Layout: LayoutModeHorizontal,
			Workers: []WorkerPane{
				{Name: "worker-1", ContainerName: "proj-w1"},
			},
		}

		session, err := mgr.StartSession(cfg)
		if err != nil {
			t.Fatalf("StartSession failed: %v", err)
		}

		// Stop the session
		err = mgr.StopSession("stop-test")
		if err != nil {
			t.Errorf("StopSession failed: %v", err)
		}

		// Verify session was killed
		if len(exec.Killed) != 1 || exec.Killed[0] != "stop-test" {
			t.Error("session was not killed in executor")
		}

		// Verify removed from cache
		if _, exists := mgr.GetSession("stop-test"); exists {
			t.Error("session should be removed from cache")
		}

		// Layout file should be removed (may already be gone from mock)
		_ = session // Used for reference, layout cleanup happens in StopSession
	})

	t.Run("no error for non-existent session", func(t *testing.T) {
		exec := NewMockExecutor()
		mgr := NewManagerWithExecutor(exec)

		// Stopping a non-existent session should not error
		// (the executor returns ErrSessionNotFound which is ignored)
		err := mgr.StopSession("nonexistent")
		if err != nil {
			t.Errorf("StopSession should not error for non-existent session: %v", err)
		}
	})

	t.Run("returns error on kill failure (non-not-found)", func(t *testing.T) {
		exec := NewMockExecutor()
		exec.Sessions["kill-fail"] = "/path.kdl"
		exec.KillSessionErr = errors.New("permission denied")
		mgr := NewManagerWithExecutor(exec)

		// Add to manager cache
		mgr.sessions["kill-fail"] = &Session{Name: "kill-fail"}

		err := mgr.StopSession("kill-fail")
		if err == nil {
			t.Fatal("expected error on kill failure")
		}
	})
}

func TestSessionExists(t *testing.T) {
	t.Run("returns true for existing session", func(t *testing.T) {
		exec := NewMockExecutor()
		exec.Sessions["existing"] = "/path.kdl"
		mgr := NewManagerWithExecutor(exec)

		exists, err := mgr.SessionExists("existing")
		if err != nil {
			t.Fatalf("SessionExists failed: %v", err)
		}
		if !exists {
			t.Error("expected session to exist")
		}
	})

	t.Run("returns false for non-existing session", func(t *testing.T) {
		exec := NewMockExecutor()
		mgr := NewManagerWithExecutor(exec)

		exists, err := mgr.SessionExists("nonexistent")
		if err != nil {
			t.Fatalf("SessionExists failed: %v", err)
		}
		if exists {
			t.Error("expected session to not exist")
		}
	})

	t.Run("returns error on executor failure", func(t *testing.T) {
		exec := NewMockExecutor()
		exec.SessionExistsErr = errors.New("check failed")
		mgr := NewManagerWithExecutor(exec)

		_, err := mgr.SessionExists("any")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestAttachSession(t *testing.T) {
	t.Run("attaches to existing session", func(t *testing.T) {
		exec := NewMockExecutor()
		exec.Sessions["attach-test"] = "/path.kdl"
		mgr := NewManagerWithExecutor(exec)

		err := mgr.AttachSession("attach-test")
		if err != nil {
			t.Fatalf("AttachSession failed: %v", err)
		}

		// Verify attach was called
		if len(exec.Attached) != 1 || exec.Attached[0] != "attach-test" {
			t.Error("session was not attached")
		}
	})

	t.Run("returns error for non-existent session", func(t *testing.T) {
		exec := NewMockExecutor()
		mgr := NewManagerWithExecutor(exec)

		err := mgr.AttachSession("nonexistent")
		if !errors.Is(err, ErrSessionNotFound) {
			t.Errorf("expected ErrSessionNotFound, got: %v", err)
		}
	})

	t.Run("returns error on session exists check failure", func(t *testing.T) {
		exec := NewMockExecutor()
		exec.SessionExistsErr = errors.New("check failed")
		mgr := NewManagerWithExecutor(exec)

		err := mgr.AttachSession("any")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("returns error on attach failure", func(t *testing.T) {
		exec := NewMockExecutor()
		exec.Sessions["attach-fail"] = "/path.kdl"
		exec.AttachSessionErr = errors.New("attach failed")
		mgr := NewManagerWithExecutor(exec)

		err := mgr.AttachSession("attach-fail")
		if err == nil {
			t.Fatal("expected error on attach failure")
		}
	})
}

func TestGetSession(t *testing.T) {
	t.Run("returns session from cache", func(t *testing.T) {
		exec := NewMockExecutor()
		mgr := NewManagerWithExecutor(exec)

		// Add session to cache manually
		mgr.sessions["cached"] = &Session{
			Name:       "cached",
			LayoutPath: "/some/path.kdl",
		}

		session, exists := mgr.GetSession("cached")
		if !exists {
			t.Fatal("session should exist in cache")
		}
		if session.Name != "cached" {
			t.Errorf("wrong session name: %q", session.Name)
		}
	})

	t.Run("returns false for non-cached session", func(t *testing.T) {
		exec := NewMockExecutor()
		mgr := NewManagerWithExecutor(exec)

		_, exists := mgr.GetSession("not-cached")
		if exists {
			t.Error("session should not exist in cache")
		}
	})
}

func TestListSessions(t *testing.T) {
	t.Run("lists sessions from executor", func(t *testing.T) {
		exec := NewMockExecutor()
		exec.Sessions["session-1"] = "/path1.kdl"
		exec.Sessions["session-2"] = "/path2.kdl"
		mgr := NewManagerWithExecutor(exec)

		sessions, err := mgr.ListSessions()
		if err != nil {
			t.Fatalf("ListSessions failed: %v", err)
		}

		if len(sessions) != 2 {
			t.Errorf("expected 2 sessions, got %d", len(sessions))
		}
	})

	t.Run("returns error on executor failure", func(t *testing.T) {
		exec := NewMockExecutor()
		exec.ListSessionsErr = errors.New("list failed")
		mgr := NewManagerWithExecutor(exec)

		_, err := mgr.ListSessions()
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestRefreshSessions(t *testing.T) {
	t.Run("removes stale sessions from cache", func(t *testing.T) {
		exec := NewMockExecutor()
		exec.Sessions["active"] = "/active.kdl"
		mgr := NewManagerWithExecutor(exec)

		// Add both active and stale session to cache
		mgr.sessions["active"] = &Session{Name: "active", LayoutPath: "/active.kdl"}
		mgr.sessions["stale"] = &Session{Name: "stale", LayoutPath: "/stale.kdl"}

		err := mgr.RefreshSessions()
		if err != nil {
			t.Fatalf("RefreshSessions failed: %v", err)
		}

		// Active should still be in cache
		if _, exists := mgr.GetSession("active"); !exists {
			t.Error("active session should still be in cache")
		}

		// Stale should be removed
		if _, exists := mgr.GetSession("stale"); exists {
			t.Error("stale session should be removed from cache")
		}
	})

	t.Run("returns error on list failure", func(t *testing.T) {
		exec := NewMockExecutor()
		exec.ListSessionsErr = errors.New("list failed")
		mgr := NewManagerWithExecutor(exec)

		err := mgr.RefreshSessions()
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestSendKeys(t *testing.T) {
	t.Run("sends keys to existing session", func(t *testing.T) {
		exec := NewMockExecutor()
		exec.Sessions["keys-test"] = "/path.kdl"
		mgr := NewManagerWithExecutor(exec)

		err := mgr.SendKeys("keys-test", "worker-1", "hello", "world")
		if err != nil {
			t.Fatalf("SendKeys failed: %v", err)
		}

		// Verify keys were sent
		if exec.SentKeys["keys-test"]["worker-1"][0] != "hello" {
			t.Error("first key not sent correctly")
		}
		if exec.SentKeys["keys-test"]["worker-1"][1] != "world" {
			t.Error("second key not sent correctly")
		}
	})

	t.Run("returns error for non-existent session", func(t *testing.T) {
		exec := NewMockExecutor()
		mgr := NewManagerWithExecutor(exec)

		err := mgr.SendKeys("nonexistent", "pane", "key")
		if !errors.Is(err, ErrSessionNotFound) {
			t.Errorf("expected ErrSessionNotFound, got: %v", err)
		}
	})

	t.Run("returns error on session exists check failure", func(t *testing.T) {
		exec := NewMockExecutor()
		exec.SessionExistsErr = errors.New("check failed")
		mgr := NewManagerWithExecutor(exec)

		err := mgr.SendKeys("any", "pane", "key")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("returns error on send failure", func(t *testing.T) {
		exec := NewMockExecutor()
		exec.Sessions["send-fail"] = "/path.kdl"
		exec.SendKeysErr = errors.New("send failed")
		mgr := NewManagerWithExecutor(exec)

		err := mgr.SendKeys("send-fail", "pane", "key")
		if err == nil {
			t.Fatal("expected error on send failure")
		}
	})
}

func TestCleanup(t *testing.T) {
	t.Run("stops all managed sessions", func(t *testing.T) {
		exec := NewMockExecutor()
		mgr := NewManagerWithExecutor(exec)

		// Start multiple sessions
		for i := 1; i <= 3; i++ {
			cfg := SessionConfig{
				Name:   "cleanup-" + string(rune('0'+i)),
				Layout: LayoutModeHorizontal,
				Workers: []WorkerPane{
					{Name: "worker-1", ContainerName: "proj-w1"},
				},
			}
			session, err := mgr.StartSession(cfg)
			if err != nil {
				t.Fatalf("StartSession failed: %v", err)
			}
			defer RemoveLayoutFile(session.LayoutPath)
		}

		// Verify sessions in cache
		if len(mgr.sessions) != 3 {
			t.Fatalf("expected 3 sessions in cache, got %d", len(mgr.sessions))
		}

		// Cleanup
		err := mgr.Cleanup()
		if err != nil {
			t.Errorf("Cleanup failed: %v", err)
		}

		// All sessions should be killed
		if len(exec.Killed) != 3 {
			t.Errorf("expected 3 sessions killed, got %d", len(exec.Killed))
		}

		// Cache should be empty
		if len(mgr.sessions) != 0 {
			t.Errorf("cache should be empty after cleanup, got %d sessions",
				len(mgr.sessions))
		}
	})

	t.Run("returns last error on partial failure", func(t *testing.T) {
		exec := NewMockExecutor()
		mgr := NewManagerWithExecutor(exec)

		// Add sessions directly to cache (simulating already started sessions)
		mgr.sessions["session-1"] = &Session{Name: "session-1"}
		mgr.sessions["session-2"] = &Session{Name: "session-2"}
		exec.Sessions["session-1"] = "/path1.kdl"
		exec.Sessions["session-2"] = "/path2.kdl"

		// Make kill fail (not with ErrSessionNotFound)
		exec.KillSessionErr = errors.New("kill failed")

		err := mgr.Cleanup()
		if err == nil {
			t.Fatal("expected error on partial cleanup failure")
		}
	})

	t.Run("handles empty manager", func(t *testing.T) {
		exec := NewMockExecutor()
		mgr := NewManagerWithExecutor(exec)

		err := mgr.Cleanup()
		if err != nil {
			t.Errorf("Cleanup should not error on empty manager: %v", err)
		}
	})
}

func TestConcurrentAccess(t *testing.T) {
	t.Run("handles concurrent session operations", func(t *testing.T) {
		exec := NewMockExecutor()
		mgr := NewManagerWithExecutor(exec)

		var wg sync.WaitGroup
		errors := make(chan error, 10)

		// Start multiple sessions concurrently
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				cfg := SessionConfig{
					Name:   "concurrent-" + string(rune('a'+id)),
					Layout: LayoutModeHorizontal,
					Workers: []WorkerPane{
						{Name: "worker-1", ContainerName: "proj-w1"},
					},
				}
				session, err := mgr.StartSession(cfg)
				if err != nil {
					errors <- err
					return
				}
				defer RemoveLayoutFile(session.LayoutPath)
			}(i)
		}

		wg.Wait()
		close(errors)

		// Check for errors
		for err := range errors {
			t.Errorf("concurrent operation failed: %v", err)
		}

		// Verify all sessions were created
		sessions, _ := mgr.ListSessions()
		if len(sessions) != 5 {
			t.Errorf("expected 5 sessions, got %d", len(sessions))
		}
	})

	t.Run("handles concurrent reads", func(t *testing.T) {
		exec := NewMockExecutor()
		exec.Sessions["test-session"] = "/path.kdl"
		mgr := NewManagerWithExecutor(exec)
		mgr.sessions["test-session"] = &Session{Name: "test-session"}

		var wg sync.WaitGroup

		// Multiple concurrent reads
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				mgr.GetSession("test-session")
				mgr.SessionExists("test-session")
			}()
		}

		wg.Wait()
	})
}

func TestSentinelErrors(t *testing.T) {
	t.Run("ErrSessionNotFound is distinct", func(t *testing.T) {
		if ErrSessionNotFound.Error() != "zellij session not found" {
			t.Errorf("unexpected error message: %s", ErrSessionNotFound.Error())
		}
	})

	t.Run("ErrSessionExists is distinct", func(t *testing.T) {
		if ErrSessionExists.Error() != "zellij session already exists" {
			t.Errorf("unexpected error message: %s", ErrSessionExists.Error())
		}
	})

	t.Run("ErrZellijNotFound is distinct", func(t *testing.T) {
		if ErrZellijNotFound.Error() != "zellij not found in PATH" {
			t.Errorf("unexpected error message: %s", ErrZellijNotFound.Error())
		}
	})

	t.Run("ErrInvalidLayout is distinct", func(t *testing.T) {
		if ErrInvalidLayout.Error() != "invalid layout mode" {
			t.Errorf("unexpected error message: %s", ErrInvalidLayout.Error())
		}
	})
}
