package store

import (
	"context"
	"errors"

	"github.com/ASmbat/shortlink/internal/models"
)

// ErrNotFound is returned when a link code does not exist or has expired.
var ErrNotFound = errors.New("link not found")

// ErrCodeTaken is returned when attempting to create a link with a code
// that already exists.
var ErrCodeTaken = errors.New("code already exists")

// Store abstracts persistence for links so the service layer can be tested
// against an in-memory implementation and run in production against a
// durable backend (e.g. DynamoDB) without changing business logic.
type Store interface {
	Save(ctx context.Context, link *models.Link) error
	Get(ctx context.Context, code string) (*models.Link, error)
	IncrementHits(ctx context.Context, code string) error
}
