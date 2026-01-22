package worker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"lxc-dev-manager/pkg/lxcmgr"

	"isollm/internal/airyra"
	"isollm/internal/config"
)

const (
	// WorkerPrefix is the prefix for worker container names
	WorkerPrefix = "worker-"
	// CleanSnapshotName is the name of the clean state snapshot
	CleanSnapshotName = "clean"
	// RepoMountPath is where the bare repo is mounted in containers
	RepoMountPath = "/repo.git"
	// ProjectPath is where the cloned repo lives in containers
	ProjectPath = "/home/dev/project"
)

// Manager is a thin wrapper around lxcmgr.Client.
// It handles worker naming and task state only - all container
// management is delegated to lxc-dev-manager.
type Manager struct {
	client   *lxcmgr.Client
	cfg      *config.Config
	stateDir string
	bareRepo string
	airyra   airyra.TaskClient // May be nil if airyra is not running
}

// WorkerInfo contains combined information about a worker
type WorkerInfo struct {
	Name      string
	Status    lxcmgr.ContainerStatus
	IP        string
	Ports     []int
	TaskID    string
	Branch    string
	ClaimedAt time.Time
}

// NewManager creates a Manager for the given project directory
func NewManager(projectDir string, cfg *config.Config) (*Manager, error) {
	// Get the lxc-dev-manager project directory
	lxcProjectDir := filepath.Join(projectDir, config.StateDir, "lxc-project")

	// Try to open existing project
	client, err := lxcmgr.New(lxcProjectDir)
	if err != nil {
		if !errors.Is(err, lxcmgr.ErrProjectNotFound) {
			return nil, fmt.Errorf("failed to open lxc-dev-manager project: %w", err)
		}

		// Create new project
		var ports []int
		for _, p := range cfg.Ports {
			port, err := parsePort(p)
			if err != nil {
				continue
			}
			ports = append(ports, port)
		}

		client, err = lxcmgr.NewProject(lxcProjectDir,
			lxcmgr.WithProjectName(cfg.Project),
			lxcmgr.WithDefaultPorts(ports...),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create lxc-dev-manager project: %w", err)
		}
	}

	// Determine bare repo path
	bareRepo, err := getBareRepoPath(cfg.Project)
	if err != nil {
		return nil, fmt.Errorf("failed to determine bare repo path: %w", err)
	}

	// Initialize airyra client for task operations (non-fatal if not running)
	airyraClient, _ := airyra.NewClientFromConfig(cfg)

	return &Manager{
		client:   client,
		cfg:      cfg,
		stateDir: filepath.Join(projectDir, config.StateDir, "tasks"),
		bareRepo: bareRepo,
		airyra:   airyraClient,
	}, nil
}

// CreateWorker creates a new worker container.
// If name is empty, the next available worker name is used.
func (m *Manager) CreateWorker(name string) error {
	if name == "" {
		name = m.nextWorkerName()
	}

	if !strings.HasPrefix(name, WorkerPrefix) {
		name = WorkerPrefix + name
	}

	// 1. Create container with dev user
	err := m.client.CreateContainer(name, m.cfg.Image,
		lxcmgr.WithUser("dev", "dev"),
	)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	// 2. Start the container
	if err := m.client.Start(name); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	// 3. Wait for container to be ready
	if err := m.client.WaitForReady(name, 60*time.Second); err != nil {
		return fmt.Errorf("container not ready: %w", err)
	}

	// 4. Mount bare repo with UID shifting
	if err := m.client.Mount(name, m.bareRepo, RepoMountPath,
		lxcmgr.WithMountName("repo"),
		lxcmgr.WithReadWrite(),
		lxcmgr.WithShift(),
	); err != nil {
		return fmt.Errorf("failed to mount bare repo: %w", err)
	}

	// 5. Clone repo in container
	_, err = m.client.Exec(name, []string{
		"git", "clone", RepoMountPath, ProjectPath,
	})
	if err != nil {
		return fmt.Errorf("failed to clone repo in container: %w", err)
	}

	// 6. Set git config in the cloned repo
	_, err = m.client.Exec(name, []string{
		"git", "-C", ProjectPath, "config", "user.name", "Worker",
	})
	if err != nil {
		return fmt.Errorf("failed to set git user.name: %w", err)
	}

	_, err = m.client.Exec(name, []string{
		"git", "-C", ProjectPath, "config", "user.email", "worker@isollm.local",
	})
	if err != nil {
		return fmt.Errorf("failed to set git user.email: %w", err)
	}

	// 7. Create "clean" snapshot for reset
	if err := m.client.CreateSnapshot(name, CleanSnapshotName, "Clean state after repo clone"); err != nil {
		return fmt.Errorf("failed to create clean snapshot: %w", err)
	}

	return nil
}

// Start starts a stopped worker
func (m *Manager) Start(name string) error {
	name = m.normalizeName(name)
	return m.client.Start(name)
}

// Stop stops a running worker
func (m *Manager) Stop(name string) error {
	name = m.normalizeName(name)
	return m.client.Stop(name)
}

// Remove removes a worker container
func (m *Manager) Remove(name string, force bool) error {
	name = m.normalizeName(name)

	// Clear any task state
	if err := ClearTaskState(m.stateDir, name); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to clear task state: %w", err)
	}

	return m.client.Remove(name, force)
}

// Reset resets a worker to its clean snapshot state
func (m *Manager) Reset(name string) error {
	name = m.normalizeName(name)

	// Clear task state
	if err := ClearTaskState(m.stateDir, name); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to clear task state: %w", err)
	}

	return m.client.Reset(name, CleanSnapshotName)
}

// Shell opens an interactive shell in a worker
func (m *Manager) Shell(name string) error {
	name = m.normalizeName(name)
	return m.client.Shell(name, lxcmgr.AsUser("dev"))
}

// Exec runs a command inside a worker
func (m *Manager) Exec(name string, cmd []string) ([]byte, error) {
	name = m.normalizeName(name)
	return m.client.Exec(name, cmd)
}

// List returns information about all workers
func (m *Manager) List() ([]WorkerInfo, error) {
	containers, err := m.client.List()
	if err != nil {
		return nil, err
	}

	var workers []WorkerInfo
	for _, c := range containers {
		// Only include workers (containers matching our prefix)
		if !strings.HasPrefix(c.Name, WorkerPrefix) {
			continue
		}

		info := WorkerInfo{
			Name:   c.Name,
			Status: c.Status,
			IP:     c.IP,
			Ports:  c.Ports,
		}

		// Load task state if available
		state, err := LoadTaskState(m.stateDir, c.Name)
		if err == nil && state != nil {
			info.TaskID = state.TaskID
			info.Branch = state.Branch
			info.ClaimedAt = state.ClaimedAt
		}

		workers = append(workers, info)
	}

	// Sort by name
	sort.Slice(workers, func(i, j int) bool {
		return workers[i].Name < workers[j].Name
	})

	return workers, nil
}

// Status returns the status of a worker
func (m *Manager) Status(name string) (lxcmgr.ContainerStatus, error) {
	name = m.normalizeName(name)
	return m.client.Status(name)
}

// IP returns the IP address of a worker
func (m *Manager) IP(name string) (string, error) {
	name = m.normalizeName(name)
	return m.client.IP(name)
}

// Exists checks if a worker exists
func (m *Manager) Exists(name string) bool {
	name = m.normalizeName(name)
	return m.client.Exists(name)
}

// StartProxy starts port forwarding for a worker
func (m *Manager) StartProxy(name string) (*lxcmgr.ProxyManager, error) {
	name = m.normalizeName(name)
	return m.client.StartProxy(name)
}

// AssignTask assigns a task to a worker
func (m *Manager) AssignTask(name, taskID, branch string) error {
	name = m.normalizeName(name)

	state := &TaskState{
		WorkerName: name,
		TaskID:     taskID,
		Branch:     branch,
		ClaimedAt:  time.Now(),
	}

	return SaveTaskState(m.stateDir, state)
}

// ClearTask clears the task assignment from a worker
func (m *Manager) ClearTask(name string) error {
	name = m.normalizeName(name)
	return ClearTaskState(m.stateDir, name)
}

// GetTask returns the current task state for a worker
func (m *Manager) GetTask(name string) (*TaskState, error) {
	name = m.normalizeName(name)
	return LoadTaskState(m.stateDir, name)
}

// ListSnapshots returns all snapshots for a worker
func (m *Manager) ListSnapshots(name string) ([]lxcmgr.SnapshotInfo, error) {
	name = m.normalizeName(name)
	return m.client.ListSnapshots(name)
}

// CreateSnapshot creates a new snapshot of a worker
func (m *Manager) CreateSnapshot(name, snapName, description string) error {
	name = m.normalizeName(name)
	return m.client.CreateSnapshot(name, snapName, description)
}

// normalizeName ensures the name has the worker prefix
func (m *Manager) normalizeName(name string) string {
	if !strings.HasPrefix(name, WorkerPrefix) {
		return WorkerPrefix + name
	}
	return name
}

// --- Airyra Integration ---

// ClaimNextTask claims the highest priority ready task from airyra
func (m *Manager) ClaimNextTask(ctx context.Context, workerName string) (*airyra.Task, error) {
	if m.airyra == nil {
		return nil, fmt.Errorf("airyra client not initialized")
	}

	workerName = m.normalizeName(workerName)

	// Get ready tasks
	ready, err := m.airyra.ListReadyTasks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list ready tasks: %w", err)
	}

	if len(ready.Tasks) == 0 {
		return nil, nil // No tasks available
	}

	// Claim the first (highest priority) task
	task, err := m.airyra.ClaimTask(ctx, ready.Tasks[0].ID)
	if err != nil {
		if airyra.IsAlreadyClaimed(err) {
			// Race condition - another worker claimed it, try next
			return m.ClaimNextTask(ctx, workerName)
		}
		return nil, fmt.Errorf("failed to claim task: %w", err)
	}

	// Update local state
	branchName := fmt.Sprintf("%s%s", m.cfg.Git.BranchPrefix, task.ID)
	if err := m.AssignTask(workerName, task.ID, branchName); err != nil {
		// Log but don't fail - airyra is authoritative
	}

	return task, nil
}

// ReleaseWorkerTask releases the task a worker is working on
func (m *Manager) ReleaseWorkerTask(ctx context.Context, workerName string) error {
	if m.airyra == nil {
		return fmt.Errorf("airyra client not initialized")
	}

	workerName = m.normalizeName(workerName)

	state, err := m.GetTask(workerName)
	if err != nil || state == nil || state.TaskID == "" {
		return nil // No task to release
	}

	_, err = m.airyra.ReleaseTask(ctx, state.TaskID, false)
	if err != nil && !airyra.IsNotOwner(err) {
		return fmt.Errorf("failed to release task: %w", err)
	}

	return m.ClearTask(workerName)
}

// CompleteWorkerTask marks the worker's current task as done
func (m *Manager) CompleteWorkerTask(ctx context.Context, workerName string) error {
	if m.airyra == nil {
		return fmt.Errorf("airyra client not initialized")
	}

	workerName = m.normalizeName(workerName)

	state, err := m.GetTask(workerName)
	if err != nil || state == nil || state.TaskID == "" {
		return fmt.Errorf("worker has no assigned task")
	}

	_, err = m.airyra.CompleteTask(ctx, state.TaskID)
	if err != nil {
		return fmt.Errorf("failed to complete task: %w", err)
	}

	return m.ClearTask(workerName)
}

// BlockWorkerTask marks the worker's current task as blocked
func (m *Manager) BlockWorkerTask(ctx context.Context, workerName string) error {
	if m.airyra == nil {
		return fmt.Errorf("airyra client not initialized")
	}

	workerName = m.normalizeName(workerName)

	state, err := m.GetTask(workerName)
	if err != nil || state == nil || state.TaskID == "" {
		return fmt.Errorf("worker has no assigned task")
	}

	_, err = m.airyra.BlockTask(ctx, state.TaskID)
	if err != nil {
		return fmt.Errorf("failed to block task: %w", err)
	}

	return nil
}

// HasAiryra returns true if the airyra client is available
func (m *Manager) HasAiryra() bool {
	return m.airyra != nil
}

// IsAiryraRunning checks if the airyra server is running
func (m *Manager) IsAiryraRunning(ctx context.Context) bool {
	if m.airyra == nil {
		return false
	}
	return m.airyra.IsServerRunning(ctx)
}

// SetAiryraClient sets the airyra client (for testing)
func (m *Manager) SetAiryraClient(client airyra.TaskClient) {
	m.airyra = client
}

// GetStateDir returns the state directory path (for testing)
func (m *Manager) GetStateDir() string {
	return m.stateDir
}

// SetStateDir sets the state directory path (for testing)
func (m *Manager) SetStateDir(dir string) {
	m.stateDir = dir
}

// LXCClient returns the underlying LXC client for direct access
func (m *Manager) LXCClient() *lxcmgr.Client {
	return m.client
}

// WriteFile writes content to a file inside the specified worker container
func (m *Manager) WriteFile(workerName, path, content string) error {
	workerName = m.normalizeName(workerName)

	// Use lxc exec to write the file via shell
	// We use sh -c with printf to handle special characters properly
	cmd := []string{"sh", "-c", fmt.Sprintf("printf '%%s' %q > %s", content, path)}
	_, err := m.client.Exec(workerName, cmd)
	if err != nil {
		return fmt.Errorf("failed to write file %s: %w", path, err)
	}
	return nil
}

// ExecInWorker executes a command inside a worker container and returns the output
func (m *Manager) ExecInWorker(workerName string, command string, args ...string) (string, error) {
	workerName = m.normalizeName(workerName)

	// Build the command slice
	cmd := append([]string{command}, args...)

	output, err := m.client.Exec(workerName, cmd)
	if err != nil {
		return "", fmt.Errorf("failed to execute command in %s: %w", workerName, err)
	}
	return string(output), nil
}

// nextWorkerName returns the next available worker name
func (m *Manager) nextWorkerName() string {
	containers, err := m.client.List()
	if err != nil {
		return WorkerPrefix + "1"
	}

	maxNum := 0
	for _, c := range containers {
		if strings.HasPrefix(c.Name, WorkerPrefix) {
			numStr := strings.TrimPrefix(c.Name, WorkerPrefix)
			if num, err := strconv.Atoi(numStr); err == nil && num > maxNum {
				maxNum = num
			}
		}
	}

	return WorkerPrefix + strconv.Itoa(maxNum+1)
}

// getBareRepoPath returns the standard bare repo path for a project
func getBareRepoPath(projectName string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".isollm", projectName+".git"), nil
}

// parsePort parses a port specification (handles "8080" or "8080:8080")
func parsePort(spec string) (int, error) {
	// Handle "hostPort:containerPort" format
	if idx := strings.Index(spec, ":"); idx != -1 {
		spec = spec[:idx]
	}
	return strconv.Atoi(spec)
}
