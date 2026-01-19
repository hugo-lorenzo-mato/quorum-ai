package report

import (
	"fmt"
	"sort"
	"strings"
)

// Frontmatter represents YAML frontmatter for markdown files
type Frontmatter struct {
	fields map[string]interface{}
	order  []string // Track insertion order
}

// NewFrontmatter creates a new frontmatter instance
func NewFrontmatter() *Frontmatter {
	return &Frontmatter{
		fields: make(map[string]interface{}),
		order:  make([]string, 0),
	}
}

// Set adds or updates a field in the frontmatter
func (f *Frontmatter) Set(key string, value interface{}) {
	if _, exists := f.fields[key]; !exists {
		f.order = append(f.order, key)
	}
	f.fields[key] = value
}

// Get retrieves a field value
func (f *Frontmatter) Get(key string) (interface{}, bool) {
	v, ok := f.fields[key]
	return v, ok
}

// Render produces the YAML frontmatter string with delimiters
func (f *Frontmatter) Render() string {
	if len(f.fields) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("---\n")

	for _, key := range f.order {
		value := f.fields[key]
		sb.WriteString(formatYAMLField(key, value))
	}

	sb.WriteString("---\n\n")
	return sb.String()
}

// formatYAMLField formats a single YAML field
func formatYAMLField(key string, value interface{}) string {
	switch v := value.(type) {
	case string:
		// Check if string needs quoting
		if needsQuoting(v) {
			return fmt.Sprintf("%s: %q\n", key, v)
		}
		return fmt.Sprintf("%s: %s\n", key, v)
	case int, int32, int64:
		return fmt.Sprintf("%s: %d\n", key, v)
	case float32, float64:
		return fmt.Sprintf("%s: %v\n", key, v)
	case bool:
		return fmt.Sprintf("%s: %t\n", key, v)
	case []string:
		return formatYAMLStringArray(key, v)
	case []interface{}:
		return formatYAMLInterfaceArray(key, v)
	default:
		return fmt.Sprintf("%s: %v\n", key, v)
	}
}

// needsQuoting checks if a string value needs YAML quoting
func needsQuoting(s string) bool {
	if s == "" {
		return true
	}
	// Quote strings with special characters
	specialChars := []string{":", "#", "[", "]", "{", "}", ",", "&", "*", "!", "|", ">", "'", "\"", "%", "@", "`"}
	for _, char := range specialChars {
		if strings.Contains(s, char) {
			return true
		}
	}
	// Quote strings that look like numbers or booleans
	lower := strings.ToLower(s)
	if lower == "true" || lower == "false" || lower == "null" || lower == "yes" || lower == "no" {
		return true
	}
	// Quote strings starting/ending with whitespace
	if strings.TrimSpace(s) != s {
		return true
	}
	return false
}

// formatYAMLStringArray formats a string array as YAML list
func formatYAMLStringArray(key string, values []string) string {
	if len(values) == 0 {
		return fmt.Sprintf("%s: []\n", key)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s:\n", key))
	for _, v := range values {
		if needsQuoting(v) {
			sb.WriteString(fmt.Sprintf("  - %q\n", v))
		} else {
			sb.WriteString(fmt.Sprintf("  - %s\n", v))
		}
	}
	return sb.String()
}

// formatYAMLInterfaceArray formats an interface array as YAML list
func formatYAMLInterfaceArray(key string, values []interface{}) string {
	if len(values) == 0 {
		return fmt.Sprintf("%s: []\n", key)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s:\n", key))
	for _, v := range values {
		switch val := v.(type) {
		case string:
			if needsQuoting(val) {
				sb.WriteString(fmt.Sprintf("  - %q\n", val))
			} else {
				sb.WriteString(fmt.Sprintf("  - %s\n", val))
			}
		default:
			sb.WriteString(fmt.Sprintf("  - %v\n", val))
		}
	}
	return sb.String()
}

// FromMap creates a Frontmatter from a map (sorted alphabetically)
func FromMap(m map[string]interface{}) *Frontmatter {
	f := NewFrontmatter()

	// Get sorted keys
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		f.Set(k, m[k])
	}

	return f
}
