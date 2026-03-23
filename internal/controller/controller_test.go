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
