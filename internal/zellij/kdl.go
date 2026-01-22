package zellij

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GenerateKDL generates a zellij KDL layout configuration
func GenerateKDL(cfg SessionConfig) string {
	var b strings.Builder

	b.WriteString("layout {\n")

	// Generate the main tab with panes
	b.WriteString("    tab name=\"workers\" {\n")

	if cfg.Dashboard.Enabled {
		// Dashboard pane at top
		b.WriteString(generateDashboardPane(cfg.Dashboard))
		b.WriteString("\n")
	}

	// Worker panes based on layout mode
	switch cfg.Layout {
	case LayoutModeHorizontal:
		b.WriteString(generateHorizontalLayout(cfg.Workers))
	case LayoutModeVertical:
		b.WriteString(generateVerticalLayout(cfg.Workers))
	case LayoutModeGrid:
		b.WriteString(generateGridLayout(cfg.Workers))
	default:
		// Default to horizontal
		b.WriteString(generateHorizontalLayout(cfg.Workers))
	}

	b.WriteString("    }\n") // close tab
	b.WriteString("}\n")     // close layout

	return b.String()
}

// generateDashboardPane generates KDL for the dashboard pane
func generateDashboardPane(dashboard DashboardConfig) string {
	var b strings.Builder

	heightStr := fmt.Sprintf("%.0f%%", dashboard.HeightPercent)

	b.WriteString(fmt.Sprintf("        pane size=\"%s\" {\n", heightStr))
	b.WriteString("            name \"dashboard\"\n")

	if dashboard.Command != "" {
		b.WriteString(fmt.Sprintf("            command \"%s\"\n", escapeKDLString(dashboard.Command)))
		if len(dashboard.Args) > 0 {
			b.WriteString("            args ")
			for i, arg := range dashboard.Args {
				if i > 0 {
					b.WriteString(" ")
				}
				b.WriteString(fmt.Sprintf("\"%s\"", escapeKDLString(arg)))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("        }\n")
	return b.String()
}

// generateHorizontalLayout generates KDL for horizontal (side-by-side) panes
func generateHorizontalLayout(workers []WorkerPane) string {
	if len(workers) == 0 {
		return ""
	}

	var b strings.Builder

	// Horizontal split - panes in a row
	b.WriteString("        pane split_direction=\"vertical\" {\n")
	for i, worker := range workers {
		b.WriteString(generateWorkerPane(worker, i == 0))
	}
	b.WriteString("        }\n")

	return b.String()
}

// generateVerticalLayout generates KDL for vertical (stacked) panes
func generateVerticalLayout(workers []WorkerPane) string {
	if len(workers) == 0 {
		return ""
	}

	var b strings.Builder

	// Vertical split - panes stacked
	b.WriteString("        pane split_direction=\"horizontal\" {\n")
	for i, worker := range workers {
		b.WriteString(generateWorkerPane(worker, i == 0))
	}
	b.WriteString("        }\n")

	return b.String()
}

// generateGridLayout generates KDL for grid layout
func generateGridLayout(workers []WorkerPane) string {
	if len(workers) == 0 {
		return ""
	}

	grid := CalculateGridDimensions(len(workers))
	var b strings.Builder

	// Create rows, each containing columns
	b.WriteString("        pane split_direction=\"horizontal\" {\n")

	workerIdx := 0
	for row := 0; row < grid.Rows; row++ {
		// Each row is a horizontal container with vertical splits inside
		b.WriteString("            pane split_direction=\"vertical\" {\n")

		for col := 0; col < grid.Cols && workerIdx < len(workers); col++ {
			worker := workers[workerIdx]
			focus := workerIdx == 0
			b.WriteString(generateWorkerPaneNested(worker, focus))
			workerIdx++
		}

		b.WriteString("            }\n")
	}

	b.WriteString("        }\n")

	return b.String()
}

// generateWorkerPane generates KDL for a single worker pane
func generateWorkerPane(worker WorkerPane, focus bool) string {
	var b strings.Builder

	// Worker panes run lxc exec to enter the container
	cmd := "lxc"
	args := []string{"exec", worker.ContainerName, "--", "su", "-l", "dev"}

	b.WriteString("            pane {\n")
	b.WriteString(fmt.Sprintf("                name \"%s\"\n", escapeKDLString(worker.Name)))
	b.WriteString(fmt.Sprintf("                command \"%s\"\n", cmd))
	b.WriteString("                args ")
	for i, arg := range args {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(fmt.Sprintf("\"%s\"", escapeKDLString(arg)))
	}
	b.WriteString("\n")
	if focus {
		b.WriteString("                focus true\n")
	}
	b.WriteString("            }\n")

	return b.String()
}

// generateWorkerPaneNested generates KDL for a worker pane (double nested for grid)
func generateWorkerPaneNested(worker WorkerPane, focus bool) string {
	var b strings.Builder

	// Worker panes run lxc exec to enter the container
	cmd := "lxc"
	args := []string{"exec", worker.ContainerName, "--", "su", "-l", "dev"}

	b.WriteString("                pane {\n")
	b.WriteString(fmt.Sprintf("                    name \"%s\"\n", escapeKDLString(worker.Name)))
	b.WriteString(fmt.Sprintf("                    command \"%s\"\n", cmd))
	b.WriteString("                    args ")
	for i, arg := range args {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(fmt.Sprintf("\"%s\"", escapeKDLString(arg)))
	}
	b.WriteString("\n")
	if focus {
		b.WriteString("                    focus true\n")
	}
	b.WriteString("                }\n")

	return b.String()
}

// escapeKDLString escapes special characters in a KDL string
func escapeKDLString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}

// WriteLayoutFile writes the KDL layout to a file
func WriteLayoutFile(cfg SessionConfig, dir string) (string, error) {
	// Generate the KDL content
	kdl := GenerateKDL(cfg)

	// Ensure directory exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create layout directory: %w", err)
	}

	// Create the layout file
	filename := fmt.Sprintf("%s.kdl", cfg.Name)
	filepath := filepath.Join(dir, filename)

	if err := os.WriteFile(filepath, []byte(kdl), 0644); err != nil {
		return "", fmt.Errorf("failed to write layout file: %w", err)
	}

	return filepath, nil
}

// WriteLayoutToTemp writes the layout to a temp directory
func WriteLayoutToTemp(cfg SessionConfig) (string, error) {
	tempDir := os.TempDir()
	layoutDir := filepath.Join(tempDir, "isollm-layouts")
	return WriteLayoutFile(cfg, layoutDir)
}

// RemoveLayoutFile removes a previously written layout file
func RemoveLayoutFile(path string) error {
	if path == "" {
		return nil
	}
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
