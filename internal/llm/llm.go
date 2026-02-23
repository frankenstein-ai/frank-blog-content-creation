package llm

import (
	"context"
	"fmt"
	"os"
)

type Provider interface {
	Generate(ctx context.Context, req Request) (string, error)
}

type Request struct {
	SystemPrompt string
	UserPrompt   string
	MaxTokens    int
	Temperature  float64
}

func New(provider, model, apiKey string) (Provider, error) {
	if model == "" {
		switch provider {
		case "openai":
			model = "gpt-4o"
		case "anthropic":
			model = "claude-sonnet-4-20250514"
		case "ollama":
			model = "llama3.2"
		case "openrouter":
			model = "anthropic/claude-sonnet-4-20250514"
		case "github":
			model = "openai/gpt-4o-mini"
		}
	}

	switch provider {
	case "openai":
		return NewOpenAI(model, apiKey), nil
	case "anthropic":
		return NewAnthropic(model, apiKey), nil
	case "ollama":
		return NewOllama(model, os.Getenv("OLLAMA_HOST")), nil
	case "openrouter":
		return NewOpenRouter(model, apiKey), nil
	case "github":
		return NewGitHub(model, apiKey), nil
	default:
		return nil, fmt.Errorf("unknown LLM provider: %q (use 'openai', 'anthropic', 'ollama', 'openrouter', or 'github')", provider)
	}
}
