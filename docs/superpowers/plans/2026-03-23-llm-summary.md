# LLM Summary Feature Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add LLM-powered article summarization with OpenAI-format API support.

**Architecture:** Store RSS raw content fields (Content, Description, FeedSummary) during scan. Add LLM client for OpenAI-format APIs. Implement `summary` CLI command for on-demand summarization.

**Tech Stack:** Go 1.24, SQLite, goquery (HTML cleaning), gofeed (RSS), cobra (CLI)

---

## Task 1: Update Article Model

**Files:**
- Modify: `internal/model/model.go`

- [ ] **Step 1: Add new fields to Article struct**

```go
type Article struct {
	ID             int64
	BlogID         int64
	Title          string
	URL            string
	PublishedDate  *time.Time
	DiscoveredDate *time.Time
	IsRead         bool
	Content        string // New: RSS content/encoded
	Description    string // New: RSS description
	FeedSummary    string // New: RSS/Atom summary
	Summary        string // New: LLM-generated summary
}
```

- [ ] **Step 2: Run tests to verify no breaking changes**

Run: `go test ./...`
Expected: All tests pass

- [ ] **Step 3: Commit**

```bash
git add internal/model/model.go
git commit -m "feat(model): add content fields to Article struct"
```

---

## Task 2: Implement Database Migration

**Files:**
- Modify: `internal/storage/database.go`
- Modify: `internal/storage/database_test.go`

- [ ] **Step 1: Write test for migration idempotency**

```go
func TestMigrateSchemaIdempotent(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	require.NoError(t, err)
	defer db.Close()

	// Create blog and article first
	blog, err := db.AddBlog(model.Blog{Name: "Test", URL: "https://example.com"})
	require.NoError(t, err)
	_, err = db.AddArticle(model.Article{BlogID: blog.ID, Title: "Test", URL: "https://example.com/1"})
	require.NoError(t, err)

	// Run migration twice
	err = db.MigrateSchema()
	require.NoError(t, err)
	err = db.MigrateSchema()
	require.NoError(t, err)

	// Verify columns exist by querying article with new fields
	article, err := db.GetArticle(1)
	require.NoError(t, err)
	// New fields should be empty strings, not cause errors
	assert.Equal(t, "", article.Content)
	assert.Equal(t, "", article.Description)
	assert.Equal(t, "", article.FeedSummary)
	assert.Equal(t, "", article.Summary)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/storage/... -run TestMigrateSchemaIdempotent -v`
Expected: FAIL with "MigrateSchema not defined"

- [ ] **Step 3: Implement MigrateSchema method**

Add to `internal/storage/database.go`:

```go
func (db *Database) MigrateSchema() error {
	rows, err := db.conn.Query("PRAGMA table_info(articles)")
	if err != nil {
		return err
	}
	defer rows.Close()

	existingCols := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dfltValue sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			return err
		}
		existingCols[name] = true
	}

	newCols := []string{"content", "description", "feed_summary", "summary"}
	for _, col := range newCols {
		if !existingCols[col] {
			_, err := db.conn.Exec(fmt.Sprintf("ALTER TABLE articles ADD COLUMN %s TEXT", col))
			if err != nil {
				return err
			}
		}
	}
	return nil
}
```

- [ ] **Step 4: Call MigrateSchema in OpenDatabase**

Update `OpenDatabase` function:

```go
func OpenDatabase(path string) (*Database, error) {
	// ... existing code ...

	db := &Database{path: path, conn: conn}
	if err := db.init(); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := db.MigrateSchema(); err != nil {  // Add this line
		_ = conn.Close()
		return nil, err
	}
	return db, nil
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/storage/... -run TestMigrateSchemaIdempotent -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/storage/database.go internal/storage/database_test.go
git commit -m "feat(storage): add idempotent schema migration for content fields"
```

---

## Task 3: Update Database CRUD Operations

**Files:**
- Modify: `internal/storage/database.go`
- Modify: `internal/storage/database_test.go`

- [ ] **Step 1: Write test for content fields round-trip**

```go
func TestArticleContentFieldsRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	require.NoError(t, err)
	defer db.Close()

	// Create blog and article with content fields
	blog, err := db.AddBlog(model.Blog{Name: "Test", URL: "https://example.com"})
	require.NoError(t, err)

	article := model.Article{
		BlogID:      blog.ID,
		Title:       "Test Article",
		URL:         "https://example.com/1",
		Content:     "<p>Full HTML content</p>",
		Description: "Short description",
		FeedSummary: "Feed summary",
		Summary:     "LLM summary",
	}

	created, err := db.AddArticle(article)
	require.NoError(t, err)
	assert.NotZero(t, created.ID)

	// Retrieve and verify
	got, err := db.GetArticle(created.ID)
	require.NoError(t, err)
	assert.Equal(t, article.Content, got.Content)
	assert.Equal(t, article.Description, got.Description)
	assert.Equal(t, article.FeedSummary, got.FeedSummary)
	assert.Equal(t, article.Summary, got.Summary)
}
```

- [ ] **Step 2: Write test for UpdateArticleSummary**

```go
func TestUpdateArticleSummary(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	require.NoError(t, err)
	defer db.Close()

	blog, err := db.AddBlog(model.Blog{Name: "Test", URL: "https://example.com"})
	require.NoError(t, err)

	article, err := db.AddArticle(model.Article{
		BlogID: blog.ID,
		Title:  "Test",
		URL:    "https://example.com/1",
	})
	require.NoError(t, err)

	// Update summary
	err = db.UpdateArticleSummary(article.ID, "New summary")
	require.NoError(t, err)

	// Verify
	got, err := db.GetArticle(article.ID)
	require.NoError(t, err)
	assert.Equal(t, "New summary", got.Summary)
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/storage/... -run "TestArticleContentFieldsRoundTrip|TestUpdateArticleSummary" -v`
Expected: FAIL (columns don't exist in queries yet)

- [ ] **Step 4: Update scanArticle function**

Update the `scanArticle` function to read new columns:

```go
func scanArticle(scanner interface{ Scan(dest ...any) error }) (*model.Article, error) {
	var (
		id            int64
		blogID        int64
		title         string
		url           string
		publishedDate sql.NullString
		discovered    sql.NullString
		isRead        bool
		content       sql.NullString  // New
		description   sql.NullString  // New
		feedSummary   sql.NullString  // New
		summary       sql.NullString  // New
	)
	if err := scanner.Scan(&id, &blogID, &title, &url, &publishedDate, &discovered, &isRead, &content, &description, &feedSummary, &summary); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	article := &model.Article{
		ID:           id,
		BlogID:       blogID,
		Title:        title,
		URL:          url,
		IsRead:       isRead,
		Content:      content.String,
		Description:  description.String,
		FeedSummary:  feedSummary.String,
		Summary:      summary.String,
	}
	if publishedDate.Valid {
		if parsed, err := parseTime(publishedDate.String); err == nil {
			article.PublishedDate = &parsed
		}
	}
	if discovered.Valid {
		if parsed, err := parseTime(discovered.String); err == nil {
			article.DiscoveredDate = &parsed
		}
	}

	return article, nil
}
```

- [ ] **Step 5: Update SELECT queries**

Update `GetArticle`, `GetArticleByURL`, `ListArticles` queries to include new columns:

```go
// GetArticle (line ~243)
row := db.conn.QueryRow(`SELECT id, blog_id, title, url, published_date, discovered_date, is_read, content, description, feed_summary, summary FROM articles WHERE id = ?`, id)

// GetArticleByURL (line ~248)
row := db.conn.QueryRow(`SELECT id, blog_id, title, url, published_date, discovered_date, is_read, content, description, feed_summary, summary FROM articles WHERE url = ?`, url)

// ListArticles (line ~305)
query := `SELECT id, blog_id, title, url, published_date, discovered_date, is_read, content, description, feed_summary, summary FROM articles WHERE 1=1`
```

- [ ] **Step 6: Update AddArticle function**

```go
func (db *Database) AddArticle(article model.Article) (model.Article, error) {
	result, err := db.conn.Exec(
		`INSERT INTO articles (blog_id, title, url, published_date, discovered_date, is_read, content, description, feed_summary, summary)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		article.BlogID,
		article.Title,
		article.URL,
		formatTimePtr(article.PublishedDate),
		formatTimePtr(article.DiscoveredDate),
		article.IsRead,
		nullIfEmpty(article.Content),
		nullIfEmpty(article.Description),
		nullIfEmpty(article.FeedSummary),
		nullIfEmpty(article.Summary),
	)
	if err != nil {
		return article, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return article, err
	}
	article.ID = id
	return article, nil
}
```

- [ ] **Step 7: Update AddArticlesBulk function**

```go
func (db *Database) AddArticlesBulk(articles []model.Article) (int, error) {
	if len(articles) == 0 {
		return 0, nil
	}
	_tx, err := db.conn.Begin()
	if err != nil {
		return 0, err
	}
	stmt, err := _tx.Prepare(`INSERT INTO articles (blog_id, title, url, published_date, discovered_date, is_read, content, description, feed_summary, summary) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		_ = _tx.Rollback()
		return 0, err
	}
	defer stmt.Close()

	for _, article := range articles {
		_, err := stmt.Exec(
			article.BlogID,
			article.Title,
			article.URL,
			formatTimePtr(article.PublishedDate),
			formatTimePtr(article.DiscoveredDate),
			article.IsRead,
			nullIfEmpty(article.Content),
			nullIfEmpty(article.Description),
			nullIfEmpty(article.FeedSummary),
			nullIfEmpty(article.Summary),
		)
		if err != nil {
			_ = _tx.Rollback()
			return 0, err
		}
	}
	if err := _tx.Commit(); err != nil {
		return 0, err
	}
	return len(articles), nil
}
```

- [ ] **Step 8: Add UpdateArticleSummary method**

```go
func (db *Database) UpdateArticleSummary(id int64, summary string) error {
	_, err := db.conn.Exec(`UPDATE articles SET summary = ? WHERE id = ?`, summary, id)
	return err
}
```

- [ ] **Step 9: Run tests to verify they pass**

Run: `go test ./internal/storage/... -v`
Expected: All tests pass

- [ ] **Step 10: Commit**

```bash
git add internal/storage/database.go internal/storage/database_test.go
git commit -m "feat(storage): update CRUD operations for content fields"
```

---

## Task 4: Update RSS Parser

**Files:**
- Modify: `internal/rss/rss.go`
- Modify: `internal/rss/rss_test.go`

- [ ] **Step 1: Write test for content extraction**

```go
func TestParseFeedWithContent(t *testing.T) {
	// Start test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:content="http://purl.org/rss/1.0/modules/content/">
<channel><title>Test</title>
<item>
<title>Test Article</title>
<link>https://example.com/article</link>
<description>Short description</description>
<content:encoded><![CDATA[<p>Full content here</p>]]></content:encoded>
</item>
</channel>
</rss>`))
	}))
	defer server.Close()

	articles, err := ParseFeed(server.URL, 10*time.Second)
	require.NoError(t, err)
	require.Len(t, articles, 1)

	assert.Equal(t, "Test Article", articles[0].Title)
	assert.Equal(t, "Short description", articles[0].Description)
	assert.Contains(t, articles[0].Content, "Full content here")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/rss/... -run TestParseFeedWithContent -v`
Expected: FAIL with "Content field not populated"

- [ ] **Step 3: Update FeedArticle struct**

```go
type FeedArticle struct {
	Title         string
	URL           string
	PublishedDate *time.Time
	Content       string // New
	Description   string // New
	FeedSummary   string // New
}
```

- [ ] **Step 4: Update ParseFeed function**

```go
articles = append(articles, FeedArticle{
	Title:         title,
	URL:           link,
	PublishedDate: pickPublishedDate(item),
	Content:       item.Content,
	Description:   item.Description,
	FeedSummary:   item.Summary,
})
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/rss/... -run TestParseFeedWithContent -v`
Expected: PASS

- [ ] **Step 6: Run all tests**

Run: `go test ./...`
Expected: All tests pass

- [ ] **Step 7: Commit**

```bash
git add internal/rss/rss.go internal/rss/rss_test.go
git commit -m "feat(rss): extract content, description, and summary fields"
```

---

## Task 5: Update Scanner

**Files:**
- Modify: `internal/scanner/scanner.go`
- Modify: `internal/scanner/scanner_test.go`

- [ ] **Step 1: Write test for content field passing**

```go
func TestConvertFeedArticlesPreservesContent(t *testing.T) {
	published := time.Now()
	feedArticles := []rss.FeedArticle{
		{
			Title:         "Test",
			URL:           "https://example.com/1",
			PublishedDate: &published,
			Content:       "<p>Content</p>",
			Description:   "Description",
			FeedSummary:   "Summary",
		},
	}

	result := convertFeedArticles(1, feedArticles)
	require.Len(t, result, 1)
	assert.Equal(t, "<p>Content</p>", result[0].Content)
	assert.Equal(t, "Description", result[0].Description)
	assert.Equal(t, "Summary", result[0].FeedSummary)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scanner/... -run TestConvertFeedArticlesPreservesContent -v`
Expected: FAIL (content fields empty)

- [ ] **Step 3: Update convertFeedArticles function**

```go
func convertFeedArticles(blogID int64, articles []rss.FeedArticle) []model.Article {
	result := make([]model.Article, 0, len(articles))
	for _, article := range articles {
		result = append(result, model.Article{
			BlogID:        blogID,
			Title:         article.Title,
			URL:           article.URL,
			PublishedDate: article.PublishedDate,
			IsRead:        false,
			Content:       article.Content,
			Description:   article.Description,
			FeedSummary:   article.FeedSummary,
		})
	}
	return result
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scanner/... -run TestConvertFeedArticlesPreservesContent -v`
Expected: PASS

- [ ] **Step 5: Run all tests**

Run: `go test ./...`
Expected: All tests pass

- [ ] **Step 6: Commit**

```bash
git add internal/scanner/scanner.go internal/scanner/scanner_test.go
git commit -m "feat(scanner): pass content fields to Article model"
```

---

## Task 6: Create HTML Cleaner

**Files:**
- Create: `internal/html/cleaner.go`
- Create: `internal/html/cleaner_test.go`

- [ ] **Step 1: Write test for HTML cleaning**

```go
package html

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToPlainText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple paragraph",
			input:    "<p>Hello World</p>",
			expected: "Hello World",
		},
		{
			name:     "multiple paragraphs",
			input:    "<p>First</p><p>Second</p>",
			expected: "First",
		},
		{
			name:     "with links",
			input:    `<p>Check <a href="https://example.com">this link</a></p>`,
			expected: "Check this link",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "plain text",
			input:    "Just plain text",
			expected: "Just plain text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToPlainText(tt.input)
			assert.Contains(t, result, tt.expected)
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/html/... -v`
Expected: FAIL with "package html not found"

- [ ] **Step 3: Create html package and implement ToPlainText**

```go
package html

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ToPlainText extracts plain text from HTML content.
func ToPlainText(html string) string {
	if html == "" {
		return ""
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return html // fallback: return as-is
	}

	return doc.Text()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/html/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/html/cleaner.go internal/html/cleaner_test.go
git commit -m "feat(html): add HTML to plain text cleaner"
```

---

## Task 7: Create LLM Error Types

**Files:**
- Create: `internal/llm/errors.go`

- [ ] **Step 1: Create LLM error types**

```go
package llm

import "fmt"

type MissingAPIKeyError struct{}

func (e MissingAPIKeyError) Error() string {
	return "OPENAI_API_KEY environment variable is not set"
}

type APICallError struct {
	StatusCode int
	Message    string
}

func (e APICallError) Error() string {
	return fmt.Sprintf("LLM API error (status %d): %s", e.StatusCode, e.Message)
}

type NoContentError struct {
	ArticleID int64
}

func (e NoContentError) Error() string {
	return fmt.Sprintf("Article %d has no content to summarize", e.ArticleID)
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/llm/errors.go
git commit -m "feat(llm): add error types for LLM operations"
```

---

## Task 8: Create LLM Client

**Files:**
- Create: `internal/llm/client.go`
- Create: `internal/llm/client_test.go`

- [ ] **Step 1: Write test for LLM client with mock server**

```go
package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

	client := NewClient(Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Model:   "gpt-4o-mini",
		Timeout: 10 * time.Second,
	})

	summary, err := client.Summarize(context.Background(), "Test Title", "Test content")
	require.NoError(t, err)
	assert.Equal(t, "This is a summary.", summary)
}

func TestSummarizeMissingAPIKey(t *testing.T) {
	client := NewClient(Config{
		BaseURL: "https://api.openai.com/v1",
		Model:   "gpt-4o-mini",
	})
	_, err := client.Summarize(context.Background(), "Title", "Content")
	assert.IsType(t, MissingAPIKeyError{}, err)
}

func TestHasAPIKey(t *testing.T) {
	client := NewClient(Config{})
	assert.False(t, client.HasAPIKey())

	client = NewClient(Config{APIKey: "test"})
	assert.True(t, client.HasAPIKey())
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/llm/... -v`
Expected: FAIL with "client not defined"

- [ ] **Step 3: Implement LLM client**

```go
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	defaultBaseURL = "https://api.openai.com/v1"
	defaultModel   = "gpt-4o-mini"
	defaultTimeout = 60 * time.Second
	maxContentLen  = 128000 // ~32k tokens
	maxTokens      = 500
)

type Config struct {
	APIKey  string
	BaseURL string
	Model   string
	Timeout time.Duration
}

type Client struct {
	config Config
	http   *http.Client
}

func NewClient(config Config) *Client {
	if config.BaseURL == "" {
		config.BaseURL = defaultBaseURL
	}
	if config.Model == "" {
		config.Model = defaultModel
	}
	if config.Timeout == 0 {
		config.Timeout = defaultTimeout
	}
	if config.APIKey == "" {
		config.APIKey = os.Getenv("OPENAI_API_KEY")
	}

	return &Client{
		config: config,
		http:   &http.Client{Timeout: config.Timeout},
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/llm/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/llm/client.go internal/llm/client_test.go
git commit -m "feat(llm): add OpenAI-format API client for summarization"
```

---

## Task 9: Add Summary Generation to Controller

**Files:**
- Create: `internal/controller/summary.go`
- Create: `internal/controller/summary_test.go`

- [ ] **Step 1: Write test for GetArticleContent**

```go
package controller

import (
	"testing"

	"github.com/Hyaxia/blogwatcher/internal/model"
	"github.com/stretchr/testify/assert"
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controller/... -run TestGetArticleContent -v`
Expected: FAIL with "GetArticleContent not defined"

- [ ] **Step 3: Implement GetArticleContent**

Create `internal/controller/summary.go`:

```go
package controller

import (
	"context"

	"github.com/Hyaxia/blogwatcher/internal/html"
	"github.com/Hyaxia/blogwatcher/internal/llm"
	"github.com/Hyaxia/blogwatcher/internal/model"
	"github.com/Hyaxia/blogwatcher/internal/storage"
)

// GetArticleContent returns the best available content for summarization.
// Priority: Content > FeedSummary > Description
func GetArticleContent(article *model.Article) string {
	if article.Content != "" {
		return article.Content
	}
	if article.FeedSummary != "" {
		return article.FeedSummary
	}
	return article.Description
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/controller/... -run TestGetArticleContent -v`
Expected: PASS

- [ ] **Step 5: Write test for GenerateSummary**

```go
func TestGenerateSummary(t *testing.T) {
	// Setup mock LLM server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "Test summary"}},
			},
		})
	}))
	defer server.Close()

	// Setup test database
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := storage.OpenDatabase(path)
	require.NoError(t, err)
	defer db.Close()

	blog, err := db.AddBlog(model.Blog{Name: "Test", URL: "https://example.com"})
	require.NoError(t, err)

	article, err := db.AddArticle(model.Article{
		BlogID:      blog.ID,
		Title:       "Test Article",
		URL:         "https://example.com/1",
		Content:     "<p>Test content</p>",
	})
	require.NoError(t, err)

	// Test successful generation
	client := llm.NewClient(llm.Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	result, err := GenerateSummary(context.Background(), db, client, article.ID, false)
	require.NoError(t, err)
	assert.Equal(t, "Test summary", result.Summary)

	// Test force flag regenerates
	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "New summary"}},
			},
		})
	}))
	defer server2.Close()

	client2 := llm.NewClient(llm.Config{APIKey: "test-key", BaseURL: server2.URL})
	result, err = GenerateSummary(context.Background(), db, client2, article.ID, true)
	require.NoError(t, err)
	assert.Equal(t, "New summary", result.Summary)

	// Test no content error
	article2, err := db.AddArticle(model.Article{
		BlogID: blog.ID,
		Title:  "Empty Article",
		URL:    "https://example.com/2",
	})
	require.NoError(t, err)

	_, err = GenerateSummary(context.Background(), db, client, article2.ID, false)
	assert.IsType(t, llm.NoContentError{}, err)

	// Test article not found
	_, err = GenerateSummary(context.Background(), db, client, 999, false)
	assert.IsType(t, ArticleNotFoundError{}, err)
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `go test ./internal/controller/... -run TestGenerateSummary -v`
Expected: FAIL with "GenerateSummary not defined"

- [ ] **Step 7: Implement GenerateSummary and GenerateAllSummaries**

Add to `internal/controller/summary.go`:

```go
// GenerateSummary generates an LLM summary for a single article.
func GenerateSummary(ctx context.Context, db *storage.Database, client *llm.Client, articleID int64, force bool) (*model.Article, error) {
	article, err := db.GetArticle(articleID)
	if err != nil {
		return nil, err
	}
	if article == nil {
		return nil, ArticleNotFoundError{ID: articleID}
	}

	if article.Summary != "" && !force {
		return article, nil
	}

	content := GetArticleContent(article)
	if content == "" {
		return nil, llm.NoContentError{ArticleID: articleID}
	}

	plainText := html.ToPlainText(content)
	if plainText == "" {
		return nil, llm.NoContentError{ArticleID: articleID}
	}

	summary, err := client.Summarize(ctx, article.Title, plainText)
	if err != nil {
		return nil, err
	}

	if err := db.UpdateArticleSummary(articleID, summary); err != nil {
		return nil, err
	}

	article.Summary = summary
	return article, nil
}

// SummaryResult represents the result of a single summary generation.
type SummaryResult struct {
	ArticleID int64
	Title     string
	Status    string // "generated", "skipped", "error"
	Error     error
}

// GenerateAllSummaries generates summaries for all articles without one.
func GenerateAllSummaries(ctx context.Context, db *storage.Database, client *llm.Client, force bool) []SummaryResult {
	articles, err := db.ListArticles(nil, nil, 1, storage.NoPagination)
	if err != nil {
		return []SummaryResult{{Status: "error", Error: err}}
	}

	var results []SummaryResult
	for _, article := range articles {
		if article.Summary != "" && !force {
			continue
		}

		result := SummaryResult{
			ArticleID: article.ID,
			Title:     article.Title,
		}

		_, err := GenerateSummary(ctx, db, client, article.ID, force)
		if err != nil {
			result.Status = "error"
			result.Error = err
		} else {
			result.Status = "generated"
		}

		results = append(results, result)
	}

	return results
}
```

- [ ] **Step 8: Run all tests**

Run: `go test ./internal/controller/... -v`
Expected: All tests pass

- [ ] **Step 9: Commit**

```bash
git add internal/controller/summary.go internal/controller/summary_test.go
git commit -m "feat(controller): add summary generation logic"
```

---

## Task 10: Add Summary CLI Command

**Files:**
- Modify: `internal/cli/commands.go`
- Modify: `internal/cli/root.go`

- [ ] **Step 1: Add newSummaryCommand function**

Add imports to `internal/cli/commands.go`:

```go
import (
	"context"
	"errors"
	// ... existing imports ...
)
```

Add command function:

```go
func newSummaryCommand() *cobra.Command {
	var allFlag bool
	var forceFlag bool

	cmd := &cobra.Command{
		Use:   "summary [article_id]",
		Short: "Generate LLM summary for articles.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()

			llmClient := llm.NewClient(llm.Config{})
			if !llmClient.HasAPIKey() {
				err := llm.MissingAPIKeyError{}
				printError(err)
				return markError(err)
			}

			ctx := cmd.Context()

			if allFlag {
				return runSummaryAll(ctx, db, llmClient, forceFlag)
			}

			if len(args) == 0 {
				return fmt.Errorf("article_id is required unless --all is specified")
			}

			articleID, err := parseID(args[0])
			if err != nil {
				return err
			}

			return runSummarySingle(ctx, db, llmClient, articleID, forceFlag)
		},
	}

	cmd.Flags().BoolVar(&allFlag, "all", false, "Generate summaries for all articles without one")
	cmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "Regenerate even if summary exists")
	return cmd
}

func runSummarySingle(ctx context.Context, db *storage.Database, client *llm.Client, articleID int64, force bool) error {
	article, err := controller.GenerateSummary(ctx, db, client, articleID, force)
	if err != nil {
		printError(err)
		return markError(err)
	}

	color.New(color.FgCyan, color.Bold).Printf("Summary for article %d:\n", articleID)
	fmt.Println(article.Summary)
	return nil
}

func runSummaryAll(ctx context.Context, db *storage.Database, client *llm.Client, force bool) error {
	articles, err := db.ListArticles(nil, nil, 1, storage.NoPagination)
	if err != nil {
		return err
	}

	// Filter to only articles without summary (unless force)
	var toProcess []model.Article
	for _, a := range articles {
		if a.Summary == "" || force {
			toProcess = append(toProcess, a)
		}
	}

	if len(toProcess) == 0 {
		color.New(color.FgGreen).Println("All articles already have summaries.")
		return nil
	}

	color.New(color.FgCyan).Printf("Processing %d articles...\n\n", len(toProcess))

	generated := 0
	skipped := 0
	failed := 0

	for _, article := range toProcess {
		_, err := controller.GenerateSummary(ctx, db, client, article.ID, force)
		if err != nil {
			var noContentErr llm.NoContentError
			if errors.As(err, &noContentErr) {
				color.New(color.FgYellow).Printf("⚠ Article %d: No content available\n", article.ID)
				skipped++
			} else {
				color.New(color.FgRed).Printf("✗ Article %d: %s\n", article.ID, err.Error())
				failed++
			}
		} else {
			color.New(color.FgGreen).Printf("✓ Article %d: Summary generated\n", article.ID)
			generated++
		}
	}

	fmt.Println()
	color.New(color.FgCyan, color.Bold).Printf("Complete: %d generated, %d skipped, %d failed\n", generated, skipped, failed)
	return nil
}
```

- [ ] **Step 2: Register command in root.go**

Read `internal/cli/root.go` and add the command in `NewRootCommand()` function after other command registrations:

```go
rootCmd.AddCommand(newSummaryCommand())
```

- [ ] **Step 3: Build and test manually**

Run: `go build ./cmd/blogwatcher && ./blogwatcher summary --help`
Expected: Shows help for summary command

- [ ] **Step 4: Run all tests**

Run: `go test ./...`
Expected: All tests pass

- [ ] **Step 5: Commit**

```bash
git add internal/cli/commands.go internal/cli/root.go
git commit -m "feat(cli): add summary command for LLM summarization"
```

---

## Task 11: Final Integration Test

**Files:**
- No new files

- [ ] **Step 1: Run full test suite**

Run: `go test ./... -v`
Expected: All tests pass

- [ ] **Step 2: Build binary**

Run: `go build ./cmd/blogwatcher`
Expected: Binary builds successfully

- [ ] **Step 3: Test with real RSS feed (manual)**

```bash
# Add a blog
./blogwatcher add test https://example.com/feed

# Scan for articles
./blogwatcher scan

# Generate summary (requires OPENAI_API_KEY)
OPENAI_API_KEY=sk-xxx ./blogwatcher summary 1

# Batch generate
OPENAI_API_KEY=sk-xxx ./blogwatcher summary --all
```

---

## Summary

| Task | Description | Files Changed |
|------|-------------|---------------|
| 1 | Update Article model | model.go |
| 2 | Database migration | database.go, database_test.go |
| 3 | Update CRUD operations | database.go, database_test.go |
| 4 | RSS parser changes | rss.go, rss_test.go |
| 5 | Scanner changes | scanner.go, scanner_test.go |
| 6 | HTML cleaner | cleaner.go, cleaner_test.go |
| 7 | LLM error types | errors.go |
| 8 | LLM client | client.go, client_test.go |
| 9 | Controller summary logic | summary.go, summary_test.go |
| 10 | CLI summary command | commands.go, root.go |
| 11 | Integration test | - |