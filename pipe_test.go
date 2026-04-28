package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	historypkg "seek/history"
	llmpkg "seek/llm"
	searchpkg "seek/search"
)

type pipeLLMProvider struct {
	name  string
	calls [][]llmpkg.Message
}

func (p *pipeLLMProvider) StreamChat(_ context.Context, messages []llmpkg.Message, onToken llmpkg.StreamCallback) (string, error) {
	p.calls = append(p.calls, append([]llmpkg.Message(nil), messages...))
	answer := "The command failed because the value has the wrong type [1]."
	if onToken != nil {
		onToken(answer)
	}
	return answer, nil
}

func (p *pipeLLMProvider) Name() string {
	return p.name
}

func TestPreparePipedInputPipeOnlyExtractsSearchQuery(t *testing.T) {
	input := preparePipedInput("./main.go:42:15: cannot use x (variable of type string) as int value\n", "")
	if input.UserQuery != "" {
		t.Fatalf("expected no user query, got %q", input.UserQuery)
	}
	if !strings.Contains(input.SearchQuery, "Go cannot use variable of type string as int value") {
		t.Fatalf("expected extracted search query, got %q", input.SearchQuery)
	}
}

func TestPreparePipedInputWithExplicitQueryPreservesQuestion(t *testing.T) {
	input := preparePipedInput("TypeError: Cannot read properties of undefined (reading 'map')\n", "why is this failing")
	if input.UserQuery != "why is this failing" {
		t.Fatalf("expected explicit user query, got %q", input.UserQuery)
	}
	if !strings.Contains(input.SearchQuery, "JavaScript TypeError") {
		t.Fatalf("expected extracted error query to still be available, got %q", input.SearchQuery)
	}
	if got := pipedEffectiveSearchQuery(input); got != "why is this failing" {
		t.Fatalf("expected explicit query to drive search, got %q", got)
	}
}

func TestReadPipedInputCapsAtLimit(t *testing.T) {
	raw := strings.Repeat("a", 12*1024) + strings.Repeat("z", 20*1024)
	got, err := readPipedInput(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("readPipedInput: %v", err)
	}
	if len(got) > pipeInputByteLimit {
		t.Fatalf("expected <= %d bytes, got %d", pipeInputByteLimit, len(got))
	}
	if !strings.HasPrefix(got, "aaa") || !strings.HasSuffix(got, "zzz") {
		t.Fatalf("expected bounded stdin read to preserve start and end")
	}
}

func TestRenderToStdoutPlainMarkdownNoANSI(t *testing.T) {
	var out bytes.Buffer
	renderToStdout(&out, "\x1b[31mAnswer [1]\x1b[0m", []Source{{Title: "Doc", URL: "https://example.com/doc"}})
	got := out.String()
	if strings.Contains(got, "\x1b[") {
		t.Fatalf("expected no ANSI escapes, got %q", got)
	}
	if !strings.Contains(got, "Answer [1]") || !strings.Contains(got, "Sources:") || !strings.Contains(got, "https://example.com/doc") {
		t.Fatalf("expected answer and sources, got %q", got)
	}
}

func TestRunPipedStdoutFullFlowSavesHistory(t *testing.T) {
	input := preparePipedInput("./main.go:42:15: cannot use x (variable of type string) as int value\n", "")
	searchProvider := &fakeSearchProvider{
		results: map[string][]searchpkg.SearchResult{
			input.SearchQuery: {
				{Title: "Go types", URL: "https://example.com/go-types", Content: "Go checks types at compile time.", Score: 1},
			},
		},
	}
	llmProvider := &pipeLLMProvider{name: "fake/pipe"}
	store, err := historypkg.NewHistoryStore(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatalf("NewHistoryStore: %v", err)
	}
	defer store.Close()

	var stdout, stderr bytes.Buffer
	code := runPipedStdout(context.Background(), input, DefaultConfig(), nil, store, searchProvider, llmProvider, t.TempDir(), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected code 0, got %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "The command failed") || !strings.Contains(stdout.String(), "Sources:") {
		t.Fatalf("expected markdown answer with sources, got %q", stdout.String())
	}
	if len(searchProvider.calls) != 1 || searchProvider.calls[0].Query != input.SearchQuery {
		t.Fatalf("expected extracted query to drive search, got %#v", searchProvider.calls)
	}
	if len(llmProvider.calls) != 1 || !strings.Contains(llmProvider.calls[0][1].Content, "./main.go:42:15") {
		t.Fatalf("expected LLM prompt to include error context, got %#v", llmProvider.calls)
	}

	records, err := store.Recent(10, "")
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(records) != 1 || !strings.Contains(records[0].Query, "Go cannot use") {
		t.Fatalf("expected piped search in history, got %#v", records)
	}
}
