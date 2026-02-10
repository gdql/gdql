# Build and test gdql in a reproducible environment.
# Usage: docker build -t gdql .   (from repo root)
#        docker run --rm gdql    (runs tests + query, exits 0 only if all pass)
FROM golang:1.24-alpine AS builder
WORKDIR /src

# Copy module files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the binary
RUN go build -o /gdql ./cmd/gdql

# Run all tests
RUN go test ./...

# --- Runtime stage: run the binary and the problematic query ---
FROM alpine:3.19
WORKDIR /app

# Copy binary from builder (embeddb is inside the binary)
COPY --from=builder /gdql /app/gdql

# Create query.gdql with exact ASCII content (no Unicode). Use printf so we control bytes.
RUN printf '%s\n' 'SHOWS FROM 1969 WHERE PLAYED "St Stephen" > "The Eleven";' > /app/query.gdql

# Verify query file content (optional: hexdump first byte of ">" position)
RUN od -c /app/query.gdql | head -2

# This must succeed; if gdql -f query.gdql fails, the container exits non-zero
RUN /app/gdql -f /app/query.gdql

# Default: run the query again when container runs (so "docker run gdql" is useful)
CMD ["/app/gdql", "-f", "/app/query.gdql"]
