package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	sdk "airyra/pkg/airyra"

	"isollm/internal/airyra"
	"isollm/internal/config"
)

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Manage tasks in the airyra queue",
	Long: `Manage tasks in the airyra task queue.

Tasks flow through airyra. The CLI works for humans and Claude alike.`,
}

// --- task add ---

var (
	addPriority    string
	addDescription string
	addDependsOn   string
)

var taskAddCmd = &cobra.Command{
	Use:   "add <title>",
	Short: "Add a task to the queue",
	Long: `Add a new task to the airyra task queue.

Examples:
  isollm task add "Implement login endpoint"
  isollm task add "Fix critical bug" -p critical
  isollm task add "Write tests" --depends-on ar-abc1`,
	Args: cobra.ExactArgs(1),
	RunE: runTaskAdd,
}

func runTaskAdd(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := getAiryraClient()
	if err != nil {
		return err
	}

	title := args[0]

	// Build options
	var opts []sdk.CreateTaskOption

	if addDescription != "" {
		opts = append(opts, sdk.WithDescription(addDescription))
	}

	if addPriority != "" {
		p, err := airyra.PriorityFromString(addPriority)
		if err != nil {
			return err
		}
		opts = append(opts, sdk.WithPriority(p))
	}

	// Create the task
	task, err := client.AddTask(ctx, title, opts...)
	if err != nil {
		return fmt.Errorf("failed to add task: %s", airyra.FormatError(err))
	}

	// Add dependency if specified
	if addDependsOn != "" {
		if err := client.AddDependency(ctx, task.ID, addDependsOn); err != nil {
			// Task was created but dependency failed - warn but don't fail
			fmt.Fprintf(os.Stderr, "Warning: failed to add dependency: %s\n", airyra.FormatError(err))
		}
	}

	fmt.Printf("Created task: %s\n", task.ID)
	fmt.Printf("  Title: %s\n", task.Title)
	fmt.Printf("  Priority: %s\n", airyra.PriorityToString(task.Priority))
	if addDependsOn != "" {
		fmt.Printf("  Depends on: %s\n", addDependsOn)
	}

	return nil
}

// --- task list ---

var (
	listReady      bool
	listInProgress bool
	listDone       bool
	listBlocked    bool
)

var taskListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "Show all tasks",
	Long: `List tasks in the airyra task queue.

By default, shows all tasks grouped by status.

Flags:
  --ready        Show only tasks ready to be claimed
  --in-progress  Show only tasks currently being worked on
  --done         Show only completed tasks
  --blocked      Show only blocked tasks`,
	RunE: runTaskList,
}

func runTaskList(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := getAiryraClient()
	if err != nil {
		return err
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// Determine which status filter to use
	if listReady {
		return listReadyTasks(ctx, client)
	} else if listInProgress {
		return listTasksByStatus(ctx, client, airyra.StatusInProgress)
	} else if listDone {
		return listTasksByStatus(ctx, client, airyra.StatusDone)
	} else if listBlocked {
		return listTasksByStatus(ctx, client, airyra.StatusBlocked)
	}

	// Default: show all tasks grouped by status
	return listAllTasks(ctx, client, cfg)
}

func listReadyTasks(ctx context.Context, client *airyra.Client) error {
	list, err := client.ListReadyTasks(ctx)
	if err != nil {
		return fmt.Errorf("failed to list ready tasks: %s", airyra.FormatError(err))
	}

	if len(list.Tasks) == 0 {
		fmt.Println("No ready tasks")
		return nil
	}

	fmt.Printf("Ready (%d):\n", list.Total)
	printTaskTable(list.Tasks)
	return nil
}

func listTasksByStatus(ctx context.Context, client *airyra.Client, status airyra.TaskStatus) error {
	list, err := client.ListTasks(ctx, airyra.WithStatus(status), airyra.WithPerPage(50))
	if err != nil {
		return fmt.Errorf("failed to list tasks: %s", airyra.FormatError(err))
	}

	if len(list.Tasks) == 0 {
		fmt.Printf("No %s tasks\n", status)
		return nil
	}

	fmt.Printf("%s (%d):\n", formatStatus(status), list.Total)
	if status == airyra.StatusInProgress {
		printTaskTableWithClaimer(list.Tasks)
	} else {
		printTaskTable(list.Tasks)
	}
	return nil
}

func listAllTasks(ctx context.Context, client *airyra.Client, cfg *config.Config) error {
	fmt.Printf("Tasks: %s\n", cfg.Airyra.Project)
	fmt.Println(strings.Repeat("-", 50))
	fmt.Println()

	// Ready tasks
	ready, err := client.ListReadyTasks(ctx)
	if err == nil && len(ready.Tasks) > 0 {
		fmt.Printf("Ready (%d):\n", ready.Total)
		printTaskTable(ready.Tasks)
		fmt.Println()
	}

	// In Progress
	inProgress, err := client.ListTasks(ctx, airyra.WithStatus(airyra.StatusInProgress), airyra.WithPerPage(50))
	if err == nil && len(inProgress.Tasks) > 0 {
		fmt.Printf("In Progress (%d):\n", inProgress.Total)
		printTaskTableWithClaimer(inProgress.Tasks)
		fmt.Println()
	}

	// Blocked
	blocked, err := client.ListTasks(ctx, airyra.WithStatus(airyra.StatusBlocked), airyra.WithPerPage(50))
	if err == nil && len(blocked.Tasks) > 0 {
		fmt.Printf("Blocked (%d):\n", blocked.Total)
		printTaskTable(blocked.Tasks)
		fmt.Println()
	}

	// Done (summary only)
	done, err := client.ListTasks(ctx, airyra.WithStatus(airyra.StatusDone), airyra.WithPerPage(1))
	if err == nil {
		fmt.Printf("Done (%d): use --done to show\n", done.Total)
	}

	return nil
}

func printTaskTable(tasks []*airyra.Task) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, t := range tasks {
		priority := fmt.Sprintf("[%s]", airyra.PriorityToString(t.Priority))
		fmt.Fprintf(w, "  %s\t%s\t%s\n", t.ID, priority, t.Title)
	}
	w.Flush()
}

func printTaskTableWithClaimer(tasks []*airyra.Task) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, t := range tasks {
		priority := fmt.Sprintf("[%s]", airyra.PriorityToString(t.Priority))
		claimer := ""
		if t.ClaimedBy != nil {
			claimer = fmt.Sprintf("-> %s", *t.ClaimedBy)
			if t.ClaimedAt != nil {
				dur := time.Since(*t.ClaimedAt).Round(time.Minute)
				claimer = fmt.Sprintf("-> %s (%v)", *t.ClaimedBy, dur)
			}
		}
		fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", t.ID, priority, t.Title, claimer)
	}
	w.Flush()
}

func formatStatus(status airyra.TaskStatus) string {
	switch status {
	case airyra.StatusOpen:
		return "Open"
	case airyra.StatusInProgress:
		return "In Progress"
	case airyra.StatusBlocked:
		return "Blocked"
	case airyra.StatusDone:
		return "Done"
	default:
		return string(status)
	}
}

// --- task clear ---

var clearAll bool

var taskClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear completed tasks from the queue",
	Long: `Remove completed tasks from the airyra task queue.

By default, only clears tasks with status "done".
Use --all to clear all tasks (with confirmation).`,
	RunE: runTaskClear,
}

func runTaskClear(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := getAiryraClient()
	if err != nil {
		return err
	}

	if clearAll {
		// Confirm before clearing all
		fmt.Print("This will delete ALL tasks. Type 'yes' to confirm: ")
		reader := bufio.NewReader(os.Stdin)
		confirm, _ := reader.ReadString('\n')
		confirm = strings.TrimSpace(confirm)
		if confirm != "yes" {
			fmt.Println("Cancelled")
			return nil
		}

		count, err := client.ClearAllTasks(ctx)
		if err != nil {
			return fmt.Errorf("failed to clear tasks: %s", airyra.FormatError(err))
		}
		fmt.Printf("Cleared %d tasks\n", count)
		return nil
	}

	count, err := client.ClearDoneTasks(ctx)
	if err != nil {
		return fmt.Errorf("failed to clear done tasks: %s", airyra.FormatError(err))
	}

	if count == 0 {
		fmt.Println("No completed tasks to clear")
	} else {
		fmt.Printf("Cleared %d completed tasks\n", count)
	}
	return nil
}

// --- Helper functions ---

func getAiryraClient() (*airyra.Client, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}
	return airyra.NewClientFromConfig(cfg)
}

func loadConfig() (*config.Config, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	projectDir, err := config.FindProjectRoot(dir)
	if err != nil {
		return nil, err
	}

	return config.Load(projectDir)
}

func init() {
	// task add flags
	taskAddCmd.Flags().StringVarP(&addPriority, "priority", "p", "", "Priority: critical, high, normal, low, lowest")
	taskAddCmd.Flags().StringVarP(&addDescription, "description", "D", "", "Task description")
	taskAddCmd.Flags().StringVarP(&addDependsOn, "depends-on", "d", "", "Task ID this depends on")

	// task list flags
	taskListCmd.Flags().BoolVar(&listReady, "ready", false, "Show only ready tasks")
	taskListCmd.Flags().BoolVar(&listInProgress, "in-progress", false, "Show only in-progress tasks")
	taskListCmd.Flags().BoolVar(&listDone, "done", false, "Show only completed tasks")
	taskListCmd.Flags().BoolVar(&listBlocked, "blocked", false, "Show only blocked tasks")

	// task clear flags
	taskClearCmd.Flags().BoolVar(&clearAll, "all", false, "Clear all tasks (with confirmation)")

	// Add subcommands
	taskCmd.AddCommand(taskAddCmd)
	taskCmd.AddCommand(taskListCmd)
	taskCmd.AddCommand(taskClearCmd)

	// Add to root
	rootCmd.AddCommand(taskCmd)
}
