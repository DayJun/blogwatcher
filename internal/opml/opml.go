// Package opml provides OPML (Outline Processor Markup Language) parsing functionality
// for importing RSS/Atom feed subscriptions into BlogWatcher.
package opml

import (
	"encoding/xml"
	"os"
)

// OPML represents the root element of an OPML document containing feed subscriptions.
type OPML struct {
	XMLName xml.Name `xml:"opml"`
	Body    struct {
		Outlines []Outline `xml:"outline"`
	} `xml:"body"`
}

// Outline represents a single feed entry or category in an OPML document.
// It can contain nested Outlines for categories.
type Outline struct {
	Text     string    `xml:"text,attr"`
	Title    string    `xml:"title,attr"`
	HTMLURL  string    `xml:"htmlUrl,attr"`
	XMLURL   string    `xml:"xmlUrl,attr"`
	Outlines []Outline `xml:"outline"`
}

// ParseFile reads and parses an OPML file at the given path.
// Returns an error if the file cannot be read or contains invalid XML.
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

// ExtractOutlines recursively extracts all outlines with xmlUrl from an OPML document.
// It skips category outlines (those without xmlUrl) and only returns actual feed entries.
func ExtractOutlines(opml *OPML) []Outline {
	var result []Outline
	extractOutlinesRecursive(opml.Body.Outlines, &result)
	return result
}

func extractOutlinesRecursive(outlines []Outline, result *[]Outline) {
	for _, o := range outlines {
		if o.XMLURL != "" {
			*result = append(*result, o)
		}
		if len(o.Outlines) > 0 {
			extractOutlinesRecursive(o.Outlines, result)
		}
	}
}