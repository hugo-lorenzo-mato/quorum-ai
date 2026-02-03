package issues

import (
	"log/slog"
	"regexp"
	"strings"
)

// IssueValidationResult contains the result of validating an issue.
type IssueValidationResult struct {
	// Valid indicates if the issue passes all validation checks.
	Valid bool

	// HasTitle indicates if the issue has a title (H1 heading).
	HasTitle bool

	// HasBody indicates if the issue has body content.
	HasBody bool

	// TitleLength is the length of the title in characters.
	TitleLength int

	// BodyLength is the length of the body in characters.
	BodyLength int

	// HasRequiredSections indicates if all required sections are present.
	HasRequiredSections bool

	// MissingSections lists sections that are required but missing.
	MissingSections []string

	// ContainsForbidden indicates if the content contains forbidden patterns.
	ContainsForbidden bool

	// ForbiddenMatches lists the forbidden patterns that were found.
	ForbiddenMatches []string

	// Warnings contains non-fatal validation warnings.
	Warnings []string
}

// IssueValidatorConfig configures the issue validator.
type IssueValidatorConfig struct {
	// MinTitleLength is the minimum allowed title length.
	MinTitleLength int

	// MaxTitleLength is the maximum allowed title length.
	MaxTitleLength int

	// MinBodyLength is the minimum allowed body length.
	MinBodyLength int

	// RequiredSections lists sections that must be present (e.g., "## Summary").
	RequiredSections []string

	// ForbiddenPatterns lists regex patterns that should not appear in issues.
	// These are used to detect LLM metadata leakage.
	ForbiddenPatterns []string

	// SanitizeForbidden enables automatic removal of forbidden content.
	SanitizeForbidden bool
}

// DefaultIssueValidatorConfig returns the default validator configuration.
func DefaultIssueValidatorConfig() IssueValidatorConfig {
	return IssueValidatorConfig{
		MinTitleLength: 5,
		MaxTitleLength: 200,
		MinBodyLength:  50,
		RequiredSections: []string{
			"## Summary",
		},
		ForbiddenPatterns: []string{
			// Model/agent names
			`(?i)\bclaude\b`,
			`(?i)\bgemini\b`,
			`(?i)\bgpt-?\d`,
			`(?i)\bopenai\b`,
			`(?i)\banthropic\b`,
			`(?i)\bllama\b`,
			`(?i)\bmistral\b`,
			// Generation metadata
			`(?i)\bmodel\s+version\b`,
			`(?i)\bgenerated\s+(?:by|at|on|with)\b`,
			`(?i)\bai-generated\b`,
			`(?i)\bthis\s+(?:issue\s+)?was\s+(?:auto-?)?generated\b`,
			// Timestamps in ISO format
			`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}`,
			// Internal IDs/workflow references
			`(?i)workflow[_-]?id\s*[:=]\s*\S+`,
			`(?i)internal[_-]?id\s*[:=]\s*\S+`,
		},
		SanitizeForbidden: true,
	}
}

// IssueValidator validates generated issue content.
type IssueValidator struct {
	config            IssueValidatorConfig
	forbiddenRegexes  []*regexp.Regexp
	requiredSectionRe []*regexp.Regexp
}

// NewIssueValidator creates a new issue validator with the given configuration.
func NewIssueValidator(cfg IssueValidatorConfig) *IssueValidator {
	v := &IssueValidator{
		config: cfg,
	}

	// Compile forbidden patterns
	for _, pattern := range cfg.ForbiddenPatterns {
		if re, err := regexp.Compile(pattern); err == nil {
			v.forbiddenRegexes = append(v.forbiddenRegexes, re)
		} else {
			slog.Warn("invalid forbidden pattern", "pattern", pattern, "error", err)
		}
	}

	// Compile required section patterns (case-insensitive)
	for _, section := range cfg.RequiredSections {
		// Escape special chars and make case-insensitive
		escaped := regexp.QuoteMeta(section)
		pattern := `(?i)` + escaped
		if re, err := regexp.Compile(pattern); err == nil {
			v.requiredSectionRe = append(v.requiredSectionRe, re)
		}
	}

	return v
}

// NewDefaultIssueValidator creates a validator with default configuration.
func NewDefaultIssueValidator() *IssueValidator {
	return NewIssueValidator(DefaultIssueValidatorConfig())
}

// Validate checks if the issue content is valid.
func (v *IssueValidator) Validate(content string) IssueValidationResult {
	result := IssueValidationResult{
		Valid: true,
	}

	// Parse title and body
	title, body := parseIssueMarkdown(content)
	result.TitleLength = len(title)
	result.BodyLength = len(body)
	result.HasTitle = title != "" && title != "Untitled Issue"
	result.HasBody = len(strings.TrimSpace(body)) > 0

	// Validate title length
	if result.TitleLength < v.config.MinTitleLength {
		result.Valid = false
		result.Warnings = append(result.Warnings,
			"title too short: minimum "+string(rune(v.config.MinTitleLength))+" characters required")
	}
	if result.TitleLength > v.config.MaxTitleLength {
		result.Valid = false
		result.Warnings = append(result.Warnings,
			"title too long: maximum "+string(rune(v.config.MaxTitleLength))+" characters allowed")
	}

	// Validate body length
	if result.BodyLength < v.config.MinBodyLength {
		result.Warnings = append(result.Warnings,
			"body is short: consider adding more detail")
	}

	// Check required sections
	result.HasRequiredSections = true
	for i, re := range v.requiredSectionRe {
		if !re.MatchString(content) {
			result.HasRequiredSections = false
			result.MissingSections = append(result.MissingSections, v.config.RequiredSections[i])
		}
	}
	if !result.HasRequiredSections {
		result.Warnings = append(result.Warnings,
			"missing required sections: "+strings.Join(result.MissingSections, ", "))
	}

	// Check for forbidden patterns
	for _, re := range v.forbiddenRegexes {
		if matches := re.FindAllString(content, -1); len(matches) > 0 {
			result.ContainsForbidden = true
			result.ForbiddenMatches = append(result.ForbiddenMatches, matches...)
		}
	}
	if result.ContainsForbidden {
		result.Warnings = append(result.Warnings,
			"contains forbidden content that should be removed: "+strings.Join(result.ForbiddenMatches, ", "))
	}

	// Final validation: must have title and body
	if !result.HasTitle || !result.HasBody {
		result.Valid = false
	}

	return result
}

// ValidateAll validates multiple issue files and returns results for each.
func (v *IssueValidator) ValidateAll(contents []string) []IssueValidationResult {
	results := make([]IssueValidationResult, len(contents))
	for i, content := range contents {
		results[i] = v.Validate(content)
	}
	return results
}

// SanitizeForbidden removes forbidden patterns from the content.
// Returns the sanitized content and a list of patterns that were removed.
func (v *IssueValidator) SanitizeForbidden(content string) (string, []string) {
	var removed []string
	sanitized := content

	for _, re := range v.forbiddenRegexes {
		matches := re.FindAllString(sanitized, -1)
		if len(matches) > 0 {
			removed = append(removed, matches...)
			// Remove the matches (replace with empty string or placeholder)
			sanitized = re.ReplaceAllString(sanitized, "")
		}
	}

	// Clean up any resulting double spaces or empty lines
	sanitized = cleanupWhitespace(sanitized)

	return sanitized, removed
}

// SanitizeAndValidate sanitizes the content (if enabled) and then validates it.
func (v *IssueValidator) SanitizeAndValidate(content string) (string, IssueValidationResult) {
	sanitized := content

	if v.config.SanitizeForbidden {
		var removed []string
		sanitized, removed = v.SanitizeForbidden(content)
		if len(removed) > 0 {
			slog.Info("sanitized forbidden content from issue",
				"removed_count", len(removed),
				"removed", removed)
		}
	}

	result := v.Validate(sanitized)
	return sanitized, result
}

// cleanupWhitespace removes excessive whitespace from content.
func cleanupWhitespace(content string) string {
	// Replace multiple consecutive empty lines with double newline
	multipleNewlines := regexp.MustCompile(`\n{3,}`)
	content = multipleNewlines.ReplaceAllString(content, "\n\n")

	// Replace multiple consecutive spaces with single space
	multipleSpaces := regexp.MustCompile(`[ \t]{2,}`)
	content = multipleSpaces.ReplaceAllString(content, " ")

	// Remove trailing whitespace from lines
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}

	return strings.Join(lines, "\n")
}

// IsValid is a convenience method to quickly check if content is valid.
func (v *IssueValidator) IsValid(content string) bool {
	return v.Validate(content).Valid
}

// MustBeValid returns an error if the content is not valid.
func (v *IssueValidator) MustBeValid(content string) error {
	result := v.Validate(content)
	if !result.Valid {
		return &ValidationError{
			Warnings: result.Warnings,
		}
	}
	return nil
}

// ValidationError represents a validation failure.
type ValidationError struct {
	Warnings []string
}

func (e *ValidationError) Error() string {
	return "issue validation failed: " + strings.Join(e.Warnings, "; ")
}
