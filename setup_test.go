package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunSetupWizardWritesConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seek", "config.toml")
	input := strings.NewReader(strings.Join([]string{
		"openai",
		"tvly-test",
		"advanced",
		"8",
		"learning",
		"gsk-test",
		"https://api.groq.com/openai",
		"llama-3.3-70b-versatile",
		"",
	}, "\n"))

	var output strings.Builder
	if err := runSetupWizard(input, &output, path); err != nil {
		t.Fatalf("runSetupWizard: %v", err)
	}

	cfg, err := loadConfigFile(path)
	if err != nil {
		t.Fatalf("loadConfigFile: %v", err)
	}

	if cfg.LLMBackend != "openai" {
		t.Fatalf("expected openai backend, got %q", cfg.LLMBackend)
	}
	if cfg.TavilyAPIKey != "tvly-test" || cfg.OpenAIAPIKey != "gsk-test" {
		t.Fatalf("expected keys to be written, got tavily=%q openai=%q", cfg.TavilyAPIKey, cfg.OpenAIAPIKey)
	}
	if cfg.SearchDepth != "advanced" || cfg.MaxResults != 8 || cfg.OutputFormat != "learning" {
		t.Fatalf("unexpected search config: depth=%q results=%d format=%q", cfg.SearchDepth, cfg.MaxResults, cfg.OutputFormat)
	}
	if !strings.Contains(output.String(), "wrote config to") {
		t.Fatalf("expected success output, got %q", output.String())
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("expected config to avoid group/world permissions, got %o", info.Mode().Perm())
	}
}

func TestRunSetupWizardKeepsExistingConfigWhenOverwriteDeclined(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	original := []byte("llm_backend = \"ollama\"\nollama_model = \"llama3.1:8b\"\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatalf("write existing config: %v", err)
	}

	var output strings.Builder
	if err := runSetupWizard(strings.NewReader("n\n"), &output, path); err != nil {
		t.Fatalf("runSetupWizard: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(got) != string(original) {
		t.Fatalf("expected config to remain unchanged, got %q", string(got))
	}
	if !strings.Contains(output.String(), "setup cancelled") {
		t.Fatalf("expected cancel message, got %q", output.String())
	}
}
