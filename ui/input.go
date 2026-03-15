package ui

import "github.com/charmbracelet/bubbles/textinput"

func NewFollowUpInput(styles Styles) textinput.Model {
	return newInput("Follow-up: ", styles)
}

func NewSearchInput(styles Styles) textinput.Model {
	return newInput("/ ", styles)
}

func RenderInput(styles Styles, input textinput.Model, width int) string {
	if width <= 0 {
		return ""
	}
	return styles.InputBar.Width(width).Render(" " + input.View())
}

func newInput(prompt string, styles Styles) textinput.Model {
	input := textinput.New()
	input.Prompt = prompt
	input.PromptStyle = styles.InputPrompt
	input.TextStyle = styles.InputText
	input.PlaceholderStyle = styles.Dimmed
	input.CursorStyle = styles.InputCursor
	input.CharLimit = 600
	return input
}
