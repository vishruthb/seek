package ui

import (
	"encoding/json"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	ColorAccent    = "#d9f99d"
	ColorDim       = "#9cb89e"
	ColorText      = "#f3ffe8"
	ColorBorder    = "#6c8a69"
	ColorHighlight = "#b7e4c7"
	ColorError     = "#ffcad4"
	ColorSuccess   = "#d9f99d"
	ColorWarning   = "#ffe8a3"
	ColorCodeBg    = "#142018"
	ColorSourceSel = "#293a2b"
	ColorPanel     = "#1b241d"
	ColorBg        = "#111713"
	ColorInk       = "#162016"
)

type Palette struct {
	Background string
	Panel      string
	Accent     string
	Dim        string
	Text       string
	Border     string
	Highlight  string
	Error      string
	Success    string
	Warning    string
	CodeBg     string
	SourceSel  string
	Ink        string
}

type Styles struct {
	Name              string
	Palette           Palette
	AppFrame          lipgloss.Style
	HeaderBar         lipgloss.Style
	HeaderBrand       lipgloss.Style
	HeaderQuery       lipgloss.Style
	HeaderCounter     lipgloss.Style
	SummaryPanel      lipgloss.Style
	SourcesPanel      lipgloss.Style
	SourcesPanelFocus lipgloss.Style
	SourceLine        lipgloss.Style
	SourceSelected    lipgloss.Style
	Dimmed            lipgloss.Style
	StatusBar         lipgloss.Style
	StatusHint        lipgloss.Style
	StatusMeta        lipgloss.Style
	InputBar          lipgloss.Style
	InputPrompt       lipgloss.Style
	InputText         lipgloss.Style
	InputCursor       lipgloss.Style
	ErrorText         lipgloss.Style
	SuccessText       lipgloss.Style
	WarningText       lipgloss.Style
	Divider           lipgloss.Style
	SearchMatch       lipgloss.Style
	SearchCurrent     lipgloss.Style
	CodeCard          lipgloss.Style
	CodeCardSelected  lipgloss.Style
	CodeLabel         lipgloss.Style
	CodeBody          lipgloss.Style
	CodeBlockFrame    lipgloss.Style
	CodeBlockHeader   lipgloss.Style
	ComposerPanel     lipgloss.Style
	ComposerMeta      lipgloss.Style
	ComposerHint      lipgloss.Style
	Spinner           lipgloss.Style
	SplashLogo        lipgloss.Style
	SplashTagline     lipgloss.Style
	SplashMeta        lipgloss.Style
	SplashHint        lipgloss.Style
}

func LoadTheme(name string) Styles {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		name = "pastel"
	}

	palette := paletteFor(name)

	return Styles{
		Name:    name,
		Palette: palette,
		AppFrame: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(palette.Highlight)),
		HeaderBar: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Ink)).
			Background(lipgloss.Color(palette.Highlight)),
		HeaderBrand: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Ink)).
			Bold(true),
		HeaderQuery: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#27402b")),
		HeaderCounter: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Ink)).
			Bold(true),
		SummaryPanel: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Text)).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(palette.Highlight)).
			Padding(0, 1),
		SourcesPanel: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(palette.Border)).
			Padding(0, 1),
		SourcesPanelFocus: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(palette.Highlight)).
			Padding(0, 1),
		SourceLine: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Text)),
		SourceSelected: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Text)).
			Background(lipgloss.Color(palette.SourceSel)).
			Bold(true),
		Dimmed: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Dim)),
		StatusBar: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Ink)).
			Background(lipgloss.Color(palette.Highlight)),
		StatusHint: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Ink)),
		StatusMeta: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#2f4a32")),
		InputBar: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Text)).
			Border(lipgloss.NormalBorder(), true, false, false, false).
			BorderForeground(lipgloss.Color(palette.Highlight)).
			Padding(0, 1),
		InputPrompt: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Accent)).
			Bold(true),
		InputText: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Text)),
		InputCursor: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Accent)),
		ErrorText: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Error)).
			Bold(true),
		SuccessText: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Success)).
			Bold(true),
		WarningText: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Warning)).
			Bold(true),
		Divider: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Border)),
		SearchMatch: lipgloss.NewStyle().
			Background(lipgloss.Color("#2c3d2e")).
			Foreground(lipgloss.Color(palette.Text)),
		SearchCurrent: lipgloss.NewStyle().
			Background(lipgloss.Color(palette.Accent)).
			Foreground(lipgloss.Color(palette.Ink)).
			Bold(true),
		CodeCard: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(palette.Highlight)).
			Padding(0, 1),
		CodeCardSelected: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(palette.Highlight)).
			Padding(0, 1),
		CodeLabel: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Accent)).
			Bold(true),
		CodeBody: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Text)),
		CodeBlockFrame: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(palette.Highlight)).
			Padding(0, 1),
		CodeBlockHeader: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Accent)).
			Bold(true).
			Padding(0, 1),
		ComposerPanel: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(palette.Highlight)).
			Padding(0, 1),
		ComposerMeta: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Text)),
		ComposerHint: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Dim)),
		Spinner: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Accent)),
		SplashLogo: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Highlight)).
			Bold(true),
		SplashTagline: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Text)).
			Bold(true),
		SplashMeta: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Dim)),
		SplashHint: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Accent)).
			Bold(true),
	}
}

func (s Styles) HorizontalRule(width int, label string) string {
	if width <= 0 {
		return ""
	}
	label = strings.TrimSpace(label)
	if label == "" {
		return s.Divider.Render(strings.Repeat("─", width))
	}
	prefix := label + " "
	if len(prefix) >= width {
		return s.Divider.Render(prefix[:width])
	}
	return s.Divider.Render(prefix + strings.Repeat("─", width-len(prefix)))
}

func (s Styles) GlamourJSON() []byte {
	style := map[string]any{
		"document": map[string]any{
			"color":        s.Palette.Text,
			"indent":       0,
			"block_prefix": "",
			"block_suffix": "",
			"margin":       0,
		},
		"paragraph": map[string]any{
			"color":  s.Palette.Text,
			"margin": 1,
		},
		"heading": map[string]any{
			"color":        s.Palette.Highlight,
			"bold":         true,
			"block_suffix": "\n",
		},
		"h1": map[string]any{
			"prefix": "## ",
			"margin": 1,
			"color":  s.Palette.Accent,
			"bold":   true,
		},
		"h2": map[string]any{
			"prefix": "### ",
			"margin": 1,
			"color":  s.Palette.Accent,
			"bold":   true,
		},
		"h3": map[string]any{
			"prefix": "#### ",
			"margin": 1,
			"color":  s.Palette.Highlight,
			"bold":   true,
		},
		"list": map[string]any{
			"color":        s.Palette.Text,
			"level_indent": 4,
		},
		"item": map[string]any{
			"block_prefix": "• ",
			"color":        s.Palette.Text,
		},
		"enumeration": map[string]any{
			"block_prefix": ". ",
			"color":        s.Palette.Accent,
			"bold":         true,
		},
		"link": map[string]any{
			"color":     s.Palette.Accent,
			"underline": true,
		},
		"link_text": map[string]any{
			"color":     s.Palette.Accent,
			"underline": true,
		},
		"code": map[string]any{
			"block_prefix": "`",
			"block_suffix": "`",
			"color":        s.Palette.Accent,
		},
		"code_block": map[string]any{
			"color":  s.Palette.Text,
			"margin": 1,
		},
		"block_quote": map[string]any{
			"color":        s.Palette.Dim,
			"indent":       1,
			"indent_token": "│ ",
		},
		"hr": map[string]any{
			"color":  s.Palette.Border,
			"format": "\n────────────────────────────────\n",
		},
		"table": map[string]any{
			"margin": 1,
		},
		"strong":        map[string]any{"bold": true},
		"emph":          map[string]any{"italic": true},
		"strikethrough": map[string]any{"crossed_out": true},
	}

	data, _ := json.Marshal(style)
	return data
}

func paletteFor(name string) Palette {
	switch name {
	case "light":
		return Palette{
			Background: "#f7fff1",
			Panel:      "#fbfff8",
			Accent:     "#84cc16",
			Dim:        "#6e866f",
			Text:       "#1d2b1f",
			Border:     "#b9d2b2",
			Highlight:  "#d9f99d",
			Error:      "#f28482",
			Success:    "#84cc16",
			Warning:    "#f7c873",
			CodeBg:     "#eef9e7",
			SourceSel:  "#ecf7dc",
			Ink:        "#1d2b1f",
		}
	case "dracula":
		return Palette{
			Background: "#121814",
			Panel:      "#1a241d",
			Accent:     "#d9f99d",
			Dim:        "#9db49a",
			Text:       "#f5ffe9",
			Border:     "#698565",
			Highlight:  "#b7e4c7",
			Error:      "#ffcad4",
			Success:    "#d9f99d",
			Warning:    "#ffe8a3",
			CodeBg:     "#142018",
			SourceSel:  "#293a2b",
			Ink:        "#162016",
		}
	case "tokyo-night":
		return Palette{
			Background: "#111713",
			Panel:      "#1a241d",
			Accent:     "#d9f99d",
			Dim:        "#99b59d",
			Text:       "#f3ffe8",
			Border:     "#678164",
			Highlight:  "#b7e4c7",
			Error:      "#ffcad4",
			Success:    "#d9f99d",
			Warning:    "#ffe8a3",
			CodeBg:     "#142018",
			SourceSel:  "#293a2b",
			Ink:        "#162016",
		}
	case "dark", "pastel":
		fallthrough
	default:
		return Palette{
			Background: ColorBg,
			Panel:      ColorPanel,
			Accent:     ColorAccent,
			Dim:        ColorDim,
			Text:       ColorText,
			Border:     ColorBorder,
			Highlight:  ColorHighlight,
			Error:      ColorError,
			Success:    ColorSuccess,
			Warning:    ColorWarning,
			CodeBg:     ColorCodeBg,
			SourceSel:  ColorSourceSel,
			Ink:        ColorInk,
		}
	}
}
