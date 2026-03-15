package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestOpenAIStreamChatStreamsSSE(t *testing.T) {
	var seen openAIRequest
	client, err := NewOpenAI("test-key", "https://api.groq.com/openai", "test-model")
	if err != nil {
		t.Fatalf("NewOpenAI: %v", err)
	}
	client.httpClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/openai/v1/chat/completions" {
				t.Fatalf("unexpected path: %s", req.URL.Path)
			}
			if got := req.Header.Get("Authorization"); got != "Bearer test-key" {
				t.Fatalf("unexpected auth header: %q", got)
			}
			if err := json.NewDecoder(req.Body).Decode(&seen); err != nil {
				t.Fatalf("decode request: %v", err)
			}

			body := "data: {\"choices\":[{\"delta\":{\"content\":\"hello \"}}]}\n\n" +
				"data: {\"choices\":[{\"delta\":{\"content\":\"world\"}}]}\n\n" +
				"data: [DONE]\n\n"
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	}

	var tokens []string
	full, err := client.StreamChat(context.Background(), []Message{{Role: "user", Content: "hello"}}, func(token string) {
		tokens = append(tokens, token)
	})
	if err != nil {
		t.Fatalf("StreamChat: %v", err)
	}

	if seen.Model != "test-model" || !seen.Stream || len(seen.Messages) != 1 {
		t.Fatalf("unexpected request payload: %#v", seen)
	}
	if full != "hello world" {
		t.Fatalf("expected accumulated response, got %q", full)
	}
	if strings.Join(tokens, "") != "hello world" {
		t.Fatalf("expected streamed tokens, got %#v", tokens)
	}
}

func TestOpenAIStreamChatParsesAPIError(t *testing.T) {
	client, err := NewOpenAI("test-key", "https://api.groq.com/openai", "test-model")
	if err != nil {
		t.Fatalf("NewOpenAI: %v", err)
	}
	client.httpClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Header:     http.Header{"Retry-After": []string{"5"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"slow down"}}`)),
			}, nil
		}),
	}

	_, err = client.StreamChat(context.Background(), []Message{{Role: "user", Content: "hello"}}, nil)
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T (%v)", err, err)
	}
	if apiErr.StatusCode != http.StatusTooManyRequests || apiErr.Message != "slow down" || apiErr.RetryAfter.Seconds() != 5 {
		t.Fatalf("unexpected API error: %#v", apiErr)
	}
}
