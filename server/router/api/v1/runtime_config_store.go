package v1

import (
	"encoding/json"
	stderrors "errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/pkg/errors"
)

type RuntimeConfig struct {
	MemoryLimitBytes int64 `json:"memory_limit_bytes"`
	UpdatedTs        int64 `json:"updated_ts"`
}

type runtimeConfigStore struct {
	path string

	mu     sync.Mutex
	loaded bool
	config RuntimeConfig
}

func newRuntimeConfigStore(path string) *runtimeConfigStore {
	return &runtimeConfigStore{
		path: path,
		config: RuntimeConfig{
			MemoryLimitBytes: -1,
			UpdatedTs:        0,
		},
	}
}

func (s *runtimeConfigStore) Get() (*RuntimeConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoadedLocked(); err != nil {
		return nil, err
	}
	cfg := s.config
	return &cfg, nil
}

func (s *runtimeConfigStore) Upsert(memoryLimitBytes int64) (*RuntimeConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoadedLocked(); err != nil {
		return nil, err
	}

	s.config.MemoryLimitBytes = memoryLimitBytes
	s.config.UpdatedTs = time.Now().Unix()
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	cfg := s.config
	return &cfg, nil
}

func (s *runtimeConfigStore) ensureLoadedLocked() error {
	if s.loaded {
		return nil
	}
	s.loaded = true
	content, err := os.ReadFile(s.path)
	if err != nil {
		if stderrors.Is(err, os.ErrNotExist) {
			return nil
		}
		return errors.Wrap(err, "failed to read runtime config")
	}
	if len(content) == 0 {
		return nil
	}
	var cfg RuntimeConfig
	if err := json.Unmarshal(content, &cfg); err != nil {
		return errors.Wrap(err, "failed to parse runtime config")
	}
	if cfg.MemoryLimitBytes == 0 {
		cfg.MemoryLimitBytes = -1
	}
	s.config = cfg
	return nil
}

func (s *runtimeConfigStore) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return errors.Wrap(err, "failed to create runtime config directory")
	}
	payload, err := json.MarshalIndent(s.config, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal runtime config")
	}
	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, payload, 0o600); err != nil {
		return errors.Wrap(err, "failed to write runtime config temp file")
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		return errors.Wrap(err, "failed to replace runtime config")
	}
	return nil
}
