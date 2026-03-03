# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`voicaa` is a Go CLI tool and daemon ("Ollama for voice") that manages the lifecycle of real-time speech-to-speech AI models. It downloads model weights from HuggingFace, pulls pre-built Docker images with GPU-accelerated inference servers, orchestrates them as local Docker containers with NVIDIA GPU passthrough, and provides a web UI for voice interaction. Users interact with running models over WebSocket.

Module path: `github.com/Krunal96369/voicaa`

## Build & Development Commands

```bash
make build       # Build binary with version tag → ./voicaa
make install     # Install to $GOPATH/bin
make clean       # Remove compiled binary
make fmt         # go fmt ./...
make vet         # go vet ./...
make test        # go test ./...
```

Version is injected at build time via `-ldflags` setting `github.com/Krunal96369/voicaa/internal/cli.Version`.

## Architecture

### Layered Structure

```
cmd/voicaa/main.go             → Entrypoint, calls cli.Execute()

internal/cli/                  → Cobra commands (presentation layer)
  serve.go                     → Daemon mode (no args) OR model serve (with model arg)
  client.go                    → (future) HTTP client for daemon communication

internal/api/                  → HTTP API layer (daemon endpoints)
  server.go                    → HTTP server setup, graceful shutdown
  routes.go                    → Route registration using Go 1.22+ ServeMux
  handlers_model.go            → Pull (SSE streaming), list models, voices
  handlers_instance.go         → Serve, run, stop, ps, health, version
  handlers_ws.go               → WebSocket proxy (browser ↔ model container)
  handlers_ui.go               → Serves embedded web UI
  types.go                     → JSON request/response structs

internal/service/              → Business logic layer
  model_service.go             → Pull, list models, find model, voices
  instance_service.go          → Serve, run, stop, ps, logs

internal/backend/              → Hardware abstraction
  backend.go                   → Backend interface + shared types (ServeRequest, InstanceInfo)
  docker/docker.go             → DockerBackend implementation
  docker/health.go             → TCP health check

internal/config/config.go      → ~/.voicaa/config.yaml loading with defaults
internal/daemon/daemon.go      → Daemon lifecycle (PID file, auto-start, health check)
internal/docker/               → Raw Docker SDK wrapper (client, image operations)
internal/huggingface/          → HuggingFace HTTP downloader with resume
internal/model/                → Model registry, manifests, local store
  manifest.go                  → Type definitions for model manifests
  registry.go                  → Multi-source registry loader (embedded + remote + local)
  store.go                     → Local model storage with voicaa.lock files

registry/                      → Embedded model definitions (go:embed)
web/                           → Embedded web UI (go:embed, single HTML file)
```

### Key Architectural Patterns

**Daemon Architecture**: `voicaa serve` (no args) starts the HTTP daemon on `:8899`. `voicaa serve <model>` starts a specific model container. All management commands can work either directly or through the daemon API.

**Backend Interface** (`internal/backend/backend.go`): All inference backends implement `Backend` with methods: `Start`, `Stop`, `ForceStop`, `Remove`, `List`, `FindByModel`, `Logs`, `WaitReady`. Currently only `DockerBackend` exists. Future backends (subprocess, ONNX, CoreML) implement the same interface.

**Service Layer**: `ModelService` handles registry/download operations. `InstanceService` handles instance lifecycle via the `Backend` interface. CLI handlers and API handlers both delegate to these services.

**Multi-Source Registry** (`internal/model/registry.go`): `RegistryLoader` merges models from three sources (later wins by model name):
1. Embedded `registry/models.yaml` (compiled into binary)
2. Remote YAML index (fetched from `registry_url` config, cached 1hr at `~/.voicaa/cache/`)
3. Local YAML files in `~/.voicaa/registry/` (one manifest per file)

**WebSocket Proxy** (`internal/api/handlers_ws.go`): The daemon proxies WebSocket connections between the browser and model containers: `Browser ↔ Daemon(:8899/api/v1/ws/{model}) ↔ Container(:8998/api/chat)`. Bidirectional binary audio pump using gorilla/websocket.

**Docker Integration** (`internal/docker/`): Containers are created with NVIDIA GPU passthrough via `DeviceRequests`, model directories bind-mounted read-only at `/models`, and custom labels (`io.voicaa.*`) for container discovery.

**Server Command Templating** (`internal/backend/docker/docker.go`): `BuildServerCmd()` resolves placeholders in manifest server commands (`{{.Port}}`, `{{.MoshiWeight}}`, etc.) via string replacement.

### API Routes (Daemon)

The daemon listens on `localhost:8899` (configurable via `daemon_port`):

- `GET /` — Web UI
- `GET /api/v1/health` — Health check
- `POST /api/v1/pull` — Pull model (SSE progress stream)
- `POST /api/v1/serve` — Start a model
- `POST /api/v1/run` — Pull + serve
- `POST /api/v1/stop` — Stop a model
- `GET /api/v1/models` — List local models
- `GET /api/v1/models/registry` — List all registry models
- `GET /api/v1/models/{name}/voices` — List voices
- `GET /api/v1/ps` — Running instances
- `GET /api/v1/ws/{model}` — WebSocket proxy

### Configuration

Config at `~/.voicaa/config.yaml` with fields: `models_dir`, `hf_token` (overridden by `HF_TOKEN` env var), `docker_host`, `gpu_runtime` (default: `nvidia`), `default_port` (default: `8998`), `log_level` (default: `info`), `daemon_port` (default: `8899`), `registry_url`, `offline`.
