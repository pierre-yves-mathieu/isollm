package zellij

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateKDL(t *testing.T) {
	t.Run("basic horizontal layout", func(t *testing.T) {
		cfg := SessionConfig{
			Name:   "test-session",
			Layout: LayoutModeHorizontal,
			Workers: []WorkerPane{
				{Name: "worker-1", ContainerName: "proj-w1"},
				{Name: "worker-2", ContainerName: "proj-w2"},
			},
			Dashboard: DashboardConfig{Enabled: false},
		}

		kdl := GenerateKDL(cfg)

		// Check structure
		if !strings.Contains(kdl, "layout {") {
			t.Error("expected 'layout {' in output")
		}
		if !strings.Contains(kdl, `tab name="workers"`) {
			t.Error("expected tab name 'workers' in output")
		}
		if !strings.Contains(kdl, `split_direction="vertical"`) {
			t.Error("expected vertical split direction for horizontal layout")
		}
		if !strings.Contains(kdl, `name "worker-1"`) {
			t.Error("expected worker-1 name in output")
		}
		if !strings.Contains(kdl, `name "worker-2"`) {
			t.Error("expected worker-2 name in output")
		}
		if !strings.Contains(kdl, `command "lxc"`) {
			t.Error("expected lxc command in output")
		}
		if !strings.Contains(kdl, `"proj-w1"`) {
			t.Error("expected container name proj-w1 in args")
		}
	})

	t.Run("vertical layout", func(t *testing.T) {
		cfg := SessionConfig{
			Name:   "test-session",
			Layout: LayoutModeVertical,
			Workers: []WorkerPane{
				{Name: "worker-1", ContainerName: "proj-w1"},
			},
			Dashboard: DashboardConfig{Enabled: false},
		}

		kdl := GenerateKDL(cfg)

		if !strings.Contains(kdl, `split_direction="horizontal"`) {
			t.Error("expected horizontal split direction for vertical layout")
		}
	})
}

func TestGenerateKDL_WithDashboard(t *testing.T) {
	t.Run("dashboard with command", func(t *testing.T) {
		cfg := SessionConfig{
			Name:   "test-session",
			Layout: LayoutModeHorizontal,
			Workers: []WorkerPane{
				{Name: "worker-1", ContainerName: "proj-w1"},
			},
			Dashboard: DashboardConfig{
				Enabled:       true,
				HeightPercent: 15,
				Command:       "htop",
			},
		}

		kdl := GenerateKDL(cfg)

		if !strings.Contains(kdl, `size="15%"`) {
			t.Error("expected dashboard size 15% in output")
		}
		if !strings.Contains(kdl, `name "dashboard"`) {
			t.Error("expected dashboard name in output")
		}
		if !strings.Contains(kdl, `command "htop"`) {
			t.Error("expected dashboard command in output")
		}
	})

	t.Run("dashboard with command and args", func(t *testing.T) {
		cfg := SessionConfig{
			Name:   "test-session",
			Layout: LayoutModeHorizontal,
			Workers: []WorkerPane{
				{Name: "worker-1", ContainerName: "proj-w1"},
			},
			Dashboard: DashboardConfig{
				Enabled:       true,
				HeightPercent: 20,
				Command:       "watch",
				Args:          []string{"-n", "1", "date"},
			},
		}

		kdl := GenerateKDL(cfg)

		if !strings.Contains(kdl, `size="20%"`) {
			t.Error("expected dashboard size 20% in output")
		}
		if !strings.Contains(kdl, `command "watch"`) {
			t.Error("expected dashboard command 'watch' in output")
		}
		if !strings.Contains(kdl, `args "-n" "1" "date"`) {
			t.Error("expected dashboard args in output")
		}
	})

	t.Run("dashboard without command", func(t *testing.T) {
		cfg := SessionConfig{
			Name:   "test-session",
			Layout: LayoutModeHorizontal,
			Workers: []WorkerPane{
				{Name: "worker-1", ContainerName: "proj-w1"},
			},
			Dashboard: DashboardConfig{
				Enabled:       true,
				HeightPercent: 15,
				Command:       "",
			},
		}

		kdl := GenerateKDL(cfg)

		// Dashboard should appear but without a command
		if !strings.Contains(kdl, `name "dashboard"`) {
			t.Error("expected dashboard name in output")
		}
		// Should not have command line after dashboard name
		lines := strings.Split(kdl, "\n")
		foundDashboard := false
		for i, line := range lines {
			if strings.Contains(line, `name "dashboard"`) {
				foundDashboard = true
				// Check the lines after dashboard name until we hit the closing brace
				for j := i + 1; j < len(lines); j++ {
					if strings.Contains(lines[j], "}") {
						break
					}
					if strings.Contains(lines[j], "command") {
						t.Error("unexpected command after dashboard without command set")
					}
				}
			}
		}
		if !foundDashboard {
			t.Error("did not find dashboard in output")
		}
	})
}

func TestGenerateKDL_SingleWorker(t *testing.T) {
	cfg := SessionConfig{
		Name:   "single-worker-session",
		Layout: LayoutModeHorizontal,
		Workers: []WorkerPane{
			{Name: "worker-1", ContainerName: "myproj-w1"},
		},
		Dashboard: DashboardConfig{Enabled: false},
	}

	kdl := GenerateKDL(cfg)

	// Should have the basic structure
	if !strings.Contains(kdl, "layout {") {
		t.Error("expected 'layout {' in output")
	}

	// Should have exactly one worker pane
	count := strings.Count(kdl, `command "lxc"`)
	if count != 1 {
		t.Errorf("expected 1 lxc command, got %d", count)
	}

	// First pane should have focus
	if !strings.Contains(kdl, "focus true") {
		t.Error("expected focus true for single worker")
	}

	// Should reference the container name
	if !strings.Contains(kdl, `"myproj-w1"`) {
		t.Error("expected container name in output")
	}
}

func TestGenerateKDL_GridLayout(t *testing.T) {
	t.Run("4 workers in 2x2 grid", func(t *testing.T) {
		cfg := SessionConfig{
			Name:   "grid-session",
			Layout: LayoutModeGrid,
			Workers: []WorkerPane{
				{Name: "worker-1", ContainerName: "proj-w1"},
				{Name: "worker-2", ContainerName: "proj-w2"},
				{Name: "worker-3", ContainerName: "proj-w3"},
				{Name: "worker-4", ContainerName: "proj-w4"},
			},
			Dashboard: DashboardConfig{Enabled: false},
		}

		kdl := GenerateKDL(cfg)

		// Grid layout should have nested structure
		// Outer horizontal split for rows, inner vertical splits for columns
		horizontalCount := strings.Count(kdl, `split_direction="horizontal"`)
		verticalCount := strings.Count(kdl, `split_direction="vertical"`)

		if horizontalCount < 1 {
			t.Error("expected at least one horizontal split for grid rows")
		}
		if verticalCount < 1 {
			t.Error("expected at least one vertical split for grid columns")
		}

		// All 4 workers should be present
		for i := 1; i <= 4; i++ {
			workerName := "worker-" + string(rune('0'+i))
			if !strings.Contains(kdl, `name "worker-`+string(rune('0'+i))+`"`) {
				t.Errorf("expected %s in output", workerName)
			}
		}

		// Only first worker should have focus
		focusCount := strings.Count(kdl, "focus true")
		if focusCount != 1 {
			t.Errorf("expected exactly 1 focus true, got %d", focusCount)
		}
	})

	t.Run("3 workers in grid", func(t *testing.T) {
		cfg := SessionConfig{
			Name:   "grid-session-3",
			Layout: LayoutModeGrid,
			Workers: []WorkerPane{
				{Name: "worker-1", ContainerName: "proj-w1"},
				{Name: "worker-2", ContainerName: "proj-w2"},
				{Name: "worker-3", ContainerName: "proj-w3"},
			},
			Dashboard: DashboardConfig{Enabled: false},
		}

		kdl := GenerateKDL(cfg)

		// All 3 workers should be present
		lxcCount := strings.Count(kdl, `command "lxc"`)
		if lxcCount != 3 {
			t.Errorf("expected 3 lxc commands for 3 workers, got %d", lxcCount)
		}
	})
}

func TestGenerateKDL_EmptyWorkers(t *testing.T) {
	layouts := []LayoutMode{
		LayoutModeHorizontal,
		LayoutModeVertical,
		LayoutModeGrid,
	}

	for _, layout := range layouts {
		t.Run(string(layout), func(t *testing.T) {
			cfg := SessionConfig{
				Name:      "empty-workers",
				Layout:    layout,
				Workers:   []WorkerPane{},
				Dashboard: DashboardConfig{Enabled: false},
			}

			kdl := GenerateKDL(cfg)

			// Should still have basic structure
			if !strings.Contains(kdl, "layout {") {
				t.Error("expected 'layout {' in output")
			}
			if !strings.Contains(kdl, `tab name="workers"`) {
				t.Error("expected tab in output")
			}

			// Should not have any lxc commands
			if strings.Contains(kdl, `command "lxc"`) {
				t.Error("unexpected lxc command with no workers")
			}
		})
	}
}

func TestGenerateKDL_DefaultLayout(t *testing.T) {
	// Test unknown layout defaults to horizontal
	cfg := SessionConfig{
		Name:   "test-session",
		Layout: LayoutMode("unknown"),
		Workers: []WorkerPane{
			{Name: "worker-1", ContainerName: "proj-w1"},
		},
		Dashboard: DashboardConfig{Enabled: false},
	}

	kdl := GenerateKDL(cfg)

	// Should fall back to horizontal (vertical split)
	if !strings.Contains(kdl, `split_direction="vertical"`) {
		t.Error("expected vertical split direction for default horizontal layout")
	}
}

func TestEscapeKDLString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no special characters",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "backslash",
			input:    `hello\world`,
			expected: `hello\\world`,
		},
		{
			name:     "double quote",
			input:    `hello "world"`,
			expected: `hello \"world\"`,
		},
		{
			name:     "newline",
			input:    "hello\nworld",
			expected: `hello\nworld`,
		},
		{
			name:     "tab",
			input:    "hello\tworld",
			expected: `hello\tworld`,
		},
		{
			name:     "multiple special characters",
			input:    "hello\n\"world\"\t\\end",
			expected: `hello\n\"world\"\t\\end`,
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only backslashes",
			input:    `\\`,
			expected: `\\\\`,
		},
		{
			name:     "only quotes",
			input:    `""`,
			expected: `\"\"`,
		},
		{
			name:     "path with backslashes",
			input:    `C:\Users\test\file.txt`,
			expected: `C:\\Users\\test\\file.txt`,
		},
		{
			name:     "json-like string",
			input:    `{"key": "value"}`,
			expected: `{\"key\": \"value\"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := escapeKDLString(tc.input)
			if result != tc.expected {
				t.Errorf("escapeKDLString(%q) = %q, want %q",
					tc.input, result, tc.expected)
			}
		})
	}
}

func TestWriteLayoutFile(t *testing.T) {
	t.Run("writes file to directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		cfg := SessionConfig{
			Name:   "test-session",
			Layout: LayoutModeHorizontal,
			Workers: []WorkerPane{
				{Name: "worker-1", ContainerName: "proj-w1"},
			},
			Dashboard: DashboardConfig{Enabled: false},
		}

		path, err := WriteLayoutFile(cfg, tmpDir)
		if err != nil {
			t.Fatalf("WriteLayoutFile failed: %v", err)
		}

		// Check path
		expectedPath := filepath.Join(tmpDir, "test-session.kdl")
		if path != expectedPath {
			t.Errorf("expected path %q, got %q", expectedPath, path)
		}

		// Check file exists
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Error("layout file was not created")
		}

		// Check content
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read layout file: %v", err)
		}

		if !strings.Contains(string(content), "layout {") {
			t.Error("layout file does not contain expected content")
		}
	})

	t.Run("creates directory if not exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		nestedDir := filepath.Join(tmpDir, "nested", "layouts")

		cfg := SessionConfig{
			Name:   "nested-session",
			Layout: LayoutModeHorizontal,
			Workers: []WorkerPane{
				{Name: "worker-1", ContainerName: "proj-w1"},
			},
		}

		path, err := WriteLayoutFile(cfg, nestedDir)
		if err != nil {
			t.Fatalf("WriteLayoutFile failed: %v", err)
		}

		// Check file exists
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Error("layout file was not created in nested directory")
		}
	})

	t.Run("generates correct filename from session name", func(t *testing.T) {
		tmpDir := t.TempDir()

		cfg := SessionConfig{
			Name:    "my-project-session",
			Layout:  LayoutModeHorizontal,
			Workers: []WorkerPane{},
		}

		path, err := WriteLayoutFile(cfg, tmpDir)
		if err != nil {
			t.Fatalf("WriteLayoutFile failed: %v", err)
		}

		if !strings.HasSuffix(path, "my-project-session.kdl") {
			t.Errorf("expected filename 'my-project-session.kdl', got %q",
				filepath.Base(path))
		}
	})

	t.Run("overwrites existing file", func(t *testing.T) {
		tmpDir := t.TempDir()

		cfg := SessionConfig{
			Name:   "overwrite-test",
			Layout: LayoutModeHorizontal,
			Workers: []WorkerPane{
				{Name: "worker-1", ContainerName: "proj-w1"},
			},
		}

		// Write first time
		path1, err := WriteLayoutFile(cfg, tmpDir)
		if err != nil {
			t.Fatalf("first WriteLayoutFile failed: %v", err)
		}

		// Modify config
		cfg.Workers = []WorkerPane{
			{Name: "worker-1", ContainerName: "proj-w1"},
			{Name: "worker-2", ContainerName: "proj-w2"},
		}

		// Write second time
		path2, err := WriteLayoutFile(cfg, tmpDir)
		if err != nil {
			t.Fatalf("second WriteLayoutFile failed: %v", err)
		}

		if path1 != path2 {
			t.Errorf("expected same path, got %q and %q", path1, path2)
		}

		// Check content has new worker
		content, err := os.ReadFile(path2)
		if err != nil {
			t.Fatalf("failed to read layout file: %v", err)
		}

		if !strings.Contains(string(content), "worker-2") {
			t.Error("overwritten file should contain worker-2")
		}
	})
}

func TestWriteLayoutToTemp(t *testing.T) {
	cfg := SessionConfig{
		Name:   "temp-test-session",
		Layout: LayoutModeHorizontal,
		Workers: []WorkerPane{
			{Name: "worker-1", ContainerName: "proj-w1"},
		},
	}

	path, err := WriteLayoutToTemp(cfg)
	if err != nil {
		t.Fatalf("WriteLayoutToTemp failed: %v", err)
	}

	// Clean up
	defer os.Remove(path)

	// Check file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("temp layout file was not created")
	}

	// Check path is in temp directory
	if !strings.Contains(path, os.TempDir()) && !strings.Contains(path, "isollm-layouts") {
		t.Errorf("expected path in temp directory with isollm-layouts, got %q", path)
	}

	// Check filename
	if !strings.HasSuffix(path, "temp-test-session.kdl") {
		t.Errorf("expected filename 'temp-test-session.kdl', got %q",
			filepath.Base(path))
	}
}

func TestRemoveLayoutFile(t *testing.T) {
	t.Run("removes existing file", func(t *testing.T) {
		tmpDir := t.TempDir()
		tmpFile := filepath.Join(tmpDir, "test.kdl")

		// Create the file
		if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		// Remove it
		err := RemoveLayoutFile(tmpFile)
		if err != nil {
			t.Errorf("RemoveLayoutFile failed: %v", err)
		}

		// Verify it's gone
		if _, err := os.Stat(tmpFile); !os.IsNotExist(err) {
			t.Error("file should have been removed")
		}
	})

	t.Run("no error for non-existent file", func(t *testing.T) {
		err := RemoveLayoutFile("/nonexistent/path/file.kdl")
		if err != nil {
			t.Errorf("RemoveLayoutFile should not error for non-existent file: %v", err)
		}
	})

	t.Run("no error for empty path", func(t *testing.T) {
		err := RemoveLayoutFile("")
		if err != nil {
			t.Errorf("RemoveLayoutFile should not error for empty path: %v", err)
		}
	})
}

func TestGenerateKDL_WorkerPaneStructure(t *testing.T) {
	cfg := SessionConfig{
		Name:   "structure-test",
		Layout: LayoutModeHorizontal,
		Workers: []WorkerPane{
			{Name: "my-worker", ContainerName: "myproj-container"},
		},
		Dashboard: DashboardConfig{Enabled: false},
	}

	kdl := GenerateKDL(cfg)

	// Verify lxc exec command structure
	if !strings.Contains(kdl, `command "lxc"`) {
		t.Error("expected lxc command")
	}

	// Verify args include exec, container name, --, su, -l, dev
	if !strings.Contains(kdl, `"exec"`) {
		t.Error("expected 'exec' in args")
	}
	if !strings.Contains(kdl, `"myproj-container"`) {
		t.Error("expected container name in args")
	}
	if !strings.Contains(kdl, `"--"`) {
		t.Error("expected '--' in args")
	}
	if !strings.Contains(kdl, `"su"`) {
		t.Error("expected 'su' in args")
	}
	if !strings.Contains(kdl, `"-l"`) {
		t.Error("expected '-l' in args")
	}
	if !strings.Contains(kdl, `"dev"`) {
		t.Error("expected 'dev' in args")
	}
}

func TestGenerateKDL_FocusOnFirstPane(t *testing.T) {
	cfg := SessionConfig{
		Name:   "focus-test",
		Layout: LayoutModeHorizontal,
		Workers: []WorkerPane{
			{Name: "worker-1", ContainerName: "proj-w1"},
			{Name: "worker-2", ContainerName: "proj-w2"},
			{Name: "worker-3", ContainerName: "proj-w3"},
		},
		Dashboard: DashboardConfig{Enabled: false},
	}

	kdl := GenerateKDL(cfg)

	// Should have exactly one focus true
	focusCount := strings.Count(kdl, "focus true")
	if focusCount != 1 {
		t.Errorf("expected exactly 1 'focus true', got %d", focusCount)
	}

	// The focus should be on worker-1 (first worker)
	// Find the position of "worker-1" and "focus true"
	worker1Pos := strings.Index(kdl, `name "worker-1"`)
	worker2Pos := strings.Index(kdl, `name "worker-2"`)
	focusPos := strings.Index(kdl, "focus true")

	if worker1Pos == -1 || focusPos == -1 {
		t.Fatal("could not find required strings in output")
	}

	// Focus should be between worker-1 and worker-2
	if focusPos < worker1Pos || focusPos > worker2Pos {
		t.Error("focus should be associated with first worker pane")
	}
}
