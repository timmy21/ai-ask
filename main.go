package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/charmbracelet/glamour"
	"golang.org/x/term"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type ChatChoice struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

type ChatResponse struct {
	Choices []ChatChoice `json:"choices"`
}

func main() {
	baseURL := os.Getenv("AI_ASK_BASE_URL")
	if baseURL == "" {
		fmt.Fprintln(os.Stderr, "Error: AI_ASK_BASE_URL is not set")
		os.Exit(1)
	}

	apiKey := os.Getenv("AI_ASK_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "Error: AI_ASK_API_KEY is not set")
		os.Exit(1)
	}

	model := os.Getenv("AI_ASK_MODEL")
	if model == "" {
		fmt.Fprintln(os.Stderr, "Error: AI_ASK_MODEL is not set")
		os.Exit(1)
	}

	// Check if data is being piped to stdin
	stat, _ := os.Stdin.Stat()
	hasPipe := (stat.Mode() & os.ModeCharDevice) == 0
	hasArgs := len(os.Args) >= 2

	var question string
	if hasPipe && hasArgs {
		// Case 1: cat error.log | ask "分析这个错误"
		pipeData, _ := io.ReadAll(os.Stdin)
		argQuestion := strings.Join(os.Args[1:], " ")
		question = fmt.Sprintf("%s\n\n```\n%s```", argQuestion, string(pipeData))
	} else if hasPipe {
		// Case 2: echo "问题" | ask
		pipeData, _ := io.ReadAll(os.Stdin)
		question = string(pipeData)
	} else if hasArgs {
		// Case 3: ask "how to install uv"
		question = strings.Join(os.Args[1:], " ")
	} else {
		fmt.Fprintln(os.Stderr, `Usage:
  ask "question"
  echo "question" | ask
  cat file | ask "question"`)
		os.Exit(1)
	}

	if err := chat(baseURL, apiKey, model, question); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

const systemPromptTemplate = `You are a highly experienced Software Engineer acting as a technical consultant. Your primary role is to help solve problems encountered during software development, debugging, and operations.

## User Environment
- Operating System: {{.OS}}
- Architecture: {{.Arch}}
- Shell: {{.Shell}}

## Core Principles
1. Solution-Oriented: Provide direct, actionable solutions. Get to the point immediately.
2. Concise: Assume the user is technically proficient. Avoid lengthy explanations of basic concepts. Be terse.
3. Practical: Prioritize commands, configuration snippets, and step-by-step fixes that can be executed immediately.

## Command Generation Rules (CRITICAL)
The user is on {{.OS}}/{{.Arch}} with {{.Shell}}. You MUST:
- ONLY provide solutions for {{.OS}}. Do NOT list installation methods for other operating systems.
- Use {{.Shell}} syntax (e.g., variable expansion, conditionals, loops).
- Use {{.OS}}-specific commands, paths, and package managers.
- If a tool has multiple installation methods on {{.OS}}, pick the most common/recommended one unless asked otherwise.

## Response Format
- For commands: provide the exact command(s) ready to be copied and run in {{.Shell}}.
- For errors: briefly state the likely cause and provide the fix.
- For complex issues: use a numbered list of steps.
- Wrap all commands and code in markdown code blocks with appropriate language tags.

## Critical Instructions
- Do NOT be verbose.
- Do NOT apologize or include pleasantries.
- Do NOT give unnecessary warnings or lectures.
- Do NOT provide alternatives for other OS/shells unless explicitly asked.
- If you don't know, say so briefly.`

func getOSInfo() string {
	if runtime.GOOS != "linux" {
		return runtime.GOOS
	}

	// Try to read /etc/os-release
	file, err := os.Open("/etc/os-release")
	if err != nil {
		return "linux"
	}
	defer file.Close()

	var prettyName, name, version string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			prettyName = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
		} else if strings.HasPrefix(line, "NAME=") {
			name = strings.Trim(strings.TrimPrefix(line, "NAME="), "\"")
		} else if strings.HasPrefix(line, "VERSION=") {
			version = strings.Trim(strings.TrimPrefix(line, "VERSION="), "\"")
		}
	}

	if prettyName != "" {
		return prettyName
	}
	if name != "" && version != "" {
		return fmt.Sprintf("%s %s", name, version)
	}
	if name != "" {
		return name
	}
	return "linux"
}

func chat(baseURL, apiKey, model, question string) error {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "sh"
	}
	// Extract shell name from path (e.g., /bin/zsh -> zsh)
	if idx := strings.LastIndex(shell, "/"); idx != -1 {
		shell = shell[idx+1:]
	}

	osInfo := getOSInfo()

	// Replace template variables
	systemPrompt := strings.ReplaceAll(systemPromptTemplate, "{{.OS}}", osInfo)
	systemPrompt = strings.ReplaceAll(systemPrompt, "{{.Arch}}", runtime.GOARCH)
	systemPrompt = strings.ReplaceAll(systemPrompt, "{{.Shell}}", shell)

	isTerminal := term.IsTerminal(int(os.Stdout.Fd()))
	if isTerminal {
		fmt.Print("Thinking...")
	}

	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: question},
	}

	reqBody := ChatRequest{
		Model:    model,
		Messages: messages,
		Stream:   false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return err
	}

	if len(chatResp.Choices) == 0 {
		fmt.Println("No response")
		return nil
	}

	content := chatResp.Choices[0].Message.Content
	if !isTerminal {
		fmt.Println(content)
		return nil
	}

	// Clear "Thinking..." line
	fmt.Print("\r\033[K")

	renderer, err := glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(100),
		glamour.WithEmoji(),
	)
	if err != nil {
		fmt.Println(content)
		return nil
	}

	out, err := renderer.Render(content)
	if err != nil {
		fmt.Println(content)
		return nil
	}
	fmt.Println(out)
	return nil
}
