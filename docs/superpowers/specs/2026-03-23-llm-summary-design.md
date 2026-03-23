# LLM Summary Feature Design

## Overview

Add LLM-powered summarization capability to BlogWatcher. Each article can have an AI-generated summary, helping users quickly understand content without reading the full article.

## Requirements Summary

- Support OpenAI-format API (compatible with Azure OpenAI, vLLM, etc.)
- Store RSS raw content fields (Content, Description, FeedSummary)
- Generate summaries on-demand via CLI command
- Support single article and batch summarization

## Data Model Changes

### Article Model

Add four new fields to `internal/model/model.go`:

| Field | Type | Description |
|-------|------|-------------|
| `Content` | `string` | RSS content/encoded |
| `Description` | `string` | RSS description |
| `FeedSummary` | `string` | RSS/Atom summary |
| `Summary` | `string` | LLM-generated summary |

### Database Schema

Add new columns to `articles` table:

```sql
ALTER TABLE articles ADD COLUMN content TEXT;
ALTER TABLE articles ADD COLUMN description TEXT;
ALTER TABLE articles ADD COLUMN feed_summary TEXT;
ALTER TABLE articles ADD COLUMN summary TEXT;
```

### Database Migration Strategy

SQLite does not support `IF NOT EXISTS` for `ALTER TABLE ADD COLUMN`. Implement idempotent migration in `internal/storage/database.go`:

```go
func (db *Database) migrateSchema() error {
    // Check existing columns using PRAGMA table_info
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

    // Add missing columns
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

Call `migrateSchema()` in `OpenDatabase()` after `init()`.

## Architecture

### Components

```
internal/
├── llm/
│   ├── client.go        # OpenAI API client
│   ├── errors.go        # LLM-specific error types
│   └── client_test.go
├── html/
│   └── cleaner.go       # HTML to plain text extraction
├── rss/
│   └── rss.go           # Modified: extract content/description/summary
├── scraper/
│   └── scraper.go       # No changes needed
├── scanner/
│   └── scanner.go       # Modified: pass content fields to article
├── storage/
│   └── database.go      # Modified: new fields, schema migration
├── model/
│   └── model.go         # Modified: new fields
├── controller/
│   └── controller.go    # Modified: summary generation logic
└── cli/
    └── commands.go      # Modified: summary command
```

### Data Flow

```
Scan Flow:
RSS Feed -> Parse -> Extract (title, url, content, description, feed_summary)
                        -> Store to database (raw HTML preserved)

Summary Generation Flow:
1. Get article from DB
2. If Summary exists and not force -> return existing
3. Get content (Content > FeedSummary > Description)
4. If no content -> try on-demand fetch (future: fallback to web scraping)
5. Clean HTML -> plain text
6. Call LLM API -> generate summary
7. Validate response (non-empty, reasonable length)
8. Store summary to DB
```

## Component Details

### 1. LLM Client (`internal/llm/client.go`)

```go
type Config struct {
    APIKey  string
    BaseURL string  // Optional, for non-OpenAI endpoints
    Model   string  // Default: gpt-4o-mini
    Timeout time.Duration  // Default: 60s
}

type Client struct {
    config Config
    http   *http.Client
}

func NewClient(config Config) *Client
func (c *Client) Summarize(ctx context.Context, title, content string) (string, error)
```

**Configuration:**
- `OPENAI_API_KEY` - Required
- `OPENAI_BASE_URL` - Optional, default: https://api.openai.com/v1
- `OPENAI_MODEL` - Optional, default: gpt-4o-mini
- Timeout: 60 seconds (configurable via code)

**Request limits:**
- Truncate input content to ~32k tokens (~128k chars) to avoid excessive costs
- Request max_tokens: 500

### 2. LLM Error Types (`internal/llm/errors.go`)

```go
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

### 3. HTML Cleaner (`internal/html/cleaner.go`)

```go
func ToPlainText(html string) string
```

**Implementation:** Use existing `goquery` dependency (already in project).

```go
func ToPlainText(html string) string {
    doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
    if err != nil {
        return html // fallback: return as-is
    }
    return doc.Text()
}
```

**Behavior:**
- Strip HTML tags
- Convert block elements to newlines (handled by goquery.Text())
- Return empty string for empty input

### 4. RSS Parser Changes (`internal/rss/rss.go`)

Update `FeedArticle` struct:

```go
type FeedArticle struct {
    Title         string
    URL           string
    PublishedDate *time.Time
    Content       string  // New
    Description   string  // New
    FeedSummary   string  // New (renamed from Summary to avoid confusion)
}
```

**gofeed field mappings:**

| FeedArticle Field | gofeed.Item Field | Notes |
|-------------------|-------------------|-------|
| `Content` | `item.Content` | Full HTML content |
| `Description` | `item.Description` | Short description |
| `FeedSummary` | `item.Summary` | Atom summary |

Update `ParseFeed`:

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

### 5. Scanner Changes (`internal/scanner/scanner.go`)

Update `convertFeedArticles`:

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
            Content:       article.Content,       // New
            Description:   article.Description,   // New
            FeedSummary:   article.FeedSummary,   // New
        })
    }
    return result
}
```

### 6. Storage Changes (`internal/storage/database.go`)

**Update all SELECT queries for articles:**

| Function | Current Columns | Add Columns |
|----------|----------------|-------------|
| `GetArticle` | id, blog_id, title, url, published_date, discovered_date, is_read | content, description, feed_summary, summary |
| `GetArticleByURL` | same | same |
| `ListArticles` | same | same |

**Update INSERT statements:**

`AddArticle`:
```sql
INSERT INTO articles (blog_id, title, url, published_date, discovered_date, is_read, content, description, feed_summary, summary)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
```

`AddArticlesBulk`: Same changes.

**Add new method:**

```go
func (db *Database) UpdateArticleSummary(id int64, summary string) error {
    _, err := db.conn.Exec(`UPDATE articles SET summary = ? WHERE id = ?`, summary, id)
    return err
}
```

**Update scanArticle:**

```go
func scanArticle(scanner interface{ Scan(dest ...any) error }) (*model.Article, error) {
    var (
        // ... existing fields ...
        content, description, feedSummary, summary sql.NullString
    )
    if err := scanner.Scan(&id, &blogID, &title, &url, &publishedDate, &discovered, &isRead, &content, &description, &feedSummary, &summary); err != nil {
        // ...
    }
    article := &model.Article{
        // ... existing fields ...
        Content:     content.String,
        Description: description.String,
        FeedSummary: feedSummary.String,
        Summary:     summary.String,
    }
    // ...
}
```

### 7. Controller (`internal/controller/controller.go`)

Add:

```go
// GetContent returns the best available content for summarization.
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

func GenerateSummary(db *storage.Database, llmClient *llm.Client, articleID int64, force bool) (*model.Article, error)
func GenerateAllSummaries(db *storage.Database, llmClient *llm.Client, force bool) (int, []error)
```

**Summary generation logic:**

1. Get article from DB
2. If article.Summary != "" and !force -> return article
3. Get content via `GetArticleContent(article)`
4. If content == "" -> return `NoContentError`
5. Clean HTML -> plain text
6. Truncate if needed (128k char limit)
7. Call `llmClient.Summarize(ctx, title, plainText)`
8. If response empty -> return error
9. Save summary to DB
10. Return updated article

### 8. CLI Commands (`internal/cli/commands.go`)

New `summary` command:

```
blogwatcher summary <article_id>          # Generate summary for single article
blogwatcher summary --all                 # Generate for all articles without summary
blogwatcher summary --all --force         # Regenerate all summaries
blogwatcher summary 1 --force             # Force regenerate for article 1
```

Flags:
- `--all` - Process all articles without summary
- `--force` - Regenerate even if summary exists

**Output format:**

```
$ blogwatcher summary 1
Summary for article 1:
[LLM-generated summary text]

$ blogwatcher summary --all
Processing 15 articles...
✓ Article 1: Summary generated
✓ Article 3: Summary generated
⚠ Article 5: No content available
✗ Article 7: API error: rate limit exceeded

Complete: 2 generated, 1 skipped, 1 failed
```

## Error Handling

| Error | Behavior |
|-------|----------|
| Missing API key | Print error with setup instructions, exit 1 |
| API call failure | Log error, continue with next article (batch mode) |
| No content available | Skip article, report in output |
| Empty LLM response | Return error, don't save empty summary |
| Network timeout | Return error after timeout |

## Backward Compatibility

### Database Migration

- Existing database automatically migrates on first run
- Migration is idempotent (safe to run multiple times)
- No data loss for existing articles

### Existing Articles Without Content

Existing articles have NULL content fields. Options to get content:

1. **Future: `blogwatcher refresh <blog_name>`** - Re-fetch articles with content
   - Would need to modify scanner to update existing articles
   - Not in initial scope

2. **Future: On-demand web scraping** - When generating summary, if no content, scrape article URL
   - Requires content extraction logic
   - Not in initial scope

For MVP: Articles without RSS content cannot be summarized. User must re-add the blog to get content.

## Testing Strategy

1. **Unit tests**
   - HTML cleaner: various HTML formats, edge cases
   - LLM client: mock HTTP responses, error cases
   - RSS parser: sample feeds with different content fields
   - Database migration: verify idempotency

2. **Integration tests**
   - End-to-end summary generation with mock LLM
   - Database migration from existing schema

## Future Enhancements (Out of Scope)

- Auto-summarize during scan (configurable)
- Multiple LLM providers (Anthropic, local models via Ollama)
- Summary length configuration via CLI flag
- Rate limiting and retry logic
- `refresh` command to update content for existing articles
- On-demand web scraping for articles without RSS content