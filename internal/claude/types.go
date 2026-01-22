package claude

// Environment holds environment variables for Claude workers.
// These are set in the container to allow Claude to interact with airyra.
type Environment struct {
	// AiryraHost is the host address for the airyra server
	AiryraHost string
	// AiryraPort is the port for the airyra server
	AiryraPort int
	// AiryraProject is the project name in airyra
	AiryraProject string
	// AiryraAgent is the unique agent identifier for this worker
	AiryraAgent string
	// ProjectPath is the path to the project inside the container
	ProjectPath string
	// BareRepoPath is the path to the bare git repo mount
	BareRepoPath string
}

// Context holds information for generating the CLAUDE.md file.
// This provides Claude with context about the task and workflow.
type Context struct {
	// ProjectName is the name of the isollm project
	ProjectName string
	// WorkerName is the name of this worker container
	WorkerName string
	// TaskBranch is the branch name for the current task
	TaskBranch string
	// BaseBranch is the base branch to create task branches from
	BaseBranch string
	// AiryraHost is the host address for airyra commands
	AiryraHost string
	// AiryraPort is the port for airyra commands
	AiryraPort int
	// CustomContext is additional context to include in CLAUDE.md
	CustomContext string
}

// LaunchConfig holds configuration for launching Claude in a worker.
type LaunchConfig struct {
	// Command is the Claude CLI command (e.g., "claude")
	Command string
	// Args are additional arguments to pass to Claude
	Args []string
	// WorkDir is the working directory to run Claude in
	WorkDir string
	// Env holds environment variables to set
	Env *Environment
	// Context holds context for CLAUDE.md generation
	Context *Context
}
