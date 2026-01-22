package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// State abstracts state persistence for testing
type State interface {
	// Session management
	CreateSession(s *Session) error      // Atomic create (fails if exists)
	SaveSession(s *Session) error        // Update existing session
	LoadSession() (*Session, error)      // Returns ErrNoSession if not found
	ClearSession() error                 // Delete session file
	HasActiveSession() (bool, error)     // Checks existence AND liveness

	// Worker state
	SaveWorker(w *WorkerState) error
	LoadWorker(name string) (*WorkerState, error)
	LoadAllWorkers() (*LoadResult, error) // Partial results + errors
	DeleteWorker(name string) error
	ClearAllWorkers() error
}

// FileState implements State using the filesystem
type FileState struct {
	stateDir string
}

// Compile-time interface check
var _ State = (*FileState)(nil)

// New creates a FileState for the project's .isollm directory
func New(projectRoot string) *FileState {
	return &FileState{
		stateDir: filepath.Join(projectRoot, ".isollm"),
	}
}

// NewWithDir creates a FileState with custom directory (for testing)
func NewWithDir(stateDir string) *FileState {
	return &FileState{stateDir: stateDir}
}

// CreateSession atomically creates a new session (fails if exists)
func (m *FileState) CreateSession(s *Session) error {
	if err := os.MkdirAll(m.stateDir, 0755); err != nil {
		return fmt.Errorf("failed to create state dir: %w", err)
	}

	fullPath := filepath.Join(m.stateDir, "session.json")
	s.Version = CurrentSessionVersion

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	// O_EXCL = fail if file exists (atomic check-and-create)
	f, err := os.OpenFile(fullPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if os.IsExist(err) {
			return ErrSessionExists
		}
		return fmt.Errorf("failed to create session file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		os.Remove(fullPath)
		return fmt.Errorf("failed to write session: %w", err)
	}
	return nil
}

// SaveSession updates an existing session (atomic write)
func (m *FileState) SaveSession(s *Session) error {
	s.Version = CurrentSessionVersion
	return m.saveJSON("session.json", s)
}

// LoadSession loads the session, returns ErrNoSession if not found
func (m *FileState) LoadSession() (*Session, error) {
	var s Session
	if err := m.loadJSON("session.json", &s); err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoSession
		}
		return nil, fmt.Errorf("failed to load session: %w", err)
	}

	// Migrate old versions if needed
	if s.Version < CurrentSessionVersion {
		// Future migrations go here
	}

	return &s, nil
}

// ClearSession removes the session file
func (m *FileState) ClearSession() error {
	fullPath := filepath.Join(m.stateDir, "session.json")
	err := os.Remove(fullPath)
	if os.IsNotExist(err) {
		return nil // Already gone
	}
	return err
}

// HasActiveSession checks if session exists AND process is alive
func (m *FileState) HasActiveSession() (bool, error) {
	session, err := m.LoadSession()
	if errors.Is(err, ErrNoSession) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	// Check if orchestrator process is still running
	if !isProcessAlive(session.PID) {
		return false, nil // Stale session
	}

	return true, nil
}

// isProcessAlive checks if a process with the given PID is running
func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 doesn't kill - just checks if process exists
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// SaveWorker saves worker state (atomic write)
func (m *FileState) SaveWorker(w *WorkerState) error {
	w.Version = CurrentWorkerVersion
	workersDir := filepath.Join(m.stateDir, "workers")
	if err := os.MkdirAll(workersDir, 0755); err != nil {
		return fmt.Errorf("failed to create workers dir: %w", err)
	}
	return m.saveJSON(filepath.Join("workers", w.Name+".json"), w)
}

// LoadWorker loads a single worker's state
func (m *FileState) LoadWorker(name string) (*WorkerState, error) {
	var w WorkerState
	if err := m.loadJSON(filepath.Join("workers", name+".json"), &w); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("worker %s not found", name)
		}
		return nil, fmt.Errorf("failed to load worker %s: %w", name, err)
	}
	return &w, nil
}

// LoadAllWorkers loads all workers, returning partial results on errors
func (m *FileState) LoadAllWorkers() (*LoadResult, error) {
	result := &LoadResult{}

	workersDir := filepath.Join(m.stateDir, "workers")
	entries, err := os.ReadDir(workersDir)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil // No workers yet
		}
		return nil, fmt.Errorf("failed to read workers dir: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		var w WorkerState
		if err := m.loadJSON(filepath.Join("workers", e.Name()), &w); err != nil {
			result.Errors = append(result.Errors,
				fmt.Errorf("failed to load %s: %w", e.Name(), err))
			continue
		}
		result.Workers = append(result.Workers, &w)
	}
	return result, nil
}

// DeleteWorker removes a worker's state file
func (m *FileState) DeleteWorker(name string) error {
	fullPath := filepath.Join(m.stateDir, "workers", name+".json")
	err := os.Remove(fullPath)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// ClearAllWorkers removes all worker state files
func (m *FileState) ClearAllWorkers() error {
	workersDir := filepath.Join(m.stateDir, "workers")
	err := os.RemoveAll(workersDir)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// saveJSON writes JSON atomically using temp file + rename
func (m *FileState) saveJSON(relPath string, v interface{}) error {
	fullPath := filepath.Join(m.stateDir, relPath)

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal: %w", err)
	}

	// Write to temp file first
	tmpPath := fullPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, fullPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename: %w", err)
	}

	return nil
}

// loadJSON reads and unmarshals JSON
func (m *FileState) loadJSON(relPath string, v interface{}) error {
	fullPath := filepath.Join(m.stateDir, relPath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}
