package main

import (
	"reflect"
	"testing"
)

func TestSplitCommandLineHonorsQuotes(t *testing.T) {
	args, err := splitCommandLine(`open -a "Google Chrome"`)
	if err != nil {
		t.Fatalf("splitCommandLine: %v", err)
	}

	want := []string{"open", "-a", "Google Chrome"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("unexpected args: got %#v want %#v", args, want)
	}
}

func TestSplitCommandLineSupportsQuotedPaths(t *testing.T) {
	args, err := splitCommandLine(`"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" --new-window`)
	if err != nil {
		t.Fatalf("splitCommandLine: %v", err)
	}

	want := []string{"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", "--new-window"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("unexpected args: got %#v want %#v", args, want)
	}
}

func TestSplitCommandLineRejectsUnterminatedQuote(t *testing.T) {
	if _, err := splitCommandLine(`open -a "Google Chrome`); err == nil {
		t.Fatalf("expected unterminated quote error")
	}
}

func TestValidateExternalURLAllowsHTTPAndHTTPS(t *testing.T) {
	tests := []string{
		"https://example.com/docs?q=seek",
		"http://127.0.0.1:8080/ui",
	}

	for _, input := range tests {
		if _, err := validateExternalURL(input); err != nil {
			t.Fatalf("validateExternalURL(%q): %v", input, err)
		}
	}
}

func TestValidateExternalURLRejectsUnsafeSchemes(t *testing.T) {
	tests := []string{
		"file:///etc/passwd",
		"javascript:alert(1)",
		"mailto:test@example.com",
		"https://user:pass@example.com",
	}

	for _, input := range tests {
		if _, err := validateExternalURL(input); err == nil {
			t.Fatalf("expected %q to be rejected", input)
		}
	}
}
