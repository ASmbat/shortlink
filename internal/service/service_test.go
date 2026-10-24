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

	// Still valid just before the TTL ends.
	svc.now = func() time.Time { return time.Unix(0, 0).Add(9 * time.Second) }
	if _, err := svc.Resolve(context.Background(), link.Code); err != nil {
		t.Fatalf("link should still be valid before its TTL: %v", err)
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

func TestGenerateCode_FormatAndUniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 10000; i++ {
		code, err := generateCode()
		if err != nil {
			t.Fatal(err)
		}
		if len(code) != codeLength {
			t.Fatalf("code %q has length %d, want %d", code, len(code), codeLength)
		}
		for _, c := range code {
			if !strings.ContainsRune(alphabet, c) {
				t.Fatalf("code %q contains %q, which is not in the base62 alphabet", code, c)
			}
		}
		if seen[code] {
			t.Fatalf("duplicate code %q after %d codes", code, i)
		}
		seen[code] = true
	}
}

// takenStore reports every code as taken, to test the retry limit.
type takenStore struct{ store.Store }

func (takenStore) Save(context.Context, *models.Link) error { return store.ErrCodeTaken }

func TestShorten_GivesUpAfterRepeatedCollisions(t *testing.T) {
	svc := New(takenStore{store.NewMemoryStore()})
	if _, err := svc.Shorten(context.Background(), models.CreateLinkRequest{LongURL: "https://example.com"}); err == nil {
		t.Fatal("expected an error when every code collides")
	}
}

func TestDeleteExpired_RemovesOnlyExpiredLinks(t *testing.T) {
	s := store.NewMemoryStore()
	svc := New(s)
	start := time.Unix(1000, 0)
	svc.now = func() time.Time { return start }

	ctx := context.Background()
	if _, err := svc.Shorten(ctx, models.CreateLinkRequest{LongURL: "https://a.example", TTLSecond: 10}); err != nil {
		t.Fatal(err)
	}
	keep, err := svc.Shorten(ctx, models.CreateLinkRequest{LongURL: "https://b.example"}) // no TTL
	if err != nil {
		t.Fatal(err)
	}

	svc.now = func() time.Time { return start.Add(time.Minute) }
	n, err := svc.DeleteExpired(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || s.Len() != 1 {
		t.Fatalf("expected 1 removed and 1 left, got removed=%d left=%d", n, s.Len())
	}
	if _, err := svc.Get(ctx, keep.Code); err != nil {
		t.Errorf("link without TTL should remain: %v", err)
	}
}

func TestRunCleanup_StopsOnCancel(t *testing.T) {
	svc := New(store.NewMemoryStore())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		svc.RunCleanup(ctx, time.Millisecond, t.Logf)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("RunCleanup did not stop after cancel")
	}
}
