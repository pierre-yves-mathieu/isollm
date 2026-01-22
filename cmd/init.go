package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"isollm/internal/config"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new isollm project",
	Long:  `Initialize a new isollm project in the current directory.`,
	RunE:  runInit,
}

var (
	initName    string
	initWorkers int
	initImage   string
)

func init() {
	initCmd.Flags().StringVarP(&initName, "name", "n", "", "Project name (default: directory name)")
	initCmd.Flags().IntVarP(&initWorkers, "workers", "w", 3, "Number of workers")
	initCmd.Flags().StringVarP(&initImage, "image", "i", "ubuntu:24.04", "Base container image")

	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Check if already initialized
	if config.Exists(dir) {
		return fmt.Errorf("isollm project already exists in this directory")
	}

	// Determine project name
	projectName := initName
	if projectName == "" {
		projectName = filepath.Base(dir)
	}

	// Create config
	cfg := config.DefaultConfig(projectName)
	cfg.Workers = initWorkers
	cfg.Image = initImage

	// Save config
	if err := config.Save(dir, cfg); err != nil {
		return err
	}

	// Create state directory
	if err := config.CreateStateDir(dir); err != nil {
		return err
	}

	// Ensure .isollm/ is in .gitignore
	if err := appendToGitignore(dir, ".isollm/"); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not update .gitignore: %v\n", err)
	}

	fmt.Printf("Initialized isollm project: %s\n", projectName)
	fmt.Printf("  Workers: %d\n", cfg.Workers)
	fmt.Printf("  Image: %s\n", cfg.Image)
	fmt.Printf("\nCreated:\n")
	fmt.Printf("  %s\n", config.ConfigFileName)
	fmt.Printf("  %s/\n", config.StateDir)

	return nil
}

// appendToGitignore adds an entry to .gitignore if not already present
func appendToGitignore(dir, entry string) error {
	gitignorePath := filepath.Join(dir, ".gitignore")

	// Read existing content
	content, err := os.ReadFile(gitignorePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// Check if already present
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == entry {
			return nil // Already there
		}
	}

	// Append
	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Add newline if file doesn't end with one
	if len(content) > 0 && content[len(content)-1] != '\n' {
		f.WriteString("\n")
	}
	_, err = f.WriteString(entry + "\n")
	return err
}
