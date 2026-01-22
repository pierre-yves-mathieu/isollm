package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// validConfig returns a minimal valid configuration for testing
func validConfig() *Config {
	return &Config{
		Project: "myproject",
		Workers: 3,
		Image:   "ubuntu:24.04",
		Git: GitConfig{
			BaseBranch:   "main",
			BranchPrefix: "isollm/",
		},
		Claude: ClaudeConfig{
			Command: "claude",
		},
		Airyra: AiryraConfig{
			Host: "localhost",
			Port: 7432,
		},
		Zellij: ZellijConfig{
			Layout:    "auto",
			Dashboard: true,
		},
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := validConfig()
	err := cfg.Validate()
	if err != nil {
		t.Errorf("expected no error for valid config, got: %v", err)
	}
}

func TestValidate_EmptyProject(t *testing.T) {
	cfg := validConfig()
	cfg.Project = ""
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for empty project name")
	}
	if !strings.Contains(err.Error(), "project name is required") {
		t.Errorf("expected 'project name is required' error, got: %v", err)
	}
}

func TestValidate_ProjectNameTooShort(t *testing.T) {
	cfg := validConfig()
	cfg.Project = "a"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for short project name")
	}
	if !strings.Contains(err.Error(), "at least 2 characters") {
		t.Errorf("expected 'at least 2 characters' error, got: %v", err)
	}
}

func TestValidate_ProjectNameTooLong(t *testing.T) {
	cfg := validConfig()
	cfg.Project = strings.Repeat("a", MaxProjectNameLen+1)
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for long project name")
	}
	if !strings.Contains(err.Error(), "cannot exceed 64 characters") {
		t.Errorf("expected 'cannot exceed 64 characters' error, got: %v", err)
	}
}

func TestValidate_InvalidProjectName(t *testing.T) {
	testCases := []struct {
		name    string
		project string
	}{
		{"starts with number", "1project"},
		{"starts with hyphen", "-project"},
		{"contains underscore", "my_project"},
		{"contains space", "my project"},
		{"contains dot", "my.project"},
		{"contains special char", "my@project"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Project = tc.project
			err := cfg.Validate()
			if err == nil {
				t.Fatal("expected error for invalid project name")
			}
			if !strings.Contains(err.Error(), "alphanumeric with hyphens") {
				t.Errorf("expected alphanumeric error, got: %v", err)
			}
		})
	}
}

func TestValidate_ValidProjectNames(t *testing.T) {
	testCases := []string{
		"ab",                   // minimum length
		"myproject",            // simple
		"my-project",           // with hyphen
		"MyProject",            // mixed case
		"project123",           // with numbers
		"A-1-B-2",              // multiple hyphens and numbers
		strings.Repeat("a", 64), // maximum length
	}

	for _, project := range testCases {
		t.Run(project, func(t *testing.T) {
			cfg := validConfig()
			cfg.Project = project
			err := cfg.Validate()
			if err != nil {
				t.Errorf("expected valid project name %q to pass, got: %v", project, err)
			}
		})
	}
}

func TestValidate_WorkersBelowMin(t *testing.T) {
	cfg := validConfig()
	cfg.Workers = 0
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for zero workers")
	}
	if !strings.Contains(err.Error(), "workers must be at least 1") {
		t.Errorf("expected workers minimum error, got: %v", err)
	}
}

func TestValidate_WorkersAboveMax(t *testing.T) {
	cfg := validConfig()
	cfg.Workers = MaxWorkers + 1
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for too many workers")
	}
	if !strings.Contains(err.Error(), "workers cannot exceed 20") {
		t.Errorf("expected workers maximum error, got: %v", err)
	}
}

func TestValidate_WorkersValidRange(t *testing.T) {
	testCases := []int{MinWorkers, 5, 10, MaxWorkers}
	for _, workers := range testCases {
		t.Run(string(rune(workers)), func(t *testing.T) {
			cfg := validConfig()
			cfg.Workers = workers
			err := cfg.Validate()
			if err != nil {
				t.Errorf("expected %d workers to be valid, got: %v", workers, err)
			}
		})
	}
}

func TestValidate_EmptyImage(t *testing.T) {
	cfg := validConfig()
	cfg.Image = ""
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for empty image")
	}
	if !strings.Contains(err.Error(), "image is required") {
		t.Errorf("expected 'image is required' error, got: %v", err)
	}
}

func TestValidate_EmptyBaseBranch(t *testing.T) {
	cfg := validConfig()
	cfg.Git.BaseBranch = ""
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for empty base branch")
	}
	if !strings.Contains(err.Error(), "git.base_branch is required") {
		t.Errorf("expected base branch required error, got: %v", err)
	}
}

func TestValidate_InvalidBaseBranch(t *testing.T) {
	testCases := []struct {
		name   string
		branch string
	}{
		{"contains space", "main branch"},
		{"contains special char", "main@branch"},
		{"contains asterisk", "main*"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Git.BaseBranch = tc.branch
			err := cfg.Validate()
			if err == nil {
				t.Fatal("expected error for invalid branch name")
			}
			if !strings.Contains(err.Error(), "invalid characters") {
				t.Errorf("expected invalid characters error, got: %v", err)
			}
		})
	}
}

func TestValidate_ValidBaseBranches(t *testing.T) {
	testCases := []string{
		"main",
		"master",
		"develop",
		"feature/my-feature",
		"release/v1.0.0",
		"bugfix/fix-123",
		"refs/heads/main",
	}

	for _, branch := range testCases {
		t.Run(branch, func(t *testing.T) {
			cfg := validConfig()
			cfg.Git.BaseBranch = branch
			err := cfg.Validate()
			if err != nil {
				t.Errorf("expected valid branch %q to pass, got: %v", branch, err)
			}
		})
	}
}

func TestValidate_InvalidBranchPrefix(t *testing.T) {
	cfg := validConfig()
	cfg.Git.BranchPrefix = "isollm"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for branch prefix without trailing slash")
	}
	if !strings.Contains(err.Error(), "must end with '/'") {
		t.Errorf("expected trailing slash error, got: %v", err)
	}
}

func TestValidate_BranchPrefixJustSlash(t *testing.T) {
	cfg := validConfig()
	cfg.Git.BranchPrefix = "/"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for branch prefix that is just '/'")
	}
	if !strings.Contains(err.Error(), "cannot be just '/'") {
		t.Errorf("expected 'cannot be just /' error, got: %v", err)
	}
}

func TestValidate_ValidBranchPrefixes(t *testing.T) {
	testCases := []string{
		"isollm/",
		"feature/",
		"ai/workers/",
	}

	for _, prefix := range testCases {
		t.Run(prefix, func(t *testing.T) {
			cfg := validConfig()
			cfg.Git.BranchPrefix = prefix
			err := cfg.Validate()
			if err != nil {
				t.Errorf("expected valid prefix %q to pass, got: %v", prefix, err)
			}
		})
	}
}

func TestValidate_EmptyBranchPrefix(t *testing.T) {
	// Empty branch prefix should be allowed (no prefix used)
	cfg := validConfig()
	cfg.Git.BranchPrefix = ""
	err := cfg.Validate()
	if err != nil {
		t.Errorf("expected empty branch prefix to be valid, got: %v", err)
	}
}

func TestValidate_InvalidAiryraPort(t *testing.T) {
	testCases := []struct {
		name string
		port int
	}{
		{"too low", 1023},
		{"zero", 0},
		{"negative", -1},
		{"too high", 65536},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Airyra.Port = tc.port
			err := cfg.Validate()
			if err == nil {
				t.Fatal("expected error for invalid port")
			}
			if !strings.Contains(err.Error(), "airyra.port must be between") {
				t.Errorf("expected port range error, got: %v", err)
			}
		})
	}
}

func TestValidate_ValidAiryraPorts(t *testing.T) {
	testCases := []int{1024, 3000, 8080, 65535}

	for _, port := range testCases {
		t.Run(string(rune(port)), func(t *testing.T) {
			cfg := validConfig()
			cfg.Airyra.Port = port
			err := cfg.Validate()
			if err != nil {
				t.Errorf("expected port %d to be valid, got: %v", port, err)
			}
		})
	}
}

func TestValidate_InvalidPortFormat(t *testing.T) {
	testCases := []struct {
		name string
		port string
	}{
		{"too many colons", "8080:8080:8080"},
		{"not a number", "abc"},
		{"host not a number", "abc:8080"},
		{"container not a number", "8080:abc"},
		{"out of range high", "70000"},
		{"out of range zero", "0"},
		{"out of range negative", "-1"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Ports = []string{tc.port}
			err := cfg.Validate()
			if err == nil {
				t.Fatalf("expected error for invalid port %q", tc.port)
			}
			if !strings.Contains(err.Error(), "invalid port") {
				t.Errorf("expected invalid port error, got: %v", err)
			}
		})
	}
}

func TestValidate_ValidPortFormats(t *testing.T) {
	testCases := []string{
		"8080",
		"3000",
		"8080:80",
		"3000:3000",
		"1:65535",
	}

	for _, port := range testCases {
		t.Run(port, func(t *testing.T) {
			cfg := validConfig()
			cfg.Ports = []string{port}
			err := cfg.Validate()
			if err != nil {
				t.Errorf("expected port %q to be valid, got: %v", port, err)
			}
		})
	}
}

func TestValidate_DuplicatePorts(t *testing.T) {
	cfg := validConfig()
	cfg.Ports = []string{"8080", "8080"}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for duplicate ports")
	}
	if !strings.Contains(err.Error(), "duplicate port: 8080") {
		t.Errorf("expected duplicate port error, got: %v", err)
	}
}

func TestValidate_DuplicateHostPorts(t *testing.T) {
	cfg := validConfig()
	cfg.Ports = []string{"8080:80", "8080:443"}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for duplicate host ports")
	}
	if !strings.Contains(err.Error(), "duplicate port: 8080") {
		t.Errorf("expected duplicate port error, got: %v", err)
	}
}

func TestValidate_PortConflictWithAiryra(t *testing.T) {
	cfg := validConfig()
	cfg.Airyra.Port = 7432
	cfg.Ports = []string{"7432"}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for port conflict with airyra")
	}
	if !strings.Contains(err.Error(), "conflicts with airyra.port") {
		t.Errorf("expected airyra conflict error, got: %v", err)
	}
}

func TestValidate_PortConflictWithAiryraHostPort(t *testing.T) {
	cfg := validConfig()
	cfg.Airyra.Port = 7432
	cfg.Ports = []string{"7432:8080"}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for port conflict with airyra")
	}
	if !strings.Contains(err.Error(), "conflicts with airyra.port") {
		t.Errorf("expected airyra conflict error, got: %v", err)
	}
}

func TestValidate_InvalidLayout(t *testing.T) {
	cfg := validConfig()
	cfg.Zellij.Layout = "invalid"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid layout")
	}
	if !strings.Contains(err.Error(), "zellij.layout must be one of") {
		t.Errorf("expected layout error, got: %v", err)
	}
}

func TestValidate_ValidLayouts(t *testing.T) {
	testCases := []string{"auto", "horizontal", "vertical", "grid"}

	for _, layout := range testCases {
		t.Run(layout, func(t *testing.T) {
			cfg := validConfig()
			cfg.Zellij.Layout = layout
			err := cfg.Validate()
			if err != nil {
				t.Errorf("expected layout %q to be valid, got: %v", layout, err)
			}
		})
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	cfg := &Config{
		Project: "",  // error 1
		Workers: 0,   // error 2
		Image:   "",  // error 3
		Git: GitConfig{
			BaseBranch:   "",  // error 4
			BranchPrefix: "/", // error 5
		},
		Airyra: AiryraConfig{
			Port: 100, // error 6
		},
		Zellij: ZellijConfig{
			Layout: "invalid", // error 7
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected multiple errors")
	}

	validationErr, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got: %T", err)
	}

	// Should have collected multiple errors
	if len(validationErr.Errors) < 5 {
		t.Errorf("expected at least 5 errors, got %d: %v", len(validationErr.Errors), validationErr.Errors)
	}
}

func TestValidationError_Error(t *testing.T) {
	ve := &ValidationError{}
	ve.Add("error 1")
	ve.Add("error 2")

	errStr := ve.Error()
	if !strings.Contains(errStr, "config validation failed") {
		t.Error("expected error string to contain 'config validation failed'")
	}
	if !strings.Contains(errStr, "error 1") {
		t.Error("expected error string to contain 'error 1'")
	}
	if !strings.Contains(errStr, "error 2") {
		t.Error("expected error string to contain 'error 2'")
	}
}

func TestValidationError_Add(t *testing.T) {
	ve := &ValidationError{}
	ve.Add("first error")
	ve.Add("second error")

	if len(ve.Errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(ve.Errors))
	}
	if ve.Errors[0] != "first error" {
		t.Errorf("expected 'first error', got %q", ve.Errors[0])
	}
	if ve.Errors[1] != "second error" {
		t.Errorf("expected 'second error', got %q", ve.Errors[1])
	}
}

func TestValidationError_HasErrors(t *testing.T) {
	ve := &ValidationError{}
	if ve.HasErrors() {
		t.Error("expected HasErrors to be false for empty ValidationError")
	}

	ve.Add("an error")
	if !ve.HasErrors() {
		t.Error("expected HasErrors to be true after adding error")
	}
}

func TestWarnings_ImageNoTag(t *testing.T) {
	cfg := validConfig()
	cfg.Image = "ubuntu"
	warnings := cfg.Warnings()

	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}
	if !strings.Contains(warnings[0], "image has no tag") {
		t.Errorf("expected 'image has no tag' warning, got: %s", warnings[0])
	}
}

func TestWarnings_ImageWithTag(t *testing.T) {
	cfg := validConfig()
	cfg.Image = "ubuntu:24.04"
	warnings := cfg.Warnings()

	if len(warnings) != 0 {
		t.Errorf("expected no warnings for image with tag, got: %v", warnings)
	}
}

func TestWarnings_EmptyImage(t *testing.T) {
	cfg := validConfig()
	cfg.Image = ""
	warnings := cfg.Warnings()

	// Empty image should not generate a warning (it's an error, caught by Validate)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for empty image, got: %v", warnings)
	}
}

func TestLoadAndValidate_ValidConfig(t *testing.T) {
	// Create a temporary directory with a valid config
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ConfigFileName)

	content := `project: myproject
workers: 3
image: ubuntu:24.04
git:
  base_branch: main
  branch_prefix: isollm/
claude:
  command: claude
airyra:
  host: localhost
  port: 7432
zellij:
  layout: auto
  dashboard: true
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadAndValidate(tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.Project != "myproject" {
		t.Errorf("expected project 'myproject', got %q", cfg.Project)
	}
}

func TestLoadAndValidate_InvalidConfig(t *testing.T) {
	// Create a temporary directory with an invalid config
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ConfigFileName)

	content := `project: ""
workers: 0
image: ""
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	_, err := LoadAndValidate(tmpDir)
	if err == nil {
		t.Fatal("expected validation error")
	}
	// Check that it's a validation error, not a load error
	if !strings.Contains(err.Error(), "config validation failed") {
		t.Errorf("expected validation error, got: %v", err)
	}
}

func TestLoadAndValidate_MissingConfig(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := LoadAndValidate(tmpDir)
	if err == nil {
		t.Fatal("expected error for missing config")
	}
	if !strings.Contains(err.Error(), "config file not found") {
		t.Errorf("expected 'config file not found' error, got: %v", err)
	}
}

func TestLoadAndValidate_MalformedYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ConfigFileName)

	content := `project: [invalid yaml
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	_, err := LoadAndValidate(tmpDir)
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
	if !strings.Contains(err.Error(), "failed to parse config") {
		t.Errorf("expected parse error, got: %v", err)
	}
}

func TestValidatePort_ValidFormats(t *testing.T) {
	testCases := []string{
		"1",
		"80",
		"8080",
		"65535",
		"80:80",
		"8080:80",
		"3000:3000",
	}

	for _, port := range testCases {
		t.Run(port, func(t *testing.T) {
			err := validatePort(port)
			if err != nil {
				t.Errorf("expected port %q to be valid, got: %v", port, err)
			}
		})
	}
}

func TestValidatePort_InvalidFormats(t *testing.T) {
	testCases := []struct {
		name   string
		port   string
		errMsg string
	}{
		{"too many colons", "1:2:3", "invalid format"},
		{"not a number", "abc", "not a valid number"},
		{"first not a number", "abc:80", "not a valid number"},
		{"second not a number", "80:abc", "not a valid number"},
		{"zero port", "0", "out of range"},
		{"negative port", "-1", "out of range"},
		{"too high", "65536", "out of range"},
		{"first too high", "65536:80", "out of range"},
		{"second too high", "80:65536", "out of range"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePort(tc.port)
			if err == nil {
				t.Fatalf("expected error for port %q", tc.port)
			}
			if !strings.Contains(err.Error(), tc.errMsg) {
				t.Errorf("expected error containing %q, got: %v", tc.errMsg, err)
			}
		})
	}
}

// Test constants are exported and have expected values
func TestConstants(t *testing.T) {
	if MinWorkers != 1 {
		t.Errorf("expected MinWorkers=1, got %d", MinWorkers)
	}
	if MaxWorkers != 20 {
		t.Errorf("expected MaxWorkers=20, got %d", MaxWorkers)
	}
	if MinUserPort != 1024 {
		t.Errorf("expected MinUserPort=1024, got %d", MinUserPort)
	}
	if MaxPort != 65535 {
		t.Errorf("expected MaxPort=65535, got %d", MaxPort)
	}
	if MinProjectNameLen != 2 {
		t.Errorf("expected MinProjectNameLen=2, got %d", MinProjectNameLen)
	}
	if MaxProjectNameLen != 64 {
		t.Errorf("expected MaxProjectNameLen=64, got %d", MaxProjectNameLen)
	}
}
