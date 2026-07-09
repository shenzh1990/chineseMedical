package ai

import (
	"sync"

	"chinese-medical/internal/config"
)

type SettingsStore struct {
	mu  sync.RWMutex
	cfg config.AIConfig
}

func NewSettingsStore(cfg config.AIConfig) *SettingsStore {
	return &SettingsStore{cfg: cfg}
}

func (s *SettingsStore) Config() config.AIConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

func (s *SettingsStore) Update(cfg config.AIConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = cfg
}

func (s *SettingsStore) ImageGenerator() ImageGenerator {
	return NewImageGenerator(s.Config())
}

func (s *SettingsStore) FoodResearcher() FoodResearcher {
	return NewFoodResearcher(s.Config())
}
