# voicaa

Ollama for voice. Run speech-to-speech AI models locally.

`voicaa` makes it easy to pull, serve, and manage real-time voice AI models on your own hardware. One command to download, one command to serve.

## Quick Start

```bash
# Install
go install github.com/Krunal96369/voicaa/cmd/voicaa@latest

# Or build from source
git clone https://github.com/Krunal96369/voicaa.git
cd voicaa && make install

# Pull and run a model (Moshi — no token needed)
voicaa run moshi

# Or PersonaPlex with persona control (needs HF token)
export HF_TOKEN=hf_your_token_here
voicaa run personaplex
```

## Requirements

- **GPU**: NVIDIA GPU with 12GB+ VRAM (8GB with `--cpu-offload`)
- **Docker**: With NVIDIA Container Toolkit
- **HuggingFace token**: Only needed for gated models (e.g., PersonaPlex)

## Commands

### Pull a model

Download model weights from HuggingFace and pull the Docker image.

```bash
voicaa pull moshi
voicaa pull personaplex
```

### Serve a model

Start the inference server. Connects via WebSocket.

```bash
# Moshi (basic full-duplex conversation)
voicaa serve moshi

# PersonaPlex (with persona + voice control)
voicaa serve personaplex --voice NATF2 --prompt "You are a friendly assistant."
```

Options:
- `--port 8998` — Host port (default: 8998)
- `--voice NATF2` — Voice embedding (see `voicaa voices`)
- `--prompt "..."` — System text prompt
- `--cpu-offload` — Use less VRAM by offloading to CPU
- `--detach` — Run in background

### Run (pull + serve)

```bash
voicaa run personaplex --voice NATM1 --prompt "You are a teacher."
```

### List models

```bash
voicaa models
```

### List voices

```bash
voicaa voices personaplex
```

### Show running instances

```bash
voicaa ps
```

### Stop a model

```bash
voicaa stop personaplex
```

## Available Models

| Model       | Size   | License           | Description                                          |
| ----------- | ------ | ----------------- | ---------------------------------------------------- |
| moshi       | ~15 GB | CC-BY-4.0         | Kyutai's full-duplex speech-to-speech model          |
| personaplex | ~14 GB | NVIDIA Open Model | Full-duplex voice model with persona + voice control |

## Configuration

Config lives at `~/.voicaa/config.yaml`:

```yaml
models_dir: ~/.voicaa/models
gpu_runtime: nvidia
default_port: 8998
log_level: info
```

Set your HuggingFace token via environment variable:

```bash
export HF_TOKEN=hf_your_token_here
```

Or in the config file:

```yaml
hf_token: hf_your_token_here
```

## How It Works

`voicaa` doesn't reimplement model inference. It manages Docker containers that run the actual model servers. When you run `voicaa serve personaplex`:

1. Model weights are mounted from `~/.voicaa/models/personaplex/` into the container
2. A GPU-enabled Docker container starts with the model server
3. The server listens on the specified port via WebSocket
4. You connect a client to `ws://localhost:8998/api/chat`

## License

MIT
