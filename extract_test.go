package main

import (
	"strings"
	"testing"
)

func TestExtractErrorGoCompilerError(t *testing.T) {
	input := "# example.com/myapp\n./main.go:42:15: cannot use x (variable of type string) as int value in argument to foo\n"
	searchQuery, errorContext, _ := ExtractError(input)

	if !strings.Contains(searchQuery, "Go cannot use variable of type string as int value") {
		t.Fatalf("expected Go search query, got %q", searchQuery)
	}
	if strings.Contains(searchQuery, "./main.go:42:15") {
		t.Fatalf("expected file location to be stripped, got %q", searchQuery)
	}
	if !strings.Contains(errorContext, "./main.go:42:15: cannot use x") {
		t.Fatalf("expected full error line in context, got %q", errorContext)
	}
}

func TestExtractErrorRustCompilerError(t *testing.T) {
	input := "error[E0308]: mismatched types\n  --> src/main.rs:10:5\n   |\n10 |     x + y\n   |     ^^^^^ expected `i32`, found `&str`\n"
	searchQuery, errorContext, _ := ExtractError(input)

	if !strings.Contains(searchQuery, "Rust error E0308 mismatched types") {
		t.Fatalf("expected Rust search query, got %q", searchQuery)
	}
	if !strings.Contains(errorContext, "expected `i32`, found `&str`") {
		t.Fatalf("expected Rust error block in context, got %q", errorContext)
	}
}

func TestExtractErrorPythonTraceback(t *testing.T) {
	input := "Traceback (most recent call last):\n  File \"train.py\", line 42, in <module>\n    model.fit(X, y)\n  File \"/usr/lib/python3.11/site-packages/example.py\", line 100, in fit\n    raise ValueError(\"bad input\")\nValueError: bad input\n"
	searchQuery, errorContext, _ := ExtractError(input)

	if !strings.Contains(searchQuery, "Python ValueError bad input") {
		t.Fatalf("expected Python search query, got %q", searchQuery)
	}
	if !strings.Contains(errorContext, "Traceback (most recent call last):") || !strings.Contains(errorContext, "ValueError: bad input") {
		t.Fatalf("expected full traceback in context, got %q", errorContext)
	}
}

func TestExtractErrorNodeError(t *testing.T) {
	input := "TypeError: Cannot read properties of undefined (reading 'map')\n    at Object.<anonymous> (/app/index.js:15:20)\n    at Module._compile (node:internal/modules/cjs/loader:1364:14)\n"
	searchQuery, _, _ := ExtractError(input)

	if !strings.Contains(searchQuery, "JavaScript TypeError Cannot read properties of undefined reading map") {
		t.Fatalf("expected JavaScript search query, got %q", searchQuery)
	}
}

func TestExtractErrorMultipleErrorsTakesFirst(t *testing.T) {
	input := "./a.go:1:2: undefined: first\n./b.go:3:4: undefined: second\n./c.go:5:6: undefined: third\n"
	searchQuery, _, _ := ExtractError(input)

	if !strings.Contains(searchQuery, "first") || strings.Contains(searchQuery, "second") {
		t.Fatalf("expected first error only, got %q", searchQuery)
	}
}

func TestExtractErrorNoErrorUsesFirstTwentyLines(t *testing.T) {
	lines := make([]string, 0, 50)
	for i := 1; i <= 50; i++ {
		lines = append(lines, "normal output line")
	}
	searchQuery, errorContext, _ := ExtractError(strings.Join(lines, "\n"))

	if strings.Count(errorContext, "\n")+1 != 20 {
		t.Fatalf("expected first 20 lines in context, got %q", errorContext)
	}
	if !strings.Contains(searchQuery, "normal output line") {
		t.Fatalf("expected generic search query, got %q", searchQuery)
	}
}

func TestExtractErrorTruncatesFullOutputWithStartAndEnd(t *testing.T) {
	input := strings.Repeat("a", 12*1024) + "ENDMARKER" + strings.Repeat("z", 20*1024)
	_, _, fullOutput := ExtractError(input)

	if len(fullOutput) > pipeInputByteLimit {
		t.Fatalf("expected full output <= %d bytes, got %d", pipeInputByteLimit, len(fullOutput))
	}
	if !strings.HasPrefix(fullOutput, "aaa") || !strings.HasSuffix(fullOutput, "zzz") {
		t.Fatalf("expected truncated output to keep start and end")
	}
}

func TestExtractErrorEmptyInput(t *testing.T) {
	searchQuery, errorContext, fullOutput := ExtractError("")
	if searchQuery != "" || errorContext != "" || fullOutput != "" {
		t.Fatalf("expected empty result, got %q %q %q", searchQuery, errorContext, fullOutput)
	}
}

func TestExtractErrorGarbageInputDoesNotPanic(t *testing.T) {
	searchQuery, errorContext, fullOutput := ExtractError(string([]byte{0xff, 0xfe, 0x00, 'E', 'R', 'R', 'O', 'R'}))
	if strings.ContainsRune(searchQuery+errorContext+fullOutput, '\x00') {
		t.Fatalf("expected NUL bytes to be removed")
	}
}

func TestExtractErrorCUndefinedReference(t *testing.T) {
	input := "/usr/bin/ld: main.o: undefined reference to `foo'\ncollect2: error: ld returned 1 exit status\n"
	searchQuery, _, _ := ExtractError(input)

	if !strings.Contains(searchQuery, "C undefined reference to foo") {
		t.Fatalf("expected C linker query, got %q", searchQuery)
	}
}

func TestExtractErrorJavaStackTrace(t *testing.T) {
	input := "Exception in thread \"main\" java.lang.NullPointerException\n\tat com.example.App.main(App.java:15)\n"
	searchQuery, _, _ := ExtractError(input)

	if !strings.Contains(searchQuery, "Java NullPointerException") {
		t.Fatalf("expected Java exception query, got %q", searchQuery)
	}
}
