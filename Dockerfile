# -----------------------------
# Stage 1: Build the Go binary
# -----------------------------
FROM golang:1.24-alpine AS builder

# Install git (needed for go mod download sometimes)
RUN apk add --no-cache git

WORKDIR /app

# Copy go mod first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy rest of the source
COPY . .

# Build statically linked binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server .

# -----------------------------
# Stage 2: Runtime image
# -----------------------------
FROM alpine:latest

# Install ffmpeg + ca certificates
RUN apk add --no-cache ffmpeg ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/server .

# Expose Railway's expected port (Railway sets $PORT)
EXPOSE 8080

# Create non-root user (security best practice)
RUN adduser -D appuser
USER appuser

# Run the server
CMD ["./server"]