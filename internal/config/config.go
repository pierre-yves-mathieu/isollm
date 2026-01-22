package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	// ConfigFileName is the name of the isollm config file
	ConfigFileName = "isollm.yaml"
	// StateDir is the name of the local state directory
	StateDir = ".isollm"
)

// Config represents the isollm.yaml configuration
type Config struct {
	Project string       `yaml:"project"`
	Workers int          `yaml:"workers"`
	Image   string       `yaml:"image"`
	Setup   string       `yaml:"setup_script,omitempty"`
	Git     GitConfig    `yaml:"git"`
	Claude  ClaudeConfig `yaml:"claude"`
	Airyra  AiryraConfig `yaml:"airyra"`
	Ports   []string     `yaml:"ports,omitempty"`
	Zellij  ZellijConfig `yaml:"zellij"`
}

// GitConfig contains git-related settings
type GitConfig struct {
	BaseBranch   string `yaml:"base_branch"`
	BranchPrefix string `yaml:"branch_prefix"`
	Upstream     string `yaml:"upstream,omitempty"`
}

// ClaudeConfig contains claude-related settings
type ClaudeConfig struct {
	Command string   `yaml:"command"`
	Args    []string `yaml:"args,omitempty"`
}

// AiryraConfig contains airyra-related settings
type AiryraConfig struct {
	Project string `yaml:"project,omitempty"`
	Host    string `yaml:"host,omitempty"`
	Port    int    `yaml:"port,omitempty"`
}

// ZellijConfig contains zellij-related settings
type ZellijConfig struct {
	Layout    string `yaml:"layout"`
	Dashboard bool   `yaml:"dashboard"`
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig(projectName string) *Config {
	return &Config{
		Project: projectName,
		Workers: 3,
		Image:   "ubuntu:24.04",
		Git: GitConfig{
			BaseBranch:   "main",
			BranchPrefix: "isollm/",
		},
		Claude: ClaudeConfig{
			Command: "claude",
		},
		Airyra: AiryraConfig{
			Host: "localhost",
			Port: 7432,
		},
		Zellij: ZellijConfig{
			Layout:    "auto",
			Dashboard: true,
		},
	}
}

// Load reads the config from the specified directory
func Load(dir string) (*Config, error) {
	configPath := filepath.Join(dir, ConfigFileName)

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file not found: %s", configPath)
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Apply defaults for missing values
	applyDefaults(&cfg)

	return &cfg, nil
}

// Save writes the config to the specified directory
func Save(dir string, cfg *Config) error {
	configPath := filepath.Join(dir, ConfigFileName)

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// CreateStateDir creates the .isollm directory
func CreateStateDir(dir string) error {
	stateDir := filepath.Join(dir, StateDir)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}
	return nil
}

// Exists checks if a config file exists in the directory
func Exists(dir string) bool {
	configPath := filepath.Join(dir, ConfigFileName)
	_, err := os.Stat(configPath)
	return err == nil
}

// applyDefaults fills in missing values with defaults
func applyDefaults(cfg *Config) {
	if cfg.Workers == 0 {
		cfg.Workers = 3
	}
	if cfg.Image == "" {
		cfg.Image = "ubuntu:24.04"
	}
	if cfg.Git.BaseBranch == "" {
		cfg.Git.BaseBranch = "main"
	}
	if cfg.Git.BranchPrefix == "" {
		cfg.Git.BranchPrefix = "isollm/"
	}
	if cfg.Claude.Command == "" {
		cfg.Claude.Command = "claude"
	}
	if cfg.Airyra.Host == "" {
		cfg.Airyra.Host = "localhost"
	}
	if cfg.Airyra.Port == 0 {
		cfg.Airyra.Port = 7432
	}
	if cfg.Airyra.Project == "" {
		cfg.Airyra.Project = cfg.Project
	}
	if cfg.Zellij.Layout == "" {
		cfg.Zellij.Layout = "auto"
	}
}

// FindProjectRoot walks up from the current directory to find isollm.yaml
func FindProjectRoot(startDir string) (string, error) {
	dir := startDir
	for {
		if Exists(dir) {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root
			return "", fmt.Errorf("not in an isollm project (no %s found)", ConfigFileName)
		}
		dir = parent
	}
}
