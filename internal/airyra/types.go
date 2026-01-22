package airyra

import (
	"context"
	"fmt"
	"strings"

	sdk "airyra/pkg/airyra"
)

// TaskClient defines the interface for airyra task operations.
// Both Client and MockClient implement this interface.
type TaskClient interface {
	// Health and status
	Health(ctx context.Context) error
	IsServerRunning(ctx context.Context) bool

	// Task CRUD
	AddTask(ctx context.Context, title string, opts ...sdk.CreateTaskOption) (*Task, error)
	GetTask(ctx context.Context, id string) (*Task, error)
	ListTasks(ctx context.Context, opts ...sdk.ListTasksOption) (*TaskList, error)
	ListReadyTasks(ctx context.Context) (*TaskList, error)
	DeleteTask(ctx context.Context, id string) error
	ClearDoneTasks(ctx context.Context) (int, error)
	ClearAllTasks(ctx context.Context) (int, error)

	// Task lifecycle
	ClaimTask(ctx context.Context, id string) (*Task, error)
	CompleteTask(ctx context.Context, id string) (*Task, error)
	ReleaseTask(ctx context.Context, id string, force bool) (*Task, error)
	BlockTask(ctx context.Context, id string) (*Task, error)
	UnblockTask(ctx context.Context, id string) (*Task, error)

	// Dependencies
	AddDependency(ctx context.Context, childID, parentID string) error
	RemoveDependency(ctx context.Context, childID, parentID string) error
	ListDependencies(ctx context.Context, taskID string) ([]Dependency, error)
}

// Re-export SDK types for convenience
type (
	Task       = sdk.Task
	TaskStatus = sdk.TaskStatus
	TaskList   = sdk.TaskList
	Dependency = sdk.Dependency
)

// Re-export status constants
const (
	StatusOpen       = sdk.StatusOpen
	StatusInProgress = sdk.StatusInProgress
	StatusBlocked    = sdk.StatusBlocked
	StatusDone       = sdk.StatusDone
)

// Re-export priority constants
const (
	PriorityCritical = sdk.PriorityCritical
	PriorityHigh     = sdk.PriorityHigh
	PriorityNormal   = sdk.PriorityNormal
	PriorityLow      = sdk.PriorityLow
	PriorityLowest   = sdk.PriorityLowest
)

// Re-export option functions for use in isollm
var (
	WithStatus  = sdk.WithStatus
	WithPage    = sdk.WithPage
	WithPerPage = sdk.WithPerPage
)

// PriorityFromString converts CLI priority names to int values
func PriorityFromString(s string) (int, error) {
	switch strings.ToLower(s) {
	case "critical":
		return PriorityCritical, nil
	case "high":
		return PriorityHigh, nil
	case "normal", "":
		return PriorityNormal, nil
	case "low":
		return PriorityLow, nil
	case "lowest":
		return PriorityLowest, nil
	default:
		return 0, fmt.Errorf("invalid priority: %s (use critical, high, normal, low, lowest)", s)
	}
}

// PriorityToString converts int priority to display name
func PriorityToString(p int) string {
	switch p {
	case PriorityCritical:
		return "critical"
	case PriorityHigh:
		return "high"
	case PriorityNormal:
		return "normal"
	case PriorityLow:
		return "low"
	case PriorityLowest:
		return "lowest"
	default:
		return fmt.Sprintf("unknown(%d)", p)
	}
}
