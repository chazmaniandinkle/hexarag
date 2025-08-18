# Build stage
FROM golang:1.24-alpine AS builder

# Set working directory
WORKDIR /app

# Install dependencies for CGO (required for SQLite)
RUN apk add --no-cache gcc musl-dev sqlite-dev

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o hexarag ./cmd/server
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o migrate ./cmd/migrate

# Runtime stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests and sqlite for runtime
RUN apk --no-cache add ca-certificates sqlite

# Create non-root user
RUN adduser -D -s /bin/sh hexarag

# Set working directory
WORKDIR /app

# Copy binaries from builder stage
COPY --from=builder /app/hexarag .
COPY --from=builder /app/migrate .

# Copy configuration and migration files
COPY --from=builder /app/deployments/config ./config
COPY --from=builder /app/internal/adapters/storage/sqlite/migrations ./migrations

# Create data directory
RUN mkdir -p /data && chown hexarag:hexarag /data

# Switch to non-root user
USER hexarag

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run migrations and start server
CMD ["sh", "-c", "./migrate && ./hexarag"]