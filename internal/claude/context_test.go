package claude

import (
	"strings"
	"testing"
)

func TestGenerateCLAUDEMD(t *testing.T) {
	ctx := &Context{
		ProjectName: "test-project",
		WorkerName:  "worker-01",
		TaskBranch:  "isollm/task-123",
		BaseBranch:  "main",
		AiryraHost:  "192.168.1.1",
		AiryraPort:  7432,
	}

	result := GenerateCLAUDEMD(ctx)

	if result == "" {
		t.Fatal("GenerateCLAUDEMD() returned empty string")
	}

	// Check for header
	if !strings.Contains(result, "# CLAUDE.md") {
		t.Error("GenerateCLAUDEMD() missing main header")
	}

	// Check for project info
	if !strings.Contains(result, "test-project") {
		t.Error("GenerateCLAUDEMD() missing project name")
	}

	// Check for worker name
	if !strings.Contains(result, "worker-01") {
		t.Error("GenerateCLAUDEMD() missing worker name")
	}

	// Check for base branch
	if !strings.Contains(result, "main") {
		t.Error("GenerateCLAUDEMD() missing base branch")
	}

	// Check for task branch
	if !strings.Contains(result, "isollm/task-123") {
		t.Error("GenerateCLAUDEMD() missing task branch")
	}

	// Check for host and port in airyra commands
	if !strings.Contains(result, "192.168.1.1") {
		t.Error("GenerateCLAUDEMD() missing host in airyra commands")
	}
	if !strings.Contains(result, "7432") {
		t.Error("GenerateCLAUDEMD() missing port in airyra commands")
	}
}

func TestGenerateCLAUDEMD_ContainsWorkflow(t *testing.T) {
	ctx := &Context{
		ProjectName: "myproject",
		WorkerName:  "worker",
		BaseBranch:  "main",
		AiryraHost:  "localhost",
		AiryraPort:  7432,
	}

	result := GenerateCLAUDEMD(ctx)

	workflowSections := []string{
		"## Task Workflow",
		"### 1. Claiming a Task",
		"### 2. Creating a Task Branch",
		"### 3. Git Commit Workflow",
		"### 4. Completing a Task",
		"### 5. Handling Blockers",
		"### 6. Releasing a Task",
	}

	for _, section := range workflowSections {
		if !strings.Contains(result, section) {
			t.Errorf("GenerateCLAUDEMD() missing workflow section: %q", section)
		}
	}
}

func TestGenerateCLAUDEMD_ContainsAiryraCommands(t *testing.T) {
	ctx := &Context{
		ProjectName: "myproject",
		WorkerName:  "worker",
		BaseBranch:  "main",
		AiryraHost:  "10.0.0.1",
		AiryraPort:  8080,
	}

	result := GenerateCLAUDEMD(ctx)

	airyraCommands := []string{
		"airyra task list",
		"airyra task claim",
		"airyra task done",
		"airyra task block",
		"airyra task release",
	}

	for _, cmd := range airyraCommands {
		if !strings.Contains(result, cmd) {
			t.Errorf("GenerateCLAUDEMD() missing airyra command: %q", cmd)
		}
	}

	// Verify host and port are in the commands
	expectedHostPort := "--host 10.0.0.1 --port 8080"
	if !strings.Contains(result, expectedHostPort) {
		t.Errorf("GenerateCLAUDEMD() missing host/port in commands: %q", expectedHostPort)
	}
}

func TestGenerateCLAUDEMD_ContainsEnvInfo(t *testing.T) {
	ctx := &Context{
		ProjectName: "myproject",
		WorkerName:  "worker",
		BaseBranch:  "main",
		AiryraHost:  "192.168.1.1",
		AiryraPort:  7432,
	}

	result := GenerateCLAUDEMD(ctx)

	// Check for environment section
	if !strings.Contains(result, "## Environment") {
		t.Error("GenerateCLAUDEMD() missing Environment section")
	}

	// Check for expected environment variable names
	envVars := []string{
		"AIRYRA_HOST",
		"AIRYRA_PORT",
		"AIRYRA_PROJECT",
		"AIRYRA_AGENT",
		"ISOLLM_PROJECT_PATH",
		"ISOLLM_BARE_REPO",
	}

	for _, envVar := range envVars {
		if !strings.Contains(result, envVar) {
			t.Errorf("GenerateCLAUDEMD() missing environment variable: %q", envVar)
		}
	}
}

func TestGenerateCLAUDEMD_ContainsImportantNotes(t *testing.T) {
	ctx := &Context{
		ProjectName: "myproject",
		WorkerName:  "worker",
		BaseBranch:  "main",
		AiryraHost:  "localhost",
		AiryraPort:  7432,
	}

	result := GenerateCLAUDEMD(ctx)

	// Check for important notes section
	if !strings.Contains(result, "## Important Notes") {
		t.Error("GenerateCLAUDEMD() missing Important Notes section")
	}

	// Check for some key notes
	importantPhrases := []string{
		"Always push your work",
		"Commit frequently",
		"Communicate blockers",
	}

	for _, phrase := range importantPhrases {
		if !strings.Contains(result, phrase) {
			t.Errorf("GenerateCLAUDEMD() missing important note: %q", phrase)
		}
	}
}

func TestGenerateCLAUDEMD_ContainsCommitConventions(t *testing.T) {
	ctx := &Context{
		ProjectName: "myproject",
		WorkerName:  "worker",
		BaseBranch:  "main",
		AiryraHost:  "localhost",
		AiryraPort:  7432,
	}

	result := GenerateCLAUDEMD(ctx)

	// Check for commit message conventions
	conventions := []string{
		"feat:",
		"fix:",
		"refactor:",
		"docs:",
		"test:",
		"chore:",
	}

	for _, convention := range conventions {
		if !strings.Contains(result, convention) {
			t.Errorf("GenerateCLAUDEMD() missing commit convention: %q", convention)
		}
	}
}

func TestGenerateCLAUDEMD_WithCustomContext(t *testing.T) {
	customText := "This project uses a specific testing framework. Always run tests before pushing."

	ctx := &Context{
		ProjectName:   "myproject",
		WorkerName:    "worker",
		BaseBranch:    "main",
		AiryraHost:    "localhost",
		AiryraPort:    7432,
		CustomContext: customText,
	}

	result := GenerateCLAUDEMD(ctx)

	// Check for custom context section
	if !strings.Contains(result, "## Project-Specific Instructions") {
		t.Error("GenerateCLAUDEMD() missing Project-Specific Instructions section when custom context provided")
	}

	// Check for custom text
	if !strings.Contains(result, customText) {
		t.Error("GenerateCLAUDEMD() missing custom context text")
	}
}

func TestGenerateCLAUDEMD_WithoutCustomContext(t *testing.T) {
	ctx := &Context{
		ProjectName:   "myproject",
		WorkerName:    "worker",
		BaseBranch:    "main",
		AiryraHost:    "localhost",
		AiryraPort:    7432,
		CustomContext: "", // Empty custom context
	}

	result := GenerateCLAUDEMD(ctx)

	// Should not have custom context section when empty
	if strings.Contains(result, "## Project-Specific Instructions") {
		t.Error("GenerateCLAUDEMD() should not include Project-Specific Instructions when custom context is empty")
	}
}

func TestGenerateCLAUDEMD_WithoutTaskBranch(t *testing.T) {
	ctx := &Context{
		ProjectName: "myproject",
		WorkerName:  "worker",
		TaskBranch:  "", // No task branch
		BaseBranch:  "main",
		AiryraHost:  "localhost",
		AiryraPort:  7432,
	}

	result := GenerateCLAUDEMD(ctx)

	// Should not have "Task Branch" line when empty
	if strings.Contains(result, "**Task Branch**:") && strings.Contains(result, "**Task Branch**: \n") {
		t.Error("GenerateCLAUDEMD() should handle empty task branch gracefully")
	}

	// Should still have base branch
	if !strings.Contains(result, "**Base Branch**: main") {
		t.Error("GenerateCLAUDEMD() missing base branch when task branch is empty")
	}
}

func TestGenerateCLAUDEMD_TableFormat(t *testing.T) {
	ctx := &Context{
		ProjectName: "myproject",
		WorkerName:  "worker",
		BaseBranch:  "main",
		AiryraHost:  "localhost",
		AiryraPort:  7432,
	}

	result := GenerateCLAUDEMD(ctx)

	// Check for markdown table format
	if !strings.Contains(result, "| Variable | Description |") {
		t.Error("GenerateCLAUDEMD() missing environment table header")
	}

	if !strings.Contains(result, "|----------|-------------|") {
		t.Error("GenerateCLAUDEMD() missing environment table separator")
	}
}

func TestGenerateMinimalCLAUDEMD(t *testing.T) {
	ctx := &Context{
		ProjectName: "test-project",
		WorkerName:  "worker-01",
		BaseBranch:  "main",
		AiryraHost:  "192.168.1.1",
		AiryraPort:  7432,
	}

	result := GenerateMinimalCLAUDEMD(ctx)

	if result == "" {
		t.Fatal("GenerateMinimalCLAUDEMD() returned empty string")
	}

	// Check for quick reference header
	if !strings.Contains(result, "Quick Reference") {
		t.Error("GenerateMinimalCLAUDEMD() missing Quick Reference header")
	}

	// Check for summary line with worker, project, and base
	if !strings.Contains(result, "worker-01") {
		t.Error("GenerateMinimalCLAUDEMD() missing worker name")
	}
	if !strings.Contains(result, "test-project") {
		t.Error("GenerateMinimalCLAUDEMD() missing project name")
	}
	if !strings.Contains(result, "main") {
		t.Error("GenerateMinimalCLAUDEMD() missing base branch")
	}
}

func TestGenerateMinimalCLAUDEMD_ContainsCommands(t *testing.T) {
	ctx := &Context{
		ProjectName: "myproject",
		WorkerName:  "worker",
		BaseBranch:  "main",
		AiryraHost:  "10.0.0.1",
		AiryraPort:  8080,
	}

	result := GenerateMinimalCLAUDEMD(ctx)

	// Check for commands section
	if !strings.Contains(result, "## Commands") {
		t.Error("GenerateMinimalCLAUDEMD() missing Commands section")
	}

	// Check for all airyra commands
	commands := []string{
		"List tasks",
		"Claim task",
		"Complete",
		"Block",
		"Release",
	}

	for _, cmd := range commands {
		if !strings.Contains(result, cmd) {
			t.Errorf("GenerateMinimalCLAUDEMD() missing command: %q", cmd)
		}
	}

	// Check for host and port
	if !strings.Contains(result, "10.0.0.1") {
		t.Error("GenerateMinimalCLAUDEMD() missing host in commands")
	}
	if !strings.Contains(result, "8080") {
		t.Error("GenerateMinimalCLAUDEMD() missing port in commands")
	}
}

func TestGenerateMinimalCLAUDEMD_WithCustomContext(t *testing.T) {
	customText := "Always run make lint before pushing."

	ctx := &Context{
		ProjectName:   "myproject",
		WorkerName:    "worker",
		BaseBranch:    "main",
		AiryraHost:    "localhost",
		AiryraPort:    7432,
		CustomContext: customText,
	}

	result := GenerateMinimalCLAUDEMD(ctx)

	// Check for notes section
	if !strings.Contains(result, "## Notes") {
		t.Error("GenerateMinimalCLAUDEMD() missing Notes section when custom context provided")
	}

	// Check for custom text
	if !strings.Contains(result, customText) {
		t.Error("GenerateMinimalCLAUDEMD() missing custom context text")
	}
}

func TestGenerateMinimalCLAUDEMD_WithoutCustomContext(t *testing.T) {
	ctx := &Context{
		ProjectName:   "myproject",
		WorkerName:    "worker",
		BaseBranch:    "main",
		AiryraHost:    "localhost",
		AiryraPort:    7432,
		CustomContext: "",
	}

	result := GenerateMinimalCLAUDEMD(ctx)

	// Should not have notes section when empty
	if strings.Contains(result, "## Notes") {
		t.Error("GenerateMinimalCLAUDEMD() should not include Notes when custom context is empty")
	}
}

func TestGenerateMinimalCLAUDEMD_IsShorterThanFull(t *testing.T) {
	ctx := &Context{
		ProjectName: "myproject",
		WorkerName:  "worker",
		BaseBranch:  "main",
		AiryraHost:  "localhost",
		AiryraPort:  7432,
	}

	full := GenerateCLAUDEMD(ctx)
	minimal := GenerateMinimalCLAUDEMD(ctx)

	if len(minimal) >= len(full) {
		t.Errorf("GenerateMinimalCLAUDEMD() length %d should be less than GenerateCLAUDEMD() length %d",
			len(minimal), len(full))
	}
}

func TestGenerateCLAUDEMD_WorkerInfoSection(t *testing.T) {
	ctx := &Context{
		ProjectName: "myproject",
		WorkerName:  "worker-01",
		TaskBranch:  "isollm/task-456",
		BaseBranch:  "develop",
		AiryraHost:  "localhost",
		AiryraPort:  7432,
	}

	result := GenerateCLAUDEMD(ctx)

	// Check for worker information section
	if !strings.Contains(result, "## Worker Information") {
		t.Error("GenerateCLAUDEMD() missing Worker Information section")
	}

	// Check for bullet points with correct formatting
	expectedBullets := []string{
		"**Project**: myproject",
		"**Worker**: worker-01",
		"**Base Branch**: develop",
		"**Task Branch**: isollm/task-456",
	}

	for _, bullet := range expectedBullets {
		if !strings.Contains(result, bullet) {
			t.Errorf("GenerateCLAUDEMD() missing worker info: %q", bullet)
		}
	}
}

func TestContextFields(t *testing.T) {
	// Test that Context struct has all expected fields
	ctx := Context{
		ProjectName:   "project",
		WorkerName:    "worker",
		TaskBranch:    "branch",
		BaseBranch:    "main",
		AiryraHost:    "host",
		AiryraPort:    1234,
		CustomContext: "custom",
	}

	if ctx.ProjectName != "project" {
		t.Error("Context.ProjectName not set correctly")
	}
	if ctx.WorkerName != "worker" {
		t.Error("Context.WorkerName not set correctly")
	}
	if ctx.TaskBranch != "branch" {
		t.Error("Context.TaskBranch not set correctly")
	}
	if ctx.BaseBranch != "main" {
		t.Error("Context.BaseBranch not set correctly")
	}
	if ctx.AiryraHost != "host" {
		t.Error("Context.AiryraHost not set correctly")
	}
	if ctx.AiryraPort != 1234 {
		t.Error("Context.AiryraPort not set correctly")
	}
	if ctx.CustomContext != "custom" {
		t.Error("Context.CustomContext not set correctly")
	}
}
