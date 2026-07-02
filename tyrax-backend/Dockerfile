# ─────────────────────────────────────────────────────────────────────────────
# TYRAX Backend — multi-stage build
# Stage 1 compiles a static binary; stage 2 ships a minimal alpine runtime.
# Final image is ~20MB (alpine base + stripped static binary).
# ─────────────────────────────────────────────────────────────────────────────

# ── Stage 1: builder ─────────────────────────────────────────────────────────
# Go toolchain must match the version pinned in go.mod.
FROM golang:1.26-alpine AS builder

WORKDIR /build

# Cache module downloads as a separate layer.
COPY go.mod go.sum ./
RUN go mod download

# Build the static, stripped binary (no CGO → portable).
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -trimpath \
    -o /tyrax ./cmd/server

# ── Stage 2: runtime ─────────────────────────────────────────────────────────
FROM alpine:3.20

# ca-certificates: outbound TLS to FreeKassa / CryptoPay / Telegram APIs.
# wget: used by the container HEALTHCHECK.
RUN apk add --no-cache ca-certificates wget \
    && adduser -D -u 10001 tyrax

COPY --from=builder /tyrax /usr/local/bin/tyrax
COPY admin /app/admin

WORKDIR /app
USER tyrax
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1

ENTRYPOINT ["tyrax"]
