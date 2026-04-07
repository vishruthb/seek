package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	releaseRepo       = "vishruthb/seek"
	latestReleaseURL  = "https://api.github.com/repos/" + releaseRepo + "/releases/latest"
	installScriptURL  = "https://seekcli.vercel.app/install.sh"
	updateCheckTimout = 2500 * time.Millisecond
	httpAPITimeout    = 15 * time.Second
	updateRunTimeout  = 60 * time.Second
)

var (
	versionTagRE          = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)`)
	latestReleaseLookup   = fetchLatestReleaseTag
	selfUpdateExecutor    = runInstallScriptUpdate
)

type releaseLatestPayload struct {
	TagName string `json:"tag_name"`
}

type semanticVersion struct {
	Major int
	Minor int
	Patch int
}

func checkForUpdateCmd(currentVersion string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), updateCheckTimout)
		defer cancel()

		latest, err := latestReleaseLookup(ctx)
		if err != nil {
			return releaseCheckMsg{Err: err}
		}
		if !newerReleaseAvailable(currentVersion, latest) {
			return releaseCheckMsg{}
		}
		return releaseCheckMsg{Latest: latest}
	}
}

func runSelfUpdate(stdout, stderr io.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), updateRunTimeout)
	defer cancel()

	latest, err := latestReleaseLookup(ctx)
	if err != nil {
		return fmt.Errorf("update check failed: %w", err)
	}
	if !newerReleaseAvailable(version, latest) {
		current := normalizeReleaseVersion(version)
		if current == "" {
			current = version
		}
		if current == "" {
			current = "current build"
		}
		_, _ = fmt.Fprintf(stdout, "seek is already up to date (%s).\n", fallbackString(latest, current))
		return nil
	}

	current := normalizeReleaseVersion(version)
	if current == "" {
		current = version
	}
	if strings.TrimSpace(current) != "" {
		_, _ = fmt.Fprintf(stdout, "Updating seek %s -> %s\n", current, latest)
	} else {
		_, _ = fmt.Fprintf(stdout, "Updating seek to %s\n", latest)
	}

	if err := selfUpdateExecutor(ctx, latest, stdout, stderr); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}
	return nil
}

func fetchLatestReleaseTag(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestReleaseURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "seek/"+fallbackString(normalizeReleaseVersion(version), "dev"))

	resp, err := (&http.Client{Timeout: httpAPITimeout}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = fmt.Sprintf("GitHub returned HTTP %d", resp.StatusCode)
		}
		return "", errors.New(message)
	}

	var payload releaseLatestPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	tag := strings.TrimSpace(payload.TagName)
	if tag == "" {
		return "", fmt.Errorf("latest release tag was empty")
	}
	return tag, nil
}

func runInstallScriptUpdate(ctx context.Context, targetVersion string, stdout, stderr io.Writer) error {
	scriptPath, cleanup, err := downloadInstallScript(ctx)
	if err != nil {
		return err
	}
	defer cleanup()

	installDir, err := currentInstallDir()
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "/bin/sh", scriptPath)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = append(os.Environ(),
		"SEEK_INSTALL_DIR="+installDir,
		"SEEK_VERSION="+strings.TrimSpace(targetVersion),
	)
	return cmd.Run()
}

func downloadInstallScript(ctx context.Context) (string, func(), error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, installScriptURL, nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("User-Agent", "seek/"+fallbackString(normalizeReleaseVersion(version), "dev"))

	resp, err := (&http.Client{Timeout: updateRunTimeout}).Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = fmt.Sprintf("install script returned HTTP %d", resp.StatusCode)
		}
		return "", nil, errors.New(message)
	}

	tmpFile, err := os.CreateTemp("", "seek-install-*.sh")
	if err != nil {
		return "", nil, err
	}
	path := tmpFile.Name()
	cleanup := func() { _ = os.Remove(path) }

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		_ = tmpFile.Close()
		cleanup()
		return "", nil, err
	}
	if err := tmpFile.Close(); err != nil {
		cleanup()
		return "", nil, err
	}
	if err := os.Chmod(path, 0o700); err != nil {
		cleanup()
		return "", nil, err
	}
	return path, cleanup, nil
}

func currentInstallDir() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("could not locate current seek binary: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exePath); err == nil && strings.TrimSpace(resolved) != "" {
		exePath = resolved
	}
	dir := filepath.Dir(exePath)
	if strings.TrimSpace(dir) == "" {
		return "", fmt.Errorf("could not resolve install directory")
	}
	return dir, nil
}

func newerReleaseAvailable(currentVersion, latestVersion string) bool {
	current, okCurrent := parseSemanticVersion(currentVersion)
	latest, okLatest := parseSemanticVersion(latestVersion)
	if !okLatest {
		return false
	}
	if !okCurrent {
		return true
	}
	switch {
	case latest.Major != current.Major:
		return latest.Major > current.Major
	case latest.Minor != current.Minor:
		return latest.Minor > current.Minor
	default:
		return latest.Patch > current.Patch
	}
}

func normalizeReleaseVersion(raw string) string {
	version, ok := parseSemanticVersion(raw)
	if !ok {
		return ""
	}
	return fmt.Sprintf("v%d.%d.%d", version.Major, version.Minor, version.Patch)
}

func parseSemanticVersion(raw string) (semanticVersion, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return semanticVersion{}, false
	}
	match := versionTagRE.FindStringSubmatch(raw)
	if len(match) != 4 {
		return semanticVersion{}, false
	}
	major, err := strconv.Atoi(match[1])
	if err != nil {
		return semanticVersion{}, false
	}
	minor, err := strconv.Atoi(match[2])
	if err != nil {
		return semanticVersion{}, false
	}
	patch, err := strconv.Atoi(match[3])
	if err != nil {
		return semanticVersion{}, false
	}
	return semanticVersion{Major: major, Minor: minor, Patch: patch}, true
}
