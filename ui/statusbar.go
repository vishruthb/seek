package ui

import "github.com/charmbracelet/lipgloss"

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

	leftStyle := m.styles.StatusHint.MaxWidth(width)
	rightStyle := m.styles.StatusMeta.MaxWidth(width)

	leftRendered := leftStyle.Render(left)
	rightRendered := rightStyle.Render(right)

	row := lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftRendered,
		lipgloss.NewStyle().Width(max(0, width-lipgloss.Width(leftRendered)-lipgloss.Width(rightRendered))).Render(""),
		rightRendered,
	)

	return m.styles.StatusBar.Width(width).Render(row)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
