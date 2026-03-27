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
	panel := m.styles.SourcesPanel
	if m.focused {
		panel = m.styles.SourcesPanelFocus
	}

	innerWidth := max(0, m.width-panel.GetHorizontalFrameSize())
	lines := []string{m.styles.HorizontalRule(innerWidth, "Sources")}

	visibleRows := m.visibleRows()
	if len(m.items) == 0 {
		lines = append(lines, m.styles.Dimmed.Width(innerWidth).Render("No sources cited"))
	} else {
		if header := renderSourceHeader(m.styles, innerWidth); header != "" {
			lines = append(lines, header)
		}
		end := min(len(m.items), m.offset+visibleRows)
		for idx := m.offset; idx < end; idx++ {
			item := m.items[idx]
			style := m.styles.SourceLine
			if idx == m.selected {
				style = m.styles.SourceSelected
			}

			title := strings.TrimSpace(item.Title)
			domain := strings.TrimSpace(item.Domain)
			if title == "" {
				title = item.URL
			}
			lines = append(lines, renderSourceRow(m.styles, style, innerWidth, idx == m.selected, idx+1, title, domain))
		}
	}

	for len(lines) < max(1, m.height-panel.GetVerticalFrameSize()) {
		lines = append(lines, strings.Repeat(" ", innerWidth))
	}

	return fitPanel(panel, m.width, m.height).Render(strings.Join(lines, "\n"))
}

func renderSourceRow(styles Styles, style lipgloss.Style, width int, selected bool, index int, title, domain string) string {
	if width <= 0 {
		return ""
	}

	prefix := "  "
	if selected {
		prefix = "› "
	}

	label := fmt.Sprintf("%s[%d] %s", prefix, index, strings.TrimSpace(title))
	if strings.TrimSpace(domain) == "" || strings.EqualFold(strings.TrimSpace(title), strings.TrimSpace(domain)) {
		return style.Width(width).MaxWidth(width).Render(truncateWidth(label, width))
	}

	if width < 28 {
		return style.Width(width).MaxWidth(width).Render(truncateWidth(label, width))
	}

	metaWidth := min(max(12, width/4), 24)
	separator := " │ "
	labelWidth := max(1, width-metaWidth-lipgloss.Width(separator))
	left := padRightWidth(label, labelWidth)
	site := styles.SourceMeta.Render(padRightWidth(strings.TrimSpace(domain), metaWidth))
	row := left + styles.Dimmed.Render(separator) + site
	return style.Width(width).MaxWidth(width).Render(row)
}

func renderSourceHeader(styles Styles, width int) string {
	if width < 28 {
		return ""
	}

	metaWidth := min(max(12, width/4), 24)
	separator := " │ "
	labelWidth := max(1, width-metaWidth-lipgloss.Width(separator))
	left := styles.Dimmed.Render(padRightWidth("Title", labelWidth))
	right := styles.Dimmed.Render(padRightWidth("Site", metaWidth))
	return left + styles.Dimmed.Render(separator) + right
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

	visibleRows := m.visibleRows()
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

func (m *SourcesModel) visibleRows() int {
	rows := max(1, m.height-m.styles.SourcesPanel.GetVerticalFrameSize()-1)
	innerWidth := max(0, m.width-m.styles.SourcesPanel.GetHorizontalFrameSize())
	if renderSourceHeader(m.styles, innerWidth) != "" {
		rows = max(1, rows-1)
	}
	return rows
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
