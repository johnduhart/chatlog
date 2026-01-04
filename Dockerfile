# syntax=docker/dockerfile:1
# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY --exclude=config.yaml . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o chatlog .

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/chatlog .

# Copy configuration file
COPY config.yaml .

# Create directory for local file buffer
RUN mkdir -p /app/data

# Run as non-root user
#RUN addgroup -g 1000 chatlog && \
#    adduser -D -u 1000 -G chatlog chatlog && \
#    chown -R chatlog:chatlog /app

#USER chatlog

CMD ["./chatlog"]
