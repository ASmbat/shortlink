package models

import "time"

// Link represents a shortened URL entry.
type Link struct {
	Code      string    `json:"code"`
	LongURL   string    `json:"long_url"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	Hits      int64     `json:"hits"`
}

// CreateLinkRequest is the payload for creating a new short link.
type CreateLinkRequest struct {
	LongURL   string `json:"long_url"`
	TTLSecond int    `json:"ttl_seconds,omitempty"`
}
