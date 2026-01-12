package logging

import (
	"regexp"
)

// Sanitizer redacts sensitive information from log messages.
type Sanitizer struct {
	patterns []*regexp.Regexp
	redacted string
}

// NewSanitizer creates a sanitizer with default patterns.
func NewSanitizer() *Sanitizer {
	return &Sanitizer{
		patterns: defaultPatterns(),
		redacted: "[REDACTED]",
	}
}

func defaultPatterns() []*regexp.Regexp {
	patterns := []string{
		// OpenAI
		`sk-[A-Za-z0-9]{20,}`,
		// Anthropic
		`sk-ant-[a-zA-Z0-9-]{40,}`,
		// Google AI
		`AIza[a-zA-Z0-9_-]{35}`,
		// GitHub PAT
		`ghp_[A-Za-z0-9]{36}`,
		// GitHub OAuth
		`gho_[A-Za-z0-9]{36}`,
		// GitHub App
		`ghu_[A-Za-z0-9]{36}`,
		`ghs_[A-Za-z0-9]{36}`,
		// AWS Access Key
		`AKIA[0-9A-Z]{16}`,
		// AWS Secret Key (looser pattern)
		`(?i)aws[_-]?secret[_-]?access[_-]?key["'\s:=]+[A-Za-z0-9/+=]{40}`,
		// Slack tokens
		`xox[baprs]-[0-9a-zA-Z-]{10,}`,
		// Generic Bearer tokens
		`(?i)bearer\s+[a-zA-Z0-9._-]{20,}`,
		// Generic API keys
		`(?i)api[_-]?key["'\s:=]+[a-zA-Z0-9_-]{20,}`,
		// Generic secrets
		`(?i)secret["'\s:=]+[a-zA-Z0-9_-]{20,}`,
		// Generic passwords
		`(?i)password["'\s:=]+[^\s"']{8,}`,
		// Generic tokens
		`(?i)token["'\s:=]+[a-zA-Z0-9_-]{20,}`,
	}

	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		compiled = append(compiled, regexp.MustCompile(p))
	}
	return compiled
}

// Sanitize redacts sensitive information from a string.
func (s *Sanitizer) Sanitize(input string) string {
	result := input
	for _, pattern := range s.patterns {
		result = pattern.ReplaceAllString(result, s.redacted)
	}
	return result
}

// SanitizeMap redacts values in a map.
func (s *Sanitizer) SanitizeMap(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		switch val := v.(type) {
		case string:
			result[k] = s.Sanitize(val)
		case map[string]interface{}:
			result[k] = s.SanitizeMap(val)
		default:
			result[k] = v
		}
	}
	return result
}

// AddPattern adds a custom pattern.
func (s *Sanitizer) AddPattern(pattern string) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	s.patterns = append(s.patterns, re)
	return nil
}

// SetRedactedPlaceholder sets the placeholder text for redacted content.
func (s *Sanitizer) SetRedactedPlaceholder(placeholder string) {
	s.redacted = placeholder
}
