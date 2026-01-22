package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"isollm/internal/config"
	"isollm/internal/session"
)

var (
	downDestroy bool
	downSave    bool
	downYes     bool
)

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Gracefully shutdown all workers",
	Long: `Gracefully shutdown all workers in the isollm session.

This command will:
1. Check for uncommitted/unpushed work in each worker
2. Prompt to salvage or discard unsaved work (unless --yes)
3. Release claimed tasks back to the airyra queue
4. Stop the zellij session
5. Save snapshots if --save is specified
6. Stop or destroy containers based on flags
7. Run garbage collection on the bare repo

Use --destroy to remove containers after stopping.
Use --save to snapshot all workers before stopping.
Use --yes to skip all confirmations.`,
	RunE: runDown,
}

func init() {
	downCmd.Flags().BoolVar(&downDestroy, "destroy", false, "Remove containers after stopping")
	downCmd.Flags().BoolVar(&downSave, "save", false, "Snapshot all workers before stopping")
	downCmd.Flags().BoolVarP(&downYes, "yes", "y", false, "Skip confirmations")

	rootCmd.AddCommand(downCmd)
}

func runDown(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	projectDir, err := config.FindProjectRoot(dir)
	if err != nil {
		return err
	}

	cfg, err := config.Load(projectDir)
	if err != nil {
		return err
	}

	opts := session.ShutdownOptions{
		Destroy:       downDestroy,
		SaveSnapshots: downSave,
		SkipConfirm:   downYes,
	}

	shutdown, err := session.NewShutdown(projectDir, cfg, opts)
	if err != nil {
		return err
	}

	return shutdown.Execute()
}
