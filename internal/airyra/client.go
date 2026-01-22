package airyra

import (
	"context"
	"fmt"
	"os"
	"time"

	sdk "airyra/pkg/airyra"

	"isollm/internal/config"
)

// Client wraps the airyra SDK client with isollm-specific configuration
type Client struct {
	sdk     *sdk.Client
	project string
	agentID string
}

// NewClient creates an airyra client from isollm config
func NewClient(cfg *config.Config, agentID string) (*Client, error) {
	sdkClient, err := sdk.NewClient(
		sdk.WithHost(cfg.Airyra.Host),
		sdk.WithPort(cfg.Airyra.Port),
		sdk.WithProject(cfg.Airyra.Project),
		sdk.WithAgentID(agentID),
		sdk.WithTimeout(30*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create airyra client: %w", err)
	}

	return &Client{
		sdk:     sdkClient,
		project: cfg.Airyra.Project,
		agentID: agentID,
	}, nil
}

// NewClientFromConfig creates a client using default agent ID (user@hostname:cwd)
func NewClientFromConfig(cfg *config.Config) (*Client, error) {
	agentID := defaultAgentID()
	return NewClient(cfg, agentID)
}

// Health checks if the airyra server is running
func (c *Client) Health(ctx context.Context) error {
	return c.sdk.Health(ctx)
}

// IsServerRunning returns true if the server is healthy
func (c *Client) IsServerRunning(ctx context.Context) bool {
	return c.sdk.Health(ctx) == nil
}

// --- Task Operations ---

// AddTask creates a new task
func (c *Client) AddTask(ctx context.Context, title string, opts ...sdk.CreateTaskOption) (*Task, error) {
	return c.sdk.CreateTask(ctx, title, opts...)
}

// GetTask retrieves a task by ID
func (c *Client) GetTask(ctx context.Context, id string) (*Task, error) {
	return c.sdk.GetTask(ctx, id)
}

// ListTasks lists tasks with optional filtering
func (c *Client) ListTasks(ctx context.Context, opts ...sdk.ListTasksOption) (*TaskList, error) {
	return c.sdk.ListTasks(ctx, opts...)
}

// ListReadyTasks lists tasks ready to be claimed
func (c *Client) ListReadyTasks(ctx context.Context) (*TaskList, error) {
	return c.sdk.ListReadyTasks(ctx)
}

// DeleteTask deletes a task
func (c *Client) DeleteTask(ctx context.Context, id string) error {
	return c.sdk.DeleteTask(ctx, id)
}

// ClearDoneTasks deletes all completed tasks
func (c *Client) ClearDoneTasks(ctx context.Context) (int, error) {
	count := 0

	for {
		list, err := c.sdk.ListTasks(ctx, sdk.WithStatus(StatusDone), sdk.WithPerPage(100))
		if err != nil {
			return count, err
		}

		if len(list.Tasks) == 0 {
			break
		}

		for _, task := range list.Tasks {
			if err := c.sdk.DeleteTask(ctx, task.ID); err != nil {
				return count, fmt.Errorf("failed to delete task %s: %w", task.ID, err)
			}
			count++
		}
	}

	return count, nil
}

// ClearAllTasks deletes all tasks (for --all flag)
func (c *Client) ClearAllTasks(ctx context.Context) (int, error) {
	count := 0

	for {
		list, err := c.sdk.ListTasks(ctx, sdk.WithPerPage(100))
		if err != nil {
			return count, err
		}

		if len(list.Tasks) == 0 {
			break
		}

		for _, task := range list.Tasks {
			if err := c.sdk.DeleteTask(ctx, task.ID); err != nil {
				return count, fmt.Errorf("failed to delete task %s: %w", task.ID, err)
			}
			count++
		}
	}

	return count, nil
}

// --- Task Lifecycle ---

// ClaimTask claims a task for this agent
func (c *Client) ClaimTask(ctx context.Context, id string) (*Task, error) {
	return c.sdk.ClaimTask(ctx, id)
}

// CompleteTask marks a task as done
func (c *Client) CompleteTask(ctx context.Context, id string) (*Task, error) {
	return c.sdk.CompleteTask(ctx, id)
}

// ReleaseTask releases a claimed task
func (c *Client) ReleaseTask(ctx context.Context, id string, force bool) (*Task, error) {
	return c.sdk.ReleaseTask(ctx, id, force)
}

// BlockTask marks a task as blocked
func (c *Client) BlockTask(ctx context.Context, id string) (*Task, error) {
	return c.sdk.BlockTask(ctx, id)
}

// UnblockTask unblocks a blocked task
func (c *Client) UnblockTask(ctx context.Context, id string) (*Task, error) {
	return c.sdk.UnblockTask(ctx, id)
}

// --- Dependencies ---

// AddDependency adds a dependency (child depends on parent)
func (c *Client) AddDependency(ctx context.Context, childID, parentID string) error {
	return c.sdk.AddDependency(ctx, childID, parentID)
}

// RemoveDependency removes a dependency
func (c *Client) RemoveDependency(ctx context.Context, childID, parentID string) error {
	return c.sdk.RemoveDependency(ctx, childID, parentID)
}

// ListDependencies lists dependencies for a task
func (c *Client) ListDependencies(ctx context.Context, taskID string) ([]Dependency, error) {
	return c.sdk.ListDependencies(ctx, taskID)
}

// --- Helpers ---

func defaultAgentID() string {
	user := os.Getenv("USER")
	if user == "" {
		user = "unknown"
	}

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "localhost"
	}

	cwd, _ := os.Getwd()

	return fmt.Sprintf("%s@%s:%s", user, hostname, cwd)
}
