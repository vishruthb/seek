```text
███████╗███████╗███████╗██╗ ██╗
██╔════╝██╔════╝██╔════╝██║ ██╔╝
███████╗█████╗ █████╗ █████╔╝
╚════██║██╔══╝ ██╔══╝ ██╔═██╗
███████║███████╗███████╗██║ ██╗
╚══════╝╚══════╝╚══════╝╚═╝ ╚═╝
```

AI-powered web search from your terminal. Fast, keyboard-driven, and lightweight.

`seek` is for the moment when you're coding, need a grounded answer, and don't wantto leave the terminal. It uses Tavily for quick web search and either Ollama or an OpenAI-compatible backend to generate summaries to answer your question.

## Install

```bash
go build -o seek .
bash install.sh
```

That installs the binary to `~/.local/bin/seek` and creates `~/.config/seek/config.toml` if it does not already exist.

If `seek` is not found after install, add this to `~/.bashrc` or `~/.zshrc`:

```bash
export PATH="$HOME/.local/bin:$PATH"
alias s='seek'
```

Reload your shell:

```bash
source ~/.bashrc
```

## Setup

You need:

1. A Tavily API key for search
2. One answer backend
   `ollama` locally, or `openai` for Groq / OpenRouter / Together / OpenAI-compatible APIs

Check the config path:

```bash
seek --config
```

### Option A: Ollama

```toml
# ~/.config/seek/config.toml
tavily_api_key = "tvly-..."
llm_backend = "ollama"
ollama_url = "http://localhost:11434"
ollama_model = "llama3.1:8b"
output_format = "concise"
theme = "pastel"
```

Start Ollama and pull a model:

```bash
ollama serve
ollama pull llama3.1:8b
```

### Option B: Groq

```toml
# ~/.config/seek/config.toml
tavily_api_key = "tvly-..."
llm_backend = "openai"
openai_api_key = "gsk-..."
openai_base_url = "https://api.groq.com/openai"
openai_model = "llama-3.3-70b-versatile"
output_format = "concise"
theme = "pastel"
```

You can also keep secrets in env vars instead of the config file:

```bash
export TAVILY_API_KEY="tvly-..."
export OPENAI_API_KEY="gsk-..."
export OPENAI_BASE_URL="https://api.groq.com/openai"
export SEEK_OPENAI_MODEL="llama-3.3-70b-versatile"
```

Env vars override `config.toml`.

## Usage

```bash
seek "what is a transformer in ML?"
seek --format learning "how does QUIC differ from TCP?"
seek --backend ollama "compare goroutines and threads"
seek
```

When launched with plain `seek`, the input window opens immediately.

### In-session slash commands

Use `/` in the input bar to reconfigure the current session without restarting:

```text
/backend openai
/backend ollama
/mode concise
/mode learning
/model llama-3.3-70b-versatile
/depth advanced
/results 8
/copy
/show
/help
```

## Core keys

| Key | Action |
| --- | --- |
| `j` / `k` | Scroll |
| `Tab` | Switch between answer and sources |
| `f` | Open follow-up input |
| `/` | Search within the answer |
| `y` | Copy the full answer |
| `Y` | Copy a fenced code block |
| `o` | Open the selected source |
| `q` | Quit |

`seek` is intentionally small. One binary, no browser UI, no background services.
