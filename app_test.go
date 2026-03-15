package main

import (
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	llmpkg "seek/llm"
	searchpkg "seek/search"
)

type fakeSearchProvider struct {
	calls   []searchCall
	results map[string][]searchpkg.SearchResult
}

type searchCall struct {
	Query      string
	MaxResults int
}

func (f *fakeSearchProvider) Search(ctx context.Context, query string, maxResults int) ([]searchpkg.SearchResult, error) {
	f.calls = append(f.calls, searchCall{Query: query, MaxResults: maxResults})
	return append([]searchpkg.SearchResult(nil), f.results[query]...), nil
}

type fakeLLMProvider struct {
	name      string
	calls     [][]llmpkg.Message
	responses map[string]string
}

func (f *fakeLLMProvider) StreamChat(ctx context.Context, messages []llmpkg.Message, onToken llmpkg.StreamCallback) (string, error) {
	cloned := make([]llmpkg.Message, len(messages))
	copy(cloned, messages)
	f.calls = append(f.calls, cloned)

	query := extractQuestion(messages[len(messages)-1].Content)
	response := f.responses[query]
	if onToken != nil {
		mid := len(response) / 2
		if mid == 0 {
			onToken(response)
		} else {
			onToken(response[:mid])
			onToken(response[mid:])
		}
	}
	return response, nil
}

func (f *fakeLLMProvider) Name() string {
	return f.name
}

func TestRenderMarkdownReplacesCodeBlockPlaceholder(t *testing.T) {
	cfg := DefaultConfig()
	m := NewModel(cfg, "", &fakeSearchProvider{}, &fakeLLMProvider{name: "fake/model"})
	m.width = 120
	m.height = 40
	m.applyLayout()

	out := m.renderMarkdown("## Example\n\n```python\nprint(\"hi\")\n```\n")
	if strings.Contains(out, "SEEKCODEBLOCK000TOKEN") {
		t.Fatalf("placeholder leaked into rendered output: %q", out)
	}
	if !strings.Contains(out, "print(\"hi\")") {
		t.Fatalf("expected rendered output to contain code content, got: %q", out)
	}
}

func TestSanitizeAssistantResponseStripsTrailingSourcesSection(t *testing.T) {
	in := "Transformers are sequence models with self-attention [1].\n\n## Sources\n- https://example.com/one\n- https://example.com/two"
	got := sanitizeAssistantResponse(in)
	if strings.Contains(strings.ToLower(got), "sources") {
		t.Fatalf("expected trailing sources section to be removed, got: %q", got)
	}
	if !strings.Contains(got, "self-attention [1]") {
		t.Fatalf("expected main answer text to remain, got: %q", got)
	}
}

func TestSanitizeAssistantResponseKeepsInlineCitations(t *testing.T) {
	in := "A transformer uses self-attention to weight tokens in context [1][2]."
	got := sanitizeAssistantResponse(in)
	if got != in {
		t.Fatalf("expected inline citations to remain unchanged, got: %q", got)
	}
}

func TestModelQueryLifecycleKeepsSourcesSeparateAndCarriesContext(t *testing.T) {
	searchProvider := &fakeSearchProvider{
		results: map[string][]searchpkg.SearchResult{
			"what is a transformer": {
				{Title: "Attention Is All You Need", URL: "https://example.com/paper", Content: "Transformers use self-attention.", Score: 0.9},
			},
			"what about attention heads": {
				{Title: "Attention Heads", URL: "https://example.com/heads", Content: "Heads let the model attend to different relationships.", Score: 0.8},
			},
		},
	}
	llmProvider := &fakeLLMProvider{
		name: "fake/model",
		responses: map[string]string{
			"what is a transformer":      "A transformer is a sequence model built around self-attention [1].\n\n## Sources\n- https://example.com/paper",
			"what about attention heads": "Attention heads let a transformer track different token relationships in parallel [1].",
		},
	}

	cfg := DefaultConfig()
	cfg.MaxTurns = 10
	cfg.MaxResults = 5
	cfg.LLMBackend = "openai"
	cfg.OpenAIModel = "test-model"

	m := NewModel(cfg, "", searchProvider, llmProvider)
	m.width = 120
	m.height = 40
	m.applyLayout()

	driveQueryCycle(t, m, "what is a transformer", false)

	if len(m.turns) != 1 || len(m.currentSources()) != 1 {
		t.Fatalf("expected first turn with sources, got turns=%d sources=%d", len(m.turns), len(m.currentSources()))
	}
	if strings.Contains(m.composeTranscript(), "## Sources") {
		t.Fatalf("expected trailing sources section to be stripped from transcript, got %q", m.composeTranscript())
	}

	driveQueryCycle(t, m, "what about attention heads", true)

	if len(searchProvider.calls) != 2 {
		t.Fatalf("expected two search calls, got %d", len(searchProvider.calls))
	}
	if len(llmProvider.calls) != 2 {
		t.Fatalf("expected two LLM calls, got %d", len(llmProvider.calls))
	}

	secondCall := llmProvider.calls[1]
	if len(secondCall) != 4 {
		t.Fatalf("expected history to be included on follow-up, got %#v", secondCall)
	}
	if secondCall[1].Role != "user" || secondCall[1].Content != "what is a transformer" {
		t.Fatalf("expected first user message in history, got %#v", secondCall[1])
	}
	if secondCall[2].Role != "assistant" || strings.Contains(secondCall[2].Content, "## Sources") {
		t.Fatalf("expected sanitized assistant history, got %#v", secondCall[2])
	}
	if !strings.Contains(secondCall[3].Content, "Attention Heads") || !strings.Contains(secondCall[3].Content, "Question: what about attention heads") {
		t.Fatalf("expected fresh search context on follow-up, got %#v", secondCall[3])
	}
}

func driveQueryCycle(t *testing.T, m *model, query string, followUp bool) {
	t.Helper()

	m.beginQuery(query, followUp)
	searchMsg, ok := m.startSearch()().(searchCompleteMsg)
	if !ok {
		t.Fatalf("expected searchCompleteMsg")
	}
	_, cmd := m.Update(searchMsg)

	queue := []tea.Cmd{cmd}
	for len(queue) > 0 {
		cmd = queue[0]
		queue = queue[1:]
		if cmd == nil {
			continue
		}

		msg := cmd()
		if msg == nil {
			continue
		}

		switch batch := msg.(type) {
		case tea.BatchMsg:
			queue = append(queue, []tea.Cmd(batch)...)
		case spinner.TickMsg:
			continue
		default:
			_, next := m.Update(msg)
			if next != nil {
				queue = append(queue, next)
			}
		}
	}
}

func extractQuestion(payload string) string {
	const marker = "Question: "
	idx := strings.LastIndex(payload, marker)
	if idx < 0 {
		return strings.TrimSpace(payload)
	}
	return strings.TrimSpace(payload[idx+len(marker):])
}
