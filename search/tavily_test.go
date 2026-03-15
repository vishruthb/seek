package search

import (
	"bytes"
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

func TestTavilySearchSendsExpectedPayloadAndSortsResults(t *testing.T) {
	var seen tavilyRequest
	tavily, err := NewTavily("test-key", "advanced", 5)
	if err != nil {
		t.Fatalf("NewTavily: %v", err)
	}
	tavily.httpClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", req.Method)
			}
			if req.URL.String() != tavilyEndpoint {
				t.Fatalf("unexpected URL: %s", req.URL.String())
			}
			if err := json.NewDecoder(req.Body).Decode(&seen); err != nil {
				t.Fatalf("decode request: %v", err)
			}

			body := `{"results":[
				{"title":"lower","url":"https://example.com/lower","content":"two","score":0.4},
				{"title":"higher","url":"https://example.com/higher","content":"one","score":0.9}
			]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	}

	results, err := tavily.Search(context.Background(), "  transformers  ", 3)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if seen.APIKey != "test-key" || seen.Query != "transformers" {
		t.Fatalf("unexpected request payload: %#v", seen)
	}
	if seen.SearchDepth != "advanced" || seen.MaxResults != 3 {
		t.Fatalf("unexpected request payload: %#v", seen)
	}
	if len(results) != 2 || results[0].Title != "higher" || results[1].Title != "lower" {
		t.Fatalf("expected results to be sorted by score, got %#v", results)
	}
}

func TestTavilySearchRejectsMissingAPIKey(t *testing.T) {
	tavily, err := NewTavily("", "basic", 5)
	if err != nil {
		t.Fatalf("NewTavily: %v", err)
	}

	_, err = tavily.Search(context.Background(), "query", 5)
	if err == nil || !strings.Contains(err.Error(), "Tavily API key is missing") {
		t.Fatalf("expected missing API key error, got %v", err)
	}
}

func TestCheckTavilyResponseParsesRetryAfter(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     http.Header{"Retry-After": []string{"6"}},
		Body:       io.NopCloser(bytes.NewBufferString(`{"detail":"slow down"}`)),
	}

	err := checkTavilyResponse(resp)
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Message != "slow down" || apiErr.RetryAfter.Seconds() != 6 {
		t.Fatalf("unexpected API error: %#v", apiErr)
	}
}
