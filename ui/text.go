package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func fitPanel(style lipgloss.Style, width, height int) lipgloss.Style {
	if width >= 0 {
		style = style.Width(max(0, width-style.GetHorizontalFrameSize()))
	}
	if height >= 0 {
		style = style.Height(max(0, height-style.GetVerticalFrameSize()))
	}
	return style
}

func fitPanelWidth(style lipgloss.Style, width int) lipgloss.Style {
	return fitPanel(style, width, -1)
}

func fitPanelHeight(style lipgloss.Style, height int) lipgloss.Style {
	return fitPanel(style, -1, height)
}

func truncateWidth(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}
	if width == 1 {
		return "…"
	}

	target := width - 1
	var b strings.Builder
	currentWidth := 0
	for _, r := range value {
		runeWidth := lipgloss.Width(string(r))
		if currentWidth+runeWidth > target {
			break
		}
		b.WriteRune(r)
		currentWidth += runeWidth
	}
	return b.String() + "…"
}

func padRightWidth(value string, width int) string {
	value = truncateWidth(value, width)
	padding := max(0, width-lipgloss.Width(value))
	return value + strings.Repeat(" ", padding)
}

func padLeftWidth(value string, width int) string {
	value = truncateWidth(value, width)
	padding := max(0, width-lipgloss.Width(value))
	return strings.Repeat(" ", padding) + value
}
