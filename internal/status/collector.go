package status

import (
	"context"
	"os"
	"os/exec"
	"sync"
	"time"

	"isollm/internal/airyra"
	"isollm/internal/barerepo"
	"isollm/internal/config"
	"isollm/internal/git"
	"isollm/internal/worker"
)

// Collector gathers status information from various sources
type Collector struct {
	projectDir string
	cfg        *config.Config
	manager    *worker.Manager
	airyra     airyra.TaskClient
	bareRepo   *barerepo.BareRepo
	gitExec    git.Executor
}

// NewCollector creates a new status collector
func NewCollector(projectDir string, cfg *config.Config) (*Collector, error) {
	// Create worker manager
	mgr, err := worker.NewManager(projectDir, cfg)
	if err != nil {
		return nil, err
	}

	// Try to create airyra client (non-fatal if not running)
	airyraClient, _ := airyra.NewClientFromConfig(cfg)

	// Get bare repo path
	barePath, err := barerepo.GetMountPath(cfg.Project)
	if err != nil {
		return nil, err
	}

	var repo *barerepo.BareRepo
	if barerepo.Exists(barePath) {
		repo = barerepo.New(barePath)
	}

	return &Collector{
		projectDir: projectDir,
		cfg:        cfg,
		manager:    mgr,
		airyra:     airyraClient,
		bareRepo:   repo,
		gitExec:    git.DefaultExecutor,
	}, nil
}

// Collect gathers all status information in parallel
func (c *Collector) Collect(ctx context.Context) (*Status, error) {
	status := &Status{
		Project:   c.cfg.Project,
		Timestamp: time.Now(),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	// Collect workers
	wg.Add(1)
	go func() {
		defer wg.Done()
		workers := c.collectWorkers(ctx)
		mu.Lock()
		status.Workers = workers
		// Check if any workers are running to determine session active
		for _, w := range workers {
			if w.Status == WorkerStatusRunning {
				status.SessionActive = true
				break
			}
		}
		mu.Unlock()
	}()

	// Collect tasks
	wg.Add(1)
	go func() {
		defer wg.Done()
		tasks := c.collectTasks(ctx)
		mu.Lock()
		status.Tasks = tasks
		mu.Unlock()
	}()

	// Collect sync status
	wg.Add(1)
	go func() {
		defer wg.Done()
		syncStatus := c.collectSync(ctx)
		mu.Lock()
		status.Sync = syncStatus
		mu.Unlock()
	}()

	// Collect service status
	wg.Add(1)
	go func() {
		defer wg.Done()
		services := c.collectServices(ctx)
		mu.Lock()
		status.Services = services
		mu.Unlock()
	}()

	wg.Wait()

	return status, nil
}

// collectWorkers gathers worker status information
func (c *Collector) collectWorkers(ctx context.Context) []WorkerStatus {
	workers, err := c.manager.List()
	if err != nil {
		return nil
	}

	var result []WorkerStatus
	for _, w := range workers {
		ws := WorkerStatus{
			Name:   w.Name,
			Status: string(w.Status),
			IP:     w.IP,
		}

		// Normalize status
		switch w.Status {
		case "RUNNING":
			ws.Status = WorkerStatusRunning
		case "STOPPED":
			ws.Status = WorkerStatusStopped
		default:
			ws.Status = WorkerStatusUnknown
		}

		// Add task info if assigned
		if w.TaskID != "" {
			ws.TaskID = w.TaskID
			ws.TaskBranch = w.Branch

			// Calculate duration
			if !w.ClaimedAt.IsZero() {
				ws.Duration = time.Since(w.ClaimedAt)
			}

			// Try to get task title from airyra
			if c.airyra != nil {
				if task, err := c.airyra.GetTask(ctx, w.TaskID); err == nil && task != nil {
					ws.TaskTitle = task.Title
				}
			}
		}

		result = append(result, ws)
	}

	return result
}

// collectTasks gathers task summary from airyra
func (c *Collector) collectTasks(ctx context.Context) TaskSummary {
	summary := TaskSummary{}

	if c.airyra == nil {
		return summary
	}

	// Check if server is running first
	if !c.airyra.IsServerRunning(ctx) {
		return summary
	}

	// Get all tasks
	list, err := c.airyra.ListTasks(ctx)
	if err != nil {
		return summary
	}

	// Count by status
	for _, task := range list.Tasks {
		switch task.Status {
		case airyra.StatusOpen:
			summary.Ready++
		case airyra.StatusInProgress:
			summary.InProgress++
		case airyra.StatusBlocked:
			summary.Blocked++
		case airyra.StatusDone:
			summary.Completed++
		}
	}

	return summary
}

// collectSync gathers git sync status
func (c *Collector) collectSync(ctx context.Context) SyncStatus {
	syncStatus := SyncStatus{
		HostBranch: c.cfg.Git.BaseBranch,
	}

	// Get current commit on host
	if commit, err := c.gitExec.Run(c.projectDir, "rev-parse", "--short", c.cfg.Git.BaseBranch); err == nil {
		syncStatus.HostCommit = commit
	}

	if c.bareRepo == nil {
		return syncStatus
	}

	// Check if host is ahead of bare repo
	if ahead, err := c.bareRepo.IsHostAhead(c.projectDir, c.cfg.Git.BaseBranch); err == nil {
		syncStatus.HostAhead = ahead
	}

	// List task branches
	if branches, err := c.bareRepo.ListTaskBranches(); err == nil {
		syncStatus.TaskBranches = branches
		syncStatus.TotalBranches = len(branches)
	}

	return syncStatus
}

// collectServices gathers service status
func (c *Collector) collectServices(ctx context.Context) ServiceStatus {
	services := ServiceStatus{}

	// Check airyra
	if c.airyra != nil {
		services.Airyra.Running = c.airyra.IsServerRunning(ctx)
	} else {
		services.Airyra.Error = "client not configured"
	}

	// Check zellij
	services.Zellij = c.checkZellij()

	return services
}

// checkZellij checks if zellij is running with an isollm session
func (c *Collector) checkZellij() ServiceInfo {
	info := ServiceInfo{}

	// Check if zellij is installed
	_, err := exec.LookPath("zellij")
	if err != nil {
		info.Error = "not installed"
		return info
	}

	// Check if a zellij session exists for this project
	sessionName := c.cfg.Project
	cmd := exec.Command("zellij", "list-sessions")
	output, err := cmd.Output()
	if err != nil {
		info.Error = "no sessions"
		return info
	}

	// Check if our session is in the list
	sessions := string(output)
	if containsSession(sessions, sessionName) {
		info.Running = true
	} else {
		info.Error = "session not found"
	}

	return info
}

// containsSession checks if a session name appears in the zellij session list
func containsSession(sessions, name string) bool {
	// zellij list-sessions outputs one session per line
	// Format varies by version but name is typically at the start
	lines := splitLines(sessions)
	for _, line := range lines {
		if len(line) > 0 && (line == name || hasPrefix(line, name+" ") || hasPrefix(line, name+"\t")) {
			return true
		}
	}
	return false
}

// splitLines splits a string into lines
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// hasPrefix checks if s starts with prefix
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// GetProjectDir returns the project directory (for external use)
func (c *Collector) GetProjectDir() string {
	return c.projectDir
}

// GetConfig returns the config (for external use)
func (c *Collector) GetConfig() *config.Config {
	return c.cfg
}

// IsZellijInstalled checks if zellij is available
func IsZellijInstalled() bool {
	_, err := exec.LookPath("zellij")
	return err == nil
}

// IsInZellijSession checks if we're currently inside a zellij session
func IsInZellijSession() bool {
	return os.Getenv("ZELLIJ") != ""
}
