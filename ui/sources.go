package ui

import (
	"fmt"
	"strings"
)

type SourceItem struct {
	Title  string
	Domain string
	URL    string
}

type SourcesModel struct {
	items    []SourceItem
	selected int
	offset   int
	width    int
	height   int
	focused  bool
	styles   Styles
}

func NewSources(styles Styles) SourcesModel {
	return SourcesModel{styles: styles}
}

func (m *SourcesModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.clamp()
}

func (m *SourcesModel) SetItems(items []SourceItem) {
	m.items = append([]SourceItem(nil), items...)
	m.selected = 0
	m.offset = 0
	m.clamp()
}

func (m *SourcesModel) Focus() {
	m.focused = true
}

func (m *SourcesModel) Blur() {
	m.focused = false
}

func (m *SourcesModel) Next() {
	if len(m.items) == 0 {
		return
	}
	if m.selected < len(m.items)-1 {
		m.selected++
	}
	m.clamp()
}

func (m *SourcesModel) Prev() {
	if len(m.items) == 0 {
		return
	}
	if m.selected > 0 {
		m.selected--
	}
	m.clamp()
}

func (m SourcesModel) Selected() (SourceItem, bool) {
	if len(m.items) == 0 {
		return SourceItem{}, false
	}
	return m.items[m.selected], true
}

func (m SourcesModel) View() string {
	panel := m.styles.SourcesPanel
	if m.focused {
		panel = m.styles.SourcesPanelFocus
	}

	innerWidth := max(0, m.width-panel.GetHorizontalFrameSize())
	lines := []string{m.styles.HorizontalRule(innerWidth, "Sources")}

	visibleRows := max(1, m.height-panel.GetVerticalFrameSize()-1)
	if len(m.items) == 0 {
		lines = append(lines, m.styles.Dimmed.Width(innerWidth).Render("No sources cited"))
	} else {
		end := min(len(m.items), m.offset+visibleRows)
		for idx := m.offset; idx < end; idx++ {
			item := m.items[idx]
			prefix := "  "
			style := m.styles.SourceLine
			if idx == m.selected {
				prefix = "› "
				style = m.styles.SourceSelected
			}

			title := strings.TrimSpace(item.Title)
			domain := strings.TrimSpace(item.Domain)
			if title == "" {
				title = domain
			}

			label := fmt.Sprintf("%s[%d] %s", prefix, idx+1, title)
			if domain != "" && !strings.EqualFold(title, domain) {
				label += " — " + domain
			}

			if innerWidth <= 1 {
				lines = append(lines, style.Width(innerWidth).MaxWidth(innerWidth).Render(truncateWidth(label, innerWidth)))
				continue
			}

			lineWidth := max(0, innerWidth-2)
			line := padRightWidth(label, lineWidth) + " ↗"
			lines = append(lines, style.Width(innerWidth).MaxWidth(innerWidth).Render(line))
		}
	}

	for len(lines) < max(1, m.height-panel.GetVerticalFrameSize()) {
		lines = append(lines, strings.Repeat(" ", innerWidth))
	}

	return fitPanel(panel, m.width, m.height).Render(strings.Join(lines, "\n"))
}

func (m *SourcesModel) clamp() {
	if len(m.items) == 0 {
		m.selected = 0
		m.offset = 0
		return
	}

	if m.selected < 0 {
		m.selected = 0
	}
	if m.selected >= len(m.items) {
		m.selected = len(m.items) - 1
	}

	visibleRows := max(1, m.height-m.styles.SourcesPanel.GetVerticalFrameSize()-1)
	if m.selected < m.offset {
		m.offset = m.selected
	}
	if m.selected >= m.offset+visibleRows {
		m.offset = m.selected - visibleRows + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
