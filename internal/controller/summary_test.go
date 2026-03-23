package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/Hyaxia/blogwatcher/internal/llm"
	"github.com/Hyaxia/blogwatcher/internal/model"
	"github.com/Hyaxia/blogwatcher/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetArticleContent(t *testing.T) {
	tests := []struct {
		name     string
		article  model.Article
		expected string
	}{
		{
			name: "prefers content",
			article: model.Article{
				Content:     "Full content",
				Description: "Description",
				FeedSummary: "Summary",
			},
			expected: "Full content",
		},
		{
			name: "falls back to feed_summary",
			article: model.Article{
				Description: "Description",
				FeedSummary: "Summary",
			},
			expected: "Summary",
		},
		{
			name: "falls back to description",
			article: model.Article{
				Description: "Description",
			},
			expected: "Description",
		},
		{
			name:     "empty when no content",
			article:  model.Article{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetArticleContent(&tt.article)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateSummary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "Test summary"}},
			},
		})
	}))
	defer server.Close()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := storage.OpenDatabase(path)
	require.NoError(t, err)
	defer db.Close()

	blog, err := db.AddBlog(model.Blog{Name: "Test", URL: "https://example.com"})
	require.NoError(t, err)

	article, err := db.AddArticle(model.Article{
		BlogID:  blog.ID,
		Title:   "Test Article",
		URL:     "https://example.com/1",
		Content: "<p>Test content</p>",
	})
	require.NoError(t, err)

	client := llm.NewClient(llm.Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	result, err := GenerateSummary(context.Background(), db, client, article.ID, false)
	require.NoError(t, err)
	assert.Equal(t, "Test summary", result.Summary)

	// Test article not found
	_, err = GenerateSummary(context.Background(), db, client, 999, false)
	assert.IsType(t, ArticleNotFoundError{}, err)
}