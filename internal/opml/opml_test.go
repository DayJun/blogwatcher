package opml

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFile_ValidOPML(t *testing.T) {
	content := `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
  <body>
    <outline text="TechCrunch" title="TechCrunch" htmlUrl="https://techcrunch.com" xmlUrl="https://techcrunch.com/feed/"/>
    <outline text="Hacker News" htmlUrl="https://news.ycombinator.com" xmlUrl="https://news.ycombinator.com/rss"/>
  </body>
</opml>`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.opml")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	opml, err := ParseFile(tmpFile)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}

	if len(opml.Body.Outlines) != 2 {
		t.Fatalf("expected 2 outlines, got %d", len(opml.Body.Outlines))
	}

	got := opml.Body.Outlines[0]
	if got.Title != "TechCrunch" {
		t.Errorf("expected Title 'TechCrunch', got %q", got.Title)
	}
	if got.Text != "TechCrunch" {
		t.Errorf("expected Text 'TechCrunch', got %q", got.Text)
	}
	if got.HTMLURL != "https://techcrunch.com" {
		t.Errorf("expected HTMLUrl 'https://techcrunch.com', got %q", got.HTMLURL)
	}
	if got.XMLURL != "https://techcrunch.com/feed/" {
		t.Errorf("expected XMLUrl 'https://techcrunch.com/feed/', got %q", got.XMLURL)
	}

	got2 := opml.Body.Outlines[1]
	if got2.Title != "" {
		t.Errorf("expected empty Title, got %q", got2.Title)
	}
	if got2.Text != "Hacker News" {
		t.Errorf("expected Text 'Hacker News', got %q", got2.Text)
	}
}

func TestParseFile_InvalidXML(t *testing.T) {
	content := `<?xml version="1.0"?>
<opml><body><outline text="Test"</body></opml>`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.opml")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := ParseFile(tmpFile)
	if err == nil {
		t.Error("expected error for malformed XML, got nil")
	}
}

func TestParseFile_FileNotFound(t *testing.T) {
	_, err := ParseFile("/nonexistent/path/to/file.opml")
	if err == nil {
		t.Error("expected error for non-existent file, got nil")
	}
}