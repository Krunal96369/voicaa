package cloud

import (
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// TrackedInstance is the persisted metadata for a cloud instance.
type TrackedInstance struct {
	ProviderID   string    `yaml:"provider_id"`
	ProviderName string    `yaml:"provider_name"`
	ModelName    string    `yaml:"model_name"`
	DisplayName  string    `yaml:"display_name"`
	Voice        string    `yaml:"voice,omitempty"`
	Prompt       string    `yaml:"prompt,omitempty"`
	Host         string    `yaml:"host,omitempty"`
	Port         int       `yaml:"port,omitempty"`
	ProxyURL     string    `yaml:"proxy_url,omitempty"`
	CreatedAt    time.Time `yaml:"created_at"`
}

// StateStore manages the in-memory and on-disk cloud instance state.
type StateStore struct {
	mu        sync.RWMutex
	instances map[string]*TrackedInstance // key: provider instance ID
	path      string
}

// NewStateStore creates a StateStore and loads existing state from disk.
func NewStateStore(path string) *StateStore {
	ss := &StateStore{
		instances: make(map[string]*TrackedInstance),
		path:      path,
	}
	_ = ss.load()
	return ss
}

// Add saves a tracked instance.
func (s *StateStore) Add(inst *TrackedInstance) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.instances[inst.ProviderID] = inst
	_ = s.saveLocked()
}

// Remove deletes a tracked instance.
func (s *StateStore) Remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.instances, id)
	_ = s.saveLocked()
}

// Get returns a tracked instance by provider ID.
func (s *StateStore) Get(id string) *TrackedInstance {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.instances[id]
}

// FindByModel returns the first tracked instance for a model name.
func (s *StateStore) FindByModel(modelName string) *TrackedInstance {
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
func (s *StateStore) All() []*TrackedInstance {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*TrackedInstance, 0, len(s.instances))
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
