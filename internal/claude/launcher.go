package claude

import (
	"fmt"
	"path/filepath"
	"strings"

	"isollm/internal/config"
)

const (
	// EnvFilePath is the path to the environment file in the container
	EnvFilePath = "/home/dev/.isollm-env"
	// DefaultProjectPath is the default project path in containers
	DefaultProjectPath = "/home/dev/project"
	// DefaultBareRepoPath is the default bare repo mount path
	DefaultBareRepoPath = "/repo.git"
)

// ContainerExecer interface for executing commands in containers
type ContainerExecer interface {
	Exec(name string, cmd []string) ([]byte, error)
}

// Launcher handles preparing and launching Claude in worker containers.
type Launcher struct {
	cfg    *config.Config
	execer ContainerExecer
	hostIP string
}

// NewLauncher creates a new Launcher with the given configuration.
func NewLauncher(cfg *config.Config, execer ContainerExecer) (*Launcher, error) {
	hostIP, err := GetHostIP()
	if err != nil {
		// Fall back to configured host if we can't determine the bridge IP
		hostIP = cfg.Airyra.Host
	}

	return &Launcher{
		cfg:    cfg,
		execer: execer,
		hostIP: hostIP,
	}, nil
}

// PrepareWorker sets up the environment for Claude in a worker container.
// It writes the environment file, CLAUDE.md, and configures the shell.
func (l *Launcher) PrepareWorker(workerName string, taskBranch string) error {
	// Build environment
	env := BuildEnvironment(
		l.hostIP,
		l.cfg.Airyra.Port,
		l.cfg.Airyra.Project,
		workerName,
		DefaultProjectPath,
		DefaultBareRepoPath,
	)

	// Write environment file
	if err := l.writeEnvFile(workerName, env); err != nil {
		return fmt.Errorf("failed to write env file: %w", err)
	}

	// Build context for CLAUDE.md
	ctx := &Context{
		ProjectName: l.cfg.Project,
		WorkerName:  workerName,
		TaskBranch:  taskBranch,
		BaseBranch:  l.cfg.Git.BaseBranch,
		AiryraHost:  l.hostIP,
		AiryraPort:  l.cfg.Airyra.Port,
	}

	// Generate and write CLAUDE.md
	if err := l.writeCLAUDEMD(workerName, ctx); err != nil {
		return fmt.Errorf("failed to write CLAUDE.md: %w", err)
	}

	// Add source of env file to .bashrc
	if err := l.setupBashrc(workerName); err != nil {
		return fmt.Errorf("failed to setup .bashrc: %w", err)
	}

	return nil
}

// GetLaunchCommand returns the command to launch Claude in a worker.
func (l *Launcher) GetLaunchCommand() []string {
	cmd := []string{l.cfg.Claude.Command}
	cmd = append(cmd, l.cfg.Claude.Args...)
	return cmd
}

// GetLaunchConfig returns the full launch configuration for a worker.
func (l *Launcher) GetLaunchConfig(workerName, taskBranch string) *LaunchConfig {
	env := BuildEnvironment(
		l.hostIP,
		l.cfg.Airyra.Port,
		l.cfg.Airyra.Project,
		workerName,
		DefaultProjectPath,
		DefaultBareRepoPath,
	)

	ctx := &Context{
		ProjectName: l.cfg.Project,
		WorkerName:  workerName,
		TaskBranch:  taskBranch,
		BaseBranch:  l.cfg.Git.BaseBranch,
		AiryraHost:  l.hostIP,
		AiryraPort:  l.cfg.Airyra.Port,
	}

	return &LaunchConfig{
		Command: l.cfg.Claude.Command,
		Args:    l.cfg.Claude.Args,
		WorkDir: DefaultProjectPath,
		Env:     env,
		Context: ctx,
	}
}

// writeEnvFile writes the environment file to the container.
func (l *Launcher) writeEnvFile(workerName string, env *Environment) error {
	content := env.ToEnvFile()

	// Use printf to write the file to avoid issues with special characters
	cmd := []string{
		"bash", "-c",
		fmt.Sprintf("printf '%%s' %q > %s", content, EnvFilePath),
	}

	_, err := l.execer.Exec(workerName, cmd)
	return err
}

// writeCLAUDEMD writes the CLAUDE.md file to the project directory.
func (l *Launcher) writeCLAUDEMD(workerName string, ctx *Context) error {
	content := GenerateCLAUDEMD(ctx)
	claudeMDPath := filepath.Join(DefaultProjectPath, "CLAUDE.md")

	// Use printf to write the file
	cmd := []string{
		"bash", "-c",
		fmt.Sprintf("printf '%%s' %q > %s", content, claudeMDPath),
	}

	_, err := l.execer.Exec(workerName, cmd)
	return err
}

// setupBashrc adds source of env file to .bashrc if not already present.
func (l *Launcher) setupBashrc(workerName string) error {
	sourceCmd := fmt.Sprintf("source %s", EnvFilePath)
	marker := "# isollm environment"

	// Check if already added
	checkCmd := []string{
		"bash", "-c",
		fmt.Sprintf("grep -q %q /home/dev/.bashrc && echo exists || echo missing", marker),
	}

	output, err := l.execer.Exec(workerName, checkCmd)
	if err != nil {
		return err
	}

	if strings.TrimSpace(string(output)) == "exists" {
		return nil // Already configured
	}

	// Add source command to .bashrc
	appendCmd := []string{
		"bash", "-c",
		fmt.Sprintf("echo '\n%s\n%s' >> /home/dev/.bashrc", marker, sourceCmd),
	}

	_, err = l.execer.Exec(workerName, appendCmd)
	return err
}

// GetHostIP returns the host IP being used by this launcher.
func (l *Launcher) GetHostIP() string {
	return l.hostIP
}
