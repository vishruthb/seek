//go:build integration

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	projectctx "seek/context"
	historypkg "seek/history"
	llmpkg "seek/llm"
	searchpkg "seek/search"
)

func TestHistoryPersistsAcrossStoreInstances(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "history.db")
	store, err := historypkg.NewHistoryStore(dbPath)
	if err != nil {
		t.Fatalf("NewHistoryStore: %v", err)
	}
	for i := 0; i < 3; i++ {
		if _, err := store.Save(&historypkg.SearchRecord{
			Query:    "query",
			Response: "response",
			TotalMs:  100,
		}); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := historypkg.NewHistoryStore(dbPath)
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	defer reopened.Close()

	records, err := reopened.Recent(10, "")
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 persisted records, got %d", len(records))
	}
}

func TestDetectContextOnSeekRepo(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}

	ctx := projectctx.DetectContext(cwd)
	if ctx == nil {
		t.Fatal("expected context for seek repo")
	}
	if ctx.Language != "go" {
		t.Fatalf("expected go context, got %#v", ctx)
	}
	if !containsAll(ctx.Dependencies, "bubbletea") {
		t.Fatalf("expected bubbletea dependency, got %#v", ctx.Dependencies)
	}
}

func TestFullPipelineWithRealTavilyAndMockLLM(t *testing.T) {
	apiKey := strings.TrimSpace(os.Getenv("TAVILY_API_KEY"))
	if apiKey == "" {
		t.Skip("TAVILY_API_KEY is not set")
	}

	searchProvider, err := searchpkg.NewTavily(apiKey, "basic", 5)
	if err != nil {
		t.Fatalf("NewTavily: %v", err)
	}
	llmProvider := &integrationLLMProvider{}
	orchestrator := NewOrchestrator(searchProvider, llmProvider, 5, "concise", &projectctx.ProjectContext{
		Language:  "go",
		Framework: "chi",
	})

	results, err := orchestrator.Search(context.Background(), "how to add middleware")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected search results")
	}

	_, err = orchestrator.StreamAnswer(context.Background(), "how to add middleware", results, nil, nil, nil)
	if err != nil {
		t.Fatalf("StreamAnswer: %v", err)
	}
	if len(llmProvider.lastMessages) == 0 {
		t.Fatal("expected llm provider to receive messages")
	}
}

type integrationLLMProvider struct {
	lastMessages []llmpkg.Message
}

func (p *integrationLLMProvider) StreamChat(_ context.Context, messages []llmpkg.Message, _ llmpkg.StreamCallback) (string, error) {
	p.lastMessages = append([]llmpkg.Message(nil), messages...)
	if len(messages) == 0 {
		return "", nil
	}
	content := messages[len(messages)-1].Content
	if len(content) > 100 {
		content = content[:100]
	}
	return content, nil
}

func (p *integrationLLMProvider) Name() string {
	return "integration/mock"
}

func containsAll(haystack []string, needles ...string) bool {
	seen := make(map[string]struct{}, len(haystack))
	for _, item := range haystack {
		seen[item] = struct{}{}
	}
	for _, needle := range needles {
		if _, ok := seen[needle]; !ok {
			return false
		}
	}
	return true
}
