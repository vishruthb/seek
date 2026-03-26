package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigAppliesFileThenEnvOverrides(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configDir := filepath.Join(home, ".config", "seek")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	configBody := `
tavily_api_key = "config-tavily"
search_depth = "advanced"
max_results = 7
llm_backend = "openai"
openai_api_key = "config-openai"
openai_base_url = "https://api.example.com/openai"
openai_model = "config-model"
output_format = "learning"
theme = "light"
max_turns = 4
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configBody), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("TAVILY_API_KEY", "env-tavily")
	t.Setenv("OPENAI_API_KEY", "env-openai")
	t.Setenv("SEEK_OPENAI_MODEL", "env-model")
	t.Setenv("SEEK_FORMAT", "oneliner")
	t.Setenv("SEEK_HISTORY_DB_PATH", filepath.Join(home, "custom-history.db"))
	t.Setenv("SEEK_HISTORY_ENABLED", "false")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.TavilyAPIKey != "env-tavily" {
		t.Fatalf("expected env Tavily key, got %q", cfg.TavilyAPIKey)
	}
	if cfg.OpenAIAPIKey != "env-openai" {
		t.Fatalf("expected env OpenAI key, got %q", cfg.OpenAIAPIKey)
	}
	if cfg.OpenAIModel != "env-model" {
		t.Fatalf("expected env model, got %q", cfg.OpenAIModel)
	}
	if cfg.OutputFormat != "oneliner" {
		t.Fatalf("expected env format override, got %q", cfg.OutputFormat)
	}
	if cfg.SearchDepth != "advanced" || cfg.MaxResults != 7 {
		t.Fatalf("expected config file values to remain, got depth=%q results=%d", cfg.SearchDepth, cfg.MaxResults)
	}
	if cfg.Theme != "light" || cfg.MaxTurns != 4 {
		t.Fatalf("expected config file values to remain, got theme=%q max_turns=%d", cfg.Theme, cfg.MaxTurns)
	}
	if cfg.HistoryEnabled {
		t.Fatalf("expected history to be disabled by env override")
	}
	if cfg.HistoryDBPath != filepath.Join(home, "custom-history.db") {
		t.Fatalf("expected history db path override, got %q", cfg.HistoryDBPath)
	}
}

func TestApplyFlagOverridesTargetsActiveBackendModel(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LLMBackend = "openai"
	cfg.ApplyFlagOverrides("openai", "llama-3.3-70b-versatile", "advanced", "learning", 9)

	if cfg.OpenAIModel != "llama-3.3-70b-versatile" {
		t.Fatalf("expected openai model override, got %q", cfg.OpenAIModel)
	}
	if cfg.SearchDepth != "advanced" || cfg.OutputFormat != "learning" || cfg.MaxResults != 9 {
		t.Fatalf("unexpected overrides: depth=%q format=%q results=%d", cfg.SearchDepth, cfg.OutputFormat, cfg.MaxResults)
	}

	cfg = DefaultConfig()
	cfg.LLMBackend = "ollama"
	cfg.ApplyFlagOverrides("ollama", "qwen2.5:7b", "", "", 0)
	if cfg.OllamaModel != "qwen2.5:7b" {
		t.Fatalf("expected ollama model override, got %q", cfg.OllamaModel)
	}
}
