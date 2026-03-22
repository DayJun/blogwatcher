package controller

import (
	"net/url"

	"github.com/Hyaxia/blogwatcher/internal/model"
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
	Name string
	URL  string
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

	seen := make(map[string]bool)

	for _, o := range outlines {
		name := resolveName(o)
		blogURL := resolveURL(o)
		feedURL := o.XMLURL

		if blogURL == "" {
			result.Failed = append(result.Failed, FailedBlog{
				Name:   name,
				URL:    blogURL,
				Reason: "missing URL",
			})
			continue
		}

		if name == "" {
			name = deriveDomain(blogURL)
		}

		// Check for duplicates within OPML
		if seen[blogURL] {
			result.Skipped = append(result.Skipped, SkippedBlog{
				Name: name,
				URL:  blogURL,
			})
			continue
		}
		seen[blogURL] = true

		// Check if blog already exists in database
		existing, err := db.GetBlogByURL(blogURL)
		if err != nil {
			result.Failed = append(result.Failed, FailedBlog{
				Name:   name,
				URL:    blogURL,
				Reason: "database error: " + err.Error(),
			})
			continue
		}
		if existing != nil {
			result.Skipped = append(result.Skipped, SkippedBlog{
				Name: name,
				URL:  blogURL,
			})
			continue
		}

		// Add the blog
		_, err = db.AddBlog(model.Blog{
			Name:    name,
			URL:     blogURL,
			FeedURL: feedURL,
		})
		if err != nil {
			result.Failed = append(result.Failed, FailedBlog{
				Name:   name,
				URL:    blogURL,
				Reason: "failed to add: " + err.Error(),
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
	return parsed.Host
}