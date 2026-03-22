# OPML Import Feature Design

**Date:** 2026-03-22
**Status:** Draft

## Overview

Add an `import` command to BlogWatcher that allows users to import blogs from OPML files, enabling bulk subscription to feeds exported from other RSS readers.

## Command Interface

```
blogwatcher import <file.opml>
```

**Arguments:**
- `file.opml` — Path to an OPML file (required)

**Output example:**
```
Importing from feeds.opml...

✓ Imported: "TechCrunch"
✓ Imported: "Hacker News"
⚠ Skipped "My Blog": blog with name 'My Blog' already exists
⚠ Skipped "Other Blog": blog with URL 'https://other.com' already exists
✗ Failed: "Broken Feed" - missing feed URL

Import complete: 2 imported, 2 skipped, 1 failed
```

## OPML Field Mapping

| OPML Attribute | BlogWatcher Field | Required | Fallback |
|----------------|-------------------|----------|----------|
| `title` or `text` | `name` | Yes | Use `text` if `title` missing; derive from domain if both missing |
| `htmlUrl` | `url` | Yes | Use `xmlUrl` as fallback (database requires non-null URL) |
| `xmlUrl` | `feed_url` | Yes | None - skip entry if missing |

**Name resolution priority:**
1. `title` attribute (if present and non-empty)
2. `text` attribute (if present and non-empty)
3. Derive from `xmlUrl` domain (e.g., `https://blog.example.com/feed.xml` → `blog.example.com`)

**URL resolution:**
- Use `htmlUrl` if present
- Otherwise use `xmlUrl` as the blog URL (required for database NOT NULL constraint)

## Duplicate Handling

**External duplicates (already in database):**
- When a blog already exists by name: `⚠ Skipped "X": blog with name 'X' already exists`
- When a blog already exists by URL: `⚠ Skipped "X": blog with URL 'Y' already exists`
- Use existing `BlogAlreadyExistsError.Field` to determine which conflict occurred

**Internal duplicates (within same OPML file):**
- Track imported names and URLs during processing
- If duplicate within file: `⚠ Skipped "X": duplicate within OPML file (name already imported)`
- Continue processing remaining feeds

## Code Structure

### New Files

**`internal/opml/opml.go`**
- Parse OPML XML files
- Define OPML struct with Outline nested struct
- Handle nested outlines (categories with nested feeds)
- `ParseFile(path string) (*OPML, error)` - read and parse OPML file
- `ExtractOutlines(opml *OPML) []Outline` - recursively extract all outlines

**`internal/opml/opml_test.go`**
- Unit tests for OPML parsing

**`internal/controller/import.go`**
- `ImportResult` struct with Imported, Skipped, Failed lists
- `ImportBlogs(db *storage.Database, outlines []opml.Outline) ImportResult`

### Modified Files

**`internal/cli/commands.go`**
- Add `newImportCommand()` function (thin CLI handler)

**`internal/cli/root.go`**
- Register the new `import` command

### OPML Struct Definition

```go
type OPML struct {
    XMLName xml.Name `xml:"opml"`
    Body    struct {
        Outlines []Outline `xml:"outline"`
    } `xml:"body"`
}

type Outline struct {
    Text    string `xml:"text,attr"`    // Required by OPML spec
    Title   string `xml:"title,attr"`   // Optional
    HTMLURL string `xml:"htmlUrl,attr"` // Blog homepage
    XMLURL  string `xml:"xmlUrl,attr"`  // Feed URL
    Outlines []Outline `xml:"outline"`   // Nested outlines for categories
}
```

### Controller Types

```go
type ImportedBlog struct {
    Name string
}

type SkippedBlog struct {
    Name   string
    Reason string // "name exists", "url exists", "duplicate in file"
}

type FailedBlog struct {
    Name   string // May be empty if name couldn't be determined
    Reason string
}

type ImportResult struct {
    Imported []ImportedBlog
    Skipped  []SkippedBlog
    Failed   []FailedBlog
}

func ImportBlogs(db *storage.Database, outlines []opml.Outline) ImportResult
```

## Import Flow

1. **CLI layer** (`commands.go`):
   - Parse file path argument
   - Call `opml.ParseFile(path)`
   - Call `opml.ExtractOutlines()` to get flat list
   - Call `controller.ImportBlogs(db, outlines)`
   - Format and print `ImportResult`

2. **Controller layer** (`controller/import.go`):
   - Initialize result struct and tracking maps (seen names, seen URLs)
   - For each outline:
     - Validate: skip if `xmlUrl` missing (Failed)
     - Resolve name: title → text → domain from xmlUrl
     - Resolve URL: htmlUrl → xmlUrl
     - Check internal duplicates: skip if name/URL seen in this import (Skipped)
     - Call `AddBlog()` - if `BlogAlreadyExistsError`, add to Skipped with specific reason
     - On success, add to Imported and tracking maps
   - Return ImportResult

**Empty string handling:** Empty attribute values (e.g., `title=""`) are treated as "not provided" and trigger fallback behavior.

## Error Handling

| Scenario | Behavior |
|----------|----------|
| File not found | Error and exit |
| Invalid XML | Error with parse details |
| Missing `xmlUrl` | Failed: "missing feed URL" |
| Missing `htmlUrl` (but `xmlUrl` exists) | Use `xmlUrl` as blog URL |
| Missing name attributes | Derive from domain; if that fails: Failed |
| Duplicate in database | Skipped with specific reason (name or URL conflict) |
| Duplicate in same OPML | Skipped: "duplicate within OPML file" |
| Empty OPML | Success message, 0 feeds found |

**Graceful processing:** Continue processing all feeds even if some fail. Report all issues at the end.

## Testing

### Unit Tests

- Parse valid OPML file with flat structure
- Parse OPML with nested outlines (categories)
- Handle missing `title` attribute (use `text`)
- Handle missing both `title` and `text` (derive from domain)
- Handle missing `htmlUrl` (use `xmlUrl` as fallback)
- Handle malformed XML
- Test `ExtractOutlines` with deeply nested structures

### Integration Tests

- Import sample OPML and verify database entries
- Verify duplicate detection works correctly
- Verify internal duplicate detection
- Verify summary output accuracy

## Dependencies

No new dependencies required. Use Go's built-in `encoding/xml` package for parsing.

## Future Considerations

These are out of scope for the initial implementation:
- Category/filter options for selective import
- Dry-run mode
- OPML export functionality