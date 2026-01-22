package state

import (
	"errors"
	"time"
)

// Schema versions - increment when structure changes
const (
	CurrentSessionVersion = 1
	CurrentWorkerVersion  = 1
)

// Sentinel errors
var (
	ErrNoSession     = errors.New("no active session")
	ErrSessionExists = errors.New("session already exists")
)

// SessionStatus represents session lifecycle
type SessionStatus string

const (
	SessionStatusInitializing SessionStatus = "initializing"   // isollm up in progress
	SessionStatusRunning      SessionStatus = "running"        // Fully started
	SessionStatusShuttingDown SessionStatus = "shutting_down"  // isollm down in progress
)

// Session represents a running isollm session
type Session struct {
	Version       int           `json:"version"`
	Status        SessionStatus `json:"status"`
	StartedAt     time.Time     `json:"started_at"`
	PID           int           `json:"pid"`             // Orchestrator process ID
	ProjectRoot   string        `json:"project_root"`
	BareRepoPath  string        `json:"bare_repo_path"`
	ZellijSession string        `json:"zellij_session"`
}

// WorkerStatus represents worker lifecycle states
type WorkerStatus string

const (
	WorkerStatusCreating WorkerStatus = "creating" // Container being built
	WorkerStatusIdle     WorkerStatus = "idle"     // Running, no task
	WorkerStatusBusy     WorkerStatus = "busy"     // Running, has task
	WorkerStatusStopping WorkerStatus = "stopping" // Graceful shutdown in progress
	WorkerStatusStopped  WorkerStatus = "stopped"  // Container stopped
	WorkerStatusError    WorkerStatus = "error"    // Unrecoverable error
)

// WorkerState represents the state of a single worker
type WorkerState struct {
	Version       int          `json:"version"`
	Name          string       `json:"name"`
	ContainerID   string       `json:"container_id,omitempty"`
	Status        WorkerStatus `json:"status"`
	IP            string       `json:"ip,omitempty"`
	CurrentTask   *TaskRef     `json:"current_task,omitempty"`
	CurrentBranch string       `json:"current_branch,omitempty"`
	StartedAt     time.Time    `json:"started_at"`
	LastActivity  time.Time    `json:"last_activity"`
	LastError     string       `json:"last_error,omitempty"`
	ErrorTime     time.Time    `json:"error_time,omitempty"`
}

// TaskRef is a cache of task info (airyra is source of truth)
type TaskRef struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	ClaimedAt time.Time `json:"claimed_at"`
}

// LoadResult contains workers and any load errors
type LoadResult struct {
	Workers []*WorkerState
	Errors  []error
}
