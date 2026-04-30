# ==============================================================
# Upraizo Server — Production Dockerfile
# ==============================================================
# Multi-stage build: ~20MB final image
# Builds natively on x86_64 (GCP N2)
# ==============================================================

# ── Stage 1: Build ────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# Cache dependencies first
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build static binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -extldflags '-static'" \
    -o /build/upraizo-server \
    ./cmd/

# ── Stage 2: Runtime ─────────────────────────────────────────
FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata \
    && cp /usr/share/zoneinfo/Asia/Kolkata /etc/localtime \
    && echo "Asia/Kolkata" > /etc/timezone

# Non-root user
RUN addgroup -S appgroup && adduser -S -G appgroup -u 1000 appuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/upraizo-server .

# Create temp storage directory
RUN mkdir -p /app/storage/tmp && chown -R appuser:appgroup /app

USER appuser

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1

CMD ["./upraizo-server"]
