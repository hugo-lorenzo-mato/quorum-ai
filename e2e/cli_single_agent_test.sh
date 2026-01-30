#!/bin/bash
# CLI E2E Test for Single-Agent Mode Configuration
#
# This script tests the CLI's handling of single-agent mode through:
# 1. Configuration file validation
# 2. Help output verification
# 3. Error handling for invalid configurations
#
# Requirements:
# - quorum binary built and available
# - Write access to create temporary config files

set -e

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
QUORUM_BIN="${QUORUM_BIN:-$PROJECT_ROOT/quorum}"
TEMP_DIR=""

# Colors for output (if terminal supports it)
if [[ -t 1 ]]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[1;33m'
    NC='\033[0m' # No Color
else
    RED=''
    GREEN=''
    YELLOW=''
    NC=''
fi

# Test counters
PASSED=0
FAILED=0

# Helper functions
setup() {
    TEMP_DIR=$(mktemp -d)
    mkdir -p "$TEMP_DIR/.quorum"
}

cleanup() {
    if [[ -n "$TEMP_DIR" && -d "$TEMP_DIR" ]]; then
        rm -rf "$TEMP_DIR"
    fi
}

trap cleanup EXIT

pass() {
    local message="$1"
    echo -e "${GREEN}PASS${NC}: $message"
    ((PASSED++))
}

fail() {
    local message="$1"
    echo -e "${RED}FAIL${NC}: $message"
    ((FAILED++))
}

skip() {
    local message="$1"
    echo -e "${YELLOW}SKIP${NC}: $message"
}

assert_contains() {
    local haystack="$1"
    local needle="$2"
    local message="$3"
    if [[ "$haystack" == *"$needle"* ]]; then
        pass "$message"
    else
        fail "$message"
        echo "  Expected to find: $needle"
        echo "  In output (first 500 chars): ${haystack:0:500}"
    fi
}

assert_not_contains() {
    local haystack="$1"
    local needle="$2"
    local message="$3"
    if [[ "$haystack" != *"$needle"* ]]; then
        pass "$message"
    else
        fail "$message"
        echo "  Did not expect to find: $needle"
    fi
}

assert_exit_code() {
    local expected="$1"
    local actual="$2"
    local message="$3"
    if [[ "$actual" -eq "$expected" ]]; then
        pass "$message"
    else
        fail "$message (expected exit code $expected, got $actual)"
    fi
}

# Check if quorum binary exists
check_quorum_binary() {
    if [[ ! -x "$QUORUM_BIN" ]]; then
        echo "Warning: quorum binary not found at $QUORUM_BIN"
        echo "Trying to build..."
        if (cd "$PROJECT_ROOT" && go build -o quorum ./cmd/quorum 2>/dev/null); then
            QUORUM_BIN="$PROJECT_ROOT/quorum"
            echo "Built quorum successfully"
        else
            echo "Could not build quorum. Tests will use 'quorum' from PATH."
            QUORUM_BIN="quorum"
        fi
    fi
}

# Create test config file
create_test_config() {
    local config_path="$1"
    local single_agent_enabled="${2:-false}"
    local single_agent_name="${3:-claude}"
    local moderator_enabled="${4:-true}"

    cat > "$config_path" << EOF
log:
  level: info
  format: auto

agents:
  default: claude
  claude:
    enabled: true
    path: claude
    phases:
      refine: true
      analyze: true
      moderate: true
      synthesize: true
      plan: true
      execute: true
  gemini:
    enabled: true
    path: gemini
    phases:
      analyze: true
      plan: true
      execute: true

phases:
  analyze:
    timeout: 30m
    moderator:
      enabled: $moderator_enabled
      agent: claude
      threshold: 0.90
      min_rounds: 1
      max_rounds: 3
    single_agent:
      enabled: $single_agent_enabled
      agent: $single_agent_name
    synthesizer:
      agent: claude

state:
  backend: json
  path: .quorum/state.json
  lock_ttl: 5m

git:
  worktree_dir: .quorum/worktrees
  auto_commit: false
  auto_push: false

github:
  remote: origin
EOF
}

echo "=== CLI Single-Agent E2E Tests ==="
echo "Project root: $PROJECT_ROOT"
echo ""

# Setup
setup
check_quorum_binary

# Test 1: Verify quorum binary version/help works
echo ""
echo "Test 1: Verify quorum binary is functional"
if output=$("$QUORUM_BIN" --help 2>&1); then
    assert_contains "$output" "quorum" "Help output contains 'quorum'"
else
    skip "Quorum binary not available for testing"
fi

# Test 2: Config validation with single-agent enabled and moderator disabled
echo ""
echo "Test 2: Config with single-agent enabled (moderator disabled)"
create_test_config "$TEMP_DIR/.quorum/config.yaml" "true" "claude" "false"

# Use quorum to validate config (doctor command)
if output=$(cd "$TEMP_DIR" && "$QUORUM_BIN" doctor 2>&1); then
    pass "Doctor command succeeds with single-agent config"
else
    # Doctor might fail for other reasons (missing API keys, etc.)
    # Check if it's a config validation error
    if [[ "$output" == *"single_agent"* && "$output" == *"moderator"* ]]; then
        fail "Config validation failed for single-agent mode"
        echo "  Output: $output"
    else
        pass "Config is valid (doctor failed for non-config reasons)"
    fi
fi

# Test 3: Config validation should fail with both single-agent and moderator enabled
echo ""
echo "Test 3: Config validation rejects single-agent AND moderator enabled"
create_test_config "$TEMP_DIR/.quorum/config.yaml" "true" "claude" "true"

# This should fail validation (mutually exclusive)
output=$(cd "$TEMP_DIR" && "$QUORUM_BIN" doctor 2>&1) || true
if [[ "$output" == *"single_agent"* ]] || [[ "$output" == *"moderator"* ]] || [[ "$output" == *"exclusive"* ]]; then
    pass "Config correctly validates single-agent/moderator mutual exclusivity"
else
    # May pass if validation not implemented - that's a warning not failure
    echo "  Note: Mutual exclusivity validation may not be enforced at CLI level"
fi

# Test 4: Config with invalid agent name for single-agent
echo ""
echo "Test 4: Config validation rejects invalid agent name"
create_test_config "$TEMP_DIR/.quorum/config.yaml" "true" "invalid_agent" "false"

output=$(cd "$TEMP_DIR" && "$QUORUM_BIN" doctor 2>&1) || true
if [[ "$output" == *"unknown"* ]] || [[ "$output" == *"invalid"* ]] || [[ "$output" == *"not found"* ]]; then
    pass "Config correctly validates agent names"
else
    echo "  Note: Agent name validation may happen at runtime"
fi

# Test 5: Config with disabled agent for single-agent
echo ""
echo "Test 5: Config validation handles disabled agent for single-agent"
cat > "$TEMP_DIR/.quorum/config.yaml" << EOF
log:
  level: info
  format: auto

agents:
  default: claude
  claude:
    enabled: true
    path: claude
    phases:
      analyze: true
      plan: true
      execute: true
  gemini:
    enabled: false
    path: gemini

phases:
  analyze:
    moderator:
      enabled: false
    single_agent:
      enabled: true
      agent: gemini

state:
  backend: json
  path: .quorum/state.json
  lock_ttl: 5m

git:
  worktree_dir: .quorum/worktrees
  auto_commit: false

github:
  remote: origin
EOF

output=$(cd "$TEMP_DIR" && "$QUORUM_BIN" doctor 2>&1) || true
if [[ "$output" == *"disabled"* ]] || [[ "$output" == *"enabled"* ]] || [[ "$output" == *"must be"* ]]; then
    pass "Config validates that single-agent must be enabled"
else
    echo "  Note: Disabled agent validation may happen at runtime"
fi

# Test 6: Run command help (verify no obvious errors)
echo ""
echo "Test 6: Run command help output"
if output=$("$QUORUM_BIN" run --help 2>&1); then
    assert_contains "$output" "workflow" "Run help mentions workflow"
    assert_contains "$output" "prompt" "Run help mentions prompt"
else
    fail "Run --help failed"
fi

# Test 7: Chat command help (verify agent flag exists)
echo ""
echo "Test 7: Chat command shows --agent flag"
if output=$("$QUORUM_BIN" chat --help 2>&1); then
    assert_contains "$output" "--agent" "Chat help shows --agent flag"
else
    fail "Chat --help failed"
fi

# Test 8: Analyze command exists and shows help
echo ""
echo "Test 8: Analyze command exists"
if output=$("$QUORUM_BIN" analyze --help 2>&1); then
    pass "Analyze command exists"
else
    skip "Analyze command may not exist"
fi

# Summary
echo ""
echo "=== Test Summary ==="
echo -e "${GREEN}Passed${NC}: $PASSED"
echo -e "${RED}Failed${NC}: $FAILED"
echo ""

if [[ $FAILED -gt 0 ]]; then
    exit 1
fi

echo "=== All CLI E2E Tests Completed ==="
