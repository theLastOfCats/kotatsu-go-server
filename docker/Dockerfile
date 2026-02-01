# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o kotatsu-server ./cmd/server

# Final stage
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies (ca-certificates for HTTPS)
RUN apk add --no-cache ca-certificates

# Copy binary
COPY --from=builder /app/kotatsu-server .

# Create data directory
RUN mkdir -p /app/data

EXPOSE 8080

CMD ["./kotatsu-server"]
