package html

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ToPlainText extracts plain text from HTML content.
func ToPlainText(html string) string {
	if html == "" {
		return ""
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return html // fallback: return as-is
	}

	return doc.Text()
}