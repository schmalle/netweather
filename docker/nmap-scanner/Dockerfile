# Multi-stage build for both x64 and arm64 support
FROM golang:1.21-alpine AS builder

# Set working directory
WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the scanner service
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o nmap-scanner ./cmd/nmap-scanner

# Final stage - lightweight runtime image
FROM alpine:3.18

# Install nmap, nmap-scripts (NSE), and other required packages
RUN apk add --no-cache \
    nmap \
    nmap-scripts \
    ca-certificates \
    tzdata \
    && rm -rf /var/cache/apk/*

# Create non-root user for security
RUN addgroup -g 1000 scanner && \
    adduser -D -s /bin/sh -u 1000 -G scanner scanner

# Create directories for batch processing
RUN mkdir -p /app/batches /app/results && \
    chown -R scanner:scanner /app

# Copy the built binary from builder stage
COPY --from=builder /app/nmap-scanner /app/nmap-scanner

# Make binary executable
RUN chmod +x /app/nmap-scanner

# Switch to non-root user
USER scanner

# Set working directory
WORKDIR /app

# Expose the API port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Start the scanner service
CMD ["./nmap-scanner"]