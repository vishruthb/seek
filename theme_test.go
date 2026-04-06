package main

import "testing"

func TestDefaultConfigUsesAutoTheme(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Theme != "auto" {
		t.Fatalf("expected default theme to be auto, got %q", cfg.Theme)
	}
}

func TestResolveThemePreferenceWithDetector(t *testing.T) {
	if got := resolveThemePreferenceWithDetector("auto", func() string { return "light" }); got != "light" {
		t.Fatalf("expected auto theme to resolve to light, got %q", got)
	}
	if got := resolveThemePreferenceWithDetector("auto", func() string { return "dark" }); got != "pastel" {
		t.Fatalf("expected auto theme to resolve dark to pastel, got %q", got)
	}
	if got := resolveThemePreferenceWithDetector("light", nil); got != "light" {
		t.Fatalf("expected explicit light theme to stay light, got %q", got)
	}
}

func TestDetectThemeFromColorFGBG(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
			{name: "empty", value: "", want: ""},
			{name: "dark background", value: "15;0", want: "pastel"},
			{name: "light background", value: "0;15", want: "light"},
			{name: "triple format dark", value: "default;default;0", want: "pastel"},
			{name: "triple format light", value: "default;default;15", want: "light"},
			{name: "invalid", value: "unknown", want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := detectThemeFromColorFGBG(tc.value); got != tc.want {
				t.Fatalf("detectThemeFromColorFGBG(%q) = %q, want %q", tc.value, got, tc.want)
			}
		})
	}
}
