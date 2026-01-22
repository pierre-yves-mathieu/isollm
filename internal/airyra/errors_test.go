package airyra

import (
	"errors"
	"testing"

	sdk "airyra/pkg/airyra"
)

func TestIsConnectionError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"server not running", ErrServerNotRunning, true},
		{"server unhealthy", ErrServerUnhealthy, true},
		{"wrapped server not running", errors.Join(errors.New("connection failed"), ErrServerNotRunning), true},
		{"generic error", errors.New("some error"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsConnectionError(tt.err)
			if got != tt.want {
				t.Errorf("IsConnectionError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestFormatError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		contains string
	}{
		{
			name:     "connection error",
			err:      ErrServerNotRunning,
			contains: "not running",
		},
		{
			name:     "generic error passes through",
			err:      errors.New("custom error message"),
			contains: "custom error message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatError(tt.err)
			if got == "" {
				t.Error("FormatError returned empty string")
			}
			if !contains(got, tt.contains) {
				t.Errorf("FormatError(%v) = %q, want to contain %q", tt.err, got, tt.contains)
			}
		})
	}
}

func TestErrorCheckersExported(t *testing.T) {
	// Verify that error checkers are properly exported and callable
	// These should all return false for nil
	checkers := []struct {
		name string
		fn   func(error) bool
	}{
		{"IsTaskNotFound", IsTaskNotFound},
		{"IsAlreadyClaimed", IsAlreadyClaimed},
		{"IsNotOwner", IsNotOwner},
		{"IsInvalidTransition", IsInvalidTransition},
		{"IsValidationFailed", IsValidationFailed},
		{"IsCycleDetected", IsCycleDetected},
		{"IsProjectNotFound", IsProjectNotFound},
		{"IsDependencyNotFound", IsDependencyNotFound},
		{"IsServerNotRunning", IsServerNotRunning},
		{"IsServerUnhealthy", IsServerUnhealthy},
	}

	for _, tc := range checkers {
		t.Run(tc.name+"_nil", func(t *testing.T) {
			if tc.fn(nil) {
				t.Errorf("%s(nil) = true, want false", tc.name)
			}
		})

		t.Run(tc.name+"_generic_error", func(t *testing.T) {
			if tc.fn(errors.New("generic")) {
				t.Errorf("%s(generic error) = true, want false", tc.name)
			}
		})
	}
}

func TestSentinelErrorsExported(t *testing.T) {
	// Verify sentinel errors match SDK errors
	if !errors.Is(ErrServerNotRunning, sdk.ErrServerNotRunning) {
		t.Error("ErrServerNotRunning should match sdk.ErrServerNotRunning")
	}
	if !errors.Is(ErrServerUnhealthy, sdk.ErrServerUnhealthy) {
		t.Error("ErrServerUnhealthy should match sdk.ErrServerUnhealthy")
	}
}

func TestFormatErrorAllTypes(t *testing.T) {
	// Test that FormatError handles various error types gracefully
	testCases := []struct {
		name string
		err  error
	}{
		{"server not running", ErrServerNotRunning},
		{"server unhealthy", ErrServerUnhealthy},
		{"simple error", errors.New("simple")},
		{"wrapped error", errors.Join(errors.New("outer"), errors.New("inner"))},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := FormatError(tc.err)
			if result == "" {
				t.Error("FormatError should not return empty string")
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
