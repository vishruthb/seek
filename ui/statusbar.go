package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type StatusBarModel struct {
	styles Styles
}

func NewStatusBar(styles Styles) StatusBarModel {
	return StatusBarModel{styles: styles}
}

func (m StatusBarModel) View(left, right string, width int) string {
	if width <= 0 {
		return ""
	}

	if strings.TrimSpace(left) == "" {
		return m.styles.StatusBar.Width(width).Render(m.styles.StatusMeta.Render(truncateWidth(strings.TrimSpace(right), width)))
	}
	if strings.TrimSpace(right) == "" {
		return m.styles.StatusBar.Width(width).Render(m.styles.StatusHint.Render(truncateWidth(strings.TrimSpace(left), width)))
	}

	rightBudget := max(1, min(width-1, (width*3)/5))
	leftBudget := max(1, width-rightBudget-1)

	leftText := truncateWidth(strings.TrimSpace(left), leftBudget)
	leftRendered := m.styles.StatusHint.Render(leftText)

	rightText := truncateWidth(strings.TrimSpace(right), width-lipgloss.Width(leftRendered)-1)
	rightRendered := m.styles.StatusMeta.Render(rightText)
	remaining := max(0, width-lipgloss.Width(leftRendered)-lipgloss.Width(rightRendered))

	row := leftRendered + strings.Repeat(" ", remaining) + rightRendered
	return m.styles.StatusBar.Width(width).Render(row)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
