package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/Hyaxia/blogwatcher/internal/model"
)

// ArticleFieldValues maps field names to display values for an article.
func ArticleFieldValues(article *model.Article, blogNames map[int64]string) map[string]string {
	values := map[string]string{
		"id":           fmt.Sprintf("%d", article.ID),
		"title":        article.Title,
		"url":          article.URL,
		"blog":         blogNames[article.BlogID],
		"published":    formatTimePtr(article.PublishedDate),
		"discovered":   formatTimePtr(article.DiscoveredDate),
		"read":         formatBool(article.IsRead),
		"content":      article.Content,
		"description":  article.Description,
		"feed_summary": article.FeedSummary,
		"summary":      article.Summary,
	}
	return values
}

// FormatArticleFields formats article fields as pipe-separated single line.
func FormatArticleFields(article *model.Article, blogNames map[int64]string, fields []string) string {
	values := ArticleFieldValues(article, blogNames)
	parts := make([]string, len(fields))
	for i, field := range fields {
		parts[i] = values[field]
	}
	return strings.Join(parts, " | ")
}

// FormatArticleDetail formats article as key-value pairs (multi-line).
func FormatArticleDetail(article *model.Article, blogNames map[int64]string, fields []string) string {
	values := ArticleFieldValues(article, blogNames)
	var lines []string
	for _, field := range fields {
		value := values[field]
		if field == "content" || field == "description" || field == "feed_summary" || field == "summary" {
			lines = append(lines, fmt.Sprintf("%s:\n  %s", fieldTitle(field), value))
		} else {
			lines = append(lines, fmt.Sprintf("%s: %s", fieldTitle(field), value))
		}
	}
	return strings.Join(lines, "\n")
}

// fieldTitle converts a field name to a display title.
func fieldTitle(field string) string {
	titles := map[string]string{
		"id":           "ID",
		"url":          "URL",
		"blog":         "Blog",
		"title":        "Title",
		"published":    "Published",
		"discovered":   "Discovered",
		"read":         "Read",
		"content":      "Content",
		"description":  "Description",
		"feed_summary": "Feed Summary",
		"summary":      "Summary",
	}
	if title, ok := titles[field]; ok {
		return title
	}
	return field
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02")
}

func formatBool(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}