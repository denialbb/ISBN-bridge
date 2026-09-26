package isbn

import (
	"errors"
	"regexp"
	"strings"
)

var (
	ErrInvalidLength   = errors.New("invalid ISBN length")
	ErrInvalidChecksum = errors.New("invalid ISBN checksum")
	ErrInvalidFormat   = errors.New("invalid characters in ISBN")
)

var whitespaceHyphenRegex = regexp.MustCompile(`[\s-]+`)

// Normalize strips spaces and hyphens, and converts check digit 'x' to uppercase 'X'.
func Normalize(raw string) string {
	cleaned := whitespaceHyphenRegex.ReplaceAllString(raw, "")
	return strings.ToUpper(strings.TrimSpace(cleaned))
}

// Validate normalizes and verifies the checksum of an ISBN-10 or ISBN-13.
func Validate(raw string) (string, error) {
	s := Normalize(raw)

	switch len(s) {
	case 13:
		if err := validateISBN13(s); err != nil {
			return "", err
		}
		return s, nil
	case 10:
		if err := validateISBN10(s); err != nil {
			return "", err
		}
		return s, nil
	default:
		return "", ErrInvalidLength
	}
}

func validateISBN13(s string) error {
	sum := 0
	for i := 0; i < 13; i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return ErrInvalidFormat
		}
		digit := int(c - '0')
		if i%2 == 0 {
			sum += digit
		} else {
			sum += digit * 3
		}
	}
	if sum%10 != 0 {
		return ErrInvalidChecksum
	}
	return nil
}

func validateISBN10(s string) error {
	sum := 0
	for i := 0; i < 9; i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return ErrInvalidFormat
		}
		digit := int(c - '0')
		sum += digit * (10 - i)
	}

	lastChar := s[9]
	var checkVal int
	if lastChar == 'X' {
		checkVal = 10
	} else if lastChar >= '0' && lastChar <= '9' {
		checkVal = int(lastChar - '0')
	} else {
		return ErrInvalidFormat
	}

	sum += checkVal
	if sum%11 != 0 {
		return ErrInvalidChecksum
	}
	return nil
}
