package model

type ModelManifest struct {
	Name         string               `yaml:"name"`
	Aliases      []string             `yaml:"aliases,omitempty"`
	Version      string               `yaml:"version"`
	Description  string               `yaml:"description"`
	License      string               `yaml:"license"`
	Backend      string               `yaml:"backend,omitempty"` // "docker", "mlx"; empty defaults to "docker"
	HuggingFace  HFSource             `yaml:"huggingface"`
	Docker       DockerSpec           `yaml:"docker"`
	MLX          MLXSpec              `yaml:"mlx,omitempty"`
	Entrypoint   EntrypointSpec       `yaml:"entrypoint"`
	Voices       VoicesSpec           `yaml:"voices"`
	Requirements HardwareRequirements `yaml:"requirements"`
}

type HFSource struct {
	Repo  string   `yaml:"repo"`
	Files []HFFile `yaml:"files"`
	Gated bool     `yaml:"gated"`
}

type HFFile struct {
	Name        string `yaml:"name"`
	Size        int64  `yaml:"size"`
	Required    bool   `yaml:"required"`
	Description string `yaml:"description,omitempty"`
	PostExtract string `yaml:"post_extract,omitempty"`
}

type DockerSpec struct {
	Image        string `yaml:"image"`
	BuildContext string `yaml:"build_context,omitempty"`
	BaseImage    string `yaml:"base_image,omitempty"`
	Platform     string `yaml:"platform,omitempty"`
}

type EntrypointSpec struct {
	ServerCmd   []string          `yaml:"server_cmd"`
	OfflineCmd  []string          `yaml:"offline_cmd,omitempty"`
	HealthCheck HealthCheckSpec   `yaml:"health_check"`
	Env         map[string]string `yaml:"env,omitempty"`
	WorkDir     string            `yaml:"workdir,omitempty"`
	DefaultPort int               `yaml:"default_port"`
}

type HealthCheckSpec struct {
	Type        string `yaml:"type"`
	Path        string `yaml:"path,omitempty"`
	TimeoutSec  int    `yaml:"timeout_sec"`
	IntervalSec int    `yaml:"interval_sec"`
}

type VoicesSpec struct {
	ArchiveFile  string       `yaml:"archive_file"`
	ExtractDir   string       `yaml:"extract_dir"`
	DefaultVoice string       `yaml:"default_voice"`
	Voices       []VoiceEntry `yaml:"voices"`
}

type VoiceEntry struct {
	Name        string `yaml:"name"`
	File        string `yaml:"file"`
	Gender      string `yaml:"gender"`
	Category    string `yaml:"category"`
	Description string `yaml:"description,omitempty"`
}

type HardwareRequirements struct {
	GPURequired          bool `yaml:"gpu_required"`
	MinVRAMGB            int  `yaml:"min_vram_gb"`
	MinVRAMWithOffloadGB int  `yaml:"min_vram_with_offload_gb"`
	MinRAMGB             int  `yaml:"min_ram_gb"`
	DiskSpaceGB          int  `yaml:"disk_space_gb"`
	AppleSilicon         bool `yaml:"apple_silicon,omitempty"`
}

// MLXSpec holds metadata for MLX subprocess-based models.
type MLXSpec struct {
	Package      string `yaml:"package"`      // PyPI package name, e.g. "moshi-mlx"
	PackageVer   string `yaml:"package_ver"`  // version constraint, e.g. ">=0.3.0"
	HFRepo       string `yaml:"hf_repo"`      // HuggingFace repo for --hf-repo flag
	Quantization string `yaml:"quantization"` // "q4", "q8", "bf16"
}
