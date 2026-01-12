#!/bin/bash
# scripts/release.sh
set -euo pipefail

VERSION="${1:-}"
if [[ -z "$VERSION" ]]; then
    echo "Usage: $0 <version>"
    echo "Example: $0 v0.1.0-alpha"
    exit 1
fi

echo "Preparing release ${VERSION}"

# Validate version format
if ! [[ "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9]+)?$ ]]; then
    echo "Error: Invalid version format. Use vX.Y.Z or vX.Y.Z-suffix"
    exit 1
fi

# Ensure clean working directory
if [[ -n "$(git status --porcelain)" ]]; then
    echo "Error: Working directory not clean"
    exit 1
fi

# Ensure on main branch
BRANCH=$(git branch --show-current)
if [[ "$BRANCH" != "main" ]]; then
    echo "Error: Must be on main branch"
    exit 1
fi

# Pull latest
echo "Pulling latest changes..."
git pull origin main

# Run tests
echo "Running tests..."
go test ./...

# Run lint if available
if command -v golangci-lint &> /dev/null; then
    echo "Running linter..."
    golangci-lint run
fi

# Build to verify
echo "Building binary..."
go build -o /dev/null ./cmd/quorum

# Dry-run release if goreleaser is available
if command -v goreleaser &> /dev/null; then
    echo "Testing release build..."
    goreleaser release --snapshot --clean --skip=publish
fi

echo ""
echo "Pre-release checks passed!"
echo ""
echo "Update CHANGELOG.md with release notes, then press Enter to continue..."
read -r

# Create and push tag
echo "Creating tag ${VERSION}..."
git tag -a "${VERSION}" -m "Release ${VERSION}"
git push origin "${VERSION}"

echo ""
echo "Tag pushed. GitHub Actions will create the release."
echo "Monitor at: https://github.com/hugo-lorenzo-mato/quorum-ai/actions"
