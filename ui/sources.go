package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
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
	innerWidth := max(0, m.width-4)
	lines := []string{m.styles.HorizontalRule(innerWidth, "Sources")}

	visibleRows := max(1, m.height-3)
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

			space := max(0, innerWidth-lipgloss.Width(label)-2)
			line := label + strings.Repeat(" ", space) + "↗"
			lines = append(lines, style.Width(innerWidth).Render(line))
		}
	}

	for len(lines) < max(1, m.height-2) {
		lines = append(lines, strings.Repeat(" ", innerWidth))
	}

	panel := m.styles.SourcesPanel
	if m.focused {
		panel = m.styles.SourcesPanelFocus
	}

	return panel.Width(m.width).Height(m.height).Render(strings.Join(lines, "\n"))
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

	visibleRows := max(1, m.height-3)
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
