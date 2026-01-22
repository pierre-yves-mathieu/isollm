package state

import (
	"time"
)

// SessionState provides a simplified view of session state for external use.
// This struct is designed for easy serialization and command-line display.
type SessionState struct {
	ProjectDir    string    `json:"project_dir"`
	BareRepoPath  string    `json:"bare_repo_path"`
	Workers       []string  `json:"workers"`
	StartedAt     time.Time `json:"started_at"`
	ZellijSession string    `json:"zellij_session"`
}

// SaveSessionState saves session state to the project's .isollm directory.
// This is a convenience function that wraps FileState.SaveSession.
func SaveSessionState(projectDir string, state *SessionState) error {
	fs := New(projectDir)

	session := &Session{
		ProjectRoot:   state.ProjectDir,
		BareRepoPath:  state.BareRepoPath,
		StartedAt:     state.StartedAt,
		ZellijSession: state.ZellijSession,
		Status:        SessionStatusRunning,
	}

	// Try to create first; if it exists, update it
	err := fs.CreateSession(session)
	if err == ErrSessionExists {
		return fs.SaveSession(session)
	}
	return err
}

// LoadSessionState loads session state from the project's .isollm directory.
// Returns nil, nil if no session exists.
func LoadSessionState(projectDir string) (*SessionState, error) {
	fs := New(projectDir)

	session, err := fs.LoadSession()
	if err != nil {
		if err == ErrNoSession {
			return nil, nil
		}
		return nil, err
	}

	return &SessionState{
		ProjectDir:    session.ProjectRoot,
		BareRepoPath:  session.BareRepoPath,
		StartedAt:     session.StartedAt,
		ZellijSession: session.ZellijSession,
	}, nil
}

// ClearSessionState removes session state from the project's .isollm directory.
func ClearSessionState(projectDir string) error {
	fs := New(projectDir)
	return fs.ClearSession()
}

// HasActiveSession checks if there's an active session for the project.
// This checks both the existence of session state and whether the process is still alive.
func HasActiveSession(projectDir string) (bool, error) {
	fs := New(projectDir)
	return fs.HasActiveSession()
}
