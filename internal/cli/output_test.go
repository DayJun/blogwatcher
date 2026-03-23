package cli

import (
	"testing"
	"time"

	"github.com/Hyaxia/blogwatcher/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestFormatFields(t *testing.T) {
	article := model.Article{
		ID:    42,
		Title: "Test Article",
		URL:   "https://example.com",
	}
	blogNames := map[int64]string{1: "Test Blog"}
	article.BlogID = 1

	result := FormatArticleFields(&article, blogNames, []string{"id", "title", "blog"})
	assert.Equal(t, "42 | Test Article | Test Blog", result)
}

func TestFormatArticleDetail(t *testing.T) {
	published := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	article := model.Article{
		ID:            42,
		Title:         "Test Article",
		URL:           "https://example.com",
		PublishedDate:  &published,
		IsRead:        false,
		Content:       "Full content here",
	}
	blogNames := map[int64]string{1: "Test Blog"}
	article.BlogID = 1

	result := FormatArticleDetail(&article, blogNames, []string{"id", "title", "url", "blog", "published", "read", "content"})
	assert.Contains(t, result, "ID: 42")
	assert.Contains(t, result, "Title: Test Article")
	assert.Contains(t, result, "Read: No")
}