package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Hyaxia/blogwatcher/internal/model"
)

func TestDatabaseCreatesFileAndCRUD(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected db file to exist: %v", err)
	}

	blog, err := db.AddBlog(model.Blog{Name: "Test", URL: "https://example.com"})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}
	if blog.ID == 0 {
		t.Fatal("expected blog ID")
	}

	fetched, err := db.GetBlog(blog.ID)
	if err != nil {
		t.Fatalf("get blog: %v", err)
	}
	if fetched == nil || fetched.Name != "Test" {
		t.Fatalf("unexpected blog: %+v", fetched)
	}

	articles := []model.Article{
		{BlogID: blog.ID, Title: "One", URL: "https://example.com/1"},
		{BlogID: blog.ID, Title: "Two", URL: "https://example.com/2"},
	}
	count, err := db.AddArticlesBulk(articles)
	if err != nil {
		t.Fatalf("add articles bulk: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 articles, got %d", count)
	}

	list, err := db.ListArticles(nil, nil, 1, NoPagination)
	if err != nil {
		t.Fatalf("list articles: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 articles, got %d", len(list))
	}

	ok, err := db.MarkArticleRead(list[0].ID)
	if err != nil || !ok {
		t.Fatalf("mark read: %v", err)
	}

	updated, err := db.GetArticle(list[0].ID)
	if err != nil {
		t.Fatalf("get article: %v", err)
	}
	if updated == nil || !updated.IsRead {
		t.Fatalf("expected article read: %+v", updated)
	}

	now := time.Now()
	if err := db.UpdateBlogLastScanned(blog.ID, now); err != nil {
		t.Fatalf("update last scanned: %v", err)
	}

	deleted, err := db.RemoveBlog(blog.ID)
	if err != nil {
		t.Fatalf("remove blog: %v", err)
	}
	if !deleted {
		t.Fatalf("expected blog removal")
	}
}

func TestGetExistingArticleURLs(t *testing.T) {
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

	_, err = db.AddArticle(model.Article{BlogID: blog.ID, Title: "One", URL: "https://example.com/1"})
	if err != nil {
		t.Fatalf("add article: %v", err)
	}

	existing, err := db.GetExistingArticleURLs([]string{"https://example.com/1", "https://example.com/2"})
	if err != nil {
		t.Fatalf("get existing: %v", err)
	}
	if _, ok := existing["https://example.com/1"]; !ok {
		t.Fatalf("expected existing url")
	}
	if _, ok := existing["https://example.com/2"]; ok {
		t.Fatalf("did not expect url")
	}
}

func TestDatabaseForeignKeyEnforced(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if _, err := db.AddArticle(model.Article{BlogID: 9999, Title: "Orphan", URL: "https://example.com/orphan"}); err == nil {
		t.Fatalf("expected foreign key error for missing blog")
	}
}

func TestBlogOptionalFieldsRoundTrip(t *testing.T) {
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

	fetched, err := db.GetBlog(blog.ID)
	if err != nil {
		t.Fatalf("get blog: %v", err)
	}
	if fetched == nil {
		t.Fatalf("expected blog")
	}
	if fetched.FeedURL != "" || fetched.ScrapeSelector != "" {
		t.Fatalf("expected empty optional fields: %+v", fetched)
	}
}

func TestBlogTimeRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	now := time.Date(2025, 1, 2, 3, 4, 5, 6, time.UTC)
	blog, err := db.AddBlog(model.Blog{
		Name:        "Test",
		URL:         "https://example.com",
		LastScanned: &now,
	})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}

	fetched, err := db.GetBlog(blog.ID)
	if err != nil {
		t.Fatalf("get blog: %v", err)
	}
	if fetched == nil || fetched.LastScanned == nil {
		t.Fatalf("expected last scanned")
	}
	if !fetched.LastScanned.Equal(now) {
		t.Fatalf("expected last scanned %s, got %s", now.Format(time.RFC3339Nano), fetched.LastScanned.Format(time.RFC3339Nano))
	}
}

func TestArticleTimeRoundTripAndNilDiscoveredDate(t *testing.T) {
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

	published := time.Date(2024, 12, 31, 23, 59, 59, 123, time.UTC)
	article, err := db.AddArticle(model.Article{
		BlogID:        blog.ID,
		Title:         "Title",
		URL:           "https://example.com/1",
		PublishedDate: &published,
	})
	if err != nil {
		t.Fatalf("add article: %v", err)
	}

	fetched, err := db.GetArticle(article.ID)
	if err != nil {
		t.Fatalf("get article: %v", err)
	}
	if fetched == nil || fetched.PublishedDate == nil {
		t.Fatalf("expected published date")
	}
	if !fetched.PublishedDate.Equal(published) {
		t.Fatalf("expected published date %s, got %s", published.Format(time.RFC3339Nano), fetched.PublishedDate.Format(time.RFC3339Nano))
	}
	if fetched.DiscoveredDate != nil {
		t.Fatalf("expected discovered date nil when not set")
	}
}

func TestListArticlesFiltersAndOrdering(t *testing.T) {
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
	blogB, err := db.AddBlog(model.Blog{Name: "B", URL: "https://b.example.com"})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}

	t1 := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC)

	first, err := db.AddArticle(model.Article{BlogID: blogA.ID, Title: "Old", URL: "https://a.example.com/old", DiscoveredDate: &t1})
	if err != nil {
		t.Fatalf("add article: %v", err)
	}
	second, err := db.AddArticle(model.Article{BlogID: blogA.ID, Title: "New", URL: "https://a.example.com/new", DiscoveredDate: &t2})
	if err != nil {
		t.Fatalf("add article: %v", err)
	}
	_, err = db.AddArticle(model.Article{BlogID: blogB.ID, Title: "Other", URL: "https://b.example.com/1", DiscoveredDate: &t2})
	if err != nil {
		t.Fatalf("add article: %v", err)
	}

	if _, err := db.MarkArticleRead(first.ID); err != nil {
		t.Fatalf("mark read: %v", err)
	}

	all, err := db.ListArticles(nil, nil, 1, NoPagination)
	if err != nil {
		t.Fatalf("list articles: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 articles, got %d", len(all))
	}
	if all[0].ID != second.ID {
		t.Fatalf("expected newest article first")
	}

	unreadFilter := true
	unreadArticles, err := db.ListArticles(&unreadFilter, nil, 1, NoPagination)
	if err != nil {
		t.Fatalf("list unread: %v", err)
	}
	if len(unreadArticles) != 2 {
		t.Fatalf("expected 2 unread articles, got %d", len(unreadArticles))
	}

	blogID := blogB.ID
	filtered, err := db.ListArticles(nil, &blogID, 1, NoPagination)
	if err != nil {
		t.Fatalf("list by blog: %v", err)
	}
	if len(filtered) != 1 || filtered[0].BlogID != blogB.ID {
		t.Fatalf("expected one article for blog B")
	}
}

func TestBulkInsertDuplicateRollbackAndEmpty(t *testing.T) {
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

	if count, err := db.AddArticlesBulk(nil); err != nil || count != 0 {
		t.Fatalf("expected empty bulk insert to be no-op, got %d, %v", count, err)
	}

	_, err = db.AddArticle(model.Article{BlogID: blog.ID, Title: "Existing", URL: "https://example.com/existing"})
	if err != nil {
		t.Fatalf("add article: %v", err)
	}

	dupArticles := []model.Article{
		{BlogID: blog.ID, Title: "Dup", URL: "https://example.com/dup"},
		{BlogID: blog.ID, Title: "Dup2", URL: "https://example.com/dup"},
	}
	if _, err := db.AddArticlesBulk(dupArticles); err == nil {
		t.Fatalf("expected bulk insert to fail on duplicate url")
	}

	articles, err := db.ListArticles(nil, nil, 1, NoPagination)
	if err != nil {
		t.Fatalf("list articles: %v", err)
	}
	if len(articles) != 1 {
		t.Fatalf("expected rollback on duplicate, got %d articles", len(articles))
	}
}

func TestLookupHelpers(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "blogwatcher.db")
	db, err := OpenDatabase(path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if blog, err := db.GetBlogByName("missing"); err != nil || blog != nil {
		t.Fatalf("expected missing blog by name")
	}
	if blog, err := db.GetBlogByURL("https://missing.example.com"); err != nil || blog != nil {
		t.Fatalf("expected missing blog by url")
	}

	blog, err := db.AddBlog(model.Blog{Name: "Test", URL: "https://example.com"})
	if err != nil {
		t.Fatalf("add blog: %v", err)
	}
	article, err := db.AddArticle(model.Article{BlogID: blog.ID, Title: "Title", URL: "https://example.com/1"})
	if err != nil {
		t.Fatalf("add article: %v", err)
	}

	if found, err := db.GetArticleByURL(article.URL); err != nil || found == nil {
		t.Fatalf("expected article by url")
	}
	if exists, err := db.ArticleExists(article.URL); err != nil || !exists {
		t.Fatalf("expected article to exist")
	}
	if exists, err := db.ArticleExists("https://example.com/missing"); err != nil || exists {
		t.Fatalf("expected missing article to not exist")
	}
}

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
		articleTime := time.Date(2024, 1, i, 0, 0, 0, 0, time.UTC)
		_, err := db.AddArticle(model.Article{
			BlogID:        blog.ID,
			Title:         fmt.Sprintf("Article %d", i),
			URL:           fmt.Sprintf("https://example.com/%d", i),
			DiscoveredDate: &articleTime,
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
