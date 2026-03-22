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
	if got2.HTMLURL != "https://news.ycombinator.com" {
		t.Errorf("expected HTMLUrl 'https://news.ycombinator.com', got %q", got2.HTMLURL)
	}
	if got2.XMLURL != "https://news.ycombinator.com/rss" {
		t.Errorf("expected XMLUrl 'https://news.ycombinator.com/rss', got %q", got2.XMLURL)
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

func TestExtractOutlines_Nested(t *testing.T) {
	opml := &OPML{}
	opml.Body.Outlines = []Outline{
		{
			Text: "Category",
			Outlines: []Outline{
				{Text: "Blog 1", XMLURL: "https://blog1.com/feed"},
				{Text: "Blog 2", XMLURL: "https://blog2.com/feed"},
			},
		},
		{
			Text:   "Blog 3",
			XMLURL: "https://blog3.com/feed",
		},
	}

	outlines := ExtractOutlines(opml)

	if len(outlines) != 3 {
		t.Fatalf("expected 3 outlines, got %d", len(outlines))
	}

	expectedTexts := []string{"Blog 1", "Blog 2", "Blog 3"}
	for i, expected := range expectedTexts {
		if outlines[i].Text != expected {
			t.Errorf("outlines[%d].Text = %q, want %q", i, outlines[i].Text, expected)
		}
	}
}

func TestExtractOutlines_Empty(t *testing.T) {
	opml := &OPML{}
	outlines := ExtractOutlines(opml)

	if len(outlines) != 0 {
		t.Errorf("expected 0 outlines, got %d", len(outlines))
	}
}

func TestExtractOutlines_DeeplyNested(t *testing.T) {
	opml := &OPML{}
	opml.Body.Outlines = []Outline{
		{
			Text: "Level 1",
			Outlines: []Outline{
				{
					Text: "Level 2",
					Outlines: []Outline{
						{Text: "Deep Blog", XMLURL: "https://deep.com/feed"},
					},
				},
			},
		},
	}

	outlines := ExtractOutlines(opml)

	if len(outlines) != 1 {
		t.Fatalf("expected 1 outline, got %d", len(outlines))
	}
	if outlines[0].Text != "Deep Blog" {
		t.Errorf("expected 'Deep Blog', got %q", outlines[0].Text)
	}
}