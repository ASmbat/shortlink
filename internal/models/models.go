package models

import "time"

// Link represents a shortened URL entry.
type Link struct {
	Code      string    `json:"code"`
	LongURL   string    `json:"long_url"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at,omitzero"`
	Hits      int64     `json:"hits"`
}

// Expired reports whether the link has a TTL that has passed at now.
func (l *Link) Expired(now time.Time) bool {
	return !l.ExpiresAt.IsZero() && !now.Before(l.ExpiresAt)
}

// CreateLinkRequest is the payload for creating a new short link.
type CreateLinkRequest struct {
	LongURL   string `json:"long_url"`
	TTLSecond int    `json:"ttl_seconds,omitempty"`
}
