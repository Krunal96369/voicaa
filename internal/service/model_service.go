package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Krunal96369/voicaa/internal/config"
	"github.com/Krunal96369/voicaa/internal/docker"
	"github.com/Krunal96369/voicaa/internal/huggingface"
	"github.com/Krunal96369/voicaa/internal/model"
)

// PullProgress reports download progress for a single file.
type PullProgress struct {
	File       string `json:"file"`
	Downloaded int64  `json:"downloaded"`
	Total      int64  `json:"total"`
	Done       bool   `json:"done"`
}

// ModelService handles model registry, downloads, and metadata operations.
type ModelService struct {
	Config   *config.Config
	Store    *model.Store
	registry *model.Registry
	loader   *model.RegistryLoader
}

// NewModelService creates a ModelService from the given config.
func NewModelService(cfg *config.Config) *ModelService {
	home, _ := os.UserHomeDir()
	localDir := filepath.Join(home, ".voicaa", "registry")
	cacheDir := filepath.Join(home, ".voicaa", "cache")

	loader := model.NewRegistryLoader(localDir, cfg.RegistryURL, cacheDir, cfg.Offline)

	return &ModelService{
		Config: cfg,
		Store:  model.NewStore(cfg.ModelsDir),
		loader: loader,
	}
}

// loadRegistry loads (and caches) the merged registry.
func (s *ModelService) loadRegistry() (*model.Registry, error) {
	if s.registry != nil {
		return s.registry, nil
	}
	reg, err := s.loader.LoadAll()
	if err != nil {
		return nil, err
	}
	s.registry = reg
	return s.registry, nil
}

// FindModel looks up a model by name or alias in the full merged registry.
func (s *ModelService) FindModel(name string) (*model.ModelManifest, error) {
	reg, err := s.loadRegistry()
	if err != nil {
		return nil, err
	}
	return model.FindModelInRegistry(reg, name)
}

// ListLocal returns locally downloaded models.
func (s *ModelService) ListLocal() ([]model.LocalModel, error) {
	return s.Store.ListLocal()
}

// ListRegistry returns all models from the merged registry.
func (s *ModelService) ListRegistry() ([]model.ModelManifest, error) {
	reg, err := s.loadRegistry()
	if err != nil {
		return nil, err
	}
	return reg.Models, nil
}

// Voices returns voice info for a model.
func (s *ModelService) Voices(modelName string) (*model.ModelManifest, error) {
	return s.FindModel(modelName)
}

// Pull downloads model weights and optionally the Docker image.
// progressFn is called with progress updates for each file.
func (s *ModelService) Pull(ctx context.Context, modelName, token string, skipDocker, force bool, progressFn func(PullProgress)) error {
	manifest, err := s.FindModel(modelName)
	if err != nil {
		return err
	}

	// Resolve HuggingFace token
	hfToken := token
	if hfToken == "" {
		hfToken = s.Config.HFToken
	}
	if hfToken == "" {
		hfToken = os.Getenv("HF_TOKEN")
	}
	if hfToken == "" && manifest.HuggingFace.Gated {
		return fmt.Errorf(
			"model %q is gated and requires a HuggingFace token\n\n"+
				"  1. Accept the license at https://huggingface.co/%s\n"+
				"  2. Set your token: export HF_TOKEN=hf_xxx\n"+
				"     Or use: voicaa pull %s --token hf_xxx",
			manifest.Name, manifest.HuggingFace.Repo, manifest.Name,
		)
	}

	if !force && s.Store.IsComplete(manifest.Name) {
		if progressFn != nil {
			progressFn(PullProgress{File: manifest.Name, Done: true})
		}
		if !skipDocker && manifest.Backend != "mlx" {
			return s.pullDockerImage(ctx, manifest)
		}
		return nil
	}

	if err := s.Store.EnsureDir(manifest.Name); err != nil {
		return fmt.Errorf("failed to create model directory: %w", err)
	}

	dl := huggingface.NewDownloader(hfToken)
	modelDir := s.Store.ModelDir(manifest.Name)

	lock := &model.LockFile{
		Model:           manifest.Name,
		RegistryVersion: "1",
		ManifestVersion: manifest.Version,
		DockerImage:     manifest.Docker.Image,
	}

	for _, file := range manifest.HuggingFace.Files {
		destPath := filepath.Join(modelDir, file.Name)

		if !force {
			if info, err := os.Stat(destPath); err == nil && info.Size() > 0 {
				lock.Files = append(lock.Files, model.LockFileEntry{
					Name:         file.Name,
					Size:         info.Size(),
					DownloadedAt: info.ModTime(),
					Extracted:    file.PostExtract != "",
				})
				if progressFn != nil {
					progressFn(PullProgress{File: file.Name, Downloaded: info.Size(), Total: info.Size(), Done: true})
				}
				continue
			}
		} else {
			os.Remove(destPath)
			os.Remove(destPath + ".partial")
		}

		err := dl.DownloadFile(
			manifest.HuggingFace.Repo,
			file.Name,
			modelDir,
			func(downloaded, total int64) {
				if progressFn != nil {
					progressFn(PullProgress{File: file.Name, Downloaded: downloaded, Total: total})
				}
			},
		)
		if err != nil {
			return fmt.Errorf("failed to download %s: %w", file.Name, err)
		}

		info, _ := os.Stat(destPath)
		var fileSize int64
		if info != nil {
			fileSize = info.Size()
		}

		lock.Files = append(lock.Files, model.LockFileEntry{
			Name:         file.Name,
			Size:         fileSize,
			DownloadedAt: time.Now(),
		})

		if file.PostExtract != "" {
			if err := huggingface.ExtractTarGz(destPath, modelDir); err != nil {
				return fmt.Errorf("failed to extract %s: %w", file.Name, err)
			}
			lock.Files[len(lock.Files)-1].Extracted = true
		}

		if progressFn != nil {
			progressFn(PullProgress{File: file.Name, Downloaded: fileSize, Total: fileSize, Done: true})
		}
	}

	lock.Complete = true
	lock.PulledAt = time.Now()

	if err := s.Store.WriteLock(manifest.Name, lock); err != nil {
		return fmt.Errorf("failed to write lock file: %w", err)
	}

	if !skipDocker && manifest.Backend != "mlx" {
		return s.pullDockerImage(ctx, manifest)
	}

	return nil
}

// AlreadyDownloaded checks if a model is fully downloaded.
func (s *ModelService) AlreadyDownloaded(modelName string) bool {
	manifest, err := s.FindModel(modelName)
	if err != nil {
		return false
	}
	return s.Store.IsComplete(manifest.Name)
}

func (s *ModelService) pullDockerImage(ctx context.Context, manifest *model.ModelManifest) error {
	dc, err := docker.NewClient()
	if err != nil {
		return nil // Docker not available is a soft failure
	}
	defer dc.Close()

	exists, err := dc.ImageExists(ctx, manifest.Docker.Image)
	if err == nil && exists {
		return nil
	}

	if err := dc.PullImage(ctx, manifest.Docker.Image, os.Stdout); err != nil {
		return nil // Pull failure is a soft failure
	}

	return nil
}
