# ── Stage 1: build ───────────────────────────────────────────────────────────
FROM golang:1.26-alpine AS builder

# git is needed by `go mod download` for VCS stamping; ca-certificates for
# any outbound TLS during the build step.
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /src

# Cache module downloads separately from source so they survive code-only changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO_ENABLED=0 ensures a fully static binary (modernc/sqlite is pure Go).
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /bin/foodscheduler ./cmd/server

# ── Stage 2: runtime ─────────────────────────────────────────────────────────
FROM scratch

# Copy only what the binary needs at runtime.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /bin/foodscheduler /foodscheduler

# The app listens on $PORT (default 8080).
EXPOSE 8080

ENTRYPOINT ["/foodscheduler"]
