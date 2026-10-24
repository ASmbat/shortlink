package store

import (
	"context"
	"errors"
	"time"

	"github.com/ASmbat/shortlink/internal/models"
)

// ErrNotFound is returned when a link code does not exist.
var ErrNotFound = errors.New("link not found")

// ErrCodeTaken is returned when saving a link whose code already exists.
var ErrCodeTaken = errors.New("code already exists")

// Store abstracts link persistence, so the service can run against the
// in-memory store in tests and a durable backend (e.g. DynamoDB) in
// production. Expiry rules live in the service, not in the store.
type Store interface {
	Save(ctx context.Context, link *models.Link) error
	Get(ctx context.Context, code string) (*models.Link, error)
	IncrementHits(ctx context.Context, code string) error
	// DeleteExpired removes links that expired before now and returns how
	// many were removed.
	DeleteExpired(ctx context.Context, now time.Time) (int, error)
}
