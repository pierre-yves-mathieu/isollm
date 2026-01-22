package zellij

import "errors"

// Sentinel errors
var (
	ErrSessionNotFound = errors.New("zellij session not found")
	ErrSessionExists   = errors.New("zellij session already exists")
	ErrZellijNotFound  = errors.New("zellij not found in PATH")
	ErrInvalidLayout   = errors.New("invalid layout mode")
)

// LayoutMode represents how panes should be arranged
type LayoutMode string

const (
	LayoutModeAuto       LayoutMode = "auto"
	LayoutModeHorizontal LayoutMode = "horizontal"
	LayoutModeVertical   LayoutMode = "vertical"
	LayoutModeGrid       LayoutMode = "grid"
)

// ValidLayoutModes contains all valid layout mode values
var ValidLayoutModes = []LayoutMode{
	LayoutModeAuto,
	LayoutModeHorizontal,
	LayoutModeVertical,
	LayoutModeGrid,
}

// IsValid checks if the layout mode is valid
func (m LayoutMode) IsValid() bool {
	for _, valid := range ValidLayoutModes {
		if m == valid {
			return true
		}
	}
	return false
}

// PaneConfig describes a single pane in the layout
type PaneConfig struct {
	Name       string  // Pane identifier (e.g., "worker-1", "dashboard")
	Command    string  // Command to run in the pane
	Args       []string // Command arguments
	SizePercent float64 // Size as percentage (0-100)
	Focus      bool    // Whether this pane should have initial focus
}

// DashboardConfig configures the dashboard pane
type DashboardConfig struct {
	Enabled       bool    // Whether to show dashboard
	HeightPercent float64 // Height as percentage of terminal (default 15%)
	Command       string  // Command to run in dashboard pane
	Args          []string // Dashboard command arguments
}

// SessionConfig contains all configuration for a zellij session
type SessionConfig struct {
	Name       string           // Session name
	Layout     LayoutMode       // Layout mode for worker panes
	Workers    []WorkerPane     // Worker pane configurations
	Dashboard  DashboardConfig  // Dashboard pane configuration
	LayoutFile string           // Path to generated layout file
}

// WorkerPane describes a worker pane
type WorkerPane struct {
	Name          string // Worker name (e.g., "worker-1")
	ContainerName string // LXC container name
}

// Session represents an active zellij session
type Session struct {
	Name       string       // Session name
	Config     SessionConfig // Configuration used to create the session
	LayoutPath string       // Path to the layout file
	Attached   bool         // Whether currently attached
}

// GridDimensions represents the calculated grid layout
type GridDimensions struct {
	Rows    int // Number of rows
	Cols    int // Number of columns
	Total   int // Total number of panes
}

// PaneSize represents calculated pane dimensions
type PaneSize struct {
	WidthPercent  float64
	HeightPercent float64
}
