# Start from golang base image
FROM golang:1.23.5-alpine as builder

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -o video-converter

# Start a new stage from alpine
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ffmpeg

# Copy the binary from builder
COPY --from=builder /app/video-converter /usr/local/bin/

# Create a directory for video processing
WORKDIR /videos

# Create volume for persistent storage
VOLUME ["/videos"]

# Command to run the executable
ENTRYPOINT ["video-converter"]
