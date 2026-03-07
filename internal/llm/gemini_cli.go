package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

const ProviderGeminiCLI = "gemini-cli"

// geminiCLIResponse is the JSON structure returned by `gemini --output-format json`.
type geminiCLIResponse struct {
	Response string `json:"response"`
	Error    string `json:"error,omitempty"`
}

// GeminiCLIClient shells out to the `gemini` CLI binary for LLM calls.
// Uses the user's browser-based Google auth — no API key needed.
type GeminiCLIClient struct {
	binaryPath string
}

// NewGeminiCLI creates a Client that uses the locally installed Gemini CLI.
// Returns an error if the binary is not found on PATH.
func NewGeminiCLI() (Client, error) {
	path, err := FindGeminiBinary()
	if err != nil {
		return nil, err
	}
	return &GeminiCLIClient{binaryPath: path}, nil
}

// FindGeminiBinary locates the gemini binary on PATH.
func FindGeminiBinary() (string, error) {
	path, err := exec.LookPath("gemini")
	if err != nil {
		return "", fmt.Errorf("gemini-cli: binary not found on PATH (install: npm install -g @anthropic-ai/gemini-cli)")
	}
	return path, nil
}

// CheckAuth verifies the Gemini CLI is authenticated by running a trivial prompt.
func CheckAuth(ctx context.Context) error {
	path, err := FindGeminiBinary()
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, path, "--prompt", "ping", "--output-format", "json")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return fmt.Errorf("gemini-cli: auth check failed: %s", errMsg)
	}

	if len(out) == 0 {
		return fmt.Errorf("gemini-cli: empty response from auth check")
	}

	return nil
}

// GenerateText sends a prompt to the Gemini CLI and returns the response text.
func (c *GeminiCLIClient) GenerateText(ctx context.Context, prompt string) (string, error) {
	cmd := exec.CommandContext(ctx, c.binaryPath, "--prompt", prompt)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return "", fmt.Errorf("gemini-cli: %s", errMsg)
	}

	return strings.TrimSpace(string(out)), nil
}

// GenerateTextWithFile sends a prompt plus file content (piped via stdin) to the Gemini CLI.
func (c *GeminiCLIClient) GenerateTextWithFile(ctx context.Context, prompt string, fileContent string) (string, error) {
	cmd := exec.CommandContext(ctx, c.binaryPath, "--prompt", prompt)
	cmd.Stdin = strings.NewReader(fileContent)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return "", fmt.Errorf("gemini-cli: %s", errMsg)
	}

	return strings.TrimSpace(string(out)), nil
}

// GenerateJSON sends a prompt and returns the raw JSON response from Gemini CLI.
func (c *GeminiCLIClient) GenerateJSON(ctx context.Context, prompt string, fileContent string) (json.RawMessage, error) {
	args := []string{"--prompt", prompt, "--output-format", "json"}
	cmd := exec.CommandContext(ctx, c.binaryPath, args...)
	if fileContent != "" {
		cmd.Stdin = strings.NewReader(fileContent)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return nil, fmt.Errorf("gemini-cli: %s", errMsg)
	}

	return json.RawMessage(out), nil
}
