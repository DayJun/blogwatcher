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