package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Hyaxia/blogwatcher/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSummarize(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		var req map[string]interface{}
		json.NewDecoder(r.Body).Decode(&req)
		assert.Contains(t, req, "messages")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]string{
						"content": "This is a summary.",
					},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(config.LLMConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Model:   "gpt-4o-mini",
	})

	summary, err := client.Summarize(context.Background(), "Test Title", "Test content")
	require.NoError(t, err)
	assert.Equal(t, "This is a summary.", summary)
}

func TestSummarizeMissingAPIKey(t *testing.T) {
	client := NewClient(config.LLMConfig{
		BaseURL: "https://api.openai.com/v1",
		Model:   "gpt-4o-mini",
	})
	_, err := client.Summarize(context.Background(), "Title", "Content")
	assert.IsType(t, MissingAPIKeyError{}, err)
}

func TestHasAPIKey(t *testing.T) {
	client := NewClient(config.LLMConfig{})
	assert.False(t, client.HasAPIKey())

	client = NewClient(config.LLMConfig{APIKey: "test"})
	assert.True(t, client.HasAPIKey())
}