package main

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
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
	if !strings.Contains(view, "mode=concise") {
		t.Fatalf("expected startup view to show current mode, got %q", view)
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
	if rows := m.suggestionVisibleRows(); rows != 3 {
		t.Fatalf("expected startup slash menu to show 3 commands, got %d", rows)
	}

	view := m.View()
	if !strings.Contains(view, "╚══════╝╚══════╝╚══════╝╚═╝ ╚═╝") {
		t.Fatalf("expected full seek logo to remain visible, got %q", view)
	}
	if !strings.Contains(view, "/backend") {
		t.Fatalf("expected slash command suggestions to remain visible, got %q", view)
	}
}

func TestActiveSlashLayoutShowsThreeCommands(t *testing.T) {
	m := NewModel(DefaultConfig(), "", &fakeSearchProvider{}, &fakeLLMProvider{name: "fake/model"})
	m.width = 140
	m.height = 36
	m.turns = []Turn{{Query: "what is tcp", Response: "TCP is a transport protocol."}}
	m.currentTurn = 0
	m.state = StateInput
	m.setFollowInputValue("/")
	m.applyLayout()

	if m.sourcesH != activeSuggestionPanelHeight {
		t.Fatalf("expected active suggestions height %d, got %d", activeSuggestionPanelHeight, m.sourcesH)
	}
	if rows := m.suggestionVisibleRows(); rows != 3 {
		t.Fatalf("expected active slash menu to show 3 commands, got %d", rows)
	}
}

func TestStartupViewShowsAvailableUpdateNotice(t *testing.T) {
	cfg := DefaultConfig()
	m := NewModel(cfg, "", &fakeSearchProvider{}, &fakeLLMProvider{name: "fake/model"})
	m.width = 140
	m.height = 36
	m.state = StateInput
	m.updateAvailable = "v1.2.4"
	m.applyLayout()

	view := m.View()
	if !strings.Contains(view, "Update available: v1.2.4") || !strings.Contains(view, "seek --update") {
		t.Fatalf("expected startup view to show update notice, got %q", view)
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
	if got := m.followInput.Value(); got != "/backend " {
		t.Fatalf("expected enter to accept selected slash suggestion, got %q", got)
	}
}

func TestModeSuggestionsAppearAfterExactSlashCommand(t *testing.T) {
	cfg := DefaultConfig()
	m := NewModel(cfg, "", &fakeSearchProvider{}, &fakeLLMProvider{name: "fake/model"})
	m.width = 140
	m.height = 36
	m.state = StateInput
	m.setFollowInputValue("/mode")
	m.applyLayout()

	if m.inputSuggestionMode != inputSuggestionSlashArg {
		t.Fatalf("expected slash argument suggestions, got mode %v", m.inputSuggestionMode)
	}
	if len(m.inputSuggestions) == 0 || m.inputSuggestions[0].Value != "concise" {
		t.Fatalf("expected mode suggestions, got %#v", m.inputSuggestions)
	}

	_ = m.handleInputKeys(tea.KeyMsg{Type: tea.KeyDown})
	_ = m.handleInputKeys(tea.KeyMsg{Type: tea.KeyEnter})
	if got := m.followInput.Value(); got != "/mode learning" {
		t.Fatalf("expected mode value suggestion to be inserted, got %q", got)
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

func TestNewSlashCommandStartsFreshChat(t *testing.T) {
	cfg := DefaultConfig()
	m := NewModelWithOptions(cfg, "", &fakeSearchProvider{}, &fakeLLMProvider{name: "fake/model"}, ModelOptions{
		PipedInput: &PipedInput{
			SearchQuery:  "Go cannot use string as int",
			ErrorContext: "./main.go:1:2: cannot use x",
			FullOutput:   "./main.go:1:2: cannot use x",
		},
	})
	m.width = 120
	m.height = 36
	m.turns = []Turn{{
		Query:       "old question",
		SearchQuery: "old question",
		Response:    "old answer",
		Sources:     []Source{{Title: "Old", URL: "https://example.com"}},
		PipedInput:  clonePipedInput(m.initialPipedInput),
	}}
	m.currentTurn = 0
	m.queryCount = 1
	m.output = "old answer"
	m.overlayContent = "old overlay"
	m.searchQuery = "old"
	m.searchMatches = []int{1}
	m.timingVisible = true
	m.initialPipedInput = clonePipedInput(m.turns[0].PipedInput)
	m.pipedInputExpanded = true
	m.state = StateInput
	m.applyLayout()

	_ = m.executeSlashCommand("/new")
	if len(m.turns) != 0 || m.currentTurn != -1 || m.queryCount != 0 {
		t.Fatalf("expected fresh chat state, got turns=%d current=%d queryCount=%d", len(m.turns), m.currentTurn, m.queryCount)
	}
	if m.initialPipedInput != nil || m.pipedInputExpanded {
		t.Fatalf("expected piped state to clear")
	}
	if m.overlayContent != "" || m.searchQuery != "" || len(m.searchMatches) != 0 || m.timingVisible {
		t.Fatalf("expected transient view state to clear")
	}
	if m.state != StateInput {
		t.Fatalf("expected input state, got %v", m.state)
	}
	if !strings.Contains(m.composeTranscript(), "AI-powered web search") {
		t.Fatalf("expected startup transcript after /new, got %q", m.composeTranscript())
	}
}

func TestToggleSlashCommandSwitchesThemeAndPersistsConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg := DefaultConfig()
	cfg.Theme = "dark"
	m := NewModel(cfg, "", &fakeSearchProvider{}, &fakeLLMProvider{name: "fake/model"})
	m.width = 120
	m.height = 36
	m.state = StateInput
	m.applyLayout()

	_ = m.executeSlashCommand("/toggle")
	if m.config.Theme != "light" {
		t.Fatalf("expected theme to toggle to light, got %q", m.config.Theme)
	}
	if m.styles.Name != "light" {
		t.Fatalf("expected active styles to switch to light, got %q", m.styles.Name)
	}

	configBody, err := os.ReadFile(ConfigPath())
	if err != nil {
		t.Fatalf("read config after toggle: %v", err)
	}
	if !strings.Contains(string(configBody), "theme = \"light\"") {
		t.Fatalf("expected persisted config to contain light theme, got %q", string(configBody))
	}

	_ = m.executeSlashCommand("/toggle")
	if m.config.Theme != "dark" {
		t.Fatalf("expected theme to toggle back to dark, got %q", m.config.Theme)
	}
	if m.styles.Name != "pastel" {
		t.Fatalf("expected dark toggle to resolve to pastel styles, got %q", m.styles.Name)
	}
}

func TestModeSlashCommandPersistsOutputFormat(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	m := NewModel(DefaultConfig(), "", &fakeSearchProvider{}, &fakeLLMProvider{name: "fake/model"})
	m.width = 120
	m.height = 36
	m.state = StateInput
	m.applyLayout()

	_ = m.executeSlashCommand("/mode explanatory")
	if m.config.OutputFormat != "explanatory" {
		t.Fatalf("expected explanatory mode, got %q", m.config.OutputFormat)
	}
	if !strings.Contains(m.flashText, "and saved") {
		t.Fatalf("expected saved flash, got %q", m.flashText)
	}

	configBody, err := os.ReadFile(ConfigPath())
	if err != nil {
		t.Fatalf("read config after mode change: %v", err)
	}
	if !strings.Contains(string(configBody), "output_format = \"explanatory\"") {
		t.Fatalf("expected persisted config to contain explanatory mode, got %q", string(configBody))
	}
}

func TestSearchWithinResponseUpdatesLiveAndMarksMatches(t *testing.T) {
	cfg := DefaultConfig()
	m := NewModel(cfg, "", &fakeSearchProvider{}, &fakeLLMProvider{name: "fake/model"})
	m.width = 120
	m.height = 36
	m.turns = []Turn{{
		Query:    "what is a transformer",
		Response: "A transformer uses attention.\n\nAttention heads track token relationships.",
	}}
	m.currentTurn = 0
	m.state = StateViewing
	m.applyLayout()
	m.refreshViewport(false)

	_ = m.handleViewingKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if m.state != StateSearchInput {
		t.Fatalf("expected / to open search input, got %v", m.state)
	}

	_ = m.handleSearchKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	_ = m.handleSearchKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	_ = m.handleSearchKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	_ = m.handleSearchKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	_ = m.handleSearchKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	_ = m.handleSearchKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	_ = m.handleSearchKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	_ = m.handleSearchKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	_ = m.handleSearchKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})

	if m.searchQuery != "attention" {
		t.Fatalf("expected live search query to update, got %q", m.searchQuery)
	}
	if len(m.searchMatches) == 0 {
		t.Fatalf("expected live search to find matches")
	}

	highlighted := m.applySearchHighlights(m.baseRendered)
	if !strings.Contains(highlighted, m.styles.SearchCurrent.Render("attention")) &&
		!strings.Contains(highlighted, m.styles.SearchCurrent.Render("Attention")) {
		t.Fatalf("expected inline highlight to appear, got %q", highlighted)
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
	if !strings.Contains(m.overlayContent, "tcp handshake") || !strings.Contains(m.overlayContent, "seek --resume <id>") {
		t.Fatalf("expected history overlay, got %q", m.overlayContent)
	}

	_ = m.executeSlashCommand("/recent")
	if !strings.Contains(m.overlayContent, "Recent Searches") {
		t.Fatalf("expected recent overlay, got %q", m.overlayContent)
	}
}

func TestResumeSlashCommandLoadsSavedThread(t *testing.T) {
	store, err := historypkg.NewHistoryStore(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatalf("NewHistoryStore: %v", err)
	}
	defer store.Close()

	rootID, err := store.Save(&historypkg.SearchRecord{
		Query:        "what is a transformer",
		Response:     "A transformer uses attention.",
		ProjectDir:   "/workspace/project",
		ProjectStack: "go/chi",
		LLMBackend:   "fake/model",
		OutputFormat: "concise",
		TotalMs:      100,
	})
	if err != nil {
		t.Fatalf("Save root: %v", err)
	}
	childID, err := store.Save(&historypkg.SearchRecord{
		Query:        "what about heads",
		Response:     "Heads attend to different relationships.",
		ProjectDir:   "/workspace/project",
		ProjectStack: "go/chi",
		LLMBackend:   "fake/model",
		OutputFormat: "concise",
		IsFollowUp:   true,
		ParentID:     &rootID,
		TotalMs:      120,
	})
	if err != nil {
		t.Fatalf("Save child: %v", err)
	}

	m := NewModelWithOptions(DefaultConfig(), "", &fakeSearchProvider{}, &fakeLLMProvider{name: "fake/model"}, ModelOptions{
		HistoryStore: store,
	})
	m.width = 120
	m.height = 40
	m.state = StateInput
	m.setFollowInputValue("/resume")
	m.applyLayout()

	_ = m.executeSlashCommand("/resume " + strconv.FormatInt(childID, 10))
	if len(m.turns) != 2 || m.currentTurn != 1 || m.queryCount != 2 {
		t.Fatalf("expected resumed two-turn thread, got turns=%d current=%d queryCount=%d", len(m.turns), m.currentTurn, m.queryCount)
	}
	if m.turns[0].Query != "what is a transformer" || m.turns[1].Query != "what about heads" {
		t.Fatalf("unexpected resumed turns: %#v", m.turns)
	}
	if m.turns[1].HistoryID == nil || *m.turns[1].HistoryID != childID {
		t.Fatalf("expected terminal history id %d, got %#v", childID, m.turns[1].HistoryID)
	}
	if m.state != StateViewing {
		t.Fatalf("expected resumed session to enter viewing state, got %v", m.state)
	}
	transcript := m.composeTranscript()
	if !strings.Contains(transcript, "**Question: what is a transformer**") || strings.Contains(transcript, "**Follow-up: what about heads**") {
		t.Fatalf("expected prior question only in resumed transcript, got %q", transcript)
	}
	if !strings.Contains(m.flashText, "Resumed 2 saved turns") {
		t.Fatalf("expected resume flash, got %q", m.flashText)
	}
}

func TestResumeSlashCommandWithoutIDShowsPicker(t *testing.T) {
	store, err := historypkg.NewHistoryStore(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatalf("NewHistoryStore: %v", err)
	}
	defer store.Close()

	firstID, err := store.Save(&historypkg.SearchRecord{
		Query:      "first chat",
		Response:   "first answer",
		ProjectDir: "/workspace/project",
	})
	if err != nil {
		t.Fatalf("Save first: %v", err)
	}
	secondID, err := store.Save(&historypkg.SearchRecord{
		Query:      "second chat",
		Response:   "second answer",
		ProjectDir: "/workspace/project",
	})
	if err != nil {
		t.Fatalf("Save second: %v", err)
	}

	m := NewModelWithOptions(DefaultConfig(), "", &fakeSearchProvider{}, &fakeLLMProvider{name: "fake/model"}, ModelOptions{
		HistoryStore: store,
	})
	m.width = 120
	m.height = 40
	m.state = StateInput
	m.applyLayout()

	_ = m.executeSlashCommand("/resume")
	if m.state != StateResumePicker {
		t.Fatalf("expected resume picker state, got %v", m.state)
	}
	if len(m.resumeCandidates) != 2 || m.resumeCandidates[0].ID != secondID || m.resumeCandidates[1].ID != firstID {
		t.Fatalf("expected newest resume candidates first, got %#v", m.resumeCandidates)
	}
	view := stripANSI(m.summaryView())
	if !strings.Contains(view, "Resume Saved Chats") || !strings.Contains(view, "second chat") {
		t.Fatalf("expected resume picker view, got %q", view)
	}

	cmd := m.handleResumePickerKeys(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected resume flash command")
	}
	if len(m.turns) != 1 || m.turns[0].HistoryID == nil || *m.turns[0].HistoryID != secondID {
		t.Fatalf("expected selected chat to load, got %#v", m.turns)
	}
}

func TestShowSlashCommandRendersSessionOverlay(t *testing.T) {
	m := NewModelWithOptions(DefaultConfig(), "", &fakeSearchProvider{}, &fakeLLMProvider{name: "fake/model"}, ModelOptions{
		ProjectContext: &projectctx.ProjectContext{Language: "go", Framework: "chi"},
	})
	m.width = 120
	m.height = 40
	m.state = StateInput
	m.applyLayout()

	_ = m.executeSlashCommand("/show")
	if !strings.Contains(m.overlayContent, "## Session") {
		t.Fatalf("expected session overlay, got %q", m.overlayContent)
	}
	if !strings.Contains(m.overlayContent, "Mode: concise") || !strings.Contains(m.overlayContent, "Project context: go/chi") {
		t.Fatalf("expected session details in overlay, got %q", m.overlayContent)
	}
}

func TestClearHistorySlashCommandDeletesSavedEntriesAndResetsSessionLinks(t *testing.T) {
	store, err := historypkg.NewHistoryStore(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatalf("NewHistoryStore: %v", err)
	}
	defer store.Close()

	id, err := store.Save(&historypkg.SearchRecord{
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
	m.turns = []Turn{{Query: "tcp handshake", Response: "response", HistoryID: &id}}
	m.currentTurn = 0
	m.width = 120
	m.height = 40
	m.state = StateInput
	m.applyLayout()

	_ = m.executeSlashCommand("/clear-history")
	if !strings.Contains(m.overlayContent, "History Cleared") {
		t.Fatalf("expected clear history overlay, got %q", m.overlayContent)
	}
	if m.turns[0].HistoryID != nil {
		t.Fatalf("expected turn history IDs to be reset after clear")
	}

	records, err := store.Recent(10, "")
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected cleared history, got %#v", records)
	}
}

func TestTimingClearsFromStatusMetaAfterTick(t *testing.T) {
	m := NewModelWithOptions(DefaultConfig(), "", &fakeSearchProvider{}, &fakeLLMProvider{name: "fake/model"}, ModelOptions{
		ProjectContext: &projectctx.ProjectContext{Language: "go", Framework: "chi"},
	})
	m.lastTiming = SearchTiming{SearchMs: 312, LLMMs: 535, TotalMs: 847}
	m.timingVisible = true
	m.timingSeq = 1

	if got := m.statusMeta(); !strings.Contains(got, "847ms") || !strings.Contains(got, "go/chi") || !strings.Contains(got, "mode=concise") {
		t.Fatalf("expected timing-rich status meta, got %q", got)
	}

	if _, cmd := m.Update(timingClearMsg{Seq: 1}); cmd != nil {
		t.Fatalf("expected no follow-up cmd from timing clear")
	}
	if got := m.statusMeta(); strings.Contains(got, "847ms") || !strings.Contains(got, "fake/model") || !strings.Contains(got, "depth=basic") {
		t.Fatalf("expected timing to clear from status meta, got %q", got)
	}
}

func TestReleaseCheckNoticeAddsStatusMeta(t *testing.T) {
	m := NewModelWithOptions(DefaultConfig(), "", &fakeSearchProvider{}, &fakeLLMProvider{name: "fake/model"}, ModelOptions{
		ProjectContext: &projectctx.ProjectContext{Language: "go", Framework: "chi"},
	})

	updated, cmd := m.Update(releaseCheckMsg{Latest: "v1.2.2"})
	if cmd == nil {
		t.Fatalf("expected flash clear command after release notice")
	}
	gotModel, ok := updated.(*model)
	if !ok {
		t.Fatalf("expected model after release notice, got %T", updated)
	}
	if gotModel.updateAvailable != "v1.2.2" {
		t.Fatalf("expected updateAvailable to be set, got %q", gotModel.updateAvailable)
	}
	if !strings.Contains(gotModel.flashText, "seek --update") {
		t.Fatalf("expected update flash text, got %q", gotModel.flashText)
	}
	if got := gotModel.statusMeta(); !strings.Contains(got, "update=v1.2.2") {
		t.Fatalf("expected status meta to include update, got %q", got)
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

func TestPipedQueryLifecycleShowsInputAndCarriesContext(t *testing.T) {
	input := preparePipedInput("./main.go:42:15: cannot use x (variable of type string) as int value\n", "")
	searchProvider := &fakeSearchProvider{
		results: map[string][]searchpkg.SearchResult{
			input.SearchQuery: {
				{Title: "Go types", URL: "https://example.com/go-types", Content: "Go checks types.", Score: 1},
			},
		},
	}
	llmProvider := &pipeLLMProvider{name: "fake/pipe"}

	m := NewModelWithOptions(DefaultConfig(), "", searchProvider, llmProvider, ModelOptions{
		PipedInput: &input,
	})
	m.width = 120
	m.height = 40
	m.applyLayout()

	drivePipedQueryCycle(t, m, input)

	if len(searchProvider.calls) != 1 || searchProvider.calls[0].Query != input.SearchQuery {
		t.Fatalf("expected extracted search query, got %#v", searchProvider.calls)
	}
	if !strings.Contains(m.composeTranscript(), "piped input") || !strings.Contains(m.composeTranscript(), "press `e` to expand") {
		t.Fatalf("expected collapsed piped input block, got %q", m.composeTranscript())
	}
	if !strings.Contains(m.statusMeta(), "piped") {
		t.Fatalf("expected piped indicator in status, got %q", m.statusMeta())
	}

	_ = m.handleViewingKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if !m.pipedInputExpanded || !strings.Contains(m.composeTranscript(), "./main.go:42:15") {
		t.Fatalf("expected expanded piped input in transcript, got %q", m.composeTranscript())
	}

	m.turns = append(m.turns, Turn{Query: "how do I fix it"})
	m.currentTurn = 1
	history := m.conversationHistory()
	if len(history) == 0 || !strings.Contains(history[0].Content, "Command output:") || !strings.Contains(history[0].Content, "./main.go:42:15") {
		t.Fatalf("expected piped command output in follow-up history, got %#v", history)
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
	if strings.Contains(m.composeTranscript(), "**Question: what is a transformer**") {
		t.Fatalf("expected current question to stay out of the transcript until a follow-up exists, got %q", m.composeTranscript())
	}
	view := stripANSI(m.View())
	if !strings.Contains(view, "seek") || !strings.Contains(view, "\"what is a transformer\"") {
		t.Fatalf("expected active session header to stay visible, got %q", view)
	}

	driveQueryCycle(t, m, "what about attention heads", true)
	if !strings.Contains(m.composeTranscript(), "**Question: what is a transformer**") {
		t.Fatalf("expected prior question above its answer after follow-up, got %q", m.composeTranscript())
	}
	if strings.Contains(m.composeTranscript(), "**Follow-up: what about attention heads**") {
		t.Fatalf("expected current follow-up to stay out of the transcript until another follow-up exists, got %q", m.composeTranscript())
	}

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

func drivePipedQueryCycle(t *testing.T, m *model, input PipedInput) {
	t.Helper()

	m.beginPipedQuery(input)
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
