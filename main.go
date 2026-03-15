package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	os.Exit(run())
}

func run() int {
	var (
		backendOverride string
		modelOverride   string
		depthOverride   string
		formatOverride  string
		resultsOverride int
		printConfig     bool
	)

	flag.StringVar(&backendOverride, "backend", "", "override the configured LLM backend (ollama or openai)")
	flag.StringVar(&modelOverride, "model", "", "override the configured model for the active backend")
	flag.StringVar(&depthOverride, "depth", "", "override Tavily search depth (basic or advanced)")
	flag.StringVar(&formatOverride, "format", "", "override the response format (concise, learning, explanatory, oneliner)")
	flag.IntVar(&resultsOverride, "results", 0, "override Tavily max search results")
	flag.BoolVar(&printConfig, "config", false, "print the config path and exit")
	flag.Parse()

	if printConfig {
		fmt.Println(ConfigPath())
		return 0
	}

	cfg, err := LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "seek: failed to load config: %v\n", err)
		return 1
	}
	cfg.ApplyFlagOverrides(backendOverride, modelOverride, depthOverride, formatOverride, resultsOverride)

	searchProvider, llmProvider, err := initProviders(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "seek: %v\n", err)
		return 1
	}

	query := strings.TrimSpace(strings.Join(flag.Args(), " "))
	m := NewModel(cfg, query, searchProvider, llmProvider)

	program := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := program.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "seek: %v\n", err)
		return 1
	}

	if app, ok := finalModel.(*model); ok && app.ShouldPrintOnExit() {
		if output := strings.TrimSpace(app.FinalOutput()); output != "" {
			fmt.Println(output)
		}
	}

	return 0
}
