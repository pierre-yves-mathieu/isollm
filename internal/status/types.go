package status

import (
	"time"

	"isollm/internal/barerepo"
)

// Status aggregates the current state of an isollm project
type Status struct {
	Project       string          `json:"project"`
	SessionActive bool            `json:"session_active"`
	Workers       []WorkerStatus  `json:"workers"`
	Tasks         TaskSummary     `json:"tasks"`
	Sync          SyncStatus      `json:"sync"`
	Services      ServiceStatus   `json:"services"`
	Timestamp     time.Time       `json:"timestamp"`
}

// WorkerStatus represents the status of a single worker
type WorkerStatus struct {
	Name       string        `json:"name"`
	Status     string        `json:"status"`
	IP         string        `json:"ip,omitempty"`
	TaskID     string        `json:"task_id,omitempty"`
	TaskTitle  string        `json:"task_title,omitempty"`
	TaskBranch string        `json:"task_branch,omitempty"`
	Duration   time.Duration `json:"duration,omitempty"`
}

// TaskSummary contains counts of tasks by status
type TaskSummary struct {
	Ready      int `json:"ready"`
	InProgress int `json:"in_progress"`
	Blocked    int `json:"blocked"`
	Completed  int `json:"completed"`
}

// Total returns the total number of tasks
func (t TaskSummary) Total() int {
	return t.Ready + t.InProgress + t.Blocked + t.Completed
}

// SyncStatus represents the git sync state
type SyncStatus struct {
	HostBranch    string              `json:"host_branch"`
	HostCommit    string              `json:"host_commit,omitempty"`
	HostAhead     int                 `json:"host_ahead"`
	TaskBranches  []barerepo.BranchInfo `json:"task_branches,omitempty"`
	TotalBranches int                 `json:"total_branches"`
}

// ServiceStatus represents the status of external services
type ServiceStatus struct {
	Airyra ServiceInfo `json:"airyra"`
	Zellij ServiceInfo `json:"zellij"`
}

// ServiceInfo contains the status of a single service
type ServiceInfo struct {
	Running bool   `json:"running"`
	Error   string `json:"error,omitempty"`
}

// BranchInfo re-exports barerepo.BranchInfo for convenience
type BranchInfo = barerepo.BranchInfo

// WorkerStatusRunning is the status string for a running worker
const (
	WorkerStatusRunning = "RUNNING"
	WorkerStatusStopped = "STOPPED"
	WorkerStatusUnknown = "UNKNOWN"
)
