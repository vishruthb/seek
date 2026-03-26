package main

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	searchpkg "seek/search"
)

var unsafeTerminalEscapeRE = regexp.MustCompile(`\x1b(?:\[[0-?]*[ -/]*[@-~]|\][^\x1b\x07]*(?:\x07|\x1b\\)|[@-_])`)

func sanitizeTerminalText(value string) string {
	if value == "" {
		return ""
	}

	value = unsafeTerminalEscapeRE.ReplaceAllString(value, "")

	var b strings.Builder
	b.Grow(len(value))

	for _, r := range value {
		switch {
		case r == '\r':
			b.WriteByte('\n')
		case r == '\n' || r == '\t':
			b.WriteRune(r)
		case r < 0x20 || (r >= 0x7f && r <= 0x9f):
			continue
		default:
			b.WriteRune(r)
		}
	}

	return b.String()
}

func sanitizeInlineText(value string) string {
	value = strings.TrimSpace(sanitizeTerminalText(value))
	if value == "" {
		return ""
	}
	return strings.Join(strings.Fields(value), " ")
}

func sanitizeURLText(value string) string {
	if value == "" {
		return ""
	}

	value = unsafeTerminalEscapeRE.ReplaceAllString(value, "")

	var b strings.Builder
	b.Grow(len(value))

	for _, r := range value {
		if r < 0x20 || (r >= 0x7f && r <= 0x9f) {
			continue
		}
		b.WriteRune(r)
	}

	return strings.TrimSpace(b.String())
}

func validateExternalURL(raw string) (string, error) {
	targetURL := sanitizeURLText(raw)
	if targetURL == "" {
		return "", errors.New("source URL is empty")
	}

	parsed, err := url.Parse(targetURL)
	if err != nil {
		return "", fmt.Errorf("invalid source URL: %w", err)
	}
	if !parsed.IsAbs() || parsed.Host == "" {
		return "", errors.New("source URL must be an absolute http(s) URL")
	}
	if parsed.User != nil {
		return "", errors.New("blocked URL with embedded credentials")
	}

	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return parsed.String(), nil
	default:
		return "", fmt.Errorf("blocked non-http URL scheme %q", parsed.Scheme)
	}
}

func sanitizeSearchResult(result searchpkg.SearchResult) searchpkg.SearchResult {
	result.Title = sanitizeInlineText(result.Title)
	result.URL = sanitizeURLText(result.URL)
	result.Content = strings.TrimSpace(sanitizeTerminalText(result.Content))
	return result
}

func sanitizeSearchResults(results []searchpkg.SearchResult) []searchpkg.SearchResult {
	sanitized := make([]searchpkg.SearchResult, 0, len(results))
	for _, result := range results {
		sanitized = append(sanitized, sanitizeSearchResult(result))
	}
	return sanitized
}
