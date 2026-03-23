package controller

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/Hyaxia/blogwatcher/internal/model"
	"github.com/Hyaxia/blogwatcher/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddBlogAndRemoveBlog(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	blog, err := AddBlog(db, "Test", "https://example.com", "", "")
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}

	if _, err := AddBlog(db, "Test", "https://other.com", "", ""); err == nil {
		t.Fatalf("expected duplicate name error")
	}

	if _, err := AddBlog(db, "Other", "https://example.com", "", ""); err == nil {
		t.Fatalf("expected duplicate url error")
	}

	if err := RemoveBlog(db, blog.Name); err != nil {
		t.Fatalf("remove blog: %v", err)
	}
}

func TestArticleReadUnread(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	blog, err := AddBlog(db, "Test", "https://example.com", "", "")
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}
	article, err := db.AddArticle(model.Article{BlogID: blog.ID, Title: "Title", URL: "https://example.com/1"})
	if err != nil {
		t.Fatalf("add article: %v", err)
	}

	read, err := MarkArticleRead(db, article.ID)
	if err != nil {
		t.Fatalf("mark read: %v", err)
	}
	if read.IsRead {
		t.Fatalf("expected original state unread")
	}

	unread, err := MarkArticleUnread(db, article.ID)
	if err != nil {
		t.Fatalf("mark unread: %v", err)
	}
	if !unread.IsRead {
		t.Fatalf("expected original state read")
	}
}

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

	result, err := GetArticles(db, "unread", "", 1, 20, "")
	if err != nil {
		t.Fatalf("get articles: %v", err)
	}
	if len(result.Articles) != 1 {
		t.Fatalf("expected article")
	}
	if result.BlogNames[blog.ID] != blog.Name {
		t.Fatalf("expected blog name")
	}

	if _, err := GetArticles(db, "unread", "Missing", 1, 20, ""); err == nil {
		t.Fatalf("expected blog not found error")
	}
}

func openTestDB(t *testing.T) *storage.Database {
	t.Helper()
	path := filepath.Join(t.TempDir(), "blogwatcher.db")
	db, err := storage.OpenDatabase(path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	return db
}

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
	articles, _ := db.ListArticles(nil, nil, 0, 1, storage.NoPagination, "")
	for _, a := range articles[:5] {
		db.MarkArticleRead(a.ID)
	}

	// Test page 1, default perPage
	result, err := GetArticles(db, "unread", "", 1, 20, "")
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
	result, err = GetArticles(db, "read", "", 1, 20, "")
	if err != nil {
		t.Fatalf("get read articles: %v", err)
	}
	if result.Total != 5 {
		t.Fatalf("expected 5 read articles, got %d", result.Total)
	}

	// Test all filter
	result, err = GetArticles(db, "all", "", 1, 20, "")
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
	result, err := GetArticles(db, "all", "", 1, 10, "")
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
	result, err = GetArticles(db, "all", "", 2, 10, "")
	if err != nil {
		t.Fatalf("get articles page 2: %v", err)
	}
	if len(result.Articles) != 10 {
		t.Fatalf("expected 10 articles on page 2, got %d", len(result.Articles))
	}

	// Page 3 (partial)
	result, err = GetArticles(db, "all", "", 3, 10, "")
	if err != nil {
		t.Fatalf("get articles page 3: %v", err)
	}
	if len(result.Articles) != 5 {
		t.Fatalf("expected 5 articles on page 3, got %d", len(result.Articles))
	}
}

func TestUpdateBlog(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	blog, err := AddBlog(db, "Original", "https://example.com", "", "")
	require.NoError(t, err)

	updated, err := UpdateBlog(db, blog.ID, "New Name", "https://newurl.com", "https://feed.com/rss", "article a")
	require.NoError(t, err)
	assert.Equal(t, "New Name", updated.Name)
	assert.Equal(t, "https://newurl.com", updated.URL)
	assert.Equal(t, "https://feed.com/rss", updated.FeedURL)
}

func TestGetBlogStats(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	blog, err := AddBlog(db, "Test", "https://example.com", "", "")
	require.NoError(t, err)

	// Add some articles
	for i := 0; i < 5; i++ {
		_, err := db.AddArticle(model.Article{BlogID: blog.ID, Title: fmt.Sprintf("Article %d", i), URL: fmt.Sprintf("https://example.com/%d", i)})
		require.NoError(t, err)
	}
	db.MarkArticleRead(1)

	stats, err := GetBlogStats(db, blog.ID)
	require.NoError(t, err)
	assert.Equal(t, 5, stats.TotalArticles)
	assert.Equal(t, 4, stats.UnreadArticles)
}

func TestGetBlogByID(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	blog, _ := AddBlog(db, "Test Blog", "https://test.com", "", "")

	found, err := GetBlogByID(db, blog.ID)
	require.NoError(t, err)
	assert.Equal(t, "Test Blog", found.Name)

	// Test not found
	_, err = GetBlogByID(db, 999)
	assert.IsType(t, BlogIDNotFoundError{}, err)
}

func TestRemoveBlogByID(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	blog, _ := AddBlog(db, "Test Blog", "https://test.com", "", "")

	err := RemoveBlogByID(db, blog.ID)
	require.NoError(t, err)

	// Verify removed
	found, _ := db.GetBlog(blog.ID)
	assert.Nil(t, found)

	// Test not found
	err = RemoveBlogByID(db, 999)
	assert.IsType(t, BlogIDNotFoundError{}, err)
}

func TestGetBlogsPaginated(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	AddBlog(db, "Alpha", "https://alpha.com", "", "")
	AddBlog(db, "Beta", "https://beta.com", "", "")
	AddBlog(db, "Gamma", "https://gamma.com", "", "")

	result, err := GetBlogsPaginated(db, 1, 2, "")
	require.NoError(t, err)
	assert.Equal(t, 3, result.Total)
	assert.Equal(t, 2, result.TotalPages)
	assert.Len(t, result.Blogs, 2)
}

func TestGetArticlesWithSearch(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	blog1, err := AddBlog(db, "Blog1", "https://blog1.com", "", "")
	require.NoError(t, err)
	blog2, err := AddBlog(db, "Blog2", "https://blog2.com", "", "")
	require.NoError(t, err)

	_, err = db.AddArticle(model.Article{BlogID: blog1.ID, Title: "Go Tips", URL: "https://blog1.com/1"})
	require.NoError(t, err)
	_, err = db.AddArticle(model.Article{BlogID: blog1.ID, Title: "Python Guide", URL: "https://blog1.com/2"})
	require.NoError(t, err)
	_, err = db.AddArticle(model.Article{BlogID: blog2.ID, Title: "Rust Intro", URL: "https://blog2.com/1"})
	require.NoError(t, err)

	// Test basic pagination
	result, err := GetArticlesWithSearch(db, "unread", "", nil, "", 1, 20)
	require.NoError(t, err)
	assert.Equal(t, 3, result.Total)
	assert.Len(t, result.Articles, 3)
	assert.Equal(t, 1, result.Page)
	assert.Equal(t, 20, result.PerPage)
	assert.Equal(t, 1, result.TotalPages)

	// Test search filter
	result, err = GetArticlesWithSearch(db, "unread", "", nil, "go", 1, 20)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Total)
	assert.Len(t, result.Articles, 1)
	assert.Equal(t, "Go Tips", result.Articles[0].Title)

	// Test blogID filter with valid ID
	result, err = GetArticlesWithSearch(db, "unread", "", &blog1.ID, "", 1, 20)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Total)
	assert.Len(t, result.Articles, 2)

	// Test blogID filter with invalid ID
	invalidID := int64(999)
	_, err = GetArticlesWithSearch(db, "unread", "", &invalidID, "", 1, 20)
	assert.IsType(t, BlogIDNotFoundError{}, err)

	// Test blogName takes precedence over blogID
	result, err = GetArticlesWithSearch(db, "unread", "Blog2", &blog1.ID, "", 1, 20)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Total) // Blog2 has 1 article, not blog1's 2
	assert.Len(t, result.Articles, 1)
	assert.Equal(t, "Rust Intro", result.Articles[0].Title)

	// Test empty results
	emptyResult, err := GetArticlesWithSearch(db, "unread", "", nil, "nonexistent", 1, 20)
	require.NoError(t, err)
	assert.Equal(t, 0, emptyResult.Total)
	assert.Len(t, emptyResult.Articles, 0)
	assert.Equal(t, 1, emptyResult.TotalPages)
}
