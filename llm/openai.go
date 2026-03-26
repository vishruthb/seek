package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type OpenAI struct {
	apiKey       string
	baseURL      string
	model        string
	httpClient   *http.Client
	providerName string
}

type openAIRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type openAIChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

func NewOpenAI(apiKey, baseURL, model string) (*OpenAI, error) {
	baseURL, parsed, err := validateBaseURL(baseURL, "https://api.groq.com/openai", "openai_base_url", true)
	if err != nil {
		return nil, err
	}

	return &OpenAI{
		apiKey:       strings.TrimSpace(apiKey),
		baseURL:      strings.TrimRight(baseURL, "/"),
		model:        fallback(strings.TrimSpace(model), "llama-3.3-70b-versatile"),
		httpClient:   newStreamingHTTPClient(),
		providerName: inferProviderName(parsed.Hostname()),
	}, nil
}

func (o *OpenAI) Name() string {
	return o.providerName + "/" + o.model
}

func (o *OpenAI) Warmup(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_, _ = o.StreamChat(ctx, []Message{{Role: "user", Content: "hi"}}, nil)
}

func (o *OpenAI) StreamChat(ctx context.Context, messages []Message, onToken StreamCallback) (string, error) {
	if strings.TrimSpace(o.apiKey) == "" {
		return "", fmt.Errorf("No API key set for %s. Set openai_api_key in config or OPENAI_API_KEY env var.", o.providerName)
	}

	body, err := json.Marshal(openAIRequest{
		Model:    o.model,
		Messages: messages,
		Stream:   true,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, chatCompletionsURL(o.baseURL), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+o.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", readSSEAPIError(resp, o.providerName)
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var (
		dataLines []string
		full      strings.Builder
	)

	processEvent := func() error {
		if len(dataLines) == 0 {
			return nil
		}

		payload := strings.Join(dataLines, "\n")
		dataLines = dataLines[:0]
		if payload == "[DONE]" {
			return io.EOF
		}

		var chunk openAIChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return err
		}
		for _, choice := range chunk.Choices {
			if choice.Delta.Content == "" {
				continue
			}
			full.WriteString(choice.Delta.Content)
			if onToken != nil {
				onToken(choice.Delta.Content)
			}
		}
		return nil
	}

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			if err := processEvent(); err != nil {
				if err == io.EOF {
					break
				}
				return full.String(), err
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}

	if err := scanner.Err(); err != nil {
		return full.String(), err
	}
	if err := processEvent(); err != nil && err != io.EOF {
		return full.String(), err
	}

	return full.String(), nil
}

func chatCompletionsURL(baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	if strings.HasSuffix(baseURL, "/v1") {
		return baseURL + "/chat/completions"
	}
	return baseURL + "/v1/chat/completions"
}

func inferProviderName(host string) string {
	host = strings.TrimPrefix(strings.ToLower(host), "www.")
	switch {
	case strings.Contains(host, "groq"):
		return "groq"
	case strings.Contains(host, "together"):
		return "together"
	case strings.Contains(host, "openrouter"):
		return "openrouter"
	case strings.Contains(host, "openai"):
		return "openai"
	case host == "":
		return "openai"
	default:
		return host
	}
}

func readSSEAPIError(resp *http.Response, provider string) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	message := strings.TrimSpace(string(body))

	var payload struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if len(body) > 0 && json.Unmarshal(body, &payload) == nil {
		switch {
		case strings.TrimSpace(payload.Error.Message) != "":
			message = payload.Error.Message
		case strings.TrimSpace(payload.Message) != "":
			message = payload.Message
		}
	}
	if message == "" {
		message = fmt.Sprintf("%s returned HTTP %d", provider, resp.StatusCode)
	}

	apiErr := &APIError{
		Provider:   provider,
		StatusCode: resp.StatusCode,
		Message:    message,
	}
	if retryAfter := strings.TrimSpace(resp.Header.Get("Retry-After")); retryAfter != "" {
		if seconds, err := strconv.Atoi(retryAfter); err == nil {
			apiErr.RetryAfter = time.Duration(seconds) * time.Second
		}
	}
	return apiErr
}
