# Stage 1: Builder
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /bot ./cmd/bot

# Stage 2: Runtime
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 botuser && \
    adduser -D -u 1000 -G botuser botuser

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /bot /app/bot

# Note: Migrations are embedded in the binary using go:embed
# No need to copy migration files separately

# Change ownership
RUN chown -R botuser:botuser /app

# Switch to non-root user
USER botuser

# Expose health check port (if implemented)
EXPOSE 8080

# Run the bot
CMD ["/app/bot"]

