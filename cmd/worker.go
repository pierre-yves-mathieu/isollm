package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"isollm/internal/config"
	"isollm/internal/worker"
)

var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "Manage worker containers",
	Long: `Manage isolated LXC containers that serve as development workers.

Workers are containers with:
- The bare repo mounted at /repo.git
- A cloned working copy at /home/dev/project
- A "clean" snapshot for easy reset`,
}

// workerAddCmd creates new worker(s)
var workerAddCmd = &cobra.Command{
	Use:   "add [name]",
	Short: "Create a new worker",
	Long: `Create a new worker container.

If no name is given, the next available worker-N name is used.
The worker is created with the bare repo mounted and a clean snapshot.`,
	RunE: runWorkerAdd,
}

var addCount int

// workerListCmd lists all workers
var workerListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List workers",
	Long:    `List all workers with their status, IP, and current task.`,
	RunE:    runWorkerList,
}

// workerStartCmd starts a worker
var workerStartCmd = &cobra.Command{
	Use:   "start <name>",
	Short: "Start a stopped worker",
	Args:  cobra.ExactArgs(1),
	RunE:  runWorkerStart,
}

// workerStopCmd stops a worker
var workerStopCmd = &cobra.Command{
	Use:   "stop <name>",
	Short: "Stop a running worker",
	Args:  cobra.ExactArgs(1),
	RunE:  runWorkerStop,
}

// workerRemoveCmd removes a worker
var workerRemoveCmd = &cobra.Command{
	Use:     "remove <name>",
	Aliases: []string{"rm"},
	Short:   "Remove a worker",
	Long:    `Remove a worker container. Use -f to force removal of a running container.`,
	Args:    cobra.ExactArgs(1),
	RunE:    runWorkerRemove,
}

var removeForce bool

// workerResetCmd resets a worker to clean state
var workerResetCmd = &cobra.Command{
	Use:   "reset <name>",
	Short: "Reset a worker to clean state",
	Long: `Reset a worker to its clean snapshot state.

This restores the container to its state immediately after creation,
with a freshly cloned repo. Any uncommitted changes are lost.`,
	Args: cobra.ExactArgs(1),
	RunE: runWorkerReset,
}

// workerShellCmd opens an interactive shell
var workerShellCmd = &cobra.Command{
	Use:   "shell <name>",
	Short: "Open a shell in a worker",
	Long:  `Open an interactive shell in a worker container as the dev user.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runWorkerShell,
}

// workerExecCmd runs a command in a worker
var workerExecCmd = &cobra.Command{
	Use:   "exec <name> -- <command...>",
	Short: "Run a command in a worker",
	Long: `Run a command inside a worker container and print the output.

Example: isollm worker exec worker-1 -- git status`,
	Args: cobra.MinimumNArgs(2),
	RunE: runWorkerExec,
}

// workerStatusCmd shows detailed status of a worker
var workerStatusCmd = &cobra.Command{
	Use:   "status <name>",
	Short: "Show detailed worker status",
	Args:  cobra.ExactArgs(1),
	RunE:  runWorkerStatus,
}

func init() {
	workerAddCmd.Flags().IntVarP(&addCount, "count", "n", 1, "Number of workers to create")

	workerRemoveCmd.Flags().BoolVarP(&removeForce, "force", "f", false, "Force removal of running container")

	workerCmd.AddCommand(workerAddCmd)
	workerCmd.AddCommand(workerListCmd)
	workerCmd.AddCommand(workerStartCmd)
	workerCmd.AddCommand(workerStopCmd)
	workerCmd.AddCommand(workerRemoveCmd)
	workerCmd.AddCommand(workerResetCmd)
	workerCmd.AddCommand(workerShellCmd)
	workerCmd.AddCommand(workerExecCmd)
	workerCmd.AddCommand(workerStatusCmd)

	rootCmd.AddCommand(workerCmd)
}

func getManager() (*worker.Manager, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	projectDir, err := config.FindProjectRoot(dir)
	if err != nil {
		return nil, err
	}

	cfg, err := config.Load(projectDir)
	if err != nil {
		return nil, err
	}

	return worker.NewManager(projectDir, cfg)
}

func runWorkerAdd(cmd *cobra.Command, args []string) error {
	mgr, err := getManager()
	if err != nil {
		return err
	}

	// If a specific name is given, create just that worker
	if len(args) > 0 {
		name := args[0]
		fmt.Printf("Creating worker %s...\n", name)
		if err := mgr.CreateWorker(name); err != nil {
			return err
		}
		fmt.Printf("Worker %s created\n", name)
		return nil
	}

	// Create N workers
	for i := 0; i < addCount; i++ {
		fmt.Printf("Creating worker...\n")
		if err := mgr.CreateWorker(""); err != nil {
			return err
		}
	}

	if addCount == 1 {
		fmt.Println("Worker created")
	} else {
		fmt.Printf("%d workers created\n", addCount)
	}
	return nil
}

func runWorkerList(cmd *cobra.Command, args []string) error {
	mgr, err := getManager()
	if err != nil {
		return err
	}

	workers, err := mgr.List()
	if err != nil {
		return err
	}

	if len(workers) == 0 {
		fmt.Println("No workers")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tIP\tTASK\tBRANCH")

	for _, worker := range workers {
		ip := worker.IP
		if ip == "" {
			ip = "-"
		}
		task := worker.TaskID
		if task == "" {
			task = "-"
		}
		branch := worker.Branch
		if branch == "" {
			branch = "-"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			worker.Name,
			worker.Status,
			ip,
			task,
			branch,
		)
	}

	return w.Flush()
}

func runWorkerStart(cmd *cobra.Command, args []string) error {
	mgr, err := getManager()
	if err != nil {
		return err
	}

	name := args[0]
	if err := mgr.Start(name); err != nil {
		return err
	}

	fmt.Printf("Started %s\n", name)
	return nil
}

func runWorkerStop(cmd *cobra.Command, args []string) error {
	mgr, err := getManager()
	if err != nil {
		return err
	}

	name := args[0]
	if err := mgr.Stop(name); err != nil {
		return err
	}

	fmt.Printf("Stopped %s\n", name)
	return nil
}

func runWorkerRemove(cmd *cobra.Command, args []string) error {
	mgr, err := getManager()
	if err != nil {
		return err
	}

	name := args[0]
	if err := mgr.Remove(name, removeForce); err != nil {
		return err
	}

	fmt.Printf("Removed %s\n", name)
	return nil
}

func runWorkerReset(cmd *cobra.Command, args []string) error {
	mgr, err := getManager()
	if err != nil {
		return err
	}

	name := args[0]
	if err := mgr.Reset(name); err != nil {
		return err
	}

	fmt.Printf("Reset %s to clean state\n", name)
	return nil
}

func runWorkerShell(cmd *cobra.Command, args []string) error {
	mgr, err := getManager()
	if err != nil {
		return err
	}

	name := args[0]
	return mgr.Shell(name)
}

func runWorkerExec(cmd *cobra.Command, args []string) error {
	mgr, err := getManager()
	if err != nil {
		return err
	}

	name := args[0]
	execCmd := args[1:]

	output, err := mgr.Exec(name, execCmd)
	if err != nil {
		return err
	}

	fmt.Print(string(output))
	return nil
}

func runWorkerStatus(cmd *cobra.Command, args []string) error {
	mgr, err := getManager()
	if err != nil {
		return err
	}

	name := args[0]

	status, err := mgr.Status(name)
	if err != nil {
		return err
	}

	ip, _ := mgr.IP(name)
	task, _ := mgr.GetTask(name)
	snapshots, _ := mgr.ListSnapshots(name)

	fmt.Printf("Worker: %s\n", name)
	fmt.Printf("Status: %s\n", status)
	if ip != "" {
		fmt.Printf("IP: %s\n", ip)
	}

	if task != nil && task.TaskID != "" {
		fmt.Printf("\nTask: %s\n", task.TaskID)
		fmt.Printf("Branch: %s\n", task.Branch)
		fmt.Printf("Claimed: %s\n", task.ClaimedAt.Format("2006-01-02 15:04:05"))
	}

	if len(snapshots) > 0 {
		fmt.Printf("\nSnapshots:\n")
		for _, s := range snapshots {
			desc := ""
			if s.Description != "" {
				desc = " - " + s.Description
			}
			fmt.Printf("  %s%s\n", s.Name, desc)
		}
	}

	return nil
}

// Helper function to check if a string slice contains a value
func contains(slice []string, val string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, val) {
			return true
		}
	}
	return false
}
