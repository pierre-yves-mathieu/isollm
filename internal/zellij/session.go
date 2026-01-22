package zellij

import (
	"fmt"
	"sync"
)

// Manager manages zellij sessions for isollm
type Manager struct {
	executor Executor
	sessions map[string]*Session
	mu       sync.RWMutex
}

// NewManager creates a new zellij session manager
func NewManager() (*Manager, error) {
	// Verify zellij is installed
	if err := CheckZellijInstalled(); err != nil {
		return nil, err
	}

	return &Manager{
		executor: DefaultExecutor,
		sessions: make(map[string]*Session),
	}, nil
}

// NewManagerWithExecutor creates a manager with a custom executor (for testing)
func NewManagerWithExecutor(exec Executor) *Manager {
	return &Manager{
		executor: exec,
		sessions: make(map[string]*Session),
	}
}

// StartSession creates and starts a new zellij session
func (m *Manager) StartSession(cfg SessionConfig) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if session already exists in zellij
	exists, err := m.executor.SessionExists(cfg.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to check session existence: %w", err)
	}
	if exists {
		return nil, ErrSessionExists
	}

	// Write the layout file
	layoutPath, err := WriteLayoutToTemp(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to write layout file: %w", err)
	}

	// Create the session
	if err := m.executor.CreateSession(cfg.Name, layoutPath); err != nil {
		// Clean up layout file on failure
		RemoveLayoutFile(layoutPath)
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	session := &Session{
		Name:       cfg.Name,
		Config:     cfg,
		LayoutPath: layoutPath,
		Attached:   false,
	}

	m.sessions[cfg.Name] = session
	return session, nil
}

// StopSession terminates a zellij session
func (m *Manager) StopSession(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Kill the zellij session
	err := m.executor.KillSession(name)
	if err != nil && err != ErrSessionNotFound {
		return fmt.Errorf("failed to kill session: %w", err)
	}

	// Clean up local state
	if session, ok := m.sessions[name]; ok {
		RemoveLayoutFile(session.LayoutPath)
		delete(m.sessions, name)
	}

	return nil
}

// AttachSession attaches to an existing zellij session
func (m *Manager) AttachSession(name string) error {
	// Check session exists
	exists, err := m.executor.SessionExists(name)
	if err != nil {
		return fmt.Errorf("failed to check session existence: %w", err)
	}
	if !exists {
		return ErrSessionNotFound
	}

	// Attach (this will block until user detaches)
	if err := m.executor.AttachSession(name); err != nil {
		return err
	}

	return nil
}

// SessionExists checks if a session exists (either in manager or in zellij)
func (m *Manager) SessionExists(name string) (bool, error) {
	return m.executor.SessionExists(name)
}

// GetSession returns a session by name from the manager's cache
func (m *Manager) GetSession(name string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[name]
	return session, ok
}

// ListSessions returns all known session names
func (m *Manager) ListSessions() ([]string, error) {
	return m.executor.ListSessions()
}

// RefreshSessions syncs the manager's cache with actual zellij sessions
func (m *Manager) RefreshSessions() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get actual sessions from zellij
	active, err := m.executor.ListSessions()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	// Build a set of active session names
	activeSet := make(map[string]bool)
	for _, name := range active {
		activeSet[name] = true
	}

	// Remove sessions that no longer exist
	for name, session := range m.sessions {
		if !activeSet[name] {
			RemoveLayoutFile(session.LayoutPath)
			delete(m.sessions, name)
		}
	}

	return nil
}

// SendKeys sends keystrokes to a pane in a session
func (m *Manager) SendKeys(sessionName, paneName string, keys ...string) error {
	exists, err := m.executor.SessionExists(sessionName)
	if err != nil {
		return fmt.Errorf("failed to check session existence: %w", err)
	}
	if !exists {
		return ErrSessionNotFound
	}

	return m.executor.SendKeys(sessionName, paneName, keys...)
}

// Cleanup stops all managed sessions and cleans up resources
func (m *Manager) Cleanup() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for name, session := range m.sessions {
		if err := m.executor.KillSession(name); err != nil && err != ErrSessionNotFound {
			lastErr = err
		}
		RemoveLayoutFile(session.LayoutPath)
		delete(m.sessions, name)
	}

	return lastErr
}
