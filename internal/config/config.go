package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	DefaultConfigDir  = ".voicaa"
	DefaultConfigFile = "config.yaml"
)

type Config struct {
	ModelsDir   string `yaml:"models_dir"`
	HFToken     string `yaml:"hf_token,omitempty"`
	DockerHost  string `yaml:"docker_host,omitempty"`
	GPURuntime  string `yaml:"gpu_runtime"`
	DefaultPort int    `yaml:"default_port"`
	LogLevel    string `yaml:"log_level"`
	DaemonPort  int    `yaml:"daemon_port"`
	RegistryURL string `yaml:"registry_url,omitempty"`
	Offline     bool   `yaml:"offline,omitempty"`
	Cloud       CloudConfig `yaml:"cloud,omitempty"`
	MLX         MLXConfig   `yaml:"mlx,omitempty"`
}

// CloudConfig holds cloud GPU backend configuration.
type CloudConfig struct {
	Provider string    `yaml:"provider,omitempty"`
	APIKey   string    `yaml:"api_key,omitempty"`
	GPUType  string    `yaml:"gpu_type,omitempty"`
	DiskGB   int       `yaml:"disk_gb,omitempty"`
	RunPod   RunPodCfg `yaml:"runpod,omitempty"`
}

// RunPodCfg holds RunPod-specific settings.
type RunPodCfg struct {
	CloudType string `yaml:"cloud_type,omitempty"`
	Region    string `yaml:"region,omitempty"`
}

// MLXConfig holds MLX backend configuration.
type MLXConfig struct {
	UVPath string `yaml:"uv_path,omitempty"` // override path to uv binary
}

func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		ModelsDir:   filepath.Join(home, DefaultConfigDir, "models"),
		GPURuntime:  "nvidia",
		DefaultPort: 8998,
		LogLevel:    "info",
		DaemonPort:  8899,
		Cloud:       CloudConfig{DiskGB: 50},
	}
}

func ConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, DefaultConfigDir)
}

func ConfigPath() string {
	return filepath.Join(ConfigDir(), DefaultConfigFile)
}

func Load() (*Config, error) {
	cfg := DefaultConfig()
	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	if envToken := os.Getenv("HF_TOKEN"); envToken != "" {
		cfg.HFToken = envToken
	}
	if envKey := os.Getenv("VOICAA_CLOUD_API_KEY"); envKey != "" {
		cfg.Cloud.APIKey = envKey
	}
	return cfg, nil
}

func (c *Config) Save() error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	dir := filepath.Dir(ConfigPath())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(ConfigPath(), data, 0644)
}
