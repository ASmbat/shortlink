package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ASmbat/shortlink/internal/models"
)

func TestMemoryStore_SaveGet(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	if err := s.Save(ctx, &models.Link{Code: "abc", LongURL: "https://example.com"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(ctx, &models.Link{Code: "abc"}); !errors.Is(err, ErrCodeTaken) {
		t.Fatalf("expected ErrCodeTaken for duplicate code, got %v", err)
	}

	got, err := s.Get(ctx, "abc")
	if err != nil || got.LongURL != "https://example.com" {
		t.Fatalf("got %+v, %v", got, err)
	}
	if _, err := s.Get(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMemoryStore_ReturnsCopies(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()
	link := &models.Link{Code: "abc", LongURL: "https://example.com"}
	_ = s.Save(ctx, link)

	link.LongURL = "https://changed.example" // caller changes its own value
	got, _ := s.Get(ctx, "abc")
	got.Hits = 99 // caller changes the returned value

	again, _ := s.Get(ctx, "abc")
	if again.LongURL != "https://example.com" || again.Hits != 0 {
		t.Fatalf("store was changed through a caller's pointer: %+v", again)
	}
}

func TestMemoryStore_IncrementHits(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()
	_ = s.Save(ctx, &models.Link{Code: "abc"})

	for i := 0; i < 3; i++ {
		if err := s.IncrementHits(ctx, "abc"); err != nil {
			t.Fatal(err)
		}
	}
	if got, _ := s.Get(ctx, "abc"); got.Hits != 3 {
		t.Fatalf("expected 3 hits, got %d", got.Hits)
	}
	if err := s.IncrementHits(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMemoryStore_DeleteExpired(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()
	now := time.Unix(1000, 0)
	_ = s.Save(ctx, &models.Link{Code: "old", ExpiresAt: now.Add(-time.Second)})
	_ = s.Save(ctx, &models.Link{Code: "edge", ExpiresAt: now}) // expires exactly now
	_ = s.Save(ctx, &models.Link{Code: "new", ExpiresAt: now.Add(time.Hour)})
	_ = s.Save(ctx, &models.Link{Code: "forever"})

	n, err := s.DeleteExpired(ctx, now)
	if err != nil || n != 2 {
		t.Fatalf("expected 2 removed, got %d (%v)", n, err)
	}
	for _, code := range []string{"new", "forever"} {
		if _, err := s.Get(ctx, code); err != nil {
			t.Errorf("%q should remain: %v", code, err)
		}
	}
}
