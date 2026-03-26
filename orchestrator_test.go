package main

import (
	"context"
	"strings"
	"testing"

	projectctx "seek/context"
	llmpkg "seek/llm"
	searchpkg "seek/search"
)

func TestBuildMessagesIncludesHistorySearchContextAndFormat(t *testing.T) {
	history := []llmpkg.Message{
		{Role: "user", Content: "what is a transformer"},
		{Role: "assistant", Content: "A transformer is a sequence model [1]."},
	}
	results := []searchpkg.SearchResult{
		{Title: "Attention Is All You Need", URL: "https://example.com/paper", Content: "Transformers rely on self-attention."},
		{Title: "PyTorch Transformer Tutorial", URL: "https://example.com/tutorial", Content: "PyTorch includes nn.Transformer."},
	}

	messages := buildMessages("show me a code example", results, history, nil, "learning", nil)
	if len(messages) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(messages))
	}
	if messages[0].Role != "system" || !strings.Contains(messages[0].Content, "Output format preference: learning") {
		t.Fatalf("expected learning system prompt, got %#v", messages[0])
	}
	if !strings.Contains(messages[0].Content, "Do not append a separate Sources") {
		t.Fatalf("expected sources-section instruction in system prompt")
	}
	if messages[1] != history[0] || messages[2] != history[1] {
		t.Fatalf("expected conversation history to be preserved, got %#v", messages[1:3])
	}
	last := messages[3]
	if last.Role != "user" {
		t.Fatalf("expected final message to be user, got %q", last.Role)
	}
	if !strings.Contains(last.Content, "[1] Attention Is All You Need") || !strings.Contains(last.Content, "[2] PyTorch Transformer Tutorial") {
		t.Fatalf("expected numbered search context, got %q", last.Content)
	}
	if !strings.Contains(last.Content, "Question: show me a code example") {
		t.Fatalf("expected question in final user message, got %q", last.Content)
	}
}

func TestSearchEnrichesQueryWithProjectContext(t *testing.T) {
	searchProvider := &fakeSearchProvider{results: map[string][]searchpkg.SearchResult{}}
	orchestrator := NewOrchestrator(
		searchProvider,
		&fakeLLMProvider{name: "fake/model"},
		5,
		"concise",
		&projectctx.ProjectContext{Language: "go", Framework: "chi"},
	)

	if _, err := orchestrator.Search(context.Background(), "add middleware"); err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(searchProvider.calls) != 1 {
		t.Fatalf("expected one search call, got %d", len(searchProvider.calls))
	}
	if !strings.Contains(searchProvider.calls[0].Query, "add middleware") || !strings.Contains(searchProvider.calls[0].Query, "go") || !strings.Contains(searchProvider.calls[0].Query, "chi") {
		t.Fatalf("expected enriched query, got %q", searchProvider.calls[0].Query)
	}
}

func TestSearchLeavesQueryUnchangedWithoutProjectContext(t *testing.T) {
	searchProvider := &fakeSearchProvider{results: map[string][]searchpkg.SearchResult{}}
	orchestrator := NewOrchestrator(searchProvider, &fakeLLMProvider{name: "fake/model"}, 5, "concise", nil)

	if _, err := orchestrator.Search(context.Background(), "add middleware"); err != nil {
		t.Fatalf("Search: %v", err)
	}
	if got := searchProvider.calls[0].Query; got != "add middleware" {
		t.Fatalf("expected query to remain unchanged, got %q", got)
	}
}

func TestBuildSystemPromptIncludesProjectContext(t *testing.T) {
	prompt := buildSystemPrompt("concise", &projectctx.ProjectContext{
		Language:     "rust",
		Framework:    "axum",
		Dependencies: []string{"tokio", "serde"},
	})

	for _, want := range []string{"rust", "axum", "tokio", "serde"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected prompt to contain %q, got %q", want, prompt)
		}
	}
}

func TestBuildMessagesIncludesAttachedFiles(t *testing.T) {
	messages := buildMessages(
		"review @[app.go]",
		nil,
		nil,
		[]AttachedFile{{DisplayPath: "app.go", Language: "go", Content: "package main\nfunc main() {}\n"}},
		"concise",
		nil,
	)

	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if !strings.Contains(messages[1].Content, "Local file context:") || !strings.Contains(messages[1].Content, "[FILE 1] app.go") || !strings.Contains(messages[1].Content, "func main() {}") {
		t.Fatalf("expected attached file content in user message, got %q", messages[1].Content)
	}
}

func TestBuildSystemPromptOmitsProjectContextWhenNil(t *testing.T) {
	prompt := buildSystemPrompt("concise", nil)
	if strings.Contains(prompt, "working in a") || strings.Contains(prompt, "framework") {
		t.Fatalf("expected prompt without project context, got %q", prompt)
	}
}

func TestSourcesFromSearchResultsNormalizesDomains(t *testing.T) {
	sources := sourcesFromSearchResults([]searchpkg.SearchResult{
		{Title: "RFC", URL: "https://www.rfc-editor.org/rfc/rfc793"},
	})
	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}
	if sources[0].Domain != "rfc-editor.org" {
		t.Fatalf("expected normalized domain, got %q", sources[0].Domain)
	}
}
