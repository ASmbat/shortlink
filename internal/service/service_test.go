package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ASmbat/shortlink/internal/models"
	"github.com/ASmbat/shortlink/internal/store"
)

func TestShorten_ValidURL(t *testing.T) {
	svc := New(store.NewMemoryStore())

	link, err := svc.Shorten(context.Background(), models.CreateLinkRequest{
		LongURL: "https://example.com/some/page",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(link.Code) != codeLength {
		t.Errorf("expected code length %d, got %d", codeLength, len(link.Code))
	}
	if link.LongURL != "https://example.com/some/page" {
		t.Errorf("unexpected long url: %s", link.LongURL)
	}
}

func TestShorten_InvalidURL(t *testing.T) {
	svc := New(store.NewMemoryStore())

	cases := []string{"", "not-a-url", "ftp:/missing-slash.com", "javascript://alert(1)", "ftp://example.com/file"}
	for _, in := range cases {
		if _, err := svc.Shorten(context.Background(), models.CreateLinkRequest{LongURL: in}); !errors.Is(err, ErrInvalidURL) {
			t.Errorf("input %q: expected ErrInvalidURL, got %v", in, err)
		}
	}
}

func TestShorten_RejectsOversizedURL(t *testing.T) {
	svc := New(store.NewMemoryStore())

	huge := "https://example.com/" + strings.Repeat("a", maxLongURLLen)
	if _, err := svc.Shorten(context.Background(), models.CreateLinkRequest{LongURL: huge}); !errors.Is(err, ErrInvalidURL) {
		t.Errorf("expected ErrInvalidURL for an oversized long_url, got %v", err)
	}
}

func TestResolve_NotFound(t *testing.T) {
	svc := New(store.NewMemoryStore())

	if _, err := svc.Resolve(context.Background(), "missing"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestResolve_ExpiredLink(t *testing.T) {
	s := store.NewMemoryStore()
	svc := New(s)
	svc.now = func() time.Time { return time.Unix(0, 0) }

	link, err := svc.Shorten(context.Background(), models.CreateLinkRequest{
		LongURL:   "https://example.com",
		TTLSecond: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Move "now" past expiry and resolve.
	svc.now = func() time.Time { return time.Unix(0, 0).Add(20 * time.Second) }
	if _, err := svc.Resolve(context.Background(), link.Code); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected expired link to be ErrNotFound, got %v", err)
	}
}

func TestResolve_IncrementsHits(t *testing.T) {
	s := store.NewMemoryStore()
	svc := New(s)

	link, _ := svc.Shorten(context.Background(), models.CreateLinkRequest{LongURL: "https://example.com"})

	for i := 0; i < 3; i++ {
		if _, err := svc.Resolve(context.Background(), link.Code); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	stored, err := s.Get(context.Background(), link.Code)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stored.Hits != 3 {
		t.Errorf("expected 3 hits, got %d", stored.Hits)
	}
}
