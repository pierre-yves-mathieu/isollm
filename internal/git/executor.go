package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Executor runs git commands
type Executor interface {
	Run(dir string, args ...string) (string, error)
	RunSilent(dir string, args ...string) error
}

// DefaultExecutor is the default git executor that runs actual git commands
var DefaultExecutor Executor = &realExecutor{}

type realExecutor struct{}

func (e *realExecutor) Run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s failed: %w\n%s", strings.Join(args, " "), err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

func (e *realExecutor) RunSilent(dir string, args ...string) error {
	_, err := e.Run(dir, args...)
	return err
}
