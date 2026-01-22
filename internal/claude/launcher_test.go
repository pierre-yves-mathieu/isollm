package claude

import (
	"errors"
	"strings"
	"testing"

	"isollm/internal/config"
)

// MockContainerExecer implements ContainerExecer for testing
type MockContainerExecer struct {
	// ExecCalls records all calls to Exec
	ExecCalls []ExecCall
	// ExecFunc allows customizing behavior per test
	ExecFunc func(name string, cmd []string) ([]byte, error)
	// ExecError if set, all Exec calls return this error
	ExecError error
	// ExecOutput if set, all Exec calls return this output
	ExecOutput []byte
}

// ExecCall records a single Exec call
type ExecCall struct {
	Name string
	Cmd  []string
}

// NewMockContainerExecer creates a new mock execer with default behavior
func NewMockContainerExecer() *MockContainerExecer {
	return &MockContainerExecer{
		ExecCalls: make([]ExecCall, 0),
	}
}

// Exec implements ContainerExecer
func (m *MockContainerExecer) Exec(name string, cmd []string) ([]byte, error) {
	m.ExecCalls = append(m.ExecCalls, ExecCall{Name: name, Cmd: cmd})

	if m.ExecFunc != nil {
		return m.ExecFunc(name, cmd)
	}

	if m.ExecError != nil {
		return nil, m.ExecError
	}

	return m.ExecOutput, nil
}

// Reset clears recorded calls
func (m *MockContainerExecer) Reset() {
	m.ExecCalls = make([]ExecCall, 0)
}

// CallCount returns the number of Exec calls
func (m *MockContainerExecer) CallCount() int {
	return len(m.ExecCalls)
}

// LastCall returns the last Exec call or nil if none
func (m *MockContainerExecer) LastCall() *ExecCall {
	if len(m.ExecCalls) == 0 {
		return nil
	}
	return &m.ExecCalls[len(m.ExecCalls)-1]
}

// testConfig creates a config for testing
func testConfig() *config.Config {
	return &config.Config{
		Project: "test-project",
		Workers: 3,
		Image:   "ubuntu:24.04",
		Git: config.GitConfig{
			BaseBranch:   "main",
			BranchPrefix: "isollm/",
		},
		Claude: config.ClaudeConfig{
			Command: "claude",
			Args:    []string{"--dangerously-skip-permissions"},
		},
		Airyra: config.AiryraConfig{
			Project: "test-project",
			Host:    "192.168.1.1",
			Port:    7432,
		},
	}
}

func TestNewLauncher(t *testing.T) {
	mock := NewMockContainerExecer()
	cfg := testConfig()

	launcher, err := NewLauncher(cfg, mock)

	if err != nil {
		t.Fatalf("NewLauncher() error = %v", err)
	}
	if launcher == nil {
		t.Fatal("NewLauncher() returned nil")
	}
}

func TestNewLauncher_ConfigStored(t *testing.T) {
	mock := NewMockContainerExecer()
	cfg := testConfig()

	launcher, err := NewLauncher(cfg, mock)
	if err != nil {
		t.Fatalf("NewLauncher() error = %v", err)
	}

	// Verify config is accessible through GetLaunchCommand
	cmd := launcher.GetLaunchCommand()
	if len(cmd) == 0 {
		t.Error("GetLaunchCommand() returned empty command")
	}
	if cmd[0] != cfg.Claude.Command {
		t.Errorf("GetLaunchCommand()[0] = %q, want %q", cmd[0], cfg.Claude.Command)
	}
}

func TestNewLauncher_HostIPFallback(t *testing.T) {
	mock := NewMockContainerExecer()
	cfg := testConfig()
	cfg.Airyra.Host = "fallback-host"

	launcher, err := NewLauncher(cfg, mock)
	if err != nil {
		t.Fatalf("NewLauncher() error = %v", err)
	}

	// The launcher should have a host IP (either from GetHostIP or fallback)
	hostIP := launcher.GetHostIP()
	if hostIP == "" {
		t.Error("Launcher has empty hostIP")
	}
}

func TestPrepareWorker(t *testing.T) {
	mock := NewMockContainerExecer()
	// Default behavior: return "missing" for bashrc check to trigger append
	mock.ExecFunc = func(name string, cmd []string) ([]byte, error) {
		cmdStr := strings.Join(cmd, " ")
		if strings.Contains(cmdStr, "grep -q") {
			return []byte("missing\n"), nil
		}
		return []byte(""), nil
	}

	cfg := testConfig()
	launcher, _ := NewLauncher(cfg, mock)

	err := launcher.PrepareWorker("worker-01", "isollm/task-123")

	if err != nil {
		t.Fatalf("PrepareWorker() error = %v", err)
	}

	// Should have made multiple exec calls
	if mock.CallCount() == 0 {
		t.Error("PrepareWorker() made no exec calls")
	}

	// Check that env file was written
	foundEnvWrite := false
	for _, call := range mock.ExecCalls {
		if call.Name == "worker-01" {
			cmdStr := strings.Join(call.Cmd, " ")
			if strings.Contains(cmdStr, EnvFilePath) && strings.Contains(cmdStr, "printf") {
				foundEnvWrite = true
				break
			}
		}
	}
	if !foundEnvWrite {
		t.Error("PrepareWorker() did not write env file")
	}
}

func TestPrepareWorker_WritesCLAUDEMD(t *testing.T) {
	mock := NewMockContainerExecer()
	mock.ExecFunc = func(name string, cmd []string) ([]byte, error) {
		cmdStr := strings.Join(cmd, " ")
		if strings.Contains(cmdStr, "grep -q") {
			return []byte("missing\n"), nil
		}
		return []byte(""), nil
	}

	cfg := testConfig()
	launcher, _ := NewLauncher(cfg, mock)

	launcher.PrepareWorker("worker-01", "isollm/task-123")

	// Check that CLAUDE.md was written
	foundCLAUDEMD := false
	for _, call := range mock.ExecCalls {
		cmdStr := strings.Join(call.Cmd, " ")
		if strings.Contains(cmdStr, "CLAUDE.md") && strings.Contains(cmdStr, "printf") {
			foundCLAUDEMD = true
			break
		}
	}
	if !foundCLAUDEMD {
		t.Error("PrepareWorker() did not write CLAUDE.md")
	}
}

func TestPrepareWorker_SetupsBashrc(t *testing.T) {
	mock := NewMockContainerExecer()
	mock.ExecFunc = func(name string, cmd []string) ([]byte, error) {
		cmdStr := strings.Join(cmd, " ")
		if strings.Contains(cmdStr, "grep -q") {
			return []byte("missing\n"), nil
		}
		return []byte(""), nil
	}

	cfg := testConfig()
	launcher, _ := NewLauncher(cfg, mock)

	launcher.PrepareWorker("worker-01", "")

	// Check that bashrc setup was done
	foundBashrcCheck := false
	foundBashrcAppend := false
	for _, call := range mock.ExecCalls {
		cmdStr := strings.Join(call.Cmd, " ")
		if strings.Contains(cmdStr, ".bashrc") && strings.Contains(cmdStr, "grep") {
			foundBashrcCheck = true
		}
		if strings.Contains(cmdStr, ".bashrc") && strings.Contains(cmdStr, "echo") {
			foundBashrcAppend = true
		}
	}
	if !foundBashrcCheck {
		t.Error("PrepareWorker() did not check .bashrc")
	}
	if !foundBashrcAppend {
		t.Error("PrepareWorker() did not append to .bashrc")
	}
}

func TestPrepareWorker_BashrcAlreadyConfigured(t *testing.T) {
	mock := NewMockContainerExecer()
	mock.ExecFunc = func(name string, cmd []string) ([]byte, error) {
		cmdStr := strings.Join(cmd, " ")
		if strings.Contains(cmdStr, "grep -q") {
			return []byte("exists\n"), nil // Already configured
		}
		return []byte(""), nil
	}

	cfg := testConfig()
	launcher, _ := NewLauncher(cfg, mock)

	launcher.PrepareWorker("worker-01", "")

	// Should not append to bashrc if already exists
	// The append command uses >> to append to .bashrc
	appendFound := false
	for _, call := range mock.ExecCalls {
		cmdStr := strings.Join(call.Cmd, " ")
		// The append command includes >> to .bashrc
		if strings.Contains(cmdStr, ">> /home/dev/.bashrc") {
			appendFound = true
			break
		}
	}
	if appendFound {
		t.Error("PrepareWorker() should not append to .bashrc when already configured")
	}
}

func TestPrepareWorker_EnvWriteError(t *testing.T) {
	mock := NewMockContainerExecer()
	expectedErr := errors.New("failed to write env file")
	callCount := 0
	mock.ExecFunc = func(name string, cmd []string) ([]byte, error) {
		callCount++
		if callCount == 1 {
			return nil, expectedErr // First call (env file) fails
		}
		return []byte(""), nil
	}

	cfg := testConfig()
	launcher, _ := NewLauncher(cfg, mock)

	err := launcher.PrepareWorker("worker-01", "")

	if err == nil {
		t.Error("PrepareWorker() should return error when env write fails")
	}
	if !strings.Contains(err.Error(), "env file") {
		t.Errorf("PrepareWorker() error = %v, should mention env file", err)
	}
}

func TestPrepareWorker_CLAUDEMDWriteError(t *testing.T) {
	mock := NewMockContainerExecer()
	expectedErr := errors.New("failed to write CLAUDE.md")
	callCount := 0
	mock.ExecFunc = func(name string, cmd []string) ([]byte, error) {
		callCount++
		if callCount == 2 {
			return nil, expectedErr // Second call (CLAUDE.md) fails
		}
		return []byte(""), nil
	}

	cfg := testConfig()
	launcher, _ := NewLauncher(cfg, mock)

	err := launcher.PrepareWorker("worker-01", "")

	if err == nil {
		t.Error("PrepareWorker() should return error when CLAUDE.md write fails")
	}
	if !strings.Contains(err.Error(), "CLAUDE.md") {
		t.Errorf("PrepareWorker() error = %v, should mention CLAUDE.md", err)
	}
}

func TestPrepareWorker_BashrcSetupError(t *testing.T) {
	mock := NewMockContainerExecer()
	expectedErr := errors.New("failed to setup bashrc")
	callCount := 0
	mock.ExecFunc = func(name string, cmd []string) ([]byte, error) {
		callCount++
		if callCount == 3 {
			return nil, expectedErr // Third call (bashrc check) fails
		}
		return []byte(""), nil
	}

	cfg := testConfig()
	launcher, _ := NewLauncher(cfg, mock)

	err := launcher.PrepareWorker("worker-01", "")

	if err == nil {
		t.Error("PrepareWorker() should return error when bashrc setup fails")
	}
	if !strings.Contains(err.Error(), "bashrc") {
		t.Errorf("PrepareWorker() error = %v, should mention bashrc", err)
	}
}

func TestGetLaunchCommand(t *testing.T) {
	tests := []struct {
		name        string
		claudeCmd   string
		claudeArgs  []string
		expectedLen int
	}{
		{
			name:        "command only",
			claudeCmd:   "claude",
			claudeArgs:  nil,
			expectedLen: 1,
		},
		{
			name:        "command with one arg",
			claudeCmd:   "claude",
			claudeArgs:  []string{"--dangerously-skip-permissions"},
			expectedLen: 2,
		},
		{
			name:        "command with multiple args",
			claudeCmd:   "claude",
			claudeArgs:  []string{"--dangerously-skip-permissions", "--verbose", "--debug"},
			expectedLen: 4,
		},
		{
			name:        "custom command",
			claudeCmd:   "/usr/local/bin/claude-code",
			claudeArgs:  []string{"--arg1", "--arg2"},
			expectedLen: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockContainerExecer()
			cfg := testConfig()
			cfg.Claude.Command = tt.claudeCmd
			cfg.Claude.Args = tt.claudeArgs

			launcher, _ := NewLauncher(cfg, mock)
			cmd := launcher.GetLaunchCommand()

			if len(cmd) != tt.expectedLen {
				t.Errorf("GetLaunchCommand() len = %d, want %d", len(cmd), tt.expectedLen)
			}

			if cmd[0] != tt.claudeCmd {
				t.Errorf("GetLaunchCommand()[0] = %q, want %q", cmd[0], tt.claudeCmd)
			}

			for i, arg := range tt.claudeArgs {
				if cmd[i+1] != arg {
					t.Errorf("GetLaunchCommand()[%d] = %q, want %q", i+1, cmd[i+1], arg)
				}
			}
		})
	}
}

func TestGetLaunchConfig(t *testing.T) {
	mock := NewMockContainerExecer()
	cfg := testConfig()
	launcher, _ := NewLauncher(cfg, mock)

	launchCfg := launcher.GetLaunchConfig("worker-01", "isollm/task-123")

	if launchCfg == nil {
		t.Fatal("GetLaunchConfig() returned nil")
	}

	// Check Command
	if launchCfg.Command != cfg.Claude.Command {
		t.Errorf("LaunchConfig.Command = %q, want %q", launchCfg.Command, cfg.Claude.Command)
	}

	// Check Args
	if len(launchCfg.Args) != len(cfg.Claude.Args) {
		t.Errorf("LaunchConfig.Args len = %d, want %d", len(launchCfg.Args), len(cfg.Claude.Args))
	}

	// Check WorkDir
	if launchCfg.WorkDir != DefaultProjectPath {
		t.Errorf("LaunchConfig.WorkDir = %q, want %q", launchCfg.WorkDir, DefaultProjectPath)
	}

	// Check Env
	if launchCfg.Env == nil {
		t.Error("LaunchConfig.Env is nil")
	} else {
		if launchCfg.Env.AiryraProject != cfg.Airyra.Project {
			t.Errorf("LaunchConfig.Env.AiryraProject = %q, want %q", launchCfg.Env.AiryraProject, cfg.Airyra.Project)
		}
		if launchCfg.Env.AiryraAgent != "worker-01" {
			t.Errorf("LaunchConfig.Env.AiryraAgent = %q, want %q", launchCfg.Env.AiryraAgent, "worker-01")
		}
	}

	// Check Context
	if launchCfg.Context == nil {
		t.Error("LaunchConfig.Context is nil")
	} else {
		if launchCfg.Context.WorkerName != "worker-01" {
			t.Errorf("LaunchConfig.Context.WorkerName = %q, want %q", launchCfg.Context.WorkerName, "worker-01")
		}
		if launchCfg.Context.TaskBranch != "isollm/task-123" {
			t.Errorf("LaunchConfig.Context.TaskBranch = %q, want %q", launchCfg.Context.TaskBranch, "isollm/task-123")
		}
		if launchCfg.Context.BaseBranch != cfg.Git.BaseBranch {
			t.Errorf("LaunchConfig.Context.BaseBranch = %q, want %q", launchCfg.Context.BaseBranch, cfg.Git.BaseBranch)
		}
	}
}

func TestGetLaunchConfig_EmptyTaskBranch(t *testing.T) {
	mock := NewMockContainerExecer()
	cfg := testConfig()
	launcher, _ := NewLauncher(cfg, mock)

	launchCfg := launcher.GetLaunchConfig("worker-01", "")

	if launchCfg.Context.TaskBranch != "" {
		t.Errorf("LaunchConfig.Context.TaskBranch = %q, want empty", launchCfg.Context.TaskBranch)
	}
}

func TestGetLaunchConfig_UsesHostIP(t *testing.T) {
	mock := NewMockContainerExecer()
	cfg := testConfig()
	launcher, _ := NewLauncher(cfg, mock)

	launchCfg := launcher.GetLaunchConfig("worker-01", "")

	// Environment should use the launcher's host IP
	hostIP := launcher.GetHostIP()
	if launchCfg.Env.AiryraHost != hostIP {
		t.Errorf("LaunchConfig.Env.AiryraHost = %q, want %q", launchCfg.Env.AiryraHost, hostIP)
	}
	if launchCfg.Context.AiryraHost != hostIP {
		t.Errorf("LaunchConfig.Context.AiryraHost = %q, want %q", launchCfg.Context.AiryraHost, hostIP)
	}
}

func TestLauncher_GetHostIP(t *testing.T) {
	mock := NewMockContainerExecer()
	cfg := testConfig()
	launcher, _ := NewLauncher(cfg, mock)

	hostIP := launcher.GetHostIP()

	if hostIP == "" {
		t.Error("GetHostIP() returned empty string")
	}
}

func TestConstants(t *testing.T) {
	// Test that constants are defined
	if EnvFilePath == "" {
		t.Error("EnvFilePath is empty")
	}
	if DefaultProjectPath == "" {
		t.Error("DefaultProjectPath is empty")
	}
	if DefaultBareRepoPath == "" {
		t.Error("DefaultBareRepoPath is empty")
	}

	// Check expected values
	if EnvFilePath != "/home/dev/.isollm-env" {
		t.Errorf("EnvFilePath = %q, want %q", EnvFilePath, "/home/dev/.isollm-env")
	}
	if DefaultProjectPath != "/home/dev/project" {
		t.Errorf("DefaultProjectPath = %q, want %q", DefaultProjectPath, "/home/dev/project")
	}
	if DefaultBareRepoPath != "/repo.git" {
		t.Errorf("DefaultBareRepoPath = %q, want %q", DefaultBareRepoPath, "/repo.git")
	}
}

func TestMockContainerExecer_RecordsCalls(t *testing.T) {
	mock := NewMockContainerExecer()

	mock.Exec("container1", []string{"echo", "hello"})
	mock.Exec("container2", []string{"ls", "-la"})

	if mock.CallCount() != 2 {
		t.Errorf("CallCount() = %d, want 2", mock.CallCount())
	}

	if mock.ExecCalls[0].Name != "container1" {
		t.Errorf("ExecCalls[0].Name = %q, want %q", mock.ExecCalls[0].Name, "container1")
	}
	if mock.ExecCalls[1].Name != "container2" {
		t.Errorf("ExecCalls[1].Name = %q, want %q", mock.ExecCalls[1].Name, "container2")
	}
}

func TestMockContainerExecer_LastCall(t *testing.T) {
	mock := NewMockContainerExecer()

	// No calls yet
	if mock.LastCall() != nil {
		t.Error("LastCall() should be nil when no calls made")
	}

	mock.Exec("container", []string{"cmd"})

	last := mock.LastCall()
	if last == nil {
		t.Fatal("LastCall() returned nil after call")
	}
	if last.Name != "container" {
		t.Errorf("LastCall().Name = %q, want %q", last.Name, "container")
	}
}

func TestMockContainerExecer_Reset(t *testing.T) {
	mock := NewMockContainerExecer()

	mock.Exec("container", []string{"cmd"})
	mock.Reset()

	if mock.CallCount() != 0 {
		t.Errorf("After Reset(), CallCount() = %d, want 0", mock.CallCount())
	}
}

func TestMockContainerExecer_CustomFunc(t *testing.T) {
	mock := NewMockContainerExecer()
	mock.ExecFunc = func(name string, cmd []string) ([]byte, error) {
		return []byte("custom output"), nil
	}

	output, err := mock.Exec("container", []string{"cmd"})

	if err != nil {
		t.Errorf("Exec() error = %v", err)
	}
	if string(output) != "custom output" {
		t.Errorf("Exec() output = %q, want %q", string(output), "custom output")
	}
}

func TestMockContainerExecer_ExecError(t *testing.T) {
	mock := NewMockContainerExecer()
	expectedErr := errors.New("exec failed")
	mock.ExecError = expectedErr

	_, err := mock.Exec("container", []string{"cmd"})

	if err != expectedErr {
		t.Errorf("Exec() error = %v, want %v", err, expectedErr)
	}
}

func TestMockContainerExecer_ExecOutput(t *testing.T) {
	mock := NewMockContainerExecer()
	mock.ExecOutput = []byte("default output")

	output, _ := mock.Exec("container", []string{"cmd"})

	if string(output) != "default output" {
		t.Errorf("Exec() output = %q, want %q", string(output), "default output")
	}
}

func TestLaunchConfigFields(t *testing.T) {
	// Test that LaunchConfig struct has all expected fields
	cfg := LaunchConfig{
		Command: "cmd",
		Args:    []string{"arg1", "arg2"},
		WorkDir: "/workdir",
		Env:     &Environment{},
		Context: &Context{},
	}

	if cfg.Command != "cmd" {
		t.Error("LaunchConfig.Command not set correctly")
	}
	if len(cfg.Args) != 2 {
		t.Error("LaunchConfig.Args not set correctly")
	}
	if cfg.WorkDir != "/workdir" {
		t.Error("LaunchConfig.WorkDir not set correctly")
	}
	if cfg.Env == nil {
		t.Error("LaunchConfig.Env not set correctly")
	}
	if cfg.Context == nil {
		t.Error("LaunchConfig.Context not set correctly")
	}
}
