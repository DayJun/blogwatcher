# OPML Import Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an `import` command that parses OPML files and bulk-imports blogs into BlogWatcher.

**Architecture:** Create a new `internal/opml` package for XML parsing with recursive outline extraction. Add import logic to controller layer. CLI command delegates to controller and formats output.

**Tech Stack:** Go 1.24+, encoding/xml (standard library), cobra CLI, existing controller/storage patterns.

---

## File Structure

| File | Purpose |
|------|---------|
| `internal/opml/opml.go` | OPML XML parsing, struct definitions, outline extraction |
| `internal/opml/opml_test.go` | Unit tests for OPML parsing |
| `internal/controller/import.go` | Import business logic, result types |
| `internal/cli/commands.go` | `newImportCommand()` CLI handler |
| `internal/cli/root.go` | Register import command |

---

### Task 1: OPML Struct Definitions and ParseFile Function

**Files:**
- Create: `internal/opml/opml.go`
- Create: `internal/opml/opml_test.go`

- [ ] **Step 1: Write failing test for parsing valid OPML**

```go
// internal/opml/opml_test.go
package opml

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFile_ValidOPML(t *testing.T) {
	// Create temp OPML file
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

	// Check first outline
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

	// Check second outline (no title, should use text)
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
<opml><body><outline text="Test"</body></opml>` // Malformed - missing closing >

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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd C:/zone/work/aigc/Yan-Studio/blogwatcher && go test ./internal/opml/... -v`
Expected: FAIL - package opml not found or ParseFile undefined

- [ ] **Step 3: Write minimal implementation**

```go
// internal/opml/opml.go
package opml

import (
	"encoding/xml"
	"os"
)

type OPML struct {
	XMLName xml.Name `xml:"opml"`
	Body    struct {
		Outlines []Outline `xml:"outline"`
	} `xml:"body"`
}

type Outline struct {
	Text     string    `xml:"text,attr"`
	Title    string    `xml:"title,attr"`
	HTMLURL  string    `xml:"htmlUrl,attr"`
	XMLURL   string    `xml:"xmlUrl,attr"`
	Outlines []Outline `xml:"outline"`
}

func ParseFile(path string) (*OPML, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var opml OPML
	if err := xml.Unmarshal(data, &opml); err != nil {
		return nil, err
	}

	return &opml, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd C:/zone/work/aigc/Yan-Studio/blogwatcher && go test ./internal/opml/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/opml/opml.go internal/opml/opml_test.go
git commit -m "feat(opml): add OPML parsing with ParseFile function

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 2: ExtractOutlines Function for Nested Outlines

**Files:**
- Modify: `internal/opml/opml.go`
- Modify: `internal/opml/opml_test.go`

- [ ] **Step 1: Write failing test for nested outlines extraction**

```go
// Add to internal/opml/opml_test.go

func TestExtractOutlines_Nested(t *testing.T) {
	opml := &OPML{}
	opml.Body.Outlines = []Outline{
		{
			Text:   "Category",
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

	// Should extract nested outlines, not the category itself
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd C:/zone/work/aigc/Yan-Studio/blogwatcher && go test ./internal/opml/... -v`
Expected: FAIL - ExtractOutlines undefined

- [ ] **Step 3: Write minimal implementation**

```go
// Add to internal/opml/opml.go

// ExtractOutlines recursively extracts all outlines with xmlUrl from an OPML document.
// It skips category outlines (those without xmlUrl) and only returns actual feed entries.
func ExtractOutlines(opml *OPML) []Outline {
	var result []Outline
	extractOutlinesRecursive(opml.Body.Outlines, &result)
	return result
}

func extractOutlinesRecursive(outlines []Outline, result *[]Outline) {
	for _, o := range outlines {
		// If this outline has an xmlUrl, it's a feed entry
		if o.XMLURL != "" {
			*result = append(*result, o)
		}
		// Always recurse into nested outlines (categories)
		if len(o.Outlines) > 0 {
			extractOutlinesRecursive(o.Outlines, result)
		}
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd C:/zone/work/aigc/Yan-Studio/blogwatcher && go test ./internal/opml/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/opml/opml.go internal/opml/opml_test.go
git commit -m "feat(opml): add ExtractOutlines for nested outline handling

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 3: Import Logic in Controller Layer (TDD)

**Files:**
- Create: `internal/controller/import.go`
- Create: `internal/controller/import_test.go`

- [ ] **Step 1: Write failing test for ImportBlogs success case**

```go
// internal/controller/import_test.go
package controller

import (
	"path/filepath"
	"testing"

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

	// Verify blogs are in database
	blog, err := db.GetBlogByName("Blog 1")
	if err != nil {
		t.Fatal(err)
	}
	if blog == nil {
		t.Error("Blog 1 not found in database")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd C:/zone/work/aigc/Yan-Studio/blogwatcher && go test ./internal/controller/... -v`
Expected: FAIL - undefined: ImportBlogs

- [ ] **Step 3: Write minimal implementation**

```go
// internal/controller/import.go
package controller

import (
	"net/url"
	"strings"

	"github.com/Hyaxia/blogwatcher/internal/opml"
	"github.com/Hyaxia/blogwatcher/internal/storage"
)

type ImportedBlog struct {
	Name string
}

type SkippedBlog struct {
	Name   string
	Reason string
}

type FailedBlog struct {
	Name   string
	Reason string
}

type ImportResult struct {
	Imported []ImportedBlog
	Skipped  []SkippedBlog
	Failed   []FailedBlog
}

// ImportBlogs imports blogs from OPML outlines into the database.
func ImportBlogs(db *storage.Database, outlines []opml.Outline) ImportResult {
	result := ImportResult{
		Imported: []ImportedBlog{},
		Skipped:  []SkippedBlog{},
		Failed:   []FailedBlog{},
	}

	seenNames := make(map[string]bool)
	seenURLs := make(map[string]bool)

	for _, o := range outlines {
		if o.XMLURL == "" {
			result.Failed = append(result.Failed, FailedBlog{
				Name:   resolveName(o),
				Reason: "missing feed URL",
			})
			continue
		}

		name := resolveName(o)
		blogURL := resolveURL(o)

		if seenNames[name] {
			result.Skipped = append(result.Skipped, SkippedBlog{
				Name:   name,
				Reason: "duplicate within OPML file (name already imported)",
			})
			continue
		}
		if seenURLs[blogURL] {
			result.Skipped = append(result.Skipped, SkippedBlog{
				Name:   name,
				Reason: "duplicate within OPML file (URL already imported)",
			})
			continue
		}

		_, err := AddBlog(db, name, blogURL, o.XMLURL, "")
		if err != nil {
			if existsErr, ok := err.(BlogAlreadyExistsError); ok {
				result.Skipped = append(result.Skipped, SkippedBlog{
					Name:   name,
					Reason: "blog with " + existsErr.Field + " '" + existsErr.Value + "' already exists",
				})
				continue
			}
			result.Failed = append(result.Failed, FailedBlog{
				Name:   name,
				Reason: err.Error(),
			})
			continue
		}

		seenNames[name] = true
		seenURLs[blogURL] = true
		result.Imported = append(result.Imported, ImportedBlog{Name: name})
	}

	return result
}

func resolveName(o opml.Outline) string {
	if o.Title != "" {
		return o.Title
	}
	if o.Text != "" {
		return o.Text
	}
	if o.XMLURL != "" {
		return deriveDomain(o.XMLURL)
	}
	return ""
}

func resolveURL(o opml.Outline) string {
	if o.HTMLURL != "" {
		return o.HTMLURL
	}
	return o.XMLURL
}

func deriveDomain(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	host := parsed.Host
	host = strings.TrimPrefix(host, "www.")
	return host
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd C:/zone/work/aigc/Yan-Studio/blogwatcher && go test ./internal/controller/... -v`
Expected: PASS

- [ ] **Step 5: Write failing test for duplicate in database**

```go
// Add to internal/controller/import_test.go

func TestImportBlogs_DuplicateInDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := storage.OpenDatabase(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = AddBlog(db, "Existing Blog", "https://existing.com", "https://existing.com/feed", "")
	if err != nil {
		t.Fatal(err)
	}

	outlines := []opml.Outline{
		{Title: "Existing Blog", HTMLURL: "https://existing.com", XMLURL: "https://existing.com/feed"},
	}

	result := ImportBlogs(db, outlines)

	if len(result.Imported) != 0 {
		t.Errorf("expected 0 imported, got %d", len(result.Imported))
	}
	if len(result.Skipped) != 1 {
		t.Errorf("expected 1 skipped, got %d", len(result.Skipped))
	}
}
```

- [ ] **Step 6: Run test to verify it passes**

Run: `cd C:/zone/work/aigc/Yan-Studio/blogwatcher && go test ./internal/controller/... -v`
Expected: PASS

- [ ] **Step 7: Write tests for remaining edge cases**

```go
// Add to internal/controller/import_test.go

func TestImportBlogs_DuplicateInOPML(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := storage.OpenDatabase(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	outlines := []opml.Outline{
		{Title: "Blog 1", HTMLURL: "https://blog1.com", XMLURL: "https://blog1.com/feed"},
		{Title: "Blog 1", HTMLURL: "https://blog1.com", XMLURL: "https://blog1.com/feed"},
	}

	result := ImportBlogs(db, outlines)

	if len(result.Imported) != 1 {
		t.Errorf("expected 1 imported, got %d", len(result.Imported))
	}
	if len(result.Skipped) != 1 {
		t.Errorf("expected 1 skipped, got %d", len(result.Skipped))
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
		{Title: "No Feed", HTMLURL: "https://nofeed.com"},
	}

	result := ImportBlogs(db, outlines)

	if len(result.Failed) != 1 {
		t.Errorf("expected 1 failed, got %d", len(result.Failed))
	}
	if result.Failed[0].Reason != "missing feed URL" {
		t.Errorf("expected 'missing feed URL', got %q", result.Failed[0].Reason)
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

	outlines := []opml.Outline{
		{Text: "Text Name", HTMLURL: "https://text.com", XMLURL: "https://text.com/feed"},
	}

	result := ImportBlogs(db, outlines)

	if len(result.Imported) != 1 {
		t.Errorf("expected 1 imported, got %d", len(result.Imported))
	}
	if result.Imported[0].Name != "Text Name" {
		t.Errorf("expected name 'Text Name', got %q", result.Imported[0].Name)
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

	outlines := []opml.Outline{
		{Title: "No HTML URL", XMLURL: "https://nohtml.com/feed"},
	}

	result := ImportBlogs(db, outlines)

	if len(result.Imported) != 1 {
		t.Errorf("expected 1 imported, got %d", len(result.Imported))
	}

	blog, _ := db.GetBlogByName("No HTML URL")
	if blog == nil {
		t.Fatal("blog not found")
	}
	if blog.URL != "https://nohtml.com/feed" {
		t.Errorf("expected URL to be xmlUrl, got %q", blog.URL)
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

	outlines := []opml.Outline{
		{XMLURL: "https://blog.example.com/feed"},
	}

	result := ImportBlogs(db, outlines)

	if len(result.Imported) != 1 {
		t.Errorf("expected 1 imported, got %d", len(result.Imported))
	}
	if result.Imported[0].Name != "blog.example.com" {
		t.Errorf("expected name 'blog.example.com', got %q", result.Imported[0].Name)
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

	outlines := []opml.Outline{
		{Title: "", Text: "Text Value", XMLURL: "https://example.com/feed"},
	}

	result := ImportBlogs(db, outlines)

	if len(result.Imported) != 1 {
		t.Errorf("expected 1 imported, got %d", len(result.Imported))
	}
	if result.Imported[0].Name != "Text Value" {
		t.Errorf("expected name 'Text Value', got %q", result.Imported[0].Name)
	}
}
```

- [ ] **Step 8: Run all tests to verify they pass**

Run: `cd C:/zone/work/aigc/Yan-Studio/blogwatcher && go test ./internal/controller/... -v`
Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add internal/controller/import.go internal/controller/import_test.go
git commit -m "feat(controller): add ImportBlogs logic with TDD

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 4: CLI Import Command

**Files:**
- Modify: `internal/cli/commands.go`
- Modify: `internal/cli/root.go`

- [ ] **Step 1: Add newImportCommand to commands.go**

```go
// Add to internal/cli/commands.go (after newUnreadCommand function)

func newImportCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import <file.opml>",
		Short: "Import blogs from an OPML file.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filePath := args[0]

			// Parse OPML file
			opmlDoc, err := opml.ParseFile(filePath)
			if err != nil {
				printError(err)
				return markError(err)
			}

			// Extract all outlines (handles nested categories)
			outlines := opml.ExtractOutlines(opmlDoc)

			if len(outlines) == 0 {
				color.New(color.FgYellow).Println("No feeds found in OPML file.")
				return nil
			}

			db, err := storage.OpenDatabase("")
			if err != nil {
				return err
			}
			defer db.Close()

			color.New(color.FgCyan).Printf("Importing from %s...\n\n", filePath)

			result := controller.ImportBlogs(db, outlines)

			// Print results
			for _, imported := range result.Imported {
				color.New(color.FgGreen).Printf("✓ Imported: %q\n", imported.Name)
			}
			for _, skipped := range result.Skipped {
				color.New(color.FgYellow).Printf("⚠ Skipped %q: %s\n", skipped.Name, skipped.Reason)
			}
			for _, failed := range result.Failed {
				name := failed.Name
				if name == "" {
					name = "(unknown)"
				}
				color.New(color.FgRed).Printf("✗ Failed: %q - %s\n", name, failed.Reason)
			}

			// Print summary
			fmt.Println()
			color.New(color.FgCyan, color.Bold).Printf(
				"Import complete: %d imported, %d skipped, %d failed\n",
				len(result.Imported),
				len(result.Skipped),
				len(result.Failed),
			)

			return nil
		},
	}
	return cmd
}
```

- [ ] **Step 2: Add import for opml package**

Add `"github.com/Hyaxia/blogwatcher/internal/opml"` to the imports in `internal/cli/commands.go`.

- [ ] **Step 3: Register command in root.go**

```go
// In internal/cli/root.go, add this line after newUnreadCommand():
rootCmd.AddCommand(newImportCommand())
```

- [ ] **Step 4: Run tests to verify it compiles**

Run: `cd C:/zone/work/aigc/Yan-Studio/blogwatcher && go build ./...`
Expected: PASS (no errors)

- [ ] **Step 5: Commit**

```bash
git add internal/cli/commands.go internal/cli/root.go
git commit -m "feat(cli): add import command for OPML files

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 5: Run All Tests and Final Verification

- [ ] **Step 1: Run all tests**

Run: `cd C:/zone/work/aigc/Yan-Studio/blogwatcher && go test ./... -v`
Expected: All tests PASS

- [ ] **Step 2: Build the binary**

Run: `cd C:/zone/work/aigc/Yan-Studio/blogwatcher && go build ./cmd/blogwatcher`
Expected: Binary builds successfully

- [ ] **Step 3: Test the command manually**

Run: `./blogwatcher import --help`
Expected: Shows help for import command

- [ ] **Step 4: Final commit (if any changes)**

```bash
git status
# If any uncommitted changes:
git add -A
git commit -m "chore: final cleanup for OPML import feature

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Summary

| Task | Files Created | Files Modified |
|------|---------------|----------------|
| 1. OPML ParseFile | `internal/opml/opml.go`, `internal/opml/opml_test.go` | - |
| 2. ExtractOutlines | - | `internal/opml/opml.go`, `internal/opml/opml_test.go` |
| 3. ImportBlogs (TDD) | `internal/controller/import.go`, `internal/controller/import_test.go` | - |
| 4. CLI Command | - | `internal/cli/commands.go`, `internal/cli/root.go` |
| 5. Verification | - | - |