package cli

import (
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// =============================================================================
// Streaming Configuration
// =============================================================================

// StreamMethod defines how an adapter provides real-time output.
type StreamMethod string

const (
	// StreamMethodNone indicates no streaming support.
	StreamMethodNone StreamMethod = "none"

	// StreamMethodJSONStdout indicates streaming via JSON lines on stdout.
	// Used by: Claude (--output-format stream-json), Gemini (--output-format stream-json), Codex (--json)
	StreamMethodJSONStdout StreamMethod = "json_stdout"

	// StreamMethodLogFile indicates streaming via log file tailing.
	// Used by: Copilot (--log-dir X --log-level debug)
	StreamMethodLogFile StreamMethod = "log_file"
)

// StreamConfig defines how to enable streaming for a specific CLI.
type StreamConfig struct {
	// Method specifies the streaming mechanism.
	Method StreamMethod

	// For StreamMethodJSONStdout:
	// OutputFormatFlag is the flag name (e.g., "--output-format" or "--json")
	OutputFormatFlag string
	// OutputFormatValue is the flag value (e.g., "stream-json")
	// Empty if the flag is boolean (like Codex's --json)
	OutputFormatValue string
	// RequiredFlags are additional flags needed for streaming (e.g., ["--verbose"] for Claude)
	RequiredFlags []string

	// For StreamMethodLogFile:
	// LogDirFlag is the flag to set log directory (e.g., "--log-dir")
	LogDirFlag string
	// LogLevelFlag is the flag to set log level (e.g., "--log-level")
	LogLevelFlag string
	// LogLevelValue is the value for debug logging (e.g., "debug")
	LogLevelValue string
}

// StreamConfigs holds the streaming configuration for each known CLI.
var StreamConfigs = map[string]StreamConfig{
	"claude": {
		Method:            StreamMethodJSONStdout,
		OutputFormatFlag:  "--output-format",
		OutputFormatValue: "stream-json",
		RequiredFlags:     []string{"--verbose"},
	},
	"gemini": {
		Method:            StreamMethodJSONStdout,
		OutputFormatFlag:  "--output-format",
		OutputFormatValue: "stream-json",
	},
	"codex": {
		Method:           StreamMethodJSONStdout,
		OutputFormatFlag: "--json",
		// No value needed - it's a boolean flag
	},
	"copilot": {
		Method:        StreamMethodLogFile,
		LogDirFlag:    "--log-dir",
		LogLevelFlag:  "--log-level",
		LogLevelValue: "debug",
	},
}

// =============================================================================
// Stream Parser Interface
// =============================================================================

// StreamParser converts CLI-specific output into generic AgentEvents.
// Each CLI has its own parser that understands its output format.
type StreamParser interface {
	// ParseLine processes a single line of output and returns any events.
	// May return nil/empty if the line doesn't contain relevant information.
	// May return multiple events if one line contains multiple pieces of info.
	ParseLine(line string) []core.AgentEvent

	// AgentName returns the name of the agent this parser handles.
	AgentName() string
}

// StreamParsers holds parser instances for each CLI.
var StreamParsers = make(map[string]StreamParser)

// RegisterStreamParser registers a parser for a CLI.
func RegisterStreamParser(name string, parser StreamParser) {
	StreamParsers[name] = parser
}

// GetStreamParser returns the parser for a CLI, or nil if none exists.
func GetStreamParser(name string) StreamParser {
	return StreamParsers[name]
}

// GetStreamConfig returns the streaming config for a CLI, with a default if not found.
func GetStreamConfig(name string) StreamConfig {
	if cfg, ok := StreamConfigs[name]; ok {
		return cfg
	}
	return StreamConfig{Method: StreamMethodNone}
}
