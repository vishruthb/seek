package main

import (
	"fmt"
	"io"
	"strings"
)

func renderToStdout(w io.Writer, answer string, sources []Source) {
	answer = strings.TrimSpace(sanitizeTerminalText(answer))
	if answer != "" {
		fmt.Fprintln(w, answer)
	}
	if len(sources) == 0 {
		return
	}
	if answer != "" {
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w, "---")
	fmt.Fprintln(w, "Sources:")
	for idx, source := range sources {
		title := sanitizeInlineText(source.Title)
		if title == "" {
			title = sanitizeURLText(source.URL)
		}
		fmt.Fprintf(w, "  [%d] %s - %s\n", idx+1, title, sanitizeURLText(source.URL))
	}
}
