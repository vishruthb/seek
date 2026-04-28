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

To upgrade an existing install later:

```bash
seek --update
```

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

Theme defaults to `auto`, which follows your terminal/system appearance when Seek can detect it. You can also override it explicitly:

```bash
export SEEK_THEME="light"
```

## Privacy

- Search queries are sent to Tavily for retrieval.
- Search results and any files you attach with `@[...]` are sent to your configured LLM backend for that query.
- Use Ollama if you want the answer step to stay local, but Tavily still receives the search query.
- Search history is stored locally at `~/.config/seek/history.db`. **You can disable it with `history_enabled = false` or clear it with `seek --clear-history`.**

## Usage

### Pipe errors directly

Pipe any command output into seek — it extracts the error, searches the web, and explains the fix:

```bash
cargo build 2>&1 | seek
go test ./... 2>&1 | seek
python train.py 2>&1 | seek
npm install 2>&1 | seek
gcc main.c 2>&1 | seek
```

Add a question for more targeted answers:

```bash
kubectl get pods 2>&1 | seek "why is my pod in CrashLoopBackOff"
cat config.yaml 2>&1 | seek "is anything wrong with this"
```

Seek extracts the relevant error from the output, searches with your project context, and streams an explanation. Works with any command — compilers, test runners, package managers, deployment tools.

If stdout is also piped, seek outputs plain markdown instead of launching the TUI:

```bash
cargo build 2>&1 | seek > fix.md
cargo build 2>&1 | seek | head -20
```

For a best-effort rerun of the last captured failed command, add a shell hook that records the command, then use:

```bash
seek --last-error
```

Example zsh hook:

```bash
seek_capture_error() {
    local exit_code=$?
    if [[ $exit_code -ne 0 ]]; then
        fc -ln -1 > "/tmp/seek_last_cmd_${USER}.txt" 2>/dev/null
    fi
    return $exit_code
}
precmd_functions+=(seek_capture_error)
```

For bash, use the same function and wire it into `PROMPT_COMMAND`:

```bash
PROMPT_COMMAND="seek_capture_error; $PROMPT_COMMAND"
```

```bash
seek
```

For quicker, more targeted searches:

```
seek "what is a transformer in ML?"
seek --format learning "how does QUIC differ from TCP?"
seek --backend ollama "compare goroutines and threads"
seek --update
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

### History and resuming saved searches

Every completed answer is saved to `~/.config/seek/history.db` by default.

```bash
seek --history "tcp handshake"
seek --recent
seek --recent <count> --project .
seek --stats
seek --resume
seek --resume <id>
seek --open <id>
```

Use `seek --resume` to open an interactive picker of saved chats, or `seek --resume <id>` to rebuild the full parent thread ending at that saved result. `seek --open <id>` still opens only one saved result.

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
/toggle
/new
/resume
/resume <id>
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
