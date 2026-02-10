package report

import (
	"strings"
	"testing"
)

func TestNewFrontmatter(t *testing.T) {
	f := NewFrontmatter()
	if f == nil {
		t.Fatal("expected non-nil")
	}
	// Empty frontmatter renders empty
	if got := f.Render(); got != "" {
		t.Errorf("expected empty render, got %q", got)
	}
}

func TestFrontmatter_SetGet(t *testing.T) {
	f := NewFrontmatter()
	f.Set("title", "Test Report")
	f.Set("version", 1)

	v, ok := f.Get("title")
	if !ok || v != "Test Report" {
		t.Errorf("Get(title) = %v, %v", v, ok)
	}

	v, ok = f.Get("version")
	if !ok || v != 1 {
		t.Errorf("Get(version) = %v, %v", v, ok)
	}

	_, ok = f.Get("missing")
	if ok {
		t.Error("expected missing key to return false")
	}
}

func TestFrontmatter_SetOverwrite(t *testing.T) {
	f := NewFrontmatter()
	f.Set("key", "value1")
	f.Set("key", "value2")

	v, _ := f.Get("key")
	if v != "value2" {
		t.Errorf("expected overwritten value, got %v", v)
	}

	// Should not duplicate in order
	rendered := f.Render()
	if strings.Count(rendered, "key:") != 1 {
		t.Errorf("key appears multiple times in render: %s", rendered)
	}
}

func TestFrontmatter_Render(t *testing.T) {
	f := NewFrontmatter()
	f.Set("title", "My Report")
	f.Set("version", 1)
	f.Set("draft", true)
	f.Set("score", 0.95)

	rendered := f.Render()

	if !strings.HasPrefix(rendered, "---\n") {
		t.Error("should start with ---")
	}
	if !strings.Contains(rendered, "---\n\n") {
		t.Error("should end with ---\\n\\n")
	}
	if !strings.Contains(rendered, "title: My Report\n") {
		t.Errorf("missing title: %s", rendered)
	}
	if !strings.Contains(rendered, "version: 1\n") {
		t.Errorf("missing version: %s", rendered)
	}
	if !strings.Contains(rendered, "draft: true\n") {
		t.Errorf("missing draft: %s", rendered)
	}
}

func TestFrontmatter_Render_PreservesOrder(t *testing.T) {
	f := NewFrontmatter()
	f.Set("zeta", "last")
	f.Set("alpha", "first")

	rendered := f.Render()
	zetaIdx := strings.Index(rendered, "zeta:")
	alphaIdx := strings.Index(rendered, "alpha:")
	if zetaIdx > alphaIdx {
		t.Error("expected insertion order to be preserved (zeta before alpha)")
	}
}

func TestNeedsQuoting(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"", true},
		{"simple", false},
		{"has:colon", true},
		{"has #comment", true},
		{"true", true},
		{"false", true},
		{"null", true},
		{"yes", true},
		{"no", true},
		{" leading space", true},
		{"trailing space ", true},
		{"has*star", true},
		{"normal-text", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := needsQuoting(tt.input); got != tt.want {
				t.Errorf("needsQuoting(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatYAMLField_Types(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value interface{}
		want  string
	}{
		{"string simple", "name", "hello", "name: hello\n"},
		{"string quoted", "name", "has:colon", "name: \"has:colon\"\n"},
		{"int", "count", 42, "count: 42\n"},
		{"bool", "flag", true, "flag: true\n"},
		{"float", "score", 0.95, "score: 0.95\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatYAMLField(tt.key, tt.value)
			if got != tt.want {
				t.Errorf("formatYAMLField(%q, %v) = %q, want %q", tt.key, tt.value, got, tt.want)
			}
		})
	}
}

func TestFormatYAMLStringArray(t *testing.T) {
	// Empty array
	got := formatYAMLStringArray("tags", nil)
	if got != "tags: []\n" {
		t.Errorf("empty array: got %q", got)
	}

	// Non-empty
	got = formatYAMLStringArray("tags", []string{"go", "test"})
	if !strings.Contains(got, "  - go\n") || !strings.Contains(got, "  - test\n") {
		t.Errorf("unexpected: %q", got)
	}

	// With quoting needed
	got = formatYAMLStringArray("values", []string{"has:colon"})
	if !strings.Contains(got, "\"has:colon\"") {
		t.Errorf("expected quoted value in: %q", got)
	}
}

func TestFormatYAMLInterfaceArray(t *testing.T) {
	got := formatYAMLInterfaceArray("items", nil)
	if got != "items: []\n" {
		t.Errorf("empty array: got %q", got)
	}

	got = formatYAMLInterfaceArray("items", []interface{}{"text", 42})
	if !strings.Contains(got, "  - text\n") || !strings.Contains(got, "  - 42\n") {
		t.Errorf("unexpected: %q", got)
	}

	// String needing quoting
	got = formatYAMLInterfaceArray("items", []interface{}{"has:colon"})
	if !strings.Contains(got, "\"has:colon\"") {
		t.Errorf("expected quoted: %q", got)
	}
}

func TestFromMap(t *testing.T) {
	m := map[string]interface{}{
		"zebra": "last",
		"alpha": "first",
		"beta":  42,
	}

	f := FromMap(m)
	rendered := f.Render()

	// FromMap sorts alphabetically
	alphaIdx := strings.Index(rendered, "alpha:")
	betaIdx := strings.Index(rendered, "beta:")
	zebraIdx := strings.Index(rendered, "zebra:")

	if alphaIdx > betaIdx || betaIdx > zebraIdx {
		t.Errorf("expected alphabetical order: alpha=%d beta=%d zebra=%d", alphaIdx, betaIdx, zebraIdx)
	}
}
