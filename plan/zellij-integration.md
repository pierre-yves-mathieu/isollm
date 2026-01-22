# Terminal Session Feature Plan

## Overview

A configurable terminal session system for isollm that supports multiple terminal emulators (Alacritty, kitty, etc.), multiple multiplexers (Zellij, tmux, or none), and flexible pane configuration.

## Goals

- **Configurable terminal**: User chooses their preferred terminal emulator
- **Configurable multiplexer**: Zellij, tmux, or single-pane mode
- **Flexible pane commands**: Claude Code, shell, or custom command
- **Layout options**: For multiplexers that support it

## Config Schema

Add to `isollm.yaml`:

```yaml
terminal:
  # Terminal emulator to launch (optional - uses current terminal if not set)
  emulator: alacritty        # alacritty | kitty | wezterm | gnome-terminal | none
  emulator_args: []          # Additional args for the emulator

  # Terminal multiplexer (optional)
  multiplexer: zellij        # zellij | tmux | none
  layout: auto               # auto | horizontal | vertical | grid | single

  # What runs in worker panes
  pane_command: claude       # Command to run in each pane
  working_dir: ~/project     # Working directory inside container
```

## Files to Create/Modify

```
internal/terminal/
├── terminal.go      # Terminal emulator abstraction
├── multiplexer.go   # Multiplexer abstraction (zellij, tmux)
├── layout.go        # Layout generation for multiplexers
├── session.go       # Session management
└── terminal_test.go

internal/config/
└── config.go        # Add TerminalConfig struct (modify existing)

cmd/
└── up.go            # Integration with isollm up
```

---

## Implementation

### Step 1: Update Config (`internal/config/config.go`)

Add `TerminalConfig` struct:

```go
type TerminalConfig struct {
    Emulator     string   `yaml:"emulator,omitempty"`      // alacritty, kitty, etc.
    EmulatorArgs []string `yaml:"emulator_args,omitempty"`
    Multiplexer  string   `yaml:"multiplexer,omitempty"`   // zellij, tmux, none
    Layout       string   `yaml:"layout,omitempty"`        // auto, horizontal, etc.
    PaneCommand  string   `yaml:"pane_command,omitempty"`  // claude, bash, etc.
    WorkingDir   string   `yaml:"working_dir,omitempty"`
}
```

Replace `ZellijConfig` with `TerminalConfig` in `Config` struct.

Defaults:
```go
Terminal: TerminalConfig{
    Emulator:    "",        // none = use current terminal
    Multiplexer: "zellij",
    Layout:      "auto",
    PaneCommand: "claude",
}
```

### Step 2: Terminal Emulator Abstraction (`internal/terminal/terminal.go`)

```go
package terminal

// Emulator launches a terminal window with a command
type Emulator interface {
    // Launch opens a new terminal window running the given command
    Launch(command string, args []string) error
    // Name returns the emulator name
    Name() string
}

// Supported emulators
func NewEmulator(name string, extraArgs []string) (Emulator, error)

// Implementations
type alacrittyEmulator struct { args []string }
type kittyEmulator struct { args []string }
type weztermEmulator struct { args []string }
type noEmulator struct {}  // runs in current terminal
```

Each emulator knows how to:
- Launch itself with a command: `alacritty -e zellij ...`
- Pass through extra args from config

### Step 3: Multiplexer Abstraction (`internal/terminal/multiplexer.go`)

```go
package terminal

// Multiplexer manages terminal panes/windows
type Multiplexer interface {
    // Start creates a new session with N panes
    Start(sessionName string, panes []PaneConfig) error
    // Attach connects to existing session
    Attach(sessionName string) error
    // Kill terminates a session
    Kill(sessionName string) error
    // IsRunning checks if session exists
    IsRunning(sessionName string) (bool, error)
    // Command returns the command string to launch this multiplexer
    Command(sessionName string, panes []PaneConfig) (string, []string, error)
}

type PaneConfig struct {
    Name       string
    Command    string
    Args       []string
    WorkingDir string
}

// Implementations
func NewMultiplexer(name string) (Multiplexer, error)

type zellijMultiplexer struct{}
type tmuxMultiplexer struct{}
type noMultiplexer struct{}  // single pane, no multiplexer
```

### Step 4: Layout Generation (`internal/terminal/layout.go`)

For Zellij - generate KDL:

```go
type LayoutMode string

const (
    LayoutAuto       LayoutMode = "auto"
    LayoutHorizontal LayoutMode = "horizontal"
    LayoutVertical   LayoutMode = "vertical"
    LayoutGrid       LayoutMode = "grid"
    LayoutSingle     LayoutMode = "single"
)

// GenerateZellijKDL creates a KDL layout file
func GenerateZellijKDL(panes []PaneConfig, mode LayoutMode) string

// GenerateTmuxCommands creates tmux split commands
func GenerateTmuxCommands(panes []PaneConfig, mode LayoutMode) []string
```

Auto mode selection:
- 1 pane → single
- 2-3 panes → horizontal (side-by-side)
- 4+ panes → grid

### Step 5: Session Manager (`internal/terminal/session.go`)

Orchestrates emulator + multiplexer:

```go
type Session struct {
    name        string
    emulator    Emulator
    multiplexer Multiplexer
    panes       []PaneConfig
    layoutMode  LayoutMode
}

func NewSession(cfg *config.Config) (*Session, error)

func (s *Session) Start() error      // Launch terminal with multiplexer
func (s *Session) Attach() error     // Attach to existing
func (s *Session) Kill() error       // Terminate
func (s *Session) IsRunning() bool
```

**Start() flow:**
1. Build pane configs from worker count + config
2. If multiplexer != none: generate layout, get multiplexer command
3. If emulator != none: launch emulator with multiplexer command
4. Else: run multiplexer command directly

### Step 6: CLI Integration (`cmd/up.go`)

```go
var upCmd = &cobra.Command{
    Use:   "up",
    Short: "Start isollm session",
    RunE:  runUp,
}

var (
    upWorkers int     // -n override worker count
    upLayout  string  // --layout override
    upAttach  bool    // --attach to existing session
)

func runUp(cmd *cobra.Command, args []string) error {
    // 1. Find project, load config
    // 2. Create Session from config
    // 3. Check if already running
    // 4. Start or attach based on flags
}
```

---

## KDL Example (Zellij)

3 workers, horizontal layout:

```kdl
layout {
    tab name="isollm" focus=true {
        pane split_direction="vertical" {
            pane name="worker-1" command="claude" cwd="/home/dev/project"
            pane name="worker-2" command="claude" cwd="/home/dev/project"
            pane name="worker-3" command="claude" cwd="/home/dev/project"
        }
    }
}
```

---

## Tmux Equivalent

```bash
tmux new-session -d -s isollm -n main
tmux split-window -h -t isollm
tmux split-window -h -t isollm
tmux send-keys -t isollm:0.0 'claude' Enter
tmux send-keys -t isollm:0.1 'claude' Enter
tmux send-keys -t isollm:0.2 'claude' Enter
tmux attach -t isollm
```

---

## Launch Examples

**Alacritty + Zellij:**
```bash
alacritty -e zellij --layout /tmp/isollm-layout.kdl --session isollm-myproject
```

**Kitty + tmux:**
```bash
kitty tmux new-session -A -s isollm-myproject
```

**Current terminal + Zellij:**
```bash
zellij --layout /tmp/isollm-layout.kdl --session isollm-myproject
```

**No multiplexer (single pane):**
```bash
alacritty -e claude
```

---

## Error Handling

| Error | Detection | Message |
|-------|-----------|---------|
| Emulator not found | `exec.LookPath()` | "alacritty not found in PATH" |
| Multiplexer not found | `exec.LookPath()` | "zellij not found in PATH" |
| Invalid emulator name | Config validation | "unknown emulator: foo" |
| Invalid multiplexer | Config validation | "unknown multiplexer: bar" |
| Session exists | `IsRunning()` | "session exists, use --attach" |
| Layout file write fail | `os.WriteFile()` | "failed to write layout" |

---

## Testing

1. **Unit tests**: Layout generation for each multiplexer
2. **Mock tests**: Emulator/multiplexer interfaces with mocks
3. **Integration**: Manual testing with real terminals

---

## Migration

Replace existing `ZellijConfig`:
```go
// Old
Zellij ZellijConfig `yaml:"zellij"`

// New
Terminal TerminalConfig `yaml:"terminal"`
```

Provide backward compatibility or migration note.

---

## Out of Scope

- Container creation/management (separate feature)
- Airyra integration (separate feature)
- Dashboard pane (future enhancement)
- Worker process management inside panes
