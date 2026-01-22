# Finalising Implementation Plan

## Overview

This document provides an in-depth implementation plan for the remaining features needed to complete isollm. The plan follows existing codebase patterns and prioritizes implementation order based on dependencies.

**Implementation Order:**
1. Zellij Integration (foundation for `up`)
2. Claude Integration (worker environment setup)
3. `isollm up` (orchestration - ties everything together)
4. `isollm status` (dashboard - read-only, safer)
5. `isollm down` (completes session lifecycle)

---

## Current Status

### Implemented Commands
- [x] `isollm init` - Create isollm.yaml, .isollm/
- [x] `isollm config show` - Print config
- [x] `isollm config edit` - Open in $EDITOR
- [x] `isollm task add <title>` - Add to queue
- [x] `isollm task list` - Show tasks by status
- [x] `isollm task clear` - Remove completed
- [x] `isollm worker add [name]` - Add container
- [x] `isollm worker list` - Show workers
- [x] `isollm worker shell <name>` - lxc exec
- [x] `isollm worker reset <name>` - Reset to snapshot
- [x] `isollm worker remove <name>` - Delete container
- [x] `isollm sync status` - Branch state
- [x] `isollm sync pull` - Fetch task branches to host
- [x] `isollm sync push` - Push host to bare repo

### Commands To Implement
- [ ] `isollm up` - Start session (containers + zellij + airyra)
- [ ] `isollm down` - Stop session, salvage prompts
- [ ] `isollm status` - Dashboard view

### Implemented Packages
- `internal/config/` - Configuration loading and validation
- `internal/airyra/` - Task queue client wrapper
- `internal/worker/` - Container management via lxc-dev-manager
- `internal/barerepo/` - Bare repository operations
- `internal/git/` - Git command execution
- `internal/state/` - Session and worker state types

### Packages To Implement
- `internal/zellij/` - Terminal multiplexer integration
- `internal/claude/` - AI environment setup in containers
- `internal/status/` - Status collection and display
- `internal/session/` - Session lifecycle (shutdown)

---

## Phase 1: Zellij Integration

**Package Location:** `internal/zellij/`

Zellij is a terminal multiplexer that will display worker panes and a dashboard. We need to generate KDL layout files dynamically and manage zellij sessions.

### 1.1 Types (`internal/zellij/types.go`)

```go
package zellij

// LayoutMode determines how worker panes are arranged
type LayoutMode string

const (
    LayoutAuto       LayoutMode = "auto"       // Choose based on worker count
    LayoutHorizontal LayoutMode = "horizontal" // Side by side
    LayoutVertical   LayoutMode = "vertical"   // Stacked
    LayoutGrid       LayoutMode = "grid"       // Grid arrangement
)

// PaneConfig configures a single pane
type PaneConfig struct {
    Name         string   // Pane name (displayed in tab)
    Command      string   // Command to run (optional)
    Args         []string // Command arguments
    Cwd          string   // Working directory
    Focus        bool     // Whether this pane has focus
    SizePercent  int      // Size as percentage (0 = auto)
    BorderLess   bool     // Whether to hide borders
    SplitDirection string // "horizontal" or "vertical" for nested splits
}

// DashboardConfig configures the status dashboard pane
type DashboardConfig struct {
    Enabled     bool   // Whether to show dashboard
    SizePercent int    // Height percentage (default: 15)
    Command     string // Command to run for status updates
}

// SessionConfig configures a complete zellij session
type SessionConfig struct {
    Name        string          // Session name (project name)
    Layout      LayoutMode      // How to arrange panes
    Dashboard   DashboardConfig // Dashboard pane config
    WorkerPanes []PaneConfig    // Worker pane configs
}

// Session represents a running zellij session
type Session struct {
    Name    string
    Running bool
    Panes   []string // Pane names
}
```

### 1.2 Layout Calculator (`internal/zellij/layout.go`)

**Responsibilities:**
- Determine optimal layout based on worker count and mode
- Calculate pane sizes and positions
- Handle edge cases (1 worker, many workers, etc.)

```go
package zellij

// CalculateLayout determines the optimal layout for given workers
func CalculateLayout(workerCount int, mode LayoutMode) LayoutMode {
    if mode != LayoutAuto {
        return mode
    }
    // Auto-select based on worker count
    switch {
    case workerCount <= 2:
        return LayoutHorizontal
    case workerCount <= 4:
        return LayoutGrid
    default:
        return LayoutGrid
    }
}

// CalculatePaneSizes returns size percentages for each pane
func CalculatePaneSizes(paneCount int, layout LayoutMode, dashboardSize int) []int

// GridDimensions returns (rows, cols) for grid layout
func GridDimensions(workerCount int) (int, int)
```

**Layout Logic:**
- 1 worker: horizontal layout with single pane
- 2 workers: horizontal, each 50%
- 3 workers: grid 2x2 with one empty
- 4 workers: grid 2x2
- 6 workers: grid 2x3
- Dashboard enabled: subtracts dashboard height first

### 1.3 KDL Generator (`internal/zellij/kdl.go`)

**Responsibilities:**
- Generate valid KDL layout files
- Handle dashboard + worker pane arrangement
- Support all layout modes

```go
package zellij

// GenerateKDL creates a KDL layout string from config
func GenerateKDL(cfg SessionConfig) (string, error)

// GenerateGridKDL creates nested pane structure for grid
func GenerateGridKDL(panes []PaneConfig, rows, cols int) string

// WriteLayoutFile writes KDL to temp file, returns path
func WriteLayoutFile(kdl string) (string, error)
```

**KDL Output Example (3 workers + dashboard):**
```kdl
layout {
    pane size="15%" {
        command "watch"
        args ["-n", "2", "isollm", "status", "--brief"]
        name "dashboard"
    }
    pane split_direction="vertical" {
        pane split_direction="horizontal" {
            pane {
                command "lxc"
                args ["exec", "myproj-worker-1", "--", "su", "-l", "dev"]
                name "worker-1"
            }
            pane {
                command "lxc"
                args ["exec", "myproj-worker-2", "--", "su", "-l", "dev"]
                name "worker-2"
            }
        }
        pane split_direction="horizontal" {
            pane {
                command "lxc"
                args ["exec", "myproj-worker-3", "--", "su", "-l", "dev"]
                name "worker-3"
            }
        }
    }
}
```

### 1.4 Zellij Executor (`internal/zellij/executor.go`)

**Responsibilities:**
- Wrap zellij CLI commands
- Abstract command execution for testing

```go
package zellij

// Executor runs zellij commands
type Executor interface {
    ListSessions() ([]string, error)
    SessionExists(name string) bool
    CreateSession(name string, layoutFile string) error
    AttachSession(name string) error
    KillSession(name string) error
    SendKeys(session, pane string, keys string) error
}

var DefaultExecutor Executor = &realExecutor{}
```

### 1.5 Session Manager (`internal/zellij/session.go`)

**Responsibilities:**
- High-level session lifecycle
- Integrate layout generation with execution

```go
package zellij

// Manager handles zellij session lifecycle
type Manager struct {
    executor   Executor
    layoutsDir string // ~/.isollm/layouts/
}

func NewManager() (*Manager, error)
func (m *Manager) StartSession(cfg SessionConfig) error
func (m *Manager) StopSession(name string) error
func (m *Manager) AttachSession(name string) error
func (m *Manager) SessionExists(name string) bool
func (m *Manager) GetSession(name string) (*Session, error)
```

### 1.6 Files to Create

| File | Lines (est.) | Purpose |
|------|-------------|---------|
| `internal/zellij/types.go` | ~80 | Type definitions |
| `internal/zellij/layout.go` | ~150 | Layout calculations |
| `internal/zellij/kdl.go` | ~200 | KDL generation |
| `internal/zellij/executor.go` | ~100 | CLI wrapper |
| `internal/zellij/session.go` | ~120 | Session lifecycle |
| `internal/zellij/session_test.go` | ~200 | Tests |

---

## Phase 2: Claude Integration

**Package Location:** `internal/claude/`

Claude runs inside each worker container. We need to set up the environment and provide context via CLAUDE.md.

### 2.1 Types (`internal/claude/types.go`)

```go
package claude

// Environment contains all env vars needed for Claude in container
type Environment struct {
    AiryraHost    string // AIRYRA_HOST - host machine IP
    AiryraPort    int    // AIRYRA_PORT - airyra port
    AiryraProject string // AIRYRA_PROJECT - project name
    AiryraAgent   string // AIRYRA_AGENT - worker name as agent ID
    ProjectPath   string // PROJECT_PATH - /home/dev/project
    BareRepoPath  string // BARE_REPO_PATH - /repo.git
}

// Context contains information for CLAUDE.md generation
type Context struct {
    ProjectName   string
    WorkerName    string
    TaskBranch    string   // Current branch format: isollm/<task-id>
    BaseBranch    string   // Branch to fork from (e.g., main)
    AiryraHost    string
    AiryraPort    int
    CustomContext string   // Optional project-specific context
}

// LaunchConfig configures Claude launch in a worker
type LaunchConfig struct {
    Command     string
    Args        []string
    WorkDir     string
    Environment Environment
}
```

### 2.2 Environment Setup (`internal/claude/env.go`)

```go
package claude

// BuildEnvironment creates environment vars for a worker
func BuildEnvironment(hostIP string, airyraPort int, project string, workerName string) Environment

// ToEnvVars converts Environment to KEY=VALUE slice
func (e Environment) ToEnvVars() []string

// GetHostIP returns the host IP accessible from containers (lxdbr0 bridge)
func GetHostIP() (string, error)
```

### 2.3 CLAUDE.md Generator (`internal/claude/context.go`)

Generates CLAUDE.md with task workflow instructions for Claude in each container:

```go
package claude

// GenerateCLAUDEMD creates CLAUDE.md content for a worker
func GenerateCLAUDEMD(ctx Context) (string, error)
```

**Generated CLAUDE.md Content:**
- Task claiming workflow (airyra commands)
- Branch creation and management
- Git push/commit workflow
- Environment information
- Project-specific context (optional)

### 2.4 Worker Launcher (`internal/claude/launcher.go`)

```go
package claude

// Launcher prepares workers to run Claude
type Launcher struct {
    lxc       *lxcmgr.Client
    cfg       *config.Config
    hostIP    string
}

func NewLauncher(lxc *lxcmgr.Client, cfg *config.Config) (*Launcher, error)

// PrepareWorker sets up Claude environment in a worker:
// 1. Build environment variables
// 2. Write /home/dev/.isollm-env file
// 3. Generate and write CLAUDE.md to /home/dev/project/
// 4. Add source of env file to .bashrc
func (l *Launcher) PrepareWorker(workerName string) error

// GetLaunchCommand returns the command to run Claude
func (l *Launcher) GetLaunchCommand(workerName string) (string, []string)
```

### 2.5 Files to Create

| File | Lines (est.) | Purpose |
|------|-------------|---------|
| `internal/claude/types.go` | ~50 | Type definitions |
| `internal/claude/env.go` | ~80 | Environment building |
| `internal/claude/context.go` | ~150 | CLAUDE.md generation |
| `internal/claude/launcher.go` | ~120 | Worker preparation |
| `internal/claude/launcher_test.go` | ~150 | Tests |

---

## Phase 3: `isollm up` Command

**File:** `cmd/up.go`

This is the main orchestration command that brings everything together.

### 3.1 Command Flags

```go
var (
    upWorkers    int    // -n, --workers: override worker count
    upBaseBranch string // --base: override base branch
    upForce      bool   // --force: start even with stale repo
    upNoZellij   bool   // --no-zellij: skip zellij launch
)
```

### 3.2 Orchestration Steps

```go
func runUp(cmd *cobra.Command, args []string) error {
    // 1. Load and validate config
    // 2. Apply flag overrides (workers, base branch)
    // 3. Check for stale repo (host commits not in bare repo)
    //    - If stale and not --force: error with instructions
    //    - If stale and --force: warn and continue
    // 4. Start airyra server (if not running)
    // 5. Create bare repo (if first run)
    // 6. Create worker manager
    // 7. Create/start workers up to configured count
    // 8. Prepare Claude environment in each worker
    // 9. Save session state to .isollm/session.json
    // 10. Launch zellij (unless --no-zellij)
}
```

### 3.3 Helper Functions

```go
// ensureAiryraRunning starts airyra if not already running
func ensureAiryraRunning(cfg *config.Config) error

// ensureWorkersRunning creates workers if needed, starts stopped ones
func ensureWorkersRunning(mgr *worker.Manager, count int) ([]string, error)

// launchZellij creates and attaches to zellij session
func launchZellij(cfg *config.Config, workers []string, mgr *worker.Manager) error
```

### 3.4 Airyra Server Management (`internal/airyra/server.go`)

```go
package airyra

// StartServer attempts to start the airyra server
// Options:
// 1. Use SDK if it provides server management
// 2. Shell out to `airyra server start --background`
// 3. Error with instructions for manual start
func StartServer(project string, port int) error

// WaitForServer polls until server is ready or timeout
func WaitForServer(ctx context.Context, host string, port int) error
```

### 3.5 Files to Create/Modify

| File | Lines (est.) | Purpose |
|------|-------------|---------|
| `cmd/up.go` | ~300 | Main orchestration command |
| `internal/airyra/server.go` | ~80 | Server management |
| Modify `internal/worker/manager.go` | +20 | Expose LXCClient method |

---

## Phase 4: `isollm status` Command

**File:** `cmd/status.go`

Read-only dashboard showing workers, tasks, sync state, and services.

### 4.1 Command Flags

```go
var (
    statusBrief bool // --brief: one-line summary
    statusJSON  bool // --json: machine-readable output
)
```

### 4.2 Status Types (`internal/status/types.go`)

```go
package status

type Status struct {
    Project       string            `json:"project"`
    SessionActive bool              `json:"session_active"`
    Workers       []WorkerStatus    `json:"workers"`
    Tasks         TaskSummary       `json:"tasks"`
    Sync          SyncStatus        `json:"sync"`
    Services      ServiceStatus     `json:"services"`
    Timestamp     time.Time         `json:"timestamp"`
}

type WorkerStatus struct {
    Name        string    `json:"name"`
    Status      string    `json:"status"`
    IP          string    `json:"ip"`
    TaskID      string    `json:"task_id,omitempty"`
    TaskTitle   string    `json:"task_title,omitempty"`
    TaskBranch  string    `json:"task_branch,omitempty"`
    Duration    string    `json:"duration,omitempty"`
}

type TaskSummary struct {
    Ready       int `json:"ready"`
    InProgress  int `json:"in_progress"`
    Blocked     int `json:"blocked"`
    Completed   int `json:"completed"`
}

type SyncStatus struct {
    HostBranch      string        `json:"host_branch"`
    HostCommit      string        `json:"host_commit"`
    HostAhead       int           `json:"host_ahead"`
    TaskBranches    []BranchInfo  `json:"task_branches"`
    TotalBranches   int           `json:"total_branches"`
}

type ServiceStatus struct {
    Airyra AiryraStatus `json:"airyra"`
    Zellij ZellijStatus `json:"zellij"`
}
```

### 4.3 Status Collector (`internal/status/collector.go`)

```go
package status

type Collector struct {
    projectDir string
    cfg        *config.Config
}

func NewCollector(projectDir string, cfg *config.Config) *Collector

// Collect gathers all status information in parallel
func (c *Collector) Collect(ctx context.Context) (*Status, error)

// Individual collectors (run in parallel via goroutines)
func (c *Collector) collectWorkers() []WorkerStatus
func (c *Collector) collectTasks(ctx context.Context) TaskSummary
func (c *Collector) collectSync() SyncStatus
func (c *Collector) collectServices(ctx context.Context) ServiceStatus
```

### 4.4 Display Functions

```go
// Full dashboard output
func printStatusFull(st *Status) error

// One-line summary: "workers: 3 running | tasks: 5 ready, 2 in-progress | airyra: ● zellij: ●"
func printStatusBrief(st *Status) error

// JSON output
func printStatusJSON(st *Status) error
```

### 4.5 Full Dashboard Output Example

```
isollm: my-project
═══════════════════════════════════════════════════

Workers (3 running):
  worker-1  ● running   192.168.1.101
            └─ task: ar-a1b2 (12m) → isollm/ar-a1b2
  worker-2  ● running   192.168.1.102
            └─ task: ar-c3d4 (5m) → isollm/ar-c3d4
  worker-3  ● running   192.168.1.103
            └─ (idle, waiting for task)

Tasks:
  Ready:        5
  In Progress:  2
  Blocked:      1
  Completed:    12

Sync:
  Host: main (in sync)
  Task branches: 8

Airyra: ● running (localhost:7432)
Zellij: ● attached (session: my-project)
```

### 4.6 Files to Create

| File | Lines (est.) | Purpose |
|------|-------------|---------|
| `cmd/status.go` | ~150 | Command and display |
| `internal/status/collector.go` | ~250 | Status gathering |
| `internal/status/types.go` | ~80 | Type definitions |

---

## Phase 5: `isollm down` Command

**File:** `cmd/down.go`

Graceful shutdown with salvage prompts for uncommitted work.

### 5.1 Command Flags

```go
var (
    downDestroy bool // --destroy: remove containers
    downSave    bool // --save: snapshot before stopping
    downYes     bool // --yes: skip confirmations
)
```

### 5.2 Shutdown Options

```go
type ShutdownOptions struct {
    Destroy         bool          // Remove containers
    SaveSnapshots   bool          // Snapshot before stopping
    SkipConfirm     bool          // Skip confirmations
    ReleaseTasksTimeout time.Duration
}
```

### 5.3 Shutdown Steps (`internal/session/shutdown.go`)

```go
func (s *Shutdown) Execute(ctx context.Context, opts ShutdownOptions) error {
    // 1. Gather worker info (list all workers)
    // 2. Check for uncommitted/unpushed work in each worker
    // 3. Salvage prompt if work found (unless --yes)
    //    - [s] Salvage: push branches before stopping
    //    - [d] Discard: stop without saving
    //    - [c] Cancel
    // 4. Release claimed tasks back to airyra queue
    // 5. Stop zellij session
    // 6. Save snapshots if --save requested
    // 7. Destroy containers if --destroy requested (with confirmation)
    //    - Or just stop containers if not destroying
    // 8. Run GC on bare repo (safe now that workers stopped)
    // 9. Clear session state
}
```

### 5.4 Worker State Check

```go
type WorkerShutdownInfo struct {
    Name            string
    TaskID          string
    Branch          string
    UnpushedCommits int
    HasUncommitted  bool
}

func (s *Shutdown) checkWorkerState(w *worker.WorkerInfo) WorkerShutdownInfo
```

### 5.5 Salvage Prompt

```
Workers with unsaved work:
  worker-1 (branch isollm/ar-a1b2) - 3 unpushed commits
  worker-3 (branch isollm/ar-e5f6) - has uncommitted changes

Options:
  [s] Salvage - push branches before stopping
  [d] Discard - stop without saving
  [c] Cancel

Choice:
```

### 5.6 Destroy Confirmation

```
This will permanently delete these containers:
  worker-1
  worker-2
  worker-3

Type 'destroy' to confirm:
```

### 5.7 Files to Create

| File | Lines (est.) | Purpose |
|------|-------------|---------|
| `cmd/down.go` | ~80 | Command definition |
| `internal/session/shutdown.go` | ~300 | Shutdown logic |

---

## Phase 6: Minor Fixes

### 6.1 Rebuild Binary

The task commands exist in code but need binary rebuild:

```bash
go build -o isollm .
```

### 6.2 Expose LXCClient in Worker Manager

The `up` command needs access to the LXC client for Claude preparation:

```go
// Add to internal/worker/manager.go

// LXCClient returns the underlying LXC client for direct access
func (m *Manager) LXCClient() *lxcmgr.Client {
    return m.client
}
```

### 6.3 Add WriteFile Helper

Claude launcher needs to write files to containers:

```go
// Add to internal/worker/manager.go if not in lxc-dev-manager

func (m *Manager) WriteFile(workerName, path, content string) error {
    // Use lxc file push or exec with echo/cat
}
```

---

## Summary: Files and Estimated Lines

| Phase | Package | File | Lines (est.) |
|-------|---------|------|-------------|
| 1 | zellij | types.go | 80 |
| 1 | zellij | layout.go | 150 |
| 1 | zellij | kdl.go | 200 |
| 1 | zellij | executor.go | 100 |
| 1 | zellij | session.go | 120 |
| 1 | zellij | session_test.go | 200 |
| 2 | claude | types.go | 50 |
| 2 | claude | env.go | 80 |
| 2 | claude | context.go | 150 |
| 2 | claude | launcher.go | 120 |
| 2 | claude | launcher_test.go | 150 |
| 3 | cmd | up.go | 300 |
| 3 | airyra | server.go | 80 |
| 4 | cmd | status.go | 150 |
| 4 | status | collector.go | 250 |
| 4 | status | types.go | 80 |
| 5 | cmd | down.go | 80 |
| 5 | session | shutdown.go | 300 |
| 6 | worker | manager.go (mod) | 20 |
| **Total** | | | **~2660** |

---

## Implementation Order and Dependencies

```
Phase 1: Zellij Integration
    └── No dependencies, can start immediately

Phase 2: Claude Integration
    └── No dependencies, can run parallel to Phase 1

Phase 3: isollm up
    ├── Requires: Phase 1 (zellij)
    ├── Requires: Phase 2 (claude)
    └── Requires: Existing barerepo, worker, airyra packages

Phase 4: isollm status
    ├── Requires: Phase 1 (zellij - for session status)
    └── Can start after Phase 1, run parallel to Phase 3

Phase 5: isollm down
    ├── Requires: Phase 1 (zellij - to stop session)
    └── Requires: Phase 3 concepts (session state)

Phase 6: Minor fixes
    └── Can happen anytime
```

**Recommended Parallel Work:**
- Phase 1 + Phase 2 can be done in parallel
- Phase 4 can start once Phase 1 is complete
- Phase 5 can start once Phase 3 is complete

---

## Testing Strategy

### Unit Tests
- Layout calculation functions
- KDL generation (compare output strings)
- CLAUDE.md generation
- Status collection (with mocks)

### Integration Tests
- Create actual zellij layout files and verify syntax
- Test worker manager interactions
- Test airyra client with mock server

### Manual Testing
- Full `isollm up` → work → `isollm down` cycle
- Test salvage prompts
- Test various layout modes with different worker counts
- Test `--force` and `--yes` flags

---

## Risk Areas and Mitigations

| Risk | Mitigation |
|------|------------|
| Zellij KDL syntax changes | Pin zellij version, document requirements |
| LXC UID mapping issues | Test on real LXC setup early |
| Airyra server management | Start simple (require manual start), add automation later |
| Container file writing | Test WriteFile approach early |
| Grid layout complexity | Start with simpler horizontal/vertical, add grid last |

---

## Codebase Patterns to Follow

### Command Pattern (Cobra)
```go
var myCmd = &cobra.Command{
    Use:   "my-command",
    Short: "Brief help",
    Long:  "Detailed help with examples",
    RunE:  runMyCommand,  // Always use RunE for error handling
}

func init() {
    myCmd.Flags().StringVar(&myFlag, "flag", "default", "help")
    rootCmd.AddCommand(myCmd)
}
```

### Error Handling
```go
// Wrap errors with context
if err != nil {
    return fmt.Errorf("operation description: %w", err)
}

// For SDK errors, use FormatError()
if err := client.AddTask(ctx, title); err != nil {
    return fmt.Errorf("failed to add task: %s", airyra.FormatError(err))
}
```

### Configuration Access
```go
dir, _ := os.Getwd()
projectDir, _ := config.FindProjectRoot(dir)
cfg, _ := config.Load(projectDir)
```

### Output Formatting
- Simple output: `fmt.Printf()` / `fmt.Println()`
- Tables: `text/tabwriter`
- YAML: `yaml.Marshal()`
- Symbols: Unicode (✓, ●, ○, →, └─)
