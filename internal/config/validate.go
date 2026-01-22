package config

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	// Worker limits
	MinWorkers = 1
	MaxWorkers = 20 // Reasonable limit for local machine resources

	// Port limits
	MinUserPort = 1024  // Ports below 1024 require root
	MaxPort     = 65535

	// Project name limits
	MinProjectNameLen = 2
	MaxProjectNameLen = 64
)

var (
	validProjectName = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9-]*$`)
	validBranchName  = regexp.MustCompile(`^[a-zA-Z0-9._/-]+$`)
	validLayouts     = map[string]struct{}{
		"auto": {}, "horizontal": {}, "vertical": {}, "grid": {},
	}
)

// ValidationError collects multiple validation failures
type ValidationError struct {
	Errors []string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("config validation failed:\n  - %s",
		strings.Join(e.Errors, "\n  - "))
}

// Add appends a validation error message
func (e *ValidationError) Add(msg string) {
	e.Errors = append(e.Errors, msg)
}

// HasErrors returns true if there are any validation errors
func (e *ValidationError) HasErrors() bool {
	return len(e.Errors) > 0
}

// Validate checks the config for semantic errors
func (c *Config) Validate() error {
	errs := &ValidationError{}

	// Project name
	if c.Project == "" {
		errs.Add("project name is required")
	} else {
		if !validProjectName.MatchString(c.Project) {
			errs.Add("project name must be alphanumeric with hyphens, starting with a letter")
		}
		if len(c.Project) < MinProjectNameLen {
			errs.Add(fmt.Sprintf("project name must be at least %d characters", MinProjectNameLen))
		}
		if len(c.Project) > MaxProjectNameLen {
			errs.Add(fmt.Sprintf("project name cannot exceed %d characters", MaxProjectNameLen))
		}
	}

	// Workers
	if c.Workers < MinWorkers {
		errs.Add(fmt.Sprintf("workers must be at least %d", MinWorkers))
	} else if c.Workers > MaxWorkers {
		errs.Add(fmt.Sprintf("workers cannot exceed %d", MaxWorkers))
	}

	// Image
	if c.Image == "" {
		errs.Add("image is required")
	}
	// Note: Warn (not error) if no tag - handled separately in Warnings()

	// Git config
	if c.Git.BaseBranch == "" {
		errs.Add("git.base_branch is required")
	} else if !validBranchName.MatchString(c.Git.BaseBranch) {
		errs.Add("git.base_branch contains invalid characters")
	}

	if c.Git.BranchPrefix == "/" {
		errs.Add("git.branch_prefix cannot be just '/'")
	} else if c.Git.BranchPrefix != "" && !strings.HasSuffix(c.Git.BranchPrefix, "/") {
		errs.Add("git.branch_prefix must end with '/'")
	}

	// Airyra port
	if c.Airyra.Port < MinUserPort || c.Airyra.Port > MaxPort {
		errs.Add(fmt.Sprintf("airyra.port must be between %d and %d", MinUserPort, MaxPort))
	}

	// Ports: format, duplicates, and airyra conflict
	seen := make(map[string]bool)
	airyraPort := strconv.Itoa(c.Airyra.Port)
	for _, p := range c.Ports {
		if err := validatePort(p); err != nil {
			errs.Add(fmt.Sprintf("invalid port %q: %v", p, err))
			continue
		}
		hostPort := strings.Split(p, ":")[0]
		if seen[hostPort] {
			errs.Add(fmt.Sprintf("duplicate port: %s", hostPort))
		}
		seen[hostPort] = true
		if hostPort == airyraPort {
			errs.Add(fmt.Sprintf("port %s conflicts with airyra.port", hostPort))
		}
	}

	// Zellij layout
	if _, ok := validLayouts[c.Zellij.Layout]; !ok {
		errs.Add("zellij.layout must be one of: auto, horizontal, vertical, grid")
	}

	if errs.HasErrors() {
		return errs
	}
	return nil
}

// validatePort checks if a port string is valid (port or host:container format)
func validatePort(p string) error {
	parts := strings.Split(p, ":")
	if len(parts) > 2 {
		return fmt.Errorf("invalid format, expected 'port' or 'host:container'")
	}
	for _, part := range parts {
		port, err := strconv.Atoi(part)
		if err != nil {
			return fmt.Errorf("not a valid number")
		}
		if port < 1 || port > MaxPort {
			return fmt.Errorf("port out of range (1-%d)", MaxPort)
		}
	}
	return nil
}

// Warnings returns non-fatal issues (call after Validate)
func (c *Config) Warnings() []string {
	var warnings []string
	if c.Image != "" && !strings.Contains(c.Image, ":") {
		warnings = append(warnings, "image has no tag, will use :latest")
	}
	return warnings
}

// LoadAndValidate reads and validates the config
func LoadAndValidate(dir string) (*Config, error) {
	cfg, err := Load(dir)
	if err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}
