package model

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Krunal96369/voicaa/registry"
	"gopkg.in/yaml.v3"
)

// Registry holds all loaded model manifests.
type Registry struct {
	Version string          `yaml:"registry_version"`
	Models  []ModelManifest `yaml:"models"`
}

// RegistryLoader loads and merges model definitions from multiple sources.
type RegistryLoader struct {
	localDir   string // ~/.voicaa/registry/
	remoteURL  string // remote index URL
	cacheDir   string // ~/.voicaa/cache/
	cacheTTL   time.Duration
	offline    bool
	httpClient *http.Client
}

// NewRegistryLoader creates a loader with the given configuration.
func NewRegistryLoader(localDir, remoteURL, cacheDir string, offline bool) *RegistryLoader {
	return &RegistryLoader{
		localDir:   localDir,
		remoteURL:  remoteURL,
		cacheDir:   cacheDir,
		cacheTTL:   1 * time.Hour,
		offline:    offline,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// LoadAll loads and merges all registry sources.
// Priority: embedded (lowest) < remote < local (highest).
func (rl *RegistryLoader) LoadAll() (*Registry, error) {
	embedded, err := rl.loadEmbedded()
	if err != nil {
		return nil, err
	}

	var remote []ModelManifest
	if !rl.offline && rl.remoteURL != "" {
		remote, _ = rl.loadRemote() // remote failure is non-fatal
	}

	var local []ModelManifest
	if rl.localDir != "" {
		local, _ = rl.loadLocal() // local failure is non-fatal
	}

	merged := merge(embedded, remote, local)

	return &Registry{
		Version: "1",
		Models:  merged,
	}, nil
}

// loadEmbedded reads from the compiled-in registry/models.yaml.
func (rl *RegistryLoader) loadEmbedded() ([]ModelManifest, error) {
	data, err := registry.FS.ReadFile("models.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded registry: %w", err)
	}
	var reg Registry
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("failed to parse embedded registry: %w", err)
	}
	return reg.Models, nil
}

// loadRemote fetches from the remote URL with caching.
func (rl *RegistryLoader) loadRemote() ([]ModelManifest, error) {
	// Check cache first
	cachePath := filepath.Join(rl.cacheDir, "remote-registry.yaml")
	cacheTimePath := filepath.Join(rl.cacheDir, "remote-registry.time")

	if data, err := os.ReadFile(cachePath); err == nil {
		if timeData, err := os.ReadFile(cacheTimePath); err == nil {
			if ts, err := time.Parse(time.RFC3339, strings.TrimSpace(string(timeData))); err == nil {
				if time.Since(ts) < rl.cacheTTL {
					var reg Registry
					if err := yaml.Unmarshal(data, &reg); err == nil {
						return reg.Models, nil
					}
				}
			}
		}
	}

	// Fetch from remote
	resp, err := rl.httpClient.Get(rl.remoteURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch remote registry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remote registry returned HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read remote registry: %w", err)
	}

	var reg Registry
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("failed to parse remote registry: %w", err)
	}

	// Write cache
	os.MkdirAll(rl.cacheDir, 0755)
	os.WriteFile(cachePath, data, 0644)
	os.WriteFile(cacheTimePath, []byte(time.Now().Format(time.RFC3339)), 0644)

	return reg.Models, nil
}

// loadLocal reads all .yaml/.yml files from the local registry directory.
// Each file contains a single ModelManifest (not wrapped in Registry).
func (rl *RegistryLoader) loadLocal() ([]ModelManifest, error) {
	entries, err := os.ReadDir(rl.localDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var models []ModelManifest
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(rl.localDir, e.Name()))
		if err != nil {
			continue
		}

		var m ModelManifest
		if err := yaml.Unmarshal(data, &m); err != nil {
			continue
		}
		if m.Name == "" {
			continue
		}

		models = append(models, m)
	}

	return models, nil
}

// merge combines models from multiple sources. Later sources override earlier
// sources when matched by name (case-insensitive).
func merge(sources ...[]ModelManifest) []ModelManifest {
	seen := make(map[string]int) // lowercase name -> index in result
	var result []ModelManifest

	for _, source := range sources {
		for _, m := range source {
			key := strings.ToLower(m.Name)
			if idx, exists := seen[key]; exists {
				result[idx] = m // override
			} else {
				seen[key] = len(result)
				result = append(result, m)
			}
		}
	}

	return result
}

// --- Backwards-compatible API ---
// These functions use a cached default loader for compatibility with
// existing callers. The service layer should use RegistryLoader directly.

var cachedRegistry *Registry

func LoadRegistry() (*Registry, error) {
	if cachedRegistry != nil {
		return cachedRegistry, nil
	}
	loader := NewRegistryLoader("", "", "", true) // embedded only
	reg, err := loader.LoadAll()
	if err != nil {
		return nil, err
	}
	cachedRegistry = reg
	return cachedRegistry, nil
}

func FindModel(name string) (*ModelManifest, error) {
	reg, err := LoadRegistry()
	if err != nil {
		return nil, err
	}
	lower := strings.ToLower(name)
	for i := range reg.Models {
		m := &reg.Models[i]
		if strings.ToLower(m.Name) == lower {
			return m, nil
		}
		for _, alias := range m.Aliases {
			if strings.ToLower(alias) == lower {
				return m, nil
			}
		}
	}
	return nil, fmt.Errorf("model %q not found in registry", name)
}

func ListRegistryModels() ([]ModelManifest, error) {
	reg, err := LoadRegistry()
	if err != nil {
		return nil, err
	}
	return reg.Models, nil
}

// FindModelInRegistry searches a specific registry for a model by name or alias.
func FindModelInRegistry(reg *Registry, name string) (*ModelManifest, error) {
	lower := strings.ToLower(name)
	for i := range reg.Models {
		m := &reg.Models[i]
		if strings.ToLower(m.Name) == lower {
			return m, nil
		}
		for _, alias := range m.Aliases {
			if strings.ToLower(alias) == lower {
				return m, nil
			}
		}
	}
	return nil, fmt.Errorf("model %q not found in registry", name)
}
