# Plan Review: Expert Feedback Analysis

This document reviews feedback from two expert perspectives and proposes concrete decisions for each issue.

---

## Category 1: Go Patterns & Idioms

### 1.1 StateManager Lacks Interface (HIGH PRIORITY)

**Issue:** The plan defines `StateManager` as a concrete struct with no interface, unlike `git.Executor` which enables testing.

**Expert Recommendation:** Define a `State` interface for testability and future flexibility.

**Decision: ACCEPT**

Add interface to mirror the `git.Executor` pattern:

```go
// State abstracts state persistence for testing
type State interface {
    // Session
    SaveSession(s *Session) error
    LoadSession() (*Session, error)
    ClearSession() error
    HasActiveSession() (bool, error)

    // Workers
    SaveWorker(w *WorkerState) error
    LoadWorker(name string) (*WorkerState, error)
    LoadAllWorkers() ([]*WorkerState, []error)  // Returns partial results + errors
    DeleteWorker(name string) error
    ClearAllWorkers() error
}

// FileState implements State using the filesystem
type FileState struct {
    stateDir string
}

// Compile-time interface check
var _ State = (*FileState)(nil)

// NewWithDir creates a FileState (for testing with custom dir)
func NewWithDir(stateDir string) *FileState

// New creates a FileState for the project's .isollm directory
func New(projectRoot string) *FileState
```

**Rationale:** Matches existing codebase patterns (`git.Executor`, `barerepo.NewWithExecutor`). Enables unit testing commands without filesystem.

---

### 1.2 Error Aggregation Pattern

**Issue:** Using `[]string` for error accumulation is acceptable but not idiomatic. Go 1.20+ has `errors.Join`.

**Expert Recommendation:** Use `errors.Join` or custom `ValidationErrors` type.

**Decision: ACCEPT (use custom type)**

```go
// ValidationError represents multiple validation failures
type ValidationError struct {
    Errors []string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("config validation failed:\n  - %s", strings.Join(e.Errors, "\n  - "))
}

// Add appends an error message
func (e *ValidationError) Add(msg string) {
    e.Errors = append(e.Errors, msg)
}

// HasErrors returns true if there are validation errors
func (e *ValidationError) HasErrors() bool {
    return len(e.Errors) > 0
}
```

**Rationale:** Custom type provides `HasErrors()` for cleaner conditional logic. `errors.Join` is more for wrapping distinct errors; this is aggregating messages.

---

### 1.3 Map for validLayouts Set

**Issue:** `map[string]bool` works but `map[string]struct{}` is more idiomatic for sets.

**Decision: ACCEPT**

```go
var validLayouts = map[string]struct{}{
    "auto": {}, "horizontal": {}, "vertical": {}, "grid": {},
}

// Usage:
if _, ok := validLayouts[c.Zellij.Layout]; !ok {
    errs.Add("zellij.layout must be one of: auto, horizontal, vertical, grid")
}
```

**Rationale:** Minor improvement, signals "set" intent, zero memory per entry.

---

### 1.4 LoadSession Returns (nil, nil) Anti-pattern (HIGH PRIORITY)

**Issue:** Returning `(nil, nil)` forces callers to check both return values.

**Decision: ACCEPT - use sentinel error**

```go
var ErrNoSession = errors.New("no active session")

func (m *FileState) LoadSession() (*Session, error) {
    var s Session
    if err := m.loadJSON("session.json", &s); err != nil {
        if os.IsNotExist(err) {
            return nil, ErrNoSession
        }
        return nil, fmt.Errorf("failed to load session: %w", err)
    }
    return &s, nil
}

// Caller usage:
session, err := state.LoadSession()
if errors.Is(err, state.ErrNoSession) {
    // No session - this is expected
} else if err != nil {
    // Actual error
}
```

**Rationale:** Idiomatic Go, enables `errors.Is()` checks, clear intent.

---

### 1.5 Silently Skipping Corrupted Files (HIGH PRIORITY)

**Issue:** `LoadAllWorkers()` silently skips files that fail to parse, hiding corruption.

**Decision: ACCEPT - return partial results with errors**

```go
// LoadResult contains workers and any errors encountered
type LoadResult struct {
    Workers []*WorkerState
    Errors  []error
}

func (m *FileState) LoadAllWorkers() (*LoadResult, error) {
    result := &LoadResult{}

    workersDir := filepath.Join(m.stateDir, "workers")
    entries, err := os.ReadDir(workersDir)
    if err != nil {
        if os.IsNotExist(err) {
            return result, nil  // Empty result, no error
        }
        return nil, err  // Can't read directory at all
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
```

**Rationale:** Caller can decide how to handle partial failures. Essential for debugging.

---

### 1.6 Missing filepath Import

**Issue:** `cmd/config.go` uses `filepath.Join` but doesn't import `"path/filepath"`.

**Decision: ACCEPT - fix the import**

Trivial fix, add to imports.

---

## Category 2: Concurrency & File Safety

### 2.1 Atomic File Writes (HIGH PRIORITY)

**Issue:** `os.WriteFile` is not atomic. Concurrent read during write = torn/corrupted data.

**Decision: ACCEPT - implement atomic writes**

```go
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

    // Atomic rename (POSIX guarantees atomicity on same filesystem)
    if err := os.Rename(tmpPath, fullPath); err != nil {
        os.Remove(tmpPath)  // Cleanup on failure
        return fmt.Errorf("failed to rename: %w", err)
    }

    return nil
}
```

**Rationale:** Standard pattern used by many production systems. Prevents torn reads entirely.

---

### 2.2 Session Creation Race Condition

**Issue:** Two `isollm up` commands could both pass `HasActiveSession()` check before either writes.

**Decision: ACCEPT - use O_EXCL for atomic creation**

```go
var ErrSessionExists = errors.New("session already exists")

func (m *FileState) CreateSession(s *Session) error {
    fullPath := filepath.Join(m.stateDir, "session.json")

    data, err := json.MarshalIndent(s, "", "  ")
    if err != nil {
        return fmt.Errorf("failed to marshal session: %w", err)
    }

    // O_EXCL fails if file exists - atomic check-and-create
    f, err := os.OpenFile(fullPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
    if err != nil {
        if os.IsExist(err) {
            return ErrSessionExists
        }
        return fmt.Errorf("failed to create session file: %w", err)
    }
    defer f.Close()

    if _, err := f.Write(data); err != nil {
        os.Remove(fullPath)  // Cleanup partial write
        return fmt.Errorf("failed to write session: %w", err)
    }

    return nil
}

// SaveSession updates an existing session (uses atomic write)
func (m *FileState) SaveSession(s *Session) error {
    return m.saveJSON("session.json", s)
}
```

**Rationale:** Prevents race condition at OS level. Clear distinction between create (exclusive) and update.

---

### 2.3 File Locking for Session

**Issue:** Multiple processes could modify session.json concurrently.

**Decision: DEFER**

For MVP, rely on:
1. Atomic writes (prevents corruption)
2. `CreateSession` with `O_EXCL` (prevents duplicate sessions)
3. Single-writer assumption (only orchestrator modifies session)

File locking (flock) can be added later if needed. Adding a dependency like `github.com/gofrs/flock` is premature.

**Rationale:** Atomic writes + O_EXCL handles the critical cases. Workers only modify their own files.

---

## Category 3: State Location & Schema

### 3.1 Split State Location

**Issue:** Runtime state in `.isollm/` (project) vs bare repo in `~/.isollm/` (home) is confusing.

**Expert Recommendation:** Consolidate to `~/.isollm/<project>/`.

**Decision: REJECT - keep split model**

Reasons to keep current design:
1. **Project portability**: `.isollm/` travels with the repo (can be gitignored)
2. **Multi-user scenarios**: Each user has their own session state in their checkout
3. **Explicit separation**: Config (project-level) vs runtime state (user-level) is clear
4. **Matches bare repo semantics**: Bare repo is shared infrastructure, session is user-specific

However, ensure `.isollm/` is in the default `.gitignore` created by `init`.

**Mitigation:** Add to `init` command:
```go
// Append to .gitignore if not already present
gitignorePath := filepath.Join(dir, ".gitignore")
appendIfMissing(gitignorePath, ".isollm/")
```

---

### 3.2 State Schema Versioning (MEDIUM PRIORITY)

**Issue:** No version field means no migration path when schema evolves.

**Decision: ACCEPT**

Add version to all state structures:

```go
const CurrentSessionVersion = 1
const CurrentWorkerVersion = 1

type Session struct {
    Version       int       `json:"version"`
    StartedAt     time.Time `json:"started_at"`
    // ...
}

type WorkerState struct {
    Version       int          `json:"version"`
    Name          string       `json:"name"`
    // ...
}

// On load, check version and migrate if needed
func (m *FileState) LoadSession() (*Session, error) {
    var s Session
    if err := m.loadJSON("session.json", &s); err != nil {
        // ...
    }
    if s.Version < CurrentSessionVersion {
        if err := migrateSession(&s); err != nil {
            return nil, fmt.Errorf("failed to migrate session: %w", err)
        }
    }
    return &s, nil
}
```

**Rationale:** Low cost now, prevents pain later.

---

## Category 4: Worker Lifecycle & State Machine

### 4.1 Ambiguous Worker States

**Issue:** `running` vs `idle` vs `busy` semantics are unclear.

**Decision: ACCEPT - simplify state machine**

```go
const (
    WorkerStatusCreating  WorkerStatus = "creating"   // Container being provisioned
    WorkerStatusIdle      WorkerStatus = "idle"       // Running, no task claimed
    WorkerStatusBusy      WorkerStatus = "busy"       // Running, task in progress
    WorkerStatusStopping  WorkerStatus = "stopping"   // Graceful shutdown in progress
    WorkerStatusStopped   WorkerStatus = "stopped"    // Container stopped
    WorkerStatusError     WorkerStatus = "error"      // Unrecoverable error
)
```

Remove `running` - it's redundant with `idle`.

State transition diagram:
```
creating ──► idle ◄──► busy
              │         │
              ▼         │
           stopping ◄───┘
              │
              ▼
           stopped

Any state ──► error (on unrecoverable failure)
```

---

### 4.2 Stale Session Detection

**Issue:** Crashed session leaves `session.json` behind. Next `isollm up` doesn't know it's stale.

**Decision: ACCEPT - add PID and liveness check**

```go
type Session struct {
    Version       int       `json:"version"`
    StartedAt     time.Time `json:"started_at"`
    PID           int       `json:"pid"`              // NEW: orchestrator PID
    ProjectRoot   string    `json:"project_root"`
    BareRepoPath  string    `json:"bare_repo_path"`
    ZellijSession string    `json:"zellij_session"`
}

// HasActiveSession checks if session exists AND is live
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
        return false, nil  // Stale session
    }

    return true, nil
}

func isProcessAlive(pid int) bool {
    process, err := os.FindProcess(pid)
    if err != nil {
        return false
    }
    // On Unix, FindProcess always succeeds. Send signal 0 to check.
    err = process.Signal(syscall.Signal(0))
    return err == nil
}
```

**Rationale:** PID check is fast and reliable. Allows `isollm up` to detect and clean up stale sessions.

---

### 4.3 Worker Error Tracking

**Issue:** No way to track why a worker failed.

**Decision: ACCEPT**

```go
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
    LastError     string       `json:"last_error,omitempty"`     // NEW
    ErrorTime     time.Time    `json:"error_time,omitempty"`     // NEW
}
```

---

### 4.4 Session Status Field

**Issue:** No way to distinguish session lifecycle phases.

**Decision: ACCEPT**

```go
type SessionStatus string

const (
    SessionStatusInitializing SessionStatus = "initializing"
    SessionStatusRunning      SessionStatus = "running"
    SessionStatusShuttingDown SessionStatus = "shutting_down"
)

type Session struct {
    Version       int           `json:"version"`
    Status        SessionStatus `json:"status"`           // NEW
    StartedAt     time.Time     `json:"started_at"`
    PID           int           `json:"pid"`
    // ...
}
```

**Rationale:** Enables recovery from partial `up` or `down` failures.

---

## Category 5: Validation Completeness

### 5.1 Additional Validations Needed

**Decision: ACCEPT - add these validations**

| Field | Add Validation |
|-------|----------------|
| `project` | Min length 2, max length 64 |
| `image` | Warn if no tag (using implicit `:latest`) |
| `git.branch_prefix` | Must not be just `/`, must be valid ref |
| `git.upstream` | If set, valid remote name format |
| `airyra.host` | Valid hostname or IP |
| `ports` | Check for duplicates, no conflict with `airyra.port` |

```go
// Project name bounds
const (
    MinProjectNameLen = 2
    MaxProjectNameLen = 64
)

if len(c.Project) < MinProjectNameLen {
    errs.Add(fmt.Sprintf("project name must be at least %d characters", MinProjectNameLen))
}
if len(c.Project) > MaxProjectNameLen {
    errs.Add(fmt.Sprintf("project name cannot exceed %d characters", MaxProjectNameLen))
}

// Image tag warning (not an error)
if !strings.Contains(c.Image, ":") {
    // Return as warning, not error
    warnings = append(warnings, "image has no tag, will use :latest")
}

// Duplicate ports
seen := make(map[string]bool)
for _, p := range c.Ports {
    hostPort := strings.Split(p, ":")[0]
    if seen[hostPort] {
        errs.Add(fmt.Sprintf("duplicate host port: %s", hostPort))
    }
    seen[hostPort] = true

    // Check airyra port conflict
    if hostPort == strconv.Itoa(c.Airyra.Port) {
        errs.Add(fmt.Sprintf("port %s conflicts with airyra.port", hostPort))
    }
}
```

---

### 5.2 MaxWorkers Constant

**Issue:** Magic number `20` should be a named constant.

**Decision: ACCEPT**

```go
const (
    MinWorkers = 1
    MaxWorkers = 20  // Reasonable upper bound for local machine
)

if c.Workers < MinWorkers {
    errs.Add(fmt.Sprintf("workers must be at least %d", MinWorkers))
} else if c.Workers > MaxWorkers {
    errs.Add(fmt.Sprintf("workers cannot exceed %d", MaxWorkers))
}
```

---

## Category 6: Integration & Architecture

### 6.1 Airyra Task State Source of Truth

**Issue:** Plan doesn't define whether airyra or isollm state is authoritative for task assignments.

**Decision: AIRYRA IS AUTHORITATIVE**

- `WorkerState.CurrentTask` is a cache for display purposes
- All task operations (claim, release, complete) go through airyra API
- `isollm status` queries airyra for current task state, updates local cache

```go
// TaskRef is a cache, not authoritative
type TaskRef struct {
    ID        string    `json:"id"`
    Title     string    `json:"title"`
    ClaimedAt time.Time `json:"claimed_at"`
    // No status here - query airyra for that
}
```

**Rationale:** Single source of truth prevents divergence. isollm state is for local display/recovery only.

---

### 6.2 Container State Reconciliation

**Issue:** `WorkerState.Status` can drift from actual LXC container state.

**Decision: DEFER detailed implementation, but add design note**

The `isollm status` command should reconcile:
1. Load worker state from files
2. Query actual container status via `lxc list`
3. Update any drifted state

This is implementation detail for the `status` command, not core state module.

Add to worker state:
```go
// LastReconciled tracks when state was verified against container
LastReconciled time.Time `json:"last_reconciled,omitempty"`
```

---

### 6.3 ContainerManager Interface

**Issue:** No interface defined for container operations.

**Decision: DEFER to container management implementation phase**

This is out of scope for project/config management. Will be defined when implementing `isollm up` / `worker` commands.

Note for future: interface should include:
```go
type ContainerManager interface {
    Create(name string, config ContainerConfig) (string, error)
    Start(name string) error
    Stop(name string) error
    Status(name string) (ContainerStatus, error)
    Destroy(name string) error
    Exec(name string, cmd []string) (string, error)
}
```

---

## Category 7: Deferred / Out of Scope

These items are valid concerns but not addressed in this implementation phase:

| Item | Reason to Defer |
|------|-----------------|
| File locking (flock) | Atomic writes + O_EXCL sufficient for MVP |
| Config hot-reload | Edge case, can detect and warn later |
| State backup/history | Nice-to-have, not essential |
| Detailed container integration | Separate implementation phase |
| Zellij pane monitoring | Separate implementation phase |
| Heartbeat-based stale detection | PID check sufficient for MVP |

---

## Summary: Changes to Plan

### High Priority (Must Do)
1. Add `State` interface for testability
2. Implement atomic file writes (temp + rename)
3. Use sentinel errors (`ErrNoSession`, `ErrSessionExists`)
4. Return partial results from `LoadAllWorkers()`
5. Add `CreateSession()` with `O_EXCL` for race prevention
6. Add schema versioning (`Version` field)
7. Add PID to session for stale detection
8. Simplify worker states (remove `running`, add `stopping`)
9. Add error tracking to WorkerState

### Medium Priority (Should Do)
1. Use custom `ValidationError` type
2. Add missing validations (duplicates, port conflicts, etc.)
3. Add bounds constants (`MinWorkers`, `MaxWorkers`, etc.)
4. Add session status field
5. Add `.isollm/` to generated `.gitignore`

### Low Priority (Nice to Have)
1. Use `map[string]struct{}` for sets
2. Image tag warning
3. `LastReconciled` timestamp

### Deferred
1. File locking
2. Container state reconciliation
3. ContainerManager interface
4. Config hot-reload
