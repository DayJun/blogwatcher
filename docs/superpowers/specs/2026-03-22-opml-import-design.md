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
⚠ Skipped "My Blog": already exists
✗ Failed: "Broken Feed" - missing URL

Import complete: 2 imported, 1 skipped, 1 failed
```

## OPML Field Mapping

| OPML Attribute | BlogWatcher Field | Description |
|----------------|-------------------|-------------|
| `title` | `name` | Blog display name |
| `htmlUrl` | `url` | Blog homepage URL |
| `xmlUrl` | `feed_url` | RSS/Atom feed URL |

## Duplicate Handling

When a blog already exists (by name or URL), skip the import for that blog and show a warning message. Continue processing remaining feeds.

## Code Structure

### New Files

**`internal/opml/opml.go`**
- Parse OPML XML files
- Define OPML struct with Outline nested struct
- Handle nested outlines (categories with nested feeds)

**`internal/opml/opml_test.go`**
- Unit tests for OPML parsing

### Modified Files

**`internal/cli/commands.go`**
- Add `newImportCommand()` function
- Implement import logic with summary output

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
    Title   string `xml:"title,attr"`
    HTMLURL string `xml:"htmlUrl,attr"`
    XMLURL  string `xml:"xmlUrl,attr"`
    Outlines []Outline `xml:"outline"` // Nested outlines for categories
}
```

## Import Flow

1. Read OPML file from provided path
2. Parse XML into OPML struct
3. Recursively extract all outlines (handle nested categories)
4. For each outline with an `xmlUrl`:
   - Call existing `controller.AddBlog()` logic
   - Track success/skip/failure counts
5. Print summary with counts

## Error Handling

| Scenario | Behavior |
|----------|----------|
| File not found | Error and exit |
| Invalid XML | Error with parse details |
| Missing `xmlUrl` | Skip with warning |
| Duplicate blog | Skip with warning |
| Empty OPML | Success message, 0 feeds found |

**Graceful processing:** Continue processing all feeds even if some fail. Report all issues at the end.

## Testing

### Unit Tests

- Parse valid OPML file with flat structure
- Parse OPML with nested outlines (categories)
- Handle missing optional fields
- Handle malformed XML

### Integration Tests

- Import sample OPML and verify database entries
- Verify duplicate detection works correctly
- Verify summary output accuracy

## Dependencies

No new dependencies required. Use Go's built-in `encoding/xml` package for parsing.

## Future Considerations

These are out of scope for the initial implementation:
- Category/filter options for selective import
- Dry-run mode
- OPML export functionality