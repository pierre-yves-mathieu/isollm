package airyra

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"time"
)

const (
	// DefaultPollInterval is the interval between server health checks
	DefaultPollInterval = 100 * time.Millisecond
	// DefaultStartTimeout is the default timeout waiting for server to start
	DefaultStartTimeout = 30 * time.Second
)

// StartServer starts the airyra server in the background.
// It shells out to `airyra server start` and runs it as a background process.
func StartServer(host string, port int) error {
	args := []string{"server", "start"}
	if host != "" || port != 0 {
		bindAddr := host
		if bindAddr == "" {
			bindAddr = "localhost"
		}
		if port != 0 {
			bindAddr = fmt.Sprintf("%s:%d", bindAddr, port)
		}
		args = append(args, "--bind", bindAddr)
	}

	cmd := exec.Command("airyra", args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start airyra server: %w", err)
	}

	// Detach from the process so it continues running after we exit
	go func() {
		_ = cmd.Wait()
	}()

	return nil
}

// WaitForServer polls until the airyra server is ready or context is cancelled.
func WaitForServer(ctx context.Context, host string, port int) error {
	addr := fmt.Sprintf("%s:%d", host, port)

	ticker := time.NewTicker(DefaultPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for airyra server at %s: %w", addr, ctx.Err())
		case <-ticker.C:
			if IsServerRunning(host, port) {
				return nil
			}
		}
	}
}

// IsServerRunning checks if the airyra server is responding.
// It does a simple TCP connection check to the server port.
func IsServerRunning(host string, port int) bool {
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// EnsureRunning starts the server if not already running and waits for it.
func EnsureRunning(ctx context.Context, host string, port int) error {
	// Check if already running
	if IsServerRunning(host, port) {
		return nil
	}

	// Start the server
	if err := StartServer(host, port); err != nil {
		return err
	}

	// Wait for it to be ready
	waitCtx, cancel := context.WithTimeout(ctx, DefaultStartTimeout)
	defer cancel()

	return WaitForServer(waitCtx, host, port)
}
