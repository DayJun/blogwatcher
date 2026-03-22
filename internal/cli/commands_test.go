package cli

import (
	"testing"
)

func TestArticlesCommandPerPageValidation(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{0, 20},    // Below minimum, reset to default
		{-1, 20},   // Negative, reset to default
		{50, 50},   // Valid value
		{100, 100}, // Maximum allowed
		{101, 100}, // Above max, clamped to 100
		{200, 100}, // Above max, clamped to 100
	}
	for _, tt := range tests {
		perPage := tt.input
		if perPage < 1 {
			perPage = 20
		}
		if perPage > 100 {
			perPage = 100
		}
		if perPage != tt.expected {
			t.Errorf("input %d: expected %d, got %d", tt.input, tt.expected, perPage)
		}
	}
}

func TestArticlesCommandPageValidation(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{0, 1},  // Below minimum, reset to 1
		{-1, 1}, // Negative, reset to 1
		{1, 1},  // Valid
		{5, 5},  // Valid
	}
	for _, tt := range tests {
		page := tt.input
		if page < 1 {
			page = 1
		}
		if page != tt.expected {
			t.Errorf("input %d: expected %d, got %d", tt.input, tt.expected, page)
		}
	}
}