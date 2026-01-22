package barerepo

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"isollm/internal/git"
)

const (
	// BranchPrefix is the prefix for task branches
	BranchPrefix = "isollm/"
	// DefaultBaseDir is the default location for bare repos
	DefaultBaseDir = ".isollm"
)

// BareRepo manages a bare git repository used as a hub between host and containers
type BareRepo struct {
	path     string
	executor git.Executor
}

// BranchInfo contains information about a task branch
type BranchInfo struct {
	Name       string // Full branch name (e.g., "isollm/ar-a1b2")
	TaskID     string // Just the task ID (e.g., "ar-a1b2")
	CommitHash string // Latest commit hash
	Subject    string // Latest commit subject
}

// New creates a BareRepo instance for an existing bare repo
func New(barePath string) *BareRepo {
	return &BareRepo{
		path:     barePath,
		executor: git.DefaultExecutor,
	}
}

// NewWithExecutor creates a BareRepo with a custom executor (for testing)
func NewWithExecutor(barePath string, executor git.Executor) *BareRepo {
	return &BareRepo{
		path:     barePath,
		executor: executor,
	}
}

// GetMountPath returns the standard bare repo location for a project
// ~/.isollm/<project>.git
func GetMountPath(projectName string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, DefaultBaseDir, projectName+".git"), nil
}

// Exists checks if a bare repo exists at the given path
func Exists(barePath string) bool {
	// A bare repo has a HEAD file directly in the directory
	headPath := filepath.Join(barePath, "HEAD")
	info, err := os.Stat(headPath)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// Create creates a new bare repo by cloning from a working directory
// Sets gc.auto 0 to prevent corruption from concurrent pushes
func Create(projectPath, barePath string) (*BareRepo, error) {
	executor := git.DefaultExecutor

	// Ensure parent directory exists
	parentDir := filepath.Dir(barePath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Clone as bare repo
	if _, err := executor.Run("", "clone", "--bare", projectPath, barePath); err != nil {
		return nil, fmt.Errorf("failed to create bare clone: %w", err)
	}

	// Disable auto gc to prevent corruption from concurrent pushes
	if err := executor.RunSilent(barePath, "config", "gc.auto", "0"); err != nil {
		return nil, fmt.Errorf("failed to disable gc.auto: %w", err)
	}

	return &BareRepo{
		path:     barePath,
		executor: executor,
	}, nil
}

// Path returns the path to the bare repo
func (b *BareRepo) Path() string {
	return b.path
}

// IsHostAhead returns the number of commits the host is ahead of the bare repo
// on the specified branch. Used for stale repo warning on `isollm up`
func (b *BareRepo) IsHostAhead(projectPath, branch string) (int, error) {
	// Get the host's commit hash for the branch
	hostHash, err := b.executor.Run(projectPath, "rev-parse", branch)
	if err != nil {
		return 0, fmt.Errorf("failed to get host commit: %w", err)
	}

	// Get the bare repo's commit hash for the branch
	bareHash, err := b.executor.Run(b.path, "rev-parse", branch)
	if err != nil {
		// Branch might not exist in bare repo yet
		return 0, nil
	}

	if hostHash == bareHash {
		return 0, nil
	}

	// Count commits between bare and host
	revRange := bareHash + ".." + hostHash
	output, err := b.executor.Run(projectPath, "rev-list", "--count", revRange)
	if err != nil {
		return 0, fmt.Errorf("failed to count commits: %w", err)
	}

	count, err := strconv.Atoi(output)
	if err != nil {
		return 0, fmt.Errorf("failed to parse commit count: %w", err)
	}

	return count, nil
}

// PushToBare pushes a branch from the project to the bare repo
func (b *BareRepo) PushToBare(projectPath, branch string) error {
	refspec := fmt.Sprintf("%s:%s", branch, branch)
	if err := b.executor.RunSilent(projectPath, "push", b.path, refspec); err != nil {
		return fmt.Errorf("failed to push to bare repo: %w", err)
	}
	return nil
}

// PullFromBare fetches all isollm/* branches from bare repo to the project
func (b *BareRepo) PullFromBare(projectPath string) error {
	// Fetch all isollm/* branches as remote-tracking branches
	refspec := "refs/heads/" + BranchPrefix + "*:refs/remotes/" + BranchPrefix + "*"
	if _, err := b.executor.Run(projectPath, "fetch", b.path, refspec); err != nil {
		return fmt.Errorf("failed to fetch from bare repo: %w", err)
	}
	return nil
}

// ListTaskBranches returns all branches matching the isollm/* pattern
func (b *BareRepo) ListTaskBranches() ([]BranchInfo, error) {
	// List branches matching the prefix
	output, err := b.executor.Run(b.path, "for-each-ref",
		"--format=%(refname:short)\t%(objectname:short)\t%(subject)",
		"refs/heads/"+BranchPrefix+"*")
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	if output == "" {
		return nil, nil
	}

	var branches []BranchInfo
	for _, line := range strings.Split(output, "\n") {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 2 {
			continue
		}

		name := parts[0]
		taskID := strings.TrimPrefix(name, BranchPrefix)

		info := BranchInfo{
			Name:       name,
			TaskID:     taskID,
			CommitHash: parts[1],
		}
		if len(parts) >= 3 {
			info.Subject = parts[2]
		}

		branches = append(branches, info)
	}

	return branches, nil
}

// DeleteBranch deletes a branch from the bare repo
func (b *BareRepo) DeleteBranch(branchName string) error {
	if err := b.executor.RunSilent(b.path, "branch", "-D", branchName); err != nil {
		return fmt.Errorf("failed to delete branch %s: %w", branchName, err)
	}
	return nil
}

// GetBranchCommitCount returns the number of commits a branch is ahead of base
func (b *BareRepo) GetBranchCommitCount(branchName, baseBranch string) (int, error) {
	revRange := baseBranch + ".." + branchName
	output, err := b.executor.Run(b.path, "rev-list", "--count", revRange)
	if err != nil {
		return 0, fmt.Errorf("failed to count commits: %w", err)
	}

	count, err := strconv.Atoi(output)
	if err != nil {
		return 0, fmt.Errorf("failed to parse commit count: %w", err)
	}

	return count, nil
}

// RunGC runs garbage collection on the bare repo
// Should only be called when no workers are active
func (b *BareRepo) RunGC() error {
	if err := b.executor.RunSilent(b.path, "gc"); err != nil {
		return fmt.Errorf("failed to run gc: %w", err)
	}
	return nil
}

// HasUnpushedCommits checks if a branch has commits not in the base branch
func (b *BareRepo) HasUnpushedCommits(branchName, baseBranch string) (bool, error) {
	count, err := b.GetBranchCommitCount(branchName, baseBranch)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
