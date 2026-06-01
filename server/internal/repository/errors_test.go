package repository

import (
	"errors"
	"testing"

	"gorm.io/gorm"
)

func TestMapError(t *testing.T) {
	tests := []struct {
		name     string
		input    error
		expected error
	}{
		{
			name:     "nil error",
			input:    nil,
			expected: nil,
		},
		{
			name:     "gorm record not found",
			input:    gorm.ErrRecordNotFound,
			expected: ErrNotFound,
		},
		{
			name:     "wrapped gorm record not found",
			input:    errors.Join(gorm.ErrRecordNotFound),
			expected: ErrNotFound,
		},
		{
			name:     "other gorm error",
			input:    gorm.ErrInvalidDB,
			expected: gorm.ErrInvalidDB,
		},
		{
			name:     "random error",
			input:    errors.New("something went wrong"),
			expected: errors.New("something went wrong"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapError(tt.input)
			if got == nil {
				if tt.expected != nil {
					t.Fatalf("expected error %v, got nil", tt.expected)
				}
				return
			}
			if tt.expected == nil {
				t.Fatalf("expected nil, got error %v", got)
			}
			if !errors.Is(got, tt.expected) && got.Error() != tt.expected.Error() {
				t.Errorf("expected error %v, got %v", tt.expected, got)
			}
		})
	}
}
