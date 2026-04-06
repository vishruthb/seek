package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/muesli/termenv"
)

const fallbackResolvedTheme = "pastel"

func resolveThemePreference(theme string) string {
	return resolveThemePreferenceWithDetector(theme, detectPreferredTheme)
}

func resolveThemePreferenceWithDetector(theme string, detector func() string) string {
	normalized := strings.TrimSpace(strings.ToLower(theme))
	switch normalized {
	case "", "auto":
		if detector != nil {
			if detected := normalizeResolvedTheme(detector()); detected != "" {
				return detected
			}
		}
		return fallbackResolvedTheme
	default:
		if resolved := normalizeResolvedTheme(normalized); resolved != "" {
			return resolved
		}
		return fallbackResolvedTheme
	}
}

func normalizeResolvedTheme(theme string) string {
	switch strings.TrimSpace(strings.ToLower(theme)) {
	case "dark", "pastel":
		return "pastel"
	case "light", "dracula", "tokyo-night":
		return strings.TrimSpace(strings.ToLower(theme))
	default:
		return ""
	}
}

func detectPreferredTheme() string {
	if termenv.HasDarkBackground() {
		return fallbackResolvedTheme
	}
	if termenv.BackgroundColor() != nil {
		return "light"
	}
	if theme := detectThemeFromColorFGBG(os.Getenv("COLORFGBG")); theme != "" {
		return theme
	}
	if runtime.GOOS == "darwin" {
		if theme := detectThemeFromMacOS(); theme != "" {
			return theme
		}
	}
	return ""
}

func detectThemeFromColorFGBG(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	fields := strings.FieldsFunc(value, func(r rune) bool {
		switch r {
		case ';', ',', ':', ' ', '\t':
			return true
		default:
			return false
		}
	})

	for i := len(fields) - 1; i >= 0; i-- {
		n, err := strconv.Atoi(fields[i])
		if err != nil {
			continue
		}
		if ansiColorLooksLight(n) {
			return "light"
		}
		return fallbackResolvedTheme
	}

	return ""
}

func ansiColorLooksLight(n int) bool {
	if n < 0 {
		return false
	}
	if n <= 6 || n == 8 {
		return false
	}
	return true
}

func detectThemeFromMacOS() string {
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	out, err := exec.CommandContext(ctx, "defaults", "read", "-g", "AppleInterfaceStyle").Output()
	if err != nil {
		if ctx.Err() != nil {
			return ""
		}
		if errors.Is(err, exec.ErrNotFound) {
			return ""
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && strings.TrimSpace(string(exitErr.Stderr)) == "" {
			return "light"
		}
		return ""
	}

	if strings.EqualFold(strings.TrimSpace(string(out)), "dark") {
		return fallbackResolvedTheme
	}
	return "light"
}
