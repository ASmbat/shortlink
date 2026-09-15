package store

import (
	"context"
	"sync"
	"time"

	"github.com/ASmbat/shortlink/internal/models"
)

// MemoryStore is a thread-safe, in-memory Store implementation. It's the
// default backend for local development and tests; swap in a DynamoDB (or
// other) implementation of Store for production without touching callers.
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
	if !link.ExpiresAt.IsZero() && time.Now().After(link.ExpiresAt) {
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
