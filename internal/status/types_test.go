package status

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTaskSummary_Total(t *testing.T) {
	tests := []struct {
		name     string
		summary  TaskSummary
		expected int
	}{
		{
			name:     "all zeros",
			summary:  TaskSummary{},
			expected: 0,
		},
		{
			name: "only ready tasks",
			summary: TaskSummary{
				Ready: 5,
			},
			expected: 5,
		},
		{
			name: "only in progress tasks",
			summary: TaskSummary{
				InProgress: 3,
			},
			expected: 3,
		},
		{
			name: "only blocked tasks",
			summary: TaskSummary{
				Blocked: 2,
			},
			expected: 2,
		},
		{
			name: "only completed tasks",
			summary: TaskSummary{
				Completed: 10,
			},
			expected: 10,
		},
		{
			name: "mixed task counts",
			summary: TaskSummary{
				Ready:      5,
				InProgress: 3,
				Blocked:    2,
				Completed:  10,
			},
			expected: 20,
		},
		{
			name: "large numbers",
			summary: TaskSummary{
				Ready:      100,
				InProgress: 200,
				Blocked:    300,
				Completed:  400,
			},
			expected: 1000,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.summary.Total()
			if got != tc.expected {
				t.Errorf("Total() = %d, want %d", got, tc.expected)
			}
		})
	}
}

func TestStatus_JSONSerialization(t *testing.T) {
	// Create a comprehensive status object
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	original := Status{
		Project:       "test-project",
		SessionActive: true,
		Workers: []WorkerStatus{
			{
				Name:       "worker-1",
				Status:     WorkerStatusRunning,
				IP:         "10.0.0.1",
				TaskID:     "ar-001",
				TaskTitle:  "Implement feature X",
				TaskBranch: "isollm/ar-001",
				Duration:   2 * time.Hour,
			},
			{
				Name:   "worker-2",
				Status: WorkerStatusStopped,
			},
		},
		Tasks: TaskSummary{
			Ready:      5,
			InProgress: 2,
			Blocked:    1,
			Completed:  10,
		},
		Sync: SyncStatus{
			HostBranch:    "main",
			HostCommit:    "abc123",
			HostAhead:     3,
			TotalBranches: 5,
		},
		Services: ServiceStatus{
			Airyra: ServiceInfo{
				Running: true,
			},
			Zellij: ServiceInfo{
				Running: false,
				Error:   "not installed",
			},
		},
		Timestamp: now,
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal status: %v", err)
	}

	// Unmarshal back
	var decoded Status
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal status: %v", err)
	}

	// Verify fields
	if decoded.Project != original.Project {
		t.Errorf("Project = %q, want %q", decoded.Project, original.Project)
	}
	if decoded.SessionActive != original.SessionActive {
		t.Errorf("SessionActive = %v, want %v", decoded.SessionActive, original.SessionActive)
	}
	if len(decoded.Workers) != len(original.Workers) {
		t.Errorf("len(Workers) = %d, want %d", len(decoded.Workers), len(original.Workers))
	}
	if decoded.Tasks.Total() != original.Tasks.Total() {
		t.Errorf("Tasks.Total() = %d, want %d", decoded.Tasks.Total(), original.Tasks.Total())
	}
	if decoded.Sync.HostBranch != original.Sync.HostBranch {
		t.Errorf("Sync.HostBranch = %q, want %q", decoded.Sync.HostBranch, original.Sync.HostBranch)
	}
	if decoded.Services.Airyra.Running != original.Services.Airyra.Running {
		t.Errorf("Services.Airyra.Running = %v, want %v", decoded.Services.Airyra.Running, original.Services.Airyra.Running)
	}
}

func TestStatus_JSONSerialization_EmptyStatus(t *testing.T) {
	original := Status{
		Project:   "empty-project",
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal empty status: %v", err)
	}

	var decoded Status
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal empty status: %v", err)
	}

	if decoded.Project != original.Project {
		t.Errorf("Project = %q, want %q", decoded.Project, original.Project)
	}
	if decoded.SessionActive != false {
		t.Errorf("SessionActive = %v, want false", decoded.SessionActive)
	}
	if decoded.Workers != nil && len(decoded.Workers) > 0 {
		t.Errorf("Workers should be nil or empty, got %v", decoded.Workers)
	}
}

func TestWorkerStatus_JSONSerialization(t *testing.T) {
	tests := []struct {
		name   string
		worker WorkerStatus
	}{
		{
			name: "running worker with task",
			worker: WorkerStatus{
				Name:       "worker-1",
				Status:     WorkerStatusRunning,
				IP:         "10.0.0.1",
				TaskID:     "ar-001",
				TaskTitle:  "Test task",
				TaskBranch: "isollm/ar-001",
				Duration:   30 * time.Minute,
			},
		},
		{
			name: "stopped worker without task",
			worker: WorkerStatus{
				Name:   "worker-2",
				Status: WorkerStatusStopped,
			},
		},
		{
			name: "worker with unknown status",
			worker: WorkerStatus{
				Name:   "worker-3",
				Status: WorkerStatusUnknown,
				IP:     "10.0.0.3",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Marshal
			data, err := json.Marshal(tc.worker)
			if err != nil {
				t.Fatalf("Failed to marshal worker: %v", err)
			}

			// Unmarshal
			var decoded WorkerStatus
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("Failed to unmarshal worker: %v", err)
			}

			// Verify required fields
			if decoded.Name != tc.worker.Name {
				t.Errorf("Name = %q, want %q", decoded.Name, tc.worker.Name)
			}
			if decoded.Status != tc.worker.Status {
				t.Errorf("Status = %q, want %q", decoded.Status, tc.worker.Status)
			}
			if decoded.IP != tc.worker.IP {
				t.Errorf("IP = %q, want %q", decoded.IP, tc.worker.IP)
			}
			if decoded.TaskID != tc.worker.TaskID {
				t.Errorf("TaskID = %q, want %q", decoded.TaskID, tc.worker.TaskID)
			}
		})
	}
}

func TestWorkerStatus_JSONTags_OmitEmpty(t *testing.T) {
	// Worker with minimal fields
	worker := WorkerStatus{
		Name:   "worker-1",
		Status: WorkerStatusStopped,
	}

	data, err := json.Marshal(worker)
	if err != nil {
		t.Fatalf("Failed to marshal worker: %v", err)
	}

	// Check that omitempty fields are not present in JSON
	jsonStr := string(data)

	// IP, TaskID, TaskTitle, TaskBranch, Duration should be omitted when empty
	if contains(jsonStr, `"ip"`) {
		t.Error("IP should be omitted when empty")
	}
	if contains(jsonStr, `"task_id"`) {
		t.Error("TaskID should be omitted when empty")
	}
	if contains(jsonStr, `"task_title"`) {
		t.Error("TaskTitle should be omitted when empty")
	}
	if contains(jsonStr, `"task_branch"`) {
		t.Error("TaskBranch should be omitted when empty")
	}
	// Duration 0 should be omitted
	if contains(jsonStr, `"duration"`) {
		t.Error("Duration should be omitted when zero")
	}

	// Required fields should be present
	if !contains(jsonStr, `"name"`) {
		t.Error("Name should be present")
	}
	if !contains(jsonStr, `"status"`) {
		t.Error("Status should be present")
	}
}

func TestServiceInfo_JSONSerialization(t *testing.T) {
	tests := []struct {
		name    string
		service ServiceInfo
	}{
		{
			name: "running service",
			service: ServiceInfo{
				Running: true,
			},
		},
		{
			name: "stopped service with error",
			service: ServiceInfo{
				Running: false,
				Error:   "service unavailable",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.service)
			if err != nil {
				t.Fatalf("Failed to marshal service info: %v", err)
			}

			var decoded ServiceInfo
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("Failed to unmarshal service info: %v", err)
			}

			if decoded.Running != tc.service.Running {
				t.Errorf("Running = %v, want %v", decoded.Running, tc.service.Running)
			}
			if decoded.Error != tc.service.Error {
				t.Errorf("Error = %q, want %q", decoded.Error, tc.service.Error)
			}
		})
	}
}

func TestSyncStatus_JSONSerialization(t *testing.T) {
	sync := SyncStatus{
		HostBranch:    "main",
		HostCommit:    "abc123def",
		HostAhead:     5,
		TotalBranches: 3,
	}

	data, err := json.Marshal(sync)
	if err != nil {
		t.Fatalf("Failed to marshal sync status: %v", err)
	}

	var decoded SyncStatus
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal sync status: %v", err)
	}

	if decoded.HostBranch != sync.HostBranch {
		t.Errorf("HostBranch = %q, want %q", decoded.HostBranch, sync.HostBranch)
	}
	if decoded.HostCommit != sync.HostCommit {
		t.Errorf("HostCommit = %q, want %q", decoded.HostCommit, sync.HostCommit)
	}
	if decoded.HostAhead != sync.HostAhead {
		t.Errorf("HostAhead = %d, want %d", decoded.HostAhead, sync.HostAhead)
	}
	if decoded.TotalBranches != sync.TotalBranches {
		t.Errorf("TotalBranches = %d, want %d", decoded.TotalBranches, sync.TotalBranches)
	}
}

func TestTaskSummary_JSONSerialization(t *testing.T) {
	summary := TaskSummary{
		Ready:      10,
		InProgress: 5,
		Blocked:    2,
		Completed:  20,
	}

	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("Failed to marshal task summary: %v", err)
	}

	var decoded TaskSummary
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal task summary: %v", err)
	}

	if decoded.Ready != summary.Ready {
		t.Errorf("Ready = %d, want %d", decoded.Ready, summary.Ready)
	}
	if decoded.InProgress != summary.InProgress {
		t.Errorf("InProgress = %d, want %d", decoded.InProgress, summary.InProgress)
	}
	if decoded.Blocked != summary.Blocked {
		t.Errorf("Blocked = %d, want %d", decoded.Blocked, summary.Blocked)
	}
	if decoded.Completed != summary.Completed {
		t.Errorf("Completed = %d, want %d", decoded.Completed, summary.Completed)
	}
}

func TestWorkerStatusConstants(t *testing.T) {
	// Verify constants have expected values
	if WorkerStatusRunning != "RUNNING" {
		t.Errorf("WorkerStatusRunning = %q, want %q", WorkerStatusRunning, "RUNNING")
	}
	if WorkerStatusStopped != "STOPPED" {
		t.Errorf("WorkerStatusStopped = %q, want %q", WorkerStatusStopped, "STOPPED")
	}
	if WorkerStatusUnknown != "UNKNOWN" {
		t.Errorf("WorkerStatusUnknown = %q, want %q", WorkerStatusUnknown, "UNKNOWN")
	}
}

// contains is a simple helper to check if a string contains a substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
