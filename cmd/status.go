package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"isollm/internal/config"
	"isollm/internal/status"
)

var (
	statusBrief bool
	statusJSON  bool
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show project status dashboard",
	Long: `Show a dashboard view of workers, tasks, sync state, and services.

Output formats:
  (default)  Full dashboard with detailed worker list
  --brief    One-line summary
  --json     Machine-readable JSON output`,
	RunE: runStatus,
}

func init() {
	statusCmd.Flags().BoolVarP(&statusBrief, "brief", "b", false, "Show one-line summary")
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "Output as JSON")

	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
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

	collector, err := status.NewCollector(projectDir, cfg)
	if err != nil {
		return fmt.Errorf("failed to create status collector: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s, err := collector.Collect(ctx)
	if err != nil {
		return fmt.Errorf("failed to collect status: %w", err)
	}

	if statusJSON {
		return printStatusJSON(s)
	}

	if statusBrief {
		return printStatusBrief(s)
	}

	return printStatusFull(s)
}

func printStatusFull(s *status.Status) error {
	// Header
	fmt.Printf("Project: %s\n", s.Project)
	fmt.Println("─────────────────────────────────────────────────")
	fmt.Println()

	// Services section
	fmt.Print("Services: ")
	airyraSymbol := serviceSymbol(s.Services.Airyra.Running)
	zellijSymbol := serviceSymbol(s.Services.Zellij.Running)
	fmt.Printf("airyra %s  zellij %s\n", airyraSymbol, zellijSymbol)
	fmt.Println()

	// Tasks section
	fmt.Printf("Tasks: %d ready, %d in-progress, %d blocked, %d completed\n",
		s.Tasks.Ready, s.Tasks.InProgress, s.Tasks.Blocked, s.Tasks.Completed)
	fmt.Println()

	// Sync section
	fmt.Printf("Sync: %s", s.Sync.HostBranch)
	if s.Sync.HostCommit != "" {
		fmt.Printf(" (%s)", s.Sync.HostCommit)
	}
	if s.Sync.HostAhead > 0 {
		fmt.Printf(" → %d commits ahead of bare repo", s.Sync.HostAhead)
	}
	fmt.Println()

	if s.Sync.TotalBranches > 0 {
		fmt.Printf("      └─ %d task branches in bare repo\n", s.Sync.TotalBranches)
	}
	fmt.Println()

	// Workers section
	fmt.Println("Workers:")
	if len(s.Workers) == 0 {
		fmt.Println("  No workers")
	} else {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "  NAME\tSTATUS\tIP\tTASK\tDURATION")

		for _, worker := range s.Workers {
			statusSymbol := workerSymbol(worker.Status)
			ip := worker.IP
			if ip == "" {
				ip = "-"
			}

			taskInfo := "-"
			if worker.TaskID != "" {
				if worker.TaskTitle != "" {
					taskInfo = fmt.Sprintf("%s: %s", worker.TaskID, truncate(worker.TaskTitle, 30))
				} else {
					taskInfo = worker.TaskID
				}
			}

			duration := "-"
			if worker.Duration > 0 {
				duration = formatDuration(worker.Duration)
			}

			fmt.Fprintf(w, "  %s\t%s %s\t%s\t%s\t%s\n",
				worker.Name,
				statusSymbol,
				worker.Status,
				ip,
				taskInfo,
				duration,
			)
		}
		w.Flush()
	}

	return nil
}

func printStatusBrief(s *status.Status) error {
	// Count running workers
	runningCount := 0
	for _, w := range s.Workers {
		if w.Status == status.WorkerStatusRunning {
			runningCount++
		}
	}

	airyraSymbol := serviceSymbol(s.Services.Airyra.Running)
	zellijSymbol := serviceSymbol(s.Services.Zellij.Running)

	fmt.Printf("workers: %d running | tasks: %d ready, %d in-progress | airyra: %s zellij: %s\n",
		runningCount,
		s.Tasks.Ready,
		s.Tasks.InProgress,
		airyraSymbol,
		zellijSymbol,
	)

	return nil
}

func printStatusJSON(s *status.Status) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(s)
}

// serviceSymbol returns the Unicode symbol for service status
func serviceSymbol(running bool) string {
	if running {
		return "●" // filled circle for running
	}
	return "○" // empty circle for not running
}

// workerSymbol returns the Unicode symbol for worker status
func workerSymbol(status string) string {
	switch status {
	case "RUNNING":
		return "●"
	case "STOPPED":
		return "○"
	default:
		return "?"
	}
}

// truncate shortens a string to maxLen, adding "..." if truncated
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if minutes > 0 {
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
	return fmt.Sprintf("%dh", hours)
}
