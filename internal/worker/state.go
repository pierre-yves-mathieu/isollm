package worker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// TaskState represents the task assignment for a worker.
// This is the only state isollm tracks locally - container status,
// IP, ports, etc. are queried from lxc-dev-manager.
type TaskState struct {
	WorkerName string    `json:"worker_name"`
	TaskID     string    `json:"task_id,omitempty"`
	Branch     string    `json:"branch,omitempty"`
	ClaimedAt  time.Time `json:"claimed_at,omitempty"`
}

// SaveTaskState saves the task state to disk
func SaveTaskState(stateDir string, state *TaskState) error {
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	path := filepath.Join(stateDir, state.WorkerName+".json")

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal task state: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write task state: %w", err)
	}

	return nil
}

// LoadTaskState loads the task state from disk
func LoadTaskState(stateDir, workerName string) (*TaskState, error) {
	path := filepath.Join(stateDir, workerName+".json")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read task state: %w", err)
	}

	var state TaskState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal task state: %w", err)
	}

	return &state, nil
}

// ClearTaskState removes the task state file
func ClearTaskState(stateDir, workerName string) error {
	path := filepath.Join(stateDir, workerName+".json")
	return os.Remove(path)
}

// ListTaskStates returns all task states in the directory
func ListTaskStates(stateDir string) ([]*TaskState, error) {
	entries, err := os.ReadDir(stateDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read state directory: %w", err)
	}

	var states []*TaskState
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		workerName := entry.Name()[:len(entry.Name())-5] // strip .json
		state, err := LoadTaskState(stateDir, workerName)
		if err != nil {
			continue
		}
		if state != nil {
			states = append(states, state)
		}
	}

	return states, nil
}
