# Build stage
FROM golang:1.23.3-alpine AS builder

WORKDIR /app

# Install dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY main.go ./

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bsky.wthr.cloud .

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/bsky.wthr.cloud .

# Make the binary executable
RUN chmod +x ./bsky.wthr.cloud

ENTRYPOINT ["./bsky.wthr.cloud"]
