package engine

import (
	"sync"

	"kerobot/internal/parser"
)

type StateManager struct {
	mu       sync.RWMutex
	snapshot parser.Snapshot
}

func NewStateManager() *StateManager {
	return &StateManager{snapshot: parser.Snapshot{State: parser.StateUnknown}}
}

func (s *StateManager) Update(snapshot parser.Snapshot) {
	s.mu.Lock()
	s.snapshot = snapshot
	s.mu.Unlock()
}

func (s *StateManager) Snapshot() parser.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshot
}
