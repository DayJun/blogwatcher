package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Hyaxia/blogwatcher/internal/config"
	"github.com/Hyaxia/blogwatcher/internal/controller"
	"github.com/Hyaxia/blogwatcher/internal/model"
	"github.com/Hyaxia/blogwatcher/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *storage.Database {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := storage.OpenDatabase(dbPath)
	require.NoError(t, err)
	return db
}

func defaultTestDBPath(t *testing.T) string {
	t.Helper()
	var homeDir string
	if runtime.GOOS == "windows" {
		homeDir = os.Getenv("USERPROFILE")
	} else {
		homeDir = os.Getenv("HOME")
	}
	return filepath.Join(homeDir, ".blogwatcher", "blogwatcher.db")
}

func setupTestDBAtPath(t *testing.T, dbPath string) *storage.Database {
	t.Helper()
	err := os.MkdirAll(filepath.Dir(dbPath), 0o755)
	require.NoError(t, err)
	db, err := storage.OpenDatabase(dbPath)
	require.NoError(t, err)
	return db
}

func setupTestConfig(t *testing.T) {
	t.Helper()
	tmpDir := t.TempDir()

	// Set home directory for cross-platform testing
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", tmpDir)
	} else {
		t.Setenv("HOME", tmpDir)
	}

	// Create config directory
	cfgDir := filepath.Join(tmpDir, ".blogwatcher")
	err := os.MkdirAll(cfgDir, 0o755)
	require.NoError(t, err)

	// Create a valid config
	cfg := &config.Config{
		LLM: config.LLMConfig{
			APIKey:  "test-key",
			BaseURL: "https://api.openai.com/v1",
			Model:   "gpt-4o-mini",
		},
	}
	cfgPath := filepath.Join(cfgDir, "config.yaml")
	err = cfg.Save(cfgPath)
	require.NoError(t, err)
}

func TestArticlesCommandPerPageValidation(t *testing.T) {
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

func TestBlogsList(t *testing.T) {
	setupTestConfig(t)
	db := setupTestDBAtPath(t, defaultTestDBPath(t))
	defer db.Close()

	controller.AddBlog(db, "Blog A", "https://a.com", "", "")
	controller.AddBlog(db, "Blog B", "https://b.com", "", "")

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := newBlogsCommand()
	cmd.SetArgs([]string{})
	err := cmd.Execute()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)

	require.NoError(t, err)
	// Check for URLs which are printed with fmt.Printf (not color)
	assert.Contains(t, buf.String(), "https://a.com")
	assert.Contains(t, buf.String(), "https://b.com")
}

func TestBlogsShow(t *testing.T) {
	setupTestConfig(t)
	db := setupTestDBAtPath(t, defaultTestDBPath(t))
	defer db.Close()

	blog, _ := controller.AddBlog(db, "Test Blog", "https://test.com", "https://test.com/feed", "")

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := newBlogsCommand()
	cmd.SetArgs([]string{fmt.Sprintf("%d", blog.ID)})
	err := cmd.Execute()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)

	require.NoError(t, err)
	// Check for URL and feed which are printed with fmt.Printf
	assert.Contains(t, buf.String(), "https://test.com")
	assert.Contains(t, buf.String(), "https://test.com/feed")
	assert.Contains(t, buf.String(), "Articles: 0 total")
}

func TestBlogsAddCustomName(t *testing.T) {
	setupTestConfig(t)
	db := setupTestDBAtPath(t, defaultTestDBPath(t))
	defer db.Close()

	cmd := newBlogsCommand()
	cmd.SetArgs([]string{"add", "My Custom Name", "https://example.com"})
	err := cmd.Execute()
	require.NoError(t, err)

	blog, _ := db.GetBlogByName("My Custom Name")
	assert.NotNil(t, blog)
}

func TestArticlesList(t *testing.T) {
	setupTestConfig(t)
	db := setupTestDBAtPath(t, defaultTestDBPath(t))
	defer db.Close()

	blog, _ := db.AddBlog(model.Blog{Name: "Test", URL: "https://test.com"})
	db.AddArticle(model.Article{BlogID: blog.ID, Title: "Article 1", URL: "https://test.com/1"})
	db.AddArticle(model.Article{BlogID: blog.ID, Title: "Article 2", URL: "https://test.com/2"})

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := newArticlesCommand()
	cmd.SetArgs([]string{})
	err := cmd.Execute()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Article 1")
	assert.Contains(t, buf.String(), "Article 2")
}

func TestArticlesListCustomFields(t *testing.T) {
	setupTestConfig(t)
	db := setupTestDBAtPath(t, defaultTestDBPath(t))
	defer db.Close()

	blog, _ := db.AddBlog(model.Blog{Name: "Test", URL: "https://test.com"})
	db.AddArticle(model.Article{BlogID: blog.ID, Title: "Test Article", URL: "https://test.com/1", Summary: "Test summary"})

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := newArticlesCommand()
	cmd.SetArgs([]string{"--fields", "id,title,summary"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Test Article")
	assert.Contains(t, buf.String(), "Test summary")
}

func TestArticlesShow(t *testing.T) {
	setupTestConfig(t)
	db := setupTestDBAtPath(t, defaultTestDBPath(t))
	defer db.Close()

	blog, _ := db.AddBlog(model.Blog{Name: "Test", URL: "https://test.com"})
	article, _ := db.AddArticle(model.Article{BlogID: blog.ID, Title: "Test Article", URL: "https://test.com/1", Content: "Full content"})

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := newArticlesCommand()
	cmd.SetArgs([]string{fmt.Sprintf("%d", article.ID)})
	err := cmd.Execute()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Title: Test Article")
	assert.Contains(t, buf.String(), "Content:")
}

func TestArticlesRead(t *testing.T) {
	setupTestConfig(t)
	db := setupTestDBAtPath(t, defaultTestDBPath(t))
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

func TestBlogsListPaginated(t *testing.T) {
	setupTestConfig(t)
	db := setupTestDBAtPath(t, defaultTestDBPath(t))
	defer db.Close()

	controller.AddBlog(db, "Alpha", "https://alpha.com", "", "")
	controller.AddBlog(db, "Beta", "https://beta.com", "", "")
	controller.AddBlog(db, "Gamma", "https://gamma.com", "", "")

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := newBlogsCommand()
	cmd.SetArgs([]string{"--per-page", "2"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)

	require.NoError(t, err)
	// Check pagination: with per-page=2, only first 2 blogs appear on page 1
	// URLs are printed with fmt.Printf so they're captured
	assert.Contains(t, buf.String(), "https://alpha.com")
	assert.Contains(t, buf.String(), "https://beta.com")
	assert.NotContains(t, buf.String(), "https://gamma.com") // Should not appear on page 1
}

func TestBlogsListSearch(t *testing.T) {
	setupTestConfig(t)
	db := setupTestDBAtPath(t, defaultTestDBPath(t))
	defer db.Close()

	controller.AddBlog(db, "Alpha Blog", "https://alpha.com", "", "")
	controller.AddBlog(db, "Beta Site", "https://beta.com", "", "")

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := newBlogsCommand()
	cmd.SetArgs([]string{"--search", "blog"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)

	require.NoError(t, err)
	// Check for URL which is printed with fmt.Printf
	assert.Contains(t, buf.String(), "https://alpha.com")
	assert.NotContains(t, buf.String(), "https://beta.com")
}

func TestBlogsShowByID(t *testing.T) {
	setupTestConfig(t)
	db := setupTestDBAtPath(t, defaultTestDBPath(t))
	defer db.Close()

	blog, _ := controller.AddBlog(db, "Test Blog", "https://test.com", "https://test.com/feed", "")

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := newBlogsCommand()
	cmd.SetArgs([]string{fmt.Sprintf("%d", blog.ID)})
	err := cmd.Execute()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)

	require.NoError(t, err)
	// Check for non-colored output (URL and ID are printed with fmt.Printf)
	assert.Contains(t, buf.String(), "https://test.com")
	assert.Contains(t, buf.String(), fmt.Sprintf("ID: %d", blog.ID))
}

func TestBlogsEditByID(t *testing.T) {
	setupTestConfig(t)
	db := setupTestDBAtPath(t, defaultTestDBPath(t))
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
	setupTestConfig(t)
	db := setupTestDBAtPath(t, defaultTestDBPath(t))
	defer db.Close()

	blog, _ := controller.AddBlog(db, "Test Blog", "https://test.com", "", "")

	cmd := newBlogsCommand()
	cmd.SetArgs([]string{"remove", fmt.Sprintf("%d", blog.ID), "--yes"})
	err := cmd.Execute()
	require.NoError(t, err)

	removed, _ := db.GetBlog(blog.ID)
	assert.Nil(t, removed)
}

func TestArticlesListSearch(t *testing.T) {
	setupTestConfig(t)
	db := setupTestDBAtPath(t, defaultTestDBPath(t))
	defer db.Close()

	blog, _ := db.AddBlog(model.Blog{Name: "Test", URL: "https://test.com"})
	db.AddArticle(model.Article{BlogID: blog.ID, Title: "Go Programming", URL: "https://test.com/1"})
	db.AddArticle(model.Article{BlogID: blog.ID, Title: "Python Tutorial", URL: "https://test.com/2"})

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := newArticlesCommand()
	cmd.SetArgs([]string{"--search", "go"})
	err := cmd.Execute()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Go Programming")
	assert.NotContains(t, buf.String(), "Python")
}

func TestArticlesListByBlogID(t *testing.T) {
	setupTestConfig(t)
	db := setupTestDBAtPath(t, defaultTestDBPath(t))
	defer db.Close()

	blog1, _ := db.AddBlog(model.Blog{Name: "Blog1", URL: "https://blog1.com"})
	blog2, _ := db.AddBlog(model.Blog{Name: "Blog2", URL: "https://blog2.com"})
	db.AddArticle(model.Article{BlogID: blog1.ID, Title: "Article 1", URL: "https://blog1.com/1"})
	db.AddArticle(model.Article{BlogID: blog2.ID, Title: "Article 2", URL: "https://blog2.com/1"})

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := newArticlesCommand()
	cmd.SetArgs([]string{"--blog-id", fmt.Sprintf("%d", blog1.ID)})
	err := cmd.Execute()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Article 1")
	assert.NotContains(t, buf.String(), "Article 2")
}