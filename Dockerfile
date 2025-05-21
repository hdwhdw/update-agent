FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go module files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download && go mod tidy

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o upgrade-agent ./cmd/upgrade-agent

# Use a minimal alpine image for the final container
FROM alpine:3.19

# Add labels for better container management
LABEL maintainer="Your Name <your.email@example.com>"
LABEL version="1.0.0"
LABEL description="gRPC client for firmware updates via the SonicUpgradeService"

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/upgrade-agent .

# Create config directory
RUN mkdir -p /etc/upgrade-agent

# Create a non-root user to run the application
RUN adduser -D -h /app appuser && \
    chown -R appuser:appuser /app /etc/upgrade-agent

USER appuser

# Command to run the executable
ENTRYPOINT ["/app/upgrade-agent"]
