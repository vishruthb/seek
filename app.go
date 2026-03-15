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

	llmpkg "seek/llm"
	searchpkg "seek/search"
	"seek/ui"
)

var (
	ansiRE       = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	codeFenceRE  = regexp.MustCompile("(?ms)```([^\\n`]*)\\n(.*?)```")
	sourceTailRE = regexp.MustCompile(`(?is)\n{2,}(?:#{1,6}\s*|[*_]{0,2})?(sources?|references?|citations?|further reading)(?:[*_]{0,2})\s*:?\s*\n`)
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
	flashClearMsg struct{}
)

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

	requestID  int
	requestCtx context.Context
	streamSub  chan tea.Msg
	cancel     context.CancelFunc
}

func NewModel(cfg Config, initialQuery string, searchProvider searchpkg.SearchProvider, llmProvider llmpkg.LLMProvider) *model {
	styles := ui.LoadTheme(cfg.Theme)

	vp := viewport.New(0, 0)
	vp.Style = lipgloss.NewStyle()
	vp.HighPerformanceRendering = false

	spin := spinner.New()
	spin.Spinner = spinner.Dot
	spin.Style = styles.Spinner

	follow := ui.NewFollowUpInput(styles)
	follow.Placeholder = "ask a question or use /help"
	searchInput := ui.NewSearchInput(styles)
	searchInput.Placeholder = "search within the summary"

	return &model{
		state:          StateViewing,
		startupQuery:   strings.TrimSpace(initialQuery),
		viewport:       vp,
		sourcesList:    ui.NewSources(styles),
		followInput:    follow,
		searchInput:    searchInput,
		spinner:        spin,
		statusBar:      ui.NewStatusBar(styles),
		styles:         styles,
		searchProvider: searchProvider,
		llmProvider:    llmProvider,
		orchestrator:   NewOrchestrator(searchProvider, llmProvider, cfg.MaxResults, cfg.OutputFormat),
		config:         cfg,
		autoScroll:     true,
		printOnExit:    cfg.PrintOnExit,
		currentTurn:    -1,
	}
}

func (m *model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.spinner.Tick}

	if m.startupQuery != "" {
		m.state = StateLoading
		cmds = append(cmds, emitQuery(m.startupQuery, false))
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
		m.beginQuery(msg.Query, msg.FollowUp)
		return m, tea.Batch(m.startSearch(), m.spinner.Tick)

	case searchCompleteMsg:
		if msg.RequestID != m.requestID {
			return m, nil
		}
		m.searching = false
		if msg.Err != nil {
			m.finishRequest()
			if m.currentTurn >= 0 {
				m.turns[m.currentTurn].Error = friendlyError(msg.Err, m.config)
				m.turns[m.currentTurn].Sources = nil
			}
			m.state = StateViewing
			m.syncSources()
			m.refreshViewport(false)
			return m, nil
		}

		if m.currentTurn >= 0 {
			m.turns[m.currentTurn].Sources = sourcesFromSearchResults(msg.Results)
			m.turns[m.currentTurn].Error = ""
		}
		m.waitingFirstToken = true
		m.syncSources()
		m.refreshViewport(false)
		return m, tea.Batch(m.startLLMStream(msg.Results), m.spinner.Tick)

	case tokenMsg:
		if msg.RequestID != m.requestID || m.currentTurn < 0 {
			return m, nil
		}
		if m.waitingFirstToken {
			m.waitingFirstToken = false
			if m.state == StateLoading {
				m.state = StateViewing
			}
		}
		m.streaming = true
		m.turns[m.currentTurn].Response += msg.Text
		m.turns[m.currentTurn].Error = ""
		m.tokenCount += len(strings.Fields(msg.Text))
		m.refreshViewport(false)
		if m.streamSub != nil {
			return m, waitForMsg(m.streamSub)
		}
		return m, nil

	case streamDoneMsg:
		if msg.RequestID != m.requestID {
			return m, nil
		}
		m.finishRequest()
		if m.state == StateLoading {
			m.state = StateViewing
		}
		m.refreshViewport(m.autoScroll)
		return m, nil

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
		m.refreshViewport(false)
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

	parts := []string{
		m.headerView(),
		m.summaryView(),
		m.sourcesSectionView(),
		m.footerView(),
	}
	return m.styles.AppFrame.Render(strings.Join(parts, "\n"))
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
		m.followInput.SetValue("")
		m.state = StateViewing
		return nil
	case "tab":
		if m.tryAutocompleteSlashCommand() {
			return nil
		}
		return nil
	case "enter":
		query := strings.TrimSpace(m.followInput.Value())
		if query == "" {
			return m.flash("Follow-up query is empty", "warning")
		}
		if strings.HasPrefix(query, "/") {
			return m.executeSlashCommand(query)
		}
		m.followInput.Blur()
		m.followInput.SetValue("")
		return emitQuery(query, m.queryCount > 0)
	default:
		var cmd tea.Cmd
		m.followInput, cmd = m.followInput.Update(msg)
		return cmd
	}
}

func (m *model) handleSearchKeys(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.searchInput.Blur()
		m.state = StateViewing
		return nil
	case "enter":
		m.searchQuery = strings.TrimSpace(m.searchInput.Value())
		m.searchInput.Blur()
		m.state = StateViewing
		m.refreshViewport(false)
		if len(m.searchMatches) == 0 && m.searchQuery != "" {
			return m.flash("No matches found", "warning")
		}
		m.jumpToSearchMatch()
		return nil
	default:
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
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
		return nil
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
		return nil
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
	case "f":
		m.state = StateInput
		m.followInput.SetValue("")
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

func (m *model) beginQuery(query string, followUp bool) {
	query = strings.TrimSpace(query)
	if query == "" {
		return
	}

	m.stopActiveRequest()
	m.requestID++
	m.requestCtx, m.cancel = context.WithCancel(context.Background())

	m.turns = append(m.turns, Turn{
		Query:      query,
		IsFollowUp: followUp,
	})
	m.currentTurn = len(m.turns) - 1
	m.queryCount = len(m.turns)
	m.tokenCount = 0
	m.searching = true
	m.streaming = false
	m.waitingFirstToken = false
	m.autoScroll = true
	m.newContent = false
	m.state = StateLoading
	m.codeBlocks = nil
	m.codeSelect = 0
	m.syncSources()
	m.refreshViewport(true)
}

func (m *model) retryCurrentTurn() tea.Cmd {
	if m.currentTurn < 0 {
		return nil
	}

	m.stopActiveRequest()
	m.requestID++
	m.requestCtx, m.cancel = context.WithCancel(context.Background())

	m.turns[m.currentTurn].Response = ""
	m.turns[m.currentTurn].Error = ""
	m.turns[m.currentTurn].Sources = nil
	m.tokenCount = 0
	m.searching = true
	m.streaming = false
	m.waitingFirstToken = false
	m.autoScroll = true
	m.newContent = false
	m.state = StateLoading
	m.syncSources()
	m.refreshViewport(true)
	return tea.Batch(m.startSearch(), m.spinner.Tick)
}

func (m *model) startSearch() tea.Cmd {
	if m.currentTurn < 0 || m.requestCtx == nil {
		return nil
	}
	return searchCmd(m.requestCtx, m.searchProvider, m.turns[m.currentTurn].Query, m.config.MaxResults, m.requestID)
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
	requestID := m.requestID
	orchestrator := m.orchestrator

	go func() {
		defer close(sub)

		sawTokens := false
		response, err := orchestrator.StreamAnswer(ctx, query, results, history, func(token string) {
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

	m.contentW = max(18, m.width-2)
	m.contentH = max(6, m.height-2)
	m.summaryW = max(12, m.contentW-2)

	headerH := 1
	footerH := 1

	sourcesH := min(6, max(4, len(m.currentSources())+2))
	available := m.contentH - headerH - footerH
	if available < 6 {
		sourcesH = max(3, available/3)
	}
	if sourcesH >= available {
		sourcesH = max(3, available-3)
	}
	m.sourcesH = max(3, sourcesH)
	m.summaryH = max(3, available-m.sourcesH)

	m.viewport.Width = max(8, m.summaryW-m.styles.SummaryPanel.GetHorizontalFrameSize())
	m.viewport.Height = max(1, m.summaryH-m.styles.SummaryPanel.GetVerticalFrameSize())
	m.sourcesList.SetSize(m.contentW, m.sourcesH)

	inputWidth := max(10, m.contentW-2)
	m.followInput.Width = inputWidth
	m.searchInput.Width = inputWidth

	m.initRenderer()
}

func (m *model) initRenderer() {
	if m.summaryW <= 0 {
		return
	}

	renderer, err := glamour.NewTermRenderer(
		glamour.WithWordWrap(max(20, m.viewport.Width-2)),
		glamour.WithStylesFromJSONBytes(m.styles.GlamourJSON()),
		glamour.WithPreservedNewLines(),
		glamour.WithEmoji(),
	)
	if err == nil {
		m.renderer = renderer
	}
}

func (m *model) refreshViewport(forceBottom bool) {
	m.output = strings.TrimSpace(m.composeTranscript())

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

		label := "Query"
		if turn.IsFollowUp {
			label = "Follow-up"
		}
		fmt.Fprintf(&b, "## %s: %q\n\n", label, turn.Query)

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
		if idx == current {
			lines[idx] = m.styles.SearchCurrent.Render(line)
			continue
		}
		if _, ok := matchSet[idx]; ok {
			lines[idx] = m.styles.SearchMatch.Render(line)
		}
	}
	return strings.Join(lines, "\n")
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
	center := m.styles.HeaderQuery.Render(fmt.Sprintf("\"%s\"", query))
	rightCount := "[0/0]"
	if m.queryCount > 0 {
		rightCount = fmt.Sprintf("[%d/%d]", m.queryCount, m.queryCount)
	}
	right := m.styles.HeaderCounter.Render(rightCount)

	space := max(0, m.contentW-lipgloss.Width(left)-lipgloss.Width(center)-lipgloss.Width(right)-4)
	row := left + " │ " + center + strings.Repeat(" ", space) + right
	return m.styles.HeaderBar.Width(m.contentW).Render(row)
}

func (m *model) summaryView() string {
	summaryWidth := m.viewport.Width
	summaryHeight := m.viewport.Height

	if m.state == StateCodeSelect {
		return m.wrapSummary(ui.RenderCodeSelection(m.styles, summaryWidth, summaryHeight, buildCodePreviews(m.codeBlocks), m.codeSelect))
	}

	if len(m.turns) == 0 {
		return m.wrapSummary(ui.RenderSplash(m.styles, summaryWidth, summaryHeight, m.config.OutputFormat, m.llmProvider.Name()))
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
	if m.state == StateInput && m.isSlashInput() {
		return m.renderSlashSuggestions()
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
	if m.searching {
		return "Searching via Tavily... · " + m.config.OutputFormat
	}
	if m.waitingFirstToken {
		return "Reading sources via " + m.llmProvider.Name() + "... · " + m.config.OutputFormat
	}
	if m.streaming {
		return "Streaming via " + m.llmProvider.Name() + " · " + m.config.OutputFormat + " · " + strconv.Itoa(m.tokenCount) + "t"
	}

	parts := []string{m.llmProvider.Name(), m.config.OutputFormat, fmt.Sprintf("%d%%", int(m.viewport.ScrollPercent()*100))}
	if strings.TrimSpace(m.searchQuery) != "" && len(m.searchMatches) > 0 {
		parts = append(parts, fmt.Sprintf("%d/%d", m.searchIndex+1, len(m.searchMatches)))
	}
	return strings.Join(parts, " · ")
}

func (m *model) renderComposer() string {
	state := ui.ComposerState{
		Backend: m.config.LLMBackend,
		Model:   activeModel(m.config),
		Format:  m.config.OutputFormat,
		Depth:   m.config.SearchDepth,
		Results: m.config.MaxResults,
		Draft:   strings.TrimSpace(m.followInput.Value()),
	}
	if m.currentTurn >= 0 {
		state.LastQuery = m.turns[m.currentTurn].Query
	}
	return ui.RenderComposer(m.styles, m.contentW, m.sourcesH, state)
}

func (m *model) flash(text, kind string) tea.Cmd {
	m.flashText = text
	m.flashKind = kind
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return flashClearMsg{}
	})
}

func friendlyError(err error, cfg Config) string {
	if err == nil {
		return ""
	}

	var searchErr *searchpkg.APIError
	if errors.As(err, &searchErr) {
		switch searchErr.StatusCode {
		case 401, 403:
			return fmt.Sprintf("Tavily rejected the API key. Update tavily_api_key in %s or TAVILY_API_KEY. Get one at https://tavily.com", ConfigPath())
		case 429:
			if searchErr.RetryAfter > 0 {
				return fmt.Sprintf("Tavily rate limited the request. Retry in %s.", searchErr.RetryAfter.Round(time.Second))
			}
			return "Tavily rate limited the request. Press `r` to retry."
		}
	}

	var llmErr *llmpkg.APIError
	if errors.As(err, &llmErr) {
		switch llmErr.StatusCode {
		case 401, 403:
			if llmErr.Provider == "ollama" {
				return llmErr.Message
			}
			return fmt.Sprintf("%s rejected the API key. Set openai_api_key in %s or OPENAI_API_KEY.", llmErr.Provider, ConfigPath())
		case 429:
			if llmErr.RetryAfter > 0 {
				return fmt.Sprintf("%s rate limited the request. Retry in %s.", llmErr.Provider, llmErr.RetryAfter.Round(time.Second))
			}
			return fmt.Sprintf("%s rate limited the request. Press `r` to retry.", llmErr.Provider)
		}
	}

	msg := err.Error()
	switch {
	case strings.Contains(strings.ToLower(msg), "tavily api key is missing"):
		return fmt.Sprintf("Tavily API key is missing. Set tavily_api_key in %s or TAVILY_API_KEY. Get one at https://tavily.com", ConfigPath())
	case strings.HasPrefix(msg, "No API key set for"):
		return msg
	case strings.HasPrefix(msg, "Cannot connect to Ollama"):
		if strings.TrimSpace(cfg.OpenAIAPIKey) != "" {
			return msg + ". You can also switch to `--backend openai`."
		}
		return msg
	default:
		return msg
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
	return m.styles.SummaryPanel.Width(m.summaryW).Height(m.summaryH).Render(content)
}

func (m *model) executeSlashCommand(raw string) tea.Cmd {
	commandLine := strings.TrimSpace(strings.TrimPrefix(raw, "/"))
	parts := strings.Fields(commandLine)
	if len(parts) == 0 {
		m.state = StateViewing
		m.followInput.SetValue("")
		m.followInput.Blur()
		return m.flash("Available: /backend /mode /model /depth /results /show /help", "warning")
	}

	command := strings.ToLower(parts[0])
	args := parts[1:]

	switch command {
	case "help":
		m.followInput.SetValue("")
		return tea.Batch(m.followInput.Focus(), m.flash("Commands: /backend <ollama|openai>, /mode|/format <concise|learning|explanatory|oneliner>, /model <name>, /depth <basic|advanced>, /results <n>, /copy, /show", "success"))
	case "show", "status":
		m.followInput.SetValue("")
		return tea.Batch(m.followInput.Focus(), m.flash(m.sessionSummary(), "success"))
	case "copy":
		if strings.TrimSpace(m.composeTranscript()) == "" || len(m.turns) == 0 {
			return m.failSlashCommand("Nothing to copy yet")
		}
		m.followInput.SetValue("")
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
	case "mode", "format":
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
	m.searchProvider = searchProvider
	m.llmProvider = llmProvider
	m.orchestrator = NewOrchestrator(searchProvider, llmProvider, cfg.MaxResults, cfg.OutputFormat)

	m.followInput.SetValue("")
	m.refreshViewport(false)
	return tea.Batch(m.followInput.Focus(), m.flash(successText+" · "+m.sessionSummary(), "success"))
}

func (m *model) failSlashCommand(text string) tea.Cmd {
	return tea.Batch(m.followInput.Focus(), m.flash(text, "error"))
}

func (m *model) sessionSummary() string {
	return fmt.Sprintf(
		"backend=%s model=%s format=%s depth=%s results=%d",
		m.config.LLMBackend,
		activeModel(m.config),
		m.config.OutputFormat,
		m.config.SearchDepth,
		m.config.MaxResults,
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
			Rendered:    ui.RenderCodeBlock(m.styles, max(10, m.viewport.Width-2), language, content),
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

func (m *model) isSlashInput() bool {
	return strings.HasPrefix(strings.TrimSpace(m.followInput.Value()), "/")
}

func (m *model) tryAutocompleteSlashCommand() bool {
	if !m.isSlashInput() {
		return false
	}

	value := strings.TrimSpace(m.followInput.Value())
	if strings.Contains(value, " ") {
		return false
	}

	matches := m.filteredSlashCommands(value)
	if len(matches) == 0 {
		return false
	}

	m.followInput.SetValue("/" + matches[0].Name + " ")
	return true
}

func (m *model) filteredSlashCommands(prefix string) []slashCommandSpec {
	query := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(prefix, "/")))
	all := []slashCommandSpec{
		{Name: "backend", Usage: "/backend <ollama|openai>", Description: "switch the active LLM backend for this session"},
		{Name: "mode", Usage: "/mode <concise|learning|explanatory|oneliner>", Description: "change the answer style without restarting"},
		{Name: "format", Usage: "/format <concise|learning|explanatory|oneliner>", Description: "alias for /mode"},
		{Name: "model", Usage: "/model <name>", Description: "set the model for the active backend"},
		{Name: "depth", Usage: "/depth <basic|advanced>", Description: "change Tavily search depth"},
		{Name: "results", Usage: "/results <n>", Description: "set how many Tavily results to inject into context"},
		{Name: "copy", Usage: "/copy", Description: "copy the full chat history including follow-ups"},
		{Name: "show", Usage: "/show", Description: "show the active session configuration"},
		{Name: "help", Usage: "/help", Description: "list available slash commands"},
	}

	if query == "" {
		return all
	}

	filtered := make([]slashCommandSpec, 0, len(all))
	for _, command := range all {
		if strings.HasPrefix(command.Name, query) {
			filtered = append(filtered, command)
		}
	}
	return filtered
}

func (m *model) renderSlashSuggestions() string {
	matches := m.filteredSlashCommands(strings.TrimSpace(m.followInput.Value()))
	innerWidth := max(0, m.contentW-4)

	lines := []string{m.styles.HorizontalRule(innerWidth, "Commands")}
	if len(matches) == 0 {
		lines = append(lines, m.styles.Dimmed.Width(innerWidth).Render("No command matches"))
	} else {
		limit := min(5, len(matches))
		for i := 0; i < limit; i++ {
			cmd := matches[i]
			header := m.styles.CodeLabel.Render(cmd.Usage)
			desc := m.styles.Dimmed.Render(cmd.Description)
			lines = append(lines, lipgloss.NewStyle().Width(innerWidth).Render(header+"  "+desc))
		}
		if len(matches) > limit {
			lines = append(lines, m.styles.Dimmed.Width(innerWidth).Render(fmt.Sprintf("+%d more", len(matches)-limit)))
		}
		lines = append(lines, m.styles.Dimmed.Width(innerWidth).Render("Tab autocomplete · Enter run command"))
	}

	for len(lines) < max(1, m.sourcesH-2) {
		lines = append(lines, strings.Repeat(" ", innerWidth))
	}

	return m.styles.SourcesPanelFocus.Width(m.contentW).Height(m.sourcesH).Render(strings.Join(lines, "\n"))
}

func emitQuery(query string, followUp bool) tea.Cmd {
	return func() tea.Msg {
		return startQueryMsg{Query: query, FollowUp: followUp}
	}
}

func searchCmd(ctx context.Context, provider searchpkg.SearchProvider, query string, maxResults, requestID int) tea.Cmd {
	return func() tea.Msg {
		results, err := provider.Search(ctx, query, maxResults)
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
