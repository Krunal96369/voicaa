package mlx

import (
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// PersistedInstance is the on-disk metadata for an MLX subprocess.
type PersistedInstance struct {
	ID        string    `yaml:"id"`
	PID       int       `yaml:"pid"`
	ModelName string    `yaml:"model_name"`
	Port      int       `yaml:"port"`
	Voice     string    `yaml:"voice,omitempty"`
	Prompt    string    `yaml:"prompt,omitempty"`
	LogFile   string    `yaml:"log_file"`
	StartedAt time.Time `yaml:"started_at"`
}

// StateStore manages in-memory and on-disk MLX instance state.
type StateStore struct {
	mu        sync.RWMutex
	instances map[string]*PersistedInstance
	path      string
}

// NewStateStore creates a StateStore and loads existing state from disk.
func NewStateStore(path string) *StateStore {
	ss := &StateStore{
		instances: make(map[string]*PersistedInstance),
		path:      path,
	}
	_ = ss.load()
	return ss
}

// Add saves a tracked instance.
func (s *StateStore) Add(inst *PersistedInstance) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.instances[inst.ID] = inst
	_ = s.saveLocked()
}

// Remove deletes a tracked instance.
func (s *StateStore) Remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.instances, id)
	_ = s.saveLocked()
}

// Get returns a tracked instance by ID.
func (s *StateStore) Get(id string) *PersistedInstance {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.instances[id]
}

// FindByModel returns the first tracked instance for a model name.
func (s *StateStore) FindByModel(modelName string) *PersistedInstance {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, inst := range s.instances {
		if inst.ModelName == modelName {
			return inst
		}
	}
	return nil
}

// All returns a copy of all tracked instances.
func (s *StateStore) All() []*PersistedInstance {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*PersistedInstance, 0, len(s.instances))
	for _, inst := range s.instances {
		out = append(out, inst)
	}
	return out
}

// Save persists state to disk.
func (s *StateStore) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.saveLocked()
}

func (s *StateStore) saveLocked() error {
	data, err := yaml.Marshal(s.instances)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}

func (s *StateStore) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, &s.instances)
}
