# CLI Pagination and Search Enhancement

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add pagination and search functionality to blogs/articles commands, and change blogs commands to use ID instead of name.

**Architecture:** Extend existing commands with new flags and modify command arguments to use numeric IDs. Storage layer needs search/filter methods.

**Tech Stack:** Go 1.24, Cobra CLI framework, SQLite

---

## Overview

### Changes

1. **blogs list**: Add ID display, pagination (`--page`, `--per-page`), and search (`--search`)
2. **blogs show/edit/remove**: Use ID instead of name
3. **articles list**: Add search (`--search`) and blog ID filter (`--blog-id`)
4. **Error messages**: Update to reference IDs instead of names

### New Command Structure

```
blogs                       # List blogs (with ID, pagination, search)
blogs <id>                  # Show blog details
blogs add <url>             # Add blog (auto-name)
blogs add <name> <url>      # Add blog (custom name)
blogs edit <id>             # Edit blog
blogs remove <id>           # Remove blog

articles                    # List articles (with search, blog-id filter)
articles <id>               # Show article details
articles read <id>          # Mark as read
articles unread <id>        # Mark as unread
articles read-all           # Mark all as read
```

---

## blogs Command

### blogs list

```
Usage:
  blogwatcher blogs [flags]

Flags:
  -p, --page num        Page number (default: 1, min: 1)
  -P, --per-page num    Blogs per page (default: 20, min: 1, max: 100)
  -s, --search keyword  Filter by blog name (partial match)

Output:
  Tracked blogs (page 1/1, 3 total):

    [1] Tech Blog
         URL: https://example.com
         Feed: https://example.com/rss.xml
         Last scanned: 2024-01-15 10:30

    [2] Another Blog
         URL: https://another.com
         ...
```

### blogs show

```
Usage:
  blogwatcher blogs <id>

Arguments:
  id    Blog ID (required)

Output:
  Blog: Tech Blog
  ID: 1
  URL: https://example.com
  Feed: https://example.com/rss.xml
  Selector: (none)
  Last scanned: 2024-01-15 10:30
  Articles: 42 total, 5 unread
```

### blogs edit

```
Usage:
  blogwatcher blogs edit <id> [flags]

Arguments:
  id    Blog ID (required)

Flags:
  --name name            New blog name
  --url url              New blog URL
  --feed-url url         New feed URL
  --scrape-selector sel  New scrape selector
```

### blogs remove

```
Usage:
  blogwatcher blogs remove <id> [flags]

Arguments:
  id    Blog ID (required)

Flags:
  -y, --yes    Skip confirmation prompt
```

---

## articles Command

### articles list

```
Usage:
  blogwatcher articles [flags]

Flags:
  -a, --all              Show all articles (including read)
  -r, --read             Show only read articles
  -b, --blog name        Filter by blog name (existing)
  --blog-id id           Filter by blog ID (new)
  -s, --search keyword   Filter by article title (partial match)
  -p, --page num         Page number (default: 1)
  -P, --per-page num     Articles per page (default: 20, max: 100)
  -f, --fields list      Fields to display (comma-separated)
```

---

## Error Handling

### blogs Errors

| Command | Condition | Message |
|---------|-----------|---------|
| `blogs <id>` | Blog not found | `Error: Blog <id> not found` |
| `blogs edit <id>` | Blog not found | `Error: Blog <id> not found` |
| `blogs remove <id>` | Blog not found | `Error: Blog <id> not found` |
| `blogs <id>` | Invalid ID format | `Error: Invalid blog ID: <value>` |

### articles Errors

| Command | Condition | Message |
|---------|-----------|---------|
| `articles --blog-id <id>` | Blog not found | `Error: Blog <id> not found` |

---

## Storage Layer Changes

### New Methods Required

```go
// ListBlogsPaginated returns paginated blogs with optional name filter
func (db *Database) ListBlogsPaginated(page, perPage int, search string) (BlogListResult, error)

type BlogListResult struct {
    Blogs      []model.Blog
    Total      int
    Page       int
    TotalPages int
}

// ListArticlesSearch returns articles with title search filter
// search: partial match on article title (empty string = no filter)
func (db *Database) ListArticlesSearch(readFilter *bool, blogID *int64, search string, page, perPage int) (ArticleListResult, error)
```

### SQL Queries

**Blog search (case-insensitive partial match):**
```sql
SELECT * FROM blogs WHERE name LIKE '%search%' ORDER BY name LIMIT ? OFFSET ?
SELECT COUNT(*) FROM blogs WHERE name LIKE '%search%'
```

**Article title search (case-insensitive partial match):**
```sql
SELECT * FROM articles WHERE title LIKE '%search%' AND ... ORDER BY discovered_date DESC LIMIT ? OFFSET ?
```

---

## Controller Layer Changes

### Update Functions

```go
// GetBlogByID retrieves a blog by its ID
func GetBlogByID(db *storage.Database, id int64) (*model.Blog, error)

// UpdateBlogByID updates blog using ID instead of internal fetch
func UpdateBlogByID(db *storage.Database, id int64, name, url, feedURL, scrapeSelector string) (model.Blog, error)

// RemoveBlogByID removes a blog by its ID
func RemoveBlogByID(db *storage.Database, id int64) error

// GetBlogsPaginated returns paginated blog list with search
func GetBlogsPaginated(db *storage.Database, page, perPage int, search string) (BlogListResult, error)

// GetArticlesWithSearch returns articles with search filter
func GetArticlesWithSearch(db *storage.Database, status, blogName string, blogID *int64, search string, page, perPage int) (ArticlesResult, error)
```

---

## Files to Modify

| File | Changes |
|------|---------|
| `internal/cli/commands.go` | Update blogs/articles commands with new flags and ID-based args |
| `internal/cli/commands_test.go` | Update tests for new command structure |
| `internal/controller/controller.go` | Add new functions for ID-based operations and search |
| `internal/controller/controller_test.go` | Add tests for new functions |
| `internal/storage/database.go` | Add pagination and search methods |
| `internal/storage/database_test.go` | Add tests for new methods |
| `README.md` | Update documentation |

---

## Implementation Tasks

1. Add `ListBlogsPaginated` to storage layer
2. Add `ListArticlesSearch` to storage layer (extends existing pagination)
3. Add `GetBlogByID`, `UpdateBlogByID`, `RemoveBlogByID` to controller
4. Add `GetBlogsPaginated`, `GetArticlesWithSearch` to controller
5. Update `newBlogsCommand` with pagination, search, and ID-based args
6. Update `newArticlesCommand` with search and `--blog-id` flag
7. Update tests
8. Update README