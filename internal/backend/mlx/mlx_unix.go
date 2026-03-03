//go:build !windows

package mlx

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Krunal96369/voicaa/internal/backend"
	"github.com/Krunal96369/voicaa/internal/model"
)

// TrackedProcess represents a running MLX subprocess.
type TrackedProcess struct {
	ID        string
	Cmd       *exec.Cmd   // nil when reattached after daemon restart
	Process   *os.Process // always set when process is alive
	ModelName string
	Port      int
	Voice     string
	Prompt    string
	StartedAt time.Time
	LogFile   string
	Cancel    context.CancelFunc // nil when reattached
}

// MLXBackend implements backend.Backend using local subprocesses.
type MLXBackend struct {
	mu        sync.RWMutex
	processes map[string]*TrackedProcess
	state     *StateStore
	uvPath    string
}

// Config holds configuration for the MLX backend.
type Config struct {
	StatePath string
	UVPath    string // override path to uv binary; empty = use PATH
}

// New creates an MLXBackend. Returns an error if uv is not found.
func New(cfg Config) (*MLXBackend, error) {
	uvPath := cfg.UVPath
	if uvPath == "" {
		var err error
		uvPath, err = exec.LookPath("uv")
		if err != nil {
			return nil, fmt.Errorf(
				"uv is required for the MLX backend but was not found on PATH.\n" +
					"  Install: curl -LsSf https://astral.sh/uv/install.sh | sh\n" +
					"  Or set mlx.uv_path in ~/.voicaa/config.yaml")
		}
	}

	b := &MLXBackend{
		processes: make(map[string]*TrackedProcess),
		state:     NewStateStore(cfg.StatePath),
		uvPath:    uvPath,
	}

	b.reattachSurvivors()

	return b, nil
}

func (b *MLXBackend) Name() string { return "mlx" }

// RequiresLocalModel returns false — moshi_mlx handles its own HF cache via --hf-repo.
func (b *MLXBackend) RequiresLocalModel() bool { return false }

// BuildServerCmd resolves template placeholders for MLX models.
func (b *MLXBackend) BuildServerCmd(manifest *model.ModelManifest, port int, voice, prompt string, cpuOffload bool, device string) []string {
	hfRepo := manifest.MLX.HFRepo
	if hfRepo == "" {
		hfRepo = manifest.HuggingFace.Repo
	}

	var cmd []string
	for _, part := range manifest.Entrypoint.ServerCmd {
		resolved := part
		resolved = strings.ReplaceAll(resolved, "{{.Port}}", fmt.Sprintf("%d", port))
		resolved = strings.ReplaceAll(resolved, "{{.HFRepo}}", hfRepo)
		cmd = append(cmd, resolved)
	}
	// MLX does not use --device or --cpu-offload flags
	return cmd
}

func (b *MLXBackend) Start(ctx context.Context, req backend.ServeRequest) (backend.InstanceID, error) {
	id := fmt.Sprintf("mlx-%s-%d", req.ModelName, time.Now().UnixNano())

	// Create log directory and file
	logDir := filepath.Join(os.TempDir(), "voicaa-mlx-logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create log directory: %w", err)
	}
	logPath := filepath.Join(logDir, id+".log")
	logFile, err := os.Create(logPath)
	if err != nil {
		return "", fmt.Errorf("failed to create log file: %w", err)
	}

	// Use context.Background so the process outlives the HTTP request
	procCtx, cancel := context.WithCancel(context.Background())

	cmd := exec.CommandContext(procCtx, req.Cmd[0], req.Cmd[1:]...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = append(os.Environ(), envMapToSlice(req.Env)...)
	// Set process group so we can signal the whole group
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		cancel()
		logFile.Close()
		return "", fmt.Errorf("failed to start MLX process: %w", err)
	}

	tracked := &TrackedProcess{
		ID:        id,
		Cmd:       cmd,
		Process:   cmd.Process,
		ModelName: req.ModelName,
		Port:      req.HostPort,
		Voice:     req.Voice,
		Prompt:    req.Prompt,
		StartedAt: time.Now(),
		LogFile:   logPath,
		Cancel:    cancel,
	}

	b.mu.Lock()
	b.processes[id] = tracked
	b.mu.Unlock()

	b.state.Add(&PersistedInstance{
		ID:        id,
		PID:       cmd.Process.Pid,
		ModelName: req.ModelName,
		Port:      req.HostPort,
		Voice:     req.Voice,
		Prompt:    req.Prompt,
		LogFile:   logPath,
		StartedAt: time.Now(),
	})

	// Monitor process exit in background
	go func() {
		cmd.Wait()
		logFile.Close()
		b.mu.Lock()
		delete(b.processes, id)
		b.mu.Unlock()
		b.state.Remove(id)
	}()

	return backend.InstanceID(id), nil
}

func (b *MLXBackend) Stop(ctx context.Context, id backend.InstanceID, timeoutSec int) error {
	b.mu.RLock()
	tracked, ok := b.processes[string(id)]
	b.mu.RUnlock()
	if !ok {
		return fmt.Errorf("instance %s not found", id)
	}

	// Send SIGTERM
	if err := tracked.Process.Signal(syscall.SIGTERM); err != nil {
		// Process might already be dead
		b.cleanup(string(id))
		return nil
	}

	// Wait for graceful shutdown with timeout
	done := make(chan struct{})
	go func() {
		if tracked.Cmd != nil {
			tracked.Cmd.Wait()
		} else {
			// Reattached process: poll until dead
			b.waitProcessExit(tracked.Process, time.Duration(timeoutSec)*time.Second)
		}
		close(done)
	}()

	select {
	case <-done:
		// Graceful shutdown succeeded
	case <-time.After(time.Duration(timeoutSec) * time.Second):
		// Force kill
		tracked.Process.Kill()
		<-done
	}

	b.cleanup(string(id))
	return nil
}

func (b *MLXBackend) ForceStop(ctx context.Context, id backend.InstanceID) error {
	b.mu.RLock()
	tracked, ok := b.processes[string(id)]
	b.mu.RUnlock()
	if !ok {
		return fmt.Errorf("instance %s not found", id)
	}

	tracked.Process.Kill()
	if tracked.Cmd != nil {
		tracked.Cmd.Wait()
	}
	b.cleanup(string(id))
	return nil
}

func (b *MLXBackend) Remove(ctx context.Context, id backend.InstanceID) error {
	b.cleanup(string(id))
	return nil
}

func (b *MLXBackend) List(ctx context.Context) ([]backend.InstanceInfo, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var result []backend.InstanceInfo
	for _, tracked := range b.processes {
		// Verify process is still alive
		if err := tracked.Process.Signal(syscall.Signal(0)); err != nil {
			continue
		}
		result = append(result, b.toInstanceInfo(tracked))
	}
	return result, nil
}

func (b *MLXBackend) FindByModel(ctx context.Context, modelName string) (*backend.InstanceInfo, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, tracked := range b.processes {
		if tracked.ModelName == modelName {
			if err := tracked.Process.Signal(syscall.Signal(0)); err != nil {
				continue
			}
			info := b.toInstanceInfo(tracked)
			return &info, nil
		}
	}
	return nil, fmt.Errorf("no MLX instance for model %q", modelName)
}

func (b *MLXBackend) Logs(ctx context.Context, id backend.InstanceID, w io.Writer) error {
	b.mu.RLock()
	tracked, ok := b.processes[string(id)]
	b.mu.RUnlock()
	if !ok {
		return fmt.Errorf("instance %s not found", id)
	}

	f, err := os.Open(tracked.LogFile)
	if err != nil {
		return err
	}
	defer f.Close()

	// Copy existing content
	io.Copy(w, f)

	// Tail for new content until context is cancelled
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			io.Copy(w, f)
		}
	}
}

func (b *MLXBackend) WaitReady(ctx context.Context, id backend.InstanceID, timeoutSec, intervalSec int) error {
	b.mu.RLock()
	tracked, ok := b.processes[string(id)]
	b.mu.RUnlock()
	if !ok {
		return fmt.Errorf("instance %s not found", id)
	}
	return backend.WaitForTCP("localhost", tracked.Port, timeoutSec, intervalSec)
}

// Close saves state. Running processes are not killed.
func (b *MLXBackend) Close() error {
	return b.state.Save()
}

func (b *MLXBackend) cleanup(id string) {
	b.mu.Lock()
	tracked, ok := b.processes[id]
	delete(b.processes, id)
	b.mu.Unlock()

	b.state.Remove(id)

	if ok && tracked.Cancel != nil {
		tracked.Cancel()
	}
}

func (b *MLXBackend) toInstanceInfo(tracked *TrackedProcess) backend.InstanceInfo {
	return backend.InstanceInfo{
		ID:           backend.InstanceID(tracked.ID),
		Name:         fmt.Sprintf("voicaa-%s", tracked.ModelName),
		ModelName:    tracked.ModelName,
		Port:         tracked.Port,
		Voice:        tracked.Voice,
		Prompt:       tracked.Prompt,
		Status:       "running",
		StartedAt:    tracked.StartedAt,
		WebSocketURL: fmt.Sprintf("ws://localhost:%d/api/chat", tracked.Port),
	}
}

// reattachSurvivors reconnects to processes that survived a daemon restart.
func (b *MLXBackend) reattachSurvivors() {
	for _, persisted := range b.state.All() {
		proc, err := os.FindProcess(persisted.PID)
		if err != nil {
			b.state.Remove(persisted.ID)
			continue
		}
		// On Unix, FindProcess always succeeds. Check if process is alive.
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			b.state.Remove(persisted.ID)
			continue
		}

		b.processes[persisted.ID] = &TrackedProcess{
			ID:        persisted.ID,
			Process:   proc,
			ModelName: persisted.ModelName,
			Port:      persisted.Port,
			Voice:     persisted.Voice,
			Prompt:    persisted.Prompt,
			StartedAt: persisted.StartedAt,
			LogFile:   persisted.LogFile,
		}
	}
}

// waitProcessExit polls a process until it exits or the timeout elapses.
func (b *MLXBackend) waitProcessExit(proc *os.Process, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func envMapToSlice(env map[string]string) []string {
	var result []string
	for k, v := range env {
		result = append(result, k+"="+v)
	}
	return result
}
