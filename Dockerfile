# Build stage
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /auth-service

# Development stage
FROM golang:1.23-alpine AS development
RUN apk add --no-cache git
WORKDIR /app
# Install air for hot reload
RUN go install github.com/air-verse/air@latest
# Install goose for migrations
RUN go install github.com/pressly/goose/v3/cmd/goose@latest
CMD ["air"]

# Production stage
FROM alpine:3.19 AS production
RUN apk add --no-cache ca-certificates
COPY --from=builder /auth-service /auth-service
CMD ["/auth-service"]
