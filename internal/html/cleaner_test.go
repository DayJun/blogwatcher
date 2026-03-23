package html

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToPlainText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple paragraph",
			input:    "<p>Hello World</p>",
			expected: "Hello World",
		},
		{
			name:     "multiple paragraphs",
			input:    "<p>First</p><p>Second</p>",
			expected: "First",
		},
		{
			name:     "with links",
			input:    `<p>Check <a href="https://example.com">this link</a></p>`,
			expected: "Check this link",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "plain text",
			input:    "Just plain text",
			expected: "Just plain text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToPlainText(tt.input)
			assert.Contains(t, result, tt.expected)
		})
	}
}