package service

import (
	"context"
	"errors"
	"net/url"
	"time"

	"github.com/ASmbat/shortlink/internal/models"
	"github.com/ASmbat/shortlink/internal/store"
	"github.com/google/uuid"
)

var ErrInvalidURL = errors.New("invalid long url")

// codeLength is the number of characters in a generated short code.
const codeLength = 7

// maxLongURLLen bounds the stored long_url length. Without a cap, an
// arbitrarily large string parses fine as a URL (url.ParseRequestURI does
// not limit length) and would be kept in the store forever unless a TTL is
// set — a cheap way to exhaust memory with a handful of requests.
const maxLongURLLen = 4096

// allowedSchemes is the set of long_url schemes Shorten will accept. A
// shortener that redirects browsers to whatever scheme a caller supplies
// would also happily mint links for "javascript://…" or other
// non-http(s) schemes; browsers generally refuse to act on those via a
// redirect response, but there's no reason to accept or store them at all.
var allowedSchemes = map[string]bool{"http": true, "https": true}

type Service struct {
	store store.Store
	now   func() time.Time // injected for testability
}

func New(s store.Store) *Service {
	return &Service{store: s, now: time.Now}
}

// Shorten validates the request and creates a new short link, retrying on
// the (astronomically unlikely) event of a code collision.
func (s *Service) Shorten(ctx context.Context, req models.CreateLinkRequest) (*models.Link, error) {
	if len(req.LongURL) > maxLongURLLen {
		return nil, ErrInvalidURL
	}

	parsed, err := url.ParseRequestURI(req.LongURL)
	if err != nil || parsed.Host == "" || !allowedSchemes[parsed.Scheme] {
		return nil, ErrInvalidURL
	}

	var expires time.Time
	if req.TTLSecond > 0 {
		expires = s.now().Add(time.Duration(req.TTLSecond) * time.Second)
	}

	const maxAttempts = 5
	for attempt := 0; attempt < maxAttempts; attempt++ {
		link := &models.Link{
			Code:      generateCode(),
			LongURL:   req.LongURL,
			CreatedAt: s.now(),
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

// Resolve looks up the destination URL for a code and records a hit.
func (s *Service) Resolve(ctx context.Context, code string) (*models.Link, error) {
	link, err := s.store.Get(ctx, code)
	if err != nil {
		return nil, err
	}
	// Best-effort: a failure to record analytics shouldn't block the redirect.
	_ = s.store.IncrementHits(ctx, code)
	return link, nil
}

func generateCode() string {
	id := uuid.New().String()
	// Strip hyphens and take a short, URL-safe prefix. Collisions are
	// handled by the retry loop in Shorten.
	clean := make([]byte, 0, len(id))
	for _, c := range id {
		if c != '-' {
			clean = append(clean, byte(c))
		}
	}
	return string(clean[:codeLength])
}
