package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/ASmbat/shortlink/internal/models"
	"github.com/ASmbat/shortlink/internal/store"
)

var ErrInvalidURL = errors.New("invalid long url")

const (
	// codeLength is the number of characters in a short code. With 62
	// symbols, 7 characters give about 3.5 trillion possible codes.
	codeLength = 7
	alphabet   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	// maxLongURLLen bounds how much a single link can store.
	maxLongURLLen = 4096
)

// reservedCodes are paths used by other routes; a generated code must
// never shadow them.
var reservedCodes = map[string]bool{"healthz": true, "links": true}

var allowedSchemes = map[string]bool{"http": true, "https": true}

type Service struct {
	store store.Store
	now   func() time.Time // replaceable in tests
}

func New(s store.Store) *Service {
	return &Service{store: s, now: time.Now}
}

// Shorten validates the request and creates a new short link, retrying if
// a generated code is already taken.
func (s *Service) Shorten(ctx context.Context, req models.CreateLinkRequest) (*models.Link, error) {
	if len(req.LongURL) > maxLongURLLen {
		return nil, ErrInvalidURL
	}
	parsed, err := url.ParseRequestURI(req.LongURL)
	if err != nil || parsed.Host == "" || !allowedSchemes[parsed.Scheme] {
		return nil, ErrInvalidURL
	}

	now := s.now()
	var expires time.Time
	if req.TTLSecond > 0 {
		expires = now.Add(time.Duration(req.TTLSecond) * time.Second)
	}

	const maxAttempts = 5
	for attempt := 0; attempt < maxAttempts; attempt++ {
		code, err := generateCode()
		if err != nil {
			return nil, err
		}
		if reservedCodes[code] {
			continue
		}
		link := &models.Link{
			Code:      code,
			LongURL:   req.LongURL,
			CreatedAt: now,
			ExpiresAt: expires,
		}
		if err := s.store.Save(ctx, link); err != nil {
			if errors.Is(err, store.ErrCodeTaken) {
				continue
			}
			return nil, err
		}
		return link, nil
	}
	return nil, errors.New("could not generate a unique code, try again")
}

// Resolve returns the link for code and records a hit.
func (s *Service) Resolve(ctx context.Context, code string) (*models.Link, error) {
	link, err := s.Get(ctx, code)
	if err != nil {
		return nil, err
	}
	// Best effort: failing to count a hit must not block the redirect.
	_ = s.store.IncrementHits(ctx, code)
	return link, nil
}

// Get returns the link for code without recording a hit. Expired links
// are reported as not found.
func (s *Service) Get(ctx context.Context, code string) (*models.Link, error) {
	link, err := s.store.Get(ctx, code)
	if err != nil {
		return nil, err
	}
	if link.Expired(s.now()) {
		return nil, store.ErrNotFound
	}
	return link, nil
}

// DeleteExpired removes expired links from the store, so links with a TTL
// don't use memory forever.
func (s *Service) DeleteExpired(ctx context.Context) (int, error) {
	return s.store.DeleteExpired(ctx, s.now())
}

// RunCleanup calls DeleteExpired every interval until ctx is cancelled.
func (s *Service) RunCleanup(ctx context.Context, interval time.Duration, logf func(string, ...any)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := s.DeleteExpired(ctx)
			if err != nil {
				logf("cleanup failed: %v", err)
			} else if n > 0 {
				logf("cleanup removed %d expired links", n)
			}
		}
	}
}

// generateCode returns a random base62 code from crypto/rand. Bytes >= 248
// are skipped so every character is equally likely (248 = 4 * 62).
func generateCode() (string, error) {
	code := make([]byte, 0, codeLength)
	buf := make([]byte, codeLength*2)
	for len(code) < codeLength {
		if _, err := rand.Read(buf); err != nil {
			return "", fmt.Errorf("reading random bytes: %w", err)
		}
		for _, b := range buf {
			if b >= 248 {
				continue
			}
			code = append(code, alphabet[int(b)%len(alphabet)])
			if len(code) == codeLength {
				break
			}
		}
	}
	return string(code), nil
}
