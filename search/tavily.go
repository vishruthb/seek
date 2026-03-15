package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const tavilyEndpoint = "https://api.tavily.com/search"

type Tavily struct {
	apiKey     string
	depth      string
	maxResults int
	httpClient *http.Client
}

type tavilyRequest struct {
	APIKey            string `json:"api_key"`
	Query             string `json:"query"`
	SearchDepth       string `json:"search_depth"`
	IncludeAnswer     bool   `json:"include_answer"`
	IncludeRawContent bool   `json:"include_raw_content"`
	MaxResults        int    `json:"max_results"`
}

type tavilyResponse struct {
	Results []SearchResult `json:"results"`
}

type tavilyErrorResponse struct {
	Detail string `json:"detail"`
	Error  string `json:"error"`
}

func NewTavily(apiKey, depth string, maxResults int) (*Tavily, error) {
	switch strings.TrimSpace(strings.ToLower(depth)) {
	case "", "basic":
		depth = "basic"
	case "advanced":
		depth = "advanced"
	default:
		return nil, fmt.Errorf("invalid search_depth %q (use \"basic\" or \"advanced\")", depth)
	}

	if maxResults <= 0 {
		maxResults = 5
	}

	return &Tavily{
		apiKey:     strings.TrimSpace(apiKey),
		depth:      depth,
		maxResults: maxResults,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (t *Tavily) Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	if strings.TrimSpace(t.apiKey) == "" {
		return nil, fmt.Errorf("Tavily API key is missing. Set tavily_api_key in the config file or TAVILY_API_KEY in your environment. Get one at https://tavily.com")
	}
	if maxResults <= 0 {
		maxResults = t.maxResults
	}

	body, err := json.Marshal(tavilyRequest{
		APIKey:            t.apiKey,
		Query:             strings.TrimSpace(query),
		SearchDepth:       t.depth,
		IncludeAnswer:     false,
		IncludeRawContent: false,
		MaxResults:        maxResults,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tavilyEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := checkTavilyResponse(resp); err != nil {
		return nil, err
	}

	var payload tavilyResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	sort.Slice(payload.Results, func(i, j int) bool {
		return payload.Results[i].Score > payload.Results[j].Score
	})

	return payload.Results, nil
}

func checkTavilyResponse(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	message := strings.TrimSpace(string(body))

	var errPayload tavilyErrorResponse
	if len(body) > 0 && json.Unmarshal(body, &errPayload) == nil {
		switch {
		case strings.TrimSpace(errPayload.Detail) != "":
			message = errPayload.Detail
		case strings.TrimSpace(errPayload.Error) != "":
			message = errPayload.Error
		}
	}
	if message == "" {
		message = fmt.Sprintf("Tavily returned HTTP %d", resp.StatusCode)
	}

	apiErr := &APIError{
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
