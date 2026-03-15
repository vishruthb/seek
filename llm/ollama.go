package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
)

type Ollama struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

type ollamaRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type ollamaResponse struct {
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	Done  bool   `json:"done"`
	Error string `json:"error"`
}

func NewOllama(baseURL, model string) *Ollama {
	return &Ollama{
		baseURL:    normalizeBaseURL(baseURL, "http://localhost:11434"),
		model:      fallback(strings.TrimSpace(model), "llama3.1:8b"),
		httpClient: &http.Client{},
	}
}

func (o *Ollama) Name() string {
	return "ollama/" + o.model
}

func (o *Ollama) StreamChat(ctx context.Context, messages []Message, onToken StreamCallback) (string, error) {
	body, err := json.Marshal(ollamaRequest{
		Model:    o.model,
		Messages: messages,
		Stream:   true,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(o.baseURL, "/")+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := o.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if isConnectionError(err) {
			return "", fmt.Errorf("Cannot connect to Ollama at %s. Is it running? (ollama serve)", strings.TrimPrefix(o.baseURL, "http://"))
		}
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", readAPIError(resp, "ollama")
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var full strings.Builder
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var chunk ollamaResponse
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			return full.String(), err
		}
		if strings.TrimSpace(chunk.Error) != "" {
			return full.String(), errors.New(chunk.Error)
		}
		if chunk.Message.Content != "" {
			full.WriteString(chunk.Message.Content)
			if onToken != nil {
				onToken(chunk.Message.Content)
			}
		}
		if chunk.Done {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return full.String(), err
	}
	return full.String(), nil
}

func normalizeBaseURL(rawURL, defaultURL string) string {
	value := strings.TrimSpace(rawURL)
	if value == "" {
		return defaultURL
	}
	if !strings.Contains(value, "://") {
		scheme := "http"
		if parsed, err := url.Parse(defaultURL); err == nil && parsed.Scheme != "" {
			scheme = parsed.Scheme
		}
		value = scheme + "://" + value
	}
	return strings.TrimRight(value, "/")
}

func fallback(value, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return strings.TrimSpace(value)
}

func isConnectionError(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return true
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "connection refused") || strings.Contains(msg, "no such host")
}

func readAPIError(resp *http.Response, provider string) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	message := strings.TrimSpace(string(body))

	var payload struct {
		Error   json.RawMessage `json:"error"`
		Message string          `json:"message"`
	}
	if len(body) > 0 && json.Unmarshal(body, &payload) == nil {
		switch {
		case strings.TrimSpace(payload.Message) != "":
			message = payload.Message
		case len(payload.Error) > 0:
			var errObj struct {
				Message string `json:"message"`
			}
			var errText string
			switch {
			case json.Unmarshal(payload.Error, &errObj) == nil && strings.TrimSpace(errObj.Message) != "":
				message = errObj.Message
			case json.Unmarshal(payload.Error, &errText) == nil && strings.TrimSpace(errText) != "":
				message = errText
			}
		}
	}
	if message == "" {
		message = fmt.Sprintf("%s returned HTTP %d", provider, resp.StatusCode)
	}

	return &APIError{
		Provider:   provider,
		StatusCode: resp.StatusCode,
		Message:    message,
	}
}
