package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"isollm/internal/barerepo"
	"isollm/internal/config"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Git sync with bare repo",
	Long:  `Sync between host repo and bare repo. Workers push to bare repo; you fetch from it.`,
}

var syncStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show sync state between host, bare repo, and workers",
	RunE:  runSyncStatus,
}

var syncPullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Fetch task branches from bare repo into host repo",
	RunE:  runSyncPull,
}

var syncPushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push host changes to bare repo",
	RunE:  runSyncPush,
}

func init() {
	syncCmd.AddCommand(syncStatusCmd)
	syncCmd.AddCommand(syncPullCmd)
	syncCmd.AddCommand(syncPushCmd)
	rootCmd.AddCommand(syncCmd)
}

func runSyncStatus(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	projectDir, err := config.FindProjectRoot(dir)
	if err != nil {
		return err
	}

	cfg, err := config.Load(projectDir)
	if err != nil {
		return err
	}

	barePath, err := barerepo.GetMountPath(cfg.Project)
	if err != nil {
		return err
	}

	fmt.Printf("Sync: %s\n", cfg.Project)
	fmt.Println("─────────────────────────────────────────────────")
	fmt.Println()

	fmt.Printf("Host repo: %s\n", projectDir)

	// Check if bare repo exists
	if !barerepo.Exists(barePath) {
		fmt.Printf("  Status: ⚠ Bare repo not created yet\n")
		fmt.Printf("  Run 'isollm up' to create it\n")
		return nil
	}

	repo := barerepo.New(barePath)

	// Check if host is ahead
	ahead, err := repo.IsHostAhead(projectDir, cfg.Git.BaseBranch)
	if err != nil {
		fmt.Printf("  Status: ⚠ Could not check sync status: %v\n", err)
	} else if ahead > 0 {
		fmt.Printf("  Status: ⚠ Host has %d commits not in bare repo\n", ahead)
		fmt.Printf("  Run 'isollm sync push' to sync\n")
	} else {
		fmt.Printf("  Status: ✓ In sync with bare repo\n")
	}

	fmt.Println()
	fmt.Printf("Bare repo: %s\n", barePath)

	// List task branches
	branches, err := repo.ListTaskBranches()
	if err != nil {
		fmt.Printf("  Error listing branches: %v\n", err)
		return nil
	}

	if len(branches) == 0 {
		fmt.Println("  No task branches")
	} else {
		fmt.Println("  Task branches:")
		for _, branch := range branches {
			count, _ := repo.GetBranchCommitCount(branch.Name, cfg.Git.BaseBranch)
			fmt.Printf("    %s  +%d commits  %q\n", branch.Name, count, branch.Subject)
		}
	}

	return nil
}

func runSyncPull(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	projectDir, err := config.FindProjectRoot(dir)
	if err != nil {
		return err
	}

	cfg, err := config.Load(projectDir)
	if err != nil {
		return err
	}

	barePath, err := barerepo.GetMountPath(cfg.Project)
	if err != nil {
		return err
	}

	if !barerepo.Exists(barePath) {
		return fmt.Errorf("bare repo does not exist: %s\nRun 'isollm up' first", barePath)
	}

	repo := barerepo.New(barePath)

	fmt.Printf("Fetching task branches from bare repo...\n")
	if err := repo.PullFromBare(projectDir); err != nil {
		return err
	}

	// List what was fetched
	branches, err := repo.ListTaskBranches()
	if err != nil {
		return err
	}

	if len(branches) == 0 {
		fmt.Println("No task branches to fetch")
	} else {
		fmt.Printf("Fetched %d task branches:\n", len(branches))
		for _, branch := range branches {
			fmt.Printf("  %s\n", branch.Name)
		}
		fmt.Println()
		fmt.Println("Merge with standard git:")
		fmt.Printf("  git merge %s<task-id>\n", barerepo.BranchPrefix)
	}

	return nil
}

func runSyncPush(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	projectDir, err := config.FindProjectRoot(dir)
	if err != nil {
		return err
	}

	cfg, err := config.Load(projectDir)
	if err != nil {
		return err
	}

	barePath, err := barerepo.GetMountPath(cfg.Project)
	if err != nil {
		return err
	}

	if !barerepo.Exists(barePath) {
		return fmt.Errorf("bare repo does not exist: %s\nRun 'isollm up' first", barePath)
	}

	repo := barerepo.New(barePath)

	fmt.Printf("Pushing %s to bare repo...\n", cfg.Git.BaseBranch)
	if err := repo.PushToBare(projectDir, cfg.Git.BaseBranch); err != nil {
		return err
	}

	fmt.Println("Done. Workers can now pull your changes.")

	return nil
}
