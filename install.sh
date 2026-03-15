#!/usr/bin/env bash
set -euo pipefail

echo "Building seek..."
go build -o seek .

INSTALL_DIR="${HOME}/.local/bin"
mkdir -p "$INSTALL_DIR"
cp seek "$INSTALL_DIR/seek"

CONFIG_DIR="${HOME}/.config/seek"
mkdir -p "$CONFIG_DIR"
if [ ! -f "$CONFIG_DIR/config.toml" ]; then
    cp config.example.toml "$CONFIG_DIR/config.toml"
    echo "Created config at $CONFIG_DIR/config.toml"
    echo "  -> Add your Tavily API key: https://tavily.com"
    echo "  -> Configure your LLM backend (Ollama or OpenAI-compatible)"
fi

echo
echo "Installed seek to $INSTALL_DIR/seek"
echo
echo "Add to your shell config (~/.zshrc or ~/.bashrc):"
echo '  alias s="seek"'
echo '  # Optional: function ? { seek "$*"; }'
echo
echo 'Then: source ~/.zshrc && s "your first query"'
