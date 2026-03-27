package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"seek/ui"
)

const (
	startupSuggestionPanelHeight = 7
	activeSuggestionPanelHeight  = 8
)

type inputSuggestionMode int

const (
	inputSuggestionNone inputSuggestionMode = iota
	inputSuggestionSlash
	inputSuggestionSlashArg
	inputSuggestionAttachment
)

type inputSuggestion struct {
	Title  string
	Detail string
	Value  string
}

func (m *model) setFollowInputValue(value string) {
	m.followInput.SetValue(value)
	m.refreshInputSuggestions()
}

func (m *model) shouldRenderInputSuggestions() bool {
	return m.state == StateInput && m.inputSuggestionMode != inputSuggestionNone
}

func (m *model) selectedInputSuggestion() (inputSuggestion, bool) {
	if len(m.inputSuggestions) == 0 || m.inputSuggestionIndex < 0 || m.inputSuggestionIndex >= len(m.inputSuggestions) {
		return inputSuggestion{}, false
	}
	return m.inputSuggestions[m.inputSuggestionIndex], true
}

func (m *model) refreshInputSuggestions() {
	mode, key, items := m.computeInputSuggestions()
	if mode != m.inputSuggestionMode || key != m.inputSuggestionKey {
		m.inputSuggestionIndex = 0
		m.inputSuggestionOffset = 0
		m.inputSuggestionsFocused = false
	}

	m.inputSuggestionMode = mode
	m.inputSuggestionKey = key
	m.inputSuggestions = items

	if mode == inputSuggestionNone {
		m.inputSuggestionIndex = 0
		m.inputSuggestionOffset = 0
		m.inputSuggestionsFocused = false
		return
	}

	m.clampInputSuggestions()
}

func (m *model) computeInputSuggestions() (inputSuggestionMode, string, []inputSuggestion) {
	raw := m.followInput.Value()
	trimmed := strings.TrimSpace(raw)

	if strings.HasPrefix(trimmed, "/") {
		command, argPrefix, hasArgSlot := parseSlashInput(raw)
		if hasArgSlot {
			items := m.slashArgumentSuggestions(command, argPrefix)
			if len(items) > 0 {
				return inputSuggestionSlashArg, "slash-arg:" + command + ":" + strings.ToLower(strings.TrimSpace(argPrefix)), items
			}
		}

		if !strings.Contains(trimmed, " ") {
			matches := m.filteredSlashCommands(trimmed)
			items := make([]inputSuggestion, 0, len(matches))
			for _, match := range matches {
				items = append(items, inputSuggestion{
					Title:  "/" + match.Name,
					Detail: match.Description,
					Value:  match.Name,
				})
			}
			return inputSuggestionSlash, "slash:" + strings.ToLower(trimmed), items
		}
	}

	completion, ok := activeAttachmentCompletion(raw, m.followInput.Position())
	if !ok {
		return inputSuggestionNone, "", nil
	}

	m.ensureLocalFiles()
	return inputSuggestionAttachment, "attach:" + strings.ToLower(filepath.ToSlash(strings.TrimSpace(completion.Prefix))), m.fileSuggestions(completion.Prefix)
}

func (m *model) ensureLocalFiles() {
	if m.localFilesLoaded {
		return
	}

	files, err := listLocalFiles(m.workingDir)
	m.localFiles = files
	m.localFilesErr = err
	m.localFilesLoaded = true
}

func (m *model) fileSuggestions(prefix string) []inputSuggestion {
	if m.localFilesErr != nil {
		return nil
	}

	prefix = strings.ToLower(filepath.ToSlash(strings.TrimSpace(prefix)))
	if prefix == "" {
		items := make([]inputSuggestion, 0, len(m.localFiles))
		for _, file := range m.localFiles {
			items = append(items, inputSuggestion{Title: "@[" + file + "]", Value: file})
		}
		return items
	}

	prefixMatches := make([]inputSuggestion, 0, len(m.localFiles))
	containsMatches := make([]inputSuggestion, 0, len(m.localFiles))
	for _, file := range m.localFiles {
		lower := strings.ToLower(file)
		base := strings.ToLower(filepath.Base(file))
		item := inputSuggestion{Title: "@[" + file + "]", Value: file}
		switch {
		case strings.HasPrefix(lower, prefix), strings.HasPrefix(base, prefix):
			prefixMatches = append(prefixMatches, item)
		case strings.Contains(lower, prefix), strings.Contains(base, prefix):
			containsMatches = append(containsMatches, item)
		}
	}

	return append(prefixMatches, containsMatches...)
}

func (m *model) currentSuggestionPanelHeight() int {
	if len(m.turns) == 0 {
		return startupSuggestionPanelHeight
	}
	return activeSuggestionPanelHeight
}

func (m *model) suggestionVisibleRows() int {
	panelHeight := m.sourcesH
	if panelHeight <= 0 {
		panelHeight = m.currentSuggestionPanelHeight()
	}
	return max(1, panelHeight-4)
}

func (m *model) clampInputSuggestions() {
	if len(m.inputSuggestions) == 0 {
		m.inputSuggestionIndex = 0
		m.inputSuggestionOffset = 0
		return
	}

	m.inputSuggestionIndex = max(0, min(m.inputSuggestionIndex, len(m.inputSuggestions)-1))
	visibleRows := m.suggestionVisibleRows()
	if m.inputSuggestionIndex < m.inputSuggestionOffset {
		m.inputSuggestionOffset = m.inputSuggestionIndex
	}
	if m.inputSuggestionIndex >= m.inputSuggestionOffset+visibleRows {
		m.inputSuggestionOffset = m.inputSuggestionIndex - visibleRows + 1
	}
	if m.inputSuggestionOffset < 0 {
		m.inputSuggestionOffset = 0
	}
}

func (m *model) moveInputSuggestion(delta int) {
	if len(m.inputSuggestions) == 0 {
		return
	}

	m.inputSuggestionsFocused = true
	m.inputSuggestionIndex += delta
	if m.inputSuggestionIndex < 0 {
		m.inputSuggestionIndex = 0
	}
	if m.inputSuggestionIndex >= len(m.inputSuggestions) {
		m.inputSuggestionIndex = len(m.inputSuggestions) - 1
	}
	m.clampInputSuggestions()
}

func (m *model) acceptSelectedInputSuggestion() bool {
	suggestion, ok := m.selectedInputSuggestion()
	if !ok {
		return false
	}

	switch m.inputSuggestionMode {
	case inputSuggestionSlash:
		m.followInput.SetValue("/" + suggestion.Value + " ")
		m.followInput.CursorEnd()
	case inputSuggestionSlashArg:
		command, _, hasArgSlot := parseSlashInput(m.followInput.Value())
		if !hasArgSlot || command == "" {
			return false
		}
		m.followInput.SetValue("/" + command + " " + suggestion.Value)
		m.followInput.CursorEnd()
	case inputSuggestionAttachment:
		completion, ok := activeAttachmentCompletion(m.followInput.Value(), m.followInput.Position())
		if !ok {
			return false
		}
		next, cursor := insertAttachmentValue(m.followInput.Value(), completion, suggestion.Value)
		m.followInput.SetValue(next)
		m.followInput.SetCursor(cursor)
	default:
		return false
	}

	m.inputSuggestionsFocused = false
	m.refreshInputSuggestions()
	m.applyLayout()
	m.refreshViewport(false)
	return true
}

func (m *model) shouldAcceptSuggestionOnEnter() bool {
	if len(m.inputSuggestions) == 0 {
		return false
	}

	switch m.inputSuggestionMode {
	case inputSuggestionAttachment:
		return true
	case inputSuggestionSlash:
		value := strings.TrimSpace(m.followInput.Value())
		if value == "" || strings.Contains(value, " ") {
			return false
		}

		selected, ok := m.selectedInputSuggestion()
		if !ok {
			return false
		}

		typed := strings.TrimPrefix(value, "/")
		return typed == "" || !strings.EqualFold(typed, selected.Value)
	case inputSuggestionSlashArg:
		_, argPrefix, hasArgSlot := parseSlashInput(m.followInput.Value())
		if !hasArgSlot {
			return false
		}

		selected, ok := m.selectedInputSuggestion()
		if !ok {
			return false
		}

		return strings.TrimSpace(argPrefix) == "" || !strings.EqualFold(strings.TrimSpace(argPrefix), selected.Value)
	default:
		return false
	}
}

func (m *model) renderInputSuggestions() string {
	panel := m.styles.SourcesPanelFocus
	innerWidth := max(0, m.contentW-panel.GetHorizontalFrameSize())
	totalRows := max(2, m.sourcesH-panel.GetVerticalFrameSize())
	visibleRows := max(1, totalRows-2)

	title := "Suggestions"
	switch m.inputSuggestionMode {
	case inputSuggestionSlash:
		title = "Commands"
	case inputSuggestionSlashArg:
		title = "Values"
	case inputSuggestionAttachment:
		title = "Files"
	}

	header := m.styles.HorizontalRule(innerWidth, title)
	body := make([]string, 0, visibleRows)

	switch {
	case m.inputSuggestionMode == inputSuggestionAttachment && m.localFilesErr != nil:
		body = append(body, m.styles.WarningText.Width(innerWidth).Render("File suggestions unavailable: "+m.localFilesErr.Error()))
	case len(m.inputSuggestions) == 0:
		empty := "No suggestions"
		if m.inputSuggestionMode == inputSuggestionSlash {
			empty = "No command matches"
		} else if m.inputSuggestionMode == inputSuggestionSlashArg {
			empty = "No value matches"
		} else if m.inputSuggestionMode == inputSuggestionAttachment {
			empty = "No file matches"
		}
		body = append(body, m.styles.Dimmed.Width(innerWidth).Render(empty))
	default:
		end := min(len(m.inputSuggestions), m.inputSuggestionOffset+visibleRows)
		for idx := m.inputSuggestionOffset; idx < end; idx++ {
			suggestion := m.inputSuggestions[idx]
			style := m.styles.SourceLine
			if idx == m.inputSuggestionIndex {
				style = m.styles.SourceSelected
			}

			if m.inputSuggestionMode == inputSuggestionSlash || m.inputSuggestionMode == inputSuggestionSlashArg {
				body = append(body, renderSlashSuggestionRow(m.styles, style, innerWidth, idx == m.inputSuggestionIndex, suggestion.Title, suggestion.Detail))
				continue
			}

			prefix := "  "
			if idx == m.inputSuggestionIndex {
				prefix = "› "
			}
			body = append(body, style.Width(innerWidth).Render(truncateSuggestionLine(prefix+suggestion.Title, innerWidth)))
		}
	}

	for len(body) < visibleRows {
		body = append(body, strings.Repeat(" ", innerWidth))
	}

	footerParts := []string{}
	if len(m.inputSuggestions) > 0 {
		end := min(len(m.inputSuggestions), m.inputSuggestionOffset+visibleRows)
		footerParts = append(footerParts, fmt.Sprintf("%d-%d/%d", m.inputSuggestionOffset+1, end, len(m.inputSuggestions)))
	}
	if m.inputSuggestionsFocused {
		footerParts = append(footerParts, "↑↓/j/k navigate", "Enter accept", "Tab accept")
	} else {
		footerParts = append(footerParts, "↑↓ select", "Tab accept")
	}
	footer := m.styles.Dimmed.Width(innerWidth).Render(strings.Join(footerParts, " · "))

	lines := []string{header}
	lines = append(lines, body...)
	lines = append(lines, footer)

	return panel.
		Width(max(0, m.contentW-panel.GetHorizontalFrameSize())).
		Height(max(0, m.sourcesH-panel.GetVerticalFrameSize())).
		Render(strings.Join(lines, "\n"))
}

func parseSlashInput(raw string) (command string, argPrefix string, hasArgSlot bool) {
	trimmedLeft := strings.TrimLeft(raw, " \t")
	if !strings.HasPrefix(trimmedLeft, "/") {
		return "", "", false
	}

	withoutSlash := strings.TrimPrefix(trimmedLeft, "/")
	fields := strings.Fields(withoutSlash)
	if len(fields) == 0 {
		return "", "", false
	}

	command = strings.ToLower(strings.TrimSpace(fields[0]))
	if command == "" {
		return "", "", false
	}

	rest := strings.TrimPrefix(withoutSlash, fields[0])
	if rest == "" {
		return command, "", true
	}

	trimmedRest := strings.TrimLeft(rest, " \t")
	if trimmedRest == "" {
		return command, "", true
	}

	if strings.Contains(trimmedRest, " ") {
		return command, "", false
	}

	return command, trimmedRest, true
}

func (m *model) slashArgumentSuggestions(command, prefix string) []inputSuggestion {
	if command == "" {
		return nil
	}

	type option struct {
		value  string
		detail string
	}

	var options []option
	switch strings.ToLower(strings.TrimSpace(command)) {
	case "mode":
		options = []option{
			{value: "concise", detail: "tight default answer style"},
			{value: "learning", detail: "teach progressively with intuition first"},
			{value: "explanatory", detail: "fuller explanation with sections and tradeoffs"},
			{value: "oneliner", detail: "one or two sentences max"},
		}
	case "backend":
		options = []option{
			{value: "ollama", detail: "local answer backend"},
			{value: "openai", detail: "hosted OpenAI-compatible backend"},
		}
	case "depth":
		options = []option{
			{value: "basic", detail: "faster Tavily retrieval"},
			{value: "advanced", detail: "broader Tavily retrieval"},
		}
	case "context":
		options = []option{
			{value: "on", detail: "enable project context enrichment"},
			{value: "off", detail: "disable project context enrichment"},
		}
	}

	if len(options) == 0 {
		return nil
	}

	prefix = strings.ToLower(strings.TrimSpace(prefix))
	items := make([]inputSuggestion, 0, len(options))
	for _, option := range options {
		if prefix != "" && !strings.HasPrefix(option.value, prefix) {
			continue
		}
		items = append(items, inputSuggestion{
			Title:  option.value,
			Detail: option.detail,
			Value:  option.value,
		})
	}
	return items
}

func truncateSuggestionLine(value string, width int) string {
	value = strings.TrimSpace(value)
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}
	if width <= 1 {
		return value[:width]
	}
	runes := []rune(value)
	if len(runes) > width-1 {
		runes = runes[:width-1]
	}
	return string(runes) + "…"
}

func renderSlashSuggestionRow(styles ui.Styles, style lipgloss.Style, width int, selected bool, title, detail string) string {
	if width <= 0 {
		return ""
	}

	prefix := "  "
	if selected {
		prefix = "› "
	}

	left := prefix + strings.TrimSpace(title)
	if strings.TrimSpace(detail) == "" || width < 32 {
		return style.Width(width).MaxWidth(width).Render(truncateSuggestionLine(left, width))
	}

	titleWidth := min(max(12, width/4), 20)
	separator := " │ "
	leftWidth := max(1, titleWidth)
	rightWidth := max(1, width-leftWidth-lipgloss.Width(separator))
	leftCell := padSuggestionRight(left, leftWidth)
	rightCell := styles.SourceMeta.Render(padSuggestionRight(strings.TrimSpace(detail), rightWidth))
	row := leftCell + styles.Dimmed.Render(separator) + rightCell
	return style.Width(width).MaxWidth(width).Render(row)
}

func padSuggestionRight(value string, width int) string {
	value = truncateSuggestionLine(value, width)
	padding := max(0, width-lipgloss.Width(value))
	return value + strings.Repeat(" ", padding)
}
