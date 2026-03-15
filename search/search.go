package search

import (
	"context"
	"fmt"
	"time"
)

type SearchResult struct {
	Title   string
	URL     string
	Content string
	Score   float64
}

type SearchProvider interface {
	Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error)
}

type APIError struct {
	StatusCode int
	Message    string
	RetryAfter time.Duration
}

func (e *APIError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("%s (retry in %s)", e.Message, e.RetryAfter.Round(time.Second))
	}
	return e.Message
}
