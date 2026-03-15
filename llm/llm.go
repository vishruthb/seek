package llm

import (
	"context"
	"fmt"
	"time"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type StreamCallback func(token string)

type LLMProvider interface {
	StreamChat(ctx context.Context, messages []Message, onToken StreamCallback) (string, error)
	Name() string
}

type APIError struct {
	Provider   string
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
