package zellij

import (
	"testing"
)

func TestCalculateLayout(t *testing.T) {
	tests := []struct {
		name        string
		mode        LayoutMode
		workerCount int
		expected    LayoutMode
	}{
		{
			name:        "auto with 1 worker returns horizontal",
			mode:        LayoutModeAuto,
			workerCount: 1,
			expected:    LayoutModeHorizontal,
		},
		{
			name:        "auto with 2 workers returns horizontal",
			mode:        LayoutModeAuto,
			workerCount: 2,
			expected:    LayoutModeHorizontal,
		},
		{
			name:        "auto with 3 workers returns grid",
			mode:        LayoutModeAuto,
			workerCount: 3,
			expected:    LayoutModeGrid,
		},
		{
			name:        "auto with 4 workers returns grid",
			mode:        LayoutModeAuto,
			workerCount: 4,
			expected:    LayoutModeGrid,
		},
		{
			name:        "auto with 10 workers returns grid",
			mode:        LayoutModeAuto,
			workerCount: 10,
			expected:    LayoutModeGrid,
		},
		{
			name:        "explicit horizontal is preserved",
			mode:        LayoutModeHorizontal,
			workerCount: 5,
			expected:    LayoutModeHorizontal,
		},
		{
			name:        "explicit vertical is preserved",
			mode:        LayoutModeVertical,
			workerCount: 5,
			expected:    LayoutModeVertical,
		},
		{
			name:        "explicit grid is preserved",
			mode:        LayoutModeGrid,
			workerCount: 2,
			expected:    LayoutModeGrid,
		},
		{
			name:        "auto with zero workers returns horizontal",
			mode:        LayoutModeAuto,
			workerCount: 0,
			expected:    LayoutModeHorizontal,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := CalculateLayout(tc.mode, tc.workerCount)
			if result != tc.expected {
				t.Errorf("CalculateLayout(%q, %d) = %q, want %q",
					tc.mode, tc.workerCount, result, tc.expected)
			}
		})
	}
}

func TestCalculateGridDimensions(t *testing.T) {
	tests := []struct {
		name      string
		paneCount int
		expected  GridDimensions
	}{
		{
			name:      "zero panes",
			paneCount: 0,
			expected:  GridDimensions{Rows: 0, Cols: 0, Total: 0},
		},
		{
			name:      "1 pane",
			paneCount: 1,
			expected:  GridDimensions{Rows: 1, Cols: 1, Total: 1},
		},
		{
			name:      "2 panes",
			paneCount: 2,
			expected:  GridDimensions{Rows: 1, Cols: 2, Total: 2},
		},
		{
			name:      "3 panes",
			paneCount: 3,
			expected:  GridDimensions{Rows: 2, Cols: 2, Total: 3},
		},
		{
			name:      "4 panes",
			paneCount: 4,
			expected:  GridDimensions{Rows: 2, Cols: 2, Total: 4},
		},
		{
			name:      "5 panes",
			paneCount: 5,
			expected:  GridDimensions{Rows: 2, Cols: 3, Total: 5},
		},
		{
			name:      "6 panes",
			paneCount: 6,
			expected:  GridDimensions{Rows: 2, Cols: 3, Total: 6},
		},
		{
			name:      "9 panes",
			paneCount: 9,
			expected:  GridDimensions{Rows: 3, Cols: 3, Total: 9},
		},
		{
			name:      "10 panes",
			paneCount: 10,
			expected:  GridDimensions{Rows: 3, Cols: 4, Total: 10},
		},
		{
			name:      "negative panes treated as zero",
			paneCount: -1,
			expected:  GridDimensions{Rows: 0, Cols: 0, Total: 0},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := CalculateGridDimensions(tc.paneCount)
			if result != tc.expected {
				t.Errorf("CalculateGridDimensions(%d) = %+v, want %+v",
					tc.paneCount, result, tc.expected)
			}
		})
	}
}

func TestCalculatePaneSizes(t *testing.T) {
	tests := []struct {
		name      string
		mode      LayoutMode
		paneCount int
		expected  []PaneSize
	}{
		{
			name:      "zero panes returns nil",
			mode:      LayoutModeHorizontal,
			paneCount: 0,
			expected:  nil,
		},
		{
			name:      "negative panes returns nil",
			mode:      LayoutModeHorizontal,
			paneCount: -1,
			expected:  nil,
		},
		{
			name:      "horizontal 1 pane",
			mode:      LayoutModeHorizontal,
			paneCount: 1,
			expected: []PaneSize{
				{WidthPercent: 100.0, HeightPercent: 100.0},
			},
		},
		{
			name:      "horizontal 2 panes",
			mode:      LayoutModeHorizontal,
			paneCount: 2,
			expected: []PaneSize{
				{WidthPercent: 50.0, HeightPercent: 100.0},
				{WidthPercent: 50.0, HeightPercent: 100.0},
			},
		},
		{
			name:      "horizontal 4 panes",
			mode:      LayoutModeHorizontal,
			paneCount: 4,
			expected: []PaneSize{
				{WidthPercent: 25.0, HeightPercent: 100.0},
				{WidthPercent: 25.0, HeightPercent: 100.0},
				{WidthPercent: 25.0, HeightPercent: 100.0},
				{WidthPercent: 25.0, HeightPercent: 100.0},
			},
		},
		{
			name:      "vertical 1 pane",
			mode:      LayoutModeVertical,
			paneCount: 1,
			expected: []PaneSize{
				{WidthPercent: 100.0, HeightPercent: 100.0},
			},
		},
		{
			name:      "vertical 2 panes",
			mode:      LayoutModeVertical,
			paneCount: 2,
			expected: []PaneSize{
				{WidthPercent: 100.0, HeightPercent: 50.0},
				{WidthPercent: 100.0, HeightPercent: 50.0},
			},
		},
		{
			name:      "vertical 3 panes",
			mode:      LayoutModeVertical,
			paneCount: 3,
			expected: []PaneSize{
				{WidthPercent: 100.0, HeightPercent: 33.333333333333336},
				{WidthPercent: 100.0, HeightPercent: 33.333333333333336},
				{WidthPercent: 100.0, HeightPercent: 33.333333333333336},
			},
		},
		{
			name:      "grid 4 panes (2x2)",
			mode:      LayoutModeGrid,
			paneCount: 4,
			expected: []PaneSize{
				{WidthPercent: 50.0, HeightPercent: 50.0},
				{WidthPercent: 50.0, HeightPercent: 50.0},
				{WidthPercent: 50.0, HeightPercent: 50.0},
				{WidthPercent: 50.0, HeightPercent: 50.0},
			},
		},
		{
			name:      "grid 6 panes (2x3)",
			mode:      LayoutModeGrid,
			paneCount: 6,
			expected: []PaneSize{
				{WidthPercent: 33.333333333333336, HeightPercent: 50.0},
				{WidthPercent: 33.333333333333336, HeightPercent: 50.0},
				{WidthPercent: 33.333333333333336, HeightPercent: 50.0},
				{WidthPercent: 33.333333333333336, HeightPercent: 50.0},
				{WidthPercent: 33.333333333333336, HeightPercent: 50.0},
				{WidthPercent: 33.333333333333336, HeightPercent: 50.0},
			},
		},
		{
			name:      "grid 9 panes (3x3)",
			mode:      LayoutModeGrid,
			paneCount: 9,
			expected: []PaneSize{
				{WidthPercent: 33.333333333333336, HeightPercent: 33.333333333333336},
				{WidthPercent: 33.333333333333336, HeightPercent: 33.333333333333336},
				{WidthPercent: 33.333333333333336, HeightPercent: 33.333333333333336},
				{WidthPercent: 33.333333333333336, HeightPercent: 33.333333333333336},
				{WidthPercent: 33.333333333333336, HeightPercent: 33.333333333333336},
				{WidthPercent: 33.333333333333336, HeightPercent: 33.333333333333336},
				{WidthPercent: 33.333333333333336, HeightPercent: 33.333333333333336},
				{WidthPercent: 33.333333333333336, HeightPercent: 33.333333333333336},
				{WidthPercent: 33.333333333333336, HeightPercent: 33.333333333333336},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := CalculatePaneSizes(tc.mode, tc.paneCount)
			if tc.expected == nil {
				if result != nil {
					t.Errorf("CalculatePaneSizes(%q, %d) = %+v, want nil",
						tc.mode, tc.paneCount, result)
				}
				return
			}

			if len(result) != len(tc.expected) {
				t.Fatalf("CalculatePaneSizes(%q, %d) returned %d panes, want %d",
					tc.mode, tc.paneCount, len(result), len(tc.expected))
			}

			for i, size := range result {
				if size.WidthPercent != tc.expected[i].WidthPercent ||
					size.HeightPercent != tc.expected[i].HeightPercent {
					t.Errorf("CalculatePaneSizes(%q, %d)[%d] = %+v, want %+v",
						tc.mode, tc.paneCount, i, size, tc.expected[i])
				}
			}
		})
	}
}

func TestCalculatePaneSizes_UnknownMode(t *testing.T) {
	// Unknown mode should fall back to horizontal
	result := CalculatePaneSizes(LayoutMode("unknown"), 2)
	expected := []PaneSize{
		{WidthPercent: 50.0, HeightPercent: 100.0},
		{WidthPercent: 50.0, HeightPercent: 100.0},
	}

	if len(result) != len(expected) {
		t.Fatalf("unknown mode fallback returned %d panes, want %d",
			len(result), len(expected))
	}

	for i, size := range result {
		if size != expected[i] {
			t.Errorf("unknown mode fallback[%d] = %+v, want %+v",
				i, size, expected[i])
		}
	}
}

func TestCalculateWorkerAreaHeight(t *testing.T) {
	tests := []struct {
		name             string
		dashboardEnabled bool
		dashboardHeight  float64
		expected         float64
	}{
		{
			name:             "dashboard disabled returns 100",
			dashboardEnabled: false,
			dashboardHeight:  0,
			expected:         100.0,
		},
		{
			name:             "dashboard disabled ignores height",
			dashboardEnabled: false,
			dashboardHeight:  20,
			expected:         100.0,
		},
		{
			name:             "dashboard enabled with default height",
			dashboardEnabled: true,
			dashboardHeight:  0,
			expected:         85.0, // 100 - DefaultDashboardHeight (15)
		},
		{
			name:             "dashboard enabled with negative height uses default",
			dashboardEnabled: true,
			dashboardHeight:  -5,
			expected:         85.0, // 100 - DefaultDashboardHeight (15)
		},
		{
			name:             "dashboard enabled with custom height",
			dashboardEnabled: true,
			dashboardHeight:  20,
			expected:         80.0,
		},
		{
			name:             "dashboard enabled with 10% height",
			dashboardEnabled: true,
			dashboardHeight:  10,
			expected:         90.0,
		},
		{
			name:             "dashboard enabled with 30% height",
			dashboardEnabled: true,
			dashboardHeight:  30,
			expected:         70.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := CalculateWorkerAreaHeight(tc.dashboardEnabled, tc.dashboardHeight)
			if result != tc.expected {
				t.Errorf("CalculateWorkerAreaHeight(%v, %.1f) = %.1f, want %.1f",
					tc.dashboardEnabled, tc.dashboardHeight, result, tc.expected)
			}
		})
	}
}

func TestAdjustPaneSizesForDashboard(t *testing.T) {
	tests := []struct {
		name             string
		sizes            []PaneSize
		workerAreaHeight float64
		expected         []PaneSize
	}{
		{
			name: "no adjustment when 100% height",
			sizes: []PaneSize{
				{WidthPercent: 50.0, HeightPercent: 100.0},
				{WidthPercent: 50.0, HeightPercent: 100.0},
			},
			workerAreaHeight: 100.0,
			expected: []PaneSize{
				{WidthPercent: 50.0, HeightPercent: 100.0},
				{WidthPercent: 50.0, HeightPercent: 100.0},
			},
		},
		{
			name: "scale to 85% height",
			sizes: []PaneSize{
				{WidthPercent: 50.0, HeightPercent: 100.0},
				{WidthPercent: 50.0, HeightPercent: 100.0},
			},
			workerAreaHeight: 85.0,
			expected: []PaneSize{
				{WidthPercent: 50.0, HeightPercent: 85.0},
				{WidthPercent: 50.0, HeightPercent: 85.0},
			},
		},
		{
			name: "scale grid to 80% height",
			sizes: []PaneSize{
				{WidthPercent: 50.0, HeightPercent: 50.0},
				{WidthPercent: 50.0, HeightPercent: 50.0},
				{WidthPercent: 50.0, HeightPercent: 50.0},
				{WidthPercent: 50.0, HeightPercent: 50.0},
			},
			workerAreaHeight: 80.0,
			expected: []PaneSize{
				{WidthPercent: 50.0, HeightPercent: 40.0},
				{WidthPercent: 50.0, HeightPercent: 40.0},
				{WidthPercent: 50.0, HeightPercent: 40.0},
				{WidthPercent: 50.0, HeightPercent: 40.0},
			},
		},
		{
			name:             "empty sizes returns empty",
			sizes:            []PaneSize{},
			workerAreaHeight: 85.0,
			expected:         []PaneSize{},
		},
		{
			name: "height greater than 100 returns unchanged",
			sizes: []PaneSize{
				{WidthPercent: 100.0, HeightPercent: 100.0},
			},
			workerAreaHeight: 110.0,
			expected: []PaneSize{
				{WidthPercent: 100.0, HeightPercent: 100.0},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := AdjustPaneSizesForDashboard(tc.sizes, tc.workerAreaHeight)

			if len(result) != len(tc.expected) {
				t.Fatalf("AdjustPaneSizesForDashboard returned %d sizes, want %d",
					len(result), len(tc.expected))
			}

			for i, size := range result {
				if size != tc.expected[i] {
					t.Errorf("AdjustPaneSizesForDashboard[%d] = %+v, want %+v",
						i, size, tc.expected[i])
				}
			}
		})
	}
}

func TestBuildSessionConfig(t *testing.T) {
	t.Run("auto layout resolves based on worker count", func(t *testing.T) {
		workers := []WorkerPane{
			{Name: "worker-1", ContainerName: "proj-w1"},
			{Name: "worker-2", ContainerName: "proj-w2"},
		}
		dashboard := DashboardConfig{Enabled: false}

		cfg := BuildSessionConfig("test-session", LayoutModeAuto, workers, dashboard)

		if cfg.Layout != LayoutModeHorizontal {
			t.Errorf("expected horizontal layout for 2 workers, got %s", cfg.Layout)
		}
	})

	t.Run("auto layout with 3+ workers becomes grid", func(t *testing.T) {
		workers := []WorkerPane{
			{Name: "worker-1", ContainerName: "proj-w1"},
			{Name: "worker-2", ContainerName: "proj-w2"},
			{Name: "worker-3", ContainerName: "proj-w3"},
		}
		dashboard := DashboardConfig{Enabled: false}

		cfg := BuildSessionConfig("test-session", LayoutModeAuto, workers, dashboard)

		if cfg.Layout != LayoutModeGrid {
			t.Errorf("expected grid layout for 3 workers, got %s", cfg.Layout)
		}
	})

	t.Run("sets default dashboard height when zero", func(t *testing.T) {
		workers := []WorkerPane{{Name: "worker-1", ContainerName: "proj-w1"}}
		dashboard := DashboardConfig{Enabled: true, HeightPercent: 0}

		cfg := BuildSessionConfig("test-session", LayoutModeHorizontal, workers, dashboard)

		if cfg.Dashboard.HeightPercent != DefaultDashboardHeight {
			t.Errorf("expected default dashboard height %.1f, got %.1f",
				DefaultDashboardHeight, cfg.Dashboard.HeightPercent)
		}
	})

	t.Run("preserves custom dashboard height", func(t *testing.T) {
		workers := []WorkerPane{{Name: "worker-1", ContainerName: "proj-w1"}}
		dashboard := DashboardConfig{Enabled: true, HeightPercent: 25}

		cfg := BuildSessionConfig("test-session", LayoutModeHorizontal, workers, dashboard)

		if cfg.Dashboard.HeightPercent != 25 {
			t.Errorf("expected dashboard height 25, got %.1f",
				cfg.Dashboard.HeightPercent)
		}
	})

	t.Run("copies session name and workers", func(t *testing.T) {
		workers := []WorkerPane{
			{Name: "worker-1", ContainerName: "proj-w1"},
		}
		dashboard := DashboardConfig{Enabled: false}

		cfg := BuildSessionConfig("my-session", LayoutModeVertical, workers, dashboard)

		if cfg.Name != "my-session" {
			t.Errorf("expected session name 'my-session', got %q", cfg.Name)
		}
		if len(cfg.Workers) != 1 {
			t.Fatalf("expected 1 worker, got %d", len(cfg.Workers))
		}
		if cfg.Workers[0].Name != "worker-1" {
			t.Errorf("expected worker name 'worker-1', got %q", cfg.Workers[0].Name)
		}
	})
}

func TestCreateWorkerPanes(t *testing.T) {
	t.Run("creates panes from container names", func(t *testing.T) {
		names := []string{"proj-w1", "proj-w2", "proj-w3"}
		panes := CreateWorkerPanes(names)

		if len(panes) != 3 {
			t.Fatalf("expected 3 panes, got %d", len(panes))
		}

		for i, pane := range panes {
			if pane.Name != names[i] {
				t.Errorf("pane[%d].Name = %q, want %q", i, pane.Name, names[i])
			}
			if pane.ContainerName != names[i] {
				t.Errorf("pane[%d].ContainerName = %q, want %q",
					i, pane.ContainerName, names[i])
			}
		}
	})

	t.Run("empty slice returns empty", func(t *testing.T) {
		panes := CreateWorkerPanes([]string{})
		if len(panes) != 0 {
			t.Errorf("expected empty slice, got %d panes", len(panes))
		}
	})

	t.Run("nil slice returns empty", func(t *testing.T) {
		panes := CreateWorkerPanes(nil)
		if len(panes) != 0 {
			t.Errorf("expected empty slice, got %d panes", len(panes))
		}
	})
}

func TestLayoutMode_IsValid(t *testing.T) {
	validModes := []LayoutMode{
		LayoutModeAuto,
		LayoutModeHorizontal,
		LayoutModeVertical,
		LayoutModeGrid,
	}

	for _, mode := range validModes {
		t.Run(string(mode), func(t *testing.T) {
			if !mode.IsValid() {
				t.Errorf("expected %q to be valid", mode)
			}
		})
	}

	invalidModes := []LayoutMode{
		"invalid",
		"",
		"HORIZONTAL",
		"Auto",
	}

	for _, mode := range invalidModes {
		t.Run(string(mode)+"_invalid", func(t *testing.T) {
			if mode.IsValid() {
				t.Errorf("expected %q to be invalid", mode)
			}
		})
	}
}

func TestConstants(t *testing.T) {
	if DefaultDashboardHeight != 15.0 {
		t.Errorf("expected DefaultDashboardHeight=15.0, got %.1f", DefaultDashboardHeight)
	}
	if MinPaneSize != 5.0 {
		t.Errorf("expected MinPaneSize=5.0, got %.1f", MinPaneSize)
	}
}
