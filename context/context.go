package context

import (
	"os"
	"path/filepath"
	"strings"
)

const maxWalkDepth = 10

type ProjectContext struct {
	Language     string
	Framework    string
	BuildSystem  string
	Dependencies []string
	GoVersion    string
	Description  string
	RootDir      string
	ManifestPath string
}

func DetectContext(startDir string) *ProjectContext {
	if strings.TrimSpace(startDir) == "" {
		return nil
	}

	absDir, err := filepath.Abs(startDir)
	if err != nil {
		return nil
	}

	info, err := os.Stat(absDir)
	if err != nil {
		return nil
	}
	if !info.IsDir() {
		absDir = filepath.Dir(absDir)
	}

	home, _ := os.UserHomeDir()
	home = filepath.Clean(home)

	dir := filepath.Clean(absDir)
	for depth := 0; depth <= maxWalkDepth; depth++ {
		if dir == "" || dir == "." {
			return nil
		}
		if home != "" && dir == home {
			return nil
		}

		if ctx := detectInDir(dir); ctx != nil {
			ctx.RootDir = dir
			if ctx.Description == "" {
				ctx.Description = describeContext(ctx)
			}
			return ctx
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return nil
		}
		dir = parent
	}

	return nil
}

func describeContext(ctx *ProjectContext) string {
	if ctx == nil || strings.TrimSpace(ctx.Language) == "" {
		return ""
	}

	parts := make([]string, 0, 4)
	if ctx.Framework != "" {
		parts = append(parts, ctx.Framework)
	}
	for _, dep := range ctx.Dependencies {
		if dep == "" || dep == ctx.Framework {
			continue
		}
		parts = append(parts, dep)
		if len(parts) >= 4 {
			break
		}
	}

	language := languageLabel(ctx.Language)
	if len(parts) == 0 {
		return language + " project"
	}

	return language + " project using " + strings.Join(parts, ", ")
}
