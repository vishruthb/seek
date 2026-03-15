package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const seekLogo = "" +
	" SSSSS  EEEEE  EEEEE  K   K\n" +
	"SS      EE     EE     K  K \n" +
	" SSSS   EEEE   EEEE   KKK  \n" +
	"    SS  EE     EE     K  K \n" +
	"SSSSS   EEEEE  EEEEE  K   K"

type CodePreview struct {
	Index    int
	Language string
	Preview  string
	Lines    int
}

type ComposerState struct {
	Backend   string
	Model     string
	Format    string
	Depth     string
	Results   int
	LastQuery string
	Draft     string
}

func RenderPlaceholder(styles Styles, width, height int, body string) string {
	block := styles.Dimmed.Width(max(0, width)).Render(body)
	return lipgloss.Place(width, height, lipgloss.Left, lipgloss.Center, block)
}

func RenderSplash(styles Styles, width, height int, format, backend string) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	content := strings.Join([]string{
		styles.SplashLogo.Render(seekLogo),
		"",
		styles.SplashTagline.Render("Search grounded answers without leaving the terminal."),
		styles.SplashMeta.Render(fmt.Sprintf("format: %s  ·  backend: %s", format, backend)),
		"",
		styles.SplashHint.Render("Type a query below or use /help"),
	}, "\n")

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}

func RenderCodeSelection(styles Styles, width, height int, previews []CodePreview, selected int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	lines := []string{styles.HorizontalRule(width, "Code Blocks")}
	for _, preview := range previews {
		card := styles.CodeCard
		if preview.Index == selected {
			card = styles.CodeCardSelected
		}

		title := fmt.Sprintf("[%d] %s · %d lines", preview.Index+1, safeLanguage(preview.Language), preview.Lines)
		body := styles.CodeLabel.Render(title) + "\n" + preview.Preview
		lines = append(lines, card.Width(max(0, width-2)).Render(body))
	}

	content := strings.Join(lines, "\n")
	return lipgloss.NewStyle().Width(width).Height(height).Render(content)
}

func RenderCodeBlock(styles Styles, width int, language, content string) string {
	if width <= 0 {
		return strings.TrimRight(content, "\n")
	}

	body := styles.CodeBody.Render(strings.TrimRight(content, "\n"))
	label := styles.CodeBlockHeader.Render(strings.ToUpper(safeLanguage(language)))
	headWidth := max(0, width-lipgloss.Width(label)-2)
	header := lipgloss.JoinHorizontal(
		lipgloss.Center,
		label,
		styles.Dimmed.Render(strings.Repeat("─", headWidth)),
	)

	frameWidth := max(0, width)
	return styles.CodeBlockFrame.Width(frameWidth).Render(header + "\n" + body)
}

func RenderComposer(styles Styles, width, height int, state ComposerState) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	innerWidth := max(0, width-4)
	lines := []string{
		styles.CodeLabel.Render(fmt.Sprintf("backend %s", state.Backend)) +
			"  " + styles.CodeLabel.Render(fmt.Sprintf("format %s", state.Format)) +
			"  " + styles.CodeLabel.Render(fmt.Sprintf("depth %s", state.Depth)) +
			"  " + styles.CodeLabel.Render(fmt.Sprintf("results %d", state.Results)),
	}

	if strings.TrimSpace(state.Model) != "" {
		lines = append(lines, styles.ComposerMeta.Width(innerWidth).Render("model: "+state.Model))
	}
	if strings.TrimSpace(state.LastQuery) != "" {
		lines = append(lines, styles.ComposerHint.Width(innerWidth).Render("continuing from: "+truncate(state.LastQuery, innerWidth-18)))
	}
	if strings.TrimSpace(state.Draft) != "" {
		lines = append(lines, styles.ComposerMeta.Width(innerWidth).Render("draft: "+truncate(state.Draft, innerWidth-8)))
	}

	lines = append(lines,
		"",
		styles.ComposerHint.Width(innerWidth).Render("Enter submit · Esc cancel · /backend, /mode, /model, /depth, /results, /copy"),
	)

	body := strings.Join(lines, "\n")
	return styles.ComposerPanel.Width(width).Height(height).Render(body)
}

func safeLanguage(lang string) string {
	if strings.TrimSpace(lang) == "" {
		return "plain text"
	}
	return lang
}

func truncate(value string, width int) string {
	value = strings.TrimSpace(value)
	if width <= 0 || lipgloss.Width(value) <= width {
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
