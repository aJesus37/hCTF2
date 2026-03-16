# Multi-stage build for hCTF

# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

ARG VERSION=dev
# Fully static binary — no libc dependency, suitable for scratch
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -extldflags=-static -X main.version=${VERSION}" \
    -o hctf main.go

# Pre-create writable directories owned by uid 1000
# scratch has no shell so we do this in the build stage
RUN mkdir -p /staging/data /staging/tmp && \
    chown -R 1000:1000 /staging

# Grab CA certs for HTTPS support (SMTP, OTLP, etc.)
FROM alpine:3.21 AS certs
RUN apk --no-cache add ca-certificates

# Runtime stage — scratch (zero OS overhead, minimal attack surface)
FROM scratch

# CA certificates for outbound HTTPS
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Static binary
COPY --from=builder /app/hctf /hctf

# Pre-created writable dirs (scratch has no shell to mkdir at runtime)
COPY --from=builder --chown=1000:1000 /staging/data /data
COPY --from=builder --chown=1000:1000 /staging/tmp /tmp

EXPOSE 8090

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/hctf", "healthcheck", "--port", "8090"]

# Run as non-root uid (no /etc/passwd needed when using numeric uid)
USER 1000

ENTRYPOINT ["/hctf"]
CMD ["serve", "--port", "8090", "--db", "/data/hctf.db"]
