# Stage 1: Build the application
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install git (needed for go get)
RUN apk add --no-cache git

# Copy source code
COPY . .

# Initialize a fresh Go module
RUN rm -f go.mod go.sum
RUN go mod init github.com/RaphaelA4U/ClipManager
RUN go get github.com/u2takey/ffmpeg-go@v0.5.0
RUN go mod tidy

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o clipmanager main.go

# Stage 2: Final image with FFmpeg installed directly
FROM alpine:3.18

WORKDIR /app

# Install FFmpeg and dependencies directly in the final image
RUN apk add --no-cache ffmpeg

# Create clips directory
RUN mkdir -p /app/clips

# Copy the binary
COPY --from=builder /app/clipmanager .

# Expose port 5000 - this is the default internal port
EXPOSE 5000

# The HOST_PORT is only for logging; the actual port mapping is handled by Docker
ENV HOST_PORT=5001

CMD ["./clipmanager"]