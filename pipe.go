package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	historypkg "seek/history"
	llmpkg "seek/llm"
	searchpkg "seek/search"

	projectctx "seek/context"
)

const lastErrorRunTimeout = 2 * time.Minute

var (
	shellFilenameRE                 = regexp.MustCompile(`[^A-Za-z0-9_.-]+`)
	inputFromPipeDetector           = isInputFromPipe
	outputTTYDetector               = isOutputTTY
	stdinPipeReader       io.Reader = os.Stdin
	openTTYForProgram               = openTTY
)

func isInputFromPipe() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) == 0
}

func isOutputTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return true
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func openTTY() (*os.File, error) {
	return os.OpenFile("/dev/tty", os.O_RDWR, 0)
}

func readPipedInput(r io.Reader) (string, error) {
	if r == nil {
		return "", errors.New("stdin is unavailable")
	}

	var retained middleTruncatingBuffer
	if _, err := io.Copy(&retained, r); err != nil {
		return "", err
	}
	return retained.String(), nil
}

type middleTruncatingBuffer struct {
	head      bytes.Buffer
	tail      []byte
	total     int
	truncated bool
}

func (b *middleTruncatingBuffer) Write(p []byte) (int, error) {
	originalLen := len(p)
	b.total += originalLen

	if b.head.Len() < pipeInputHalfLimit {
		remaining := pipeInputHalfLimit - b.head.Len()
		if remaining > len(p) {
			remaining = len(p)
		}
		_, _ = b.head.Write(p[:remaining])
		p = p[remaining:]
	}

	if len(p) > 0 {
		b.truncated = true
		b.tail = append(b.tail, p...)
		if len(b.tail) > pipeInputHalfLimit {
			b.tail = append([]byte(nil), b.tail[len(b.tail)-pipeInputHalfLimit:]...)
		}
	}

	return originalLen, nil
}

func (b *middleTruncatingBuffer) String() string {
	if !b.truncated {
		return b.head.String()
	}
	return b.head.String() + string(b.tail)
}

func lastErrorCommandPath() string {
	user := strings.TrimSpace(os.Getenv("USER"))
	if user == "" {
		user = strings.TrimSpace(os.Getenv("USERNAME"))
	}
	if user == "" {
		user = "unknown"
	}
	user = shellFilenameRE.ReplaceAllString(user, "_")
	return filepath.Join(os.TempDir(), "seek_last_cmd_"+user+".txt")
}

func readLastErrorOutput(ctx context.Context) (string, string, error) {
	path := lastErrorCommandPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", "", fmt.Errorf("No recent error found. Try: your-command 2>&1 | seek")
		}
		return "", "", fmt.Errorf("read last error command: %w", err)
	}

	command := strings.TrimSpace(string(data))
	if command == "" {
		return "", "", fmt.Errorf("No recent error found. Try: your-command 2>&1 | seek")
	}

	output, err := captureCommandOutput(ctx, command)
	if err != nil && strings.TrimSpace(output) == "" {
		return command, "", err
	}
	return command, output, nil
}

func captureCommandOutput(ctx context.Context, command string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, lastErrorRunTimeout)
	defer cancel()

	shell := strings.TrimSpace(os.Getenv("SHELL"))
	if shell == "" {
		shell = "/bin/sh"
	}

	cmd := exec.CommandContext(ctx, shell, "-lc", command)
	var retained middleTruncatingBuffer
	cmd.Stdout = &retained
	cmd.Stderr = &retained
	err := cmd.Run()
	if ctx.Err() != nil {
		return retained.String(), ctx.Err()
	}
	return retained.String(), err
}

func runPipedStdout(
	ctx context.Context,
	input PipedInput,
	cfg Config,
	projectCtx *projectctx.ProjectContext,
	historyStore *historypkg.HistoryStore,
	searchProvider searchpkg.SearchProvider,
	llmProvider llmpkg.LLMProvider,
	workingDir string,
	stdout io.Writer,
	stderr io.Writer,
) int {
	orchestrator := NewOrchestrator(searchProvider, llmProvider, cfg.MaxResults, cfg.OutputFormat, projectCtx)
	answer, sources, timing, err := orchestrator.SearchAndAnswerPiped(ctx, input, nil)
	if err != nil {
		fmt.Fprintf(stderr, "seek: %s\n", friendlyError(err, cfg))
		return 1
	}

	answer = sanitizeAssistantResponse(answer)
	renderToStdout(stdout, answer, sources)

	if historyStore != nil && strings.TrimSpace(answer) != "" {
		if _, err := historyStore.Save(&historypkg.SearchRecord{
			Query:        pipedDisplayQuery(input),
			Response:     answer,
			Sources:      convertSources(sources),
			ProjectDir:   workingDir,
			ProjectStack: projectStackLabel(projectCtx),
			LLMBackend:   llmProvider.Name(),
			OutputFormat: cfg.OutputFormat,
			SearchMs:     timing.SearchMs,
			LLMMs:        timing.LLMMs,
			TotalMs:      timing.TotalMs,
		}); err != nil {
			fmt.Fprintf(stderr, "warning: could not save history: %v\n", err)
		}
	}

	return 0
}
