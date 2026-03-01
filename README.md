# ai-ask

A command-line AI assistant for developers. Supports `openai`, `claude`, and `gemini` protocols.

## Build

### Prerequisites
- Go 1.25.6 or later

### Build from source

```bash
go build -o ask main.go
```

## Quick Setup

`ai-ask` reads 4 env vars:

| Variable | Required | Description |
|----------|----------|-------------|
| `AI_ASK_PROTOCOL` | No | Protocol: `openai` (default), `claude`, `gemini` |
| `AI_ASK_BASE_URL` | Yes | Request endpoint/base URL (depends on protocol) |
| `AI_ASK_API_KEY` | Yes | API key |
| `AI_ASK_MODEL` | Yes | Model name |

### 1) OpenAI protocol (default)

```bash
export AI_ASK_PROTOCOL="openai"
export AI_ASK_BASE_URL="https://api.openai.com/v1/chat/completions"
export AI_ASK_API_KEY="sk-xxx..."
export AI_ASK_MODEL="gpt-4o-mini"
```

### 2) Claude protocol

```bash
export AI_ASK_PROTOCOL="claude"
export AI_ASK_BASE_URL="https://api.anthropic.com/v1/messages"
export AI_ASK_API_KEY="sk-ant-xxx..."
export AI_ASK_MODEL="claude-3-5-sonnet-20241022"
```

### 3) Gemini protocol

```bash
export AI_ASK_PROTOCOL="gemini"
export AI_ASK_BASE_URL="https://generativelanguage.googleapis.com/v1beta"
export AI_ASK_API_KEY="AIzaSy..."
export AI_ASK_MODEL="gemini-2.0-flash"
```

### Persist to shell profile

Add one of the protocol blocks above to `~/.zshrc` or `~/.bashrc`, then reload:

```bash
source ~/.zshrc
```

## Usage

### 1. Direct question

```bash
ask "how to install docker on ubuntu"
```

### 2. Pipe question from stdin

```bash
echo "explain git rebase" | ask
```

### 3. Analyze file/log with context

```bash
# Analyze error logs
cat error.log | ask "what's wrong with this error"

# Explain code
cat main.py | ask "explain this code"

# Debug output
docker logs container_id | ask "why is this container failing"
```

## Features

- Automatically detects your OS, architecture, and shell environment
- Provides OS-specific commands and solutions
- Beautiful markdown rendering in terminal
- Supports piped input for log analysis and debugging
- Concise, actionable responses optimized for developers

## Examples

```bash
# Quick command lookup
ask "git undo last commit"

# Error analysis
npm install 2>&1 | ask "why is npm failing"

# Code review
git diff | ask "review these changes"

# System troubleshooting
dmesg | tail -50 | ask "diagnose this kernel error"
```

## License

This project is open source and available for personal and commercial use.
