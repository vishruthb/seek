package main

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	pipeInputByteLimit    = 16 * 1024
	pipeInputHalfLimit    = pipeInputByteLimit / 2
	errorContextByteLimit = 4 * 1024
	searchQueryByteLimit  = 100
)

var (
	goCompilerErrorRE = regexp.MustCompile(`^(?:\./|/)?[^:\n]+\.go:\d+:\d+:\s+(.+)$`)
	goCannotUseRE     = regexp.MustCompile(`cannot use \S+ \(([^)]+)\) as ([^,\n]+?)(?: in .*)?$`)
	rustErrorRE       = regexp.MustCompile(`(?i)\berror\[E([0-9]+)\]:\s*(.+)$`)
	cLocationErrorRE  = regexp.MustCompile(`^(?:/|\.?/)?[^:\n]+\.(?:c|cc|cpp|cxx|h|hpp|hh|hxx):\d+(?::\d+)?:\s*(?:fatal\s+)?error:\s*(.+)$`)
	fileLocationRE    = regexp.MustCompile(`(?:^|\s)(?:/|\.?/)?[^\s:]+\.(?:go|rs|c|cc|cpp|cxx|h|hpp|hh|hxx|java|js|jsx|ts|tsx|py|rb|php):\d+(?::\d+)?:\s*`)
	ansiCodeRE        = regexp.MustCompile("`([^`]*)`")
	spaceRE           = regexp.MustCompile(`\s+`)
)

type extractedErrorMatch struct {
	index       int
	end         int
	line        string
	queryLine   string
	language    string
	contextOnly bool
}

// ExtractError takes raw command output and returns:
// 1. A short search query to send to Tavily
// 2. The relevant error context to include prominently in the LLM prompt
// 3. The full output, sanitized and truncated to 16KB for LLM context
func ExtractError(raw string) (searchQuery string, errorContext string, fullOutput string) {
	fullOutput = truncateMiddleBytes(sanitizePipeText(raw), pipeInputByteLimit)
	if strings.TrimSpace(fullOutput) == "" {
		return "", "", ""
	}

	lines := splitOutputLines(fullOutput)
	if len(lines) == 0 {
		return "", "", fullOutput
	}

	if match, ok := findErrorMatch(lines); ok {
		if match.contextOnly {
			errorContext = joinLines(lines[match.index:match.end])
		} else {
			errorContext = contextAround(lines, match.index, 3, 5)
		}
		errorContext = truncateEndBytes(strings.TrimSpace(errorContext), errorContextByteLimit)
		searchQuery = buildErrorSearchQuery(match.queryLine, match.language)
		return searchQuery, errorContext, fullOutput
	}

	contextLines := firstNonEmptyWindow(lines, 20)
	errorContext = truncateEndBytes(strings.TrimSpace(joinLines(contextLines)), errorContextByteLimit)
	searchQuery = buildGenericSearchQuery(contextLines)
	return searchQuery, errorContext, fullOutput
}

func sanitizePipeText(raw string) string {
	if raw == "" {
		return ""
	}
	return sanitizeTerminalText(strings.ToValidUTF8(raw, ""))
}

func splitOutputLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return strings.Split(text, "\n")
}

func findErrorMatch(lines []string) (extractedErrorMatch, bool) {
	if match, ok := findPythonTraceback(lines); ok {
		return match, true
	}
	for idx, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		switch {
		case trimmed == "":
			continue
		case goCompilerErrorRE.MatchString(trimmed):
			return extractedErrorMatch{index: idx, line: trimmed, queryLine: trimmed, language: "Go"}, true
		case strings.HasPrefix(trimmed, "panic:"):
			return extractedErrorMatch{index: idx, line: trimmed, queryLine: trimmed, language: "Go"}, true
		case rustErrorRE.MatchString(trimmed):
			return extractedErrorMatch{index: idx, line: trimmed, queryLine: trimmed, language: "Rust"}, true
		case isNodeErrorLine(trimmed):
			return extractedErrorMatch{index: idx, line: trimmed, queryLine: trimmed, language: "JavaScript"}, true
		case cLocationErrorRE.MatchString(trimmed), strings.Contains(lower, "undefined reference to"):
			return extractedErrorMatch{index: idx, line: trimmed, queryLine: trimmed, language: "C"}, true
		case strings.HasPrefix(trimmed, "Exception in thread"):
			return extractedErrorMatch{index: idx, line: trimmed, queryLine: trimmed, language: "Java"}, true
		case strings.HasPrefix(trimmed, "at com."):
			return extractedErrorMatch{index: idx, line: trimmed, queryLine: trimmed, language: "Java"}, true
		case isGenericErrorLine(trimmed):
			return extractedErrorMatch{index: idx, line: trimmed, queryLine: trimmed, language: ""}, true
		}
	}
	return extractedErrorMatch{}, false
}

func findPythonTraceback(lines []string) (extractedErrorMatch, bool) {
	start := -1
	for idx, line := range lines {
		if strings.Contains(line, "Traceback (most recent call last):") {
			start = idx
		}
	}
	if start < 0 {
		return extractedErrorMatch{}, false
	}

	end := len(lines)
	for idx := start + 1; idx < len(lines); idx++ {
		if strings.TrimSpace(lines[idx]) == "" && idx > start+1 {
			end = idx
			break
		}
	}

	queryLine := ""
	for idx := end - 1; idx >= start; idx-- {
		if line := strings.TrimSpace(lines[idx]); line != "" {
			queryLine = line
			break
		}
	}
	if queryLine == "" {
		queryLine = strings.TrimSpace(lines[start])
	}

	return extractedErrorMatch{
		index:       start,
		end:         end,
		line:        strings.TrimSpace(lines[start]),
		queryLine:   queryLine,
		language:    "Python",
		contextOnly: true,
	}, true
}

func isNodeErrorLine(line string) bool {
	for _, marker := range []string{"TypeError", "ReferenceError", "SyntaxError", "RangeError", "at Object.<anonymous>"} {
		if strings.Contains(line, marker) {
			return true
		}
	}
	return false
}

func isGenericErrorLine(line string) bool {
	lower := strings.ToLower(line)
	if strings.HasPrefix(line, "E ") || strings.Contains(lower, " error: ") || strings.Contains(lower, ": error:") {
		return true
	}
	for _, marker := range []string{
		"error", "fatal", "panic", "failed", "exception", "traceback", "cannot",
		"undefined", "not found", "no such file", "permission denied", "segfault",
		"sigsegv", "compilation failed",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func contextAround(lines []string, index, before, after int) string {
	start := max(0, index-before)
	end := min(len(lines), index+after+1)
	return joinLines(lines[start:end])
}

func firstNonEmptyWindow(lines []string, limit int) []string {
	if limit <= 0 || len(lines) <= limit {
		return lines
	}
	return lines[:limit]
}

func joinLines(lines []string) string {
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

func buildErrorSearchQuery(line, language string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}

	clean := stripProjectSpecificLocation(line)

	switch language {
	case "Go":
		clean = goCompilerErrorRE.ReplaceAllString(clean, "$1")
		if match := goCannotUseRE.FindStringSubmatch(clean); len(match) == 3 {
			clean = "cannot use " + strings.TrimSpace(match[1]) + " as " + strings.TrimSpace(match[2])
		}
	case "Rust":
		if match := rustErrorRE.FindStringSubmatch(clean); len(match) == 3 {
			clean = "error E" + match[1] + " " + match[2]
		}
	case "C":
		if strings.Contains(strings.ToLower(clean), "undefined reference to") {
			clean = cleanUndefinedReference(clean)
		} else if match := cLocationErrorRE.FindStringSubmatch(clean); len(match) == 2 {
			clean = match[1]
		}
	case "Java":
		clean = cleanJavaException(clean)
	}

	clean = normalizeSearchText(clean)
	if clean == "" {
		return ""
	}
	if language != "" && !strings.HasPrefix(strings.ToLower(clean), strings.ToLower(language)+" ") {
		clean = language + " " + clean
	}
	return truncateEndBytes(clean, searchQueryByteLimit)
}

func buildGenericSearchQuery(lines []string) string {
	parts := make([]string, 0, 3)
	for _, line := range lines {
		line = normalizeSearchText(stripProjectSpecificLocation(line))
		if line == "" {
			continue
		}
		parts = append(parts, line)
		if len(parts) >= 3 {
			break
		}
	}
	return truncateEndBytes(strings.Join(parts, " "), searchQueryByteLimit)
}

func stripProjectSpecificLocation(line string) string {
	line = fileLocationRE.ReplaceAllString(line, " ")
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "-->")
	return strings.TrimSpace(line)
}

func cleanUndefinedReference(line string) string {
	lower := strings.ToLower(line)
	idx := strings.Index(lower, "undefined reference to")
	if idx < 0 {
		return line
	}
	return line[idx:]
}

func cleanJavaException(line string) string {
	line = strings.TrimSpace(strings.TrimPrefix(line, "Exception in thread"))
	fields := strings.Fields(line)
	for _, field := range fields {
		field = strings.Trim(field, `"':`)
		if strings.Contains(field, ".") && strings.Contains(field, "Exception") {
			parts := strings.Split(field, ".")
			return parts[len(parts)-1]
		}
		if strings.HasSuffix(field, "Exception") || strings.HasSuffix(field, "Error") {
			return field
		}
	}
	return line
}

func normalizeSearchText(text string) string {
	text = ansiCodeRE.ReplaceAllString(text, "$1")
	replacer := strings.NewReplacer(
		":", " ",
		"(", " ",
		")", " ",
		"[", " ",
		"]", " ",
		"\"", "",
		"'", "",
		"`", "",
		",", " ",
		";", " ",
	)
	text = replacer.Replace(text)
	text = strings.TrimSpace(text)
	text = spaceRE.ReplaceAllString(text, " ")
	return text
}

func truncateMiddleBytes(text string, limit int) string {
	if limit <= 0 || len(text) <= limit {
		return text
	}
	if limit < 2 {
		return text[:limit]
	}
	head := limit / 2
	tail := limit - head
	return safeBytePrefix(text, head) + safeByteSuffix(text, tail)
}

func truncateEndBytes(text string, limit int) string {
	if limit <= 0 || len(text) <= limit {
		return text
	}
	return safeBytePrefix(text, limit)
}

func safeBytePrefix(text string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if len(text) <= limit {
		return text
	}
	for limit > 0 && !isRuneBoundary(text, limit) {
		limit--
	}
	return text[:limit]
}

func safeByteSuffix(text string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if len(text) <= limit {
		return text
	}
	start := len(text) - limit
	for start < len(text) && !isRuneBoundary(text, start) {
		start++
	}
	return text[start:]
}

func isRuneBoundary(text string, index int) bool {
	if index <= 0 || index >= len(text) {
		return true
	}
	return (text[index] & 0xC0) != 0x80
}

func pipedDisplayQuery(input PipedInput) string {
	if query := strings.TrimSpace(input.UserQuery); query != "" {
		return query
	}
	if query := strings.TrimSpace(input.SearchQuery); query != "" {
		return query
	}
	return "explain piped command output"
}

func pipedEffectiveSearchQuery(input PipedInput) string {
	if query := strings.TrimSpace(input.UserQuery); query != "" {
		return query
	}
	return strings.TrimSpace(input.SearchQuery)
}

func clonePipedInput(input *PipedInput) *PipedInput {
	if input == nil {
		return nil
	}
	cloned := *input
	return &cloned
}

func pipedInputLineCount(input PipedInput) int {
	text := strings.TrimRight(input.FullOutput, "\n")
	if text == "" {
		return 0
	}
	return strings.Count(text, "\n") + 1
}

func preparePipedInput(raw, userQuery string) PipedInput {
	searchQuery, errorContext, fullOutput := ExtractError(raw)
	return PipedInput{
		SearchQuery:  strings.TrimSpace(searchQuery),
		ErrorContext: strings.TrimSpace(errorContext),
		FullOutput:   strings.TrimSpace(fullOutput),
		UserQuery:    strings.TrimSpace(userQuery),
	}
}

func formatPipedInputHeader(input PipedInput) string {
	lines := pipedInputLineCount(input)
	label := "lines"
	if lines == 1 {
		label = "line"
	}
	return fmt.Sprintf("piped input (%d %s)", lines, label)
}
