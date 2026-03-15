package main

import (
	"strings"
	"testing"

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

	messages := buildMessages("show me a code example", results, history, "learning")
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
