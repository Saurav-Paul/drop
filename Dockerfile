# Multi-stage build for Drop (Go).
# Stage 1: Build a fully static binary with CGO (required for SQLite).
# Stage 2: Copy into a scratch-based image — no OS, no shell, just the binary.
# Final image: ~10-12MB.

# --- Build stage ---
FROM golang:1.25-alpine AS builder

# gcc + musl-dev are required because mattn/go-sqlite3 is a CGO package
# (C library wrapped in Go). Without these, the build fails.
RUN apk add --no-cache gcc musl-dev

WORKDIR /src

# Cache dependencies — this layer only rebuilds when go.mod/go.sum change.
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build a fully static binary.
# -linkmode external -extldflags "-static" forces static linking of C deps,
# so the binary doesn't depend on musl/libc at runtime.
# -s -w strips debug symbols → smaller binary.
COPY . .
RUN CGO_ENABLED=1 go build \
    -ldflags="-s -w -linkmode external -extldflags '-static'" \
    -o /drop ./cmd/server/main.go \
    && mkdir /empty

# --- Runtime stage ---
# scratch = empty image. No shell, no OS, no package manager.
# The binary is fully static so it doesn't need anything else.
# This gives us the smallest possible image and zero attack surface.
FROM scratch

WORKDIR /app

# Copy the static binary
COPY --from=builder /drop /app/drop

# Copy templates and static assets (loaded at runtime, not embedded)
COPY templates/ /app/templates/
COPY static/ /app/static/

# scratch has no mkdir — create /data by copying an empty dir from builder.
# At runtime, docker-compose mounts a volume here anyway.
COPY --from=builder /empty /data

EXPOSE 8802

ENTRYPOINT ["/app/drop"]
