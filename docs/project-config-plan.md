# Project/Config Management Implementation Plan (v2)

This plan incorporates feedback from expert review. See `project-config-plan-review.md` for detailed rationale.

---

## Current State

The `internal/config/config.go` module already provides:
- ✓ Config struct with all subsections (Project, Workers, Image, Git, Claude, Airyra, Zellij, Ports)
- ✓ `Load()` - YAML parsing with error handling
- ✓ `Save()` - YAML serialization
- ✓ `DefaultConfig()` - Sensible defaults for new projects
- ✓ `applyDefaults()` - Fills missing values post-load
- ✓ `FindProjectRoot()` - Walks up directory tree
- ✓ `CreateStateDir()` - Creates `.isollm/` directory
- ✓ `Exists()` - Checks for isollm.yaml

---

## What Needs to Be Added

### 1. Schema Validation (`internal/config/validate.go`)

Add validation beyond YAML parsing to catch configuration errors early.

#### Constants

```go
package config

const (
    // Worker limits
    MinWorkers = 1
    MaxWorkers = 20  // Reasonable limit for local machine resources

    // Port limits
    MinUserPort = 1024   // Ports below 1024 require root
    MaxPort     = 65535

    // Project name limits
    MinProjectNameLen = 2
    MaxProjectNameLen = 64
)

var (
    validProjectName = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9-]*$`)
    validBranchName  = regexp.MustCompile(`^[a-zA-Z0-9._/-]+$`)
    validLayouts     = map[string]struct{}{
        "auto": {}, "horizontal": {}, "vertical": {}, "grid": {},
    }
)
```

#### ValidationError Type

```go
// ValidationError collects multiple validation failures
type ValidationError struct {
    Errors []string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("config validation failed:\n  - %s",
        strings.Join(e.Errors, "\n  - "))
}

func (e *ValidationError) Add(msg string) {
    e.Errors = append(e.Errors, msg)
}

func (e *ValidationError) HasErrors() bool {
    return len(e.Errors) > 0
}
```

#### Validations

| Field | Validation |
|-------|------------|
| `project` | Required, valid identifier, length 2-64 |
| `workers` | Must be >= MinWorkers and <= MaxWorkers |
| `image` | Required, non-empty. Warn if no tag. |
| `git.base_branch` | Required, valid git branch name |
| `git.branch_prefix` | Required, must end with `/`, not just `/` |
| `airyra.port` | Valid port range (MinUserPort-MaxPort) |
| `airyra.host` | Valid hostname or IP |
| `ports` | Valid format, no duplicates, no conflict with airyra.port |
| `zellij.layout` | Must be one of: auto, horizontal, vertical, grid |

#### Implementation

```go
// Validate checks the config for semantic errors
func (c *Config) Validate() error {
    errs := &ValidationError{}

    // Project name
    if c.Project == "" {
        errs.Add("project name is required")
    } else {
        if !validProjectName.MatchString(c.Project) {
            errs.Add("project name must be alphanumeric with hyphens, starting with a letter")
        }
        if len(c.Project) < MinProjectNameLen {
            errs.Add(fmt.Sprintf("project name must be at least %d characters", MinProjectNameLen))
        }
        if len(c.Project) > MaxProjectNameLen {
            errs.Add(fmt.Sprintf("project name cannot exceed %d characters", MaxProjectNameLen))
        }
    }

    // Workers
    if c.Workers < MinWorkers {
        errs.Add(fmt.Sprintf("workers must be at least %d", MinWorkers))
    } else if c.Workers > MaxWorkers {
        errs.Add(fmt.Sprintf("workers cannot exceed %d", MaxWorkers))
    }

    // Image
    if c.Image == "" {
        errs.Add("image is required")
    }
    // Note: Warn (not error) if no tag - handled separately

    // Git config
    if c.Git.BaseBranch == "" {
        errs.Add("git.base_branch is required")
    } else if !validBranchName.MatchString(c.Git.BaseBranch) {
        errs.Add("git.base_branch contains invalid characters")
    }

    if c.Git.BranchPrefix == "/" {
        errs.Add("git.branch_prefix cannot be just '/'")
    } else if !strings.HasSuffix(c.Git.BranchPrefix, "/") {
        errs.Add("git.branch_prefix must end with '/'")
    }

    // Airyra port
    if c.Airyra.Port < MinUserPort || c.Airyra.Port > MaxPort {
        errs.Add(fmt.Sprintf("airyra.port must be between %d and %d", MinUserPort, MaxPort))
    }

    // Ports: format, duplicates, and airyra conflict
    seen := make(map[string]bool)
    airyraPort := strconv.Itoa(c.Airyra.Port)
    for _, p := range c.Ports {
        if err := validatePort(p); err != nil {
            errs.Add(fmt.Sprintf("invalid port %q: %v", p, err))
            continue
        }
        hostPort := strings.Split(p, ":")[0]
        if seen[hostPort] {
            errs.Add(fmt.Sprintf("duplicate port: %s", hostPort))
        }
        seen[hostPort] = true
        if hostPort == airyraPort {
            errs.Add(fmt.Sprintf("port %s conflicts with airyra.port", hostPort))
        }
    }

    // Zellij layout
    if _, ok := validLayouts[c.Zellij.Layout]; !ok {
        errs.Add("zellij.layout must be one of: auto, horizontal, vertical, grid")
    }

    if errs.HasErrors() {
        return errs
    }
    return nil
}

func validatePort(p string) error {
    parts := strings.Split(p, ":")
    if len(parts) > 2 {
        return fmt.Errorf("invalid format, expected 'port' or 'host:container'")
    }
    for _, part := range parts {
        port, err := strconv.Atoi(part)
        if err != nil {
            return fmt.Errorf("not a valid number")
        }
        if port < 1 || port > MaxPort {
            return fmt.Errorf("port out of range (1-%d)", MaxPort)
        }
    }
    return nil
}

// Warnings returns non-fatal issues (call after Validate)
func (c *Config) Warnings() []string {
    var warnings []string
    if c.Image != "" && !strings.Contains(c.Image, ":") {
        warnings = append(warnings, "image has no tag, will use :latest")
    }
    return warnings
}
```

#### LoadAndValidate Convenience Function

```go
// LoadAndValidate reads and validates the config
func LoadAndValidate(dir string) (*Config, error) {
    cfg, err := Load(dir)
    if err != nil {
        return nil, err
    }
    if err := cfg.Validate(); err != nil {
        return nil, err
    }
    return cfg, nil
}
```

---

### 2. State Management (`internal/state/`)

New module to track runtime state of workers and sessions.

#### State Directory Structure

```
.isollm/
├── session.json       # Current session info (if running)
└── workers/           # Per-worker state (one file per worker)
    ├── worker-1.json
    ├── worker-2.json
    └── worker-3.json
```

#### State Interface (for testability)

```go
package state

import "errors"

// Sentinel errors
var (
    ErrNoSession     = errors.New("no active session")
    ErrSessionExists = errors.New("session already exists")
)

// State abstracts state persistence for testing
type State interface {
    // Session management
    CreateSession(s *Session) error           // Atomic create (fails if exists)
    SaveSession(s *Session) error             // Update existing session
    LoadSession() (*Session, error)           // Returns ErrNoSession if not found
    ClearSession() error                      // Delete session file
    HasActiveSession() (bool, error)          // Checks existence AND liveness

    // Worker state
    SaveWorker(w *WorkerState) error
    LoadWorker(name string) (*WorkerState, error)
    LoadAllWorkers() (*LoadResult, error)     // Partial results + errors
    DeleteWorker(name string) error
    ClearAllWorkers() error
}

// LoadResult contains workers and any load errors
type LoadResult struct {
    Workers []*WorkerState
    Errors  []error
}
```

#### Data Structures

```go
// Schema versions - increment when structure changes
const (
    CurrentSessionVersion = 1
    CurrentWorkerVersion  = 1
)

// SessionStatus represents session lifecycle
type SessionStatus string

const (
    SessionStatusInitializing SessionStatus = "initializing"  // isollm up in progress
    SessionStatusRunning      SessionStatus = "running"       // Fully started
    SessionStatusShuttingDown SessionStatus = "shutting_down" // isollm down in progress
)

// Session represents a running isollm session
type Session struct {
    Version       int           `json:"version"`
    Status        SessionStatus `json:"status"`
    StartedAt     time.Time     `json:"started_at"`
    PID           int           `json:"pid"`              // Orchestrator process ID
    ProjectRoot   string        `json:"project_root"`
    BareRepoPath  string        `json:"bare_repo_path"`
    ZellijSession string        `json:"zellij_session"`
}

// WorkerStatus represents worker lifecycle states
type WorkerStatus string

const (
    WorkerStatusCreating WorkerStatus = "creating"  // Container being built
    WorkerStatusIdle     WorkerStatus = "idle"      // Running, no task
    WorkerStatusBusy     WorkerStatus = "busy"      // Running, has task
    WorkerStatusStopping WorkerStatus = "stopping"  // Graceful shutdown in progress
    WorkerStatusStopped  WorkerStatus = "stopped"   // Container stopped
    WorkerStatusError    WorkerStatus = "error"     // Unrecoverable error
)

// WorkerState represents the state of a single worker
type WorkerState struct {
    Version       int          `json:"version"`
    Name          string       `json:"name"`
    ContainerID   string       `json:"container_id,omitempty"`
    Status        WorkerStatus `json:"status"`
    IP            string       `json:"ip,omitempty"`
    CurrentTask   *TaskRef     `json:"current_task,omitempty"`
    CurrentBranch string       `json:"current_branch,omitempty"`
    StartedAt     time.Time    `json:"started_at"`
    LastActivity  time.Time    `json:"last_activity"`
    LastError     string       `json:"last_error,omitempty"`
    ErrorTime     time.Time    `json:"error_time,omitempty"`
}

// TaskRef is a cache of task info (airyra is source of truth)
type TaskRef struct {
    ID        string    `json:"id"`
    Title     string    `json:"title"`
    ClaimedAt time.Time `json:"claimed_at"`
}
```

#### FileState Implementation

```go
// FileState implements State using the filesystem
type FileState struct {
    stateDir string
}

// Compile-time interface check
var _ State = (*FileState)(nil)

// New creates a FileState for the project's .isollm directory
func New(projectRoot string) *FileState {
    return &FileState{
        stateDir: filepath.Join(projectRoot, ".isollm"),
    }
}

// NewWithDir creates a FileState with custom directory (for testing)
func NewWithDir(stateDir string) *FileState {
    return &FileState{stateDir: stateDir}
}
```

#### Session Methods

```go
// CreateSession atomically creates a new session (fails if exists)
func (m *FileState) CreateSession(s *Session) error {
    if err := os.MkdirAll(m.stateDir, 0755); err != nil {
        return fmt.Errorf("failed to create state dir: %w", err)
    }

    fullPath := filepath.Join(m.stateDir, "session.json")
    s.Version = CurrentSessionVersion

    data, err := json.MarshalIndent(s, "", "  ")
    if err != nil {
        return fmt.Errorf("failed to marshal session: %w", err)
    }

    // O_EXCL = fail if file exists (atomic check-and-create)
    f, err := os.OpenFile(fullPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
    if err != nil {
        if os.IsExist(err) {
            return ErrSessionExists
        }
        return fmt.Errorf("failed to create session file: %w", err)
    }
    defer f.Close()

    if _, err := f.Write(data); err != nil {
        os.Remove(fullPath)
        return fmt.Errorf("failed to write session: %w", err)
    }
    return nil
}

// SaveSession updates an existing session (atomic write)
func (m *FileState) SaveSession(s *Session) error {
    s.Version = CurrentSessionVersion
    return m.saveJSON("session.json", s)
}

// LoadSession loads the session, returns ErrNoSession if not found
func (m *FileState) LoadSession() (*Session, error) {
    var s Session
    if err := m.loadJSON("session.json", &s); err != nil {
        if os.IsNotExist(err) {
            return nil, ErrNoSession
        }
        return nil, fmt.Errorf("failed to load session: %w", err)
    }

    // Migrate old versions if needed
    if s.Version < CurrentSessionVersion {
        // Future migrations go here
    }

    return &s, nil
}

// ClearSession removes the session file
func (m *FileState) ClearSession() error {
    fullPath := filepath.Join(m.stateDir, "session.json")
    err := os.Remove(fullPath)
    if os.IsNotExist(err) {
        return nil // Already gone
    }
    return err
}

// HasActiveSession checks if session exists AND process is alive
func (m *FileState) HasActiveSession() (bool, error) {
    session, err := m.LoadSession()
    if errors.Is(err, ErrNoSession) {
        return false, nil
    }
    if err != nil {
        return false, err
    }

    // Check if orchestrator process is still running
    if !isProcessAlive(session.PID) {
        return false, nil // Stale session
    }

    return true, nil
}

func isProcessAlive(pid int) bool {
    if pid <= 0 {
        return false
    }
    process, err := os.FindProcess(pid)
    if err != nil {
        return false
    }
    // Signal 0 doesn't kill - just checks if process exists
    err = process.Signal(syscall.Signal(0))
    return err == nil
}
```

#### Worker Methods

```go
// SaveWorker saves worker state (atomic write)
func (m *FileState) SaveWorker(w *WorkerState) error {
    w.Version = CurrentWorkerVersion
    workersDir := filepath.Join(m.stateDir, "workers")
    if err := os.MkdirAll(workersDir, 0755); err != nil {
        return fmt.Errorf("failed to create workers dir: %w", err)
    }
    return m.saveJSON(filepath.Join("workers", w.Name+".json"), w)
}

// LoadWorker loads a single worker's state
func (m *FileState) LoadWorker(name string) (*WorkerState, error) {
    var w WorkerState
    if err := m.loadJSON(filepath.Join("workers", name+".json"), &w); err != nil {
        if os.IsNotExist(err) {
            return nil, fmt.Errorf("worker %s not found", name)
        }
        return nil, fmt.Errorf("failed to load worker %s: %w", name, err)
    }
    return &w, nil
}

// LoadAllWorkers loads all workers, returning partial results on errors
func (m *FileState) LoadAllWorkers() (*LoadResult, error) {
    result := &LoadResult{}

    workersDir := filepath.Join(m.stateDir, "workers")
    entries, err := os.ReadDir(workersDir)
    if err != nil {
        if os.IsNotExist(err) {
            return result, nil // No workers yet
        }
        return nil, fmt.Errorf("failed to read workers dir: %w", err)
    }

    for _, e := range entries {
        if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
            continue
        }
        var w WorkerState
        if err := m.loadJSON(filepath.Join("workers", e.Name()), &w); err != nil {
            result.Errors = append(result.Errors,
                fmt.Errorf("failed to load %s: %w", e.Name(), err))
            continue
        }
        result.Workers = append(result.Workers, &w)
    }
    return result, nil
}

// DeleteWorker removes a worker's state file
func (m *FileState) DeleteWorker(name string) error {
    fullPath := filepath.Join(m.stateDir, "workers", name+".json")
    err := os.Remove(fullPath)
    if os.IsNotExist(err) {
        return nil
    }
    return err
}

// ClearAllWorkers removes all worker state files
func (m *FileState) ClearAllWorkers() error {
    workersDir := filepath.Join(m.stateDir, "workers")
    err := os.RemoveAll(workersDir)
    if os.IsNotExist(err) {
        return nil
    }
    return err
}
```

#### Atomic File Helpers

```go
// saveJSON writes JSON atomically using temp file + rename
func (m *FileState) saveJSON(relPath string, v interface{}) error {
    fullPath := filepath.Join(m.stateDir, relPath)

    // Ensure parent directory exists
    if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
        return fmt.Errorf("failed to create directory: %w", err)
    }

    data, err := json.MarshalIndent(v, "", "  ")
    if err != nil {
        return fmt.Errorf("failed to marshal: %w", err)
    }

    // Write to temp file first
    tmpPath := fullPath + ".tmp"
    if err := os.WriteFile(tmpPath, data, 0644); err != nil {
        return fmt.Errorf("failed to write temp file: %w", err)
    }

    // Atomic rename
    if err := os.Rename(tmpPath, fullPath); err != nil {
        os.Remove(tmpPath)
        return fmt.Errorf("failed to rename: %w", err)
    }

    return nil
}

// loadJSON reads and unmarshals JSON
func (m *FileState) loadJSON(relPath string, v interface{}) error {
    fullPath := filepath.Join(m.stateDir, relPath)
    data, err := os.ReadFile(fullPath)
    if err != nil {
        return err
    }
    if err := json.Unmarshal(data, v); err != nil {
        return fmt.Errorf("invalid JSON: %w", err)
    }
    return nil
}
```

---

### 3. Config Commands (`cmd/config.go`)

Add `isollm config show` and `isollm config edit` commands.

```go
package cmd

import (
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "strings"

    "github.com/spf13/cobra"
    "gopkg.in/yaml.v3"

    "isollm/internal/config"
)

var configCmd = &cobra.Command{
    Use:   "config",
    Short: "Configuration management",
    Long:  "View and edit isollm project configuration.",
}

var configShowCmd = &cobra.Command{
    Use:   "show",
    Short: "Show current configuration",
    Long:  "Display the current isollm.yaml configuration with resolved defaults.",
    RunE:  runConfigShow,
}

var configEditCmd = &cobra.Command{
    Use:   "edit",
    Short: "Edit configuration in $EDITOR",
    Long:  "Open isollm.yaml in your default editor.",
    RunE:  runConfigEdit,
}

func init() {
    configCmd.AddCommand(configShowCmd)
    configCmd.AddCommand(configEditCmd)
    rootCmd.AddCommand(configCmd)
}

func runConfigShow(cmd *cobra.Command, args []string) error {
    cwd, err := os.Getwd()
    if err != nil {
        return err
    }

    projectRoot, err := config.FindProjectRoot(cwd)
    if err != nil {
        return err
    }

    cfg, err := config.Load(projectRoot)
    if err != nil {
        return err
    }

    // Validate and show warnings
    if err := cfg.Validate(); err != nil {
        fmt.Fprintf(os.Stderr, "Errors:\n%v\n\n", err)
    }
    for _, w := range cfg.Warnings() {
        fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
    }

    // Pretty print as YAML
    data, err := yaml.Marshal(cfg)
    if err != nil {
        return err
    }

    fmt.Printf("# Configuration: %s/isollm.yaml\n", projectRoot)
    fmt.Printf("# (defaults applied for missing values)\n\n")
    fmt.Print(string(data))

    return nil
}

func runConfigEdit(cmd *cobra.Command, args []string) error {
    cwd, err := os.Getwd()
    if err != nil {
        return err
    }

    projectRoot, err := config.FindProjectRoot(cwd)
    if err != nil {
        return err
    }

    configPath := filepath.Join(projectRoot, config.ConfigFileName)

    // Determine editor
    editor := os.Getenv("EDITOR")
    if editor == "" {
        editor = os.Getenv("VISUAL")
    }
    if editor == "" {
        editor = "vi"
    }

    // Open editor
    editorCmd := exec.Command(editor, configPath)
    editorCmd.Stdin = os.Stdin
    editorCmd.Stdout = os.Stdout
    editorCmd.Stderr = os.Stderr

    if err := editorCmd.Run(); err != nil {
        return fmt.Errorf("editor failed: %w", err)
    }

    // Validate after edit
    cfg, err := config.Load(projectRoot)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: config has syntax errors: %v\n", err)
        fmt.Fprintf(os.Stderr, "Run 'isollm config edit' to fix.\n")
        return err
    }

    if err := cfg.Validate(); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        fmt.Fprintf(os.Stderr, "Run 'isollm config edit' to fix.\n")
        return err
    }

    for _, w := range cfg.Warnings() {
        fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
    }

    fmt.Println("Configuration saved and validated.")
    return nil
}
```

---

### 4. Update Init Command

Update `cmd/init.go` to add `.isollm/` to `.gitignore`:

```go
// Add to runInit function, after creating config:

// Ensure .isollm/ is in .gitignore
if err := appendToGitignore(dir, ".isollm/"); err != nil {
    fmt.Fprintf(os.Stderr, "Warning: could not update .gitignore: %v\n", err)
}

// Helper function
func appendToGitignore(dir, entry string) error {
    gitignorePath := filepath.Join(dir, ".gitignore")

    // Read existing content
    content, err := os.ReadFile(gitignorePath)
    if err != nil && !os.IsNotExist(err) {
        return err
    }

    // Check if already present
    lines := strings.Split(string(content), "\n")
    for _, line := range lines {
        if strings.TrimSpace(line) == entry {
            return nil // Already there
        }
    }

    // Append
    f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return err
    }
    defer f.Close()

    // Add newline if file doesn't end with one
    if len(content) > 0 && content[len(content)-1] != '\n' {
        f.WriteString("\n")
    }
    _, err = f.WriteString(entry + "\n")
    return err
}
```

---

## File Structure After Implementation

```
internal/
├── config/
│   ├── config.go        # Existing (unchanged)
│   ├── validate.go      # NEW: Validation logic + ValidationError type
│   └── validate_test.go # NEW: Validation tests
├── state/
│   ├── state.go         # NEW: State interface + FileState implementation
│   ├── types.go         # NEW: Session, WorkerState, etc.
│   └── state_test.go    # NEW: State tests
├── barerepo/
│   └── ...
└── git/
    └── ...

cmd/
├── root.go
├── init.go              # MODIFIED: Add .gitignore handling
├── sync.go
└── config.go            # NEW: config show/edit commands
```

---

## Implementation Order

1. **`internal/config/validate.go`** - Schema validation
   - Add constants (MinWorkers, MaxWorkers, etc.)
   - Add ValidationError type
   - Add Validate() method with all checks
   - Add Warnings() method
   - Add LoadAndValidate() convenience function
   - Write tests

2. **`internal/state/types.go`** - State types
   - Define Session, WorkerState, TaskRef structs
   - Define status constants
   - Define sentinel errors

3. **`internal/state/state.go`** - State manager
   - Define State interface
   - Implement FileState with atomic writes
   - Implement all session/worker methods
   - Write tests

4. **`cmd/config.go`** - Config commands
   - `config show` command
   - `config edit` command
   - Wire into root command

5. **`cmd/init.go`** - Update init
   - Add appendToGitignore helper
   - Call it after creating config

---

## Testing Plan

### Validation Tests (`validate_test.go`)

```go
func TestValidate_ValidConfig(t *testing.T)
func TestValidate_EmptyProject(t *testing.T)
func TestValidate_ProjectNameTooShort(t *testing.T)
func TestValidate_ProjectNameTooLong(t *testing.T)
func TestValidate_InvalidProjectName(t *testing.T)
func TestValidate_WorkersBelowMin(t *testing.T)
func TestValidate_WorkersAboveMax(t *testing.T)
func TestValidate_InvalidBranchPrefix(t *testing.T)
func TestValidate_BranchPrefixJustSlash(t *testing.T)
func TestValidate_InvalidPort(t *testing.T)
func TestValidate_DuplicatePorts(t *testing.T)
func TestValidate_PortConflictWithAiryra(t *testing.T)
func TestValidate_InvalidLayout(t *testing.T)
func TestValidate_MultipleErrors(t *testing.T)
func TestWarnings_ImageNoTag(t *testing.T)
```

### State Tests (`state_test.go`)

```go
func TestFileState_CreateSession(t *testing.T)
func TestFileState_CreateSession_AlreadyExists(t *testing.T)
func TestFileState_SaveLoadSession(t *testing.T)
func TestFileState_LoadSession_NotFound(t *testing.T)
func TestFileState_ClearSession(t *testing.T)
func TestFileState_HasActiveSession_NoSession(t *testing.T)
func TestFileState_HasActiveSession_StaleSession(t *testing.T)
func TestFileState_HasActiveSession_LiveSession(t *testing.T)
func TestFileState_SaveLoadWorker(t *testing.T)
func TestFileState_LoadAllWorkers(t *testing.T)
func TestFileState_LoadAllWorkers_PartialFailure(t *testing.T)
func TestFileState_DeleteWorker(t *testing.T)
func TestFileState_ClearAllWorkers(t *testing.T)
func TestFileState_AtomicWrite(t *testing.T)
```

---

## Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| State interface | Enables unit testing without filesystem |
| Atomic writes (temp + rename) | Prevents corrupted reads from concurrent access |
| Sentinel errors | Idiomatic Go, enables `errors.Is()` checks |
| O_EXCL for session creation | Prevents race condition on `isollm up` |
| Schema versioning | Enables future migrations |
| PID in session | Detects stale sessions from crashed processes |
| One JSON file per worker | Concurrent writes don't conflict |
| Partial results from LoadAllWorkers | Don't hide corruption, let caller decide |
| .isollm/ in .gitignore | Runtime state shouldn't be committed |

---

## Estimated Scope

| Component | Lines | Test Lines |
|-----------|-------|------------|
| validate.go | ~150 | ~200 |
| state/types.go | ~80 | - |
| state/state.go | ~200 | ~250 |
| cmd/config.go | ~100 | - |
| cmd/init.go changes | ~30 | - |
| **Total** | **~560** | **~450** |
