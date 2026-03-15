package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func runSetup() error {
	return runSetupWizard(os.Stdin, os.Stdout, ConfigPath())
}

func runSetupWizard(in io.Reader, out io.Writer, path string) error {
	cfg, err := loadConfigFile(path)
	if err != nil {
		return fmt.Errorf("load existing config: %w", err)
	}

	reader := bufio.NewReader(in)
	exists := false
	if _, err := os.Stat(path); err == nil {
		exists = true
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("stat config path: %w", err)
	}

	fmt.Fprintln(out, "seek setup")
	fmt.Fprintln(out, "press enter to keep the current value shown in brackets.")
	fmt.Fprintf(out, "config path: %s\n\n", path)

	if exists {
		overwrite, err := promptBool(reader, out, "overwrite the existing config", true)
		if err != nil {
			return err
		}
		if !overwrite {
			fmt.Fprintln(out, "setup cancelled.")
			return nil
		}
		fmt.Fprintln(out)
	}

	cfg.LLMBackend, err = promptChoice(reader, out, "llm backend", cfg.LLMBackend, []string{"ollama", "openai"})
	if err != nil {
		return err
	}
	cfg.TavilyAPIKey, err = promptSecret(reader, out, "tavily api key", cfg.TavilyAPIKey)
	if err != nil {
		return err
	}
	cfg.SearchDepth, err = promptChoice(reader, out, "search depth", cfg.SearchDepth, []string{"basic", "advanced"})
	if err != nil {
		return err
	}
	cfg.MaxResults, err = promptInt(reader, out, "max results", cfg.MaxResults)
	if err != nil {
		return err
	}
	cfg.OutputFormat, err = promptChoice(reader, out, "output format", cfg.OutputFormat, []string{"concise", "learning", "explanatory", "oneliner"})
	if err != nil {
		return err
	}

	switch cfg.LLMBackend {
	case "openai":
		cfg.OpenAIAPIKey, err = promptSecret(reader, out, "openai-compatible api key", cfg.OpenAIAPIKey)
		if err != nil {
			return err
		}
		cfg.OpenAIBaseURL, err = promptString(reader, out, "openai-compatible base url", cfg.OpenAIBaseURL)
		if err != nil {
			return err
		}
		cfg.OpenAIModel, err = promptString(reader, out, "openai-compatible model", cfg.OpenAIModel)
		if err != nil {
			return err
		}
	case "ollama":
		cfg.OllamaURL, err = promptString(reader, out, "ollama url", cfg.OllamaURL)
		if err != nil {
			return err
		}
		cfg.OllamaModel, err = promptString(reader, out, "ollama model", cfg.OllamaModel)
		if err != nil {
			return err
		}
	}

	cfg.normalize()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(renderConfigTOML(cfg)), 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	fmt.Fprintln(out)
	fmt.Fprintf(out, "wrote config to %s\n", path)
	fmt.Fprintln(out, "run `seek --config` to print the path again.")
	fmt.Fprintln(out, "try: seek \"hello world\"")
	return nil
}

func promptString(reader *bufio.Reader, out io.Writer, label, current string) (string, error) {
	fmt.Fprintf(out, "%s [%s]: ", label, current)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("read %s: %w", label, err)
	}
	value := strings.TrimSpace(line)
	if value == "" {
		return current, nil
	}
	return value, nil
}

func promptSecret(reader *bufio.Reader, out io.Writer, label, current string) (string, error) {
	hint := "blank"
	if strings.TrimSpace(current) != "" {
		hint = "currently set"
	}
	fmt.Fprintf(out, "%s [%s]: ", label, hint)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("read %s: %w", label, err)
	}
	value := strings.TrimSpace(line)
	if value == "" {
		return current, nil
	}
	return value, nil
}

func promptChoice(reader *bufio.Reader, out io.Writer, label, current string, allowed []string) (string, error) {
	for {
		fmt.Fprintf(out, "%s [%s] (%s): ", label, current, strings.Join(allowed, "/"))
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return "", fmt.Errorf("read %s: %w", label, err)
		}
		value := strings.TrimSpace(strings.ToLower(line))
		if value == "" {
			return current, nil
		}
		for _, option := range allowed {
			if value == option {
				return value, nil
			}
		}
		fmt.Fprintf(out, "enter one of: %s\n", strings.Join(allowed, ", "))
	}
}

func promptInt(reader *bufio.Reader, out io.Writer, label string, current int) (int, error) {
	for {
		fmt.Fprintf(out, "%s [%d]: ", label, current)
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return 0, fmt.Errorf("read %s: %w", label, err)
		}
		value := strings.TrimSpace(line)
		if value == "" {
			return current, nil
		}
		n, err := strconv.Atoi(value)
		if err == nil && n > 0 {
			return n, nil
		}
		fmt.Fprintln(out, "enter a positive number.")
	}
}

func promptBool(reader *bufio.Reader, out io.Writer, label string, current bool) (bool, error) {
	defaultValue := "y"
	if !current {
		defaultValue = "n"
	}
	for {
		fmt.Fprintf(out, "%s [y/n] (%s): ", label, defaultValue)
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return false, fmt.Errorf("read %s: %w", label, err)
		}
		value := strings.TrimSpace(strings.ToLower(line))
		switch value {
		case "":
			return current, nil
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			fmt.Fprintln(out, "enter y or n.")
		}
	}
}

func renderConfigTOML(cfg Config) string {
	return fmt.Sprintf(`# seek configuration

# === Search ===
tavily_api_key = %q
search_depth = %q
max_results = %d

# === LLM Backend ===
llm_backend = %q

# -- Ollama --
ollama_url = %q
ollama_model = %q

# -- OpenAI-compatible --
openai_api_key = %q
openai_base_url = %q
openai_model = %q

# === Output ===
output_format = %q
theme = %q
print_on_exit = %t
browser = %q
max_turns = %d
`, cfg.TavilyAPIKey, cfg.SearchDepth, cfg.MaxResults, cfg.LLMBackend, cfg.OllamaURL, cfg.OllamaModel, cfg.OpenAIAPIKey, cfg.OpenAIBaseURL, cfg.OpenAIModel, cfg.OutputFormat, cfg.Theme, cfg.PrintOnExit, cfg.Browser, cfg.MaxTurns)
}
