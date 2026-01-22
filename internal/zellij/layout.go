package zellij

import "math"

const (
	// DefaultDashboardHeight is the default dashboard height percentage
	DefaultDashboardHeight = 15.0
	// MinPaneSize is the minimum pane size percentage
	MinPaneSize = 5.0
)

// CalculateLayout determines the effective layout mode based on worker count
// When mode is "auto":
//   - 1-2 workers: horizontal split
//   - 3+ workers: grid layout
func CalculateLayout(mode LayoutMode, workerCount int) LayoutMode {
	if mode != LayoutModeAuto {
		return mode
	}

	if workerCount <= 2 {
		return LayoutModeHorizontal
	}
	return LayoutModeGrid
}

// CalculateGridDimensions calculates optimal grid dimensions for N panes
// Tries to make the grid as square as possible
func CalculateGridDimensions(paneCount int) GridDimensions {
	if paneCount <= 0 {
		return GridDimensions{Rows: 0, Cols: 0, Total: 0}
	}

	if paneCount == 1 {
		return GridDimensions{Rows: 1, Cols: 1, Total: 1}
	}

	if paneCount == 2 {
		return GridDimensions{Rows: 1, Cols: 2, Total: 2}
	}

	// Calculate optimal grid dimensions
	// Start with square root and adjust
	sqrt := math.Sqrt(float64(paneCount))
	cols := int(math.Ceil(sqrt))
	rows := int(math.Ceil(float64(paneCount) / float64(cols)))

	return GridDimensions{
		Rows:  rows,
		Cols:  cols,
		Total: paneCount,
	}
}

// CalculatePaneSizes calculates the size percentages for panes in a layout
func CalculatePaneSizes(mode LayoutMode, paneCount int) []PaneSize {
	if paneCount <= 0 {
		return nil
	}

	sizes := make([]PaneSize, paneCount)

	switch mode {
	case LayoutModeHorizontal:
		// All panes in a row, equal width
		widthPercent := 100.0 / float64(paneCount)
		for i := range sizes {
			sizes[i] = PaneSize{
				WidthPercent:  widthPercent,
				HeightPercent: 100.0,
			}
		}

	case LayoutModeVertical:
		// All panes in a column, equal height
		heightPercent := 100.0 / float64(paneCount)
		for i := range sizes {
			sizes[i] = PaneSize{
				WidthPercent:  100.0,
				HeightPercent: heightPercent,
			}
		}

	case LayoutModeGrid:
		grid := CalculateGridDimensions(paneCount)
		widthPercent := 100.0 / float64(grid.Cols)
		heightPercent := 100.0 / float64(grid.Rows)

		for i := range sizes {
			sizes[i] = PaneSize{
				WidthPercent:  widthPercent,
				HeightPercent: heightPercent,
			}
		}

	default:
		// Fall back to horizontal
		return CalculatePaneSizes(LayoutModeHorizontal, paneCount)
	}

	return sizes
}

// CalculateWorkerAreaHeight returns the height available for workers
// after accounting for dashboard
func CalculateWorkerAreaHeight(dashboardEnabled bool, dashboardHeight float64) float64 {
	if !dashboardEnabled {
		return 100.0
	}
	if dashboardHeight <= 0 {
		dashboardHeight = DefaultDashboardHeight
	}
	return 100.0 - dashboardHeight
}

// AdjustPaneSizesForDashboard adjusts pane sizes to fit within the worker area
func AdjustPaneSizesForDashboard(sizes []PaneSize, workerAreaHeight float64) []PaneSize {
	if workerAreaHeight >= 100.0 {
		return sizes
	}

	adjusted := make([]PaneSize, len(sizes))
	scale := workerAreaHeight / 100.0

	for i, size := range sizes {
		adjusted[i] = PaneSize{
			WidthPercent:  size.WidthPercent,
			HeightPercent: size.HeightPercent * scale,
		}
	}

	return adjusted
}

// BuildSessionConfig creates a SessionConfig from the given parameters
func BuildSessionConfig(
	sessionName string,
	layout LayoutMode,
	workers []WorkerPane,
	dashboard DashboardConfig,
) SessionConfig {
	// Resolve auto layout
	effectiveLayout := CalculateLayout(layout, len(workers))

	// Set default dashboard height if not specified
	if dashboard.Enabled && dashboard.HeightPercent <= 0 {
		dashboard.HeightPercent = DefaultDashboardHeight
	}

	return SessionConfig{
		Name:      sessionName,
		Layout:    effectiveLayout,
		Workers:   workers,
		Dashboard: dashboard,
	}
}

// CreateWorkerPanes creates WorkerPane configs from container names
func CreateWorkerPanes(containerNames []string) []WorkerPane {
	panes := make([]WorkerPane, len(containerNames))
	for i, name := range containerNames {
		panes[i] = WorkerPane{
			Name:          name,
			ContainerName: name,
		}
	}
	return panes
}
