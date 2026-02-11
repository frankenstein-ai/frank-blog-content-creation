package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type OpenRouterProvider struct {
	model  string
	apiKey string
	client *http.Client
}

func NewOpenRouter(model, apiKey string) *OpenRouterProvider {
	return &OpenRouterProvider{
		model:  model,
		apiKey: apiKey,
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

func (o *OpenRouterProvider) Generate(ctx context.Context, req Request) (string, error) {
	body := map[string]any{
		"model": o.model,
		"messages": []map[string]string{
			{"role": "system", "content": req.SystemPrompt},
			{"role": "user", "content": req.UserPrompt},
		},
		"max_tokens":  maxTokensOrDefault(req.MaxTokens),
		"temperature": temperatureOrDefault(req.Temperature),
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	var lastErr error
	for attempt := range 3 {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt*2) * time.Second)
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewReader(jsonBody))
		if err != nil {
			return "", fmt.Errorf("creating request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)

		resp, err := o.client.Do(httpReq)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("OpenRouter API error (status %d): %s", resp.StatusCode, string(respBody))
			continue
		}

		if resp.StatusCode != 200 {
			return "", fmt.Errorf("OpenRouter API error (status %d): %s", resp.StatusCode, string(respBody))
		}

		var result struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(respBody, &result); err != nil {
			return "", fmt.Errorf("parsing response: %w", err)
		}
		if len(result.Choices) == 0 {
			return "", fmt.Errorf("no choices in response")
		}
		return result.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("OpenRouter API failed after 3 attempts: %w", lastErr)
}
