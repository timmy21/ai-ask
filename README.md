# ai-ask

A command-line AI assistant for developers that provides quick, concise answers to technical questions using OpenAI-compatible APIs.

## Build

### Prerequisites
- Go 1.25.6 or later

### Build from source

```bash
go build -o ask main.go
```

### Install to system

```bash
# Build and install to $GOPATH/bin
go install

# Or manually copy to a directory in your PATH
sudo cp ask /usr/local/bin/
```

## Environment Variables Setup

Three environment variables are required:

| Variable | Description | Example |
|----------|-------------|---------|
| `AI_ASK_BASE_URL` | OpenAI-compatible API base URL | `https://api.openai.com/v1` |
| `AI_ASK_API_KEY` | API authentication key | `sk-xxx...` |
| `AI_ASK_MODEL` | Model name to use | `gpt-4` or `gpt-3.5-turbo` |

### Configuration

Add to your shell configuration file (`~/.bashrc`, `~/.zshrc`, etc.):

```bash
export AI_ASK_BASE_URL="https://api.openai.com/v1"
export AI_ASK_API_KEY="your-api-key-here"
export AI_ASK_MODEL="gpt-4"
```

Then reload your shell:

```bash
source ~/.zshrc  # or ~/.bashrc
```

### Alternative: Per-session setup

```bash
export AI_ASK_BASE_URL="https://api.openai.com/v1"
export AI_ASK_API_KEY="sk-xxx..."
export AI_ASK_MODEL="gpt-4"
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
