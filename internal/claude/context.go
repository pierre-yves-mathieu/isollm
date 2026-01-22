package claude

import (
	"fmt"
	"strings"
)

// GenerateCLAUDEMD generates the content for the CLAUDE.md file.
// This file provides Claude with context about the task workflow and environment.
func GenerateCLAUDEMD(ctx *Context) string {
	var b strings.Builder

	// Header
	b.WriteString("# CLAUDE.md - isollm Worker Instructions\n\n")
	b.WriteString("You are running inside an isolated worker container managed by isollm.\n")
	b.WriteString("This file contains important information about your workflow and environment.\n\n")

	// Worker info
	b.WriteString("## Worker Information\n\n")
	b.WriteString(fmt.Sprintf("- **Project**: %s\n", ctx.ProjectName))
	b.WriteString(fmt.Sprintf("- **Worker**: %s\n", ctx.WorkerName))
	b.WriteString(fmt.Sprintf("- **Base Branch**: %s\n", ctx.BaseBranch))
	if ctx.TaskBranch != "" {
		b.WriteString(fmt.Sprintf("- **Task Branch**: %s\n", ctx.TaskBranch))
	}
	b.WriteString("\n")

	// Task workflow
	b.WriteString("## Task Workflow\n\n")
	b.WriteString("### 1. Claiming a Task\n\n")
	b.WriteString("Before starting work, claim a task from airyra:\n\n")
	b.WriteString("```bash\n")
	b.WriteString("# List available tasks\n")
	b.WriteString(fmt.Sprintf("airyra task list --host %s --port %d\n\n", ctx.AiryraHost, ctx.AiryraPort))
	b.WriteString("# Claim the next ready task\n")
	b.WriteString(fmt.Sprintf("airyra task claim --host %s --port %d\n", ctx.AiryraHost, ctx.AiryraPort))
	b.WriteString("```\n\n")

	b.WriteString("### 2. Creating a Task Branch\n\n")
	b.WriteString("After claiming a task, create a branch for your work:\n\n")
	b.WriteString("```bash\n")
	b.WriteString("# Ensure you're on the base branch\n")
	b.WriteString(fmt.Sprintf("git checkout %s\n", ctx.BaseBranch))
	b.WriteString("git pull origin %s\n\n")
	b.WriteString("# Create task branch (use the task ID from airyra)\n")
	b.WriteString("git checkout -b isollm/<task-id>\n")
	b.WriteString("```\n\n")

	// Git workflow
	b.WriteString("### 3. Git Commit Workflow\n\n")
	b.WriteString("Make frequent, atomic commits as you work:\n\n")
	b.WriteString("```bash\n")
	b.WriteString("# Stage changes\n")
	b.WriteString("git add <files>\n\n")
	b.WriteString("# Commit with clear message\n")
	b.WriteString("git commit -m \"feat: description of change\"\n\n")
	b.WriteString("# Push to remote\n")
	b.WriteString("git push origin HEAD\n")
	b.WriteString("```\n\n")

	b.WriteString("**Commit message conventions:**\n")
	b.WriteString("- `feat:` - New feature\n")
	b.WriteString("- `fix:` - Bug fix\n")
	b.WriteString("- `refactor:` - Code refactoring\n")
	b.WriteString("- `docs:` - Documentation changes\n")
	b.WriteString("- `test:` - Adding or modifying tests\n")
	b.WriteString("- `chore:` - Maintenance tasks\n\n")

	// Completing task
	b.WriteString("### 4. Completing a Task\n\n")
	b.WriteString("When the task is done:\n\n")
	b.WriteString("```bash\n")
	b.WriteString("# Ensure all changes are pushed\n")
	b.WriteString("git push origin HEAD\n\n")
	b.WriteString("# Mark task as complete\n")
	b.WriteString(fmt.Sprintf("airyra task done --host %s --port %d\n", ctx.AiryraHost, ctx.AiryraPort))
	b.WriteString("```\n\n")

	// Handling blocks
	b.WriteString("### 5. Handling Blockers\n\n")
	b.WriteString("If you encounter a blocker:\n\n")
	b.WriteString("```bash\n")
	b.WriteString("# Mark task as blocked\n")
	b.WriteString(fmt.Sprintf("airyra task block --host %s --port %d\n", ctx.AiryraHost, ctx.AiryraPort))
	b.WriteString("```\n\n")
	b.WriteString("Describe the blocker clearly so it can be addressed.\n\n")

	// Releasing task
	b.WriteString("### 6. Releasing a Task\n\n")
	b.WriteString("If you need to stop working on a task without completing it:\n\n")
	b.WriteString("```bash\n")
	b.WriteString("# Commit and push any work in progress\n")
	b.WriteString("git add .\n")
	b.WriteString("git commit -m \"wip: partial progress on task\"\n")
	b.WriteString("git push origin HEAD\n\n")
	b.WriteString("# Release the task back to the queue\n")
	b.WriteString(fmt.Sprintf("airyra task release --host %s --port %d\n", ctx.AiryraHost, ctx.AiryraPort))
	b.WriteString("```\n\n")

	// Environment section
	b.WriteString("## Environment\n\n")
	b.WriteString("The following environment variables are set:\n\n")
	b.WriteString("| Variable | Description |\n")
	b.WriteString("|----------|-------------|\n")
	b.WriteString(fmt.Sprintf("| `AIRYRA_HOST` | %s |\n", ctx.AiryraHost))
	b.WriteString(fmt.Sprintf("| `AIRYRA_PORT` | %d |\n", ctx.AiryraPort))
	b.WriteString("| `AIRYRA_PROJECT` | Project name in airyra |\n")
	b.WriteString("| `AIRYRA_AGENT` | Worker agent identifier |\n")
	b.WriteString("| `ISOLLM_PROJECT_PATH` | Path to project in container |\n")
	b.WriteString("| `ISOLLM_BARE_REPO` | Path to bare git repository |\n\n")

	// Important notes
	b.WriteString("## Important Notes\n\n")
	b.WriteString("1. **Always push your work** - The bare repo is the bridge between this container and the host.\n")
	b.WriteString("2. **Commit frequently** - Small, focused commits are easier to review and merge.\n")
	b.WriteString("3. **Communicate blockers** - Use `airyra task block` immediately when stuck.\n")
	b.WriteString("4. **One task at a time** - Complete or release a task before claiming another.\n")
	b.WriteString("5. **Stay in your branch** - Don't modify the base branch directly.\n\n")

	// Custom context
	if ctx.CustomContext != "" {
		b.WriteString("## Project-Specific Instructions\n\n")
		b.WriteString(ctx.CustomContext)
		b.WriteString("\n")
	}

	return b.String()
}

// GenerateMinimalCLAUDEMD generates a minimal CLAUDE.md for quick reference.
func GenerateMinimalCLAUDEMD(ctx *Context) string {
	var b strings.Builder

	b.WriteString("# isollm Worker Quick Reference\n\n")
	b.WriteString(fmt.Sprintf("**Worker**: %s | **Project**: %s | **Base**: %s\n\n", ctx.WorkerName, ctx.ProjectName, ctx.BaseBranch))

	b.WriteString("## Commands\n\n")
	b.WriteString("```bash\n")
	b.WriteString(fmt.Sprintf("# List tasks:    airyra task list --host %s --port %d\n", ctx.AiryraHost, ctx.AiryraPort))
	b.WriteString(fmt.Sprintf("# Claim task:    airyra task claim --host %s --port %d\n", ctx.AiryraHost, ctx.AiryraPort))
	b.WriteString(fmt.Sprintf("# Complete:      airyra task done --host %s --port %d\n", ctx.AiryraHost, ctx.AiryraPort))
	b.WriteString(fmt.Sprintf("# Block:         airyra task block --host %s --port %d\n", ctx.AiryraHost, ctx.AiryraPort))
	b.WriteString(fmt.Sprintf("# Release:       airyra task release --host %s --port %d\n", ctx.AiryraHost, ctx.AiryraPort))
	b.WriteString("```\n")

	if ctx.CustomContext != "" {
		b.WriteString("\n## Notes\n\n")
		b.WriteString(ctx.CustomContext)
		b.WriteString("\n")
	}

	return b.String()
}
