# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o /retry-middleware ./cmd/proxy

# Runtime stage
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary and default config
COPY --from=builder /retry-middleware /app/retry-middleware
COPY configs/config.yaml /app/configs/config.yaml

# Create logs directory
RUN mkdir -p /app/logs

# Expose proxy and metrics ports
EXPOSE 15722 9090

ENTRYPOINT ["/app/retry-middleware"]
CMD ["-config", "/app/configs/config.yaml"]
