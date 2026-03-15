package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	"seek/llm"
	"seek/search"
)

const (
	defaultSearchDepth  = "basic"
	defaultMaxResults   = 5
	defaultLLMBackend   = "ollama"
	defaultOllamaURL    = "http://localhost:11434"
	defaultOllamaModel  = "llama3.1:8b"
	defaultOpenAIURL    = "https://api.groq.com/openai"
	defaultOpenAIModel  = "llama-3.3-70b-versatile"
	defaultTheme        = "pastel"
	defaultOutputFormat = "concise"
	defaultMaxTurns     = 10
)

type Config struct {
	TavilyAPIKey  string `toml:"tavily_api_key"`
	SearchDepth   string `toml:"search_depth"`
	MaxResults    int    `toml:"max_results"`
	LLMBackend    string `toml:"llm_backend"`
	OllamaURL     string `toml:"ollama_url"`
	OllamaModel   string `toml:"ollama_model"`
	OpenAIAPIKey  string `toml:"openai_api_key"`
	OpenAIBaseURL string `toml:"openai_base_url"`
	OpenAIModel   string `toml:"openai_model"`
	OutputFormat  string `toml:"output_format"`
	Theme         string `toml:"theme"`
	PrintOnExit   bool   `toml:"print_on_exit"`
	Browser       string `toml:"browser"`
	MaxTurns      int    `toml:"max_turns"`
}

func DefaultConfig() Config {
	return Config{
		SearchDepth:   defaultSearchDepth,
		MaxResults:    defaultMaxResults,
		LLMBackend:    defaultLLMBackend,
		OllamaURL:     defaultOllamaURL,
		OllamaModel:   defaultOllamaModel,
		OpenAIBaseURL: defaultOpenAIURL,
		OpenAIModel:   defaultOpenAIModel,
		OutputFormat:  defaultOutputFormat,
		Theme:         defaultTheme,
		MaxTurns:      defaultMaxTurns,
	}
}

func ConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".config/seek/config.toml"
	}
	return filepath.Join(configDirPath(home), "config.toml")
}

func configDirPath(home string) string {
	return filepath.Join(home, ".config", "seek")
}

func LoadConfig() (Config, error) {
	cfg, err := loadConfigFile(ConfigPath())
	if err != nil {
		return cfg, err
	}
	applyEnvOverrides(&cfg)
	cfg.normalize()
	return cfg, nil
}

func loadConfigFile(path string) (Config, error) {
	cfg := DefaultConfig()

	if _, err := os.Stat(path); err == nil {
		if _, err := toml.DecodeFile(path, &cfg); err != nil {
			return cfg, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return cfg, err
	}

	cfg.normalize()
	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if key := strings.TrimSpace(os.Getenv("TAVILY_API_KEY")); key != "" {
		cfg.TavilyAPIKey = key
	}
	if model := strings.TrimSpace(os.Getenv("SEEK_OLLAMA_MODEL")); model != "" {
		cfg.OllamaModel = model
	}
	if host := strings.TrimSpace(os.Getenv("OLLAMA_HOST")); host != "" {
		cfg.OllamaURL = host
	}
	if key := strings.TrimSpace(os.Getenv("OPENAI_API_KEY")); key != "" {
		cfg.OpenAIAPIKey = key
	}
	if baseURL := strings.TrimSpace(os.Getenv("OPENAI_BASE_URL")); baseURL != "" {
		cfg.OpenAIBaseURL = baseURL
	}
	if model := strings.TrimSpace(os.Getenv("SEEK_OPENAI_MODEL")); model != "" {
		cfg.OpenAIModel = model
	}
	if format := strings.TrimSpace(os.Getenv("SEEK_FORMAT")); format != "" {
		cfg.OutputFormat = format
	}
}

func (c *Config) ApplyFlagOverrides(backend, model, depth, format string, results int) {
	if value := strings.TrimSpace(strings.ToLower(backend)); value != "" {
		c.LLMBackend = value
	}
	if value := strings.TrimSpace(model); value != "" {
		switch strings.ToLower(c.LLMBackend) {
		case "openai":
			c.OpenAIModel = value
		default:
			c.OllamaModel = value
		}
	}
	if value := strings.TrimSpace(strings.ToLower(depth)); value != "" {
		c.SearchDepth = value
	}
	if value := strings.TrimSpace(strings.ToLower(format)); value != "" {
		c.OutputFormat = value
	}
	if results > 0 {
		c.MaxResults = results
	}
	c.normalize()
}

func (c *Config) normalize() {
	switch strings.TrimSpace(strings.ToLower(c.SearchDepth)) {
	case "", "basic":
		c.SearchDepth = defaultSearchDepth
	case "advanced":
		c.SearchDepth = "advanced"
	default:
		c.SearchDepth = defaultSearchDepth
	}

	if c.MaxResults <= 0 {
		c.MaxResults = defaultMaxResults
	}

	switch strings.TrimSpace(strings.ToLower(c.LLMBackend)) {
	case "", "ollama":
		c.LLMBackend = defaultLLMBackend
	case "openai":
		c.LLMBackend = "openai"
	default:
		c.LLMBackend = defaultLLMBackend
	}

	if strings.TrimSpace(c.OllamaURL) == "" {
		c.OllamaURL = defaultOllamaURL
	}
	if strings.TrimSpace(c.OllamaModel) == "" {
		c.OllamaModel = defaultOllamaModel
	}
	if strings.TrimSpace(c.OpenAIBaseURL) == "" {
		c.OpenAIBaseURL = defaultOpenAIURL
	}
	if strings.TrimSpace(c.OpenAIModel) == "" {
		c.OpenAIModel = defaultOpenAIModel
	}

	switch format := strings.TrimSpace(strings.ToLower(c.OutputFormat)); format {
	case "", "concise":
		c.OutputFormat = defaultOutputFormat
	case "learning", "explanatory", "oneliner":
		c.OutputFormat = format
	default:
		c.OutputFormat = defaultOutputFormat
	}

	switch theme := strings.TrimSpace(strings.ToLower(c.Theme)); theme {
	case "", "dark", "pastel", "light", "dracula", "tokyo-night":
		if theme == "" {
			c.Theme = defaultTheme
		} else {
			c.Theme = theme
		}
	default:
		c.Theme = defaultTheme
	}

	if c.MaxTurns <= 0 {
		c.MaxTurns = defaultMaxTurns
	}
}

func initProviders(cfg Config) (search.SearchProvider, llm.LLMProvider, error) {
	searchProvider, err := search.NewTavily(cfg.TavilyAPIKey, cfg.SearchDepth, cfg.MaxResults)
	if err != nil {
		return nil, nil, fmt.Errorf("search setup failed: %w", err)
	}

	switch cfg.LLMBackend {
	case "ollama":
		return searchProvider, llm.NewOllama(cfg.OllamaURL, cfg.OllamaModel), nil
	case "openai":
		llmProvider, err := llm.NewOpenAI(cfg.OpenAIAPIKey, cfg.OpenAIBaseURL, cfg.OpenAIModel)
		if err != nil {
			return nil, nil, fmt.Errorf("openai setup failed: %w", err)
		}
		return searchProvider, llmProvider, nil
	default:
		return nil, nil, fmt.Errorf("unknown llm_backend: %q (use \"ollama\" or \"openai\")", cfg.LLMBackend)
	}
}
