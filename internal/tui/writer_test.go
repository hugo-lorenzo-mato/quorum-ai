package tui

import "testing"

func TestParseLogLine_JSON(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input         string
		expectedLevel string
		expectedMsg   string
	}{
		{
			`{"level":"info","msg":"test message"}`,
			"info",
			"test message",
		},
		{
			`{"level":"WARN","message":"warning here"}`,
			"warn",
			"warning here",
		},
		{
			`{"level":"error","msg":"error occurred","key":"value"}`,
			"error",
			"error occurred",
		},
	}

	for _, tc := range tests {
		level, msg := parseLogLine(tc.input)
		if level != tc.expectedLevel {
			t.Errorf("parseLogLine(%q) level = %q, want %q", tc.input, level, tc.expectedLevel)
		}
		if msg != tc.expectedMsg {
			t.Errorf("parseLogLine(%q) msg = %q, want %q", tc.input, msg, tc.expectedMsg)
		}
	}
}

func TestParseLogLine_Pretty(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input         string
		expectedLevel string
		expectedMsg   string
	}{
		{
			"15:04:05 INF test message key=value",
			"info",
			"test message key=value",
		},
		{
			"15:04:05 WRN warning here",
			"warn",
			"warning here",
		},
		{
			"15:04:05 ERR error occurred",
			"error",
			"error occurred",
		},
	}

	for _, tc := range tests {
		level, msg := parseLogLine(tc.input)
		if level != tc.expectedLevel {
			t.Errorf("parseLogLine(%q) level = %q, want %q", tc.input, level, tc.expectedLevel)
		}
		if msg != tc.expectedMsg {
			t.Errorf("parseLogLine(%q) msg = %q, want %q", tc.input, msg, tc.expectedMsg)
		}
	}
}
