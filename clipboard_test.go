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
