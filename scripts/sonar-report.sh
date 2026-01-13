#!/bin/bash
# Download SonarCloud report locally
# Requires: SONAR_TOKEN environment variable
#
# Usage: ./scripts/sonar-report.sh [pr-number]

set -e

PROJECT_KEY="hugo-lorenzo-mato_quorum-ai"
REPORT_DIR="${REPORT_DIR:-/tmp/sonar-report}"

if [ -z "$SONAR_TOKEN" ]; then
  echo "Error: SONAR_TOKEN not set"
  echo "Export it: export SONAR_TOKEN=your_token"
  exit 1
fi

mkdir -p "$REPORT_DIR"

echo "=== SonarCloud Report Download ==="
echo ""

# Check if PR number provided
if [ -n "$1" ]; then
  echo "Fetching PR #$1 analysis..."
  PR_PARAM="&pullRequest=$1"
else
  echo "Fetching main branch analysis..."
  PR_PARAM=""
fi

# 1. Project metrics
echo "[1/4] Downloading metrics..."
curl -s -u "$SONAR_TOKEN:" \
  "https://sonarcloud.io/api/measures/component?component=$PROJECT_KEY&metricKeys=bugs,vulnerabilities,code_smells,coverage,duplicated_lines_density,ncloc,cognitive_complexity,security_hotspots" \
  | jq '.' > "$REPORT_DIR/metrics.json"

# 2. Issues
echo "[2/4] Downloading issues..."
curl -s -u "$SONAR_TOKEN:" \
  "https://sonarcloud.io/api/issues/search?projectKeys=$PROJECT_KEY&ps=500&resolved=false$PR_PARAM" \
  | jq '.' > "$REPORT_DIR/issues.json"

# 3. Quality Gate status
echo "[3/4] Downloading quality gate..."
curl -s -u "$SONAR_TOKEN:" \
  "https://sonarcloud.io/api/qualitygates/project_status?projectKey=$PROJECT_KEY" \
  | jq '.' > "$REPORT_DIR/quality-gate.json"

# 4. Hotspots (security)
echo "[4/4] Downloading security hotspots..."
curl -s -u "$SONAR_TOKEN:" \
  "https://sonarcloud.io/api/hotspots/search?projectKey=$PROJECT_KEY&ps=500" \
  | jq '.' > "$REPORT_DIR/hotspots.json"

echo ""
echo "=== Summary ==="

# Parse and display summary
BUGS=$(jq -r '.component.measures[] | select(.metric=="bugs") | .value' "$REPORT_DIR/metrics.json" 2>/dev/null || echo "N/A")
VULNS=$(jq -r '.component.measures[] | select(.metric=="vulnerabilities") | .value' "$REPORT_DIR/metrics.json" 2>/dev/null || echo "N/A")
SMELLS=$(jq -r '.component.measures[] | select(.metric=="code_smells") | .value' "$REPORT_DIR/metrics.json" 2>/dev/null || echo "N/A")
COVERAGE=$(jq -r '.component.measures[] | select(.metric=="coverage") | .value' "$REPORT_DIR/metrics.json" 2>/dev/null || echo "N/A")
HOTSPOTS=$(jq -r '.paging.total' "$REPORT_DIR/hotspots.json" 2>/dev/null || echo "N/A")
GATE=$(jq -r '.projectStatus.status' "$REPORT_DIR/quality-gate.json" 2>/dev/null || echo "N/A")

echo "Quality Gate:    $GATE"
echo "Bugs:            $BUGS"
echo "Vulnerabilities: $VULNS"
echo "Code Smells:     $SMELLS"
echo "Coverage:        $COVERAGE%"
echo "Hotspots:        $HOTSPOTS"
echo ""
echo "Issues by severity:"
jq -r '.issues | group_by(.severity) | map({severity: .[0].severity, count: length}) | .[] | "  \(.severity): \(.count)"' "$REPORT_DIR/issues.json" 2>/dev/null || echo "  No issues"

echo ""
echo "Reports saved to: $REPORT_DIR"
ls -la "$REPORT_DIR"
