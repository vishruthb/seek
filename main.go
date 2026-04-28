package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	projectctx "seek/context"
	historypkg "seek/history"
	llmpkg "seek/llm"
	searchpkg "seek/search"
)

var (
	version = "v1.3.0"
	commit  = "none"
	date    = "unknown"
)

func main() {
	os.Exit(run())
}

func run() int {
	return runWithArgs(os.Args[1:], os.Stdout, os.Stderr)
}

func runWithArgs(args []string, stdout, stderr io.Writer) int {
	var (
		backendOverride string
		modelOverride   string
		depthOverride   string
		formatOverride  string
		resultsOverride int
		historyQuery    string
		projectFilter   string
		printConfig     bool
		runSetupWizard  bool
		runUpdate       bool
		printVersion    bool
		showRecent      bool
		showStats       bool
		clearHistory    bool
		useLastError    bool
		resumeHistory   bool
		openID          int64
	)

	fs := flag.NewFlagSet("seek", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&backendOverride, "backend", "", "override the configured LLM backend (ollama or openai)")
	fs.StringVar(&modelOverride, "model", "", "override the configured model for the active backend")
	fs.StringVar(&depthOverride, "depth", "", "override Tavily search depth (basic or advanced)")
	fs.StringVar(&formatOverride, "format", "", "override the response format (concise, learning, explanatory, oneliner)")
	fs.IntVar(&resultsOverride, "results", 0, "override Tavily max search results")
	fs.StringVar(&historyQuery, "history", "", "search local history and print matching entries")
	fs.StringVar(&projectFilter, "project", "", "filter history commands to a project directory")
	fs.BoolVar(&showRecent, "recent", false, "print recent history entries and exit")
	fs.BoolVar(&showStats, "stats", false, "print history statistics and exit")
	fs.BoolVar(&clearHistory, "clear-history", false, "delete all saved local history entries and exit")
	fs.BoolVar(&useLastError, "last-error", false, "rerun the last captured failed command and explain its output")
	fs.BoolVar(&resumeHistory, "resume", false, "show saved chats to resume, or pass a history id as the only arg")
	fs.Int64Var(&openID, "open", 0, "open a saved history entry in the TUI")
	fs.BoolVar(&printConfig, "config", false, "print the config path and exit")
	fs.BoolVar(&runSetupWizard, "setup", false, "create or update ~/.config/seek/config.toml interactively")
	fs.BoolVar(&runUpdate, "update", false, "download and install the latest released version of seek")
	fs.BoolVar(&printVersion, "version", false, "print version information and exit")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := validateFlagOverrides(backendOverride, depthOverride, formatOverride); err != nil {
		fmt.Fprintf(stderr, "seek: %v\n", err)
		return 2
	}

	if printConfig {
		fmt.Fprintln(stdout, ConfigPath())
		return 0
	}
	if runUpdate {
		if printVersion || printConfig || runSetupWizard || strings.TrimSpace(historyQuery) != "" || showRecent || showStats || clearHistory || useLastError || resumeHistory || openID > 0 || len(fs.Args()) > 0 {
			fmt.Fprintln(stderr, "seek: --update cannot be combined with other actions or a query")
			return 2
		}
		if err := runSelfUpdate(stdout, stderr); err != nil {
			fmt.Fprintf(stderr, "seek: %v\n", err)
			return 1
		}
		return 0
	}
	if runSetupWizard {
		if err := runSetup(); err != nil {
			fmt.Fprintf(stderr, "seek: setup failed: %v\n", err)
			return 1
		}
		return 0
	}
	if printVersion {
		fmt.Fprintf(stdout, "seek version %s (%s, %s)\n", version, commit, date)
		return 0
	}
	if useLastError && (strings.TrimSpace(historyQuery) != "" || showRecent || showStats || clearHistory || resumeHistory || openID > 0) {
		fmt.Fprintln(stderr, "seek: --last-error cannot be combined with history actions, --resume, or --open")
		return 2
	}
	if resumeHistory && (strings.TrimSpace(historyQuery) != "" || showRecent || showStats || clearHistory || openID > 0) {
		fmt.Fprintln(stderr, "seek: --resume cannot be combined with history actions or --open")
		return 2
	}

	cfg, err := LoadConfig()
	if err != nil {
		fmt.Fprintf(stderr, "seek: failed to load config: %v\n", err)
		return 1
	}
	cfg.ApplyFlagOverrides(backendOverride, modelOverride, depthOverride, formatOverride, resultsOverride)

	cwd, _ := os.Getwd()
	cwd = filepath.Clean(cwd)
	sessionWorkingDir := cwd
	projectCtx := projectctx.DetectContext(sessionWorkingDir)

	var historyStore *historypkg.HistoryStore
	if cfg.HistoryEnabled || clearHistory {
		historyStore, err = historypkg.NewHistoryStore(cfg.HistoryDBPath)
		if err != nil {
			fmt.Fprintf(stderr, "warning: could not open history database: %v\n", err)
		}
	}
	if historyStore != nil {
		defer historyStore.Close()
	}

	extraArgs := fs.Args()
	if clearHistory {
		if strings.TrimSpace(historyQuery) != "" || showRecent || showStats || openID > 0 || len(extraArgs) > 0 {
			fmt.Fprintln(stderr, "seek: --clear-history cannot be combined with other history actions or a query")
			return 2
		}
		if historyStore == nil {
			fmt.Fprintln(stderr, "seek: history is unavailable")
			return 1
		}
		deleted, err := historyStore.Clear()
		if err != nil {
			fmt.Fprintf(stderr, "seek: %v\n", err)
			return 1
		}
		label := "entries"
		if deleted == 1 {
			label = "entry"
		}
		fmt.Fprintf(stdout, "Cleared %d history %s.\n", deleted, label)
		return 0
	}
	if showRecent {
		if historyStore == nil {
			fmt.Fprintln(stderr, "seek: history is unavailable")
			return 1
		}
		limit, remaining, err := parseRecentLimit(extraArgs)
		if err != nil {
			fmt.Fprintf(stderr, "seek: %v\n", err)
			return 2
		}
		if len(remaining) > 0 {
			fmt.Fprintf(stderr, "seek: unexpected args: %s\n", strings.Join(remaining, " "))
			return 2
		}
		records, err := historyStore.Recent(limit, normalizeProjectFilter(cwd, projectFilter))
		if err != nil {
			fmt.Fprintf(stderr, "seek: %v\n", err)
			return 1
		}
		fmt.Fprint(stdout, historyRecordsTable(records))
		if len(records) > 0 {
			fmt.Fprint(stdout, "\nUse: seek --resume <id>  to continue a saved thread, or seek --open <id> to view one result\n")
		}
		return 0
	}
	if strings.TrimSpace(historyQuery) != "" {
		if historyStore == nil {
			fmt.Fprintln(stderr, "seek: history is unavailable")
			return 1
		}
		records, err := historyStore.Search(historyQuery, 10)
		if err != nil {
			fmt.Fprintf(stderr, "seek: %v\n", err)
			return 1
		}
		fmt.Fprint(stdout, historyRecordsTable(records))
		if len(records) > 0 {
			fmt.Fprint(stdout, "\nUse: seek --resume <id>  to continue a saved thread, or seek --open <id> to view one result\n")
		}
		return 0
	}
	if showStats {
		if historyStore == nil {
			fmt.Fprintln(stderr, "seek: history is unavailable")
			return 1
		}
		stats, err := historyStore.Stats()
		if err != nil {
			fmt.Fprintf(stderr, "seek: %v\n", err)
			return 1
		}
		fmt.Fprint(stdout, historyStatsTable(stats))
		return 0
	}

	var openRecord *historypkg.SearchRecord
	var resumeRecords []historypkg.SearchRecord
	var resumePicker []historypkg.SearchRecord
	if resumeHistory {
		if historyStore == nil {
			fmt.Fprintln(stderr, "seek: history is unavailable")
			return 1
		}
		resumeID, remaining, err := parseResumeArgs(extraArgs)
		if err != nil {
			fmt.Fprintf(stderr, "seek: %v\n", err)
			return 2
		}
		if len(remaining) > 0 {
			fmt.Fprintf(stderr, "seek: unexpected args: %s\n", strings.Join(remaining, " "))
			return 2
		}
		if resumeID > 0 {
			records, err := loadResumeRecords(historyStore, resumeID)
			if err != nil {
				fmt.Fprintf(stderr, "seek: %v\n", err)
				return 1
			}
			resumeRecords = records
		} else {
			records, err := loadResumeCandidates(historyStore, normalizeProjectFilter(cwd, projectFilter))
			if err != nil {
				fmt.Fprintf(stderr, "seek: %v\n", err)
				return 1
			}
			resumePicker = records
		}
		extraArgs = nil
		if len(resumeRecords) > 0 {
			sessionWorkingDir, projectCtx = resolveSessionProjectContext(cwd, &resumeRecords[len(resumeRecords)-1])
		}
	}
	if openID > 0 {
		if historyStore == nil {
			fmt.Fprintln(stderr, "seek: history is unavailable")
			return 1
		}
		record, err := historyStore.Get(openID)
		if err != nil {
			fmt.Fprintf(stderr, "seek: failed to open history entry %d: %v\n", openID, err)
			return 1
		}
		openRecord = record
		sessionWorkingDir, projectCtx = resolveSessionProjectContext(cwd, openRecord)
	}

	searchProvider, llmProvider, err := initProviders(cfg)
	if err != nil {
		if openRecord == nil && len(resumeRecords) == 0 && resumePicker == nil {
			fmt.Fprintf(stderr, "seek: %v\n", err)
			return 1
		}
		searchProvider = unavailableSearchProvider{err: err}
		llmProvider = unavailableLLMProvider{err: err}
	}
	if warmer, ok := llmProvider.(interface{ Warmup(context.Context) }); ok {
		go warmer.Warmup(context.Background())
	}

	query := strings.TrimSpace(strings.Join(extraArgs, " "))
	var pipedInput *PipedInput
	if useLastError {
		_, raw, err := readLastErrorOutput(context.Background())
		if err != nil {
			fmt.Fprintf(stderr, "seek: %v\n", err)
			return 1
		}
		input := preparePipedInput(raw, query)
		pipedInput = &input
	} else if openRecord == nil && len(resumeRecords) == 0 && inputFromPipeDetector() {
		raw, err := readPipedInput(stdinPipeReader)
		if err != nil {
			fmt.Fprintf(stderr, "seek: failed to read stdin: %v\n", err)
			return 1
		}
		input := preparePipedInput(raw, query)
		if strings.TrimSpace(input.FullOutput) == "" {
			fmt.Fprintln(stderr, "seek: piped input is empty. Try: your-command 2>&1 | seek")
			return 1
		}
		pipedInput = &input
	}
	if pipedInput != nil && strings.TrimSpace(pipedInput.FullOutput) == "" {
		fmt.Fprintln(stderr, "seek: piped input is empty. Try: your-command 2>&1 | seek")
		return 1
	}
	if pipedInput != nil && !outputTTYDetector() {
		return runPipedStdout(context.Background(), *pipedInput, cfg, projectCtx, historyStore, searchProvider, llmProvider, sessionWorkingDir, stdout, stderr)
	}
	if openRecord != nil {
		query = ""
	}
	m := NewModelWithOptions(cfg, query, searchProvider, llmProvider, ModelOptions{
		ProjectContext: projectCtx,
		WorkingDir:     sessionWorkingDir,
		HistoryStore:   historyStore,
		OpenRecord:     openRecord,
		ResumeRecords:  resumeRecords,
		ResumePicker:   resumePicker,
		PipedInput:     pipedInput,
	})

	programOptions := []tea.ProgramOption{tea.WithAltScreen()}
	var ttyInput *os.File
	if pipedInput != nil {
		var err error
		ttyInput, err = openTTYForProgram()
		if err != nil {
			fmt.Fprintf(stderr, "seek: failed to open terminal for pipe mode: %v\n", err)
			return 1
		}
		defer ttyInput.Close()
		programOptions = append(programOptions, tea.WithInput(ttyInput))
	}
	program := tea.NewProgram(m, programOptions...)
	finalModel, err := program.Run()
	if err != nil {
		fmt.Fprintf(stderr, "seek: %v\n", err)
		return 1
	}

	if app, ok := finalModel.(*model); ok && app.ShouldPrintOnExit() {
		if output := strings.TrimSpace(app.FinalOutput()); output != "" {
			fmt.Fprintln(stdout, output)
		}
	}

	return 0
}

func validateFlagOverrides(backend, depth, format string) error {
	if value := strings.TrimSpace(strings.ToLower(backend)); value != "" {
		switch value {
		case "ollama", "openai":
		default:
			return fmt.Errorf("unknown backend %q (use \"ollama\" or \"openai\")", backend)
		}
	}
	if value := strings.TrimSpace(strings.ToLower(depth)); value != "" {
		switch value {
		case "basic", "advanced":
		default:
			return fmt.Errorf("unknown depth %q (use \"basic\" or \"advanced\")", depth)
		}
	}
	if value := strings.TrimSpace(strings.ToLower(format)); value != "" {
		switch value {
		case "concise", "learning", "explanatory", "oneliner":
		default:
			return fmt.Errorf("unknown format %q (use \"concise\", \"learning\", \"explanatory\", or \"oneliner\")", format)
		}
	}
	return nil
}

func resolveSessionProjectContext(cwd string, record *historypkg.SearchRecord) (string, *projectctx.ProjectContext) {
	workingDir := filepath.Clean(cwd)
	if record == nil || strings.TrimSpace(record.ProjectDir) == "" {
		return workingDir, projectctx.DetectContext(workingDir)
	}

	workingDir = filepath.Clean(record.ProjectDir)
	if ctx := projectctx.DetectContext(workingDir); ctx != nil {
		return workingDir, ctx
	}

	stack := strings.TrimSpace(record.ProjectStack)
	if stack == "" {
		return workingDir, nil
	}

	parts := strings.SplitN(stack, "/", 2)
	ctx := &projectctx.ProjectContext{
		Language:    strings.TrimSpace(parts[0]),
		Description: stack + " project",
		RootDir:     workingDir,
	}
	if len(parts) == 2 {
		ctx.Framework = strings.TrimSpace(parts[1])
	}
	return workingDir, ctx
}

func parseRecentLimit(args []string) (int, []string, error) {
	limit := 10
	if len(args) == 0 {
		return limit, args, nil
	}
	value, err := strconv.Atoi(args[0])
	if err != nil {
		return limit, args, nil
	}
	if value <= 0 {
		return 0, nil, fmt.Errorf("recent count must be positive")
	}
	return value, args[1:], nil
}

func parseResumeArgs(args []string) (int64, []string, error) {
	if len(args) == 0 {
		return 0, nil, nil
	}
	value, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil || value <= 0 {
		return 0, args, fmt.Errorf("--resume expects an optional positive history id")
	}
	return value, args[1:], nil
}

func loadResumeRecords(store *historypkg.HistoryStore, id int64) ([]historypkg.SearchRecord, error) {
	if store == nil {
		return nil, fmt.Errorf("history is unavailable")
	}
	if id <= 0 {
		return nil, fmt.Errorf("history id must be positive")
	}
	records, err := store.Thread(id)
	if err != nil {
		return nil, fmt.Errorf("failed to resume history entry %d: %w", id, err)
	}
	return records, nil
}

func loadResumeCandidates(store *historypkg.HistoryStore, projectFilter string) ([]historypkg.SearchRecord, error) {
	if store == nil {
		return nil, fmt.Errorf("history is unavailable")
	}
	records, err := store.ResumeCandidates(100, projectFilter)
	if err != nil {
		return nil, err
	}
	return records, nil
}

type unavailableSearchProvider struct {
	err error
}

func (p unavailableSearchProvider) Search(_ context.Context, _ string, _ int) ([]searchpkg.SearchResult, error) {
	return nil, p.err
}

type unavailableLLMProvider struct {
	err error
}

func (p unavailableLLMProvider) StreamChat(_ context.Context, _ []llmpkg.Message, _ llmpkg.StreamCallback) (string, error) {
	return "", p.err
}

func (p unavailableLLMProvider) Name() string {
	return "unavailable"
}
