package llm

import (
	"context"
	"fmt"

	"google.golang.org/genai"
)

// DefaultGeminiModel is the Gemini model used for generation (matches previous crev behaviour).
const DefaultGeminiModel = "gemini-2.0-flash"

// geminiClient wraps the Google Gen AI SDK for Gemini API calls.
// It implements the Client interface.
type geminiClient struct {
	client *genai.Client
	model  string
}

// NewGemini creates an LLM client that uses the Gemini API (Google AI / Gemini Developer API).
// apiKey can be CREV_API_KEY or a Google AI API key.
// The returned Client uses DefaultGeminiModel; use NewGeminiWithModel for a custom model.
func NewGemini(ctx context.Context, apiKey string) (Client, error) {
	return NewGeminiWithModel(ctx, apiKey, DefaultGeminiModel)
}

// NewGeminiWithModel is like NewGemini but uses the specified model (e.g. "gemini-2.0-flash", "gemini-2.5-flash").
func NewGeminiWithModel(ctx context.Context, apiKey string, model string) (Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("gemini: API key is required")
	}
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("gemini: create client: %w", err)
	}
	if model == "" {
		model = DefaultGeminiModel
	}
	return &geminiClient{client: client, model: model}, nil
}

// GenerateText sends a text prompt to Gemini and returns the generated text.
func (c *geminiClient) GenerateText(ctx context.Context, prompt string) (string, error) {
	result, err := c.client.Models.GenerateContent(ctx, c.model, genai.Text(prompt), nil)
	if err != nil {
		return "", fmt.Errorf("gemini: generate content: %w", err)
	}
	return result.Text(), nil
}
