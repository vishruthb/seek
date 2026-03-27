package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const seekLogo = "" +
	"███████╗███████╗███████╗██╗ ██╗\n" +
	"██╔════╝██╔════╝██╔════╝██║ ██╔╝\n" +
	"███████╗█████╗ █████╗ █████╔╝\n" +
	"╚════██║██╔══╝ ██╔══╝ ██╔═██╗\n" +
	"███████║███████╗███████╗██║ ██╗\n" +
	"╚══════╝╚══════╝╚══════╝╚═╝ ╚═╝"

const compactSeekLogo = "" +
	"███████╗███████╗███████╗██╗ ██╗\n" +
	"╚════██║██╔════╝██╔════╝██║ ██╔╝\n" +
	"███████║███████╗███████╗█████╔╝ "

type CodePreview struct {
	Index    int
	Language string
	Preview  string
	Lines    int
}

type ComposerState struct {
	LastQuery string
	Draft     string
}

func RenderPlaceholder(styles Styles, width, height int, body string) string {
	block := styles.Dimmed.Width(max(0, width)).Render(body)
	return lipgloss.Place(width, height, lipgloss.Left, lipgloss.Center, block)
}

func PreferredSplashHeight(width int) int {
	if width <= 0 {
		return 3
	}
	logo := selectSplashLogo(width, 0)
	return strings.Count(logo, "\n") + 1 + 2
}

func RenderSplash(styles Styles, width, height int, _ string, _ string) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	logo := normalizeBlockWidth(selectSplashLogo(width, height))
	logoHeight := strings.Count(logo, "\n") + 1
	spareLines := max(0, height-(logoHeight+2))

	lines := []string{styles.SplashLogo.Width(width).Align(lipgloss.Left).Render(logo)}
	if spareLines > 0 {
		lines = append(lines, "")
		spareLines--
	}
	lines = append(lines, styles.SplashTagline.Width(width).Align(lipgloss.Left).Render("Search grounded answers without leaving the terminal."))
	if spareLines > 0 {
		lines = append(lines, "")
	}
	lines = append(lines, styles.SplashHint.Width(width).Align(lipgloss.Left).Render("Start typing below or use /"))

	content := strings.Join(lines, "\n")
	return lipgloss.Place(width, height, lipgloss.Left, lipgloss.Bottom, content)
}

func RenderWelcomeHint(styles Styles, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	line := styles.ComposerHint.Width(width).Align(lipgloss.Center).Render("/ for commands")
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, line)
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
		lines = append(lines, fitPanelWidth(card, width).Render(body))
	}

	content := strings.Join(lines, "\n")
	return lipgloss.NewStyle().Width(width).Height(height).Render(content)
}

func RenderCodeBlock(styles Styles, width int, language, content string) string {
	if width <= 0 {
		return strings.TrimRight(content, "\n")
	}

	innerWidth := max(0, width-styles.CodeBlockFrame.GetHorizontalFrameSize())
	if innerWidth <= 1 {
		return fitPanelWidth(styles.CodeBlockFrame, width).Render(truncateWidth(strings.TrimRight(content, "\n"), max(1, width-styles.CodeBlockFrame.GetHorizontalFrameSize())))
	}

	body := styles.CodeBody.
		Width(innerWidth).
		MaxWidth(innerWidth).
		Render(strings.TrimRight(content, "\n"))
	labelText := truncateWidth(strings.ToUpper(safeLanguage(language)), max(1, innerWidth-2))
	label := styles.CodeBlockHeader.Render(labelText)
	headWidth := max(0, innerWidth-lipgloss.Width(label))
	header := lipgloss.JoinHorizontal(
		lipgloss.Center,
		label,
		styles.Dimmed.Render(strings.Repeat("─", headWidth)),
	)

	return fitPanelWidth(styles.CodeBlockFrame, width).Render(header + "\n" + body)
}

func RenderComposer(styles Styles, width, height int, state ComposerState) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	innerWidth := max(0, width-4)
	lines := []string{styles.CodeLabel.Width(innerWidth).Render("start typing...")}
	if strings.TrimSpace(state.LastQuery) != "" {
		lines = append(lines, styles.ComposerMeta.Width(innerWidth).Render("follow-up context: "+truncate(state.LastQuery, innerWidth-19)))
	}
	if strings.TrimSpace(state.Draft) != "" {
		lines = append(lines, styles.ComposerMeta.Width(innerWidth).Render("draft: "+truncate(state.Draft, innerWidth-8)))
	}
	lines = append(lines, styles.ComposerHint.Width(innerWidth).Render("use / for commands or @[file] to attach local code"))

	body := strings.Join(lines, "\n")
	return fitPanel(styles.ComposerPanel, width, height).Render(body)
}

func safeLanguage(lang string) string {
	if strings.TrimSpace(lang) == "" {
		return "plain text"
	}
	return lang
}

func truncate(value string, width int) string {
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

func selectSplashLogo(width, height int) string {
	for _, candidate := range []string{seekLogo, compactSeekLogo} {
		if width < lipgloss.Width(candidate) {
			continue
		}
		if height > 0 {
			requiredHeight := strings.Count(candidate, "\n") + 1 + 2
			if height < requiredHeight {
				continue
			}
		}
		return candidate
	}
	return "SEEK"
}
