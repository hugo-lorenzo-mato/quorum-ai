package cmd

import (
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
)

func TestParseTraceConfig_DefaultsToOff(t *testing.T) {
	cfg := &config.Config{}

	trace, err := parseTraceConfig(cfg, "")
	if err != nil {
		t.Fatalf("parseTraceConfig error: %v", err)
	}
	if trace.Mode != "off" {
		t.Fatalf("expected mode off, got %s", trace.Mode)
	}
}

func TestParseTraceConfig_Override(t *testing.T) {
	cfg := &config.Config{
		Trace: config.TraceConfig{Mode: "off"},
	}

	trace, err := parseTraceConfig(cfg, "full")
	if err != nil {
		t.Fatalf("parseTraceConfig error: %v", err)
	}
	if trace.Mode != "full" {
		t.Fatalf("expected mode full, got %s", trace.Mode)
	}
}

func TestParseTraceConfig_InvalidMode(t *testing.T) {
	cfg := &config.Config{
		Trace: config.TraceConfig{Mode: "off"},
	}

	if _, err := parseTraceConfig(cfg, "invalid"); err == nil {
		t.Fatalf("expected error for invalid mode")
	}
}
