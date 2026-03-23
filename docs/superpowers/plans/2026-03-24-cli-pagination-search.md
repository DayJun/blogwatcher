# CLI Pagination and Search Enhancement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add pagination and search to blogs/articles commands, change blogs commands to use ID instead of name.

**Architecture:** Extend storage layer with paginated/search methods, update controller with ID-based operations, modify CLI commands to use new flags and ID arguments.

**Tech Stack:** Go 1.24, Cobra CLI framework, SQLite

---

## File Structure

```
internal/
├── cli/
│   ├── commands.go          # Modify: blogs/articles commands with new flags
│   └── commands_test.go      # Modify: update tests
├── controller/
│   ├── controller.go         # Modify: add ID-based functions, search
│   └── controller_test.go    # Modify: add tests
└── storage/
    ├── database.go           # Modify: add paginated/search methods
    └── database_test.go      # Modify: add tests
```

---

## Task 1: Add BlogListResult and ListBlogsPaginated to storage

**Files:**
- Modify: `internal/storage/database.go`
- Modify: `internal/storage/database_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestListBlogsPaginated(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Add test blogs
	db.AddBlog(model.Blog{Name: "Alpha Blog", URL: "https://alpha.com"})
	db.AddBlog(model.Blog{Name: "Beta Blog", URL: "https://beta.com"})
	db.AddBlog(model.Blog{Name: "Gamma Blog", URL: "https://gamma.com"})
	db.AddBlog(model.Blog{Name: "Delta Blog", URL: "https://delta.com"})

	// Test pagination
	result, err := db.ListBlogsPaginated(1, 2, "")
	require.NoError(t, err)
	assert.Equal(t, 4, result.Total)
	assert.Equal(t, 1, result.Page)
	assert.Equal(t, 2, result.TotalPages)
	assert.Len(t, result.Blogs, 2)
	assert.Equal(t, "Alpha Blog", result.Blogs[0].Name) // Ordered by name

	// Test search
	result, err = db.ListBlogsPaginated(1, 20, "ta")
	require.NoError(t, err)
	assert.Equal(t, 2, result.Total) // Beta and Delta match
	assert.Len(t, result.Blogs, 2)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/storage/... -run TestListBlogsPaginated -v`
Expected: FAIL

- [ ] **Step 3: Implement BlogListResult and ListBlogsPaginated**

```go
// BlogListResult contains paginated blog list results
type BlogListResult struct {
	Blogs      []model.Blog
	Total      int
	Page       int
	TotalPages int
}

// ListBlogsPaginated returns paginated blogs with optional name search filter.
// search is a case-insensitive partial match on blog name (empty = no filter).
func (db *Database) ListBlogsPaginated(page, perPage int, search string) (BlogListResult, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}

	// Build WHERE clause for search
	whereClause := "1=1"
	var args []interface{}
	if search != "" {
		whereClause = "name LIKE ?"
		args = append(args, "%"+search+"%")
	}

	// Get total count
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM blogs WHERE %s", whereClause)
	row := db.conn.QueryRow(countQuery, args...)
	var total int
	if err := row.Scan(&total); err != nil {
		return BlogListResult{}, err
	}

	// Calculate pagination
	totalPages := (total + perPage - 1) / perPage
	if totalPages < 1 {
		totalPages = 1
	}

	// Get paginated results
	offset := (page - 1) * perPage
	query := fmt.Sprintf("SELECT id, name, url, feed_url, scrape_selector, last_scanned FROM blogs WHERE %s ORDER BY name LIMIT ? OFFSET ?", whereClause)
	args = append(args, perPage, offset)

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return BlogListResult{}, err
	}
	defer rows.Close()

	var blogs []model.Blog
	for rows.Next() {
		blog, err := scanBlog(rows)
		if err != nil {
			return BlogListResult{}, err
		}
		if blog != nil {
			blogs = append(blogs, *blog)
		}
	}

	return BlogListResult{
		Blogs:      blogs,
		Total:      total,
		Page:       page,
		TotalPages: totalPages,
	}, rows.Err()
}

// CountBlogs returns total number of blogs, optionally filtered by name search.
func (db *Database) CountBlogs(search string) (int, error) {
	whereClause := "1=1"
	var args []interface{}
	if search != "" {
		whereClause = "name LIKE ?"
		args = append(args, "%"+search+"%")
	}

	query := fmt.Sprintf("SELECT COUNT(*) FROM blogs WHERE %s", whereClause)
	row := db.conn.QueryRow(query, args...)
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/storage/... -run TestListBlogsPaginated -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/storage/database.go internal/storage/database_test.go
git commit -m "feat(storage): add ListBlogsPaginated with search support"
```

---

## Task 2: Add search parameter to ListArticles and CountArticles

**Files:**
- Modify: `internal/storage/database.go`
- Modify: `internal/storage/database_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestListArticlesWithSearch(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	blog, _ := db.AddBlog(model.Blog{Name: "Test", URL: "https://test.com"})
	db.AddArticle(model.Article{BlogID: blog.ID, Title: "Go Programming Tips", URL: "https://test.com/1"})
	db.AddArticle(model.Article{BlogID: blog.ID, Title: "Python Tutorial", URL: "https://test.com/2"})
	db.AddArticle(model.Article{BlogID: blog.ID, Title: "Go Concurrency Guide", URL: "https://test.com/3"})

	// Test search
	articles, err := db.ListArticles(nil, nil, 0, 1, 20, "go")
	require.NoError(t, err)
	assert.Len(t, articles, 2) // "Go Programming Tips" and "Go Concurrency Guide"

	// Test count with search
	count, err := db.CountArticles(nil, nil, "python")
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/storage/... -run TestListArticlesWithSearch -v`
Expected: FAIL

- [ ] **Step 3: Modify ListArticles and CountArticles signatures**

Update `ListArticles` signature to add search parameter:

```go
func (db *Database) ListArticles(unreadOnly *bool, blogID *int64, days int, page int, perPage int, search string) ([]model.Article, error) {
	query := `SELECT id, blog_id, title, url, published_date, discovered_date, is_read, content, description, feed_summary, summary FROM articles WHERE 1=1`
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
	if days > 0 {
		query += " AND discovered_date >= datetime('now', '-' || ? || ' days')"
		args = append(args, days)
	}
	if search != "" {
		query += " AND title LIKE ?"
		args = append(args, "%"+search+"%")
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

func (db *Database) CountArticles(unreadOnly *bool, blogID *int64, search string) (int, error) {
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
	if search != "" {
		query += " AND title LIKE ?"
		args = append(args, "%"+search+"%")
	}

	row := db.conn.QueryRow(query, args...)
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}
```

- [ ] **Step 4: Update all callers of ListArticles and CountArticles**

Update `internal/controller/controller.go`:
- `GetArticles` function: add `search` parameter, pass to `db.ListArticles` and `db.CountArticles`

Update `internal/cli/commands.go`:
- `runSummaryAll`: pass empty string for search parameter

Update `internal/scanner/scanner.go` if needed.

- [ ] **Step 5: Run all tests**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/storage/database.go internal/storage/database_test.go internal/controller/controller.go internal/cli/commands.go
git commit -m "feat(storage): add search parameter to ListArticles and CountArticles"
```

---

## Task 3: Add ID-based blog functions to controller

**Files:**
- Modify: `internal/controller/controller.go`
- Modify: `internal/controller/controller_test.go`

- [ ] **Step 1: Write the failing tests**

```go
func TestGetBlogByID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	blog, _ := controller.AddBlog(db, "Test Blog", "https://test.com", "", "")

	found, err := controller.GetBlogByID(db, blog.ID)
	require.NoError(t, err)
	assert.Equal(t, "Test Blog", found.Name)

	// Test not found
	_, err = controller.GetBlogByID(db, 999)
	assert.IsType(t, controller.BlogNotFoundError{}, err)
}

func TestRemoveBlogByID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	blog, _ := controller.AddBlog(db, "Test Blog", "https://test.com", "", "")

	err := controller.RemoveBlogByID(db, blog.ID)
	require.NoError(t, err)

	// Verify removed
	found, _ := db.GetBlog(blog.ID)
	assert.Nil(t, found)

	// Test not found
	err = controller.RemoveBlogByID(db, 999)
	assert.IsType(t, controller.BlogNotFoundError{}, err)
}

func TestGetBlogsPaginated(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	controller.AddBlog(db, "Alpha", "https://alpha.com", "", "")
	controller.AddBlog(db, "Beta", "https://beta.com", "", "")
	controller.AddBlog(db, "Gamma", "https://gamma.com", "", "")

	result, err := controller.GetBlogsPaginated(db, 1, 2, "")
	require.NoError(t, err)
	assert.Equal(t, 3, result.Total)
	assert.Equal(t, 2, result.TotalPages)
	assert.Len(t, result.Blogs, 2)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controller/... -run "TestGetBlogByID|TestRemoveBlogByID|TestGetBlogsPaginated" -v`
Expected: FAIL

- [ ] **Step 3: Implement the functions**

```go
// BlogIDNotFoundError is returned when a blog is not found by ID.
type BlogIDNotFoundError struct {
	ID int64
}

func (e BlogIDNotFoundError) Error() string {
	return fmt.Sprintf("Blog %d not found", e.ID)
}

// GetBlogByID retrieves a blog by its ID.
func GetBlogByID(db *storage.Database, id int64) (*model.Blog, error) {
	blog, err := db.GetBlog(id)
	if err != nil {
		return nil, err
	}
	if blog == nil {
		return nil, BlogIDNotFoundError{ID: id}
	}
	return blog, nil
}

// RemoveBlogByID removes a blog by its ID.
func RemoveBlogByID(db *storage.Database, id int64) error {
	blog, err := db.GetBlog(id)
	if err != nil {
		return err
	}
	if blog == nil {
		return BlogIDNotFoundError{ID: id}
	}
	_, err = db.RemoveBlog(id)
	return err
}

// BlogListResult contains paginated blog list results.
type BlogListResult struct {
	Blogs      []model.Blog
	Total      int
	Page       int
	TotalPages int
}

// GetBlogsPaginated returns paginated blogs with optional name search.
func GetBlogsPaginated(db *storage.Database, page, perPage int, search string) (BlogListResult, error) {
	return db.ListBlogsPaginated(page, perPage, search)
}

// GetArticlesWithSearch returns articles with search filter.
func GetArticlesWithSearch(db *storage.Database, status string, blogName string, blogID *int64, search string, page, perPage int) (*ArticlesResult, error) {
	// Resolve blog name to ID if provided
	var filterBlogID *int64
	if blogName != "" {
		blog, err := db.GetBlogByName(blogName)
		if err != nil {
			return nil, err
		}
		if blog == nil {
			return nil, BlogNotFoundError{Name: blogName}
		}
		filterBlogID = &blog.ID
	} else if blogID != nil {
		// Verify blog exists
		blog, err := db.GetBlog(*blogID)
		if err != nil {
			return nil, err
		}
		if blog == nil {
			return nil, BlogIDNotFoundError{ID: *blogID}
		}
		filterBlogID = blogID
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
	total, err := db.CountArticles(readFilter, filterBlogID, search)
	if err != nil {
		return nil, err
	}

	// Get paginated articles
	articles, err := db.ListArticles(readFilter, filterBlogID, 0, page, perPage, search)
	if err != nil {
		return nil, err
	}

	// Get blog names
	blogs, err := db.ListBlogs()
	if err != nil {
		return nil, err
	}
	blogNames := make(map[int64]string)
	for _, b := range blogs {
		blogNames[b.ID] = b.Name
	}

	// Calculate total pages
	totalPages := (total + perPage - 1) / perPage
	if totalPages < 1 {
		totalPages = 1
	}

	return &ArticlesResult{
		Articles:   articles,
		Total:      total,
		Page:       page,
		TotalPages: totalPages,
		BlogNames:  blogNames,
	}, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/controller/... -run "TestGetBlogByID|TestRemoveBlogByID|TestGetBlogsPaginated" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/controller/controller.go internal/controller/controller_test.go
git commit -m "feat(controller): add ID-based blog functions and search support"
```

---

## Task 4: Update blogs command with pagination, search, and ID-based args

**Files:**
- Modify: `internal/cli/commands.go`
- Modify: `internal/cli/commands_test.go`

- [ ] **Step 1: Write tests for updated blogs command**

```go
func TestBlogsListPaginated(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	controller.AddBlog(db, "Alpha", "https://alpha.com", "", "")
	controller.AddBlog(db, "Beta", "https://beta.com", "", "")
	controller.AddBlog(db, "Gamma", "https://gamma.com", "", "")

	cmd := newBlogsCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--per-page", "2"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "[1]")
	assert.Contains(t, buf.String(), "[2]")
}

func TestBlogsListSearch(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	controller.AddBlog(db, "Alpha Blog", "https://alpha.com", "", "")
	controller.AddBlog(db, "Beta Site", "https://beta.com", "", "")

	cmd := newBlogsCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--search", "blog"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Alpha Blog")
	assert.NotContains(t, buf.String(), "Beta Site")
}

func TestBlogsShowByID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	blog, _ := controller.AddBlog(db, "Test Blog", "https://test.com", "https://test.com/feed", "")

	cmd := newBlogsCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{fmt.Sprintf("%d", blog.ID)})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Blog: Test Blog")
	assert.Contains(t, buf.String(), fmt.Sprintf("ID: %d", blog.ID))
}

func TestBlogsEditByID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	blog, _ := controller.AddBlog(db, "Original", "https://test.com", "", "")

	cmd := newBlogsCommand()
	cmd.SetArgs([]string{"edit", fmt.Sprintf("%d", blog.ID), "--name", "Updated"})
	err := cmd.Execute()
	require.NoError(t, err)

	updated, _ := db.GetBlog(blog.ID)
	assert.Equal(t, "Updated", updated.Name)
}

func TestBlogsRemoveByID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	blog, _ := controller.AddBlog(db, "Test Blog", "https://test.com", "", "")

	cmd := newBlogsCommand()
	cmd.SetArgs([]string{"remove", fmt.Sprintf("%d", blog.ID), "--yes"})
	err := cmd.Execute()
	require.NoError(t, err)

	removed, _ := db.GetBlog(blog.ID)
	assert.Nil(t, removed)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/... -run "TestBlogsListPaginated|TestBlogsListSearch|TestBlogsShowByID|TestBlogsEditByID|TestBlogsRemoveByID" -v`
Expected: FAIL

- [ ] **Step 3: Update newBlogsCommand and related functions**

Replace the existing `newBlogsCommand`, `runBlogsList`, `runBlogsShow`, `newBlogsEditCommand`, and `newBlogsRemoveCommand`:

```go
func newBlogsCommand() *cobra.Command {
	var page int
	var perPage int
	var search string

	cmd := &cobra.Command{
		Use:   "blogs [id]",
		Short: "Manage tracked blogs.",
		Long: `Manage tracked blogs.

Without arguments, lists all tracked blogs.
With a blog ID argument, shows detailed information about that blog.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := RequireConfig(); err != nil {
				printError(err)
				return markError(err)
			}

			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()

			if len(args) == 0 {
				return runBlogsList(db, page, perPage, search)
			}
			return runBlogsShow(db, args[0])
		},
	}

	cmd.Flags().IntVarP(&page, "page", "p", 1, "Page number")
	cmd.Flags().IntVarP(&perPage, "per-page", "P", 20, "Blogs per page (max 100)")
	cmd.Flags().StringVarP(&search, "search", "s", "", "Filter by blog name (partial match)")

	cmd.AddCommand(newBlogsAddCommand())
	cmd.AddCommand(newBlogsEditCommand())
	cmd.AddCommand(newBlogsRemoveCommand())

	return cmd
}

func runBlogsList(db *storage.Database, page, perPage int, search string) error {
	// Validate pagination
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}

	result, err := controller.GetBlogsPaginated(db, page, perPage, search)
	if err != nil {
		printError(err)
		return markError(err)
	}

	if result.Total == 0 {
		if search != "" {
			color.New(color.FgCyan, color.Bold).Printf("No blogs found matching '%s'\n", search)
		} else {
			fmt.Println("No blogs tracked yet. Use 'blogwatcher blogs add' to add one.")
		}
		return nil
	}

	label := "Tracked blogs"
	if search != "" {
		label = fmt.Sprintf("Blogs matching '%s'", search)
	}
	color.New(color.FgCyan, color.Bold).Printf("%s (page %d/%d, %d total):\n\n", label, result.Page, result.TotalPages, result.Total)

	for _, blog := range result.Blogs {
		color.New(color.FgWhite, color.Bold).Printf("  [%d] %s\n", blog.ID, blog.Name)
		fmt.Printf("       URL: %s\n", blog.URL)
		if blog.FeedURL != "" {
			fmt.Printf("       Feed: %s\n", blog.FeedURL)
		} else {
			fmt.Println("       Feed: (auto-discovered)")
		}
		if blog.LastScanned != nil {
			fmt.Printf("       Last scanned: %s\n", blog.LastScanned.Format("2006-01-02 15:04"))
		} else {
			fmt.Println("       Last scanned: never")
		}
		fmt.Println()
	}
	return nil
}

func runBlogsShow(db *storage.Database, idStr string) error {
	blogID, err := parseID(idStr)
	if err != nil {
		printError(fmt.Errorf("Invalid blog ID: %s", idStr))
		return markError(err)
	}

	blog, err := controller.GetBlogByID(db, blogID)
	if err != nil {
		printError(err)
		return markError(err)
	}

	stats, err := controller.GetBlogStats(db, blog.ID)
	if err != nil {
		return err
	}

	fmt.Printf("Blog: %s\n", blog.Name)
	fmt.Printf("ID: %d\n", blog.ID)
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

func newBlogsEditCommand() *cobra.Command {
	var name string
	var url string
	var feedURL string
	var scrapeSelector string

	cmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "Edit a tracked blog.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := RequireConfig(); err != nil {
				printError(err)
				return markError(err)
			}

			blogID, err := parseID(args[0])
			if err != nil {
				printError(fmt.Errorf("Invalid blog ID: %s", args[0]))
				return markError(err)
			}

			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()

			_, err = controller.UpdateBlog(db, blogID, name, url, feedURL, scrapeSelector)
			if err != nil {
				printError(err)
				return markError(err)
			}
			color.New(color.FgGreen).Printf("Blog %d updated.\n", blogID)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "New blog name")
	cmd.Flags().StringVar(&url, "url", "", "New blog URL")
	cmd.Flags().StringVar(&feedURL, "feed-url", "", "New feed URL")
	cmd.Flags().StringVar(&scrapeSelector, "scrape-selector", "", "New scrape selector")
	return cmd
}

func newBlogsRemoveCommand() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "remove <id>",
		Short: "Remove a blog from tracking.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := RequireConfig(); err != nil {
				printError(err)
				return markError(err)
			}

			blogID, err := parseID(args[0])
			if err != nil {
				printError(fmt.Errorf("Invalid blog ID: %s", args[0]))
				return markError(err)
			}

			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()

			// Get blog name for confirmation message
			blog, err := db.GetBlog(blogID)
			if err != nil {
				return err
			}
			if blog == nil {
				err := controller.BlogIDNotFoundError{ID: blogID}
				printError(err)
				return markError(err)
			}

			if !yes {
				confirmed, err := confirm(fmt.Sprintf("Remove blog '%s' (ID: %d) and all its articles?", blog.Name, blogID))
				if err != nil {
					return err
				}
				if !confirmed {
					return nil
				}
			}

			if err := controller.RemoveBlogByID(db, blogID); err != nil {
				printError(err)
				return markError(err)
			}
			color.New(color.FgGreen).Printf("Removed blog '%s' (ID: %d)\n", blog.Name, blogID)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt")
	return cmd
}
```

- [ ] **Step 4: Run tests to verify**

Run: `go test ./internal/cli/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/commands.go internal/cli/commands_test.go
git commit -m "feat(cli): update blogs command with pagination, search, and ID-based args"
```

---

## Task 5: Update articles command with search and --blog-id flag

**Files:**
- Modify: `internal/cli/commands.go`
- Modify: `internal/cli/commands_test.go`

- [ ] **Step 1: Write tests for updated articles command**

```go
func TestArticlesListSearch(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	blog, _ := db.AddBlog(model.Blog{Name: "Test", URL: "https://test.com"})
	db.AddArticle(model.Article{BlogID: blog.ID, Title: "Go Programming", URL: "https://test.com/1"})
	db.AddArticle(model.Article{BlogID: blog.ID, Title: "Python Tutorial", URL: "https://test.com/2"})

	cmd := newArticlesCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--search", "go"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Go Programming")
	assert.NotContains(t, buf.String(), "Python")
}

func TestArticlesListByBlogID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	blog1, _ := db.AddBlog(model.Blog{Name: "Blog1", URL: "https://blog1.com"})
	blog2, _ := db.AddBlog(model.Blog{Name: "Blog2", URL: "https://blog2.com"})
	db.AddArticle(model.Article{BlogID: blog1.ID, Title: "Article 1", URL: "https://blog1.com/1"})
	db.AddArticle(model.Article{BlogID: blog2.ID, Title: "Article 2", URL: "https://blog2.com/1"})

	cmd := newArticlesCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--blog-id", fmt.Sprintf("%d", blog1.ID)})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Article 1")
	assert.NotContains(t, buf.String(), "Article 2")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/... -run "TestArticlesListSearch|TestArticlesListByBlogID" -v`
Expected: FAIL

- [ ] **Step 3: Update newArticlesCommand**

Modify `newArticlesCommand` to add `--search` and `--blog-id` flags:

```go
func newArticlesCommand() *cobra.Command {
	var showAll bool
	var showRead bool
	var blogName string
	var blogID int64
	var page int
	var perPage int
	var fields string
	var search string

	cmd := &cobra.Command{
		Use:   "articles [id]",
		Short: "Manage articles.",
		Long: `Manage articles.

Without arguments, lists unread articles.
With an article ID, shows detailed information about that article.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := RequireConfig(); err != nil {
				printError(err)
				return markError(err)
			}

			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()

			if len(args) == 1 {
				return runArticlesShow(db, args[0])
			}

			// Build blogID pointer
			var blogIDPtr *int64
			if blogID > 0 {
				blogIDPtr = &blogID
			}

			return runArticlesList(db, showAll, showRead, blogName, blogIDPtr, search, page, perPage, fields)
		},
	}

	cmd.Flags().BoolVarP(&showAll, "all", "a", false, "Show all articles (including read)")
	cmd.Flags().BoolVarP(&showRead, "read", "r", false, "Show only read articles")
	cmd.Flags().StringVarP(&blogName, "blog", "b", "", "Filter by blog name")
	cmd.Flags().Int64Var(&blogID, "blog-id", 0, "Filter by blog ID")
	cmd.Flags().IntVarP(&page, "page", "p", 1, "Page number")
	cmd.Flags().IntVarP(&perPage, "per-page", "P", 20, "Articles per page (max 100)")
	cmd.Flags().StringVar(&fields, "fields", "id,title,blog,read,url,published", "Comma-separated fields to display")
	cmd.Flags().StringVarP(&search, "search", "s", "", "Filter by article title (partial match)")

	cmd.AddCommand(newArticlesReadCommand())
	cmd.AddCommand(newArticlesUnreadCommand())
	cmd.AddCommand(newArticlesReadAllCommand())

	return cmd
}

func runArticlesList(db *storage.Database, showAll bool, showRead bool, blogName string, blogID *int64, search string, page int, perPage int, fields string) error {
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

	result, err := controller.GetArticlesWithSearch(db, status, blogName, blogID, search, page, perPage)
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
		msg := fmt.Sprintf("%s (no results)", label)
		if search != "" {
			msg = fmt.Sprintf("%s matching '%s' (no results)", label, search)
		}
		color.New(color.FgCyan, color.Bold).Println(msg)
		return nil
	}

	// Parse fields
	fieldList := parseFields(fields)

	label := "Unread articles"
	if status == "read" {
		label = "Read articles"
	} else if status == "all" {
		label = "All articles"
	}
	if search != "" {
		label = fmt.Sprintf("%s matching '%s'", label, search)
	}
	color.New(color.FgCyan, color.Bold).Printf("%s (page %d/%d, %d total):\n\n", label, result.Page, result.TotalPages, result.Total)

	for _, article := range result.Articles {
		output := FormatArticleFields(&article, result.BlogNames, fieldList)
		fmt.Printf("  %s\n", output)
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify**

Run: `go test ./internal/cli/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/commands.go internal/cli/commands_test.go
git commit -m "feat(cli): add search and --blog-id flags to articles command"
```

---

## Task 6: Update README documentation

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Update README with new flags**

Update the documentation to reflect:
- `blogs` now shows ID, supports `--page`, `--per-page`, `--search`
- `blogs <id>`, `blogs edit <id>`, `blogs remove <id>` now use ID
- `articles` supports `--search` and `--blog-id`

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: update README with pagination and search features"
```

---

## Final Verification

- [ ] **Run all tests**

Run: `go test ./...`
Expected: All PASS

- [ ] **Build and smoke test**

Run: `go build ./cmd/blogwatcher && ./blogwatcher --help`
Verify all commands appear correctly

- [ ] **Test new functionality**

```bash
./blogwatcher blogs --help
./blogwatcher blogs --search "test"
./blogwatcher articles --search "go" --blog-id 1
```