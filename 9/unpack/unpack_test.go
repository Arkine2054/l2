package unpack_test

import (
	"gitlab.com/arkine/l2/9/unpack"
	"testing"
)

func TestUnpack(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		wantErr  bool
	}{
		{"a4bc2d5e", "aaaabccddddde", false},
		{"abcd", "abcd", false},
		{"45", "", true},
		{"", "", false},
		{"qwe\\4\\5", "qwe45", false},
		{"qwe\\45", "qwe44444", false},
		{"\\", "", true},
		{"a\\", "", true},
		{"\\3", "3", false},
	}

	for _, tt := range tests {
		res, err := unpack.Unpack(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("Unpack(%q) unexpected error status: got %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if res != tt.expected {
			t.Errorf("Unpack(%q) = %q; want %q", tt.input, res, tt.expected)
		}
	}
}
