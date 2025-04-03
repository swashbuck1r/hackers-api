# Build stage
FROM golang:1.23-alpine AS builder

# Install git for fetching dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download
RUN go install github.com/swaggo/swag/cmd/swag@latest

# Copy source code
COPY . .

# Generate Swagger docs
RUN swag init

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/hackers-api

# Final stage
FROM alpine:3.19.1

# Add CA certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/hackers-api .

# Expose port
EXPOSE 8080

# Run the application
CMD ["./hackers-api"] 
