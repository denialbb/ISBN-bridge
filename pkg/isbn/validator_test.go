package isbn

import (
	"testing"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty", "", ""},
		{"with spaces and hyphens", " 978-0-306-40615-7 ", "9780306406157"},
		{"isbn 10 lowercase x", "0-19-852663-x", "019852663X"},
		{"newlines and tabs", "\r\n9780241316757\t", "9780241316757"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Normalize(tt.input)
			if got != tt.expected {
				t.Errorf("Normalize(%q) = %q; want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectError bool
	}{
		{"valid ISBN-13 standard", "9780306406157", "9780306406157", false},
		{"valid ISBN-13 with hyphens", "978-0-241-31675-7", "9780241316757", false},
		{"valid ISBN-10 with digit check", "0306406152", "0306406152", false},
		{"valid ISBN-10 with X check", "080442957X", "080442957X", false},
		{"valid ISBN-10 lowercase x", "0-8044-2957-x", "080442957X", false},
		{"invalid checksum ISBN-13", "9780306406158", "", true},
		{"invalid checksum ISBN-10", "0306406153", "", true},
		{"too short", "12345", "", true},
		{"too long", "9780306406157999", "", true},
		{"non-numeric characters", "978030640615A", "", true},
		{"empty string", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Validate(tt.input)
			if tt.expectError {
				if err == nil {
					t.Errorf("Validate(%q) expected error, got nil", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("Validate(%q) unexpected error: %v", tt.input, err)
				}
				if got != tt.expected {
					t.Errorf("Validate(%q) = %q; want %q", tt.input, got, tt.expected)
				}
			}
		})
	}
}
