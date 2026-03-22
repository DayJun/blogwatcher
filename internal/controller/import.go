package controller

import (
	"net/url"
	"strings"

	"github.com/Hyaxia/blogwatcher/internal/opml"
	"github.com/Hyaxia/blogwatcher/internal/storage"
)

// ImportedBlog represents a blog that was successfully imported.
type ImportedBlog struct {
	Name string
	URL  string
}

// SkippedBlog represents a blog that was skipped because it already exists.
type SkippedBlog struct {
	Name   string
	Reason string
}

// FailedBlog represents a blog that failed to import.
type FailedBlog struct {
	Name   string
	URL    string
	Reason string
}

// ImportResult contains the results of an OPML import operation.
type ImportResult struct {
	Imported []ImportedBlog
	Skipped  []SkippedBlog
	Failed   []FailedBlog
}

// ImportBlogs imports blogs from OPML outlines into the database.
// It handles duplicates, missing data, and reports all results.
func ImportBlogs(db *storage.Database, outlines []opml.Outline) ImportResult {
	result := ImportResult{
		Imported: make([]ImportedBlog, 0),
		Skipped:  make([]SkippedBlog, 0),
		Failed:   make([]FailedBlog, 0),
	}

	seenNames := make(map[string]bool)
	seenURLs := make(map[string]bool)

	for _, o := range outlines {
		name := resolveName(o)
		blogURL := resolveURL(o)

		// Check for missing feed URL (xmlUrl) - this is required
		if o.XMLURL == "" {
			result.Failed = append(result.Failed, FailedBlog{
				Name:   name,
				URL:    blogURL,
				Reason: "missing feed URL",
			})
			continue
		}

		if name == "" {
			name = deriveDomain(blogURL)
		}

		// Check for duplicates within OPML by name
		if seenNames[name] {
			result.Skipped = append(result.Skipped, SkippedBlog{
				Name:   name,
				Reason: "duplicate within OPML file (name already imported)",
			})
			continue
		}
		// Check for duplicates within OPML by URL
		if seenURLs[blogURL] {
			result.Skipped = append(result.Skipped, SkippedBlog{
				Name:   name,
				Reason: "duplicate within OPML file (URL already imported)",
			})
			continue
		}
		seenNames[name] = true
		seenURLs[blogURL] = true

		// Use controller.AddBlog which handles duplicate checking
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
				URL:    blogURL,
				Reason: err.Error(),
			})
			continue
		}

		result.Imported = append(result.Imported, ImportedBlog{
			Name: name,
			URL:  blogURL,
		})
	}

	return result
}

// resolveName returns the name for an outline using title -> text -> empty fallback.
func resolveName(o opml.Outline) string {
	if o.Title != "" {
		return o.Title
	}
	if o.Text != "" {
		return o.Text
	}
	return ""
}

// resolveURL returns the URL for an outline using htmlUrl -> xmlUrl fallback.
func resolveURL(o opml.Outline) string {
	if o.HTMLURL != "" {
		return o.HTMLURL
	}
	return o.XMLURL
}

// deriveDomain extracts the domain from a URL.
func deriveDomain(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	host := parsed.Host
	// Remove www. prefix if present
	host = strings.TrimPrefix(host, "www.")
	return host
}