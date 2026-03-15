package main

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
)

func CopyToClipboard(text string) error {
	cmd, err := clipboardCommand()
	if err != nil {
		return err
	}

	in, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	if _, err := io.WriteString(in, text); err != nil {
		_ = in.Close()
		_ = cmd.Process.Kill()
		return err
	}

	if err := in.Close(); err != nil {
		return err
	}

	return cmd.Wait()
}

func clipboardCommand() (*exec.Cmd, error) {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("pbcopy"), nil
	case "linux":
		if path, err := exec.LookPath("wl-copy"); err == nil {
			return exec.Command(path), nil
		}
		if path, err := exec.LookPath("xclip"); err == nil {
			return exec.Command(path, "-selection", "clipboard"), nil
		}
		if path, err := exec.LookPath("xsel"); err == nil {
			return exec.Command(path, "--clipboard", "--input"), nil
		}
		return nil, errors.New("no clipboard tool found; install wl-copy, xclip, or xsel")
	case "windows":
		return exec.Command("clip"), nil
	default:
		return nil, fmt.Errorf("clipboard is not supported on %s", runtime.GOOS)
	}
}

func OpenBrowser(targetURL string, browser string) error {
	if strings.TrimSpace(browser) != "" {
		parts, err := splitCommandLine(browser)
		if err != nil {
			return err
		}
		if len(parts) == 0 {
			return errors.New("browser command is empty")
		}
		args := append(parts[1:], targetURL)
		return exec.Command(parts[0], args...).Start()
	}

	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", targetURL).Start()
	case "linux":
		return exec.Command("xdg-open", targetURL).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", targetURL).Start()
	default:
		return fmt.Errorf("opening a browser is not supported on %s", runtime.GOOS)
	}
}

func splitCommandLine(command string) ([]string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil, nil
	}

	var (
		args         []string
		current      strings.Builder
		quote        rune
		escaped      bool
		flushCurrent = func() {
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		}
	)

	for _, r := range command {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote = r
		case r == ' ' || r == '\t' || r == '\n':
			flushCurrent()
		default:
			current.WriteRune(r)
		}
	}

	if escaped {
		current.WriteRune('\\')
	}
	if quote != 0 {
		return nil, fmt.Errorf("browser command has an unterminated quote")
	}
	flushCurrent()
	return args, nil
}
