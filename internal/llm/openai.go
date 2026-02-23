package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type OpenAIProvider struct {
	model  string
	apiKey string
	client *http.Client
}

func NewOpenAI(model, apiKey string) *OpenAIProvider {
	return &OpenAIProvider{
		model:  model,
		apiKey: apiKey,
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

func (o *OpenAIProvider) Generate(ctx context.Context, req Request) (string, error) {
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

		httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(jsonBody))
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
			lastErr = fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(respBody))
			continue
		}

		if resp.StatusCode != 200 {
			return "", fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(respBody))
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
		content := result.Choices[0].Message.Content
		if strings.TrimSpace(content) == "" {
			return "", fmt.Errorf("empty content in response")
		}
		return content, nil
	}

	return "", fmt.Errorf("OpenAI API failed after 3 attempts: %w", lastErr)
}

func maxTokensOrDefault(n int) int {
	if n > 0 {
		return n
	}
	return 4096
}

func temperatureOrDefault(t float64) float64 {
	if t > 0 {
		return t
	}
	return 0.7
}
