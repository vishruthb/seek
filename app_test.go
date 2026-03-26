package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	projectctx "seek/context"
	historypkg "seek/history"
	llmpkg "seek/llm"
	searchpkg "seek/search"
	"seek/ui"
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

func TestStartupLayoutStaysCompactWithoutExtraMiddlePanel(t *testing.T) {
	cfg := DefaultConfig()
	m := NewModel(cfg, "", &fakeSearchProvider{}, &fakeLLMProvider{name: "fake/model"})
	m.width = 160
	m.height = 40
	m.state = StateInput
	m.applyLayout()

	if m.contentW != 158 {
		t.Fatalf("expected startup content width to use the terminal width, got %d", m.contentW)
	}
	if m.sourcesH != 0 {
		t.Fatalf("expected no middle sources/composer panel on empty startup, got %d", m.sourcesH)
	}
	if m.contentH != startupShellMaxHeight {
		t.Fatalf("expected capped startup shell height, got %d", m.contentH)
	}
	expectedSplashHeight := ui.PreferredSplashHeight(m.contentW-2) + 3
	if m.summaryH != expectedSplashHeight {
		t.Fatalf("expected startup summary height %d, got %d", expectedSplashHeight, m.summaryH)
	}

	view := m.View()
	if !strings.Contains(view, "███████╗") {
		t.Fatalf("expected the startup view to show the seek logo, got %q", view)
	}
	if strings.Contains(view, "press `f` to search") {
		t.Fatalf("expected startup view to omit the header strip, got %q", view)
	}
	if strings.Contains(view, "┌") {
		t.Fatalf("expected startup view to avoid nested summary boxes, got %q", view)
	}
	if strings.Contains(view, "\n││                                                           / for commands") {
		t.Fatalf("expected no extra middle helper strip in empty startup view, got %q", view)
	}
}

func TestStartupViewFitsNarrowWindow(t *testing.T) {
	cfg := DefaultConfig()
	m := NewModel(cfg, "", &fakeSearchProvider{}, &fakeLLMProvider{name: "fake/model"})
	m.width = 18
	m.height = 12
	m.state = StateInput
	m.applyLayout()

	assertViewFits(t, m.View(), m.width)
}

func TestStartupEscapeKeepsInputModeAndBottomAnchoredShell(t *testing.T) {
	cfg := DefaultConfig()
	m := NewModel(cfg, "", &fakeSearchProvider{}, &fakeLLMProvider{name: "fake/model"})
	m.width = 120
	m.height = 36
	m.state = StateInput
	m.applyLayout()

	_ = m.handleInputKeys(tea.KeyMsg{Type: tea.KeyEsc})
	if m.state != StateInput {
		t.Fatalf("expected startup esc to stay in input mode, got %v", m.state)
	}

	view := m.View()
	if idx := strings.Index(view, "╭"); idx <= 0 {
		t.Fatalf("expected bottom-anchored startup shell with leading padding, got %q", view)
	}
}

func TestStartupSlashLayoutKeepsFullLogoVisible(t *testing.T) {
	cfg := DefaultConfig()
	m := NewModel(cfg, "", &fakeSearchProvider{}, &fakeLLMProvider{name: "fake/model"})
	m.width = 140
	m.height = 36
	m.state = StateInput
	m.followInput.SetValue("/")
	m.applyLayout()

	minSplashHeight := ui.PreferredSplashHeight(m.contentW-2) + 2
	if m.summaryH < minSplashHeight {
		t.Fatalf("expected slash layout to preserve splash height %d, got %d", minSplashHeight, m.summaryH)
	}
	if m.sourcesH != startupSuggestionPanelHeight {
		t.Fatalf("expected startup suggestions height %d, got %d", startupSuggestionPanelHeight, m.sourcesH)
	}

	view := m.View()
	if !strings.Contains(view, "╚══════╝╚══════╝╚══════╝╚═╝ ╚═╝") {
		t.Fatalf("expected full seek logo to remain visible, got %q", view)
	}
	if !strings.Contains(view, "/backend <ollama|openai>") {
		t.Fatalf("expected slash command suggestions to remain visible, got %q", view)
	}
}

func TestSlashSuggestionsNavigateAndAcceptSelection(t *testing.T) {
	cfg := DefaultConfig()
	m := NewModel(cfg, "", &fakeSearchProvider{}, &fakeLLMProvider{name: "fake/model"})
	m.width = 140
	m.height = 36
	m.state = StateInput
	m.setFollowInputValue("/")
	m.applyLayout()

	_ = m.handleInputKeys(tea.KeyMsg{Type: tea.KeyDown})
	_ = m.handleInputKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.inputSuggestionIndex != 2 {
		t.Fatalf("expected j to move the focused suggestion, got %d", m.inputSuggestionIndex)
	}

	_ = m.handleInputKeys(tea.KeyMsg{Type: tea.KeyEnter})
	if got := m.followInput.Value(); got != "/format " {
		t.Fatalf("expected enter to accept selected slash suggestion, got %q", got)
	}
}

func TestAttachmentSuggestionsInsertSelectedFile(t *testing.T) {
	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, "alpha.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write alpha.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "beta.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write beta.go: %v", err)
	}

	m := NewModelWithOptions(DefaultConfig(), "", &fakeSearchProvider{}, &fakeLLMProvider{name: "fake/model"}, ModelOptions{
		WorkingDir: projectDir,
	})
	m.width = 140
	m.height = 36
	m.state = StateInput
	m.setFollowInputValue("review @[")
	m.applyLayout()

	_ = m.handleInputKeys(tea.KeyMsg{Type: tea.KeyDown})
	_ = m.handleInputKeys(tea.KeyMsg{Type: tea.KeyEnter})

	if got := m.followInput.Value(); got != "review @[beta.go] " {
		t.Fatalf("expected file suggestion to be inserted, got %q", got)
	}
}

func TestExitSlashCommandReturnsQuit(t *testing.T) {
	cfg := DefaultConfig()
	m := NewModel(cfg, "", &fakeSearchProvider{}, &fakeLLMProvider{name: "fake/model"})

	cmd := m.executeSlashCommand("/exit")
	if cmd == nil {
		t.Fatal("expected /exit to return a quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected /exit to emit tea.QuitMsg")
	}
}

func TestSanitizeTerminalTextStripsEscapeSequences(t *testing.T) {
	value := "safe\x1b]52;c;bad\a text\x1b[31m!\x1b[0m"
	got := sanitizeTerminalText(value)
	if got != "safe text!" {
		t.Fatalf("unexpected sanitized text: %q", got)
	}
}

func TestContextSlashCommandShowsAndTogglesProjectContext(t *testing.T) {
	projectDir := t.TempDir()
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte("module example.com/test\nrequire github.com/go-chi/chi/v5 v5.0.0\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	projectContext := projectctx.DetectContext(projectDir)
	m := NewModelWithOptions(DefaultConfig(), "", &fakeSearchProvider{}, &fakeLLMProvider{name: "fake/model"}, ModelOptions{
		ProjectContext: projectContext,
		WorkingDir:     projectDir,
	})
	m.width = 120
	m.height = 40
	m.state = StateInput
	m.applyLayout()

	_ = m.executeSlashCommand("/context")
	if !strings.Contains(m.overlayContent, "Go project using chi") {
		t.Fatalf("expected context overlay, got %q", m.overlayContent)
	}

	_ = m.executeSlashCommand("/context off")
	if m.projectContext != nil {
		t.Fatalf("expected project context to be disabled")
	}
	if !strings.Contains(strings.ToLower(m.overlayContent), "disabled") {
		t.Fatalf("expected disabled context overlay, got %q", m.overlayContent)
	}

	_ = m.executeSlashCommand("/context on")
	if m.projectContext == nil || m.projectContext.Framework != "chi" {
		t.Fatalf("expected project context to re-enable, got %#v", m.projectContext)
	}
}

func TestHistorySlashCommandsRenderSavedEntries(t *testing.T) {
	store, err := historypkg.NewHistoryStore(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatalf("NewHistoryStore: %v", err)
	}
	defer store.Close()

	_, err = store.Save(&historypkg.SearchRecord{
		Query:        "tcp handshake",
		Response:     "A TCP handshake uses SYN, SYN-ACK, ACK.",
		ProjectDir:   "/workspace/project",
		ProjectStack: "go/chi",
		LLMBackend:   "fake/model",
		OutputFormat: "concise",
		TotalMs:      250,
	})
	if err != nil {
		t.Fatalf("Save history: %v", err)
	}

	m := NewModelWithOptions(DefaultConfig(), "", &fakeSearchProvider{}, &fakeLLMProvider{name: "fake/model"}, ModelOptions{
		HistoryStore: store,
	})
	m.width = 120
	m.height = 40
	m.state = StateInput
	m.applyLayout()

	_ = m.executeSlashCommand("/history tcp")
	if !strings.Contains(m.overlayContent, "tcp handshake") || !strings.Contains(m.overlayContent, "seek --open <id>") {
		t.Fatalf("expected history overlay, got %q", m.overlayContent)
	}

	_ = m.executeSlashCommand("/recent")
	if !strings.Contains(m.overlayContent, "Recent Searches") {
		t.Fatalf("expected recent overlay, got %q", m.overlayContent)
	}
}

func TestTimingClearsFromStatusMetaAfterTick(t *testing.T) {
	m := NewModelWithOptions(DefaultConfig(), "", &fakeSearchProvider{}, &fakeLLMProvider{name: "fake/model"}, ModelOptions{
		ProjectContext: &projectctx.ProjectContext{Language: "go", Framework: "chi"},
	})
	m.lastTiming = SearchTiming{SearchMs: 312, LLMMs: 535, TotalMs: 847}
	m.timingVisible = true
	m.timingSeq = 1

	if got := m.statusMeta(); !strings.Contains(got, "847ms") || !strings.Contains(got, "go/chi") {
		t.Fatalf("expected timing-rich status meta, got %q", got)
	}

	if _, cmd := m.Update(timingClearMsg{Seq: 1}); cmd != nil {
		t.Fatalf("expected no follow-up cmd from timing clear")
	}
	if got := m.statusMeta(); strings.Contains(got, "847ms") || !strings.Contains(got, "fake/model") {
		t.Fatalf("expected timing to clear from status meta, got %q", got)
	}
}

func TestQueryLifecycleIncludesAttachedFileContext(t *testing.T) {
	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, "app.go"), []byte("package main\n\nfunc handler() {}\n"), 0o644); err != nil {
		t.Fatalf("write app.go: %v", err)
	}

	searchProvider := &fakeSearchProvider{
		results: map[string][]searchpkg.SearchResult{
			"explain app.go": {
				{Title: "Go handler", URL: "https://example.com/handler", Content: "Handlers respond to requests.", Score: 0.9},
			},
		},
	}
	llmProvider := &fakeLLMProvider{
		name: "fake/model",
		responses: map[string]string{
			"explain @[app.go]": "This defines a handler [1].",
		},
	}

	m := NewModelWithOptions(DefaultConfig(), "", searchProvider, llmProvider, ModelOptions{
		WorkingDir: projectDir,
	})
	m.width = 120
	m.height = 40
	m.applyLayout()

	driveQueryCycle(t, m, "explain @[app.go]", false)

	if len(searchProvider.calls) != 1 || searchProvider.calls[0].Query != "explain app.go" {
		t.Fatalf("expected attachment tokens to be stripped from search query, got %#v", searchProvider.calls)
	}
	if len(llmProvider.calls) != 1 {
		t.Fatalf("expected one LLM call, got %d", len(llmProvider.calls))
	}

	last := llmProvider.calls[0][len(llmProvider.calls[0])-1].Content
	if !strings.Contains(last, "Local file context:") || !strings.Contains(last, "[FILE 1] app.go") || !strings.Contains(last, "func handler() {}") {
		t.Fatalf("expected attached file contents in prompt, got %q", last)
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
	if m.contentH != 28 {
		t.Fatalf("expected capped active shell height, got %d", m.contentH)
	}
	if m.sourcesH != 6 {
		t.Fatalf("expected standardized sources panel height, got %d", m.sourcesH)
	}
	if strings.Contains(m.composeTranscript(), "## Sources") {
		t.Fatalf("expected trailing sources section to be stripped from transcript, got %q", m.composeTranscript())
	}
	if strings.Contains(m.composeTranscript(), "## Query:") || strings.Contains(m.composeTranscript(), "## Follow-up:") {
		t.Fatalf("expected transcript to omit query headings, got %q", m.composeTranscript())
	}
	if !strings.Contains(m.View(), "seek │ \"what is a transformer\"") {
		t.Fatalf("expected active session header to stay visible, got %q", m.View())
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

func TestActiveViewFitsNarrowWindowWithLongSources(t *testing.T) {
	searchProvider := &fakeSearchProvider{
		results: map[string][]searchpkg.SearchResult{
			"what is a transformer": {
				{
					Title:   "An extremely long source title that would previously blow past the right edge of the shell",
					URL:     "https://example.com/really/long/source/title",
					Content: "Transformers use self-attention.",
					Score:   0.9,
				},
			},
		},
	}
	llmProvider := &fakeLLMProvider{
		name: "fake/model/with/a/very/long/name",
		responses: map[string]string{
			"what is a transformer": "A transformer is a sequence model built around self-attention [1].",
		},
	}

	m := NewModel(DefaultConfig(), "", searchProvider, llmProvider)
	m.width = 42
	m.height = 16
	m.applyLayout()

	driveQueryCycle(t, m, "what is a transformer", false)
	assertViewFits(t, m.View(), m.width)
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

func assertViewFits(t *testing.T, view string, width int) {
	t.Helper()

	for _, line := range strings.Split(view, "\n") {
		if lipgloss.Width(line) > width {
			t.Fatalf("expected line width <= %d, got %d in %q", width, lipgloss.Width(line), line)
		}
	}
}
