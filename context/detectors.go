package context

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func detectInDir(dir string) *ProjectContext {
	type manifest struct {
		name   string
		detect func(string) *ProjectContext
	}

	manifests := []manifest{
		{name: "go.mod", detect: detectGoProject},
		{name: "package.json", detect: detectNodeProject},
		{name: "Cargo.toml", detect: detectRustProject},
		{name: "pyproject.toml", detect: detectPyProject},
		{name: "requirements.txt", detect: detectRequirementsProject},
		{name: "Pipfile", detect: detectPipfileProject},
		{name: "pom.xml", detect: detectMavenProject},
		{name: "build.gradle", detect: detectGradleProject},
		{name: "Gemfile", detect: detectRubyProject},
		{name: "composer.json", detect: detectComposerProject},
	}

	for _, manifest := range manifests {
		path := filepath.Join(dir, manifest.name)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		if ctx := manifest.detect(path); ctx != nil {
			ctx.ManifestPath = path
			return finalizeProjectContext(ctx)
		}
	}

	if ctx := detectCProject(dir); ctx != nil {
		return finalizeProjectContext(ctx)
	}

	return nil
}

func finalizeProjectContext(ctx *ProjectContext) *ProjectContext {
	if ctx == nil {
		return nil
	}
	ctx.Language = strings.TrimSpace(strings.ToLower(ctx.Language))
	ctx.Framework = strings.TrimSpace(strings.ToLower(ctx.Framework))
	ctx.BuildSystem = strings.TrimSpace(strings.ToLower(ctx.BuildSystem))
	ctx.Dependencies = uniqueNonEmpty(ctx.Dependencies, 15)
	return ctx
}

func detectGoProject(path string) *ProjectContext {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	deps := make([]string, 0, 15)
	framework := ""
	goVersion := ""
	inRequireBlock := false

	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if strings.HasPrefix(line, "go ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				goVersion = parts[1]
			}
			continue
		}
		if line == "require (" {
			inRequireBlock = true
			continue
		}
		if inRequireBlock && line == ")" {
			inRequireBlock = false
			continue
		}

		var modulePath string
		switch {
		case inRequireBlock:
			modulePath = firstField(line)
		case strings.HasPrefix(line, "require "):
			modulePath = firstField(strings.TrimSpace(strings.TrimPrefix(line, "require ")))
		}
		if modulePath == "" {
			continue
		}

		depName := shortModuleName(modulePath)
		deps = append(deps, depName)
		if framework == "" {
			framework = detectGoFramework(modulePath)
		}
	}

	return &ProjectContext{
		Language:     "go",
		Framework:    framework,
		BuildSystem:  "go",
		Dependencies: deps,
		GoVersion:    goVersion,
	}
}

func detectNodeProject(path string) *ProjectContext {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var payload struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(content, &payload); err != nil {
		return nil
	}

	deps := sortedKeys(payload.Dependencies)
	devDeps := sortedKeys(payload.DevDependencies)
	allDeps := append(append([]string(nil), deps...), devDeps...)

	language := "javascript"
	if hasKey(payload.Dependencies, "typescript") || hasKey(payload.DevDependencies, "typescript") {
		language = "typescript"
	}

	framework := ""
	for _, dep := range allDeps {
		if framework = detectNodeFramework(dep); framework != "" {
			break
		}
	}

	return &ProjectContext{
		Language:     language,
		Framework:    framework,
		BuildSystem:  "npm",
		Dependencies: allDeps,
	}
}

func detectRustProject(path string) *ProjectContext {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	deps := parseSimpleSectionKeys(string(content), []string{"[dependencies]", "[workspace.dependencies]"})
	framework := ""
	for _, dep := range deps {
		if framework = detectRustFramework(dep); framework != "" {
			break
		}
	}

	return &ProjectContext{
		Language:     "rust",
		Framework:    framework,
		BuildSystem:  "cargo",
		Dependencies: deps,
	}
}

func detectRequirementsProject(path string) *ProjectContext {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	deps := parseRequirements(string(content))
	return &ProjectContext{
		Language:     "python",
		Framework:    detectPythonFrameworkFromDeps(deps, string(content)),
		BuildSystem:  "pip",
		Dependencies: deps,
	}
}

func detectPyProject(path string) *ProjectContext {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	deps := parsePyProjectDependencies(string(content))
	return &ProjectContext{
		Language:     "python",
		Framework:    detectPythonFrameworkFromDeps(deps, string(content)),
		BuildSystem:  "pip",
		Dependencies: deps,
	}
}

func detectPipfileProject(path string) *ProjectContext {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	deps := parseSimpleSectionKeys(string(content), []string{"[packages]", "[dev-packages]"})
	return &ProjectContext{
		Language:     "python",
		Framework:    detectPythonFrameworkFromDeps(deps, string(content)),
		BuildSystem:  "pip",
		Dependencies: deps,
	}
}

func detectMavenProject(path string) *ProjectContext {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	body := strings.ToLower(string(content))
	framework := ""
	switch {
	case strings.Contains(body, "spring-boot"):
		framework = "spring-boot"
	case strings.Contains(body, "quarkus"):
		framework = "quarkus"
	}

	deps := make([]string, 0, 2)
	if framework != "" {
		deps = append(deps, framework)
	}

	return &ProjectContext{
		Language:     "java",
		Framework:    framework,
		BuildSystem:  "maven",
		Dependencies: deps,
	}
}

func detectGradleProject(path string) *ProjectContext {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	body := strings.ToLower(string(content))
	framework := ""
	switch {
	case strings.Contains(body, "spring-boot"):
		framework = "spring-boot"
	case strings.Contains(body, "quarkus"):
		framework = "quarkus"
	}

	deps := make([]string, 0, 2)
	if framework != "" {
		deps = append(deps, framework)
	}

	return &ProjectContext{
		Language:     "java",
		Framework:    framework,
		BuildSystem:  "gradle",
		Dependencies: deps,
	}
}

func detectRubyProject(path string) *ProjectContext {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	body := strings.ToLower(string(content))
	framework := ""
	switch {
	case strings.Contains(body, "rails"):
		framework = "rails"
	case strings.Contains(body, "sinatra"):
		framework = "sinatra"
	}

	deps := make([]string, 0, 2)
	if framework != "" {
		deps = append(deps, framework)
	}

	return &ProjectContext{
		Language:     "ruby",
		Framework:    framework,
		BuildSystem:  "bundler",
		Dependencies: deps,
	}
}

func detectComposerProject(path string) *ProjectContext {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var payload struct {
		Require map[string]string `json:"require"`
	}
	if err := json.Unmarshal(content, &payload); err != nil {
		return nil
	}

	deps := sortedKeys(payload.Require)
	framework := ""
	for _, dep := range deps {
		if dep == "laravel/framework" {
			framework = "laravel"
			break
		}
	}

	return &ProjectContext{
		Language:     "php",
		Framework:    framework,
		BuildSystem:  "composer",
		Dependencies: deps,
	}
}

func detectCProject(dir string) *ProjectContext {
	buildSystem := ""
	manifestPath := ""
	if fileExists(filepath.Join(dir, "CMakeLists.txt")) {
		buildSystem = "cmake"
		manifestPath = filepath.Join(dir, "CMakeLists.txt")
	} else if fileExists(filepath.Join(dir, "Makefile")) {
		buildSystem = "make"
		manifestPath = filepath.Join(dir, "Makefile")
	}
	if buildSystem == "" {
		return nil
	}

	hasC := false
	hasCPP := false
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		switch ext {
		case ".c", ".h":
			hasC = true
		case ".cc", ".cpp", ".cxx", ".hpp", ".hh", ".hxx":
			hasCPP = true
		}
	}
	if !hasC && !hasCPP {
		return nil
	}

	language := "c"
	if hasCPP {
		language = "cpp"
	}

	return &ProjectContext{
		Language:     language,
		BuildSystem:  buildSystem,
		Dependencies: nil,
		ManifestPath: manifestPath,
	}
}

func detectGoFramework(modulePath string) string {
	rules := []struct {
		match     string
		framework string
	}{
		{match: "github.com/go-chi/chi", framework: "chi"},
		{match: "github.com/gin-gonic/gin", framework: "gin"},
		{match: "github.com/labstack/echo", framework: "echo"},
		{match: "github.com/gofiber/fiber", framework: "fiber"},
		{match: "gorm.io/gorm", framework: "gorm"},
		{match: "github.com/jmoiron/sqlx", framework: "sqlx"},
	}
	for _, rule := range rules {
		if strings.Contains(modulePath, rule.match) {
			return rule.framework
		}
	}
	return ""
}

func detectNodeFramework(dep string) string {
	switch dep {
	case "next":
		return "nextjs"
	case "react":
		return "react"
	case "vue":
		return "vue"
	case "express":
		return "express"
	case "fastify":
		return "fastify"
	case "@nestjs/core":
		return "nestjs"
	default:
		return ""
	}
}

func detectRustFramework(dep string) string {
	switch dep {
	case "axum":
		return "axum"
	case "actix-web":
		return "actix"
	case "rocket":
		return "rocket"
	case "tokio":
		return "tokio"
	default:
		return ""
	}
}

func detectPythonFrameworkFromDeps(deps []string, content string) string {
	for _, dep := range deps {
		switch dep {
		case "django":
			return "django"
		case "flask":
			return "flask"
		case "fastapi":
			return "fastapi"
		case "torch":
			return "pytorch"
		case "tensorflow":
			return "tensorflow"
		}
	}

	body := strings.ToLower(content)
	switch {
	case strings.Contains(body, "django"):
		return "django"
	case strings.Contains(body, "flask"):
		return "flask"
	case strings.Contains(body, "fastapi"):
		return "fastapi"
	case strings.Contains(body, "torch"):
		return "pytorch"
	case strings.Contains(body, "tensorflow"):
		return "tensorflow"
	default:
		return ""
	}
}

func parseSimpleSectionKeys(content string, sections []string) []string {
	allowed := make(map[string]struct{}, len(sections))
	for _, section := range sections {
		allowed[strings.ToLower(section)] = struct{}{}
	}

	deps := make([]string, 0, 16)
	currentSection := ""

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.ToLower(line)
			continue
		}
		if _, ok := allowed[currentSection]; !ok {
			continue
		}

		name := depNameFromAssignment(line)
		if name != "" {
			deps = append(deps, name)
		}
	}

	return deps
}

func parseRequirements(content string) []string {
	deps := make([]string, 0, 16)
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name := requirementName(line)
		if name != "" {
			deps = append(deps, name)
		}
	}
	return deps
}

func parsePyProjectDependencies(content string) []string {
	deps := make([]string, 0, 16)
	scanner := bufio.NewScanner(strings.NewReader(content))
	inProjectDeps := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inProjectDeps = false
			if line == "[project]" || line == "[tool.poetry.dependencies]" {
				inProjectDeps = true
			}
			continue
		}

		switch {
		case inProjectDeps && strings.HasPrefix(line, "dependencies"):
			deps = append(deps, extractQuotedRequirementNames(line)...)
		case inProjectDeps:
			name := depNameFromAssignment(line)
			if name != "" && name != "python" {
				deps = append(deps, name)
			}
		default:
			if strings.Contains(line, "\"") {
				deps = append(deps, extractQuotedRequirementNames(line)...)
			}
		}
	}

	return deps
}

func extractQuotedRequirementNames(line string) []string {
	parts := strings.Split(line, "\"")
	deps := make([]string, 0, len(parts)/2)
	for idx := 1; idx < len(parts); idx += 2 {
		name := requirementName(parts[idx])
		if name != "" {
			deps = append(deps, name)
		}
	}
	return deps
}

func depNameFromAssignment(line string) string {
	if idx := strings.Index(line, "="); idx >= 0 {
		line = line[:idx]
	}
	return normalizeDepName(line)
}

func requirementName(line string) string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "-e ")
	line = strings.TrimPrefix(line, "--extra-index-url ")
	line = strings.Trim(line, "\"'")

	separators := []string{"==", ">=", "<=", "~=", "!=", ">", "<", "[", ";", " ", "\t"}
	for _, sep := range separators {
		if idx := strings.Index(line, sep); idx >= 0 {
			line = line[:idx]
			break
		}
	}

	return normalizeDepName(line)
}

func normalizeDepName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	name = strings.Trim(name, "\"'")
	if name == "" {
		return ""
	}
	return name
}

func shortModuleName(modulePath string) string {
	modulePath = strings.TrimSpace(modulePath)
	if modulePath == "" {
		return ""
	}

	parts := strings.Split(modulePath, "/")
	last := parts[len(parts)-1]
	if strings.HasPrefix(last, "v") && len(parts) > 1 && isDigits(last[1:]) {
		last = parts[len(parts)-2]
	}
	return normalizeDepName(last)
}

func firstField(line string) string {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func uniqueNonEmpty(values []string, limit int) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, min(limit, len(values)))
	for _, value := range values {
		value = normalizeDepName(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		if normalized := normalizeDepName(key); normalized != "" {
			keys = append(keys, normalized)
		}
	}
	sort.Strings(keys)
	return keys
}

func hasKey(values map[string]string, key string) bool {
	if len(values) == 0 {
		return false
	}
	_, ok := values[key]
	return ok
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func languageLabel(language string) string {
	if language == "" {
		return ""
	}
	return strings.ToUpper(language[:1]) + language[1:]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
