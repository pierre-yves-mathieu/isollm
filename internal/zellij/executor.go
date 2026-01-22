package zellij

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Executor defines the interface for running zellij commands
type Executor interface {
	// ListSessions returns a list of active zellij session names
	ListSessions() ([]string, error)

	// SessionExists checks if a session with the given name exists
	SessionExists(name string) (bool, error)

	// CreateSession creates a new zellij session with the given layout
	CreateSession(name, layoutPath string) error

	// AttachSession attaches to an existing session
	AttachSession(name string) error

	// KillSession terminates a zellij session
	KillSession(name string) error

	// SendKeys sends keystrokes to a pane in a session
	SendKeys(session, pane string, keys ...string) error
}

// DefaultExecutor is the default zellij executor
var DefaultExecutor Executor = &realExecutor{}

type realExecutor struct{}

// ListSessions returns active zellij sessions
func (e *realExecutor) ListSessions() ([]string, error) {
	cmd := exec.Command("zellij", "list-sessions", "--no-formatting")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// zellij returns error if no sessions exist
		if strings.Contains(stderr.String(), "No active") ||
			strings.Contains(stdout.String(), "No active") {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to list sessions: %w: %s", err, stderr.String())
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return nil, nil
	}

	// Parse session names (one per line, may have status info after)
	lines := strings.Split(output, "\n")
	sessions := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Session name is the first field
		fields := strings.Fields(line)
		if len(fields) > 0 {
			sessions = append(sessions, fields[0])
		}
	}

	return sessions, nil
}

// SessionExists checks if a named session exists
func (e *realExecutor) SessionExists(name string) (bool, error) {
	sessions, err := e.ListSessions()
	if err != nil {
		return false, err
	}

	for _, s := range sessions {
		if s == name {
			return true, nil
		}
	}
	return false, nil
}

// CreateSession creates a new detached zellij session with the given layout
func (e *realExecutor) CreateSession(name, layoutPath string) error {
	args := []string{
		"--session", name,
		"--layout", layoutPath,
	}

	cmd := exec.Command("zellij", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start zellij session: %w", err)
	}

	// Don't wait for the process - zellij runs in the foreground
	// The caller should handle this appropriately (e.g., run in background)
	return nil
}

// AttachSession attaches to an existing zellij session
func (e *realExecutor) AttachSession(name string) error {
	cmd := exec.Command("zellij", "attach", name)
	cmd.Stdin = nil // Let zellij take over the terminal

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to attach to session %s: %w", name, err)
	}

	return nil
}

// KillSession terminates a zellij session
func (e *realExecutor) KillSession(name string) error {
	cmd := exec.Command("zellij", "kill-session", name)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if strings.Contains(stderr.String(), "not found") ||
			strings.Contains(stderr.String(), "No session") {
			return ErrSessionNotFound
		}
		return fmt.Errorf("failed to kill session %s: %w: %s", name, err, stderr.String())
	}

	return nil
}

// SendKeys sends keystrokes to a pane
func (e *realExecutor) SendKeys(session, pane string, keys ...string) error {
	args := []string{
		"--session", session,
		"action", "write-chars",
	}
	args = append(args, strings.Join(keys, ""))

	cmd := exec.Command("zellij", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to send keys to session %s: %w: %s", session, err, stderr.String())
	}

	return nil
}

// CheckZellijInstalled verifies that zellij is available in PATH
func CheckZellijInstalled() error {
	_, err := exec.LookPath("zellij")
	if err != nil {
		return ErrZellijNotFound
	}
	return nil
}
