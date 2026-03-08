# Multi-stage build for hCTF2

# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o hctf2 main.go

# Runtime stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 1000 hctf && \
    adduser -D -u 1000 -G hctf hctf

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/hctf2 .

# Create directory for database with correct permissions
RUN mkdir -p /app/data && chown -R hctf:hctf /app

# Switch to non-root user
USER hctf

# Expose port
EXPOSE 8090

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8090/healthz || exit 1

# Run the application
ENTRYPOINT ["./hctf2"]
CMD ["--port", "8090", "--db", "/app/data/hctf2.db"]
