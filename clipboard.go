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
		parts := strings.Fields(browser)
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
