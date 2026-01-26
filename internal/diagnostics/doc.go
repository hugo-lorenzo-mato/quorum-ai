// Package diagnostics provides system resource monitoring, crash dump generation,
// and safe command execution for long-running CLI sessions.
//
// The package implements three main components:
//
//   - ResourceMonitor: Periodically tracks file descriptors, goroutines, memory
//     usage, and command execution counts. Detects concerning trends like FD leaks.
//
//   - CrashDumpWriter: Captures and persists diagnostic information when panics
//     occur, enabling post-mortem debugging of crashes in production.
//
//   - SafeExecutor: Wraps command execution with pre-flight health checks and
//     guaranteed pipe cleanup, preventing resource leaks when cmd.Start() fails.
//
// Configuration is managed through DiagnosticsConfig in the config package.
// All monitoring is opt-in and configurable via the diagnostics section of
// the quorum configuration file.
package diagnostics
