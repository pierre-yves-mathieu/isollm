package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"isollm/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration management",
	Long:  "View and edit isollm project configuration.",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Long:  "Display the current isollm.yaml configuration with resolved defaults.",
	RunE:  runConfigShow,
}

var configEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit configuration in $EDITOR",
	Long:  "Open isollm.yaml in your default editor.",
	RunE:  runConfigEdit,
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configEditCmd)
	rootCmd.AddCommand(configCmd)
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	projectRoot, err := config.FindProjectRoot(cwd)
	if err != nil {
		return err
	}

	cfg, err := config.Load(projectRoot)
	if err != nil {
		return err
	}

	// Validate and show errors
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Errors:\n%v\n\n", err)
	}

	// Show warnings
	for _, w := range cfg.Warnings() {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
	}

	// Pretty print as YAML
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	fmt.Printf("# Configuration: %s/isollm.yaml\n", projectRoot)
	fmt.Printf("# (defaults applied for missing values)\n\n")
	fmt.Print(string(data))

	return nil
}

func runConfigEdit(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	projectRoot, err := config.FindProjectRoot(cwd)
	if err != nil {
		return err
	}

	configPath := filepath.Join(projectRoot, config.ConfigFileName)

	// Determine editor: $EDITOR, then $VISUAL, then "vi"
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vi"
	}

	// Open editor with stdin/stdout/stderr connected
	editorCmd := exec.Command(editor, configPath)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr

	if err := editorCmd.Run(); err != nil {
		return fmt.Errorf("editor failed: %w", err)
	}

	// Validate after edit
	cfg, err := config.Load(projectRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: config has syntax errors: %v\n", err)
		fmt.Fprintf(os.Stderr, "Run 'isollm config edit' to fix.\n")
		return err
	}

	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintf(os.Stderr, "Run 'isollm config edit' to fix.\n")
		return err
	}

	// Show warnings
	for _, w := range cfg.Warnings() {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
	}

	fmt.Println("Configuration saved and validated.")
	return nil
}
