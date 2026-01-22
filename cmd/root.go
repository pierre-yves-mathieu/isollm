package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "isollm",
	Short: "Isolated LLM Orchestrator",
	Long: `isollm orchestrates multiple Claude instances working in parallel on tasks
in isolated LXC containers.

Your local repo is the source of truth; workers sync to it via a bare repo hub.`,
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Global flags can be added here
}
