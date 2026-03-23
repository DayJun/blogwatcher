# CLI Commands Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restructure CLI commands into resource-based subcommands with consistent patterns and LLM-friendly help text.

**Architecture:** Reorganize `blogs` and `articles` into parent commands with subcommands. Add `blogs edit`, `blogs show`, `articles show`. Add `--fields` flag. Improve `summary` output. Deprecate old top-level commands.

**Tech Stack:** Go 1.24, Cobra CLI framework

---

## File Structure

```
internal/
├── cli/
│   ├── root.go          # Modify: update command registration
│   ├── commands.go      # Modify: restructure into subcommands, add --fields
│   ├── errors.go        # Keep: existing error handling
│   └── output.go        # Create: shared output formatting helpers
├── controller/
│   ├── controller.go    # Modify: add UpdateBlog, GetBlogWithStats
│   └── summary.go       # Modify: improve output format
├── rss/
│   └── rss.go           # Modify: add GetFeedTitle function
└── storage/
    └── database.go      # Keep: existing functions sufficient
```

---

## Task 1: Add GetFeedTitle to RSS package

**Files:**
- Modify: `internal/rss/rss.go`
- Modify: `internal/rss/rss_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestGetFeedTitle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(`<?xml version="1.0"?>
<rss><channel><title>Test Feed Title</title></channel></rss>`))
	}))
	defer server.Close()

	title, err := GetFeedTitle(server.URL, 10*time.Second)
	require.NoError(t, err)
	assert.Equal(t, "Test Feed Title", title)
}

func TestGetFeedTitleNoTitle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(`<?xml version="1.0"?><rss><channel></channel></rss>`))
	}))
	defer server.Close()

	title, err := GetFeedTitle(server.URL, 10*time.Second)
	require.NoError(t, err)
	assert.Equal(t, "", title)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/rss/... -run TestGetFeedTitle`
Expected: FAIL with "undefined: GetFeedTitle"

- [ ] **Step 3: Implement GetFeedTitle**

```go
// GetFeedTitle fetches and extracts the title from an RSS/Atom feed.
// Returns empty string if the feed has no title or cannot be fetched.
func GetFeedTitle(feedURL string, timeout time.Duration) (string, error) {
	client := &http.Client{Timeout: timeout}
	response, err := client.Get(feedURL)
	if err != nil {
		return "", FeedParseError{Message: fmt.Sprintf("failed to fetch feed: %v", err)}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", FeedParseError{Message: fmt.Sprintf("failed to fetch feed: status %d", response.StatusCode)}
	}

	parser := gofeed.NewParser()
	feed, err := parser.Parse(response.Body)
	if err != nil {
		return "", FeedParseError{Message: fmt.Sprintf("failed to parse feed: %v", err)}
	}

	return strings.TrimSpace(feed.Title), nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/rss/... -run TestGetFeedTitle -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/rss/rss.go internal/rss/rss_test.go
git commit -m "feat(rss): add GetFeedTitle function for auto-naming blogs"
```

---

## Task 2: Add UpdateBlog and GetBlogStats to controller

**Files:**
- Modify: `internal/controller/controller.go`
- Modify: `internal/controller/controller_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestUpdateBlog(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	blog, err := controller.AddBlog(db, "Original", "https://example.com", "", "")
	require.NoError(t, err)

	updated, err := controller.UpdateBlog(db, blog.ID, "New Name", "https://newurl.com", "https://feed.com/rss", "article a")
	require.NoError(t, err)
	assert.Equal(t, "New Name", updated.Name)
	assert.Equal(t, "https://newurl.com", updated.URL)
	assert.Equal(t, "https://feed.com/rss", updated.FeedURL)
}

func TestGetBlogStats(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	blog, err := controller.AddBlog(db, "Test", "https://example.com", "", "")
	require.NoError(t, err)

	// Add some articles
	for i := 0; i < 5; i++ {
		_, err := db.AddArticle(model.Article{BlogID: blog.ID, Title: fmt.Sprintf("Article %d", i), URL: fmt.Sprintf("https://example.com/%d", i)})
		require.NoError(t, err)
	}
	db.MarkArticleRead(1)

	stats, err := controller.GetBlogStats(db, blog.ID)
	require.NoError(t, err)
	assert.Equal(t, 5, stats.TotalArticles)
	assert.Equal(t, 4, stats.UnreadArticles)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controller/... -run "TestUpdateBlog|TestGetBlogStats" -v`
Expected: FAIL

- [ ] **Step 3: Implement UpdateBlog and GetBlogStats**

Add to `internal/controller/controller.go`:

```go
type BlogStats struct {
	TotalArticles  int
	UnreadArticles int
}

func UpdateBlog(db *storage.Database, id int64, name string, url string, feedURL string, scrapeSelector string) (model.Blog, error) {
	blog, err := db.GetBlog(id)
	if err != nil {
		return model.Blog{}, err
	}
	if blog == nil {
		return model.Blog{}, BlogNotFoundError{Name: ""}
	}

	// Check for duplicate name if name changed
	if name != "" && name != blog.Name {
		if existing, err := db.GetBlogByName(name); err != nil {
			return model.Blog{}, err
		} else if existing != nil {
			return model.Blog{}, BlogAlreadyExistsError{Field: "name", Value: name}
		}
	}

	// Update fields (empty string means keep existing)
	if name != "" {
		blog.Name = name
	}
	if url != "" {
		blog.URL = url
	}
	blog.FeedURL = feedURL
	blog.ScrapeSelector = scrapeSelector

	if err := db.UpdateBlog(*blog); err != nil {
		return model.Blog{}, err
	}
	return *blog, nil
}

func GetBlogStats(db *storage.Database, blogID int64) (BlogStats, error) {
	total, err := db.CountArticles(nil, &blogID)
	if err != nil {
		return BlogStats{}, err
	}
	unread := true
	unreadCount, err := db.CountArticles(&unread, &blogID)
	if err != nil {
		return BlogStats{}, err
	}
	return BlogStats{TotalArticles: total, UnreadArticles: unreadCount}, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/controller/... -run "TestUpdateBlog|TestGetBlogStats" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/controller/controller.go internal/controller/controller_test.go
git commit -m "feat(controller): add UpdateBlog and GetBlogStats functions"
```

---

## Task 3: Create output formatting helpers

**Files:**
- Create: `internal/cli/output.go`
- Create: `internal/cli/output_test.go`

- [ ] **Step 1: Write the failing test**

```go
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
		PublishedDate: &published,
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/... -run "TestFormat" -v`
Expected: FAIL

- [ ] **Step 3: Implement output helpers**

```go
package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/Hyaxia/blogwatcher/internal/model"
)

// ArticleFieldValues maps field names to display values for an article.
func ArticleFieldValues(article *model.Article, blogNames map[int64]string) map[string]string {
	values := map[string]string{
		"id":           fmt.Sprintf("%d", article.ID),
		"title":        article.Title,
		"url":          article.URL,
		"blog":         blogNames[article.BlogID],
		"published":    formatTimePtr(article.PublishedDate),
		"discovered":   formatTimePtr(article.DiscoveredDate),
		"read":         formatBool(article.IsRead),
		"content":      article.Content,
		"description":  article.Description,
		"feed_summary": article.FeedSummary,
		"summary":      article.Summary,
	}
	return values
}

// FormatArticleFields formats article fields as pipe-separated single line.
func FormatArticleFields(article *model.Article, blogNames map[int64]string, fields []string) string {
	values := ArticleFieldValues(article, blogNames)
	parts := make([]string, len(fields))
	for i, field := range fields {
		parts[i] = values[field]
	}
	return strings.Join(parts, " | ")
}

// FormatArticleDetail formats article as key-value pairs (multi-line).
func FormatArticleDetail(article *model.Article, blogNames map[int64]string, fields []string) string {
	values := ArticleFieldValues(article, blogNames)
	var lines []string
	for _, field := range fields {
		value := values[field]
		if field == "content" || field == "description" || field == "feed_summary" || field == "summary" {
			lines = append(lines, fmt.Sprintf("%s:\n  %s", strings.Title(field), value))
		} else {
			lines = append(lines, fmt.Sprintf("%s: %s", strings.Title(field), value))
		}
	}
	return strings.Join(lines, "\n")
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02")
}

func formatBool(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/... -run "TestFormat" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/output.go internal/cli/output_test.go
git commit -m "feat(cli): add output formatting helpers for articles"
```

---

## Task 4: Add configuration check helper

**Files:**
- Modify: `internal/cli/errors.go`
- Modify: `internal/cli/errors_test.go`

This task implements the pre-requisite state check from the spec: all commands except `init` should verify configuration exists.

- [ ] **Step 1: Write the failing test**

```go
func TestRequireConfig(t *testing.T) {
	// Create a temp directory with no config
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	err := RequireConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Not configured")
}

func TestRequireConfigExists(t *testing.T) {
	// Create a temp directory with a valid config
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	cfgDir := filepath.Join(tmpDir, ".blogwatcher")
	os.MkdirAll(cfgDir, 0o755)
	cfg := &config.Config{
		LLM: config.LLMConfig{
			APIKey:  "test-key",
			BaseURL: "https://api.openai.com/v1",
			Model:   "gpt-4o-mini",
		},
	}
	cfg.Save(filepath.Join(cfgDir, "config.yaml"))

	err := RequireConfig()
	assert.NoError(t, err)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/... -run TestRequireConfig -v`
Expected: FAIL

- [ ] **Step 3: Implement RequireConfig**

Add to `internal/cli/errors.go`:

```go
import (
	"os"
	"path/filepath"

	"github.com/Hyaxia/blogwatcher/internal/config"
)

// NotConfiguredError is returned when the app is not initialized.
type NotConfiguredError struct{}

func (e NotConfiguredError) Error() string {
	return "Not configured. Run 'blogwatcher init' first."
}

// RequireConfig checks if configuration exists and is valid.
// Returns NotConfiguredError if config is missing or empty.
func RequireConfig() error {
	cfgPath, err := config.DefaultConfigPath()
	if err != nil {
		return err
	}

	// Check if config file exists
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		return NotConfiguredError{}
	}

	// Check if config has API key
	cfg, err := config.Load("")
	if err != nil {
		return err
	}

	if !cfg.IsConfigured() {
		return NotConfiguredError{}
	}

	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/... -run TestRequireConfig -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/errors.go internal/cli/errors_test.go
git commit -m "feat(cli): add RequireConfig helper for configuration validation"
```

---

## Task 5: Create blogs subcommands

**Files:**
- Modify: `internal/cli/commands.go`
- Modify: `internal/cli/commands_test.go`

- [ ] **Step 1: Write tests for blogs command structure**

Add to `internal/cli/commands_test.go`:

```go
func TestBlogsList(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	controller.AddBlog(db, "Blog A", "https://a.com", "", "")
	controller.AddBlog(db, "Blog B", "https://b.com", "", "")

	cmd := newBlogsCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Blog A")
	assert.Contains(t, buf.String(), "Blog B")
}

func TestBlogsShow(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	controller.AddBlog(db, "Test Blog", "https://test.com", "https://test.com/feed", "")

	cmd := newBlogsCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"Test Blog"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Blog: Test Blog")
	assert.Contains(t, buf.String(), "https://test.com")
	assert.Contains(t, buf.String(), "Articles: 0 total")
}

func TestBlogsAddAutoName(t *testing.T) {
	// Test that blogs add <url> auto-extracts name from feed
	// Requires mocking the RSS feed server
}

func TestBlogsAddCustomName(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cmd := newBlogsCommand()
	cmd.SetArgs([]string{"add", "My Custom Name", "https://example.com"})
	err := cmd.Execute()
	require.NoError(t, err)

	blog, _ := db.GetBlogByName("My Custom Name")
	assert.NotNil(t, blog)
}
```

- [ ] **Step 2: Implement newBlogsCommand with subcommands**

Replace `newBlogsCommand`, `newAddCommand`, `newRemoveCommand` with:

```go
func newBlogsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "blogs [name]",
		Short: "Manage tracked blogs.",
		Long: `Manage tracked blogs.

Usage:
  blogwatcher blogs              List all blogs
  blogwatcher blogs <name>       Show blog details
  blogwatcher blogs add ...      Add a new blog
  blogwatcher blogs edit <name>  Edit a blog
  blogwatcher blogs remove <name> Remove a blog`,
		RunE: runBlogsList,
	}

	cmd.AddCommand(newBlogsAddCommand())
	cmd.AddCommand(newBlogsEditCommand())
	cmd.AddCommand(newBlogsRemoveCommand())

	return cmd
}

func runBlogsList(cmd *cobra.Command, args []string) error {
	// Check configuration
	if err := RequireConfig(); err != nil {
		printError(err)
		return markError(err)
	}
	// If positional arg provided, show blog details
	if len(args) > 0 {
		return runBlogsShow(cmd, args[0])
	}
	db, err := storage.OpenDatabase("")
	if err != nil {
		return err
	}
	defer db.Close()

	blogs, err := db.ListBlogs()
	if err != nil {
		return err
	}
	if len(blogs) == 0 {
		fmt.Println("No blogs tracked yet. Use 'blogwatcher blogs add' to add one.")
		return nil
	}
	color.New(color.FgCyan, color.Bold).Printf("Tracked blogs (%d):\n\n", len(blogs))
	for _, blog := range blogs {
		color.New(color.FgWhite, color.Bold).Printf("  %s\n", blog.Name)
		fmt.Printf("    URL: %s\n", blog.URL)
		if blog.FeedURL != "" {
			fmt.Printf("    Feed: %s\n", blog.FeedURL)
		} else {
			fmt.Println("    Feed: (auto-discovered)")
		}
		if blog.LastScanned != nil {
			fmt.Printf("    Last scanned: %s\n", blog.LastScanned.Format("2006-01-02 15:04"))
		} else {
			fmt.Println("    Last scanned: never")
		}
		fmt.Println()
	}
	return nil
}

func runBlogsShow(cmd *cobra.Command, name string) error {
	db, err := storage.OpenDatabase("")
	if err != nil {
		return err
	}
	defer db.Close()

	blog, err := db.GetBlogByName(name)
	if err != nil {
		return err
	}
	if blog == nil {
		err := fmt.Errorf("Blog '%s' not found", name)
		printError(err)
		return markError(err)
	}

	stats, err := controller.GetBlogStats(db, blog.ID)
	if err != nil {
		return err
	}

	fmt.Printf("Blog: %s\n", blog.Name)
	fmt.Printf("URL: %s\n", blog.URL)
	if blog.FeedURL != "" {
		fmt.Printf("Feed: %s\n", blog.FeedURL)
	} else {
		fmt.Println("Feed: (auto-discovered)")
	}
	if blog.ScrapeSelector != "" {
		fmt.Printf("Selector: %s\n", blog.ScrapeSelector)
	} else {
		fmt.Println("Selector: (none)")
	}
	if blog.LastScanned != nil {
		fmt.Printf("Last scanned: %s\n", blog.LastScanned.Format("2006-01-02 15:04"))
	} else {
		fmt.Println("Last scanned: never")
	}
	fmt.Printf("Articles: %d total, %d unread\n", stats.TotalArticles, stats.UnreadArticles)
	return nil
}

func newBlogsAddCommand() *cobra.Command {
	var feedURL string
	var scrapeSelector string

	cmd := &cobra.Command{
		Use:   "add <url> OR add <name> <url>",
		Short: "Add a new blog to track.",
		Long: `Add a new blog to track.

If only URL is provided, the blog name is automatically extracted from the feed.
If both name and URL are provided, the given name is used.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			var name, url string
			if len(args) == 1 {
				url = args[0]
				// Auto-extract name from feed
				if feedURL == "" {
					discovered, err := rss.DiscoverFeedURL(url, 30*time.Second)
					if err != nil || discovered == "" {
						err := fmt.Errorf("Could not discover feed. Please provide --feed-url")
						printError(err)
						return markError(err)
					}
					feedURL = discovered
				}
				title, err := rss.GetFeedTitle(feedURL, 30*time.Second)
				if err != nil {
					err := fmt.Errorf("Could not extract name from feed: %v. Please provide a name.", err)
					printError(err)
					return markError(err)
				}
				if title == "" {
					err := fmt.Errorf("Feed has no title. Please provide a name.")
					printError(err)
					return markError(err)
				}
				name = title
			} else {
				name = args[0]
				url = args[1]
			}

			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()

			_, err = controller.AddBlog(db, name, url, feedURL, scrapeSelector)
			if err != nil {
				printError(err)
				return markError(err)
			}
			color.New(color.FgGreen).Printf("Added blog '%s'\n", name)
			return nil
		},
	}
	cmd.Flags().StringVar(&feedURL, "feed-url", "", "RSS/Atom feed URL (auto-discovered if not provided)")
	cmd.Flags().StringVar(&scrapeSelector, "scrape-selector", "", "CSS selector for HTML scraping fallback")
	return cmd
}

func newBlogsEditCommand() *cobra.Command {
	var newName string
	var feedURL string
	var scrapeSelector string

	cmd := &cobra.Command{
		Use:   "edit <name>",
		Short: "Edit a tracked blog.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()

			blog, err := db.GetBlogByName(name)
			if err != nil {
				return err
			}
			if blog == nil {
				err := fmt.Errorf("Blog '%s' not found", name)
				printError(err)
				return markError(err)
			}

			_, err = controller.UpdateBlog(db, blog.ID, newName, "", feedURL, scrapeSelector)
			if err != nil {
				printError(err)
				return markError(err)
			}
			color.New(color.FgGreen).Printf("Blog '%s' updated.\n", name)
			return nil
		},
	}
	cmd.Flags().StringVar(&newName, "name", "", "New blog name")
	cmd.Flags().StringVar(&feedURL, "feed-url", "", "New feed URL")
	cmd.Flags().StringVar(&scrapeSelector, "scrape-selector", "", "New scrape selector")
	return cmd
}

func newBlogsRemoveCommand() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a blog from tracking.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if !yes {
				confirmed, err := confirm(fmt.Sprintf("Remove blog '%s' and all its articles?", name))
				if err != nil {
					return err
				}
				if !confirmed {
					return nil
				}
			}
			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()
			if err := controller.RemoveBlog(db, name); err != nil {
				printError(err)
				return markError(err)
			}
			color.New(color.FgGreen).Printf("Removed blog '%s'\n", name)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt")
	return cmd
}
```

- [ ] **Step 3: Run tests to verify**

Run: `go test ./internal/cli/... -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/cli/commands.go internal/cli/commands_test.go
git commit -m "feat(cli): restructure blogs command with subcommands"
```

---

## Task 6: Create articles subcommands with --fields

**Files:**
- Modify: `internal/cli/commands.go`
- Modify: `internal/cli/commands_test.go`

- [ ] **Step 1: Write tests for articles command structure**

Add to `internal/cli/commands_test.go`:

```go
func TestArticlesList(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	blog, _ := db.AddBlog(model.Blog{Name: "Test", URL: "https://test.com"})
	db.AddArticle(model.Article{BlogID: blog.ID, Title: "Article 1", URL: "https://test.com/1"})
	db.AddArticle(model.Article{BlogID: blog.ID, Title: "Article 2", URL: "https://test.com/2"})

	cmd := newArticlesCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Article 1")
	assert.Contains(t, buf.String(), "Article 2")
}

func TestArticlesListCustomFields(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	blog, _ := db.AddBlog(model.Blog{Name: "Test", URL: "https://test.com"})
	db.AddArticle(model.Article{BlogID: blog.ID, Title: "Test Article", URL: "https://test.com/1", Summary: "Test summary"})

	cmd := newArticlesCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--fields", "id,title,summary"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Test Article")
	assert.Contains(t, buf.String(), "Test summary")
}

func TestArticlesShow(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	blog, _ := db.AddBlog(model.Blog{Name: "Test", URL: "https://test.com"})
	article, _ := db.AddArticle(model.Article{BlogID: blog.ID, Title: "Test Article", URL: "https://test.com/1", Content: "Full content"})

	cmd := newArticlesCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{fmt.Sprintf("%d", article.ID)})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Title: Test Article")
	assert.Contains(t, buf.String(), "Content:")
}

func TestArticlesRead(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	blog, _ := db.AddBlog(model.Blog{Name: "Test", URL: "https://test.com"})
	article, _ := db.AddArticle(model.Article{BlogID: blog.ID, Title: "Test", URL: "https://test.com/1"})

	cmd := newArticlesCommand()
	cmd.SetArgs([]string{"read", fmt.Sprintf("%d", article.ID)})
	err := cmd.Execute()
	require.NoError(t, err)

	updated, _ := db.GetArticle(article.ID)
	assert.True(t, updated.IsRead)
}
```

- [ ] **Step 2: Implement newArticlesCommand with subcommands**

```go
var defaultListFields = []string{"id", "title", "blog", "read", "url", "published"}
var defaultDetailFields = []string{"id", "title", "url", "blog", "published", "discovered", "read", "content"}
var allArticleFields = []string{"id", "title", "url", "blog", "published", "discovered", "read", "content", "description", "feed_summary", "summary"}

// Package-level variables for flags (used by articles command)
var (
	showAll   bool
	showRead  bool
	blogName  string
	fieldsFlag string
	page      int
	perPage   int
)

func newArticlesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "articles [id]",
		Short: "List and manage articles.",
		Long: `List and manage articles.

Usage:
  blogwatcher articles [flags]     List articles
  blogwatcher articles <id>        Show article details
  blogwatcher articles read <id>   Mark as read
  blogwatcher articles unread <id> Mark as unread
  blogwatcher articles read-all    Mark all as read`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				// Check if it's a subcommand or article ID
				if args[0] == "read" || args[0] == "unread" || args[0] == "read-all" {
					return fmt.Errorf("unknown command")
				}
				return runArticlesShow(cmd, args)
			}
			return runArticlesList(cmd, args)
		},
	}

	// Filtering flags
	cmd.Flags().BoolVarP(&showAll, "all", "a", false, "Show all articles (including read)")
	cmd.Flags().BoolVarP(&showRead, "read", "r", false, "Show only read articles")
	cmd.Flags().StringVarP(&blogName, "blog", "b", "", "Filter by blog name")

	// Output flags
	cmd.Flags().StringVarP(&fieldsFlag, "fields", "f", "", "Fields to display (comma-separated)")

	// Pagination flags
	cmd.Flags().IntVarP(&page, "page", "p", 1, "Page number")
	cmd.Flags().IntVarP(&perPage, "per-page", "P", 20, "Articles per page (max 100)")

	cmd.AddCommand(newArticlesReadCommand())
	cmd.AddCommand(newArticlesUnreadCommand())
	cmd.AddCommand(newArticlesReadAllCommand())

	return cmd
}

func runArticlesList(cmd *cobra.Command, args []string) error {
	db, err := storage.OpenDatabase("")
	if err != nil {
		return err
	}
	defer db.Close()

	// Determine status filter
	status := "unread"
	if showAll {
		status = "all"
	} else if showRead {
		status = "read"
	}

	// Validate and normalize pagination
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}

	result, err := controller.GetArticles(db, status, blogName, page, perPage)
	if err != nil {
		printError(err)
		return markError(err)
	}

	if result.Total == 0 {
		label := "Unread articles"
		if status == "read" {
			label = "Read articles"
		} else if status == "all" {
			label = "Articles"
		}
		color.New(color.FgCyan, color.Bold).Printf("%s (no results):\n\n", label)
		return nil
	}

	// Determine fields
	fields := defaultListFields
	if fieldsFlag != "" {
		fields = strings.Split(fieldsFlag, ",")
	}

	label := "Unread articles"
	if status == "read" {
		label = "Read articles"
	} else if status == "all" {
		label = "All articles"
	}
	color.New(color.FgCyan, color.Bold).Printf("%s (page %d/%d, %d total):\n\n", label, result.Page, result.TotalPages, result.Total)

	for _, article := range result.Articles {
		if len(fields) == len(defaultListFields) && fieldsFlag == "" {
			// Default multi-line format
			printArticle(article, result.BlogNames[article.BlogID])
		} else {
			// Custom fields, single-line format
			fmt.Printf("  %s\n", FormatArticleFields(&article, result.BlogNames, fields))
		}
	}
	return nil
}

func runArticlesShow(cmd *cobra.Command, args []string) error {
	articleID, err := parseID(args[0])
	if err != nil {
		return err
	}

	db, err := storage.OpenDatabase("")
	if err != nil {
		return err
	}
	defer db.Close()

	article, err := db.GetArticle(articleID)
	if err != nil {
		return err
	}
	if article == nil {
		err := fmt.Errorf("Article %d not found", articleID)
		printError(err)
		return markError(err)
	}

	blogs, err := db.ListBlogs()
	if err != nil {
		return err
	}
	blogNames := make(map[int64]string)
	for _, b := range blogs {
		blogNames[b.ID] = b.Name
	}

	fields := defaultDetailFields
	if fieldsFlag != "" {
		fields = strings.Split(fieldsFlag, ",")
	}

	fmt.Println(FormatArticleDetail(article, blogNames, fields))
	return nil
}

func newArticlesReadCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "read <id>",
		Short: "Mark an article as read.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			articleID, err := parseID(args[0])
			if err != nil {
				return err
			}
			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()
			article, err := controller.MarkArticleRead(db, articleID)
			if err != nil {
				printError(err)
				return markError(err)
			}
			if article.IsRead {
				fmt.Printf("Article %d is already marked as read.\n", articleID)
			} else {
				color.New(color.FgGreen).Printf("Marked article %d as read\n", articleID)
			}
			return nil
		},
	}
	return cmd
}

func newArticlesUnreadCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unread <id>",
		Short: "Mark an article as unread.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			articleID, err := parseID(args[0])
			if err != nil {
				return err
			}
			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()
			article, err := controller.MarkArticleUnread(db, articleID)
			if err != nil {
				printError(err)
				return markError(err)
			}
			if !article.IsRead {
				fmt.Printf("Article %d is already marked as unread.\n", articleID)
			} else {
				color.New(color.FgGreen).Printf("Marked article %d as unread\n", articleID)
			}
			return nil
		},
	}
	return cmd
}

func newArticlesReadAllCommand() *cobra.Command {
	var blogNameFilter string
	var yes bool

	cmd := &cobra.Command{
		Use:   "read-all",
		Short: "Mark all unread articles as read.",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()

			result, err := controller.GetArticles(db, "unread", blogNameFilter, 1, 1000)
			if err != nil {
				printError(err)
				return markError(err)
			}
			if len(result.Articles) == 0 {
				color.New(color.FgGreen).Println("No unread articles to mark as read.")
				return nil
			}

			if !yes {
				scope := "all blogs"
				if blogNameFilter != "" {
					scope = fmt.Sprintf("from '%s'", blogNameFilter)
				}
				confirmed, err := confirm(fmt.Sprintf("Mark %d article(s) %s as read?", len(result.Articles), scope))
				if err != nil {
					return err
				}
				if !confirmed {
					return nil
				}
			}

			marked, err := controller.MarkAllArticlesRead(db, blogNameFilter)
			if err != nil {
				printError(err)
				return markError(err)
			}

			color.New(color.FgGreen).Printf("Marked %d article(s) as read\n", len(marked))
			return nil
		},
	}

	cmd.Flags().StringVarP(&blogNameFilter, "blog", "b", "", "Only mark articles from this blog")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt")
	return cmd
}
```

- [ ] **Step 3: Run tests to verify**

Run: `go test ./internal/cli/... -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/cli/commands.go internal/cli/commands_test.go
git commit -m "feat(cli): restructure articles command with --fields support"
```

---

## Task 7: Improve summary output

**Files:**
- Modify: `internal/cli/commands.go`

- [ ] **Step 1: Update summary single article output**

Modify `runSummarySingle` to show title + summary format:

```go
func runSummarySingle(ctx context.Context, db *storage.Database, client *llm.Client, articleID int64, force bool) error {
	article, err := controller.GenerateSummary(ctx, db, client, articleID, force)
	if err != nil {
		printError(err)
		return markError(err)
	}

	fmt.Printf("Title: %s\n\n", article.Title)
	fmt.Printf("Summary: %s\n", article.Summary)
	return nil
}
```

- [ ] **Step 2: Update summary --all output**

Modify `runSummaryAll` to only show failures:

```go
func runSummaryAll(ctx context.Context, db *storage.Database, client *llm.Client, force bool, days int) error {
	articles, err := db.ListArticles(nil, nil, days, 1, storage.NoPagination)
	if err != nil {
		return err
	}

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
			generated++
		}
	}

	fmt.Println()
	color.New(color.FgCyan, color.Bold).Printf("Complete: %d generated, %d skipped, %d failed\n", generated, skipped, failed)
	return nil
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/cli/commands.go
git commit -m "feat(cli): improve summary output format"
```

---

## Task 8: Add deprecated commands for backward compatibility

**Files:**
- Modify: `internal/cli/root.go`
- Modify: `internal/cli/commands.go`

- [ ] **Step 1: Add deprecated command wrappers**

Add to `internal/cli/commands.go`:

```go
func newDeprecatedAddCommand() *cobra.Command {
	var feedURL string
	var scrapeSelector string

	cmd := &cobra.Command{
		Use:    "add <name> <url>",
		Short:  "Add a new blog to track. (deprecated)",
		Hidden: true,
		Args:   cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(os.Stderr, "Warning: 'add' is deprecated, use 'blogs add'")
			name := args[0]
			url := args[1]
			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()
			_, err = controller.AddBlog(db, name, url, feedURL, scrapeSelector)
			if err != nil {
				printError(err)
				return markError(err)
			}
			color.New(color.FgGreen).Printf("Added blog '%s'\n", name)
			return nil
		},
	}
	cmd.Flags().StringVar(&feedURL, "feed-url", "", "RSS/Atom feed URL")
	cmd.Flags().StringVar(&scrapeSelector, "scrape-selector", "", "CSS selector for HTML scraping")
	return cmd
}

func newDeprecatedRemoveCommand() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:    "remove <name>",
		Short:  "Remove a blog from tracking. (deprecated)",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(os.Stderr, "Warning: 'remove' is deprecated, use 'blogs remove'")
			name := args[0]
			if !yes {
				confirmed, err := confirm(fmt.Sprintf("Remove blog '%s' and all its articles?", name))
				if err != nil {
					return err
				}
				if !confirmed {
					return nil
				}
			}
			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()
			if err := controller.RemoveBlog(db, name); err != nil {
				printError(err)
				return markError(err)
			}
			color.New(color.FgGreen).Printf("Removed blog '%s'\n", name)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt")
	return cmd
}

func newDeprecatedReadCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "read <article_id>",
		Short:  "Mark an article as read. (deprecated)",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(os.Stderr, "Warning: 'read' is deprecated, use 'articles read'")
			articleID, err := parseID(args[0])
			if err != nil {
				return err
			}
			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()
			article, err := controller.MarkArticleRead(db, articleID)
			if err != nil {
				printError(err)
				return markError(err)
			}
			if article.IsRead {
				fmt.Printf("Article %d is already marked as read.\n", articleID)
			} else {
				color.New(color.FgGreen).Printf("Marked article %d as read\n", articleID)
			}
			return nil
		},
	}
	return cmd
}

func newDeprecatedUnreadCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "unread <article_id>",
		Short:  "Mark an article as unread. (deprecated)",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(os.Stderr, "Warning: 'unread' is deprecated, use 'articles unread'")
			articleID, err := parseID(args[0])
			if err != nil {
				return err
			}
			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()
			article, err := controller.MarkArticleUnread(db, articleID)
			if err != nil {
				printError(err)
				return markError(err)
			}
			if !article.IsRead {
				fmt.Printf("Article %d is already marked as unread.\n", articleID)
			} else {
				color.New(color.FgGreen).Printf("Marked article %d as unread\n", articleID)
			}
			return nil
		},
	}
	return cmd
}

func newDeprecatedReadAllCommand() *cobra.Command {
	var blogName string
	var yes bool

	cmd := &cobra.Command{
		Use:    "read-all",
		Short:  "Mark all unread articles as read. (deprecated)",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(os.Stderr, "Warning: 'read-all' is deprecated, use 'articles read-all'")
			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()

			result, err := controller.GetArticles(db, "unread", blogName, 1, 1000)
			if err != nil {
				printError(err)
				return markError(err)
			}
			if len(result.Articles) == 0 {
				color.New(color.FgGreen).Println("No unread articles to mark as read.")
				return nil
			}

			if !yes {
				scope := "all blogs"
				if blogName != "" {
					scope = fmt.Sprintf("from '%s'", blogName)
				}
				confirmed, err := confirm(fmt.Sprintf("Mark %d article(s) %s as read?", len(result.Articles), scope))
				if err != nil {
					return err
				}
				if !confirmed {
					return nil
				}
			}

			marked, err := controller.MarkAllArticlesRead(db, blogName)
			if err != nil {
				printError(err)
				return markError(err)
			}

			color.New(color.FgGreen).Printf("Marked %d article(s) as read\n", len(marked))
			return nil
		},
	}

	cmd.Flags().StringVarP(&blogName, "blog", "b", "", "Only mark articles from this blog")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt")
	return cmd
}
```

- [ ] **Step 2: Register deprecated commands in root**

Modify `internal/cli/root.go`:

```go
func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "blogwatcher",
		Short:         "BlogWatcher - Track blog articles and detect new posts.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	rootCmd.Version = version.Version
	rootCmd.SetVersionTemplate("{{.Version}}\n")

	// New commands
	rootCmd.AddCommand(newInitCommand())
	rootCmd.AddCommand(newBlogsCommand())
	rootCmd.AddCommand(newArticlesCommand())
	rootCmd.AddCommand(newScanCommand())
	rootCmd.AddCommand(newImportCommand())
	rootCmd.AddCommand(newSummaryCommand())

	// Deprecated commands (hidden)
	rootCmd.AddCommand(newDeprecatedAddCommand())
	rootCmd.AddCommand(newDeprecatedRemoveCommand())
	rootCmd.AddCommand(newDeprecatedReadCommand())
	rootCmd.AddCommand(newDeprecatedUnreadCommand())
	rootCmd.AddCommand(newDeprecatedReadAllCommand())

	return rootCmd
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/cli/root.go internal/cli/commands.go
git commit -m "feat(cli): add deprecated commands for backward compatibility"
```

---

## Task 9: Update README documentation

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Update README with new command structure**

Update all command examples to use new syntax:
- `blogwatcher blogs add` instead of `blogwatcher add`
- `blogwatcher blogs remove` instead of `blogwatcher remove`
- `blogwatcher articles` instead of `blogwatcher articles`
- `blogwatcher articles read` instead of `blogwatcher read`
- etc.

Add documentation for:
- `--fields` flag
- `blogs edit` command
- `blogs <name>` detail view
- `articles <id>` detail view

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: update README with new CLI structure"
```

---

## Final Verification

- [ ] **Run all tests**

Run: `go test ./...`
Expected: All PASS

- [ ] **Build and smoke test**

Run: `go build ./cmd/blogwatcher && ./blogwatcher --help`
Verify all commands appear correctly

- [ ] **Push to remote**

```bash
git push origin main
```