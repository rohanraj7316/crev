package llm

import (
	"context"
	"fmt"
)

// Provider identifies an LLM backend. Use with NewClient to get a Client by name.
const (
	ProviderGemini = "gemini"
)

// Client is the interface for any LLM provider (Gemini, OpenAI, Claude, etc.).
// Implementations must be safe for concurrent use.
type Client interface {
	// GenerateText sends a text prompt to the LLM and returns the generated text.
	GenerateText(ctx context.Context, prompt string) (string, error)
}

// NewClient returns an LLM client for the given provider. apiKey is passed to the provider's auth.
// Supported providers: "gemini", "gemini-cli". Returns an error for unknown provider.
func NewClient(ctx context.Context, provider string, apiKey string) (Client, error) {
	switch provider {
	case ProviderGemini:
		return NewGemini(ctx, apiKey)
	case ProviderGeminiCLI:
		return NewGeminiCLI()
	default:
		return nil, fmt.Errorf("llm: unknown provider %q (supported: %s, %s)", provider, ProviderGemini, ProviderGeminiCLI)
	}
}
