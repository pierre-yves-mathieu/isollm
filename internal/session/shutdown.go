package session

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"isollm/internal/airyra"
	"isollm/internal/config"
	"isollm/internal/git"
	"isollm/internal/worker"
	"isollm/internal/zellij"
)

// ShutdownOptions configures the shutdown behavior
type ShutdownOptions struct {
	Destroy             bool
	SaveSnapshots       bool
	SkipConfirm         bool
	ReleaseTasksTimeout time.Duration
}

// WorkerShutdownInfo contains information about a worker's unsaved state
type WorkerShutdownInfo struct {
	Name            string
	TaskID          string
	Branch          string
	UnpushedCommits int
	HasUncommitted  bool
}

// Shutdown handles graceful shutdown of all workers
type Shutdown struct {
	projectDir string
	cfg        *config.Config
	opts       ShutdownOptions
	mgr        *worker.Manager
	zellij     *zellij.Manager
	airyra     airyra.TaskClient
	reader     *bufio.Reader
	gitExec    git.Executor
}

// NewShutdown creates a new Shutdown handler
func NewShutdown(projectDir string, cfg *config.Config, opts ShutdownOptions) (*Shutdown, error) {
	mgr, err := worker.NewManager(projectDir, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create worker manager: %w", err)
	}

	// Set default timeout
	if opts.ReleaseTasksTimeout == 0 {
		opts.ReleaseTasksTimeout = 30 * time.Second
	}

	// Initialize zellij manager (may fail if zellij not installed)
	zellijMgr, _ := zellij.NewManager()

	// Initialize airyra client (may fail if not running)
	airyraClient, _ := airyra.NewClientFromConfig(cfg)

	return &Shutdown{
		projectDir: projectDir,
		cfg:        cfg,
		opts:       opts,
		mgr:        mgr,
		zellij:     zellijMgr,
		airyra:     airyraClient,
		reader:     bufio.NewReader(os.Stdin),
		gitExec:    git.DefaultExecutor,
	}, nil
}

// Execute performs the graceful shutdown
func (s *Shutdown) Execute() error {
	fmt.Println("Shutting down isollm session...")
	fmt.Println()

	// Step 1: Gather worker info
	workers, err := s.mgr.List()
	if err != nil {
		return fmt.Errorf("failed to list workers: %w", err)
	}

	if len(workers) == 0 {
		fmt.Println("No workers to shut down")
		return s.cleanup()
	}

	// Step 2: Check for uncommitted/unpushed work
	unsavedWorkers, err := s.checkUnsavedWork(workers)
	if err != nil {
		return fmt.Errorf("failed to check worker states: %w", err)
	}

	// Step 3: Handle salvage prompt if work found
	if len(unsavedWorkers) > 0 && !s.opts.SkipConfirm {
		action, err := s.promptSalvage(unsavedWorkers)
		if err != nil {
			return err
		}

		switch action {
		case "salvage":
			if err := s.salvageWork(unsavedWorkers); err != nil {
				return fmt.Errorf("failed to salvage work: %w", err)
			}
		case "discard":
			fmt.Println("Discarding unsaved work...")
		case "cancel":
			fmt.Println("Shutdown cancelled")
			return nil
		}
	}

	// Step 4: Release claimed tasks back to airyra queue
	if err := s.releaseTasks(workers); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to release some tasks: %v\n", err)
	}

	// Step 5: Stop zellij session
	if err := s.stopZellijSession(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to stop zellij session: %v\n", err)
	}

	// Step 6: Save snapshots if requested
	if s.opts.SaveSnapshots {
		if err := s.saveSnapshots(workers); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to save some snapshots: %v\n", err)
		}
	}

	// Step 7: Stop or destroy containers
	if s.opts.Destroy {
		if err := s.destroyContainers(workers); err != nil {
			return err
		}
	} else {
		if err := s.stopContainers(workers); err != nil {
			return fmt.Errorf("failed to stop containers: %w", err)
		}
	}

	// Step 8: Run GC on bare repo (safe now that workers are stopped)
	if err := s.runBareRepoGC(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to run GC on bare repo: %v\n", err)
	}

	// Step 9: Clear session state
	if err := s.cleanup(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to clear session state: %v\n", err)
	}

	fmt.Println()
	fmt.Println("Shutdown complete")
	return nil
}

// checkUnsavedWork checks each worker for uncommitted or unpushed changes
func (s *Shutdown) checkUnsavedWork(workers []worker.WorkerInfo) ([]WorkerShutdownInfo, error) {
	var unsaved []WorkerShutdownInfo

	for _, w := range workers {
		if w.Status != "RUNNING" {
			continue
		}

		info := WorkerShutdownInfo{
			Name:   w.Name,
			TaskID: w.TaskID,
			Branch: w.Branch,
		}

		// Check for uncommitted changes (git status)
		statusOutput, err := s.mgr.Exec(w.Name, []string{"git", "-C", worker.ProjectPath, "status", "--porcelain"})
		if err == nil && len(strings.TrimSpace(string(statusOutput))) > 0 {
			info.HasUncommitted = true
		}

		// Check for unpushed commits (compare with origin)
		if w.Branch != "" {
			logOutput, err := s.mgr.Exec(w.Name, []string{
				"git", "-C", worker.ProjectPath,
				"rev-list", "--count", "origin/" + w.Branch + ".." + w.Branch,
			})
			if err == nil {
				var count int
				fmt.Sscanf(strings.TrimSpace(string(logOutput)), "%d", &count)
				info.UnpushedCommits = count
			} else {
				// Branch might not exist on origin yet, check all commits
				logOutput, err = s.mgr.Exec(w.Name, []string{
					"git", "-C", worker.ProjectPath,
					"rev-list", "--count", w.Branch,
				})
				if err == nil {
					var count int
					fmt.Sscanf(strings.TrimSpace(string(logOutput)), "%d", &count)
					info.UnpushedCommits = count
				}
			}
		}

		if info.HasUncommitted || info.UnpushedCommits > 0 {
			unsaved = append(unsaved, info)
		}
	}

	return unsaved, nil
}

// promptSalvage displays the salvage prompt and returns the user's choice
func (s *Shutdown) promptSalvage(unsaved []WorkerShutdownInfo) (string, error) {
	fmt.Println("Workers with unsaved work:")
	for _, w := range unsaved {
		desc := fmt.Sprintf("  %s", w.Name)
		if w.Branch != "" {
			desc += fmt.Sprintf(" (branch %s)", w.Branch)
		}
		desc += " -"
		if w.UnpushedCommits > 0 {
			desc += fmt.Sprintf(" %d unpushed commits", w.UnpushedCommits)
		}
		if w.HasUncommitted {
			if w.UnpushedCommits > 0 {
				desc += ","
			}
			desc += " has uncommitted changes"
		}
		fmt.Println(desc)
	}

	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  [s] Salvage - push branches before stopping")
	fmt.Println("  [d] Discard - stop without saving")
	fmt.Println("  [c] Cancel")
	fmt.Println()

	for {
		fmt.Print("Choice: ")
		input, err := s.reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("failed to read input: %w", err)
		}

		choice := strings.ToLower(strings.TrimSpace(input))
		switch choice {
		case "s", "salvage":
			return "salvage", nil
		case "d", "discard":
			return "discard", nil
		case "c", "cancel":
			return "cancel", nil
		default:
			fmt.Println("Invalid choice. Please enter 's', 'd', or 'c'.")
		}
	}
}

// salvageWork pushes branches from workers with unsaved work
func (s *Shutdown) salvageWork(unsaved []WorkerShutdownInfo) error {
	fmt.Println("Salvaging work...")

	for _, w := range unsaved {
		fmt.Printf("  %s: ", w.Name)

		// First commit any uncommitted changes
		if w.HasUncommitted {
			// Stage all changes
			_, err := s.mgr.Exec(w.Name, []string{"git", "-C", worker.ProjectPath, "add", "-A"})
			if err != nil {
				fmt.Printf("failed to stage changes: %v\n", err)
				continue
			}

			// Commit with salvage message
			commitMsg := fmt.Sprintf("isollm salvage: auto-commit before shutdown")
			_, err = s.mgr.Exec(w.Name, []string{
				"git", "-C", worker.ProjectPath,
				"commit", "-m", commitMsg,
			})
			if err != nil {
				fmt.Printf("failed to commit: %v\n", err)
				continue
			}
		}

		// Push the branch
		if w.Branch != "" {
			_, err := s.mgr.Exec(w.Name, []string{
				"git", "-C", worker.ProjectPath,
				"push", "-u", "origin", w.Branch,
			})
			if err != nil {
				fmt.Printf("failed to push: %v\n", err)
				continue
			}
		}

		fmt.Println("saved")
	}

	return nil
}

// releaseTasks releases all claimed tasks back to the airyra queue
func (s *Shutdown) releaseTasks(workers []worker.WorkerInfo) error {
	if s.airyra == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.opts.ReleaseTasksTimeout)
	defer cancel()

	// Check if airyra is running
	if !s.airyra.IsServerRunning(ctx) {
		return nil
	}

	fmt.Println("Releasing tasks...")

	var lastErr error
	for _, w := range workers {
		if w.TaskID == "" {
			continue
		}

		_, err := s.airyra.ReleaseTask(ctx, w.TaskID, false)
		if err != nil && !airyra.IsNotOwner(err) {
			fmt.Fprintf(os.Stderr, "  Warning: failed to release task %s: %v\n", w.TaskID, err)
			lastErr = err
			continue
		}

		// Clear local task state
		if err := s.mgr.ClearTask(w.Name); err != nil {
			fmt.Fprintf(os.Stderr, "  Warning: failed to clear task state for %s: %v\n", w.Name, err)
		}

		fmt.Printf("  Released %s from %s\n", w.TaskID, w.Name)
	}

	return lastErr
}

// stopZellijSession stops the zellij session for this project
func (s *Shutdown) stopZellijSession() error {
	if s.zellij == nil {
		return nil
	}

	sessionName := s.cfg.Project

	exists, err := s.zellij.SessionExists(sessionName)
	if err != nil || !exists {
		return nil
	}

	fmt.Printf("Stopping zellij session '%s'...\n", sessionName)
	return s.zellij.StopSession(sessionName)
}

// saveSnapshots creates snapshots of all workers
func (s *Shutdown) saveSnapshots(workers []worker.WorkerInfo) error {
	fmt.Println("Saving snapshots...")

	timestamp := time.Now().Format("20060102-150405")
	snapName := fmt.Sprintf("shutdown-%s", timestamp)

	var lastErr error
	for _, w := range workers {
		err := s.mgr.CreateSnapshot(w.Name, snapName, "Snapshot before shutdown")
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Warning: failed to snapshot %s: %v\n", w.Name, err)
			lastErr = err
			continue
		}
		fmt.Printf("  %s: snapshot '%s' created\n", w.Name, snapName)
	}

	return lastErr
}

// stopContainers stops all worker containers
func (s *Shutdown) stopContainers(workers []worker.WorkerInfo) error {
	fmt.Println("Stopping containers...")

	var lastErr error
	for _, w := range workers {
		if w.Status != "RUNNING" {
			continue
		}

		if err := s.mgr.Stop(w.Name); err != nil {
			fmt.Fprintf(os.Stderr, "  Warning: failed to stop %s: %v\n", w.Name, err)
			lastErr = err
			continue
		}
		fmt.Printf("  Stopped %s\n", w.Name)
	}

	return lastErr
}

// destroyContainers removes all worker containers after confirmation
func (s *Shutdown) destroyContainers(workers []worker.WorkerInfo) error {
	if !s.opts.SkipConfirm {
		fmt.Println()
		fmt.Println("This will permanently delete these containers:")
		for _, w := range workers {
			fmt.Printf("  %s\n", w.Name)
		}
		fmt.Println()
		fmt.Print("Type 'destroy' to confirm: ")

		input, err := s.reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}

		if strings.TrimSpace(input) != "destroy" {
			fmt.Println("Destruction cancelled")
			return nil
		}
	}

	fmt.Println("Destroying containers...")

	var lastErr error
	for _, w := range workers {
		// Stop first if running
		if w.Status == "RUNNING" {
			if err := s.mgr.Stop(w.Name); err != nil {
				fmt.Fprintf(os.Stderr, "  Warning: failed to stop %s: %v\n", w.Name, err)
			}
		}

		// Remove the container
		if err := s.mgr.Remove(w.Name, true); err != nil {
			fmt.Fprintf(os.Stderr, "  Warning: failed to remove %s: %v\n", w.Name, err)
			lastErr = err
			continue
		}
		fmt.Printf("  Destroyed %s\n", w.Name)
	}

	return lastErr
}

// runBareRepoGC runs garbage collection on the bare repo
func (s *Shutdown) runBareRepoGC() error {
	bareRepoPath, err := getBareRepoPath(s.cfg.Project)
	if err != nil {
		return err
	}

	// Check if bare repo exists
	if _, err := os.Stat(bareRepoPath); os.IsNotExist(err) {
		return nil
	}

	fmt.Println("Running garbage collection on bare repo...")
	return s.gitExec.RunSilent(bareRepoPath, "gc", "--auto")
}

// cleanup clears any remaining session state
func (s *Shutdown) cleanup() error {
	// Clear any stale session state files
	sessionStateDir := filepath.Join(s.projectDir, config.StateDir, "session")
	if _, err := os.Stat(sessionStateDir); err == nil {
		if err := os.RemoveAll(sessionStateDir); err != nil {
			return fmt.Errorf("failed to clear session state: %w", err)
		}
	}
	return nil
}

// getBareRepoPath returns the standard bare repo path for a project
func getBareRepoPath(projectName string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".isollm", projectName+".git"), nil
}
