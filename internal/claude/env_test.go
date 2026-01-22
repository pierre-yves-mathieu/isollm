package claude

import (
	"strings"
	"testing"
)

func TestBuildEnvironment(t *testing.T) {
	tests := []struct {
		name         string
		host         string
		port         int
		project      string
		agent        string
		projectPath  string
		bareRepoPath string
	}{
		{
			name:         "basic environment",
			host:         "192.168.1.1",
			port:         7432,
			project:      "myproject",
			agent:        "worker-01",
			projectPath:  "/home/dev/project",
			bareRepoPath: "/repo.git",
		},
		{
			name:         "localhost with default port",
			host:         "localhost",
			port:         7432,
			project:      "testproj",
			agent:        "test-agent",
			projectPath:  "/workspace",
			bareRepoPath: "/bare.git",
		},
		{
			name:         "custom paths",
			host:         "10.0.0.1",
			port:         8080,
			project:      "custom-project",
			agent:        "custom-agent-01",
			projectPath:  "/custom/path/to/project",
			bareRepoPath: "/custom/bare/repo.git",
		},
		{
			name:         "empty values",
			host:         "",
			port:         0,
			project:      "",
			agent:        "",
			projectPath:  "",
			bareRepoPath: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := BuildEnvironment(tt.host, tt.port, tt.project, tt.agent, tt.projectPath, tt.bareRepoPath)

			if env == nil {
				t.Fatal("BuildEnvironment() returned nil")
			}
			if env.AiryraHost != tt.host {
				t.Errorf("AiryraHost = %q, want %q", env.AiryraHost, tt.host)
			}
			if env.AiryraPort != tt.port {
				t.Errorf("AiryraPort = %d, want %d", env.AiryraPort, tt.port)
			}
			if env.AiryraProject != tt.project {
				t.Errorf("AiryraProject = %q, want %q", env.AiryraProject, tt.project)
			}
			if env.AiryraAgent != tt.agent {
				t.Errorf("AiryraAgent = %q, want %q", env.AiryraAgent, tt.agent)
			}
			if env.ProjectPath != tt.projectPath {
				t.Errorf("ProjectPath = %q, want %q", env.ProjectPath, tt.projectPath)
			}
			if env.BareRepoPath != tt.bareRepoPath {
				t.Errorf("BareRepoPath = %q, want %q", env.BareRepoPath, tt.bareRepoPath)
			}
		})
	}
}

func TestToEnvVars(t *testing.T) {
	tests := []struct {
		name     string
		env      *Environment
		expected []string
	}{
		{
			name: "basic environment",
			env: &Environment{
				AiryraHost:    "192.168.1.1",
				AiryraPort:    7432,
				AiryraProject: "myproject",
				AiryraAgent:   "worker-01",
				ProjectPath:   "/home/dev/project",
				BareRepoPath:  "/repo.git",
			},
			expected: []string{
				"AIRYRA_HOST=192.168.1.1",
				"AIRYRA_PORT=7432",
				"AIRYRA_PROJECT=myproject",
				"AIRYRA_AGENT=worker-01",
				"ISOLLM_PROJECT_PATH=/home/dev/project",
				"ISOLLM_BARE_REPO=/repo.git",
			},
		},
		{
			name: "empty values",
			env: &Environment{
				AiryraHost:    "",
				AiryraPort:    0,
				AiryraProject: "",
				AiryraAgent:   "",
				ProjectPath:   "",
				BareRepoPath:  "",
			},
			expected: []string{
				"AIRYRA_HOST=",
				"AIRYRA_PORT=0",
				"AIRYRA_PROJECT=",
				"AIRYRA_AGENT=",
				"ISOLLM_PROJECT_PATH=",
				"ISOLLM_BARE_REPO=",
			},
		},
		{
			name: "special characters in values",
			env: &Environment{
				AiryraHost:    "host.with.dots",
				AiryraPort:    8080,
				AiryraProject: "project-with-dashes",
				AiryraAgent:   "agent_with_underscores",
				ProjectPath:   "/path/with/slashes",
				BareRepoPath:  "/path.with.dots.git",
			},
			expected: []string{
				"AIRYRA_HOST=host.with.dots",
				"AIRYRA_PORT=8080",
				"AIRYRA_PROJECT=project-with-dashes",
				"AIRYRA_AGENT=agent_with_underscores",
				"ISOLLM_PROJECT_PATH=/path/with/slashes",
				"ISOLLM_BARE_REPO=/path.with.dots.git",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.env.ToEnvVars()

			if len(result) != len(tt.expected) {
				t.Errorf("ToEnvVars() returned %d items, want %d", len(result), len(tt.expected))
			}

			for i, expected := range tt.expected {
				if i >= len(result) {
					t.Errorf("Missing env var at index %d: %s", i, expected)
					continue
				}
				if result[i] != expected {
					t.Errorf("ToEnvVars()[%d] = %q, want %q", i, result[i], expected)
				}
			}
		})
	}
}

func TestToEnvVars_ContainsAllKeys(t *testing.T) {
	env := &Environment{
		AiryraHost:    "test",
		AiryraPort:    1234,
		AiryraProject: "proj",
		AiryraAgent:   "agent",
		ProjectPath:   "/path",
		BareRepoPath:  "/repo",
	}

	result := env.ToEnvVars()

	requiredKeys := []string{
		"AIRYRA_HOST=",
		"AIRYRA_PORT=",
		"AIRYRA_PROJECT=",
		"AIRYRA_AGENT=",
		"ISOLLM_PROJECT_PATH=",
		"ISOLLM_BARE_REPO=",
	}

	for _, key := range requiredKeys {
		found := false
		for _, envVar := range result {
			if strings.HasPrefix(envVar, key) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ToEnvVars() missing key starting with %q", key)
		}
	}
}

func TestToEnvFile(t *testing.T) {
	tests := []struct {
		name              string
		env               *Environment
		expectedContains  []string
		expectedNotContain []string
	}{
		{
			name: "basic environment",
			env: &Environment{
				AiryraHost:    "192.168.1.1",
				AiryraPort:    7432,
				AiryraProject: "myproject",
				AiryraAgent:   "worker-01",
				ProjectPath:   "/home/dev/project",
				BareRepoPath:  "/repo.git",
			},
			expectedContains: []string{
				"# isollm environment variables",
				"export AIRYRA_HOST=",
				"export AIRYRA_PORT=7432",
				"export AIRYRA_PROJECT=",
				"export AIRYRA_AGENT=",
				"export ISOLLM_PROJECT_PATH=",
				"export ISOLLM_BARE_REPO=",
				"192.168.1.1",
				"myproject",
				"worker-01",
				"/home/dev/project",
				"/repo.git",
			},
		},
		{
			name: "values are quoted",
			env: &Environment{
				AiryraHost:    "test-host",
				AiryraPort:    1234,
				AiryraProject: "test-project",
				AiryraAgent:   "test-agent",
				ProjectPath:   "/test/path",
				BareRepoPath:  "/test/repo.git",
			},
			expectedContains: []string{
				`export AIRYRA_HOST="test-host"`,
				`export AIRYRA_PROJECT="test-project"`,
				`export AIRYRA_AGENT="test-agent"`,
				`export ISOLLM_PROJECT_PATH="/test/path"`,
				`export ISOLLM_BARE_REPO="/test/repo.git"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.env.ToEnvFile()

			for _, expected := range tt.expectedContains {
				if !strings.Contains(result, expected) {
					t.Errorf("ToEnvFile() missing content: %q", expected)
				}
			}

			for _, notExpected := range tt.expectedNotContain {
				if strings.Contains(result, notExpected) {
					t.Errorf("ToEnvFile() should not contain: %q", notExpected)
				}
			}
		})
	}
}

func TestToEnvFile_IsShellSourceable(t *testing.T) {
	env := &Environment{
		AiryraHost:    "localhost",
		AiryraPort:    7432,
		AiryraProject: "test",
		AiryraAgent:   "agent",
		ProjectPath:   "/path",
		BareRepoPath:  "/repo",
	}

	result := env.ToEnvFile()

	// Each export line should be on its own line
	lines := strings.Split(result, "\n")
	exportCount := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "export ") {
			exportCount++
			// Check that the line contains = for assignment
			if !strings.Contains(line, "=") {
				t.Errorf("export line missing assignment: %q", line)
			}
		}
	}

	// Should have 6 export statements
	if exportCount != 6 {
		t.Errorf("ToEnvFile() has %d export statements, want 6", exportCount)
	}
}

func TestToEnvFile_HasHeader(t *testing.T) {
	env := &Environment{
		AiryraHost:    "localhost",
		AiryraPort:    7432,
		AiryraProject: "test",
		AiryraAgent:   "agent",
		ProjectPath:   "/path",
		BareRepoPath:  "/repo",
	}

	result := env.ToEnvFile()

	// First line should be a comment
	lines := strings.Split(result, "\n")
	if len(lines) == 0 {
		t.Fatal("ToEnvFile() returned empty content")
	}
	if !strings.HasPrefix(lines[0], "#") {
		t.Errorf("ToEnvFile() first line should be a comment, got: %q", lines[0])
	}
}

func TestGetHostIP(t *testing.T) {
	// This test may need to be skipped in CI environments without network
	// interfaces or LXC bridge
	ip, err := GetHostIP()

	// If we can't get the host IP, that's ok - it depends on the environment
	if err != nil {
		t.Skipf("GetHostIP() returned error (expected in CI): %v", err)
	}

	// If we got an IP, verify it looks valid
	if ip == "" {
		t.Error("GetHostIP() returned empty IP without error")
	}

	// Basic IP format check (should contain dots for IPv4)
	if !strings.Contains(ip, ".") {
		t.Errorf("GetHostIP() = %q, doesn't look like an IPv4 address", ip)
	}

	// Should have 4 octets
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		t.Errorf("GetHostIP() = %q, expected 4 octets, got %d", ip, len(parts))
	}
}

func TestGetHostIP_NotEmpty(t *testing.T) {
	ip, err := GetHostIP()

	if err == nil && ip == "" {
		t.Error("GetHostIP() returned empty string with nil error")
	}
}

func TestDefaultBridgeInterface(t *testing.T) {
	// Verify the constant is set
	if DefaultBridgeInterface == "" {
		t.Error("DefaultBridgeInterface is empty")
	}

	// Should be the expected LXC bridge name
	if DefaultBridgeInterface != "lxdbr0" {
		t.Errorf("DefaultBridgeInterface = %q, want %q", DefaultBridgeInterface, "lxdbr0")
	}
}
