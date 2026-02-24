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

type GitHubProvider struct {
	model  string
	apiKey string
	client *http.Client
}

func NewGitHub(model, apiKey string) *GitHubProvider {
	return &GitHubProvider{
		model:  model,
		apiKey: apiKey,
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

func (g *GitHubProvider) Generate(ctx context.Context, req Request) (string, error) {
	body := map[string]any{
		"model": g.model,
		"messages": []map[string]string{
			{"role": "system", "content": req.SystemPrompt},
			{"role": "user", "content": req.UserPrompt},
		},
		"max_completion_tokens": maxTokensOrDefault(req.MaxTokens),
	}
	if temp := temperatureOrDefault(req.Temperature); temp >= 0 {
		body["temperature"] = temp
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

		httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://models.github.ai/inference/chat/completions", bytes.NewReader(jsonBody))
		if err != nil {
			return "", fmt.Errorf("creating request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+g.apiKey)

		resp, err := g.client.Do(httpReq)
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
			lastErr = fmt.Errorf("GitHub Models API error (status %d): %s", resp.StatusCode, string(respBody))
			continue
		}

		if resp.StatusCode != 200 {
			return "", fmt.Errorf("GitHub Models API error (status %d): %s", resp.StatusCode, string(respBody))
		}

		var result struct {
			Choices []struct {
				FinishReason string `json:"finish_reason"`
				Message      struct {
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
			lastErr = fmt.Errorf("empty content in response (finish_reason: %s)", result.Choices[0].FinishReason)
			continue
		}
		return content, nil
	}

	return "", fmt.Errorf("GitHub Models API failed after 3 attempts: %w", lastErr)
}
