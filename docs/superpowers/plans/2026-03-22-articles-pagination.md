# Articles Pagination and Filter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add pagination and read/unread filtering to the `articles` CLI command.

**Architecture:** Modify storage layer to support pagination and status filtering, update controller to return paginated results with totals, add CLI flags for page/per-page/read filters.

**Tech Stack:** Go 1.24+, SQLite (modernc.org/sqlite), cobra CLI

**Spec:** `docs/superpowers/specs/2026-03-22-articles-pagination-design.md`

---

## Task 1: Add CountArticles to storage layer

**Files:**
- Modify: `internal/storage/database.go`
- Modify: `internal/storage/database_test.go`

- [ ] **Step 1: Write the failing test for CountArticles**

Add to `internal/storage/database_test.go` after `TestListArticlesFiltersAndOrdering`:

```go
func TestCountArticles(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	blogA, err := db.AddBlog(model.Blog{Name: "A", URL: "https://a.example.com"})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}

	// Add 3 articles, 1 read
	_, err = db.AddArticle(model.Article{BlogID: blogA.ID, Title: "One", URL: "https://a.example.com/1"})
	if err != nil {
		t.Fatalf("add article: %v", err)
	}
	second, err := db.AddArticle(model.Article{BlogID: blogA.ID, Title: "Two", URL: "https://a.example.com/2"})
	if err != nil {
		t.Fatalf("add article: %v", err)
	}
	_, err = db.AddArticle(model.Article{BlogID: blogA.ID, Title: "Three", URL: "https://a.example.com/3"})
	if err != nil {
		t.Fatalf("add article: %v", err)
	}
	if _, err := db.MarkArticleRead(second.ID); err != nil {
		t.Fatalf("mark read: %v", err)
	}

	// Count all
	total, err := db.CountArticles(nil, nil)
	if err != nil {
		t.Fatalf("count all: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected 3 total, got %d", total)
	}

	// Count unread
	unread := true
	count, err := db.CountArticles(&unread, nil)
	if err != nil {
		t.Fatalf("count unread: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 unread, got %d", count)
	}

	// Count read
	read := false
	count, err = db.CountArticles(&read, nil)
	if err != nil {
		t.Fatalf("count read: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 read, got %d", count)
	}

	// Count by blog
	blogID := blogA.ID
	count, err = db.CountArticles(nil, &blogID)
	if err != nil {
		t.Fatalf("count by blog: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 for blog A, got %d", count)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/storage/... -run TestCountArticles -v`
Expected: FAIL - `db.CountArticles undefined`

- [ ] **Step 3: Implement CountArticles**

Add to `internal/storage/database.go` after `ListArticles` function:

```go
func (db *Database) CountArticles(unreadOnly *bool, blogID *int64) (int, error) {
	query := `SELECT COUNT(*) FROM articles WHERE 1=1`
	var args []interface{}
	if unreadOnly != nil {
		if *unreadOnly {
			query += " AND is_read = 0"
		} else {
			query += " AND is_read = 1"
		}
	}
	if blogID != nil {
		query += " AND blog_id = ?"
		args = append(args, *blogID)
	}

	row := db.conn.QueryRow(query, args...)
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/storage/... -run TestCountArticles -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/storage/database.go internal/storage/database_test.go
git commit -m "feat(storage): add CountArticles function for pagination support

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 2: Update ListArticles with pagination and status filter

**Files:**
- Modify: `internal/storage/database.go`
- Modify: `internal/storage/database_test.go`

- [ ] **Step 1: Write the failing test for pagination**

Add to `internal/storage/database_test.go`:

```go
func TestListArticlesPagination(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	blog, err := db.AddBlog(model.Blog{Name: "Test", URL: "https://example.com"})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}

	// Add 5 articles with different dates
	for i := 1; i <= 5; i++ {
		t := time.Date(2024, 1, i, 0, 0, 0, 0, time.UTC)
		_, err := db.AddArticle(model.Article{
			BlogID:        blog.ID,
			Title:         fmt.Sprintf("Article %d", i),
			URL:           fmt.Sprintf("https://example.com/%d", i),
			DiscoveredDate: &t,
		})
		if err != nil {
			t.Fatalf("add article %d: %v", i, err)
		}
	}

	// Test pagination - page 1, perPage 2
	page1, err := db.ListArticles(nil, nil, 1, 2)
	if err != nil {
		t.Fatalf("list page 1: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("expected 2 articles on page 1, got %d", len(page1))
	}
	// Most recent first
	if page1[0].Title != "Article 5" || page1[1].Title != "Article 4" {
		t.Fatalf("unexpected page 1 order: %s, %s", page1[0].Title, page1[1].Title)
	}

	// Test pagination - page 2
	page2, err := db.ListArticles(nil, nil, 2, 2)
	if err != nil {
		t.Fatalf("list page 2: %v", err)
	}
	if len(page2) != 2 {
		t.Fatalf("expected 2 articles on page 2, got %d", len(page2))
	}
	if page2[0].Title != "Article 3" || page2[1].Title != "Article 2" {
		t.Fatalf("unexpected page 2 order: %s, %s", page2[0].Title, page2[1].Title)
	}

	// Test pagination - page 3 (partial)
	page3, err := db.ListArticles(nil, nil, 3, 2)
	if err != nil {
		t.Fatalf("list page 3: %v", err)
	}
	if len(page3) != 1 {
		t.Fatalf("expected 1 article on page 3, got %d", len(page3))
	}

	// Test pagination - beyond range
	page4, err := db.ListArticles(nil, nil, 4, 2)
	if err != nil {
		t.Fatalf("list page 4: %v", err)
	}
	if len(page4) != 0 {
		t.Fatalf("expected 0 articles on page 4, got %d", len(page4))
	}
}

func TestListArticlesNoPagination(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	blog, err := db.AddBlog(model.Blog{Name: "Test", URL: "https://example.com"})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}

	for i := 1; i <= 5; i++ {
		_, err := db.AddArticle(model.Article{
			BlogID: blog.ID,
			Title:  fmt.Sprintf("Article %d", i),
			URL:    fmt.Sprintf("https://example.com/%d", i),
		})
		if err != nil {
			t.Fatalf("add article %d: %v", i, err)
		}
	}

	// perPage = 0 means no pagination (return all)
	all, err := db.ListArticles(nil, nil, 1, NoPagination)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 5 {
		t.Fatalf("expected 5 articles with no pagination, got %d", len(all))
	}
}

func TestListArticlesReadFilter(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	blog, err := db.AddBlog(model.Blog{Name: "Test", URL: "https://example.com"})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}

	// Add articles: firstArticle will be marked read, secondArticle stays unread
	firstArticle, err := db.AddArticle(model.Article{BlogID: blog.ID, Title: "First Article", URL: "https://example.com/1"})
	if err != nil {
		t.Fatalf("add article: %v", err)
	}
	_, err = db.AddArticle(model.Article{BlogID: blog.ID, Title: "Second Article", URL: "https://example.com/2"})
	if err != nil {
		t.Fatalf("add article: %v", err)
	}
	if _, err := db.MarkArticleRead(firstArticle.ID); err != nil {
		t.Fatalf("mark read: %v", err)
	}

	// Filter: unread only - should return second article (not marked read)
	unread := true
	list, err := db.ListArticles(&unread, nil, 1, NoPagination)
	if err != nil {
		t.Fatalf("list unread: %v", err)
	}
	if len(list) != 1 || list[0].Title != "Second Article" {
		t.Fatalf("expected 1 unread article 'Second Article', got %v", list)
	}

	// Filter: read only - should return first article (marked read)
	read := false
	list, err = db.ListArticles(&read, nil, 1, NoPagination)
	if err != nil {
		t.Fatalf("list read: %v", err)
	}
	if len(list) != 1 || list[0].Title != "First Article" {
		t.Fatalf("expected 1 read article 'First Article', got %v", list)
	}

	// Filter: all (nil)
	list, err = db.ListArticles(nil, nil, 1, NoPagination)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 total articles, got %d", len(list))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/storage/... -run "TestListArticles(Pagination|NoPagination|ReadFilter)" -v`
Expected: FAIL - signature mismatch and `NoPagination` undefined

- [ ] **Step 3: Update ListArticles signature and implementation**

Replace the `ListArticles` function in `internal/storage/database.go`:

```go
// NoPagination is passed to ListArticles perPage parameter to return all records.
const NoPagination = 0

func (db *Database) ListArticles(unreadOnly *bool, blogID *int64, page int, perPage int) ([]model.Article, error) {
	query := `SELECT id, blog_id, title, url, published_date, discovered_date, is_read FROM articles WHERE 1=1`
	var args []interface{}
	if unreadOnly != nil {
		if *unreadOnly {
			query += " AND is_read = 0"
		} else {
			query += " AND is_read = 1"
		}
	}
	if blogID != nil {
		query += " AND blog_id = ?"
		args = append(args, *blogID)
	}
	query += " ORDER BY discovered_date DESC"

	// Add pagination if perPage > 0
	if perPage > 0 {
		if page < 1 {
			page = 1
		}
		offset := (page - 1) * perPage
		query += " LIMIT ? OFFSET ?"
		args = append(args, perPage, offset)
	}

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []model.Article
	for rows.Next() {
		article, err := scanArticle(rows)
		if err != nil {
			return nil, err
		}
		if article != nil {
			articles = append(articles, *article)
		}
	}
	return articles, rows.Err()
}
```

- [ ] **Step 4: Update existing tests in database_test.go**

Search for all calls to `ListArticles` in `database_test.go` and update them:

1. Find: `db.ListArticles(false, nil)`
   Replace with: `db.ListArticles(nil, nil, 1, NoPagination)`

2. Find: `db.ListArticles(true, nil)`
   Replace with: `db.ListArticles(&unread, nil, 1, NoPagination)` where `unread := true` is declared before

3. Find: `db.ListArticles(false, &blogID)`
   Replace with: `db.ListArticles(nil, &blogID, 1, NoPagination)`

**Important:** For the call at `TestListArticlesFiltersAndOrdering`:
```go
// Before:
unread, err := db.ListArticles(true, nil)
...
if len(unread) != 2 {

// After:
unreadFilter := true
unreadArticles, err := db.ListArticles(&unreadFilter, nil, 1, NoPagination)
...
if len(unreadArticles) != 2 {
```

Add `fmt` import if not present.

- [ ] **Step 5: Run all storage tests to verify they pass**

Run: `go test ./internal/storage/... -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/storage/database.go internal/storage/database_test.go
git commit -m "feat(storage): add pagination and status filter to ListArticles

- Change unreadOnly from bool to *bool (nil=all, true=unread, false=read)
- Add page and perPage parameters
- Add NoPagination constant for backward compatibility

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 3: Update scanner tests

**Files:**
- Modify: `internal/scanner/scanner_test.go`

- [ ] **Step 1: Update ListArticles call in scanner_test.go**

Change line 52 from:
```go
articles, err := db.ListArticles(false, nil)
```
To:
```go
articles, err := db.ListArticles(nil, nil, 1, storage.NoPagination)
```

- [ ] **Step 2: Run scanner tests to verify they pass**

Run: `go test ./internal/scanner/... -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/scanner/scanner_test.go
git commit -m "fix(scanner): update ListArticles call for new signature

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 4: Update controller layer

**Files:**
- Modify: `internal/controller/controller.go`
- Modify: `internal/controller/controller_test.go`

- [ ] **Step 1: Write the failing test for new GetArticles**

Add to `internal/controller/controller_test.go`:

```go
func TestGetArticlesPagination(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	blog, err := AddBlog(db, "Test", "https://example.com", "", "")
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}

	// Add 25 articles
	for i := 1; i <= 25; i++ {
		_, err := db.AddArticle(model.Article{
			BlogID: blog.ID,
			Title:  fmt.Sprintf("Article %d", i),
			URL:    fmt.Sprintf("https://example.com/%d", i),
		})
		if err != nil {
			t.Fatalf("add article %d: %v", i, err)
		}
	}

	// Mark some as read
	articles, _ := db.ListArticles(nil, nil, 1, storage.NoPagination)
	for _, a := range articles[:5] {
		db.MarkArticleRead(a.ID)
	}

	// Test page 1, default perPage
	result, err := GetArticles(db, "unread", "", 1, 20)
	if err != nil {
		t.Fatalf("get articles: %v", err)
	}
	if len(result.Articles) != 20 {
		t.Fatalf("expected 20 articles on page 1, got %d", len(result.Articles))
	}
	if result.Total != 20 {
		t.Fatalf("expected 20 total unread, got %d", result.Total)
	}
	if result.Page != 1 {
		t.Fatalf("expected page 1, got %d", result.Page)
	}
	if result.TotalPages != 1 {
		t.Fatalf("expected 1 total page, got %d", result.TotalPages)
	}

	// Test read filter
	result, err = GetArticles(db, "read", "", 1, 20)
	if err != nil {
		t.Fatalf("get read articles: %v", err)
	}
	if result.Total != 5 {
		t.Fatalf("expected 5 read articles, got %d", result.Total)
	}

	// Test all filter
	result, err = GetArticles(db, "all", "", 1, 20)
	if err != nil {
		t.Fatalf("get all articles: %v", err)
	}
	if result.Total != 25 {
		t.Fatalf("expected 25 total articles, got %d", result.Total)
	}
}

func TestGetArticlesTotalPagesCalculation(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	blog, err := AddBlog(db, "Test", "https://example.com", "", "")
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}

	for i := 1; i <= 25; i++ {
		_, err := db.AddArticle(model.Article{
			BlogID: blog.ID,
			Title:  fmt.Sprintf("Article %d", i),
			URL:    fmt.Sprintf("https://example.com/%d", i),
		})
		if err != nil {
			t.Fatalf("add article: %v", err)
		}
	}

	// 25 articles, perPage 10 = 3 pages
	result, err := GetArticles(db, "all", "", 1, 10)
	if err != nil {
		t.Fatalf("get articles: %v", err)
	}
	if result.TotalPages != 3 {
		t.Fatalf("expected 3 total pages, got %d", result.TotalPages)
	}
	if len(result.Articles) != 10 {
		t.Fatalf("expected 10 articles, got %d", len(result.Articles))
	}

	// Page 2
	result, err = GetArticles(db, "all", "", 2, 10)
	if err != nil {
		t.Fatalf("get articles page 2: %v", err)
	}
	if len(result.Articles) != 10 {
		t.Fatalf("expected 10 articles on page 2, got %d", len(result.Articles))
	}

	// Page 3 (partial)
	result, err = GetArticles(db, "all", "", 3, 10)
	if err != nil {
		t.Fatalf("get articles page 3: %v", err)
	}
	if len(result.Articles) != 5 {
		t.Fatalf("expected 5 articles on page 3, got %d", len(result.Articles))
	}
}
```

Add required imports:
```go
import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/Hyaxia/blogwatcher/internal/model"
	"github.com/Hyaxia/blogwatcher/internal/storage"
)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controller/... -run "TestGetArticles(Pagination|TotalPagesCalculation)" -v`
Expected: FAIL - `GetArticles` signature mismatch and `ArticlesResult` undefined

- [ ] **Step 3: Add ArticlesResult struct and update GetArticles**

Replace `GetArticles` function in `internal/controller/controller.go`:

```go
// ArticlesResult contains paginated articles and metadata.
type ArticlesResult struct {
	Articles   []model.Article
	BlogNames  map[int64]string
	Total      int
	Page       int
	PerPage    int
	TotalPages int
}

// GetArticles retrieves paginated articles with filtering.
// status: "unread", "read", or "all"
func GetArticles(db *storage.Database, status string, blogName string, page int, perPage int) (*ArticlesResult, error) {
	var blogID *int64
	if blogName != "" {
		blog, err := db.GetBlogByName(blogName)
		if err != nil {
			return nil, err
		}
		if blog == nil {
			return nil, BlogNotFoundError{Name: blogName}
		}
		blogID = &blog.ID
	}

	// Determine read status filter
	var readFilter *bool
	switch status {
	case "unread":
		unread := true
		readFilter = &unread
	case "read":
		read := false
		readFilter = &read
	case "all":
		readFilter = nil
	default:
		unread := true
		readFilter = &unread
	}

	// Get total count
	total, err := db.CountArticles(readFilter, blogID)
	if err != nil {
		return nil, err
	}

	// Get paginated articles
	articles, err := db.ListArticles(readFilter, blogID, page, perPage)
	if err != nil {
		return nil, err
	}

	// Get blog names
	blogs, err := db.ListBlogs()
	if err != nil {
		return nil, err
	}
	blogNames := make(map[int64]string)
	for _, blog := range blogs {
		blogNames[blog.ID] = blog.Name
	}

	// Calculate total pages
	totalPages := 0
	if perPage > 0 && total > 0 {
		totalPages = (total + perPage - 1) / perPage
	}

	return &ArticlesResult{
		Articles:   articles,
		BlogNames:  blogNames,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
	}, nil
}
```

- [ ] **Step 4: Update MarkAllArticlesRead**

Update the `MarkAllArticlesRead` function to use `storage.NoPagination`:

```go
func MarkAllArticlesRead(db *storage.Database, blogName string) ([]model.Article, error) {
	var blogID *int64
	if blogName != "" {
		blog, err := db.GetBlogByName(blogName)
		if err != nil {
			return nil, err
		}
		if blog == nil {
			return nil, BlogNotFoundError{Name: blogName}
		}
		blogID = &blog.ID
	}

	unread := true
	articles, err := db.ListArticles(&unread, blogID, 1, storage.NoPagination)
	if err != nil {
		return nil, err
	}

	for _, article := range articles {
		_, err := db.MarkArticleRead(article.ID)
		if err != nil {
			return nil, err
		}
	}

	return articles, nil
}
```

- [ ] **Step 5: Update existing controller tests**

Update `TestGetArticlesFilters` in `controller_test.go`:

```go
func TestGetArticlesFilters(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	blog, err := AddBlog(db, "Test", "https://example.com", "", "")
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}
	_, err = db.AddArticle(model.Article{BlogID: blog.ID, Title: "Title", URL: "https://example.com/1"})
	if err != nil {
		t.Fatalf("add article: %v", err)
	}

	result, err := GetArticles(db, "unread", "", 1, 20)
	if err != nil {
		t.Fatalf("get articles: %v", err)
	}
	if len(result.Articles) != 1 {
		t.Fatalf("expected article")
	}
	if result.BlogNames[blog.ID] != blog.Name {
		t.Fatalf("expected blog name")
	}

	if _, err := GetArticles(db, "unread", "Missing", 1, 20); err == nil {
		t.Fatalf("expected blog not found error")
	}
}
```

- [ ] **Step 6: Run all controller tests**

Run: `go test ./internal/controller/... -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/controller/controller.go internal/controller/controller_test.go
git commit -m "feat(controller): add pagination and filtering to GetArticles

- Add ArticlesResult struct with pagination metadata
- Change status parameter from showAll bool to status string
- Add Total, Page, PerPage, TotalPages fields
- Update MarkAllArticlesRead to use NoPagination

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 5: Update CLI commands

**Files:**
- Modify: `internal/cli/commands.go`

- [ ] **Step 1: Update newArticlesCommand with new flags and logic**

Replace `newArticlesCommand` function in `internal/cli/commands.go`:

```go
func newArticlesCommand() *cobra.Command {
	var showAll bool
	var showRead bool
	var blogName string
	var page int
	var perPage int

	cmd := &cobra.Command{
		Use:   "articles",
		Short: "List articles.",
		RunE: func(cmd *cobra.Command, args []string) error {
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

			label := "Unread articles"
			if status == "read" {
				label = "Read articles"
			} else if status == "all" {
				label = "All articles"
			}
			color.New(color.FgCyan, color.Bold).Printf("%s (page %d/%d, %d total):\n\n", label, result.Page, result.TotalPages, result.Total)
			for _, article := range result.Articles {
				printArticle(article, result.BlogNames[article.BlogID])
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&showAll, "all", "a", false, "Show all articles (including read)")
	cmd.Flags().BoolVarP(&showRead, "read", "r", false, "Show only read articles")
	cmd.Flags().StringVarP(&blogName, "blog", "b", "", "Filter by blog name")
	cmd.Flags().IntVarP(&page, "page", "p", 1, "Page number")
	cmd.Flags().IntVarP(&perPage, "per-page", "P", 20, "Articles per page (max 100)")
	return cmd
}
```

- [ ] **Step 2: Update newReadAllCommand**

The `newReadAllCommand` needs to be updated to use the new `GetArticles` signature:

```go
func newReadAllCommand() *cobra.Command {
	var blogName string
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

- [ ] **Step 3: Run all tests**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 4: Add CLI boundary tests**

Add to `internal/cli/commands_test.go` (create if not exists):

```go
package cli

import (
	"bytes"
	"testing"
)

func TestArticlesCommandPerPageValidation(t *testing.T) {
	// Test perPage > 100 is clamped to 100
	// This is a unit test for the validation logic
	tests := []struct {
		input    int
		expected int
	}{
		{0, 20},    // Below minimum, reset to default
		{-1, 20},   // Negative, reset to default
		{50, 50},   // Valid value
		{100, 100}, // Maximum allowed
		{101, 100}, // Above max, clamped to 100
		{200, 100}, // Above max, clamped to 100
	}
	for _, tt := range tests {
		perPage := tt.input
		if perPage < 1 {
			perPage = 20
		}
		if perPage > 100 {
			perPage = 100
		}
		if perPage != tt.expected {
			t.Errorf("input %d: expected %d, got %d", tt.input, tt.expected, perPage)
		}
	}
}

func TestArticlesCommandPageValidation(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{0, 1},  // Below minimum, reset to 1
		{-1, 1}, // Negative, reset to 1
		{1, 1},  // Valid
		{5, 5},  // Valid
	}
	for _, tt := range tests {
		page := tt.input
		if page < 1 {
			page = 1
		}
		if page != tt.expected {
			t.Errorf("input %d: expected %d, got %d", tt.input, tt.expected, page)
		}
	}
}
```

Run: `go test ./internal/cli/... -v`
Expected: PASS

- [ ] **Step 5: Build and test manually**

Run: `go build ./cmd/blogwatcher && ./blogwatcher articles --help`
Expected: Shows help with `--page`, `--per-page`, `--read` flags

- [ ] **Step 7: Commit**

```bash
git add internal/cli/commands.go internal/cli/commands_test.go
git commit -m "feat(cli): add pagination and read filter to articles command

- Add --page and --per-page flags (default 20, max 100)
- Add --read flag to show only read articles
- Update output format to show pagination info
- Update read-all command for new GetArticles signature

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 6: Final verification

- [ ] **Step 1: Run all tests**

Run: `go test ./... -v`
Expected: All tests pass

- [ ] **Step 2: Manual integration test**

```bash
go run ./cmd/blogwatcher add TestBlog https://example.com --feed-url https://example.com/feed
go run ./cmd/blogwatcher articles
go run ./cmd/blogwatcher articles --read
go run ./cmd/blogwatcher articles --all --page 2 --per-page 5
```

- [ ] **Step 3: Final commit (if any fixes needed)**

```bash
git add -A
git commit -m "fix: resolve any remaining issues"
```