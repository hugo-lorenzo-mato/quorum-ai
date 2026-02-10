# syntax=docker/dockerfile:1.7

# =============================================================================
# Build stage: Compile Go binary
# =============================================================================
FROM --platform=$BUILDPLATFORM golang:1.22-alpine AS builder

# Build arguments
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown

# Install build dependencies
RUN apk add --no-cache \
    git \
    ca-certificates \
    tzdata

# Set working directory
WORKDIR /build

# Copy dependency files first for layer caching
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source code (validated by .dockerignore to exclude sensitive files)
COPY . .

# Build the binary
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH} \
    go build \
        -trimpath \
        -ldflags="-s -w \
            -X main.version=${VERSION} \
            -X main.commit=${COMMIT} \
            -X main.date=${DATE}" \
        -o /build/quorum \
        ./cmd/quorum

# =============================================================================
# Runtime stage: Minimal production image
# =============================================================================
FROM alpine:3.20 AS runtime

# Labels following OCI image spec
LABEL org.opencontainers.image.title="quorum-ai"
LABEL org.opencontainers.image.description="Multi-agent LLM orchestrator with consensus-based validation"
LABEL org.opencontainers.image.source="https://github.com/hugo-lorenzo-mato/quorum-ai"
LABEL org.opencontainers.image.vendor="Hugo Lorenzo Mato"
LABEL org.opencontainers.image.licenses="MIT"

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    git \
    && rm -rf /var/cache/apk/*

# Create non-root user
RUN addgroup -g 1000 quorum && \
    adduser -u 1000 -G quorum -s /bin/sh -D quorum

# Create necessary directories
RUN mkdir -p /home/quorum/.config/quorum /home/quorum/.quorum && \
    chown -R quorum:quorum /home/quorum

# Copy binary from builder
COPY --from=builder /build/quorum /usr/local/bin/quorum

# Set environment variables
ENV HOME=/home/quorum
ENV QUORUM_LOG_FORMAT=json

# Switch to non-root user
USER quorum
WORKDIR /home/quorum

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD ["quorum", "version"] || exit 1

# Default command
ENTRYPOINT ["quorum"]
CMD ["--help"]
