```text
███████╗███████╗███████╗██╗ ██╗
██╔════╝██╔════╝██╔════╝██║ ██╔╝
███████╗█████╗ █████╗ █████╔╝
╚════██║██╔══╝ ██╔══╝ ██╔═██╗
███████║███████╗███████╗██║ ██╗
╚══════╝╚══════╝╚══════╝╚═╝ ╚═╝
```

AI-powered web search from your terminal. Fast, keyboard-driven, and lightweight.

`seek` detects your project stack from the current directory (go.mod, package.json, Cargo.toml, etc.) and tailors searches and answers to your specific frameworks and dependencies. It uses Tavily for web search and either Ollama or any OpenAI-compatible backend for answer generation. All searches are saved to a local SQLite history with full-text search, and per-query latency is shown directly in the TUI.

![seek demo](assets/seek_demo.gif)

## Install

```bash
curl -fsSL https://seekcli.vercel.app/install.sh | sh
```

That installs the binary to `~/.local/bin/seek`.

If you're working from source instead:

```bash
go build -o ~/.local/bin/seek .
```

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

Run the guided setup wizard:

```bash
seek --setup
```

That writes `~/.config/seek/config.toml` and prints the exact path when it's done. You can check it anytime with:

```bash
seek --config
```

You need:

1. A Tavily API key for search
2. One answer backend - can use `ollama` locally, or choose `openai` if using Groq, OpenRouter, Together, or other OpenAI-compatible APIs in the `seek --setup` wizard

### Option A: Ollama

```bash
ollama serve
ollama pull llama3.1:8b
```

### Option B: Groq

You can also keep secrets in env vars instead of the config file:

```bash
export TAVILY_API_KEY="tvly-..."
export OPENAI_API_KEY="gsk-..."
export OPENAI_BASE_URL="https://api.groq.com/openai"
export SEEK_OPENAI_MODEL="llama-3.3-70b-versatile"
```

Env vars override `config.toml`.

## Privacy

- Search queries are sent to Tavily for retrieval.
- Search results and any files you attach with `@[...]` are sent to your configured LLM backend for that query.
- Use Ollama if you want the answer step to stay local, but Tavily still receives the search query.
- Search history is stored locally at `~/.config/seek/history.db`. **You can disable it with `history_enabled = false` or clear it with `seek --clear-history`.**

## Usage

```bash
seek
```

For quicker, more targeted searches:

```
seek "what is a transformer in ML?"
seek --format learning "how does QUIC differ from TCP?"
seek --backend ollama "compare goroutines and threads"
seek
```

When launched with just `seek`, the input window opens immediately.

If `seek` detects a project manifest in your current directory or one of its parents, it tailors searches and answers to that stack automatically. As an example:

```bash
cd ~/work/my-chi-api
seek "how to add middleware"
```

This query is enriched with the detected stack, so Seek prefers Go/Chi results over generic framework docs.

### Local file attachments

You can attach local files directly from the follow-up input with `@[...]`.

```text
explain @[app.go]
compare @[internal/server.go] and @[internal/router.go]
```

As soon as you type `@[`, Seek suggests files from the current working directory. Use `↑` / `↓` to select, then `Enter` or `Tab` to insert the file path. Attached files are read locally and sent to the configured LLM backend as context for that query.

### History and reopening saved searches

Every completed answer is saved to `~/.config/seek/history.db` by default.

```bash
seek --history "tcp handshake"
seek --recent
seek --recent <count> --project .
seek --stats
seek --open <id>
```

Use `seek --open <id>` to reopen a saved result in the full TUI and continue with follow-up searches from there.

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
/context
/context off
/history tcp
/recent
/stats
/copy
/show
/help
/exit
```

`/context` shows the detected stack for the current session. `/context off` disables stack-aware query enrichment until you turn it back on with `/context on`.

When the slash-command picker is open, use `↑` / `↓` to move through commands. After you move once with the arrows, `j` / `k` will keep moving the selection. `Enter` accepts the currently selected command if the slash input is only partially typed.

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
