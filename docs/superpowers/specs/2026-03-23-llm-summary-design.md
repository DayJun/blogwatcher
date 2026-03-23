# LLM Summary Feature Design

## Overview

Add L-powered summarization capability to BlogWatcher. Each article can have an AI-generated summary, helping users quickly understand content without reading the full article.

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

Update `articles` table:

```sql
ALTER TABLE articles ADD COLUMN content TEXT;
ALTER TABLE articles ADD COLUMN description TEXT;
ALTER TABLE articles ADD COLUMN feed_summary TEXT;
ALTER TABLE articles ADD COLUMN summary TEXT;
```

## Architecture

### Components

```
internal/
├── llm/
│   ├── client.go        # OpenAI API client
│   └── client_test.go
├── html/
│   └── cleaner.go       # HTML to plain text extraction
├── rss/
│   └── rss.go           # Modified: extract content/description/summary
├── storage/
│   └── database.go      # Modified: new fields, schema migration
├── model/
│   └── model.go         # Modified: new fields
└── cli/
    └── commands.go      # Modified: summary command
```

### Data Flow

```
Scan Flow:
RSS Feed -> Parse -> Extract (title, url, content, description, summary)
                        -> Store to database (raw HTML preserved)

Summary Generation Flow:
1. Get article from DB
2. If Summary exists and not force -> return existing
3. Get content (Content > FeedSummary > Description)
4. Clean HTML -> plain text
5. Call LLM API -> generate summary
6. Store summary to DB
```

## Component Details

### 1. LLM Client (`internal/llm/client.go`)

```go
type Config struct {
    APIKey  string
    BaseURL string  // Optional, for non-OpenAI endpoints
    Model   string  // Default: gpt-4o-mini
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

### 2. HTML Cleaner (`internal/html/cleaner.go`)

```go
func ToPlainText(html string) string
```

- Strip HTML tags
- Convert block elements to newlines
- Preserve readable text structure

### 3. RSS Parser Changes (`internal/rss/rss.go`)

Update `FeedArticle` struct:

```go
type FeedArticle struct {
    Title         string
    URL           string
    PublishedDate *time.Time
    Content       string  // New: item.Content
    Description   string  // New: item.Description
    Summary       string  // New: item.Summary (Atom)
}
```

Update `ParseFeed` to extract all content fields.

### 4. Storage Changes (`internal/storage/database.go`)

- Update schema migration to add new columns
- Update `AddArticle`, `AddArticlesBulk` to handle new fields
- Add `UpdateArticleSummary(id int64, summary string) error`
- Update `scanArticle` to read new fields

### 5. CLI Commands (`internal/cli/commands.go`)

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

### 6. Controller (`internal/controller/controller.go`)

Add:

```go
func GenerateSummary(db *storage.Database, llmClient *llm.Client, articleID int64, force bool) (*model.Article, error)
func GenerateAllSummaries(db *storage.Database, llmClient *llm.Client, force bool) (int, error)
```

## Error Handling

- Missing API key: Clear error message with setup instructions
- API call failure: Log error, continue with remaining articles (batch mode)
- No content available: Skip article, report in output
- Rate limiting: Basic retry with backoff (optional, future enhancement)

## Backward Compatibility

- Existing database automatically migrates on first run (ALTER TABLE ADD COLUMN is safe)
- Existing articles with null content fields can still be summarized by:
  1. Re-scanning the blog (gets content from RSS)
  2. On-demand fetch when generating summary (fallback)

## Testing Strategy

1. **Unit tests**
   - HTML cleaner: various HTML formats
   - LLM client: mock HTTP responses
   - RSS parser: sample feeds with different content fields

2. **Integration tests**
   - End-to-end summary generation
   - Database migration

## Future Enhancements (Out of Scope)

- Auto-summarize during scan (configurable)
- Multiple LLM providers (Anthropic, local models via Ollama)
- Summary length configuration
- Rate limiting and retry logic
- Caching to avoid duplicate API calls