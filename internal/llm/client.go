package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Hyaxia/blogwatcher/internal/config"
)

const (
	defaultTimeout = 60 * time.Second
	maxContentLen  = 128000 // ~32k tokens
	maxTokens      = 500
)

type Client struct {
	config config.LLMConfig
	http   *http.Client
}

func NewClient(cfg config.LLMConfig) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = config.DefaultBaseURL
	}
	if cfg.Model == "" {
		cfg.Model = config.DefaultModel
	}

	return &Client{
		config: cfg,
		http:   &http.Client{Timeout: defaultTimeout},
	}
}

// HasAPIKey returns true if the client has an API key configured.
func (c *Client) HasAPIKey() bool {
	return c.config.APIKey != ""
}

func (c *Client) Summarize(ctx context.Context, title, content string) (string, error) {
	if c.config.APIKey == "" {
		return "", MissingAPIKeyError{}
	}

	// Truncate content if needed
	if len(content) > maxContentLen {
		content = content[:maxContentLen]
	}

	prompt := fmt.Sprintf("请用中文总结以下文章，2-3句话概括要点：\n\n标题：%s\n\n内容：%s", title, content)

	reqBody := map[string]interface{}{
		"model": c.config.Model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens": maxTokens,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.config.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		json.Unmarshal(respBody, &errResp)
		return "", APICallError{StatusCode: resp.StatusCode, Message: errResp.Error.Message}
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}

	if len(result.Choices) == 0 || result.Choices[0].Message.Content == "" {
		return "", APICallError{StatusCode: resp.StatusCode, Message: "empty response from API"}
	}

	return result.Choices[0].Message.Content, nil
}