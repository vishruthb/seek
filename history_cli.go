package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	projectctx "seek/context"
	historypkg "seek/history"
)

func cloneProjectContext(pc *projectctx.ProjectContext) *projectctx.ProjectContext {
	if pc == nil {
		return nil
	}
	cloned := *pc
	cloned.Dependencies = append([]string(nil), pc.Dependencies...)
	return &cloned
}

func projectStackLabel(pc *projectctx.ProjectContext) string {
	if pc == nil || strings.TrimSpace(pc.Language) == "" {
		return ""
	}
	if framework := strings.TrimSpace(pc.Framework); framework != "" {
		return pc.Language + "/" + framework
	}
	return pc.Language
}

func fallbackString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func contextSummaryMarkdown(current, detected *projectctx.ProjectContext) string {
	if current == nil && detected == nil {
		return "## Project Context\n\nNo project context detected in the current working directory."
	}
	if current == nil && detected != nil {
		return "## Project Context\n\nContext is currently disabled for this session.\n\n" + projectContextDetails("Detected", detected)
	}
	return "## Project Context\n\n" + projectContextDetails("Active", current)
}

func projectContextDetails(prefix string, pc *projectctx.ProjectContext) string {
	if pc == nil {
		return prefix + ": not detected"
	}

	lines := []string{
		fmt.Sprintf("%s: %s", prefix, pc.Description),
		fmt.Sprintf("Stack: %s", fallbackString(projectStackLabel(pc), pc.Language)),
		fmt.Sprintf("Build system: %s", fallbackString(pc.BuildSystem, "unknown")),
	}
	if pc.GoVersion != "" {
		lines = append(lines, fmt.Sprintf("Go version: %s", pc.GoVersion))
	}
	if len(pc.Dependencies) > 0 {
		lines = append(lines, fmt.Sprintf("Dependencies: %s", strings.Join(pc.Dependencies, ", ")))
	}
	if pc.ManifestPath != "" {
		lines = append(lines, fmt.Sprintf("Manifest: %s", pc.ManifestPath))
	} else if pc.RootDir != "" {
		lines = append(lines, fmt.Sprintf("Root: %s", pc.RootDir))
	}

	return strings.Join(lines, "\n")
}

func convertSources(sources []Source) []historypkg.Source {
	converted := make([]historypkg.Source, 0, len(sources))
	for _, source := range sources {
		converted = append(converted, historypkg.Source{
			Title:  source.Title,
			URL:    source.URL,
			Domain: source.Domain,
		})
	}
	return converted
}

func convertHistorySources(sources []historypkg.Source) []Source {
	converted := make([]Source, 0, len(sources))
	for _, source := range sources {
		converted = append(converted, Source{
			Title:  source.Title,
			URL:    source.URL,
			Domain: source.Domain,
		})
	}
	return converted
}

func historyRecordsMarkdown(title string, records []historypkg.SearchRecord) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## %s\n\n", title)
	if len(records) == 0 {
		b.WriteString("No saved searches matched.\n")
		return b.String()
	}

	for _, record := range records {
		stack := fallbackString(record.ProjectStack, "no stack")
		when := humanizeAge(record.CreatedAt)
		fmt.Fprintf(&b, "- `%d` · %s · %s\n", record.ID, when, stack)
		fmt.Fprintf(&b, "  %s\n", record.Query)
	}
	b.WriteString("\nUse `seek --open <id>` to reopen a saved result in the full TUI.\n")
	return b.String()
}

func historyStatsMarkdown(stats *historypkg.HistoryStats) string {
	if stats == nil {
		return "## History Stats\n\nNo history statistics available.\n"
	}

	return fmt.Sprintf(
		"## History Stats\n\n- Total searches: %d\n- Unique projects: %d\n- Avg latency: %dms\n- Most searched project: %s\n",
		stats.TotalSearches,
		stats.UniqueProjects,
		stats.AvgTotalMs,
		fallbackString(stats.MostSearchedDir, "n/a"),
	)
}

func sessionStatusMarkdown(cfg Config, providerName, stack string) string {
	themeLine := themeStatusLine(cfg.Theme)
	return fmt.Sprintf(
		"## Session\n\n- Backend: %s\n- Provider: %s\n- Model: %s\n- Mode: %s\n- Theme: %s\n- Search depth: %s\n- Max results: %d\n- Project context: %s\n",
		fallbackString(cfg.LLMBackend, "unknown"),
		fallbackString(providerName, "unknown"),
		fallbackString(activeModel(cfg), "unknown"),
		fallbackString(cfg.OutputFormat, "concise"),
		themeLine,
		fallbackString(cfg.SearchDepth, "basic"),
		cfg.MaxResults,
		fallbackString(stack, "off"),
	)
}

func themeStatusLine(configured string) string {
	configured = strings.TrimSpace(strings.ToLower(configured))
	if configured == "" {
		configured = defaultTheme
	}
	resolved := resolveThemePreference(configured)
	if configured == "auto" {
		return fmt.Sprintf("auto (resolved: %s)", resolved)
	}
	return configured
}

func historyClearMarkdown(deleted int64) string {
	label := "entries"
	if deleted == 1 {
		label = "entry"
	}
	return fmt.Sprintf(
		"## History Cleared\n\nRemoved %d saved history %s from the local database.\n",
		deleted,
		label,
	)
}

func historyRecordsTable(records []historypkg.SearchRecord) string {
	headers := []string{"ID", "When", "Stack", "Query"}
	rows := make([][]string, 0, len(records))
	for _, record := range records {
		rows = append(rows, []string{
			fmt.Sprintf("%d", record.ID),
			humanizeAge(record.CreatedAt),
			record.ProjectStack,
			record.Query,
		})
	}
	return renderTable(headers, rows)
}

func historyStatsTable(stats *historypkg.HistoryStats) string {
	if stats == nil {
		return "No history statistics available.\n"
	}
	return fmt.Sprintf(
		"Total searches: %d\nUnique projects: %d\nAvg latency: %dms\nMost searched project: %s\n",
		stats.TotalSearches,
		stats.UniqueProjects,
		stats.AvgTotalMs,
		fallbackString(stats.MostSearchedDir, "n/a"),
	)
}

func renderTable(headers []string, rows [][]string) string {
	widths := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = len(header)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	var b strings.Builder
	renderRow := func(values []string) {
		for i, value := range values {
			if i > 0 {
				b.WriteString(" │ ")
			}
			b.WriteString(value)
			if pad := widths[i] - len(value); pad > 0 {
				b.WriteString(strings.Repeat(" ", pad))
			}
		}
		b.WriteByte('\n')
	}

	renderRule := func() {
		for i, width := range widths {
			if i > 0 {
				b.WriteString("─┼─")
			}
			b.WriteString(strings.Repeat("─", width))
		}
		b.WriteByte('\n')
	}

	renderRow(headers)
	renderRule()
	for _, row := range rows {
		renderRow(row)
	}
	return b.String()
}

func humanizeAge(ts time.Time) string {
	if ts.IsZero() {
		return "unknown"
	}

	delta := time.Since(ts)
	if delta < 0 {
		delta = 0
	}
	switch {
	case delta < time.Minute:
		return fmt.Sprintf("%ds ago", int(delta.Seconds()))
	case delta < time.Hour:
		return fmt.Sprintf("%dm ago", int(delta.Minutes()))
	case delta < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(delta.Hours()))
	case delta < 48*time.Hour:
		return "yesterday"
	default:
		return fmt.Sprintf("%dd ago", int(delta.Hours()/24))
	}
}

func normalizeProjectFilter(baseDir, project string) string {
	project = strings.TrimSpace(project)
	if project == "" {
		return ""
	}
	if project == "." && baseDir != "" {
		return baseDir
	}
	if filepath.IsAbs(project) {
		return filepath.Clean(project)
	}
	if baseDir == "" {
		return filepath.Clean(project)
	}
	return filepath.Clean(filepath.Join(baseDir, project))
}
