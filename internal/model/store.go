package model

import (
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type LocalModel struct {
	Name              string
	Path              string
	Manifest          *ModelManifest
	Complete          bool
	TotalSizeBytes    int64
	DownloadedAt      time.Time
	DockerImagePulled bool
}

type LockFile struct {
	Model            string          `yaml:"model"`
	RegistryVersion  string          `yaml:"registry_version"`
	ManifestVersion  string          `yaml:"manifest_version"`
	PulledAt         time.Time       `yaml:"pulled_at"`
	DockerImage      string          `yaml:"docker_image"`
	DockerImageReady bool            `yaml:"docker_image_ready"`
	Files            []LockFileEntry `yaml:"files"`
	Complete         bool            `yaml:"complete"`
}

type LockFileEntry struct {
	Name         string    `yaml:"name"`
	Size         int64     `yaml:"size"`
	DownloadedAt time.Time `yaml:"downloaded_at"`
	Extracted    bool      `yaml:"extracted,omitempty"`
}

type Store struct {
	BaseDir string
}

func NewStore(baseDir string) *Store {
	return &Store{BaseDir: baseDir}
}

func (s *Store) ModelDir(name string) string {
	return filepath.Join(s.BaseDir, name)
}

func (s *Store) ModelExists(name string) bool {
	lockPath := filepath.Join(s.ModelDir(name), "voicaa.lock")
	_, err := os.Stat(lockPath)
	return err == nil
}

func (s *Store) IsComplete(name string) bool {
	lock, err := s.ReadLock(name)
	if err != nil {
		return false
	}
	return lock.Complete
}

func (s *Store) ReadLock(name string) (*LockFile, error) {
	lockPath := filepath.Join(s.ModelDir(name), "voicaa.lock")
	data, err := os.ReadFile(lockPath)
	if err != nil {
		return nil, err
	}
	var lock LockFile
	if err := yaml.Unmarshal(data, &lock); err != nil {
		return nil, err
	}
	return &lock, nil
}

func (s *Store) WriteLock(name string, lock *LockFile) error {
	dir := s.ModelDir(name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(lock)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "voicaa.lock"), data, 0644)
}

func (s *Store) EnsureDir(name string) error {
	return os.MkdirAll(s.ModelDir(name), 0755)
}

func (s *Store) ListLocal() ([]LocalModel, error) {
	entries, err := os.ReadDir(s.BaseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var models []LocalModel
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		lock, err := s.ReadLock(name)
		if err != nil {
			continue
		}
		var totalSize int64
		for _, f := range lock.Files {
			totalSize += f.Size
		}
		manifest, _ := FindModel(name)
		models = append(models, LocalModel{
			Name:              name,
			Path:              s.ModelDir(name),
			Manifest:          manifest,
			Complete:          lock.Complete,
			TotalSizeBytes:    totalSize,
			DownloadedAt:      lock.PulledAt,
			DockerImagePulled: lock.DockerImageReady,
		})
	}
	return models, nil
}

func (s *Store) VoicesDir(name string) string {
	return filepath.Join(s.ModelDir(name), "voices")
}

func (s *Store) ListVoiceFiles(name string) ([]string, error) {
	voicesDir := s.VoicesDir(name)
	entries, err := os.ReadDir(voicesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var voices []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".pt" {
			voices = append(voices, e.Name())
		}
	}
	return voices, nil
}
