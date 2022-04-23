package sinking_yachts

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGenerateVariants(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Single",
			input:    "example.com",
			expected: []string{"example.com"},
		},
		{
			name:     "Multiples",
			input:    "foo.bar.example.com",
			expected: []string{"foo.bar.example.com", "bar.example.com", "example.com"},
		},
		{
			name:     "Consecutive Dots",
			input:    "foo.bar..example.com",
			expected: []string{"foo.bar..example.com", "bar..example.com", ".example.com", "example.com"},
		},
		{
			name:     "No TLD",
			input:    "foo",
			expected: nil,
		},
		{
			name:     "Empty",
			input:    "",
			expected: nil,
		},
		{
			name:     "Trailing slash",
			input:    "foo.example.com/",
			expected: []string{"foo.example.com/", "example.com/"},
		},
	}
	for _, data := range tests {
		t.Run(data.name, func(t *testing.T) {
			a := assert.New(t)
			result := generateVariants(data.input)
			a.Equal(data.expected, result)
		})
	}
}
