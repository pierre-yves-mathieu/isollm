package airyra

import (
	"errors"

	sdk "airyra/pkg/airyra"
)

// Re-export SDK sentinel errors
var (
	ErrServerNotRunning = sdk.ErrServerNotRunning
	ErrServerUnhealthy  = sdk.ErrServerUnhealthy
)

// Re-export error type checkers
var (
	IsTaskNotFound       = sdk.IsTaskNotFound
	IsAlreadyClaimed     = sdk.IsAlreadyClaimed
	IsNotOwner           = sdk.IsNotOwner
	IsInvalidTransition  = sdk.IsInvalidTransition
	IsValidationFailed   = sdk.IsValidationFailed
	IsCycleDetected      = sdk.IsCycleDetected
	IsProjectNotFound    = sdk.IsProjectNotFound
	IsDependencyNotFound = sdk.IsDependencyNotFound
	IsServerNotRunning   = sdk.IsServerNotRunning
	IsServerUnhealthy    = sdk.IsServerUnhealthy
)

// IsConnectionError checks if the error indicates airyra server is unreachable
func IsConnectionError(err error) bool {
	return errors.Is(err, ErrServerNotRunning) || errors.Is(err, ErrServerUnhealthy)
}

// FormatError returns a user-friendly error message
func FormatError(err error) string {
	if IsConnectionError(err) {
		return "Airyra server is not running. Start it with: airyra server start"
	}
	if IsTaskNotFound(err) {
		return "Task not found"
	}
	if IsAlreadyClaimed(err) {
		return "Task is already claimed by another agent"
	}
	if IsNotOwner(err) {
		return "Cannot modify task - claimed by another agent"
	}
	if IsInvalidTransition(err) {
		return "Invalid status transition for this task"
	}
	if IsCycleDetected(err) {
		return "Cannot add dependency - would create a cycle"
	}
	return err.Error()
}
