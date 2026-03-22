package controller

import (
	"path/filepath"
	"testing"

	"github.com/Hyaxia/blogwatcher/internal/model"
	"github.com/Hyaxia/blogwatcher/internal/opml"
	"github.com/Hyaxia/blogwatcher/internal/storage"
)

func TestImportBlogs_Success(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := storage.OpenDatabase(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	outlines := []opml.Outline{
		{Title: "Blog 1", HTMLURL: "https://blog1.com", XMLURL: "https://blog1.com/feed"},
		{Title: "Blog 2", HTMLURL: "https://blog2.com", XMLURL: "https://blog2.com/feed"},
	}

	result := ImportBlogs(db, outlines)

	if len(result.Imported) != 2 {
		t.Errorf("expected 2 imported, got %d", len(result.Imported))
	}
	if len(result.Skipped) != 0 {
		t.Errorf("expected 0 skipped, got %d", len(result.Skipped))
	}
	if len(result.Failed) != 0 {
		t.Errorf("expected 0 failed, got %d", len(result.Failed))
	}

	blog, err := db.GetBlogByName("Blog 1")
	if err != nil {
		t.Fatal(err)
	}
	if blog == nil {
		t.Error("Blog 1 not found in database")
	}
}

func TestImportBlogs_DuplicateInDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := storage.OpenDatabase(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Pre-add a blog
	_, err = db.AddBlog(model.Blog{
		Name:    "Existing Blog",
		URL:     "https://blog1.com",
		FeedURL: "https://blog1.com/feed",
	})
	if err != nil {
		t.Fatal(err)
	}

	outlines := []opml.Outline{
		{Title: "Blog 1", HTMLURL: "https://blog1.com", XMLURL: "https://blog1.com/feed"},
		{Title: "Blog 2", HTMLURL: "https://blog2.com", XMLURL: "https://blog2.com/feed"},
	}

	result := ImportBlogs(db, outlines)

	if len(result.Imported) != 1 {
		t.Errorf("expected 1 imported, got %d", len(result.Imported))
	}
	if len(result.Skipped) != 1 {
		t.Errorf("expected 1 skipped, got %d", len(result.Skipped))
	}
	if len(result.Failed) != 0 {
		t.Errorf("expected 0 failed, got %d", len(result.Failed))
	}

	if len(result.Skipped) > 0 && result.Skipped[0].Reason != "blog with URL 'https://blog1.com' already exists" {
		t.Errorf("expected reason 'blog with URL 'https://blog1.com' already exists', got %s", result.Skipped[0].Reason)
	}
}

func TestImportBlogs_DuplicateInOPML(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := storage.OpenDatabase(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Same URL appears twice in OPML
	outlines := []opml.Outline{
		{Title: "Blog 1", HTMLURL: "https://blog1.com", XMLURL: "https://blog1.com/feed"},
		{Title: "Blog 1 Duplicate", HTMLURL: "https://blog1.com", XMLURL: "https://blog1.com/feed"},
		{Title: "Blog 2", HTMLURL: "https://blog2.com", XMLURL: "https://blog2.com/feed"},
	}

	result := ImportBlogs(db, outlines)

	if len(result.Imported) != 2 {
		t.Errorf("expected 2 imported, got %d", len(result.Imported))
	}
	if len(result.Skipped) != 1 {
		t.Errorf("expected 1 skipped, got %d", len(result.Skipped))
	}
	if len(result.Failed) != 0 {
		t.Errorf("expected 0 failed, got %d", len(result.Failed))
	}

	if len(result.Skipped) > 0 && result.Skipped[0].Reason != "duplicate within OPML file (URL already imported)" {
		t.Errorf("expected reason 'duplicate within OPML file (URL already imported)', got %s", result.Skipped[0].Reason)
	}
}

func TestImportBlogs_DuplicateNameInOPML(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := storage.OpenDatabase(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Same name appears twice in OPML with different URLs
	outlines := []opml.Outline{
		{Title: "Blog 1", HTMLURL: "https://blog1.com", XMLURL: "https://blog1.com/feed"},
		{Title: "Blog 1", HTMLURL: "https://blog1-alt.com", XMLURL: "https://blog1-alt.com/feed"},
		{Title: "Blog 2", HTMLURL: "https://blog2.com", XMLURL: "https://blog2.com/feed"},
	}

	result := ImportBlogs(db, outlines)

	if len(result.Imported) != 2 {
		t.Errorf("expected 2 imported, got %d", len(result.Imported))
	}
	if len(result.Skipped) != 1 {
		t.Errorf("expected 1 skipped, got %d", len(result.Skipped))
	}
	if len(result.Failed) != 0 {
		t.Errorf("expected 0 failed, got %d", len(result.Failed))
	}

	if len(result.Skipped) > 0 && result.Skipped[0].Reason != "duplicate within OPML file (name already imported)" {
		t.Errorf("expected reason 'duplicate within OPML file (name already imported)', got %s", result.Skipped[0].Reason)
	}
}

func TestImportBlogs_MissingXMLURL(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := storage.OpenDatabase(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	outlines := []opml.Outline{
		{Title: "Blog No URL"}, // No HTMLURL or XMLURL
		{Title: "Blog 1", HTMLURL: "https://blog1.com", XMLURL: "https://blog1.com/feed"},
	}

	result := ImportBlogs(db, outlines)

	if len(result.Imported) != 1 {
		t.Errorf("expected 1 imported, got %d", len(result.Imported))
	}
	if len(result.Failed) != 1 {
		t.Errorf("expected 1 failed, got %d", len(result.Failed))
	}
	if len(result.Failed) > 0 && result.Failed[0].Reason != "missing feed URL" {
		t.Errorf("expected missing feed URL reason, got %s", result.Failed[0].Reason)
	}
}

func TestImportBlogs_NameFallback(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := storage.OpenDatabase(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// No title, but has text
	outlines := []opml.Outline{
		{Text: "Text Name", HTMLURL: "https://blog1.com", XMLURL: "https://blog1.com/feed"},
	}

	result := ImportBlogs(db, outlines)

	if len(result.Imported) != 1 {
		t.Fatalf("expected 1 imported, got %d", len(result.Imported))
	}
	if result.Imported[0].Name != "Text Name" {
		t.Errorf("expected name 'Text Name', got %s", result.Imported[0].Name)
	}
}

func TestImportBlogs_URLFallback(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := storage.OpenDatabase(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// No htmlUrl, but has xmlUrl
	outlines := []opml.Outline{
		{Title: "Blog 1", XMLURL: "https://blog1.com/feed"},
	}

	result := ImportBlogs(db, outlines)

	if len(result.Imported) != 1 {
		t.Fatalf("expected 1 imported, got %d", len(result.Imported))
	}
	if result.Imported[0].URL != "https://blog1.com/feed" {
		t.Errorf("expected URL 'https://blog1.com/feed', got %s", result.Imported[0].URL)
	}

	// Verify blog was stored with xmlUrl as URL
	blog, err := db.GetBlogByName("Blog 1")
	if err != nil {
		t.Fatal(err)
	}
	if blog == nil {
		t.Fatal("Blog not found")
	}
	if blog.URL != "https://blog1.com/feed" {
		t.Errorf("expected blog URL to be 'https://blog1.com/feed', got %s", blog.URL)
	}
}

func TestImportBlogs_DomainFallback(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := storage.OpenDatabase(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// No title or text, name should be derived from domain
	outlines := []opml.Outline{
		{HTMLURL: "https://example.com/blog", XMLURL: "https://example.com/feed"},
	}

	result := ImportBlogs(db, outlines)

	if len(result.Imported) != 1 {
		t.Fatalf("expected 1 imported, got %d", len(result.Imported))
	}
	if result.Imported[0].Name != "example.com" {
		t.Errorf("expected name 'example.com', got %s", result.Imported[0].Name)
	}
}

func TestImportBlogs_EmptyTitleUsesText(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := storage.OpenDatabase(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Empty title, text should be used
	outlines := []opml.Outline{
		{Title: "", Text: "Text Value", HTMLURL: "https://blog1.com", XMLURL: "https://blog1.com/feed"},
	}

	result := ImportBlogs(db, outlines)

	if len(result.Imported) != 1 {
		t.Fatalf("expected 1 imported, got %d", len(result.Imported))
	}
	if result.Imported[0].Name != "Text Value" {
		t.Errorf("expected name 'Text Value', got %s", result.Imported[0].Name)
	}
}

func TestParseFile_RealOPML(t *testing.T) {
	opmlData, err := opml.ParseFile("C:/Users/dajun/Downloads/tiny.opml")
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}

	outlines := opml.ExtractOutlines(opmlData)
	if len(outlines) == 0 {
		t.Error("expected to extract feeds from real OPML")
	}

	// Verify all outlines have xmlUrl
	for i, o := range outlines {
		if o.XMLURL == "" {
			t.Errorf("outlines[%d] missing xmlUrl", i)
		}
	}

	t.Logf("Extracted %d feeds from real OPML file", len(outlines))
}