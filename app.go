package main

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	projectctx "seek/context"
	historypkg "seek/history"
	llmpkg "seek/llm"
	searchpkg "seek/search"
	"seek/ui"
)

var (
	ansiRE       = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	codeFenceRE  = regexp.MustCompile("(?ms)```([^\\n`]*)\\n(.*?)```")
	sourceTailRE = regexp.MustCompile(`(?is)\n{2,}(?:#{1,6}\s*|[*_]{0,2})?(sources?|references?|citations?|further reading)(?:[*_]{0,2})\s*:?\s*\n`)
)

const (
	startupShellMaxHeight            = 16
	startupInteractiveShellMaxHeight = 20
	activeShellMaxHeight             = 28
	startupSlashSuggestionsHeight    = 4
	inputComposerHeight              = 5
	sourcesPanelHeight               = 6
)

type (
	slashCommandSpec struct {
		Name        string
		Usage       string
		Description string
	}
	renderedCodeBlock struct {
		Placeholder string
		Rendered    string
	}
	searchCompleteMsg struct {
		RequestID int
		Results   []searchpkg.SearchResult
		Err       error
	}
	tokenMsg struct {
		RequestID int
		Text      string
	}
	streamDoneMsg struct {
		RequestID int
	}
	streamErrMsg struct {
		RequestID int
		Err       error
	}
	startQueryMsg struct {
		Query    string
		FollowUp bool
	}
	clipboardResultMsg struct {
		Label string
		Err   error
		Lines int
	}
	browserResultMsg struct {
		Err error
	}
	historySavedMsg struct {
		TurnIndex int
		ID        int64
		Err       error
	}
	releaseCheckMsg struct {
		Latest string
		Err    error
	}
	timingClearMsg struct {
		Seq int
	}
	flashClearMsg struct{}
)

type ModelOptions struct {
	ProjectContext *projectctx.ProjectContext
	WorkingDir     string
	HistoryStore   *historypkg.HistoryStore
	OpenRecord     *historypkg.SearchRecord
}

type model struct {
	state        AppState
	startupQuery string

	viewport    viewport.Model
	sourcesList ui.SourcesModel
	followInput textinput.Model
	searchInput textinput.Model
	spinner     spinner.Model
	statusBar   ui.StatusBarModel
	styles      ui.Styles
	renderer    *glamour.TermRenderer

	searchProvider searchpkg.SearchProvider
	llmProvider    llmpkg.LLMProvider
	orchestrator   *Orchestrator
	historyStore   *historypkg.HistoryStore

	turns       []Turn
	currentTurn int
	queryCount  int

	config       Config
	width        int
	height       int
	contentW     int
	contentH     int
	summaryH     int
	summaryW     int
	sourcesH     int
	tokenCount   int
	baseRendered string
	output       string

	searching         bool
	streaming         bool
	waitingFirstToken bool
	autoScroll        bool
	newContent        bool
	printOnExit       bool

	searchQuery   string
	searchMatches []int
	searchIndex   int

	codeBlocks []CodeBlock
	codeSelect int

	flashText string
	flashKind string

	workingDir              string
	projectContext          *projectctx.ProjectContext
	detectedProjectContext  *projectctx.ProjectContext
	inputSuggestionMode     inputSuggestionMode
	inputSuggestions        []inputSuggestion
	inputSuggestionIndex    int
	inputSuggestionOffset   int
	inputSuggestionKey      string
	inputSuggestionsFocused bool
	localFiles              []string
	localFilesLoaded        bool
	localFilesErr           error
	overlayContent          string
	searchStartTime         time.Time
	llmStartTime            time.Time
	lastSearchMs            int64
	lastLLMMs               int64
	lastTotalMs             int64
	lastTiming              SearchTiming
	timingVisible           bool
	timingSeq               int
	updateAvailable         string

	requestID  int
	requestCtx context.Context
	streamSub  chan tea.Msg
	cancel     context.CancelFunc
}

func NewModel(cfg Config, initialQuery string, searchProvider searchpkg.SearchProvider, llmProvider llmpkg.LLMProvider) *model {
	return NewModelWithOptions(cfg, initialQuery, searchProvider, llmProvider, ModelOptions{})
}

func NewModelWithOptions(cfg Config, initialQuery string, searchProvider searchpkg.SearchProvider, llmProvider llmpkg.LLMProvider, options ModelOptions) *model {
	styles := ui.LoadTheme(resolveThemePreference(cfg.Theme))

	vp := viewport.New(0, 0)
	vp.Style = lipgloss.NewStyle()
	vp.HighPerformanceRendering = false

	spin := spinner.New()
	spin.Spinner = spinner.Dot
	spin.Style = styles.Spinner

	follow := ui.NewFollowUpInput(styles)
	follow.Placeholder = "start typing... use / for commands or @[file]"
	searchInput := ui.NewSearchInput(styles)
	searchInput.Placeholder = "search within the summary"

	projectContext := cloneProjectContext(options.ProjectContext)
	detectedContext := cloneProjectContext(options.ProjectContext)

	m := &model{
		state:                  StateViewing,
		startupQuery:           strings.TrimSpace(initialQuery),
		viewport:               vp,
		sourcesList:            ui.NewSources(styles),
		followInput:            follow,
		searchInput:            searchInput,
		spinner:                spin,
		statusBar:              ui.NewStatusBar(styles),
		styles:                 styles,
		searchProvider:         searchProvider,
		llmProvider:            llmProvider,
		orchestrator:           NewOrchestrator(searchProvider, llmProvider, cfg.MaxResults, cfg.OutputFormat, projectContext),
		historyStore:           options.HistoryStore,
		config:                 cfg,
		autoScroll:             true,
		printOnExit:            cfg.PrintOnExit,
		currentTurn:            -1,
		workingDir:             strings.TrimSpace(options.WorkingDir),
		projectContext:         projectContext,
		detectedProjectContext: detectedContext,
	}

	if options.OpenRecord != nil {
		m.loadHistoryRecord(options.OpenRecord)
	}
	m.refreshInputSuggestions()

	return m
}

func (m *model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.spinner.Tick, checkForUpdateCmd(version)}

	if m.startupQuery != "" {
		m.state = StateLoading
		cmds = append(cmds, emitQuery(m.startupQuery, false))
	} else if m.currentTurn >= 0 {
		m.state = StateViewing
		m.syncSources()
	} else {
		m.state = StateInput
		cmds = append(cmds, m.followInput.Focus())
	}

	return tea.Batch(cmds...)
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.applyLayout()
		m.refreshViewport(false)
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if m.searching || m.waitingFirstToken || m.streaming {
			return m, cmd
		}
		return m, nil

	case startQueryMsg:
		return m, m.beginQuery(msg.Query, msg.FollowUp)

	case searchCompleteMsg:
		if msg.RequestID != m.requestID {
			return m, nil
		}
		m.searching = false
		if !m.searchStartTime.IsZero() {
			m.lastSearchMs = time.Since(m.searchStartTime).Milliseconds()
		}
		if msg.Err != nil {
			m.finishRequest()
			if m.currentTurn >= 0 {
				m.turns[m.currentTurn].Error = friendlyError(msg.Err, m.config)
				m.turns[m.currentTurn].Sources = nil
			}
			m.state = StateViewing
			m.syncSources()
			m.applyLayout()
			m.refreshViewport(false)
			return m, nil
		}

		results := sanitizeSearchResults(msg.Results)
		if m.currentTurn >= 0 {
			m.turns[m.currentTurn].Sources = sourcesFromSearchResults(results)
			m.turns[m.currentTurn].Error = ""
		}
		m.llmStartTime = time.Now()
		m.waitingFirstToken = true
		m.syncSources()
		m.applyLayout()
		m.refreshViewport(false)
		return m, tea.Batch(m.startLLMStream(results), m.spinner.Tick)

	case tokenMsg:
		if msg.RequestID != m.requestID || m.currentTurn < 0 {
			return m, nil
		}
		token := sanitizeTerminalText(msg.Text)
		if token == "" {
			if m.streamSub != nil {
				return m, waitForMsg(m.streamSub)
			}
			return m, nil
		}
		if m.waitingFirstToken {
			m.waitingFirstToken = false
			if m.state == StateLoading {
				m.state = StateViewing
				m.applyLayout()
			}
		}
		m.streaming = true
		m.turns[m.currentTurn].Response += token
		m.turns[m.currentTurn].Error = ""
		m.tokenCount += len(strings.Fields(token))
		m.refreshViewport(false)
		if m.streamSub != nil {
			return m, waitForMsg(m.streamSub)
		}
		return m, nil

	case streamDoneMsg:
		if msg.RequestID != m.requestID {
			return m, nil
		}
		if !m.llmStartTime.IsZero() {
			m.lastLLMMs = time.Since(m.llmStartTime).Milliseconds()
		}
		m.lastTotalMs = m.lastSearchMs + m.lastLLMMs
		m.lastTiming = SearchTiming{
			SearchMs: m.lastSearchMs,
			LLMMs:    m.lastLLMMs,
			TotalMs:  m.lastTotalMs,
		}
		m.timingVisible = true
		m.timingSeq++
		turnIndex := m.currentTurn
		m.finishRequest()
		if m.state == StateLoading {
			m.state = StateViewing
			m.applyLayout()
		}
		m.refreshViewport(m.autoScroll)
		cmds := []tea.Cmd{clearTimingCmd(m.timingSeq)}
		if cmd := m.saveTurnToHistoryCmd(turnIndex); cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)

	case streamErrMsg:
		if msg.RequestID != m.requestID {
			return m, nil
		}
		m.finishRequest()
		if m.currentTurn >= 0 {
			m.turns[m.currentTurn].Error = friendlyError(msg.Err, m.config)
		}
		m.state = StateViewing
		m.syncSources()
		m.applyLayout()
		m.refreshViewport(false)
		return m, nil

	case historySavedMsg:
		if msg.Err != nil {
			return m, nil
		}
		if msg.TurnIndex >= 0 && msg.TurnIndex < len(m.turns) {
			id := msg.ID
			m.turns[msg.TurnIndex].HistoryID = &id
		}
		return m, nil

	case releaseCheckMsg:
		if strings.TrimSpace(msg.Latest) == "" || msg.Err != nil {
			return m, nil
		}
		m.updateAvailable = strings.TrimSpace(msg.Latest)
		return m, m.flashFor("Update available: "+m.updateAvailable+" · run `seek --update`", "warning", 8*time.Second)

	case timingClearMsg:
		if msg.Seq == m.timingSeq {
			m.timingVisible = false
		}
		return m, nil

	case clipboardResultMsg:
		if msg.Err != nil {
			return m, m.flash("clipboard failed: "+msg.Err.Error(), "error")
		}
		label := msg.Label
		if msg.Lines > 0 {
			label = fmt.Sprintf("%s (%d lines)", msg.Label, msg.Lines)
		}
		return m, m.flash("Copied "+label, "success")

	case browserResultMsg:
		if msg.Err != nil {
			return m, m.flash("open failed: "+msg.Err.Error(), "error")
		}
		return m, m.flash("Opened source in browser", "success")

	case flashClearMsg:
		m.flashText = ""
		m.flashKind = ""
		return m, nil

	case tea.KeyMsg:
		return m, m.handleKey(msg)
	}

	if m.state == StateInput {
		var cmd tea.Cmd
		m.followInput, cmd = m.followInput.Update(msg)
		return m, cmd
	}

	if m.state == StateSearchInput {
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}

	parts := make([]string, 0, 4)
	if !m.isStartupState() {
		parts = append(parts, m.headerView())
	}
	parts = append(parts, m.summaryView())
	if m.sourcesH > 0 && !m.isPlainStartupState() {
		parts = append(parts, m.sourcesSectionView())
	}
	parts = append(parts, m.footerView())
	frame := m.styles.AppFrame.Render(strings.Join(parts, "\n"))
	return lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Bottom, frame)
}

func (m *model) FinalOutput() string {
	return strings.TrimSpace(m.output)
}

func (m *model) ShouldPrintOnExit() bool {
	return m.printOnExit
}

func (m *model) handleKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "ctrl+c":
		m.stopActiveRequest()
		return tea.Quit
	case "ctrl+l":
		return clearScreenCmd()
	}

	switch m.state {
	case StateInput:
		return m.handleInputKeys(msg)
	case StateSearchInput:
		return m.handleSearchKeys(msg)
	case StateCodeSelect:
		return m.handleCodeSelectKeys(msg)
	}

	switch msg.String() {
	case "esc", "q":
		m.stopActiveRequest()
		return tea.Quit
	}

	if m.state == StateLoading {
		return nil
	}

	if m.state == StateSources {
		return m.handleSourcesKeys(msg)
	}
	return m.handleViewingKeys(msg)
}

func (m *model) handleInputKeys(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.followInput.Blur()
		m.setFollowInputValue("")
		m.inputSuggestionsFocused = false
		if len(m.turns) == 0 {
			m.state = StateInput
			m.applyLayout()
			m.refreshViewport(false)
			return m.followInput.Focus()
		}
		m.state = StateViewing
		return m.syncLayout(false)
	case "down", "ctrl+n":
		if m.shouldRenderInputSuggestions() {
			m.moveInputSuggestion(1)
			return nil
		}
		return nil
	case "up", "ctrl+p":
		if m.shouldRenderInputSuggestions() {
			m.moveInputSuggestion(-1)
			return nil
		}
		return nil
	case "j":
		if m.inputSuggestionsFocused {
			m.moveInputSuggestion(1)
			return nil
		}
	case "k":
		if m.inputSuggestionsFocused {
			m.moveInputSuggestion(-1)
			return nil
		}
	case "tab":
		if m.acceptSelectedInputSuggestion() {
			return m.followInput.Focus()
		}
		return nil
	case "enter":
		if m.shouldAcceptSuggestionOnEnter() && m.acceptSelectedInputSuggestion() {
			return m.followInput.Focus()
		}
		query := strings.TrimSpace(m.followInput.Value())
		if query == "" {
			return m.flash("Follow-up query is empty", "warning")
		}
		if strings.HasPrefix(query, "/") {
			return m.executeSlashCommand(query)
		}
		if _, _, err := prepareAttachedFiles(query, m.workingDir); err != nil {
			return m.flash("Attachment failed: "+err.Error(), "error")
		}
		m.followInput.Blur()
		m.setFollowInputValue("")
		m.inputSuggestionsFocused = false
		return emitQuery(query, m.queryCount > 0)
	}

	if m.inputSuggestionsFocused && (len(msg.Runes) > 0 || isEditingKey(msg.String())) {
		m.inputSuggestionsFocused = false
	}
	var cmd tea.Cmd
	m.followInput, cmd = m.followInput.Update(msg)
	m.refreshInputSuggestions()
	if len(m.turns) == 0 {
		m.applyLayout()
		m.refreshViewport(false)
	}
	return cmd
}

func (m *model) handleSearchKeys(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.searchInput.Blur()
		m.state = StateViewing
		return m.syncLayout(false)
	case "enter":
		m.searchQuery = strings.TrimSpace(m.searchInput.Value())
		m.searchInput.Blur()
		m.state = StateViewing
		m.applyLayout()
		m.refreshViewport(false)
		if len(m.searchMatches) == 0 && m.searchQuery != "" {
			return m.flash("No matches found", "warning")
		}
		m.jumpToSearchMatch()
		return nil
	default:
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		m.searchQuery = strings.TrimSpace(m.searchInput.Value())
		m.refreshViewport(false)
		return cmd
	}
}

func (m *model) handleCodeSelectKeys(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "q":
		m.stopActiveRequest()
		return tea.Quit
	case "esc":
		m.state = StateViewing
		return m.syncLayout(false)
	case "j", "down":
		if m.codeSelect < len(m.codeBlocks)-1 {
			m.codeSelect++
		}
		return nil
	case "k", "up":
		if m.codeSelect > 0 {
			m.codeSelect--
		}
		return nil
	case "enter":
		return m.yankSelectedCodeBlock()
	}

	if len(msg.Runes) == 1 && msg.Runes[0] >= '1' && msg.Runes[0] <= '9' {
		idx := int(msg.Runes[0] - '1')
		if idx >= 0 && idx < len(m.codeBlocks) {
			m.codeSelect = idx
			return m.yankSelectedCodeBlock()
		}
	}

	return nil
}

func (m *model) handleSourcesKeys(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "tab":
		m.state = StateViewing
		m.sourcesList.Blur()
		return m.syncLayout(false)
	case "j", "down":
		m.sourcesList.Next()
	case "k", "up":
		m.sourcesList.Prev()
	case "o", "enter":
		item, ok := m.sourcesList.Selected()
		if !ok {
			return nil
		}
		return openSourceCmd(item.URL, m.config.Browser)
	case "y":
		item, ok := m.sourcesList.Selected()
		if !ok {
			return nil
		}
		return copyTextCmd(item.URL, "source URL", 0)
	}
	return nil
}

func (m *model) handleViewingKeys(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "j", "down":
		m.viewport.ScrollDown(1)
		m.afterManualScroll()
	case "k", "up":
		m.viewport.ScrollUp(1)
		m.afterManualScroll()
	case "d", "ctrl+d":
		m.viewport.HalfPageDown()
		m.afterManualScroll()
	case "u", "ctrl+u":
		m.viewport.HalfPageUp()
		m.afterManualScroll()
	case "g":
		m.viewport.GotoTop()
		m.afterManualScroll()
	case "G":
		m.viewport.GotoBottom()
		m.autoScroll = true
		m.newContent = false
	case "tab":
		m.state = StateSources
		m.sourcesList.Focus()
		return m.syncLayout(false)
	case "f":
		m.state = StateInput
		m.setFollowInputValue("")
		m.applyLayout()
		m.refreshViewport(false)
		return m.followInput.Focus()
	case "y":
		if strings.TrimSpace(m.output) == "" {
			return m.flash("Nothing to copy yet", "warning")
		}
		return copyTextCmd(m.output, "summary", lineCount(m.output))
	case "Y":
		return m.handleCodeYank()
	case "/":
		m.state = StateSearchInput
		m.searchInput.SetValue(m.searchQuery)
		m.applyLayout()
		m.refreshViewport(false)
		return m.searchInput.Focus()
	case "n":
		if len(m.searchMatches) > 0 {
			m.searchIndex = (m.searchIndex + 1) % len(m.searchMatches)
			m.refreshViewport(false)
			m.jumpToSearchMatch()
		}
	case "N":
		if len(m.searchMatches) > 0 {
			m.searchIndex--
			if m.searchIndex < 0 {
				m.searchIndex = len(m.searchMatches) - 1
			}
			m.refreshViewport(false)
			m.jumpToSearchMatch()
		}
	case "p":
		m.printOnExit = !m.printOnExit
		if m.printOnExit {
			return m.flash("Print summary on exit enabled", "success")
		}
		return m.flash("Print summary on exit disabled", "warning")
	case "r":
		if m.currentTurn >= 0 && m.turns[m.currentTurn].Error != "" {
			return m.retryCurrentTurn()
		}
	}
	return nil
}

func (m *model) beginQuery(query string, followUp bool) tea.Cmd {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}

	searchQuery, attachedFiles, err := prepareAttachedFiles(query, m.workingDir)
	if err != nil {
		m.state = StateInput
		m.applyLayout()
		m.refreshViewport(false)
		return tea.Batch(m.followInput.Focus(), m.flash("Attachment failed: "+err.Error(), "error"))
	}

	m.stopActiveRequest()
	m.requestID++
	m.requestCtx, m.cancel = context.WithCancel(context.Background())

	m.turns = append(m.turns, Turn{
		Query:         query,
		SearchQuery:   effectiveSearchQuery(query, searchQuery, attachedFiles),
		AttachedFiles: attachedFiles,
		IsFollowUp:    followUp,
	})
	m.currentTurn = len(m.turns) - 1
	m.queryCount = len(m.turns)
	m.tokenCount = 0
	m.searching = true
	m.streaming = false
	m.waitingFirstToken = false
	m.autoScroll = true
	m.newContent = false
	m.overlayContent = ""
	m.timingVisible = false
	m.state = StateLoading
	m.codeBlocks = nil
	m.codeSelect = 0
	m.syncSources()
	m.applyLayout()
	m.refreshViewport(true)
	return tea.Batch(m.startSearch(), m.spinner.Tick)
}

func (m *model) retryCurrentTurn() tea.Cmd {
	if m.currentTurn < 0 {
		return nil
	}

	searchQuery, attachedFiles, err := prepareAttachedFiles(m.turns[m.currentTurn].Query, m.workingDir)
	if err != nil {
		return m.flash("Attachment failed: "+err.Error(), "error")
	}

	m.stopActiveRequest()
	m.requestID++
	m.requestCtx, m.cancel = context.WithCancel(context.Background())

	m.turns[m.currentTurn].SearchQuery = effectiveSearchQuery(m.turns[m.currentTurn].Query, searchQuery, attachedFiles)
	m.turns[m.currentTurn].AttachedFiles = attachedFiles
	m.turns[m.currentTurn].Response = ""
	m.turns[m.currentTurn].Error = ""
	m.turns[m.currentTurn].Sources = nil
	m.tokenCount = 0
	m.searching = true
	m.streaming = false
	m.waitingFirstToken = false
	m.autoScroll = true
	m.newContent = false
	m.overlayContent = ""
	m.timingVisible = false
	m.state = StateLoading
	m.syncSources()
	m.applyLayout()
	m.refreshViewport(true)
	return tea.Batch(m.startSearch(), m.spinner.Tick)
}

func (m *model) startSearch() tea.Cmd {
	if m.currentTurn < 0 || m.requestCtx == nil || m.orchestrator == nil {
		return nil
	}
	m.searchStartTime = time.Now()
	m.llmStartTime = time.Time{}
	m.lastSearchMs = 0
	m.lastLLMMs = 0
	m.lastTotalMs = 0
	query := strings.TrimSpace(m.turns[m.currentTurn].SearchQuery)
	if query == "" {
		query = strings.TrimSpace(m.turns[m.currentTurn].Query)
	}
	return searchCmd(m.requestCtx, m.orchestrator, query, m.requestID)
}

func (m *model) startLLMStream(results []searchpkg.SearchResult) tea.Cmd {
	if m.currentTurn < 0 || m.requestCtx == nil {
		return nil
	}

	sub := make(chan tea.Msg, 64)
	m.streamSub = sub

	ctx := m.requestCtx
	query := m.turns[m.currentTurn].Query
	history := m.conversationHistory()
	attachedFiles := append([]AttachedFile(nil), m.turns[m.currentTurn].AttachedFiles...)
	requestID := m.requestID
	orchestrator := m.orchestrator

	go func() {
		defer close(sub)

		sawTokens := false
		response, err := orchestrator.StreamAnswer(ctx, query, results, history, attachedFiles, func(token string) {
			sawTokens = true
			sendStreamMsg(ctx, sub, tokenMsg{RequestID: requestID, Text: token})
		})
		if err != nil {
			sendStreamMsg(ctx, sub, streamErrMsg{RequestID: requestID, Err: err})
			return
		}
		if !sawTokens && strings.TrimSpace(response) != "" {
			sendStreamMsg(ctx, sub, tokenMsg{RequestID: requestID, Text: response})
		}
		sendStreamMsg(ctx, sub, streamDoneMsg{RequestID: requestID})
	}()

	return waitForMsg(sub)
}

func (m *model) conversationHistory() []llmpkg.Message {
	history := make([]llmpkg.Message, 0)
	if m.currentTurn < 0 {
		return history
	}

	start := 0
	if m.currentTurn > m.config.MaxTurns {
		start = m.currentTurn - m.config.MaxTurns
	}

	for idx := start; idx < m.currentTurn; idx++ {
		turn := m.turns[idx]
		history = append(history, llmpkg.Message{Role: "user", Content: turn.Query})
		if response := strings.TrimSpace(sanitizeAssistantResponse(turn.Response)); response != "" {
			history = append(history, llmpkg.Message{Role: "assistant", Content: response})
		}
	}
	return history
}

func (m *model) applyLayout() {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	if m.state == StateInput {
		m.refreshInputSuggestions()
	}

	m.contentW = max(1, m.width-m.styles.AppFrame.GetHorizontalFrameSize())
	maxContentH := max(1, m.height-m.styles.AppFrame.GetVerticalFrameSize())
	if len(m.turns) == 0 {
		shellMaxHeight := startupShellMaxHeight
		if m.shouldRenderInputSuggestions() {
			shellMaxHeight = startupInteractiveShellMaxHeight
		}
		m.contentH = min(maxContentH, shellMaxHeight)
	} else {
		m.contentH = min(maxContentH, activeShellMaxHeight)
	}
	m.summaryW = m.contentW

	headerH := 1
	if len(m.turns) == 0 {
		headerH = 0
	}
	footerH := 1
	if m.state == StateInput || m.state == StateSearchInput {
		footerH = 1 + m.styles.InputBar.GetVerticalFrameSize()
	}

	sourcesH := 0
	switch {
	case m.shouldRenderInputSuggestions():
		sourcesH = m.currentSuggestionPanelHeight()
	case len(m.turns) > 0 && m.state == StateInput && !m.isSlashInput():
		sourcesH = inputComposerHeight
	case len(m.turns) > 0:
		sourcesH = sourcesPanelHeight
	}

	available := m.contentH - headerH - footerH
	if available < 1 {
		headerH = 0
		available = m.contentH - footerH
	}
	if available < 1 {
		sourcesH = 0
		available = max(1, m.contentH-footerH)
	}
	minSummary := 6
	if len(m.turns) == 0 {
		minSummary = 3
	}

	startupPreferredSummary := ui.PreferredSplashHeight(max(1, m.contentW-2)) + 2
	if len(m.turns) == 0 && sourcesH > 0 && available-sourcesH < startupPreferredSummary {
		sourcesH = max(0, available-startupPreferredSummary)
	}
	if len(m.turns) > 0 && sourcesH > 0 && available-sourcesH < minSummary {
		sourcesH = max(0, available-minSummary)
	}
	if sourcesH > 0 && sourcesH < 3 {
		sourcesH = 0
	}
	if sourcesH > 0 {
		m.sourcesH = sourcesH
	} else {
		m.sourcesH = 0
	}
	m.summaryH = max(1, available-m.sourcesH)
	if m.isPlainStartupState() {
		m.summaryH = min(max(1, available), startupPreferredSummary+1)
	}

	m.viewport.Width = max(1, m.summaryW-m.styles.SummaryPanel.GetHorizontalFrameSize())
	m.viewport.Height = max(1, m.summaryH-m.styles.SummaryPanel.GetVerticalFrameSize())
	m.sourcesList.SetSize(m.contentW, m.sourcesH)

	inputWidth := max(1, m.contentW-2)
	m.followInput.Width = inputWidth
	m.searchInput.Width = inputWidth

	m.initRenderer()
}

func (m *model) initRenderer() {
	if m.summaryW <= 0 {
		return
	}

	renderer, err := glamour.NewTermRenderer(
		glamour.WithWordWrap(max(1, m.viewport.Width-2)),
		glamour.WithStylesFromJSONBytes(m.styles.GlamourJSON()),
		glamour.WithPreservedNewLines(),
		glamour.WithEmoji(),
	)
	if err == nil {
		m.renderer = renderer
	}
}

func (m *model) refreshViewport(forceBottom bool) {
	m.output = strings.TrimSpace(m.currentViewportMarkdown())

	if m.renderer == nil {
		m.initRenderer()
	}

	rendered := m.renderMarkdown(m.output)
	if strings.TrimSpace(rendered) == "" {
		rendered = "Press `f` to start a search."
	}

	m.baseRendered = strings.TrimRight(rendered, "\n")
	m.recomputeSearchMatches()
	m.viewport.SetContent(m.applySearchHighlights(m.baseRendered))

	if forceBottom || (m.autoScroll && (m.searching || m.waitingFirstToken || m.streaming)) {
		m.viewport.GotoBottom()
		m.newContent = false
	} else if m.streaming && !m.autoScroll {
		m.newContent = true
	}
}

func (m *model) composeTranscript() string {
	if len(m.turns) == 0 {
		return "## seek\n\nAI-powered web search from your terminal.\n\nPress `f` to ask a question."
	}

	var b strings.Builder
	for idx, turn := range m.turns {
		if idx > 0 {
			b.WriteString("\n\n---\n\n")
		}

		switch {
		case strings.TrimSpace(turn.Error) != "":
			fmt.Fprintf(&b, "> Error: %s\n\nPress `r` to retry.\n", turn.Error)
		case strings.TrimSpace(turn.Response) != "":
			display := sanitizeAssistantResponse(turn.Response)
			b.WriteString(strings.TrimSpace(display))
			if idx == m.currentTurn && m.streaming {
				if !strings.HasSuffix(display, "\n") {
					b.WriteString("\n\n")
				} else {
					b.WriteString("\n")
				}
				b.WriteString("▊")
			}
		case idx == m.currentTurn && m.searching:
			b.WriteString("_Searching the web..._")
		case idx == m.currentTurn && m.waitingFirstToken:
			b.WriteString("_Reading sources..._")
		default:
			b.WriteString("_No response yet._")
		}
	}

	return b.String()
}

func (m *model) currentViewportMarkdown() string {
	if strings.TrimSpace(m.overlayContent) != "" {
		return strings.TrimSpace(m.overlayContent)
	}
	return strings.TrimSpace(m.composeTranscript())
}

func (m *model) recomputeSearchMatches() {
	m.searchMatches = nil
	if strings.TrimSpace(m.searchQuery) == "" {
		m.searchIndex = 0
		return
	}

	query := strings.ToLower(m.searchQuery)
	for idx, line := range strings.Split(m.baseRendered, "\n") {
		if strings.Contains(strings.ToLower(stripANSI(line)), query) {
			m.searchMatches = append(m.searchMatches, idx)
		}
	}

	if len(m.searchMatches) == 0 {
		m.searchIndex = 0
		return
	}
	if m.searchIndex >= len(m.searchMatches) {
		m.searchIndex = 0
	}
}

func (m *model) applySearchHighlights(content string) string {
	if len(m.searchMatches) == 0 {
		return content
	}

	matchSet := make(map[int]struct{}, len(m.searchMatches))
	for _, idx := range m.searchMatches {
		matchSet[idx] = struct{}{}
	}

	current := m.searchMatches[m.searchIndex]
	lines := strings.Split(content, "\n")
	for idx, line := range lines {
		if _, ok := matchSet[idx]; ok {
			plain := stripANSI(line)
			if idx == current {
				lines[idx] = highlightInlineSearchMatches(plain, m.searchQuery, m.styles.SearchCurrent)
			} else {
				lines[idx] = highlightInlineSearchMatches(plain, m.searchQuery, m.styles.SearchMatch)
			}
		}
	}
	return strings.Join(lines, "\n")
}

func highlightInlineSearchMatches(text, query string, style lipgloss.Style) string {
	query = strings.TrimSpace(query)
	if query == "" || text == "" {
		return text
	}

	textRunes := []rune(text)
	queryRunes := []rune(query)
	if len(queryRunes) == 0 || len(textRunes) < len(queryRunes) {
		return text
	}

	var b strings.Builder
	last := 0
	for i := 0; i <= len(textRunes)-len(queryRunes); {
		segment := string(textRunes[i : i+len(queryRunes)])
		if strings.EqualFold(segment, query) {
			if last < i {
				b.WriteString(string(textRunes[last:i]))
			}
			b.WriteString(style.Render(segment))
			i += len(queryRunes)
			last = i
			continue
		}
		i++
	}

	if last == 0 {
		return text
	}
	if last < len(textRunes) {
		b.WriteString(string(textRunes[last:]))
	}
	return b.String()
}

func (m *model) jumpToSearchMatch() {
	if len(m.searchMatches) == 0 {
		return
	}
	line := m.searchMatches[m.searchIndex]
	m.viewport.SetYOffset(max(0, line-m.summaryH/2))
}

func (m *model) currentSources() []Source {
	if m.currentTurn < 0 || m.currentTurn >= len(m.turns) {
		return nil
	}
	return m.turns[m.currentTurn].Sources
}

func (m *model) syncSources() {
	items := make([]ui.SourceItem, 0, len(m.currentSources()))
	for _, source := range m.currentSources() {
		items = append(items, ui.SourceItem{
			Title:  source.Title,
			Domain: source.Domain,
			URL:    source.URL,
		})
	}
	m.sourcesList.SetItems(items)
	if m.state != StateSources {
		m.sourcesList.Blur()
	}
}

func (m *model) afterManualScroll() {
	m.autoScroll = m.viewport.AtBottom()
	m.newContent = m.streaming && !m.autoScroll
}

func (m *model) stopActiveRequest() {
	if m.cancel != nil {
		m.cancel()
	}
	m.finishRequest()
}

func (m *model) finishRequest() {
	m.cancel = nil
	m.requestCtx = nil
	m.streamSub = nil
	m.searching = false
	m.waitingFirstToken = false
	m.streaming = false
}

func (m *model) handleCodeYank() tea.Cmd {
	blocks := extractCodeBlocks(m.output)
	if len(blocks) == 0 {
		return m.flash("No fenced code blocks found", "warning")
	}
	if len(blocks) == 1 {
		return copyTextCmd(blocks[0].Content, "code block", lineCount(blocks[0].Content))
	}

	m.codeBlocks = blocks
	m.codeSelect = 0
	m.state = StateCodeSelect
	return nil
}

func (m *model) yankSelectedCodeBlock() tea.Cmd {
	if m.codeSelect < 0 || m.codeSelect >= len(m.codeBlocks) {
		return nil
	}
	block := m.codeBlocks[m.codeSelect]
	m.state = StateViewing
	return copyTextCmd(block.Content, "code block", lineCount(block.Content))
}

func (m *model) headerView() string {
	query := "press `f` to search"
	if m.currentTurn >= 0 {
		query = m.turns[m.currentTurn].Query
	} else if m.startupQuery != "" {
		query = m.startupQuery
	}

	left := m.styles.HeaderBrand.Render("seek")
	rightCount := "[0/0]"
	if m.queryCount > 0 {
		rightCount = fmt.Sprintf("[%d/%d]", m.queryCount, m.queryCount)
	}
	right := m.styles.HeaderCounter.Render(rightCount)
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	remaining := max(0, m.contentW-leftWidth-rightWidth)
	if remaining <= 1 {
		row := left + strings.Repeat(" ", max(0, m.contentW-leftWidth-rightWidth)) + right
		return m.styles.HeaderBar.Width(m.contentW).Render(row)
	}

	centerBudget := max(0, remaining-3)
	center := ""
	if centerBudget > 0 {
		queryFrame := m.styles.HeaderQuery.GetHorizontalFrameSize()
		queryBudget := max(0, centerBudget-queryFrame)
		center = m.styles.HeaderQuery.Render(fmt.Sprintf("\"%s\"", truncateForHeader(query, max(0, queryBudget-2))))
	}

	row := left
	if center != "" {
		row += " │ " + center
	}
	padding := max(0, m.contentW-lipgloss.Width(row)-rightWidth)
	row += strings.Repeat(" ", padding) + right
	return m.styles.HeaderBar.Width(m.contentW).Render(row)
}

func (m *model) summaryView() string {
	summaryWidth := m.viewport.Width
	summaryHeight := m.viewport.Height

	if m.state == StateCodeSelect {
		return m.wrapSummary(ui.RenderCodeSelection(m.styles, summaryWidth, summaryHeight, buildCodePreviews(m.codeBlocks), m.codeSelect))
	}

	if strings.TrimSpace(m.overlayContent) != "" {
		return m.wrapSummary(m.viewport.View())
	}

	if len(m.turns) == 0 {
		return m.wrapStartup(ui.RenderSplash(m.styles, max(1, m.contentW-2), max(1, m.summaryH), m.config.OutputFormat, m.llmProvider.Name()))
	}

	if m.currentTurn >= 0 && strings.TrimSpace(m.turns[m.currentTurn].Response) == "" {
		switch {
		case m.searching:
			return m.wrapSummary(ui.RenderPlaceholder(m.styles, summaryWidth, summaryHeight, m.spinner.View()+" Searching the web..."))
		case m.waitingFirstToken:
			return m.wrapSummary(ui.RenderPlaceholder(m.styles, summaryWidth, summaryHeight, m.spinner.View()+" Reading sources..."))
		}
	}

	return m.wrapSummary(m.viewport.View())
}

func (m *model) footerView() string {
	if m.state == StateInput {
		return ui.RenderInput(m.styles, m.followInput, m.contentW)
	}
	if m.state == StateSearchInput {
		return ui.RenderInput(m.styles, m.searchInput, m.contentW)
	}

	left := m.statusHints()
	if m.flashText != "" {
		switch m.flashKind {
		case "error":
			left = m.styles.ErrorText.Render(m.flashText)
		case "success":
			left = m.styles.SuccessText.Render(m.flashText)
		case "warning":
			left = m.styles.WarningText.Render(m.flashText)
		default:
			left = m.flashText
		}
	}

	return m.statusBar.View(left, m.statusMeta(), m.contentW)
}

func (m *model) sourcesSectionView() string {
	if len(m.turns) == 0 && m.state == StateInput && !m.shouldRenderInputSuggestions() {
		return ui.RenderWelcomeHint(m.styles, m.contentW, max(1, m.sourcesH))
	}
	if m.shouldRenderInputSuggestions() {
		return m.renderInputSuggestions()
	}
	if m.state == StateInput {
		return m.renderComposer()
	}
	return m.sourcesList.View()
}

func (m *model) statusHints() string {
	switch m.state {
	case StateLoading:
		return "q quit"
	case StateSources:
		return "j/k navigate  o open  y yank  Tab summary  q quit"
	case StateCodeSelect:
		return "1-9 choose  j/k navigate  Enter yank  Esc cancel"
	default:
		if m.currentTurn >= 0 && m.turns[m.currentTurn].Error != "" {
			return "j/k scroll  Tab sources  f follow-up  r retry  y yank  q quit"
		}
		hints := "j/k scroll  Tab sources  f follow-up  /help commands  y yank  Y code  / search  p print  q quit"
		if m.newContent {
			hints += "  ↓ new content"
		}
		return hints
	}
}

func (m *model) statusMeta() string {
	stack := m.projectStackLabel()
	join := func(parts ...string) string {
		filtered := make([]string, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				filtered = append(filtered, part)
			}
		}
		return strings.Join(filtered, " │ ")
	}
	configParts := []string{
		"mode=" + m.config.OutputFormat,
		"depth=" + m.config.SearchDepth,
		fmt.Sprintf("n=%d", m.config.MaxResults),
	}

	if m.searching {
		parts := append([]string{"Searching via Tavily..."}, configParts...)
		if stack != "" {
			parts = append(parts, stack)
		}
		return join(parts...)
	}
	if m.waitingFirstToken {
		parts := append([]string{"Reading sources via " + m.llmProvider.Name() + "..."}, configParts...)
		if stack != "" {
			parts = append(parts, stack)
		}
		return join(parts...)
	}
	if m.streaming {
		parts := append([]string{"Streaming via " + m.llmProvider.Name(), strconv.Itoa(m.tokenCount) + "t"}, configParts...)
		if stack != "" {
			parts = append(parts, stack)
		}
		return join(parts...)
	}
	if m.timingVisible && m.lastTiming.TotalMs > 0 {
		parts := []string{
			fmt.Sprintf("%dms (search: %dms, llm: %dms)", m.lastTiming.TotalMs, m.lastTiming.SearchMs, m.lastTiming.LLMMs),
			m.llmProvider.Name(),
		}
		parts = append(parts, configParts...)
		if stack != "" {
			parts = append(parts, stack)
		}
		return join(parts...)
	}

	parts := []string{m.llmProvider.Name()}
	parts = append(parts, configParts...)
	if stack != "" {
		parts = append(parts, stack)
	}
	if strings.TrimSpace(m.updateAvailable) != "" {
		parts = append(parts, "update="+m.updateAvailable)
	}
	if strings.TrimSpace(m.searchQuery) != "" && len(m.searchMatches) > 0 {
		parts = append(parts, fmt.Sprintf("%d/%d", m.searchIndex+1, len(m.searchMatches)))
	}
	return join(parts...)
}

func (m *model) renderComposer() string {
	state := ui.ComposerState{
		Draft: strings.TrimSpace(m.followInput.Value()),
	}
	if m.currentTurn >= 0 {
		state.LastQuery = m.turns[m.currentTurn].Query
	}
	return ui.RenderComposer(m.styles, m.contentW, m.sourcesH, state)
}

func (m *model) projectStackLabel() string {
	return projectStackLabel(m.projectContext)
}

func (m *model) showOverlay(markdown string) tea.Cmd {
	m.overlayContent = strings.TrimSpace(markdown)
	m.setFollowInputValue("")
	m.applyLayout()
	m.refreshViewport(false)
	return m.followInput.Focus()
}

func (m *model) flash(text, kind string) tea.Cmd {
	return m.flashFor(text, kind, 2*time.Second)
}

func (m *model) flashFor(text, kind string, duration time.Duration) tea.Cmd {
	m.flashText = text
	m.flashKind = kind
	return tea.Tick(duration, func(time.Time) tea.Msg {
		return flashClearMsg{}
	})
}

func friendlyError(err error, cfg Config) string {
	if err == nil {
		return ""
	}

	safe := func(text string) string {
		return strings.TrimSpace(sanitizeTerminalText(text))
	}

	var searchErr *searchpkg.APIError
	if errors.As(err, &searchErr) {
		switch searchErr.StatusCode {
		case 401, 403:
			return safe(fmt.Sprintf("Tavily rejected the API key. Update tavily_api_key in %s or TAVILY_API_KEY. Get one at https://tavily.com", ConfigPath()))
		case 429:
			if searchErr.RetryAfter > 0 {
				return safe(fmt.Sprintf("Tavily rate limited the request. Retry in %s.", searchErr.RetryAfter.Round(time.Second)))
			}
			return safe("Tavily rate limited the request. Press `r` to retry.")
		}
	}

	var llmErr *llmpkg.APIError
	if errors.As(err, &llmErr) {
		switch llmErr.StatusCode {
		case 401, 403:
			if llmErr.Provider == "ollama" {
				return safe(llmErr.Message)
			}
			return safe(fmt.Sprintf("%s rejected the API key. Set openai_api_key in %s or OPENAI_API_KEY.", llmErr.Provider, ConfigPath()))
		case 429:
			if llmErr.RetryAfter > 0 {
				return safe(fmt.Sprintf("%s rate limited the request. Retry in %s.", llmErr.Provider, llmErr.RetryAfter.Round(time.Second)))
			}
			return safe(fmt.Sprintf("%s rate limited the request. Press `r` to retry.", llmErr.Provider))
		}
	}

	msg := err.Error()
	switch {
	case strings.Contains(strings.ToLower(msg), "tavily api key is missing"):
		return safe(fmt.Sprintf("Tavily API key is missing. Set tavily_api_key in %s or TAVILY_API_KEY. Get one at https://tavily.com", ConfigPath()))
	case strings.HasPrefix(msg, "No API key set for"):
		return safe(msg)
	case strings.HasPrefix(msg, "Cannot connect to Ollama"):
		if strings.TrimSpace(cfg.OpenAIAPIKey) != "" {
			return safe(msg + ". You can also switch to `--backend openai`.")
		}
		return safe(msg)
	default:
		return safe(msg)
	}
}

func buildCodePreviews(blocks []CodeBlock) []ui.CodePreview {
	previews := make([]ui.CodePreview, 0, len(blocks))
	for idx, block := range blocks {
		lines := strings.Split(block.Content, "\n")
		if len(lines) > 4 {
			lines = lines[:4]
			lines = append(lines, "...")
		}
		previews = append(previews, ui.CodePreview{
			Index:    idx,
			Language: block.Language,
			Preview:  strings.Join(lines, "\n"),
			Lines:    lineCount(block.Content),
		})
	}
	return previews
}

func (m *model) wrapSummary(content string) string {
	return m.styles.SummaryPanel.
		Width(max(0, m.summaryW-m.styles.SummaryPanel.GetHorizontalFrameSize())).
		Height(max(0, m.summaryH-m.styles.SummaryPanel.GetVerticalFrameSize())).
		Render(content)
}

func (m *model) wrapStartup(content string) string {
	style := lipgloss.NewStyle().
		Padding(0, 1).
		Width(max(0, m.contentW-2)).
		Height(max(0, m.summaryH))
	return style.Render(content)
}

func (m *model) isStartupState() bool {
	return len(m.turns) == 0 && m.state == StateInput
}

func (m *model) isPlainStartupState() bool {
	return m.isStartupState() && !m.shouldRenderInputSuggestions()
}

func (m *model) syncLayout(forceBottom bool) tea.Cmd {
	m.applyLayout()
	m.refreshViewport(forceBottom)
	return nil
}

func (m *model) executeSlashCommand(raw string) tea.Cmd {
	commandLine := strings.TrimSpace(strings.TrimPrefix(raw, "/"))
	parts := strings.Fields(commandLine)
	if len(parts) == 0 {
		m.setFollowInputValue("")
		m.applyLayout()
		m.refreshViewport(false)
		return m.flash("Available: /backend /mode /model /depth /results /toggle /context /history /recent /stats /clear-history /show /help /exit", "warning")
	}

	command := strings.ToLower(parts[0])
	args := parts[1:]

	switch command {
	case "exit", "quit":
		m.stopActiveRequest()
		return tea.Quit
	case "help":
		m.setFollowInputValue("")
		m.applyLayout()
		m.refreshViewport(false)
		return tea.Batch(m.followInput.Focus(), m.flash("Commands: /backend, /mode, /model, /depth, /results, /toggle, /context, /history, /recent, /stats, /clear-history, /copy, /show, /help, /exit", "success"))
	case "show", "status":
		return m.showOverlay(sessionStatusMarkdown(m.config, m.llmProvider.Name(), m.projectStackLabel()))
	case "context":
		switch {
		case len(args) == 0:
			return m.showOverlay(contextSummaryMarkdown(m.projectContext, m.detectedProjectContext))
		case len(args) == 1 && strings.EqualFold(args[0], "off"):
			m.projectContext = nil
			if m.orchestrator != nil {
				m.orchestrator.projectContext = nil
			}
			return m.showOverlay(contextSummaryMarkdown(m.projectContext, m.detectedProjectContext))
		case len(args) == 1 && strings.EqualFold(args[0], "on"):
			m.detectedProjectContext = cloneProjectContext(projectctx.DetectContext(m.workingDir))
			m.projectContext = cloneProjectContext(m.detectedProjectContext)
			if m.orchestrator != nil {
				m.orchestrator.projectContext = m.projectContext
			}
			return m.showOverlay(contextSummaryMarkdown(m.projectContext, m.detectedProjectContext))
		default:
			return m.failSlashCommand("Usage: /context [on|off]")
		}
	case "history":
		if len(args) == 0 {
			return m.failSlashCommand("Usage: /history <query>")
		}
		if m.historyStore == nil {
			return m.failSlashCommand("History is not available for this session")
		}
		records, err := m.historyStore.Search(strings.Join(args, " "), 10)
		if err != nil {
			return m.failSlashCommand("History search failed: " + err.Error())
		}
		return m.showOverlay(historyRecordsMarkdown("History Search", records))
	case "recent":
		if m.historyStore == nil {
			return m.failSlashCommand("History is not available for this session")
		}
		limit := 10
		if len(args) == 1 {
			value, err := strconv.Atoi(args[0])
			if err != nil || value <= 0 {
				return m.failSlashCommand("Usage: /recent [count]")
			}
			limit = value
		} else if len(args) > 1 {
			return m.failSlashCommand("Usage: /recent [count]")
		}
		records, err := m.historyStore.Recent(limit, "")
		if err != nil {
			return m.failSlashCommand("Recent history failed: " + err.Error())
		}
		return m.showOverlay(historyRecordsMarkdown("Recent Searches", records))
	case "stats":
		if len(args) != 0 {
			return m.failSlashCommand("Usage: /stats")
		}
		if m.historyStore == nil {
			return m.failSlashCommand("History is not available for this session")
		}
		stats, err := m.historyStore.Stats()
		if err != nil {
			return m.failSlashCommand("History stats failed: " + err.Error())
		}
		return m.showOverlay(historyStatsMarkdown(stats))
	case "clear-history":
		if len(args) != 0 {
			return m.failSlashCommand("Usage: /clear-history")
		}
		if m.historyStore == nil {
			return m.failSlashCommand("History is not available for this session")
		}
		deleted, err := m.historyStore.Clear()
		if err != nil {
			return m.failSlashCommand("Clear history failed: " + err.Error())
		}
		m.clearTurnHistoryIDs()
		return m.showOverlay(historyClearMarkdown(deleted))
	case "copy":
		if strings.TrimSpace(m.composeTranscript()) == "" || len(m.turns) == 0 {
			return m.failSlashCommand("Nothing to copy yet")
		}
		m.setFollowInputValue("")
		m.applyLayout()
		m.refreshViewport(false)
		return tea.Batch(m.followInput.Focus(), copyTextCmd(m.composeTranscript(), "chat history", lineCount(m.composeTranscript())))
	case "backend":
		if len(args) != 1 {
			return m.failSlashCommand("Usage: /backend ollama|openai")
		}
		value := strings.ToLower(strings.TrimSpace(args[0]))
		if value != "ollama" && value != "openai" {
			return m.failSlashCommand("Usage: /backend ollama|openai")
		}
		cfg := m.config
		cfg.LLMBackend = value
		return m.applySessionConfig(cfg, "Backend set to "+value)
	case "mode":
		if len(args) != 1 {
			return m.failSlashCommand("Usage: /mode concise|learning|explanatory|oneliner")
		}
		value := strings.ToLower(strings.TrimSpace(args[0]))
		if !isValidOutputFormat(value) {
			return m.failSlashCommand("Usage: /mode concise|learning|explanatory|oneliner")
		}
		cfg := m.config
		cfg.OutputFormat = value
		return m.applySessionConfig(cfg, "Output format set to "+value)
	case "model":
		if len(args) == 0 {
			return m.failSlashCommand("Usage: /model <name>")
		}
		value := strings.TrimSpace(strings.Join(args, " "))
		if value == "" {
			return m.failSlashCommand("Usage: /model <name>")
		}
		cfg := m.config
		if strings.ToLower(cfg.LLMBackend) == "openai" {
			cfg.OpenAIModel = value
		} else {
			cfg.OllamaModel = value
		}
		return m.applySessionConfig(cfg, "Model set to "+value)
	case "depth":
		if len(args) != 1 {
			return m.failSlashCommand("Usage: /depth basic|advanced")
		}
		value := strings.ToLower(strings.TrimSpace(args[0]))
		if value != "basic" && value != "advanced" {
			return m.failSlashCommand("Usage: /depth basic|advanced")
		}
		cfg := m.config
		cfg.SearchDepth = value
		return m.applySessionConfig(cfg, "Search depth set to "+value)
	case "results":
		if len(args) != 1 {
			return m.failSlashCommand("Usage: /results <1-20>")
		}
		count, err := strconv.Atoi(args[0])
		if err != nil || count < 1 || count > 20 {
			return m.failSlashCommand("Usage: /results <1-20>")
		}
		cfg := m.config
		cfg.MaxResults = count
		return m.applySessionConfig(cfg, fmt.Sprintf("Max results set to %d", count))
	case "toggle":
		if len(args) != 0 {
			return m.failSlashCommand("Usage: /toggle")
		}
		return m.toggleThemePreference()
	default:
		return m.failSlashCommand("Unknown command. Try /help")
	}
}

func (m *model) applySessionConfig(cfg Config, successText string) tea.Cmd {
	cfg.normalize()

	searchProvider, llmProvider, err := initProviders(cfg)
	if err != nil {
		return m.failSlashCommand(err.Error())
	}

	m.config = cfg
	m.applyThemeStyles()
	m.searchProvider = searchProvider
	m.llmProvider = llmProvider
	m.orchestrator = NewOrchestrator(searchProvider, llmProvider, cfg.MaxResults, cfg.OutputFormat, m.projectContext)

	m.setFollowInputValue("")
	m.applyLayout()
	m.refreshViewport(false)
	return tea.Batch(m.followInput.Focus(), m.flash(successText+" · "+m.sessionSummary(), "success"))
}

func (m *model) saveTurnToHistoryCmd(turnIndex int) tea.Cmd {
	if m.historyStore == nil || turnIndex < 0 || turnIndex >= len(m.turns) {
		return nil
	}

	turn := m.turns[turnIndex]
	if strings.TrimSpace(turn.Query) == "" || strings.TrimSpace(turn.Response) == "" {
		return nil
	}

	var parentID *int64
	if turn.IsFollowUp && turnIndex > 0 && m.turns[turnIndex-1].HistoryID != nil {
		value := *m.turns[turnIndex-1].HistoryID
		parentID = &value
	}

	record := &historypkg.SearchRecord{
		Query:        turn.Query,
		Response:     sanitizeAssistantResponse(turn.Response),
		Sources:      convertSources(turn.Sources),
		ProjectDir:   m.workingDir,
		ProjectStack: m.projectStackLabel(),
		LLMBackend:   m.llmProvider.Name(),
		OutputFormat: m.config.OutputFormat,
		SearchMs:     m.lastSearchMs,
		LLMMs:        m.lastLLMMs,
		TotalMs:      m.lastTotalMs,
		IsFollowUp:   turn.IsFollowUp,
		ParentID:     parentID,
	}

	store := m.historyStore
	return func() tea.Msg {
		id, err := store.Save(record)
		return historySavedMsg{TurnIndex: turnIndex, ID: id, Err: err}
	}
}

func (m *model) clearTurnHistoryIDs() {
	for idx := range m.turns {
		m.turns[idx].HistoryID = nil
	}
}

func (m *model) failSlashCommand(text string) tea.Cmd {
	return tea.Batch(m.followInput.Focus(), m.flash(text, "error"))
}

func (m *model) sessionSummary() string {
	return fmt.Sprintf(
		"backend=%s model=%s mode=%s theme=%s depth=%s results=%d context=%s",
		m.config.LLMBackend,
		activeModel(m.config),
		m.config.OutputFormat,
		themeStatusLine(m.config.Theme),
		m.config.SearchDepth,
		m.config.MaxResults,
		fallbackString(m.projectStackLabel(), "off"),
	)
}

func extractCodeBlocks(markdown string) []CodeBlock {
	matches := codeFenceRE.FindAllStringSubmatch(markdown, -1)
	blocks := make([]CodeBlock, 0, len(matches))
	for _, match := range matches {
		language := strings.TrimSpace(match[1])
		content := strings.TrimSpace(match[2])
		if content == "" {
			continue
		}
		blocks = append(blocks, CodeBlock{
			Language: language,
			Content:  content,
		})
	}
	return blocks
}

func lineCount(text string) int {
	if text == "" {
		return 0
	}
	return strings.Count(text, "\n") + 1
}

func stripANSI(text string) string {
	return ansiRE.ReplaceAllString(text, "")
}

func prepareMarkdownForRender(markdown string) string {
	markdown = strings.ReplaceAll(markdown, "\r\n", "\n")
	markdown = strings.ReplaceAll(markdown, "\t", "    ")
	return markdown
}

func (m *model) renderMarkdown(markdown string) string {
	prepared, blocks := m.injectCodeBlockPlaceholders(prepareMarkdownForRender(markdown))
	rendered := prepared
	if m.renderer != nil && strings.TrimSpace(prepared) != "" {
		if out, err := m.renderer.Render(prepared); err == nil {
			rendered = out
		}
	}

	for _, block := range blocks {
		rendered = strings.ReplaceAll(rendered, block.Placeholder, block.Rendered)
	}
	return rendered
}

func (m *model) injectCodeBlockPlaceholders(markdown string) (string, []renderedCodeBlock) {
	index := 0
	blocks := make([]renderedCodeBlock, 0)

	replaced := codeFenceRE.ReplaceAllStringFunc(markdown, func(match string) string {
		parts := codeFenceRE.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}

		language := normalizeCodeLanguage(parts[1])
		content := strings.TrimRight(parts[2], "\n")
		placeholder := fmt.Sprintf("SEEKCODEBLOCK%03dTOKEN", index)
		index++

		blocks = append(blocks, renderedCodeBlock{
			Placeholder: placeholder,
			Rendered:    ui.RenderCodeBlock(m.styles, max(1, m.viewport.Width-2), language, content),
		})

		return "\n" + placeholder + "\n"
	})

	return replaced, blocks
}

func normalizeCodeLanguage(language string) string {
	switch strings.TrimSpace(strings.ToLower(language)) {
	case "", "plain", "plaintext":
		return "text"
	case "ascii", "diagram":
		return "diagram"
	default:
		return strings.TrimSpace(language)
	}
}

func sanitizeAssistantResponse(response string) string {
	response = strings.ReplaceAll(response, "\r\n", "\n")
	response = strings.TrimSpace(response)
	if response == "" {
		return response
	}

	match := sourceTailRE.FindStringIndex(response)
	if match == nil {
		return response
	}

	prefix := strings.TrimSpace(response[:match[0]])
	tail := strings.TrimSpace(response[match[0]:])
	if !looksLikeSourceTail(tail) {
		return response
	}
	return prefix
}

func looksLikeSourceTail(tail string) bool {
	lines := strings.Split(strings.TrimSpace(tail), "\n")
	if len(lines) < 2 {
		return false
	}

	sourceLike := 0
	meaningful := 0
	for idx, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		meaningful++
		if idx == 0 && isSourceHeading(line) {
			sourceLike++
			continue
		}
		if isSourceListLine(line) {
			sourceLike++
		}
	}

	return meaningful >= 2 && sourceLike == meaningful
}

func isSourceHeading(line string) bool {
	line = strings.TrimSpace(strings.Trim(line, "*_"))
	line = strings.TrimSpace(strings.TrimLeft(line, "#"))
	line = strings.TrimSpace(strings.TrimSuffix(line, ":"))
	switch strings.ToLower(line) {
	case "source", "sources", "reference", "references", "citation", "citations", "further reading":
		return true
	default:
		return false
	}
}

func isSourceListLine(line string) bool {
	line = strings.TrimSpace(line)
	line = strings.TrimLeft(line, "-*0123456789.[]() ")
	lower := strings.ToLower(line)
	switch {
	case strings.HasPrefix(lower, "http://"), strings.HasPrefix(lower, "https://"):
		return true
	case strings.Contains(lower, "://"):
		return true
	case strings.Contains(lower, ".com"), strings.Contains(lower, ".org"), strings.Contains(lower, ".dev"), strings.Contains(lower, ".io"), strings.Contains(lower, ".net"), strings.Contains(lower, ".edu"), strings.Contains(lower, ".gov"):
		return true
	case strings.HasPrefix(lower, "[") && strings.Contains(lower, "]"):
		return true
	default:
		return false
	}
}

func activeModel(cfg Config) string {
	if strings.ToLower(cfg.LLMBackend) == "openai" {
		return cfg.OpenAIModel
	}
	return cfg.OllamaModel
}

func isValidOutputFormat(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "concise", "learning", "explanatory", "oneliner":
		return true
	default:
		return false
	}
}

func effectiveSearchQuery(rawQuery, preparedQuery string, attachedFiles []AttachedFile) string {
	query := strings.TrimSpace(preparedQuery)
	if query != "" {
		return query
	}
	if len(attachedFiles) == 0 {
		return strings.TrimSpace(rawQuery)
	}

	paths := make([]string, 0, len(attachedFiles))
	for _, file := range attachedFiles {
		if path := strings.TrimSpace(file.DisplayPath); path != "" {
			paths = append(paths, path)
		}
	}
	if len(paths) == 0 {
		return strings.TrimSpace(rawQuery)
	}
	return "explain " + strings.Join(paths, " ")
}

func isEditingKey(key string) bool {
	switch key {
	case "backspace", "delete", "left", "right", "home", "end", "ctrl+a", "ctrl+e", "alt+b", "alt+f", "ctrl+w", "ctrl+u", "ctrl+k":
		return true
	default:
		return false
	}
}

func (m *model) isSlashInput() bool {
	return strings.HasPrefix(strings.TrimSpace(m.followInput.Value()), "/")
}

func (m *model) tryAutocompleteSlashCommand() bool {
	return m.inputSuggestionMode == inputSuggestionSlash && m.acceptSelectedInputSuggestion()
}

func (m *model) filteredSlashCommands(prefix string) []slashCommandSpec {
	query := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(prefix, "/")))
	all := []slashCommandSpec{
		{Name: "backend", Usage: "/backend <ollama|openai>", Description: "switch the active LLM backend for this session"},
		{Name: "mode", Usage: "/mode <concise|learning|explanatory|oneliner>", Description: "change the answer style without restarting"},
		{Name: "model", Usage: "/model <name>", Description: "set the model for the active backend"},
		{Name: "depth", Usage: "/depth <basic|advanced>", Description: "change Tavily search depth"},
		{Name: "results", Usage: "/results <n>", Description: "set how many Tavily results to inject into context"},
		{Name: "toggle", Usage: "/toggle", Description: "switch between light and dark themes and save it"},
		{Name: "context", Usage: "/context [on|off]", Description: "show or toggle detected project context"},
		{Name: "history", Usage: "/history <query>", Description: "search saved local history from previous queries"},
		{Name: "recent", Usage: "/recent [count]", Description: "show the most recent saved searches"},
		{Name: "stats", Usage: "/stats", Description: "show aggregate search history stats"},
		{Name: "clear-history", Usage: "/clear-history", Description: "delete all saved local history entries"},
		{Name: "copy", Usage: "/copy", Description: "copy the full chat history including follow-ups"},
		{Name: "show", Usage: "/show", Description: "show the active session configuration"},
		{Name: "exit", Usage: "/exit", Description: "gracefully exit seek"},
	}

	if query == "" {
		return all
	}

	filtered := make([]slashCommandSpec, 0, len(all))
	fallback := make([]slashCommandSpec, 0, len(all))
	for _, command := range all {
		switch {
		case strings.HasPrefix(command.Name, query):
			filtered = append(filtered, command)
		case strings.Contains(command.Name, query):
			fallback = append(fallback, command)
		}
	}
	return append(filtered, fallback...)
}

func (m *model) toggleThemePreference() tea.Cmd {
	cfg := m.config
	if resolveThemePreference(cfg.Theme) == "light" {
		cfg.Theme = "dark"
	} else {
		cfg.Theme = "light"
	}
	cfg.normalize()

	saveErr := writeConfigFile(ConfigPath(), cfg)
	m.config = cfg
	m.applyThemeStyles()
	m.setFollowInputValue("")
	m.applyLayout()
	m.refreshViewport(false)

	message := "Theme set to " + cfg.Theme
	kind := "success"
	if saveErr != nil {
		message += " (session only: " + saveErr.Error() + ")"
		kind = "warning"
	} else {
		message += " and saved"
	}
	return tea.Batch(m.followInput.Focus(), m.flash(message+" · "+m.sessionSummary(), kind))
}

func (m *model) applyThemeStyles() {
	styles := ui.LoadTheme(resolveThemePreference(m.config.Theme))
	m.styles = styles
	m.spinner.Style = styles.Spinner
	applyInputTheme(&m.followInput, styles)
	applyInputTheme(&m.searchInput, styles)
	m.statusBar = ui.NewStatusBar(styles)

	selectedURL := ""
	if item, ok := m.sourcesList.Selected(); ok {
		selectedURL = strings.TrimSpace(item.URL)
	}
	focusedSources := m.state == StateSources

	newSources := ui.NewSources(styles)
	newSources.SetSize(m.contentW, m.sourcesH)
	m.sourcesList = newSources
	m.syncSources()
	if focusedSources {
		m.sourcesList.Focus()
	}
	if selectedURL != "" {
		items := m.currentSources()
		for idx, source := range items {
			if strings.TrimSpace(source.URL) != selectedURL {
				continue
			}
			for step := 0; step < idx; step++ {
				m.sourcesList.Next()
			}
			break
		}
	}

	m.renderer = nil
	m.initRenderer()
}

func applyInputTheme(input *textinput.Model, styles ui.Styles) {
	input.PromptStyle = styles.InputPrompt
	input.TextStyle = styles.InputText
	input.PlaceholderStyle = styles.Dimmed
	input.CursorStyle = styles.InputCursor
}

func emitQuery(query string, followUp bool) tea.Cmd {
	return func() tea.Msg {
		return startQueryMsg{Query: query, FollowUp: followUp}
	}
}

func clearTimingCmd(seq int) tea.Cmd {
	return tea.Tick(5*time.Second, func(time.Time) tea.Msg {
		return timingClearMsg{Seq: seq}
	})
}

func searchCmd(ctx context.Context, orchestrator *Orchestrator, query string, requestID int) tea.Cmd {
	return func() tea.Msg {
		results, err := orchestrator.Search(ctx, query)
		return searchCompleteMsg{RequestID: requestID, Results: results, Err: err}
	}
}

func waitForMsg(sub <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-sub
		if !ok {
			return nil
		}
		return msg
	}
}

func sendStreamMsg(ctx context.Context, sub chan<- tea.Msg, msg tea.Msg) {
	select {
	case <-ctx.Done():
	case sub <- msg:
	}
}

func copyTextCmd(text, label string, lines int) tea.Cmd {
	return func() tea.Msg {
		return clipboardResultMsg{
			Label: label,
			Err:   CopyToClipboard(text),
			Lines: lines,
		}
	}
}

func openSourceCmd(targetURL, browser string) tea.Cmd {
	return func() tea.Msg {
		return browserResultMsg{Err: OpenBrowser(targetURL, browser)}
	}
}

func clearScreenCmd() tea.Cmd {
	return func() tea.Msg {
		return tea.ClearScreen()
	}
}

func (m *model) loadHistoryRecord(record *historypkg.SearchRecord) {
	if record == nil {
		return
	}

	m.turns = []Turn{{
		Query:       record.Query,
		SearchQuery: record.Query,
		Response:    record.Response,
		Sources:     convertHistorySources(record.Sources),
		IsFollowUp:  record.IsFollowUp,
		HistoryID:   int64Ptr(record.ID),
	}}
	m.currentTurn = 0
	m.queryCount = 1
	m.lastSearchMs = record.SearchMs
	m.lastLLMMs = record.LLMMs
	m.lastTotalMs = record.TotalMs
	m.lastTiming = SearchTiming{
		SearchMs: record.SearchMs,
		LLMMs:    record.LLMMs,
		TotalMs:  record.TotalMs,
	}
}

func truncateForHeader(value string, width int) string {
	value = strings.TrimSpace(value)
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}

	runes := []rune(value)
	if width <= 3 {
		if width > len(runes) {
			width = len(runes)
		}
		return string(runes[:width])
	}

	cut := width - 3
	if cut > len(runes) {
		cut = len(runes)
	}
	return string(runes[:cut]) + "..."
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func int64Ptr(value int64) *int64 {
	return &value
}
