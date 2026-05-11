# Build stage
FROM golang:alpine AS builder

WORKDIR /app

# Install necessary build tools
RUN apk add --no-cache git

# Copy dependency files and download them
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application code
COPY . .

# Build the statically linked Go binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o memos-server cmd/memos/main.go

# Final stage (minimal image)
FROM alpine:latest

WORKDIR /app

# Add CA certificates so the app can make HTTPS requests (e.g. to OpenAI)
RUN apk --no-cache add ca-certificates tzdata

# Copy the binary from the builder stage
COPY --from=builder /app/memos-server .

# Expose gRPC port and Prometheus metrics port
EXPOSE 50051 9090

# Command to run the executable
CMD ["./memos-server"]
