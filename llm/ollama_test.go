package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestOllamaStreamChatStreamsNDJSON(t *testing.T) {
	var seen ollamaRequest
	client := NewOllama("http://localhost:11434", "llama3.1:8b")
	client.httpClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/api/chat" {
				t.Fatalf("unexpected path: %s", req.URL.Path)
			}
			if err := json.NewDecoder(req.Body).Decode(&seen); err != nil {
				t.Fatalf("decode request: %v", err)
			}

			body := "{\"message\":{\"role\":\"assistant\",\"content\":\"hello \"},\"done\":false}\n" +
				"{\"message\":{\"role\":\"assistant\",\"content\":\"ollama\"},\"done\":true}\n"
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
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

	if seen.Model != "llama3.1:8b" || !seen.Stream || len(seen.Messages) != 1 {
		t.Fatalf("unexpected request payload: %#v", seen)
	}
	if full != "hello ollama" || strings.Join(tokens, "") != "hello ollama" {
		t.Fatalf("unexpected streamed output: full=%q tokens=%#v", full, tokens)
	}
}

func TestOllamaStreamChatReturnsHelpfulConnectionError(t *testing.T) {
	client := NewOllama("http://localhost:11434", "llama3.1:8b")
	client.httpClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return nil, &url.Error{Op: "Post", URL: req.URL.String(), Err: errors.New("connection refused")}
		}),
	}

	_, err := client.StreamChat(context.Background(), []Message{{Role: "user", Content: "hello"}}, nil)
	if err == nil || !strings.Contains(err.Error(), "Cannot connect to Ollama") {
		t.Fatalf("expected helpful connection error, got %v", err)
	}
}
