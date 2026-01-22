package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"isollm/internal/airyra"
	"isollm/internal/barerepo"
	"isollm/internal/claude"
	"isollm/internal/config"
	"isollm/internal/worker"
	"isollm/internal/zellij"
)

// SessionState represents the saved session state
type SessionState struct {
	Project     string    `json:"project"`
	Workers     []string  `json:"workers"`
	StartedAt   time.Time `json:"started_at"`
	ZellijName  string    `json:"zellij_session,omitempty"`
	AiryraPort  int       `json:"airyra_port"`
	BaseBranch  string    `json:"base_branch"`
}

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start the isollm orchestration environment",
	Long: `Start the isollm orchestration environment.

This command:
1. Validates the configuration
2. Starts the airyra task server (if not running)
3. Creates the bare repo (if first run)
4. Creates/starts workers up to the configured count
5. Prepares Claude environment in each worker
6. Launches a zellij session with worker panes

Use --no-zellij to skip the zellij launch and just prepare workers.
Use --force to start even if the host repo has commits not in the bare repo.`,
	RunE: runUp,
}

var (
	upWorkers  int
	upBase     string
	upForce    bool
	upNoZellij bool
)

func init() {
	upCmd.Flags().IntVarP(&upWorkers, "workers", "n", 0, "Override worker count from config")
	upCmd.Flags().StringVar(&upBase, "base", "", "Override base branch")
	upCmd.Flags().BoolVar(&upForce, "force", false, "Start even with stale repo")
	upCmd.Flags().BoolVar(&upNoZellij, "no-zellij", false, "Skip zellij launch")

	rootCmd.AddCommand(upCmd)
}

func runUp(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// 1. Load and validate config
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	projectDir, err := config.FindProjectRoot(dir)
	if err != nil {
		return err
	}

	cfg, err := config.Load(projectDir)
	if err != nil {
		return err
	}

	// 2. Apply flag overrides
	if upWorkers > 0 {
		cfg.Workers = upWorkers
	}
	if upBase != "" {
		cfg.Git.BaseBranch = upBase
	}

	fmt.Printf("Starting isollm for project: %s\n", cfg.Project)
	fmt.Printf("  Workers: %d\n", cfg.Workers)
	fmt.Printf("  Base branch: %s\n", cfg.Git.BaseBranch)

	// 3. Check for stale repo
	bareRepoPath, err := barerepo.GetMountPath(cfg.Project)
	if err != nil {
		return fmt.Errorf("failed to get bare repo path: %w", err)
	}

	if barerepo.Exists(bareRepoPath) {
		repo := barerepo.New(bareRepoPath)
		ahead, err := repo.IsHostAhead(projectDir, cfg.Git.BaseBranch)
		if err != nil {
			// Non-fatal, just warn
			fmt.Fprintf(os.Stderr, "Warning: could not check repo staleness: %v\n", err)
		} else if ahead > 0 && !upForce {
			return fmt.Errorf("host repo is %d commit(s) ahead of bare repo on %s\n\n"+
				"Run 'isollm sync push' to update the bare repo, or use --force to start anyway",
				ahead, cfg.Git.BaseBranch)
		} else if ahead > 0 {
			fmt.Printf("  Warning: host is %d commit(s) ahead (--force used)\n", ahead)
		}
	}

	// 4. Start airyra server
	fmt.Print("Checking airyra server... ")
	if err := ensureAiryraRunning(ctx, cfg); err != nil {
		fmt.Println("failed")
		return fmt.Errorf("failed to start airyra server: %w", err)
	}
	fmt.Println("ok")

	// 5. Create bare repo if first run
	if !barerepo.Exists(bareRepoPath) {
		fmt.Print("Creating bare repo... ")
		_, err := barerepo.Create(projectDir, bareRepoPath)
		if err != nil {
			fmt.Println("failed")
			return fmt.Errorf("failed to create bare repo: %w", err)
		}
		fmt.Println("ok")
	}

	// 6. Create worker manager
	mgr, err := worker.NewManager(projectDir, cfg)
	if err != nil {
		return fmt.Errorf("failed to create worker manager: %w", err)
	}

	// 7. Create/start workers
	fmt.Print("Starting workers... ")
	workerNames, err := ensureWorkersRunning(mgr, cfg.Workers)
	if err != nil {
		fmt.Println("failed")
		return err
	}
	fmt.Printf("%d workers ready\n", len(workerNames))

	// 8. Prepare Claude environment in each worker
	fmt.Print("Preparing Claude environment... ")
	launcher, err := claude.NewLauncher(cfg, mgr)
	if err != nil {
		fmt.Println("failed")
		return fmt.Errorf("failed to create Claude launcher: %w", err)
	}

	for _, name := range workerNames {
		branchName := fmt.Sprintf("%s%s", cfg.Git.BranchPrefix, name)
		if err := launcher.PrepareWorker(name, branchName); err != nil {
			fmt.Println("failed")
			return fmt.Errorf("failed to prepare worker %s: %w", name, err)
		}
	}
	fmt.Println("ok")

	// 9. Save session state
	sessionState := &SessionState{
		Project:    cfg.Project,
		Workers:    workerNames,
		StartedAt:  time.Now(),
		AiryraPort: cfg.Airyra.Port,
		BaseBranch: cfg.Git.BaseBranch,
	}

	if !upNoZellij {
		sessionState.ZellijName = fmt.Sprintf("isollm-%s", cfg.Project)
	}

	if err := saveSessionState(projectDir, sessionState); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not save session state: %v\n", err)
	}

	// 10. Launch zellij
	if !upNoZellij {
		fmt.Print("Launching zellij... ")
		if err := launchZellij(cfg, workerNames, mgr); err != nil {
			fmt.Println("failed")
			return err
		}
		// launchZellij attaches to the session, so we only get here after detach
		fmt.Println("Session ended")
	} else {
		fmt.Println("\nWorkers ready. Skipping zellij (--no-zellij)")
		fmt.Printf("Workers: %v\n", workerNames)
		fmt.Printf("To attach later: zellij attach isollm-%s\n", cfg.Project)
	}

	return nil
}

// ensureAiryraRunning ensures the airyra server is running
func ensureAiryraRunning(ctx context.Context, cfg *config.Config) error {
	return airyra.EnsureRunning(ctx, cfg.Airyra.Host, cfg.Airyra.Port)
}

// ensureWorkersRunning creates/starts workers up to the desired count
// Returns the list of running worker names
func ensureWorkersRunning(mgr *worker.Manager, count int) ([]string, error) {
	// List existing workers
	workers, err := mgr.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list workers: %w", err)
	}

	var runningNames []string
	var stoppedNames []string

	// Categorize existing workers
	for _, w := range workers {
		if w.Status == "running" {
			runningNames = append(runningNames, w.Name)
		} else {
			stoppedNames = append(stoppedNames, w.Name)
		}
	}

	// Start stopped workers if we need more
	for _, name := range stoppedNames {
		if len(runningNames) >= count {
			break
		}
		if err := mgr.Start(name); err != nil {
			return nil, fmt.Errorf("failed to start worker %s: %w", name, err)
		}
		runningNames = append(runningNames, name)
	}

	// Create new workers if still not enough
	existingCount := len(workers)
	for len(runningNames) < count {
		// Create will auto-name as worker-N
		workerNum := existingCount + 1
		existingCount++
		name := fmt.Sprintf("%s%d", worker.WorkerPrefix, workerNum)

		if err := mgr.CreateWorker(name); err != nil {
			return nil, fmt.Errorf("failed to create worker: %w", err)
		}
		runningNames = append(runningNames, name)
	}

	// If we have more running than needed, that's ok - we don't stop them
	// Just return the first 'count' workers
	if len(runningNames) > count {
		runningNames = runningNames[:count]
	}

	return runningNames, nil
}

// launchZellij creates and attaches to a zellij session
func launchZellij(cfg *config.Config, workers []string, mgr *worker.Manager) error {
	// Create zellij manager
	zellijMgr, err := zellij.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create zellij manager: %w", err)
	}

	sessionName := fmt.Sprintf("isollm-%s", cfg.Project)

	// Check if session already exists
	exists, err := zellijMgr.SessionExists(sessionName)
	if err != nil {
		return fmt.Errorf("failed to check zellij session: %w", err)
	}

	if exists {
		// Session exists, just attach
		fmt.Println("attaching to existing session")
		return zellijMgr.AttachSession(sessionName)
	}

	// Build worker panes
	workerPanes := zellij.CreateWorkerPanes(workers)

	// Build dashboard config
	dashboard := zellij.DashboardConfig{
		Enabled:       cfg.Zellij.Dashboard,
		HeightPercent: zellij.DefaultDashboardHeight,
		Command:       "isollm",
		Args:          []string{"status", "--watch"},
	}

	// Parse layout mode
	layoutMode := zellij.LayoutMode(cfg.Zellij.Layout)
	if !layoutMode.IsValid() {
		layoutMode = zellij.LayoutModeAuto
	}

	// Build session config
	sessionCfg := zellij.BuildSessionConfig(
		sessionName,
		layoutMode,
		workerPanes,
		dashboard,
	)

	// Start session
	_, err = zellijMgr.StartSession(sessionCfg)
	if err != nil {
		return fmt.Errorf("failed to start zellij session: %w", err)
	}

	fmt.Println("ok")

	// Send commands to each worker pane to launch Claude
	launcher, err := claude.NewLauncher(cfg, mgr)
	if err != nil {
		return fmt.Errorf("failed to create Claude launcher: %w", err)
	}

	launchCmd := launcher.GetLaunchCommand()
	cmdStr := formatCommand(launchCmd)

	for _, name := range workers {
		// Send the launch command to the worker pane
		if err := zellijMgr.SendKeys(sessionName, name, cmdStr, "Enter"); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to send command to %s: %v\n", name, err)
		}
	}

	// Attach to the session
	return zellijMgr.AttachSession(sessionName)
}

// saveSessionState saves the session state to .isollm/session.json
func saveSessionState(projectDir string, state *SessionState) error {
	stateDir := filepath.Join(projectDir, config.StateDir)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	statePath := filepath.Join(stateDir, "session.json")
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session state: %w", err)
	}

	if err := os.WriteFile(statePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write session state: %w", err)
	}

	return nil
}

// loadSessionState loads the session state from .isollm/session.json
func loadSessionState(projectDir string) (*SessionState, error) {
	statePath := filepath.Join(projectDir, config.StateDir, "session.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read session state: %w", err)
	}

	var state SessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session state: %w", err)
	}

	return &state, nil
}

// formatCommand formats a command slice as a single shell command string
func formatCommand(cmd []string) string {
	if len(cmd) == 0 {
		return ""
	}
	if len(cmd) == 1 {
		return cmd[0]
	}

	// Simple join for now - could add shell escaping if needed
	result := cmd[0]
	for _, arg := range cmd[1:] {
		// Quote args with spaces
		if containsSpace(arg) {
			result += fmt.Sprintf(" %q", arg)
		} else {
			result += " " + arg
		}
	}
	return result
}

// containsSpace checks if a string contains whitespace
func containsSpace(s string) bool {
	for _, c := range s {
		if c == ' ' || c == '\t' || c == '\n' {
			return true
		}
	}
	return false
}
