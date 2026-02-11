package llm

import (
	"context"
	"fmt"
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
		}
	}

	switch provider {
	case "openai":
		return NewOpenAI(model, apiKey), nil
	case "anthropic":
		return NewAnthropic(model, apiKey), nil
	default:
		return nil, fmt.Errorf("unknown LLM provider: %q (use 'openai' or 'anthropic')", provider)
	}
}
