package store

import (
	"context"
	"sync"
	"time"

	"github.com/ASmbat/shortlink/internal/models"
)

// MemoryStore is a thread-safe, in-memory Store for local development and
// tests. It returns copies, so callers can't change stored links by accident.
type MemoryStore struct {
	mu    sync.RWMutex
	links map[string]*models.Link
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{links: make(map[string]*models.Link)}
}

func (s *MemoryStore) Save(_ context.Context, link *models.Link) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.links[link.Code]; exists {
		return ErrCodeTaken
	}
	cp := *link
	s.links[link.Code] = &cp
	return nil
}

func (s *MemoryStore) Get(_ context.Context, code string) (*models.Link, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	link, ok := s.links[code]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *link
	return &cp, nil
}

func (s *MemoryStore) IncrementHits(_ context.Context, code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	link, ok := s.links[code]
	if !ok {
		return ErrNotFound
	}
	link.Hits++
	return nil
}

func (s *MemoryStore) DeleteExpired(_ context.Context, now time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	removed := 0
	for code, link := range s.links {
		if link.Expired(now) {
			delete(s.links, code)
			removed++
		}
	}
	return removed, nil
}

// Len returns the number of stored links (including expired ones not yet
// cleaned up).
func (s *MemoryStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.links)
}
