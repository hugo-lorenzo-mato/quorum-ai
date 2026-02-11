#!/bin/bash
# Pre-commit validation script - mirrors CI checks
# Run this before pushing to catch issues early

set -e  # Exit on any error

FAILED=0
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT"

echo "=================================================="
echo "  Pre-Commit CI Validation"
echo "=================================================="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

run_check() {
    local name="$1"
    shift
    echo -n "[$name] "
    if "$@" > /tmp/validate-ci-$$.log 2>&1; then
        echo -e "${GREEN}PASS${NC}"
        return 0
    else
        echo -e "${RED}FAIL${NC}"
        echo "  Error output:"
        cat /tmp/validate-ci-$$.log | head -20
        echo ""
        FAILED=1
        return 1
    fi
}

# 1. Format Check
echo "=== Code Formatting ==="
run_check "gofmt" sh -c "test -z \"\$(gofmt -l internal/ cmd/)\""
echo ""

# 2. Build Frontend (required for Go tests)
echo "=== Building Frontend ==="
run_check "make build-frontend" make build-frontend
echo ""

# 3. Lint
echo "=== Linting ==="
# Note: golangci-lint has continue-on-error in CI for Go 1.25 compatibility
# We run it but don't fail the script if it reports warnings
echo -n "[golangci-lint] "
if golangci-lint run --timeout=10m > /tmp/validate-ci-$$.log 2>&1; then
    echo -e "${GREEN}PASS${NC}"
else
    # Check if there are only warnings (exit code 1) vs actual errors
    if grep -q "Error: " /tmp/validate-ci-$$.log; then
        echo -e "${RED}FAIL${NC}"
        cat /tmp/validate-ci-$$.log | head -20
        FAILED=1
    else
        echo -e "${YELLOW}WARNINGS${NC} (non-blocking in CI)"
    fi
fi
run_check "eslint" sh -c "cd frontend && npm run lint"
echo ""

# 4. Backend Tests
echo "=== Backend Tests ==="
run_check "go test" go test -race -v -timeout 10m ./...
echo ""

# 5. Frontend Tests
echo "=== Frontend Tests ==="
run_check "npm test" sh -c "cd frontend && npm run test:coverage"
echo ""

# 6. Integration Tests
echo "=== Integration Tests ==="
run_check "integration" go test -race -v -tags=integration -timeout 20m ./...
echo ""

# 7. Security Checks
echo "=== Security Checks ==="
run_check "govulncheck" govulncheck ./...
# Note: gosec has continue-on-error in CI
echo -n "[gosec] "
if gosec -exclude=G104,G301,G302,G306 -exclude-dir=.git -exclude-dir=frontend ./... > /tmp/validate-ci-$$.log 2>&1; then
    echo -e "${GREEN}PASS${NC}"
else
    echo -e "${YELLOW}WARNINGS${NC} (non-blocking in CI)"
fi
# Note: npm audit has continue-on-error in CI
echo -n "[npm audit] "
if (cd frontend && npm audit --audit-level=high) > /tmp/validate-ci-$$.log 2>&1; then
    echo -e "${GREEN}PASS${NC}"
else
    echo -e "${YELLOW}WARNINGS${NC} (non-blocking in CI)"
fi
echo ""

# 8. Cross-Compilation
echo "=== Cross-Compilation ==="
run_check "linux/amd64" sh -c "GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o /tmp/quorum-test ./cmd/quorum"
run_check "darwin/arm64" sh -c "GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o /tmp/quorum-test ./cmd/quorum"
run_check "windows/amd64" sh -c "GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o /tmp/quorum-test.exe ./cmd/quorum"
echo ""

# Cleanup
rm -f /tmp/validate-ci-$$.log
rm -f /tmp/quorum-test /tmp/quorum-test.exe

echo "=================================================="
if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ All validations passed! Safe to push.${NC}"
    echo "=================================================="
    exit 0
else
    echo -e "${RED}✗ Some validations failed. Fix issues before pushing.${NC}"
    echo "=================================================="
    exit 1
fi
