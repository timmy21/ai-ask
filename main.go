package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"text/template"
	"time"

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

type ClaudeRequest struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system,omitempty"`
}

type ClaudeResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

type GeminiPart struct {
	Text    string `json:"text,omitempty"`
	Thought bool   `json:"thought,omitempty"`
}

type GeminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []GeminiPart `json:"parts"`
}

type GeminiRequest struct {
	Contents          []GeminiContent `json:"contents"`
	SystemInstruction *GeminiContent  `json:"systemInstruction,omitempty"`
}

type GeminiResponse struct {
	Candidates []struct {
		Content *GeminiContent `json:"content"`
	} `json:"candidates"`
}

const requestTimeout = 90 * time.Second

var httpClient = &http.Client{
	Timeout: requestTimeout,
}

var systemPromptTmpl = template.Must(template.New("systemPrompt").Parse(systemPromptTemplate))

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

	protocol := strings.ToLower(os.Getenv("AI_ASK_PROTOCOL"))
	if protocol == "" {
		protocol = "openai"
	}
	switch protocol {
	case "openai", "claude", "gemini":
	default:
		fmt.Fprintf(os.Stderr, "Error: unsupported AI_ASK_PROTOCOL %q (supported: openai, claude, gemini)\n", protocol)
		os.Exit(1)
	}

	// Check if data is being piped to stdin
	stat, err := os.Stdin.Stat()
	hasPipe := err == nil && (stat.Mode()&os.ModeCharDevice) == 0
	hasArgs := len(os.Args) >= 2

	var question string
	if hasPipe && hasArgs {
		// Case 1: cat error.log | ask "分析这个错误"
		pipeData, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to read stdin: %v\n", err)
			os.Exit(1)
		}
		argQuestion := strings.Join(os.Args[1:], " ")
		question = fmt.Sprintf("%s\n\n```\n%s```", argQuestion, string(pipeData))
	} else if hasPipe {
		// Case 2: echo "问题" | ask
		pipeData, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to read stdin: %v\n", err)
			os.Exit(1)
		}
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

	if err := chat(protocol, baseURL, apiKey, model, question); err != nil {
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
	if err := scanner.Err(); err != nil {
		return "linux"
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

func getSystemPrompt() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "sh"
	}
	// Extract shell name from path (e.g., /bin/zsh -> zsh)
	if idx := strings.LastIndex(shell, "/"); idx != -1 {
		shell = shell[idx+1:]
	}

	var out bytes.Buffer
	data := struct {
		OS    string
		Arch  string
		Shell string
	}{
		OS:    getOSInfo(),
		Arch:  runtime.GOARCH,
		Shell: shell,
	}
	if err := systemPromptTmpl.Execute(&out, data); err != nil {
		return systemPromptTemplate
	}
	return out.String()
}

func buildSystemFallbackPrefix() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "sh"
	}
	if idx := strings.LastIndex(shell, "/"); idx != -1 {
		shell = shell[idx+1:]
	}

	return fmt.Sprintf(
		"You MUST only provide solutions for %s, using %s syntax, and never include other operating systems.",
		getOSInfo(),
		shell,
	)
}

func withSystemFallbackInUser(question string) string {
	prefix := buildSystemFallbackPrefix()
	if strings.TrimSpace(question) == "" {
		return prefix
	}
	return prefix + "\n\n" + question
}

func chat(protocol, baseURL, apiKey, model, question string) error {
	isTerminal := term.IsTerminal(int(os.Stdout.Fd()))
	if isTerminal {
		fmt.Print("Thinking...")
	}

	var content string
	var err error

	switch protocol {
	case "claude":
		content, err = callClaude(baseURL, apiKey, model, question)
	case "gemini":
		content, err = callGemini(baseURL, apiKey, model, question)
	default:
		content, err = callOpenAI(baseURL, apiKey, model, question)
	}

	if isTerminal {
		fmt.Print("\r\033[K")
	}

	if err != nil {
		return err
	}

	if content == "" {
		fmt.Println("No response")
		return nil
	}

	if !isTerminal {
		fmt.Println(content)
		return nil
	}

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

func sanitizeForError(text string, sensitiveValues ...string) string {
	sanitized := text
	for _, v := range sensitiveValues {
		if v == "" {
			continue
		}
		sanitized = strings.ReplaceAll(sanitized, v, "[REDACTED]")
	}
	return sanitized
}

func doJSONRequest(ctx context.Context, method, url string, body []byte, headers map[string]string, sensitiveValues ...string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, sanitizeForError(string(respBody), sensitiveValues...))
	}
	return respBody, nil
}

func callOpenAI(baseURL, apiKey, model, question string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	systemPrompt := getSystemPrompt()
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
		return "", err
	}

	body, err := doJSONRequest(ctx, http.MethodPost, baseURL, jsonData, map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + apiKey,
	}, apiKey)
	if err != nil {
		return "", err
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", err
	}
	if len(chatResp.Choices) > 0 {
		return chatResp.Choices[0].Message.Content, nil
	}
	return "", nil
}

func callClaude(baseURL, apiKey, model, question string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	systemPrompt := getSystemPrompt()
	messages := []Message{
		{Role: "user", Content: withSystemFallbackInUser(question)},
	}

	reqBody := ClaudeRequest{
		Model:     model,
		Messages:  messages,
		MaxTokens: 4096,
		System:    systemPrompt,
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}
	body, err := doJSONRequest(ctx, http.MethodPost, baseURL, jsonData, map[string]string{
		"Content-Type":      "application/json",
		"x-api-key":         apiKey,
		"anthropic-version": "2023-06-01",
	}, apiKey)
	if err != nil {
		return "", err
	}

	var claudeResp ClaudeResponse
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		return "", err
	}

	var result strings.Builder
	for _, block := range claudeResp.Content {
		if block.Type == "text" || (block.Type == "" && block.Text != "") {
			result.WriteString(block.Text)
		}
	}

	if result.Len() > 0 {
		return result.String(), nil
	}
	return "", nil
}

func callGemini(baseURL, apiKey, model, question string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	systemPrompt := getSystemPrompt()

	reqBody := GeminiRequest{
		Contents: []GeminiContent{
			{
				Role: "user",
				Parts: []GeminiPart{
					{Text: withSystemFallbackInUser(question)},
				},
			},
		},
		SystemInstruction: &GeminiContent{
			Role: "system",
			Parts: []GeminiPart{
				{Text: systemPrompt},
			},
		},
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}
	fullUrl, err := url.JoinPath(baseURL, "models", fmt.Sprintf("%s:generateContent", model))
	if err != nil {
		return "", err
	}
	body, err := doJSONRequest(ctx, http.MethodPost, fullUrl, jsonData, map[string]string{
		"Content-Type":   "application/json",
		"x-goog-api-key": apiKey,
	}, apiKey)
	if err != nil {
		return "", err
	}

	var resp GeminiResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", err
	}

	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", nil
	}

	var texts []string
	for _, part := range resp.Candidates[0].Content.Parts {
		if part.Text != "" {
			if part.Thought {
				continue
			}
			texts = append(texts, part.Text)
		}
	}

	return strings.Join(texts, ""), nil
}
