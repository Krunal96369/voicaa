package daemon

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	PidFileName    = "daemon.pid"
	DefaultAddr    = "localhost:8899"
	DefaultPort    = 8899
	healthEndpoint = "/api/v1/health"
)

// PidPath returns the path to the daemon PID file.
func PidPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".voicaa", PidFileName)
}

// IsRunning checks if the daemon is alive by hitting the health endpoint.
func IsRunning(addr string) bool {
	url := fmt.Sprintf("http://%s%s", addr, healthEndpoint)
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// Start launches the daemon as a background process.
// It runs `voicaa serve` with no model argument (daemon mode).
func Start(addr string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot find voicaa executable: %w", err)
	}

	cmd := exec.Command(exePath, "serve", "--daemon", "--daemon-addr", addr)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	// Detach from parent process
	cmd.SysProcAttr = nil // Platform-specific; basic detach

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Write PID file
	pidDir := filepath.Dir(PidPath())
	os.MkdirAll(pidDir, 0755)
	os.WriteFile(PidPath(), []byte(strconv.Itoa(cmd.Process.Pid)), 0644)

	// Don't wait for the child — let it run independently
	cmd.Process.Release()

	return nil
}

// EnsureRunning starts the daemon if not already running, then waits for it.
func EnsureRunning(addr string) error {
	if IsRunning(addr) {
		return nil
	}

	if err := Start(addr); err != nil {
		return err
	}

	// Wait up to 5 seconds for the daemon to be ready
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		if IsRunning(addr) {
			return nil
		}
	}

	return fmt.Errorf("daemon failed to start within 5 seconds")
}

// Stop sends a signal to the daemon process.
func Stop() error {
	data, err := os.ReadFile(PidPath())
	if err != nil {
		return fmt.Errorf("no daemon PID file found")
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return fmt.Errorf("invalid PID file")
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("process %d not found", pid)
	}

	if err := proc.Signal(os.Interrupt); err != nil {
		return fmt.Errorf("failed to stop daemon (PID %d): %w", pid, err)
	}

	os.Remove(PidPath())
	return nil
}

// Addr returns the daemon address from config or default.
func Addr(port int) string {
	if port == 0 {
		port = DefaultPort
	}
	return fmt.Sprintf("localhost:%d", port)
}
